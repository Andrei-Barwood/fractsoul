package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/storage"
	"github.com/gin-gonic/gin"
)

type EfficiencyHandler struct {
	logger     *slog.Logger
	repository storage.Repository
}

func NewEfficiencyHandler(logger *slog.Logger, repository storage.Repository) *EfficiencyHandler {
	return &EfficiencyHandler{
		logger:     logger,
		repository: repository,
	}
}

func (h *EfficiencyHandler) MinerEfficiency(c *gin.Context) {
	if h.repository == nil {
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "read api unavailable", nil)
		return
	}

	filter, err := buildEfficiencyFilter(c)
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	items, err := h.repository.ListMinerEfficiency(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to query miner efficiency", "error", err)
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to query miner efficiency", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id": RequestID(c),
		"count":      len(items),
		"items":      items,
	})
}

func (h *EfficiencyHandler) RackEfficiency(c *gin.Context) {
	if h.repository == nil {
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "read api unavailable", nil)
		return
	}

	filter, err := buildEfficiencyFilter(c)
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	items, err := h.repository.ListRackEfficiency(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to query rack efficiency", "error", err)
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to query rack efficiency", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id": RequestID(c),
		"count":      len(items),
		"items":      items,
	})
}

func (h *EfficiencyHandler) SiteEfficiency(c *gin.Context) {
	if h.repository == nil {
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "read api unavailable", nil)
		return
	}

	filter, err := buildEfficiencyFilter(c)
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	items, err := h.repository.ListSiteEfficiency(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to query site efficiency", "error", err)
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to query site efficiency", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id": RequestID(c),
		"count":      len(items),
		"items":      items,
	})
}

func buildEfficiencyFilter(c *gin.Context) (storage.EfficiencyFilter, error) {
	siteID, rackID, minerID, err := normalizeFilterIDs(
		c.Query("site_id"),
		c.Query("rack_id"),
		c.Query("miner_id"),
	)
	if err != nil {
		return storage.EfficiencyFilter{}, err
	}

	windowMinutes, err := parsePositiveInt(c.Query("window_minutes"), 60, 24*60, "window_minutes")
	if err != nil {
		return storage.EfficiencyFilter{}, err
	}

	limit, err := parsePositiveInt(c.Query("limit"), 100, 1000, "limit")
	if err != nil {
		return storage.EfficiencyFilter{}, err
	}

	return storage.EfficiencyFilter{
		SiteID:        siteID,
		RackID:        rackID,
		MinerID:       minerID,
		Model:         parseOptionalModel(c.Query("model")),
		WindowMinutes: windowMinutes,
		Limit:         limit,
	}, nil
}
