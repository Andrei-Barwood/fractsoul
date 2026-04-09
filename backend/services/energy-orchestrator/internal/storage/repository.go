package storage

import (
	"context"
	"time"

	"github.com/fractsoul/mvp/backend/services/energy-orchestrator/internal/orchestrator"
)

type BudgetSnapshotCreateInput struct {
	SiteID               string
	Source               string
	PolicyMode           string
	CalculatedAt         time.Time
	TelemetryObservedAt  *time.Time
	AmbientCelsius       float64
	NominalCapacityKW    float64
	EffectiveCapacityKW  float64
	ReservedCapacityKW   float64
	SafeCapacityKW       float64
	CurrentLoadKW        float64
	AvailableCapacityKW  float64
	SafeDispatchableKW   float64
	ConstraintFlags      []string
	SnapshotPayload      any
	UpstreamContext      any
}

type BudgetSnapshot struct {
	SnapshotID   string    `json:"snapshot_id"`
	SiteID       string    `json:"site_id"`
	Source       string    `json:"source"`
	CalculatedAt time.Time `json:"calculated_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type Repository interface {
	LoadBudgetInput(ctx context.Context, siteID string, at time.Time) (orchestrator.BudgetInput, error)
	CreateBudgetSnapshot(ctx context.Context, input BudgetSnapshotCreateInput) (BudgetSnapshot, error)
	Close()
}
