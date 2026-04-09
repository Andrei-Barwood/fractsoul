package contracts

import "time"

const (
	SubjectLoadBudgetUpdated      = "energy.load_budget_updated.v1"
	SubjectCurtailmentRecommended = "energy.curtailment_recommended.v1"
	SubjectDispatchRejected       = "energy.dispatch_rejected.v1"
)

type LoadBudgetUpdatedEvent struct {
	SnapshotID           string    `json:"snapshot_id,omitempty"`
	EventID              string    `json:"event_id"`
	EventType            string    `json:"event_type"`
	OccurredAt           time.Time `json:"occurred_at"`
	SiteID               string    `json:"site_id"`
	PolicyMode           string    `json:"policy_mode"`
	NominalCapacityKW    float64   `json:"nominal_capacity_kw"`
	SafeCapacityKW       float64   `json:"safe_capacity_kw"`
	CurrentLoadKW        float64   `json:"current_load_kw"`
	AvailableCapacityKW float64   `json:"available_capacity_kw"`
	ConstraintFlags      []string  `json:"constraint_flags,omitempty"`
}

type CurtailmentRecommendedEvent struct {
	SnapshotID             string    `json:"snapshot_id,omitempty"`
	EventID                string    `json:"event_id"`
	EventType              string    `json:"event_type"`
	OccurredAt             time.Time `json:"occurred_at"`
	SiteID                 string    `json:"site_id"`
	PolicyMode             string    `json:"policy_mode"`
	RecommendedReductionKW float64   `json:"recommended_reduction_kw"`
	Reason                 string    `json:"reason"`
	AffectedRacks          []string  `json:"affected_racks,omitempty"`
}

type DispatchRejectedEvent struct {
	SnapshotID       string    `json:"snapshot_id,omitempty"`
	EventID          string    `json:"event_id"`
	EventType        string    `json:"event_type"`
	OccurredAt       time.Time `json:"occurred_at"`
	SiteID           string    `json:"site_id"`
	PolicyMode       string    `json:"policy_mode"`
	RequestedBy      string    `json:"requested_by"`
	RackID           string    `json:"rack_id"`
	RequestedDelta   float64   `json:"requested_delta_kw"`
	AcceptedDelta    float64   `json:"accepted_delta_kw"`
	Code             string    `json:"code"`
	Message          string    `json:"message"`
}
