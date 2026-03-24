package httpapi

import (
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

func NewAnomalyHandler(logger *slog.Logger, repository storage.Repository) *AnomalyHandler {
	return &AnomalyHandler{
		logger:     logger,
		repository: repository,
	}
}

func (h *AnomalyHandler) AnalyzeMiner(c *gin.Context) {
	if h.repository == nil {
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "read api unavailable", nil)
		return
	}

	minerID, err := telemetry.NormalizeMinerID(c.Param("miner_id"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid miner_id: %v", err), nil)
		return
	}

	resolution, err := parseResolution(c.Query("resolution"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	from, err := parseOptionalTime(c.Query("from"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid from timestamp: %v", err), nil)
		return
	}
	to, err := parseOptionalTime(c.Query("to"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid to timestamp: %v", err), nil)
		return
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
		return
	}

	limit, err := parsePositiveInt(c.Query("limit"), defaultSeriesLimitByResolution(resolution), 10000, "limit")
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
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
		return
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
		return
	}
	if len(readings) == 0 || len(series) == 0 {
		WriteError(c, http.StatusNotFound, "not_found", "insufficient telemetry data for anomaly analysis", nil)
		return
	}

	ambientOverride, err := parseOptionalFloat(c.Query("ambient_c"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	report := anomaly.Analyze(readings[0], series, ambientOverride)
	c.JSON(http.StatusOK, gin.H{
		"request_id":   RequestID(c),
		"resolution":   resolution,
		"from":         resolvedFrom,
		"to":           resolvedTo,
		"summary_line": anomaly.SummaryLine(report),
		"report":       report,
	})
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
