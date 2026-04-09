package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/fractsoul"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/observability"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/orchestrator"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type EnergyHandler struct {
	logger         *slog.Logger
	service        *service.Service
	runtimeOptions RuntimeOptions
}

type budgetResponse struct {
	RequestID        string                        `json:"request_id"`
	SnapshotID       string                        `json:"snapshot_id,omitempty"`
	Budget           orchestrator.SiteBudget       `json:"budget"`
	FractsoulContext *fractsoul.ContextEnrichment `json:"fractsoul_context,omitempty"`
}

type validateDispatchRequest struct {
	RequestedBy    string                         `json:"requested_by" binding:"required"`
	AmbientCelsius *float64                       `json:"ambient_celsius,omitempty"`
	Requests       []orchestrator.DispatchRequest `json:"requests" binding:"required,min=1,dive"`
}

type validateDispatchResponse struct {
	RequestID        string                                `json:"request_id"`
	SnapshotID       string                                `json:"snapshot_id,omitempty"`
	Result           orchestrator.DispatchValidationResult `json:"result"`
	Budget           orchestrator.SiteBudget               `json:"budget"`
	FractsoulContext *fractsoul.ContextEnrichment          `json:"fractsoul_context,omitempty"`
}

func NewEnergyHandler(logger *slog.Logger, appService *service.Service, options RuntimeOptions) *EnergyHandler {
	return &EnergyHandler{
		logger:         logger,
		service:        appService,
		runtimeOptions: options,
	}
}

func (h *EnergyHandler) SiteBudget(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		WriteError(c, http.StatusBadRequest, "validation_error", "site_id is required", nil)
		return
	}

	at, err := parseAt(c.Query("at"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid at timestamp: %v", err), nil)
		return
	}

	includeContext, err := parseOptionalBool(c.Query("include_context"), true)
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}
	contextRackLimit, err := parsePositiveInt(c.Query("context_rack_limit"), h.runtimeOptions.ContextRackLimit, 20, "context_rack_limit")
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}
	contextWindowMinutes, err := parsePositiveInt(c.Query("context_window_minutes"), h.runtimeOptions.ContextWindowMinutes, 24*60, "context_window_minutes")
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}
	var ambientOverride *float64
	if rawAmbient := strings.TrimSpace(c.Query("ambient_celsius")); rawAmbient != "" {
		value, err := parseFloat64(rawAmbient)
		if err != nil {
			WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid ambient_celsius: %v", err), nil)
			return
		}
		ambientOverride = &value
	}

	result, err := h.service.ComputeBudget(c.Request.Context(), siteID, at, service.ComputeBudgetOptions{
		RequestID:       RequestID(c),
		Source:          "budget_endpoint",
		AmbientOverride: ambientOverride,
		IncludeContext:  includeContext,
		ContextOptions: fractsoul.ContextOptions{
			WindowMinutes: contextWindowMinutes,
			RackLimit:     contextRackLimit,
			RequestID:     RequestID(c),
		},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			observability.RecordBudgetCalculation("not_found")
			WriteError(c, http.StatusNotFound, "not_found", "site_id is not configured in energy inventory", nil)
			return
		}
		h.logger.Error("failed to compute budget", "site_id", siteID, "error", err)
		observability.RecordBudgetCalculation("error")
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to compute budget", nil)
		return
	}

	observability.RecordBudgetCalculation("ok")
	c.JSON(http.StatusOK, budgetResponse{
		RequestID:        RequestID(c),
		SnapshotID:       result.SnapshotID,
		Budget:           result.Budget,
		FractsoulContext: result.FractsoulContext,
	})
}

func (h *EnergyHandler) ValidateDispatch(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		WriteError(c, http.StatusBadRequest, "validation_error", "site_id is required", nil)
		return
	}

	at, err := parseAt(c.Query("at"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid at timestamp: %v", err), nil)
		return
	}

	var request validateDispatchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid request body: %v", err), ValidationDetails(err))
		return
	}

	includeContext, err := parseOptionalBool(c.Query("include_context"), true)
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}
	contextRackLimit, err := parsePositiveInt(c.Query("context_rack_limit"), h.runtimeOptions.ContextRackLimit, 20, "context_rack_limit")
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}
	contextWindowMinutes, err := parsePositiveInt(c.Query("context_window_minutes"), h.runtimeOptions.ContextWindowMinutes, 24*60, "context_window_minutes")
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	result, err := h.service.ValidateDispatch(c.Request.Context(), siteID, at, service.ValidateDispatchOptions{
		RequestID:       RequestID(c),
		Source:          "dispatch_validate",
		AmbientOverride: request.AmbientCelsius,
		IncludeContext:  includeContext,
		ContextOptions: fractsoul.ContextOptions{
			WindowMinutes: contextWindowMinutes,
			RackLimit:     contextRackLimit,
			RequestID:     RequestID(c),
		},
		RequestedBy: request.RequestedBy,
		Requests:    request.Requests,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			observability.RecordDispatchValidation("not_found")
			WriteError(c, http.StatusNotFound, "not_found", "site_id is not configured in energy inventory", nil)
			return
		}
		h.logger.Error("failed to validate dispatch", "site_id", siteID, "error", err)
		observability.RecordDispatchValidation("error")
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to validate dispatch", nil)
		return
	}

	statusLabel := strings.ToLower(result.Result.SummaryStatus)
	if statusLabel == "" {
		statusLabel = "accepted"
	}
	observability.RecordDispatchValidation(statusLabel)

	c.JSON(http.StatusOK, validateDispatchResponse{
		RequestID:        RequestID(c),
		SnapshotID:       result.SnapshotID,
		Result:           result.Result,
		Budget:           result.Budget,
		FractsoulContext: result.FractsoulContext,
	})
}

func parseAt(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Now().UTC(), nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func parseFloat64(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	return strconv.ParseFloat(value, 64)
}

func parseOptionalBool(raw string, fallback bool) (bool, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return fallback, nil
	}

	switch value {
	case "1", "true", "yes", "y", "on":
		return true, nil
	case "0", "false", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("value must be a valid boolean")
	}
}

func parsePositiveInt(raw string, fallback int, max int, field string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer", field)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be > 0", field)
	}
	if max > 0 && parsed > max {
		return 0, fmt.Errorf("%s must be <= %d", field, max)
	}
	return parsed, nil
}
