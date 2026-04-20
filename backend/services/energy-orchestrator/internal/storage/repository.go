package storage

import (
	"context"
	"time"

	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/orchestrator"
)

type BudgetSnapshotCreateInput struct {
	SiteID              string
	Source              string
	PolicyMode          string
	CalculatedAt        time.Time
	TelemetryObservedAt *time.Time
	AmbientCelsius      float64
	NominalCapacityKW   float64
	EffectiveCapacityKW float64
	ReservedCapacityKW  float64
	SafeCapacityKW      float64
	CurrentLoadKW       float64
	AvailableCapacityKW float64
	SafeDispatchableKW  float64
	ConstraintFlags     []string
	SnapshotPayload     any
	UpstreamContext     any
}

type BudgetSnapshot struct {
	SnapshotID   string    `json:"snapshot_id"`
	SiteID       string    `json:"site_id"`
	Source       string    `json:"source"`
	CalculatedAt time.Time `json:"calculated_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type RecommendationReviewCreateInput struct {
	SiteID                   string
	SnapshotID               string
	RecommendationID         string
	RackID                   string
	Action                   string
	CriticalityClass         orchestrator.LoadCriticalityClass
	RequestedDeltaKW         float64
	RecommendedDeltaKW       float64
	Reason                   string
	Decision                 orchestrator.RecommendationDecision
	Status                   orchestrator.RecommendationReviewStatus
	Sensitivity              orchestrator.RecommendationReviewSensitivity
	RequiresDualConfirmation bool
	RequestedBy              string
	RequestedByRole          string
	FirstApprovedBy          string
	SecondApprovedBy         string
	RejectedBy               string
	PostponedBy              string
	PostponedUntil           *time.Time
	Comment                  string
	FinalDecisionAt          *time.Time
}

type RecommendationReviewUpdateInput struct {
	ReviewID         string
	Status           orchestrator.RecommendationReviewStatus
	Decision         orchestrator.RecommendationDecision
	FirstApprovedBy  string
	SecondApprovedBy string
	RejectedBy       string
	PostponedBy      string
	PostponedUntil   *time.Time
	Comment          string
	FinalDecisionAt  *time.Time
}

type RecommendationReviewEventCreateInput struct {
	ReviewID  string
	SiteID    string
	RackID    string
	ActorID   string
	ActorRole string
	EventType string
	Decision  orchestrator.RecommendationDecision
	Comment   string
}

type Repository interface {
	LoadBudgetInput(ctx context.Context, siteID string, at time.Time) (orchestrator.BudgetInput, error)
	LoadHistoricalReplayInput(ctx context.Context, siteID string, day time.Time) (orchestrator.HistoricalReplayInput, error)
	LoadRiskProjectionInput(ctx context.Context, siteID string, at time.Time) (orchestrator.RiskProjectionInput, error)
	ListSiteProfiles(ctx context.Context) ([]orchestrator.SiteProfile, error)
	GetRecommendationReview(ctx context.Context, siteID, recommendationID string) (orchestrator.RecommendationReview, error)
	ListRecommendationReviews(ctx context.Context, siteID, status string) ([]orchestrator.RecommendationReview, error)
	ListRecommendationReviewEvents(ctx context.Context, reviewID string) ([]orchestrator.RecommendationReviewEvent, error)
	CreateRecommendationReview(ctx context.Context, input RecommendationReviewCreateInput) (orchestrator.RecommendationReview, error)
	UpdateRecommendationReview(ctx context.Context, input RecommendationReviewUpdateInput) (orchestrator.RecommendationReview, error)
	AppendRecommendationReviewEvent(ctx context.Context, input RecommendationReviewEventCreateInput) (orchestrator.RecommendationReviewEvent, error)
	CreateBudgetSnapshot(ctx context.Context, input BudgetSnapshotCreateInput) (BudgetSnapshot, error)
	Close()
}
