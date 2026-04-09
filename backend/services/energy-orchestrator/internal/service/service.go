package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/contracts"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/events"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/fractsoul"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/orchestrator"
	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/storage"
	"github.com/google/uuid"
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
	input, err := s.repository.LoadBudgetInput(ctx, siteID, at)
	if err != nil {
		return BudgetResult{}, err
	}

	if options.AmbientOverride != nil {
		input.AmbientCelsius = *options.AmbientOverride
	}

	budget := orchestrator.ComputeSiteBudget(input)
	fractsoulContext := s.loadContext(ctx, siteID, budget, options)
	snapshotID := s.persistBudgetSnapshot(ctx, budget, fractsoulContext, defaultSource(options.Source, "budget_endpoint"))
	s.publishBudgetEvents(ctx, options.RequestID, snapshotID, budget)

	return BudgetResult{
		SnapshotID:       snapshotID,
		Budget:           budget,
		FractsoulContext: fractsoulContext,
	}, nil
}

func (s *Service) ValidateDispatch(ctx context.Context, siteID string, at time.Time, options ValidateDispatchOptions) (DispatchResult, error) {
	input, err := s.repository.LoadBudgetInput(ctx, siteID, at)
	if err != nil {
		return DispatchResult{}, err
	}

	if options.AmbientOverride != nil {
		input.AmbientCelsius = *options.AmbientOverride
	}

	budget := orchestrator.ComputeSiteBudget(input)
	fractsoulContext := s.loadContext(ctx, siteID, budget, ComputeBudgetOptions{
		RequestID:       options.RequestID,
		Source:          options.Source,
		IncludeContext:  options.IncludeContext,
		ContextOptions:  options.ContextOptions,
		AmbientOverride: options.AmbientOverride,
	})

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
		SnapshotID:           snapshotID,
		EventID:              uuid.NewString(),
		EventType:            "load_budget_updated",
		OccurredAt:           time.Now().UTC(),
		SiteID:               budget.SiteID,
		PolicyMode:           budget.PolicyMode,
		NominalCapacityKW:    budget.NominalCapacityKW,
		SafeCapacityKW:       budget.SafeCapacityKW,
		CurrentLoadKW:        budget.CurrentLoadKW,
		AvailableCapacityKW:  budget.AvailableCapacityKW,
		ConstraintFlags:      budget.ConstraintFlags,
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
