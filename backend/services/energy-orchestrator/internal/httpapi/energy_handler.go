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
	RequestID        string                       `json:"request_id"`
	SnapshotID       string                       `json:"snapshot_id,omitempty"`
	Budget           orchestrator.SiteBudget      `json:"budget"`
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

type operationalViewResponse struct {
	RequestID        string                       `json:"request_id"`
	SnapshotID       string                       `json:"snapshot_id,omitempty"`
	View             orchestrator.OperationalView `json:"view"`
	FractsoulContext *fractsoul.ContextEnrichment `json:"fractsoul_context,omitempty"`
}

type activeConstraintsResponse struct {
	RequestID         string                          `json:"request_id"`
	SnapshotID        string                          `json:"snapshot_id,omitempty"`
	SiteID            string                          `json:"site_id"`
	CalculatedAt      time.Time                       `json:"calculated_at"`
	ActiveConstraints []orchestrator.ActiveConstraint `json:"active_constraints,omitempty"`
}

type pendingRecommendationsResponse struct {
	RequestID              string                               `json:"request_id"`
	SnapshotID             string                               `json:"snapshot_id,omitempty"`
	SiteID                 string                               `json:"site_id"`
	CalculatedAt           time.Time                            `json:"calculated_at"`
	PendingRecommendations []orchestrator.PendingRecommendation `json:"pending_recommendations,omitempty"`
}

type blockedActionsResponse struct {
	RequestID      string                       `json:"request_id"`
	SnapshotID     string                       `json:"snapshot_id,omitempty"`
	SiteID         string                       `json:"site_id"`
	CalculatedAt   time.Time                    `json:"calculated_at"`
	BlockedActions []orchestrator.BlockedAction `json:"blocked_actions,omitempty"`
}

type explanationsResponse struct {
	RequestID    string                             `json:"request_id"`
	SnapshotID   string                             `json:"snapshot_id,omitempty"`
	SiteID       string                             `json:"site_id"`
	CalculatedAt time.Time                          `json:"calculated_at"`
	Explanations []orchestrator.DecisionExplanation `json:"explanations,omitempty"`
}

type replayHistoricalResponse struct {
	RequestID string                              `json:"request_id"`
	Result    orchestrator.HistoricalReplayResult `json:"result"`
}

type campusOverviewResponse struct {
	RequestID string                      `json:"request_id"`
	Overview  orchestrator.CampusOverview `json:"overview"`
}

type shadowPilotResponse struct {
	RequestID string                         `json:"request_id"`
	Result    orchestrator.ShadowPilotResult `json:"result"`
}

type recommendationReviewRequest struct {
	SnapshotID         string                              `json:"snapshot_id" binding:"required"`
	RecommendationID   string                              `json:"recommendation_id" binding:"required"`
	RackID             string                              `json:"rack_id,omitempty"`
	Action             string                              `json:"action" binding:"required"`
	CriticalityClass   orchestrator.LoadCriticalityClass   `json:"criticality_class" binding:"required"`
	RequestedDeltaKW   float64                             `json:"requested_delta_kw"`
	RecommendedDeltaKW float64                             `json:"recommended_delta_kw"`
	Reason             string                              `json:"reason" binding:"required"`
	Decision           orchestrator.RecommendationDecision `json:"decision" binding:"required"`
	Comment            string                              `json:"comment,omitempty"`
	PostponeUntil      *time.Time                          `json:"postpone_until,omitempty"`
}

type recommendationReviewResponse struct {
	RequestID string                                 `json:"request_id"`
	Review    orchestrator.RecommendationReview      `json:"review"`
	Event     orchestrator.RecommendationReviewEvent `json:"event"`
}

type recommendationReviewListResponse struct {
	RequestID string                              `json:"request_id"`
	SiteID    string                              `json:"site_id"`
	Status    string                              `json:"status,omitempty"`
	Items     []orchestrator.RecommendationReview `json:"items"`
}

func NewEnergyHandler(logger *slog.Logger, appService *service.Service, options RuntimeOptions) *EnergyHandler {
	return &EnergyHandler{
		logger:         logger,
		service:        appService,
		runtimeOptions: options,
	}
}

func (h *EnergyHandler) CampusOverview(c *gin.Context) {
	at, err := parseAt(c.Query("at"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid at timestamp: %v", err), nil)
		return
	}

	result, err := h.service.GetCampusOverview(c.Request.Context(), at, service.CampusOverviewOptions{
		RequestID:      RequestID(c),
		AllowedSiteIDs: PrincipalSiteScopes(c),
	})
	if err != nil {
		h.logger.Error("failed to build campus overview", "error", err)
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to build campus overview", nil)
		return
	}

	c.JSON(http.StatusOK, campusOverviewResponse{
		RequestID: RequestID(c),
		Overview:  result.Overview,
	})
}

func (h *EnergyHandler) ShadowPilot(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		WriteError(c, http.StatusBadRequest, "validation_error", "site_id is required", nil)
		return
	}

	day, err := parseDay(c.Query("day"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid day value: %v", err), nil)
		return
	}

	result, err := h.service.RunShadowPilot(c.Request.Context(), siteID, day, service.ShadowPilotOptions{
		RequestID: RequestID(c),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(c, http.StatusNotFound, "not_found", "site_id is not configured in energy inventory or the pilot day has no data", nil)
			return
		}
		h.logger.Error("failed to run shadow pilot", "site_id", siteID, "day", day.Format(time.DateOnly), "error", err)
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to run shadow pilot", nil)
		return
	}

	c.JSON(http.StatusOK, shadowPilotResponse{
		RequestID: RequestID(c),
		Result:    result.Result,
	})
}

func (h *EnergyHandler) ListRecommendationReviews(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		WriteError(c, http.StatusBadRequest, "validation_error", "site_id is required", nil)
		return
	}

	status, err := parseReviewStatus(c.Query("status"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	result, err := h.service.ListRecommendationReviews(c.Request.Context(), siteID, status)
	if err != nil {
		h.logger.Error("failed to list recommendation reviews", "site_id", siteID, "status", status, "error", err)
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to list recommendation reviews", nil)
		return
	}

	c.JSON(http.StatusOK, recommendationReviewListResponse{
		RequestID: RequestID(c),
		SiteID:    siteID,
		Status:    status,
		Items:     result.Items,
	})
}

func (h *EnergyHandler) ReviewRecommendation(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		WriteError(c, http.StatusBadRequest, "validation_error", "site_id is required", nil)
		return
	}

	var request recommendationReviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid request body: %v", err), ValidationDetails(err))
		return
	}

	if !EnsureRackAccess(c, nonEmptyRackIDs(request.RackID)) {
		return
	}

	result, err := h.service.ReviewRecommendation(c.Request.Context(), siteID, service.RecommendationReviewOptions{
		ActorID:   PrincipalID(c),
		ActorRole: PrincipalRole(c),
		Request: orchestrator.RecommendationReviewRequest{
			SnapshotID:         request.SnapshotID,
			RecommendationID:   request.RecommendationID,
			RackID:             request.RackID,
			Action:             request.Action,
			CriticalityClass:   request.CriticalityClass,
			RequestedDeltaKW:   request.RequestedDeltaKW,
			RecommendedDeltaKW: request.RecommendedDeltaKW,
			Reason:             request.Reason,
			Decision:           request.Decision,
			Comment:            request.Comment,
			PostponeUntil:      request.PostponeUntil,
		},
	})
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			WriteError(c, http.StatusNotFound, "not_found", "recommendation review target was not found", nil)
			return
		case strings.Contains(err.Error(), "already finalized"):
			WriteError(c, http.StatusConflict, "review_finalized", err.Error(), nil)
			return
		case strings.Contains(err.Error(), "second approval must come from a different actor"):
			WriteError(c, http.StatusConflict, "dual_confirmation_violation", err.Error(), nil)
			return
		case strings.Contains(err.Error(), "unsupported recommendation decision"):
			WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
			return
		default:
			h.logger.Error("failed to review recommendation", "site_id", siteID, "recommendation_id", request.RecommendationID, "error", err)
			WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to review recommendation", nil)
			return
		}
	}

	c.JSON(http.StatusOK, recommendationReviewResponse{
		RequestID: RequestID(c),
		Review:    result.Review,
		Event:     result.Event,
	})
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
	if !EnsureRackAccess(c, dispatchRequestRackIDs(request.Requests)) {
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

func (h *EnergyHandler) SiteOperationalView(c *gin.Context) {
	siteID, at, options, ok := h.readComputeBudgetRequest(c)
	if !ok {
		return
	}

	result, err := h.service.GetOperationalView(c.Request.Context(), siteID, at, options)
	if err != nil {
		h.writeBudgetDependencyError(c, siteID, "failed to build operational view", err)
		return
	}

	observability.RecordBudgetCalculation("ok")
	c.JSON(http.StatusOK, operationalViewResponse{
		RequestID:        RequestID(c),
		SnapshotID:       result.SnapshotID,
		View:             result.View,
		FractsoulContext: result.FractsoulContext,
	})
}

func (h *EnergyHandler) ActiveConstraints(c *gin.Context) {
	siteID, at, options, ok := h.readComputeBudgetRequest(c)
	if !ok {
		return
	}

	result, err := h.service.GetOperationalView(c.Request.Context(), siteID, at, options)
	if err != nil {
		h.writeBudgetDependencyError(c, siteID, "failed to compute active constraints", err)
		return
	}

	observability.RecordBudgetCalculation("ok")
	c.JSON(http.StatusOK, activeConstraintsResponse{
		RequestID:         RequestID(c),
		SnapshotID:        result.SnapshotID,
		SiteID:            result.View.SiteID,
		CalculatedAt:      result.View.CalculatedAt,
		ActiveConstraints: result.View.ActiveConstraints,
	})
}

func (h *EnergyHandler) PendingRecommendations(c *gin.Context) {
	siteID, at, options, ok := h.readComputeBudgetRequest(c)
	if !ok {
		return
	}

	result, err := h.service.GetOperationalView(c.Request.Context(), siteID, at, options)
	if err != nil {
		h.writeBudgetDependencyError(c, siteID, "failed to compute pending recommendations", err)
		return
	}

	observability.RecordBudgetCalculation("ok")
	c.JSON(http.StatusOK, pendingRecommendationsResponse{
		RequestID:              RequestID(c),
		SnapshotID:             result.SnapshotID,
		SiteID:                 result.View.SiteID,
		CalculatedAt:           result.View.CalculatedAt,
		PendingRecommendations: result.View.PendingRecommendations,
	})
}

func (h *EnergyHandler) BlockedActions(c *gin.Context) {
	siteID, at, options, ok := h.readComputeBudgetRequest(c)
	if !ok {
		return
	}

	result, err := h.service.GetOperationalView(c.Request.Context(), siteID, at, options)
	if err != nil {
		h.writeBudgetDependencyError(c, siteID, "failed to compute blocked actions", err)
		return
	}

	observability.RecordBudgetCalculation("ok")
	c.JSON(http.StatusOK, blockedActionsResponse{
		RequestID:      RequestID(c),
		SnapshotID:     result.SnapshotID,
		SiteID:         result.View.SiteID,
		CalculatedAt:   result.View.CalculatedAt,
		BlockedActions: result.View.BlockedActions,
	})
}

func (h *EnergyHandler) DecisionExplanations(c *gin.Context) {
	siteID, at, options, ok := h.readComputeBudgetRequest(c)
	if !ok {
		return
	}

	result, err := h.service.GetOperationalView(c.Request.Context(), siteID, at, options)
	if err != nil {
		h.writeBudgetDependencyError(c, siteID, "failed to compute decision explanations", err)
		return
	}

	observability.RecordBudgetCalculation("ok")
	c.JSON(http.StatusOK, explanationsResponse{
		RequestID:    RequestID(c),
		SnapshotID:   result.SnapshotID,
		SiteID:       result.View.SiteID,
		CalculatedAt: result.View.CalculatedAt,
		Explanations: result.View.Explanations,
	})
}

func (h *EnergyHandler) HistoricalReplay(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		WriteError(c, http.StatusBadRequest, "validation_error", "site_id is required", nil)
		return
	}

	day, err := parseDay(c.Query("day"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid day value: %v", err), nil)
		return
	}

	result, err := h.service.ReplayHistoricalDay(c.Request.Context(), siteID, day, service.ReplayHistoricalOptions{
		RequestID: RequestID(c),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			WriteError(c, http.StatusNotFound, "not_found", "site_id is not configured in energy inventory or the replay day has no data", nil)
			return
		}
		h.logger.Error("failed to compute historical replay", "site_id", siteID, "day", day.Format(time.DateOnly), "error", err)
		WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", "failed to compute historical replay", nil)
		return
	}

	c.JSON(http.StatusOK, replayHistoricalResponse{
		RequestID: RequestID(c),
		Result:    result.Result,
	})
}

func (h *EnergyHandler) readComputeBudgetRequest(c *gin.Context) (string, time.Time, service.ComputeBudgetOptions, bool) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		WriteError(c, http.StatusBadRequest, "validation_error", "site_id is required", nil)
		return "", time.Time{}, service.ComputeBudgetOptions{}, false
	}

	at, err := parseAt(c.Query("at"))
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid at timestamp: %v", err), nil)
		return "", time.Time{}, service.ComputeBudgetOptions{}, false
	}

	includeContext, err := parseOptionalBool(c.Query("include_context"), true)
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return "", time.Time{}, service.ComputeBudgetOptions{}, false
	}
	contextRackLimit, err := parsePositiveInt(c.Query("context_rack_limit"), h.runtimeOptions.ContextRackLimit, 20, "context_rack_limit")
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return "", time.Time{}, service.ComputeBudgetOptions{}, false
	}
	contextWindowMinutes, err := parsePositiveInt(c.Query("context_window_minutes"), h.runtimeOptions.ContextWindowMinutes, 24*60, "context_window_minutes")
	if err != nil {
		WriteError(c, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return "", time.Time{}, service.ComputeBudgetOptions{}, false
	}
	var ambientOverride *float64
	if rawAmbient := strings.TrimSpace(c.Query("ambient_celsius")); rawAmbient != "" {
		value, err := parseFloat64(rawAmbient)
		if err != nil {
			WriteError(c, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid ambient_celsius: %v", err), nil)
			return "", time.Time{}, service.ComputeBudgetOptions{}, false
		}
		ambientOverride = &value
	}

	return siteID, at, service.ComputeBudgetOptions{
		RequestID:       RequestID(c),
		Source:          "budget_endpoint",
		AmbientOverride: ambientOverride,
		IncludeContext:  includeContext,
		ContextOptions: fractsoul.ContextOptions{
			WindowMinutes: contextWindowMinutes,
			RackLimit:     contextRackLimit,
			RequestID:     RequestID(c),
		},
	}, true
}

func (h *EnergyHandler) writeBudgetDependencyError(c *gin.Context, siteID, message string, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		observability.RecordBudgetCalculation("not_found")
		WriteError(c, http.StatusNotFound, "not_found", "site_id is not configured in energy inventory", nil)
		return
	}
	h.logger.Error(message, "site_id", siteID, "error", err)
	observability.RecordBudgetCalculation("error")
	WriteError(c, http.StatusServiceUnavailable, "dependency_unavailable", message, nil)
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

func parseDay(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, fmt.Errorf("day must be provided in YYYY-MM-DD format")
	}

	parsed, err := time.Parse(time.DateOnly, value)
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

func parseReviewStatus(raw string) (string, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return "", nil
	}

	switch value {
	case string(orchestrator.ReviewStatusPendingSecondApproval),
		string(orchestrator.ReviewStatusApproved),
		string(orchestrator.ReviewStatusRejected),
		string(orchestrator.ReviewStatusPostponed):
		return value, nil
	default:
		return "", fmt.Errorf("status must be one of pending_second_approval, approved, rejected or postponed")
	}
}

func dispatchRequestRackIDs(requests []orchestrator.DispatchRequest) []string {
	rackIDs := make([]string, 0, len(requests))
	for _, request := range requests {
		rackIDs = append(rackIDs, request.RackID)
	}
	return rackIDs
}

func nonEmptyRackIDs(values ...string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		normalizedValue := strings.TrimSpace(value)
		if normalizedValue == "" {
			continue
		}
		items = append(items, normalizedValue)
	}
	return items
}
