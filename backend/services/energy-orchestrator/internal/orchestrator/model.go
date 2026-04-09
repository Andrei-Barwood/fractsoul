package orchestrator

import "time"

type AssetStatus string

const (
	StatusActive      AssetStatus = "active"
	StatusDegraded    AssetStatus = "degraded"
	StatusMaintenance AssetStatus = "maintenance"
	StatusInactive    AssetStatus = "inactive"
)

type AssetKind string

const (
	AssetKindSite        AssetKind = "site"
	AssetKindTransformer AssetKind = "transformer"
	AssetKindBus         AssetKind = "bus"
	AssetKindFeeder      AssetKind = "feeder"
	AssetKindPDU         AssetKind = "pdu"
	AssetKindRack        AssetKind = "rack"
)

type SiteProfile struct {
	SiteID                  string  `json:"site_id"`
	CampusName              string  `json:"campus_name"`
	TargetCapacityMW        float64 `json:"target_capacity_mw"`
	OperatingReservePct     float64 `json:"operating_reserve_pct"`
	AmbientReferenceC       float64 `json:"ambient_reference_c"`
	AmbientDerateStartC     float64 `json:"ambient_derate_start_c"`
	AmbientDeratePctPerDeg  float64 `json:"ambient_derate_pct_per_deg"`
	AdvisoryMode            string  `json:"advisory_mode"`
}

type Substation struct {
	SubstationID     string      `json:"substation_id"`
	SiteID           string      `json:"site_id"`
	SubstationName   string      `json:"substation_name"`
	VoltageLevelKV   float64     `json:"voltage_level_kv"`
	RedundancyMode   string      `json:"redundancy_mode"`
	Status           AssetStatus `json:"status"`
	MaintenanceActive bool       `json:"maintenance_active"`
}

type CapacityAsset struct {
	ID                     string      `json:"id"`
	Kind                   AssetKind   `json:"kind"`
	SiteID                 string      `json:"site_id"`
	ParentID               string      `json:"parent_id,omitempty"`
	Name                   string      `json:"name"`
	NominalCapacityKW      float64     `json:"nominal_capacity_kw"`
	OperatingMarginPct     float64     `json:"operating_margin_pct"`
	AmbientDerateStartC    float64     `json:"ambient_derate_start_c"`
	AmbientDeratePctPerDeg float64     `json:"ambient_derate_pct_per_deg"`
	Status                 AssetStatus `json:"status"`
	MaintenanceActive      bool        `json:"maintenance_active"`
}

type RackProfile struct {
	RackID               string      `json:"rack_id"`
	SiteID               string      `json:"site_id"`
	BusID                string      `json:"bus_id,omitempty"`
	FeederID             string      `json:"feeder_id,omitempty"`
	PDUID                string      `json:"pdu_id,omitempty"`
	NominalCapacityKW    float64     `json:"nominal_capacity_kw"`
	OperatingMarginPct   float64     `json:"operating_margin_pct"`
	ThermalDensityLimitKW float64    `json:"thermal_density_limit_kw"`
	AisleZone            string      `json:"aisle_zone"`
	Status               AssetStatus `json:"status"`
	MaintenanceActive    bool        `json:"maintenance_active"`
}

type BudgetInput struct {
	At                 time.Time          `json:"at"`
	Site               SiteProfile        `json:"site"`
	AmbientCelsius     float64            `json:"ambient_celsius"`
	TelemetryObservedAt *time.Time        `json:"telemetry_observed_at,omitempty"`
	Substations        []Substation       `json:"substations,omitempty"`
	Transformers       []CapacityAsset    `json:"transformers,omitempty"`
	Buses              []CapacityAsset    `json:"buses,omitempty"`
	Feeders            []CapacityAsset    `json:"feeders,omitempty"`
	PDUs               []CapacityAsset    `json:"pdus,omitempty"`
	Racks              []RackProfile      `json:"racks,omitempty"`
	CurrentRackLoadKW  map[string]float64 `json:"current_rack_load_kw,omitempty"`
}

type AssetBudget struct {
	ID                  string      `json:"id"`
	Name                string      `json:"name"`
	Kind                AssetKind   `json:"kind"`
	CurrentLoadKW       float64     `json:"current_load_kw"`
	NominalCapacityKW   float64     `json:"nominal_capacity_kw"`
	EffectiveCapacityKW float64     `json:"effective_capacity_kw"`
	ReservedCapacityKW  float64     `json:"reserved_capacity_kw"`
	SafeCapacityKW      float64     `json:"safe_capacity_kw"`
	AvailableCapacityKW float64     `json:"available_capacity_kw"`
	Status              AssetStatus `json:"status"`
	MaintenanceActive   bool        `json:"maintenance_active"`
}

type RackBudget struct {
	RackID               string   `json:"rack_id"`
	BusID                string   `json:"bus_id,omitempty"`
	FeederID             string   `json:"feeder_id,omitempty"`
	PDUID                string   `json:"pdu_id,omitempty"`
	CurrentLoadKW        float64  `json:"current_load_kw"`
	NominalCapacityKW    float64  `json:"nominal_capacity_kw"`
	EffectiveCapacityKW  float64  `json:"effective_capacity_kw"`
	ReservedCapacityKW   float64  `json:"reserved_capacity_kw"`
	SafeCapacityKW       float64  `json:"safe_capacity_kw"`
	AvailableCapacityKW  float64  `json:"available_capacity_kw"`
	ThermalDensityLimitKW float64 `json:"thermal_density_limit_kw"`
	ThermalHeadroomKW    float64  `json:"thermal_headroom_kw"`
	SafeDispatchableKW   float64  `json:"safe_dispatchable_kw"`
	ConstraintFlags      []string `json:"constraint_flags,omitempty"`
}

type SiteBudget struct {
	SiteID               string        `json:"site_id"`
	PolicyMode           string        `json:"policy_mode"`
	CalculatedAt         time.Time     `json:"calculated_at"`
	TelemetryObservedAt  *time.Time    `json:"telemetry_observed_at,omitempty"`
	AmbientCelsius       float64       `json:"ambient_celsius"`
	NominalCapacityKW    float64       `json:"nominal_capacity_kw"`
	EffectiveCapacityKW  float64       `json:"effective_capacity_kw"`
	ReservedCapacityKW   float64       `json:"reserved_capacity_kw"`
	SafeCapacityKW       float64       `json:"safe_capacity_kw"`
	CurrentLoadKW        float64       `json:"current_load_kw"`
	AvailableCapacityKW  float64       `json:"available_capacity_kw"`
	SafeDispatchableKW   float64       `json:"safe_dispatchable_kw"`
	ConstraintFlags      []string      `json:"constraint_flags,omitempty"`
	Substations          []Substation  `json:"substations,omitempty"`
	Transformers         []AssetBudget `json:"transformers,omitempty"`
	Buses                []AssetBudget `json:"buses,omitempty"`
	Feeders              []AssetBudget `json:"feeders,omitempty"`
	PDUs                 []AssetBudget `json:"pdus,omitempty"`
	Racks                []RackBudget  `json:"racks,omitempty"`
}

type DispatchRequest struct {
	RackID string  `json:"rack_id" binding:"required"`
	DeltaKW float64 `json:"delta_kw" binding:"required"`
	Reason  string  `json:"reason,omitempty"`
}

type ConstraintViolation struct {
	Scope            string  `json:"scope"`
	ScopeID          string  `json:"scope_id"`
	Code             string  `json:"code"`
	Message          string  `json:"message"`
	LimitKW          float64 `json:"limit_kw"`
	CurrentLoadKW    float64 `json:"current_load_kw"`
	RequestedDeltaKW float64 `json:"requested_delta_kw"`
	AvailableKW      float64 `json:"available_kw"`
}

type DispatchDecision struct {
	RackID           string                `json:"rack_id"`
	RequestedDeltaKW float64               `json:"requested_delta_kw"`
	AcceptedDeltaKW  float64               `json:"accepted_delta_kw"`
	Status           string                `json:"status"`
	Violations       []ConstraintViolation `json:"violations,omitempty"`
}

type DispatchValidationResult struct {
	SiteID        string                `json:"site_id"`
	PolicyMode    string                `json:"policy_mode"`
	RequestedAt   time.Time             `json:"requested_at"`
	RequestedBy   string                `json:"requested_by"`
	SummaryStatus string                `json:"summary_status"`
	Decisions     []DispatchDecision    `json:"decisions"`
	Violations    []ConstraintViolation `json:"violations,omitempty"`
}
