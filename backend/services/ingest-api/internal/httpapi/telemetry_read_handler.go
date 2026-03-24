package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/storage"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
	"github.com/gin-gonic/gin"
)

type TelemetryReadHandler struct {
	logger     *slog.Logger
	repository storage.Repository
}

func NewTelemetryReadHandler(logger *slog.Logger, repository storage.Repository) *TelemetryReadHandler {
	return &TelemetryReadHandler{
		logger:     logger,
		repository: repository,
	}
}

func (h *TelemetryReadHandler) Readings(c *gin.Context) {
	if h.repository == nil {
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "read api unavailable", nil)
		return
	}

	filter, err := buildReadingsFilter(c)
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	items, err := h.repository.ListReadings(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to query telemetry readings", "error", err)
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to query telemetry readings", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id": RequestID(c),
		"count":      len(items),
		"items":      items,
	})
}

func (h *TelemetryReadHandler) Summary(c *gin.Context) {
	if h.repository == nil {
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "read api unavailable", nil)
		return
	}

	filter, err := buildSummaryFilter(c)
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	summary, err := h.repository.SummarizeReadings(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to query telemetry summary", "error", err)
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to query telemetry summary", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id": RequestID(c),
		"summary":    summary,
	})
}

func buildReadingsFilter(c *gin.Context) (storage.ReadingsFilter, error) {
	siteID, rackID, minerID, err := normalizeFilterIDs(
		c.Query("site_id"),
		c.Query("rack_id"),
		c.Query("miner_id"),
	)
	if err != nil {
		return storage.ReadingsFilter{}, err
	}

	limit := 50
	if value := c.Query("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return storage.ReadingsFilter{}, fmt.Errorf("limit must be a positive integer")
		}
		if parsed > 500 {
			parsed = 500
		}
		limit = parsed
	}

	from, err := parseOptionalTime(c.Query("from"))
	if err != nil {
		return storage.ReadingsFilter{}, fmt.Errorf("invalid from timestamp: %w", err)
	}

	to, err := parseOptionalTime(c.Query("to"))
	if err != nil {
		return storage.ReadingsFilter{}, fmt.Errorf("invalid to timestamp: %w", err)
	}

	if from != nil && to != nil && from.After(*to) {
		return storage.ReadingsFilter{}, fmt.Errorf("from must be before to")
	}

	return storage.ReadingsFilter{
		SiteID:  siteID,
		RackID:  rackID,
		MinerID: minerID,
		From:    from,
		To:      to,
		Limit:   limit,
	}, nil
}

func buildSummaryFilter(c *gin.Context) (storage.SummaryFilter, error) {
	siteID, rackID, minerID, err := normalizeFilterIDs(
		c.Query("site_id"),
		c.Query("rack_id"),
		c.Query("miner_id"),
	)
	if err != nil {
		return storage.SummaryFilter{}, err
	}

	windowMinutes := 60
	if value := c.Query("window_minutes"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return storage.SummaryFilter{}, fmt.Errorf("window_minutes must be a positive integer")
		}
		if parsed > 24*60 {
			parsed = 24 * 60
		}
		windowMinutes = parsed
	}

	return storage.SummaryFilter{
		SiteID:        siteID,
		RackID:        rackID,
		MinerID:       minerID,
		WindowMinutes: windowMinutes,
	}, nil
}

func normalizeFilterIDs(siteID, rackID, minerID string) (string, string, string, error) {
	normalizedSite := strings.TrimSpace(siteID)
	normalizedRack := strings.TrimSpace(rackID)
	normalizedMiner := strings.TrimSpace(minerID)

	if normalizedSite != "" {
		site, err := telemetry.NormalizeSiteID(normalizedSite)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid site_id: %w", err)
		}
		normalizedSite = site
	}

	if normalizedRack != "" {
		if normalizedSite != "" {
			rack, err := telemetry.NormalizeRackID(normalizedSite, normalizedRack)
			if err != nil {
				return "", "", "", fmt.Errorf("invalid rack_id: %w", err)
			}
			normalizedRack = rack
		} else {
			normalizedRack = normalizeLooseID(normalizedRack)
		}
	}

	if normalizedMiner != "" {
		miner, err := telemetry.NormalizeMinerID(normalizedMiner)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid miner_id: %w", err)
		}
		normalizedMiner = miner
	}

	return normalizedSite, normalizedRack, normalizedMiner, nil
}

func parseOptionalTime(raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}

	utc := parsed.UTC()
	return &utc, nil
}

func normalizeLooseID(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	value = strings.ReplaceAll(value, "_", "-")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return strings.Trim(value, "-")
}
