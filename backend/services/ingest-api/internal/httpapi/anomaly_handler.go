package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/anomaly"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/storage"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
	"github.com/gin-gonic/gin"
)

type AnomalyHandler struct {
	logger     *slog.Logger
	repository storage.Repository
}

type analyzedMiner struct {
	MinerID    string
	Resolution storage.BucketResolution
	From       time.Time
	To         time.Time
	Report     anomaly.Report
}

type changeApplyRequest struct {
	Reason      string `json:"reason"`
	RequestedBy string `json:"requested_by"`
}

type changeRollbackRequest struct {
	Reason      string `json:"reason"`
	RequestedBy string `json:"requested_by"`
}

func NewAnomalyHandler(logger *slog.Logger, repository storage.Repository) *AnomalyHandler {
	return &AnomalyHandler{
		logger:     logger,
		repository: repository,
	}
}

func (h *AnomalyHandler) AnalyzeMiner(c *gin.Context) {
	analysis, ok := h.computeMinerReport(c)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id":   RequestID(c),
		"resolution":   analysis.Resolution,
		"from":         analysis.From,
		"to":           analysis.To,
		"summary_line": anomaly.SummaryLine(analysis.Report),
		"report":       analysis.Report,
	})
}

func (h *AnomalyHandler) ApplyRecommendationChange(c *gin.Context) {
	if h.repository == nil {
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "anomaly api unavailable", nil)
		return
	}

	var request changeApplyRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid request body: %v", err), nil)
			return
		}
	}

	analysis, ok := h.computeMinerReport(c)
	if !ok {
		return
	}

	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		reason = "apply guarded recommendation generated from anomaly analysis"
	}
	requestedBy := strings.TrimSpace(request.RequestedBy)
	if requestedBy == "" {
		requestedBy = "system"
	}

	sourceReport, err := toMap(analysis.Report)
	if err != nil {
		h.logger.Error("failed to encode source report", "error", err, "miner_id", analysis.MinerID)
		WriteError(c, http.StatusInternalServerError, "internal_error", "failed to encode source report", nil)
		return
	}

	recommendations, err := toSliceMap(analysis.Report.Recommendations)
	if err != nil {
		h.logger.Error("failed to encode recommendations", "error", err, "miner_id", analysis.MinerID)
		WriteError(c, http.StatusInternalServerError, "internal_error", "failed to encode recommendations", nil)
		return
	}

	impactEstimate, err := toMap(analysis.Report.ImpactEstimate)
	if err != nil {
		h.logger.Error("failed to encode impact estimate", "error", err, "miner_id", analysis.MinerID)
		WriteError(c, http.StatusInternalServerError, "internal_error", "failed to encode impact estimate", nil)
		return
	}

	change, err := h.repository.CreateRecommendationChange(c.Request.Context(), storage.RecommendationChangeCreateInput{
		SiteID:          analysis.Report.SiteID,
		RackID:          analysis.Report.RackID,
		MinerID:         analysis.MinerID,
		Reason:          reason,
		RequestedBy:     requestedBy,
		Summary:         anomaly.SummaryLine(analysis.Report),
		SourceReport:    sourceReport,
		Recommendations: recommendations,
		ImpactEstimate:  impactEstimate,
	})
	if err != nil {
		h.logger.Error("failed to persist recommendation change", "error", err, "miner_id", analysis.MinerID)
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			WriteError(c, http.StatusNotFound, "not_found", err.Error(), nil)
			return
		}
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to persist recommendation change", nil)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"request_id":   RequestID(c),
		"change":       change,
		"resolution":   analysis.Resolution,
		"from":         analysis.From,
		"to":           analysis.To,
		"summary_line": anomaly.SummaryLine(analysis.Report),
		"report":       analysis.Report,
	})
}

func (h *AnomalyHandler) RollbackRecommendationChange(c *gin.Context) {
	if h.repository == nil {
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "anomaly api unavailable", nil)
		return
	}

	changeID := strings.TrimSpace(c.Param("change_id"))
	if changeID == "" {
		WriteError(c, http.StatusBadRequest, "validation_error", "change_id is required", nil)
		return
	}

	var request changeRollbackRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid request body: %v", err), nil)
			return
		}
	}

	rollbackChange, err := h.repository.RollbackRecommendationChange(c.Request.Context(), storage.RecommendationRollbackInput{
		ChangeID:    changeID,
		Reason:      request.Reason,
		RequestedBy: request.RequestedBy,
	})
	if err != nil {
		h.logger.Error("failed to rollback recommendation change", "error", err, "change_id", changeID)
		lower := strings.ToLower(err.Error())
		switch {
		case strings.Contains(lower, "not found"):
			WriteError(c, http.StatusNotFound, "not_found", err.Error(), nil)
		case strings.Contains(lower, "already rolled back"):
			WriteError(c, http.StatusConflict, "conflict", err.Error(), nil)
		default:
			WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to rollback recommendation change", nil)
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"request_id": RequestID(c),
		"change":     rollbackChange,
	})
}

func (h *AnomalyHandler) ListRecommendationChanges(c *gin.Context) {
	if h.repository == nil {
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "anomaly api unavailable", nil)
		return
	}

	filter := storage.RecommendationChangeFilter{}

	if minerRaw := strings.TrimSpace(c.Query("miner_id")); minerRaw != "" {
		minerID, err := telemetry.NormalizeMinerID(minerRaw)
		if err != nil {
			WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid miner_id: %v", err), nil)
			return
		}
		filter.MinerID = minerID
	}

	statusRaw := strings.TrimSpace(strings.ToLower(c.Query("status")))
	switch statusRaw {
	case "":
	case string(storage.ChangeStatusApplied):
		filter.Status = storage.ChangeStatusApplied
	case string(storage.ChangeStatusRolledBack):
		filter.Status = storage.ChangeStatusRolledBack
	default:
		WriteError(c, http.StatusBadRequest, "validation_error", "status must be one of: applied, rolled_back", nil)
		return
	}

	limit, err := parsePositiveInt(c.Query("limit"), 50, 500, "limit")
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}
	filter.Limit = limit

	items, err := h.repository.ListRecommendationChanges(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list recommendation changes", "error", err)
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to list recommendation changes", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id": RequestID(c),
		"count":      len(items),
		"items":      items,
	})
}

func (h *AnomalyHandler) computeMinerReport(c *gin.Context) (analyzedMiner, bool) {
	if h.repository == nil {
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "read api unavailable", nil)
		return analyzedMiner{}, false
	}

	minerID, err := telemetry.NormalizeMinerID(c.Param("miner_id"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid miner_id: %v", err), nil)
		return analyzedMiner{}, false
	}

	resolution, err := parseResolution(c.Query("resolution"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return analyzedMiner{}, false
	}

	from, err := parseOptionalTime(c.Query("from"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid from timestamp: %v", err), nil)
		return analyzedMiner{}, false
	}
	to, err := parseOptionalTime(c.Query("to"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid to timestamp: %v", err), nil)
		return analyzedMiner{}, false
	}

	now := time.Now().UTC()
	resolvedTo := now
	if to != nil {
		resolvedTo = to.UTC()
	}
	defaultWindow := 3 * time.Hour
	if resolution == storage.ResolutionHour {
		defaultWindow = 72 * time.Hour
	}
	resolvedFrom := resolvedTo.Add(-defaultWindow)
	if from != nil {
		resolvedFrom = from.UTC()
	}

	if resolvedFrom.After(resolvedTo) {
		WriteError(c, http.StatusBadRequest, "validation_error", "from must be before to", nil)
		return analyzedMiner{}, false
	}

	limit, err := parsePositiveInt(c.Query("limit"), defaultSeriesLimitByResolution(resolution), 10000, "limit")
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return analyzedMiner{}, false
	}

	series, err := h.repository.ListMinerSeries(c.Request.Context(), storage.MinerSeriesFilter{
		MinerID:    minerID,
		From:       resolvedFrom,
		To:         resolvedTo,
		Resolution: resolution,
		Limit:      limit,
	})
	if err != nil {
		h.logger.Error("failed to query miner series for anomaly analysis", "error", err, "miner_id", minerID)
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to query miner timeseries", nil)
		return analyzedMiner{}, false
	}

	readings, err := h.repository.ListReadings(c.Request.Context(), storage.ReadingsFilter{
		MinerID: minerID,
		From:    &resolvedFrom,
		To:      &resolvedTo,
		Limit:   1,
	})
	if err != nil {
		h.logger.Error("failed to query miner latest reading for anomaly analysis", "error", err, "miner_id", minerID)
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to query miner readings", nil)
		return analyzedMiner{}, false
	}
	if len(readings) == 0 || len(series) == 0 {
		WriteError(c, http.StatusNotFound, "not_found", "insufficient telemetry data for anomaly analysis", nil)
		return analyzedMiner{}, false
	}

	ambientOverride, err := parseOptionalFloat(c.Query("ambient_c"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return analyzedMiner{}, false
	}

	report := anomaly.Analyze(readings[0], series, ambientOverride)
	return analyzedMiner{
		MinerID:    minerID,
		Resolution: resolution,
		From:       resolvedFrom,
		To:         resolvedTo,
		Report:     report,
	}, true
}

func parseOptionalFloat(raw string) (*float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("ambient_c must be a valid decimal number")
	}

	return &parsed, nil
}

func toMap(value any) (map[string]any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	decoded := map[string]any{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, err
	}

	return decoded, nil
}

func toSliceMap(value any) ([]map[string]any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	decoded := []map[string]any{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return nil, err
	}

	return decoded, nil
}
