package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/contracts"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/events"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/fractsoul"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/orchestrator"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	logger          *slog.Logger
	repository      storage.Repository
	publisher       events.Publisher
	fractsoulClient *fractsoul.Client
}

type ComputeBudgetOptions struct {
	RequestID       string
	Source          string
	AmbientOverride *float64
	IncludeContext  bool
	ContextOptions  fractsoul.ContextOptions
}

type BudgetResult struct {
	SnapshotID       string                       `json:"snapshot_id,omitempty"`
	Budget           orchestrator.SiteBudget      `json:"budget"`
	FractsoulContext *fractsoul.ContextEnrichment `json:"fractsoul_context,omitempty"`
}

type OperationalViewResult struct {
	SnapshotID       string                       `json:"snapshot_id,omitempty"`
	View             orchestrator.OperationalView `json:"view"`
	FractsoulContext *fractsoul.ContextEnrichment `json:"fractsoul_context,omitempty"`
}

type ValidateDispatchOptions struct {
	RequestID       string
	Source          string
	AmbientOverride *float64
	IncludeContext  bool
	ContextOptions  fractsoul.ContextOptions
	RequestedBy     string
	Requests        []orchestrator.DispatchRequest
}

type DispatchResult struct {
	SnapshotID       string                                `json:"snapshot_id,omitempty"`
	Result           orchestrator.DispatchValidationResult `json:"result"`
	Budget           orchestrator.SiteBudget               `json:"budget"`
	FractsoulContext *fractsoul.ContextEnrichment          `json:"fractsoul_context,omitempty"`
}

type ReplayHistoricalOptions struct {
	RequestID string
}

type ReplayHistoricalResult struct {
	Result orchestrator.HistoricalReplayResult `json:"result"`
}

type CampusOverviewOptions struct {
	RequestID      string
	AllowedSiteIDs []string
}

type CampusOverviewResult struct {
	Overview orchestrator.CampusOverview `json:"overview"`
}

type RecommendationReviewOptions struct {
	ActorID   string
	ActorRole string
	Request   orchestrator.RecommendationReviewRequest
}

type RecommendationReviewResult struct {
	Review orchestrator.RecommendationReview      `json:"review"`
	Event  orchestrator.RecommendationReviewEvent `json:"event"`
}

type RecommendationReviewListResult struct {
	Items []orchestrator.RecommendationReview `json:"items"`
}

type ShadowPilotOptions struct {
	RequestID string
}

type ShadowPilotResult struct {
	Result orchestrator.ShadowPilotResult `json:"result"`
}

func NewService(logger *slog.Logger, repository storage.Repository, publisher events.Publisher, fractsoulClient *fractsoul.Client) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if publisher == nil {
		publisher = events.NewNoopPublisher()
	}

	return &Service{
		logger:          logger,
		repository:      repository,
		publisher:       publisher,
		fractsoulClient: fractsoulClient,
	}
}

func (s *Service) ComputeBudget(ctx context.Context, siteID string, at time.Time, options ComputeBudgetOptions) (BudgetResult, error) {
	budget, fractsoulContext, err := s.computeBudget(ctx, siteID, at, options)
	if err != nil {
		return BudgetResult{}, err
	}

	snapshotID := s.persistBudgetSnapshot(ctx, budget, fractsoulContext, defaultSource(options.Source, "budget_endpoint"))
	s.publishBudgetEvents(ctx, options.RequestID, snapshotID, budget)

	return BudgetResult{
		SnapshotID:       snapshotID,
		Budget:           budget,
		FractsoulContext: fractsoulContext,
	}, nil
}

func (s *Service) GetOperationalView(ctx context.Context, siteID string, at time.Time, options ComputeBudgetOptions) (OperationalViewResult, error) {
	budget, fractsoulContext, err := s.computeBudget(ctx, siteID, at, options)
	if err != nil {
		return OperationalViewResult{}, err
	}

	view := orchestrator.BuildOperationalView(budget)
	snapshotID := s.persistBudgetSnapshot(ctx, budget, fractsoulContext, defaultSource(options.Source, "budget_endpoint"))
	s.publishBudgetEvents(ctx, options.RequestID, snapshotID, budget)

	return OperationalViewResult{
		SnapshotID:       snapshotID,
		View:             view,
		FractsoulContext: fractsoulContext,
	}, nil
}

func (s *Service) ReplayHistoricalDay(ctx context.Context, siteID string, day time.Time, _ ReplayHistoricalOptions) (ReplayHistoricalResult, error) {
	input, err := s.repository.LoadHistoricalReplayInput(ctx, siteID, day)
	if err != nil {
		return ReplayHistoricalResult{}, err
	}

	return ReplayHistoricalResult{
		Result: orchestrator.RunHistoricalReplay(input),
	}, nil
}

func (s *Service) RunShadowPilot(ctx context.Context, siteID string, day time.Time, _ ShadowPilotOptions) (ShadowPilotResult, error) {
	input, err := s.repository.LoadHistoricalReplayInput(ctx, siteID, day)
	if err != nil {
		return ShadowPilotResult{}, err
	}

	return ShadowPilotResult{
		Result: orchestrator.RunShadowPilot(input),
	}, nil
}

func (s *Service) GetCampusOverview(ctx context.Context, at time.Time, options CampusOverviewOptions) (CampusOverviewResult, error) {
	sites, err := s.repository.ListSiteProfiles(ctx)
	if err != nil {
		return CampusOverviewResult{}, err
	}
	allowed := make(map[string]struct{}, len(options.AllowedSiteIDs))
	for _, siteID := range options.AllowedSiteIDs {
		if siteID == "*" || siteID == "" {
			allowed = nil
			break
		}
		allowed[siteID] = struct{}{}
	}

	siteOverviews := make([]orchestrator.SiteOverview, 0, len(sites))
	for _, site := range sites {
		if allowed != nil {
			if _, ok := allowed[site.SiteID]; !ok {
				continue
			}
		}
		budget, _, err := s.computeBudget(ctx, site.SiteID, at, ComputeBudgetOptions{
			RequestID:      options.RequestID,
			Source:         "campus_overview",
			IncludeContext: false,
		})
		if err != nil {
			return CampusOverviewResult{}, err
		}
		view := orchestrator.BuildOperationalView(budget)
		riskInput, err := s.repository.LoadRiskProjectionInput(ctx, site.SiteID, at)
		if err != nil {
			return CampusOverviewResult{}, err
		}
		projection := orchestrator.ProjectSiteRisk(riskInput)
		siteOverviews = append(siteOverviews, orchestrator.BuildSiteOverview(site, view, projection, riskInput.CurrentTariff))
	}

	return CampusOverviewResult{
		Overview: orchestrator.BuildCampusOverview(at, siteOverviews),
	}, nil
}

func (s *Service) ReviewRecommendation(ctx context.Context, siteID string, options RecommendationReviewOptions) (RecommendationReviewResult, error) {
	actorID := strings.TrimSpace(options.ActorID)
	if actorID == "" {
		actorID = "unknown"
	}
	actorRole := strings.TrimSpace(options.ActorRole)
	if actorRole == "" {
		actorRole = "viewer"
	}

	sensitivity, requiresDual := orchestrator.SensitiveReviewLevel(options.Request)
	existing, err := s.repository.GetRecommendationReview(ctx, siteID, options.Request.RecommendationID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return RecommendationReviewResult{}, err
	}

	if existing.ReviewID == "" {
		status := orchestrator.ReviewStatusApproved
		eventType := "approved"
		finalDecisionAt := timePtr(time.Now().UTC())
		firstApprovedBy := ""
		rejectedBy := ""
		postponedBy := ""

		switch options.Request.Decision {
		case orchestrator.DecisionApprove:
			if requiresDual {
				status = orchestrator.ReviewStatusPendingSecondApproval
				eventType = "awaiting_second_approval"
				finalDecisionAt = nil
				firstApprovedBy = actorID
			} else {
				firstApprovedBy = actorID
			}
		case orchestrator.DecisionReject:
			status = orchestrator.ReviewStatusRejected
			eventType = "rejected"
			rejectedBy = actorID
		case orchestrator.DecisionPostpone:
			status = orchestrator.ReviewStatusPostponed
			eventType = "postponed"
			postponedBy = actorID
		default:
			return RecommendationReviewResult{}, fmt.Errorf("unsupported recommendation decision: %s", options.Request.Decision)
		}

		review, err := s.repository.CreateRecommendationReview(ctx, storage.RecommendationReviewCreateInput{
			SiteID:                   siteID,
			SnapshotID:               options.Request.SnapshotID,
			RecommendationID:         options.Request.RecommendationID,
			RackID:                   options.Request.RackID,
			Action:                   options.Request.Action,
			CriticalityClass:         options.Request.CriticalityClass,
			RequestedDeltaKW:         options.Request.RequestedDeltaKW,
			RecommendedDeltaKW:       options.Request.RecommendedDeltaKW,
			Reason:                   options.Request.Reason,
			Decision:                 options.Request.Decision,
			Status:                   status,
			Sensitivity:              sensitivity,
			RequiresDualConfirmation: requiresDual,
			RequestedBy:              actorID,
			RequestedByRole:          actorRole,
			FirstApprovedBy:          firstApprovedBy,
			RejectedBy:               rejectedBy,
			PostponedBy:              postponedBy,
			PostponedUntil:           options.Request.PostponeUntil,
			Comment:                  options.Request.Comment,
			FinalDecisionAt:          finalDecisionAt,
		})
		if err != nil {
			return RecommendationReviewResult{}, err
		}
		event, err := s.repository.AppendRecommendationReviewEvent(ctx, storage.RecommendationReviewEventCreateInput{
			ReviewID:  review.ReviewID,
			SiteID:    siteID,
			RackID:    options.Request.RackID,
			ActorID:   actorID,
			ActorRole: actorRole,
			EventType: eventType,
			Decision:  options.Request.Decision,
			Comment:   options.Request.Comment,
		})
		if err != nil {
			return RecommendationReviewResult{}, err
		}
		review, err = s.decorateRecommendationReview(ctx, review)
		if err != nil {
			return RecommendationReviewResult{}, err
		}
		return RecommendationReviewResult{Review: review, Event: event}, nil
	}

	if existing.Status == orchestrator.ReviewStatusApproved || existing.Status == orchestrator.ReviewStatusRejected || existing.Status == orchestrator.ReviewStatusPostponed {
		return RecommendationReviewResult{}, fmt.Errorf("recommendation review already finalized")
	}

	eventType := "updated"
	finalDecisionAt := timePtr(time.Now().UTC())
	update := storage.RecommendationReviewUpdateInput{
		ReviewID:         existing.ReviewID,
		Status:           existing.Status,
		Decision:         options.Request.Decision,
		FirstApprovedBy:  existing.FirstApprovedBy,
		SecondApprovedBy: existing.SecondApprovedBy,
		RejectedBy:       existing.RejectedBy,
		PostponedBy:      existing.PostponedBy,
		PostponedUntil:   options.Request.PostponeUntil,
		Comment:          options.Request.Comment,
		FinalDecisionAt:  finalDecisionAt,
	}

	switch options.Request.Decision {
	case orchestrator.DecisionApprove:
		if existing.RequiresDualConfirmation {
			if existing.FirstApprovedBy == actorID {
				return RecommendationReviewResult{}, fmt.Errorf("second approval must come from a different actor")
			}
			update.Status = orchestrator.ReviewStatusApproved
			update.SecondApprovedBy = actorID
			eventType = "second_approved"
		} else {
			update.Status = orchestrator.ReviewStatusApproved
			update.FirstApprovedBy = actorID
			eventType = "approved"
		}
	case orchestrator.DecisionReject:
		update.Status = orchestrator.ReviewStatusRejected
		update.RejectedBy = actorID
		eventType = "rejected"
	case orchestrator.DecisionPostpone:
		update.Status = orchestrator.ReviewStatusPostponed
		update.PostponedBy = actorID
		eventType = "postponed"
	default:
		return RecommendationReviewResult{}, fmt.Errorf("unsupported recommendation decision: %s", options.Request.Decision)
	}

	review, err := s.repository.UpdateRecommendationReview(ctx, update)
	if err != nil {
		return RecommendationReviewResult{}, err
	}
	event, err := s.repository.AppendRecommendationReviewEvent(ctx, storage.RecommendationReviewEventCreateInput{
		ReviewID:  review.ReviewID,
		SiteID:    siteID,
		RackID:    review.RackID,
		ActorID:   actorID,
		ActorRole: actorRole,
		EventType: eventType,
		Decision:  options.Request.Decision,
		Comment:   options.Request.Comment,
	})
	if err != nil {
		return RecommendationReviewResult{}, err
	}
	review, err = s.decorateRecommendationReview(ctx, review)
	if err != nil {
		return RecommendationReviewResult{}, err
	}
	return RecommendationReviewResult{Review: review, Event: event}, nil
}

func (s *Service) ListRecommendationReviews(ctx context.Context, siteID, status string) (RecommendationReviewListResult, error) {
	items, err := s.repository.ListRecommendationReviews(ctx, siteID, status)
	if err != nil {
		return RecommendationReviewListResult{}, err
	}
	for idx := range items {
		items[idx], err = s.decorateRecommendationReview(ctx, items[idx])
		if err != nil {
			return RecommendationReviewListResult{}, err
		}
	}
	return RecommendationReviewListResult{Items: items}, nil
}

func (s *Service) decorateRecommendationReview(ctx context.Context, review orchestrator.RecommendationReview) (orchestrator.RecommendationReview, error) {
	if strings.TrimSpace(review.ReviewID) == "" {
		return review, nil
	}

	events, err := s.repository.ListRecommendationReviewEvents(ctx, review.ReviewID)
	if err != nil {
		return orchestrator.RecommendationReview{}, err
	}

	review.Events = events
	review.Summary = orchestrator.BuildReviewSummary(review)
	return review, nil
}

func (s *Service) computeBudget(ctx context.Context, siteID string, at time.Time, options ComputeBudgetOptions) (orchestrator.SiteBudget, *fractsoul.ContextEnrichment, error) {
	input, err := s.repository.LoadBudgetInput(ctx, siteID, at)
	if err != nil {
		return orchestrator.SiteBudget{}, nil, err
	}

	if options.AmbientOverride != nil {
		input.AmbientCelsius = *options.AmbientOverride
	}

	budget := orchestrator.ComputeSiteBudget(input)
	fractsoulContext := s.loadContext(ctx, siteID, budget, options)
	return budget, fractsoulContext, nil
}

func (s *Service) ValidateDispatch(ctx context.Context, siteID string, at time.Time, options ValidateDispatchOptions) (DispatchResult, error) {
	budget, fractsoulContext, err := s.computeBudget(ctx, siteID, at, ComputeBudgetOptions{
		RequestID:       options.RequestID,
		Source:          options.Source,
		AmbientOverride: options.AmbientOverride,
		IncludeContext:  options.IncludeContext,
		ContextOptions:  options.ContextOptions,
	})
	if err != nil {
		return DispatchResult{}, err
	}

	requestedBy := strings.TrimSpace(options.RequestedBy)
	if requestedBy == "" {
		requestedBy = "system"
	}
	result := orchestrator.ValidateDispatch(budget, options.Requests, requestedBy)
	result.RequestedAt = at.UTC()

	snapshotID := s.persistBudgetSnapshot(ctx, budget, fractsoulContext, defaultSource(options.Source, "dispatch_validate"))
	s.publishBudgetEvents(ctx, options.RequestID, snapshotID, budget)
	s.publishDispatchEvents(ctx, options.RequestID, snapshotID, budget.SiteID, requestedBy, result)

	return DispatchResult{
		SnapshotID:       snapshotID,
		Result:           result,
		Budget:           budget,
		FractsoulContext: fractsoulContext,
	}, nil
}

func (s *Service) loadContext(ctx context.Context, siteID string, budget orchestrator.SiteBudget, options ComputeBudgetOptions) *fractsoul.ContextEnrichment {
	if !options.IncludeContext || s.fractsoulClient == nil || !s.fractsoulClient.Enabled() {
		return nil
	}

	contextOptions := options.ContextOptions
	if contextOptions.RequestID == "" {
		contextOptions.RequestID = options.RequestID
	}

	return s.fractsoulClient.LoadContext(ctx, siteID, budget, contextOptions)
}

func (s *Service) persistBudgetSnapshot(ctx context.Context, budget orchestrator.SiteBudget, fractsoulContext *fractsoul.ContextEnrichment, source string) string {
	payload := map[string]any{
		"budget": budget,
	}
	if fractsoulContext != nil {
		payload["fractsoul_context"] = fractsoulContext
	}

	snapshot, err := s.repository.CreateBudgetSnapshot(ctx, storage.BudgetSnapshotCreateInput{
		SiteID:              budget.SiteID,
		Source:              source,
		PolicyMode:          budget.PolicyMode,
		CalculatedAt:        budget.CalculatedAt,
		TelemetryObservedAt: budget.TelemetryObservedAt,
		AmbientCelsius:      budget.AmbientCelsius,
		NominalCapacityKW:   budget.NominalCapacityKW,
		EffectiveCapacityKW: budget.EffectiveCapacityKW,
		ReservedCapacityKW:  budget.ReservedCapacityKW,
		SafeCapacityKW:      budget.SafeCapacityKW,
		CurrentLoadKW:       budget.CurrentLoadKW,
		AvailableCapacityKW: budget.AvailableCapacityKW,
		SafeDispatchableKW:  budget.SafeDispatchableKW,
		ConstraintFlags:     budget.ConstraintFlags,
		SnapshotPayload:     payload,
		UpstreamContext:     fractsoulContext,
	})
	if err != nil {
		s.logger.Warn("failed to persist budget snapshot", "site_id", budget.SiteID, "error", err)
		return ""
	}

	return snapshot.SnapshotID
}

func (s *Service) publishBudgetEvents(ctx context.Context, requestID, snapshotID string, budget orchestrator.SiteBudget) {
	loadBudgetUpdated := contracts.LoadBudgetUpdatedEvent{
		SnapshotID:          snapshotID,
		EventID:             uuid.NewString(),
		EventType:           "load_budget_updated",
		OccurredAt:          time.Now().UTC(),
		SiteID:              budget.SiteID,
		PolicyMode:          budget.PolicyMode,
		NominalCapacityKW:   budget.NominalCapacityKW,
		SafeCapacityKW:      budget.SafeCapacityKW,
		CurrentLoadKW:       budget.CurrentLoadKW,
		AvailableCapacityKW: budget.AvailableCapacityKW,
		ConstraintFlags:     budget.ConstraintFlags,
	}
	s.publish(ctx, contracts.SubjectLoadBudgetUpdated, requestID, snapshotID, loadBudgetUpdated)

	reductionKW := maxFloat(budget.CurrentLoadKW-budget.SafeCapacityKW, 0)
	if reductionKW <= 0 {
		return
	}

	affectedRacks := make([]string, 0)
	for _, rack := range budget.Racks {
		if rack.CurrentLoadKW > rack.SafeCapacityKW || rack.SafeDispatchableKW <= 0 {
			affectedRacks = append(affectedRacks, rack.RackID)
		}
	}

	curtailment := contracts.CurtailmentRecommendedEvent{
		SnapshotID:             snapshotID,
		EventID:                uuid.NewString(),
		EventType:              "curtailment_recommended",
		OccurredAt:             time.Now().UTC(),
		SiteID:                 budget.SiteID,
		PolicyMode:             budget.PolicyMode,
		RecommendedReductionKW: reductionKW,
		Reason:                 "current load exceeds safe site capacity after derating and reserve",
		AffectedRacks:          affectedRacks,
	}
	s.publish(ctx, contracts.SubjectCurtailmentRecommended, requestID, snapshotID, curtailment)
}

func (s *Service) publishDispatchEvents(ctx context.Context, requestID, snapshotID, siteID, requestedBy string, result orchestrator.DispatchValidationResult) {
	for _, decision := range result.Decisions {
		if decision.Status == "accepted" {
			continue
		}

		code := "dispatch_rejected"
		message := "dispatch validation rejected by internal constraints"
		if len(decision.Violations) > 0 {
			code = decision.Violations[0].Code
			message = decision.Violations[0].Message
		}

		event := contracts.DispatchRejectedEvent{
			SnapshotID:     snapshotID,
			EventID:        uuid.NewString(),
			EventType:      "dispatch_rejected",
			OccurredAt:     time.Now().UTC(),
			SiteID:         siteID,
			PolicyMode:     result.PolicyMode,
			RequestedBy:    requestedBy,
			RackID:         decision.RackID,
			RequestedDelta: decision.RequestedDeltaKW,
			AcceptedDelta:  decision.AcceptedDeltaKW,
			Code:           code,
			Message:        message,
		}
		s.publish(ctx, contracts.SubjectDispatchRejected, requestID, snapshotID, event)
	}
}

func (s *Service) publish(ctx context.Context, subject, requestID, snapshotID string, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		s.logger.Warn("failed to encode event payload", "subject", subject, "error", err)
		return
	}

	headers := map[string]string{
		"X-Event-Subject": subject,
	}
	if requestID != "" {
		headers["X-Request-ID"] = requestID
	}
	if snapshotID != "" {
		headers["X-Snapshot-ID"] = snapshotID
	}

	if err := s.publisher.Publish(ctx, subject, encoded, headers); err != nil {
		s.logger.Warn("failed to publish event", "subject", subject, "error", err)
	}
}

func defaultSource(value, fallback string) string {
	source := strings.TrimSpace(value)
	if source == "" {
		return fallback
	}
	return source
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func timePtr(value time.Time) *time.Time {
	return &value
}
