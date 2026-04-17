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
	SiteID                 string  `json:"site_id"`
	CampusName             string  `json:"campus_name"`
	TargetCapacityMW       float64 `json:"target_capacity_mw"`
	OperatingReservePct    float64 `json:"operating_reserve_pct"`
	AmbientReferenceC      float64 `json:"ambient_reference_c"`
	AmbientDerateStartC    float64 `json:"ambient_derate_start_c"`
	AmbientDeratePctPerDeg float64 `json:"ambient_derate_pct_per_deg"`
	RampUpKWPerInterval    float64 `json:"ramp_up_kw_per_interval"`
	RampDownKWPerInterval  float64 `json:"ramp_down_kw_per_interval"`
	RampIntervalSeconds    int     `json:"ramp_interval_seconds"`
	AdvisoryMode           string  `json:"advisory_mode"`
}

type LoadCriticalityClass string

const (
	LoadClassNormalProduction    LoadCriticalityClass = "normal_production"
	LoadClassPreferredProduction LoadCriticalityClass = "preferred_production"
	LoadClassSacrificableLoad    LoadCriticalityClass = "sacrificable_load"
	LoadClassSafetyBlocked       LoadCriticalityClass = "safety_blocked"
)

type RampPolicy struct {
	IntervalSeconds       int     `json:"interval_seconds"`
	RampUpKWPerInterval   float64 `json:"ramp_up_kw_per_interval"`
	RampDownKWPerInterval float64 `json:"ramp_down_kw_per_interval"`
}

type Substation struct {
	SubstationID      string      `json:"substation_id"`
	SiteID            string      `json:"site_id"`
	SubstationName    string      `json:"substation_name"`
	VoltageLevelKV    float64     `json:"voltage_level_kv"`
	RedundancyMode    string      `json:"redundancy_mode"`
	Status            AssetStatus `json:"status"`
	MaintenanceActive bool        `json:"maintenance_active"`
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
	RackID                string               `json:"rack_id"`
	SiteID                string               `json:"site_id"`
	BusID                 string               `json:"bus_id,omitempty"`
	FeederID              string               `json:"feeder_id,omitempty"`
	PDUID                 string               `json:"pdu_id,omitempty"`
	NominalCapacityKW     float64              `json:"nominal_capacity_kw"`
	OperatingMarginPct    float64              `json:"operating_margin_pct"`
	ThermalDensityLimitKW float64              `json:"thermal_density_limit_kw"`
	AisleZone             string               `json:"aisle_zone"`
	CriticalityClass      LoadCriticalityClass `json:"criticality_class"`
	CriticalityReason     string               `json:"criticality_reason"`
	SafetyLocked          bool                 `json:"safety_locked"`
	SafetyLockReason      string               `json:"safety_lock_reason,omitempty"`
	RampUpKWPerInterval   float64              `json:"ramp_up_kw_per_interval"`
	RampDownKWPerInterval float64              `json:"ramp_down_kw_per_interval"`
	Status                AssetStatus          `json:"status"`
	MaintenanceActive     bool                 `json:"maintenance_active"`
}

type BudgetInput struct {
	At                  time.Time          `json:"at"`
	Site                SiteProfile        `json:"site"`
	AmbientCelsius      float64            `json:"ambient_celsius"`
	TelemetryObservedAt *time.Time         `json:"telemetry_observed_at,omitempty"`
	Substations         []Substation       `json:"substations,omitempty"`
	Transformers        []CapacityAsset    `json:"transformers,omitempty"`
	Buses               []CapacityAsset    `json:"buses,omitempty"`
	Feeders             []CapacityAsset    `json:"feeders,omitempty"`
	PDUs                []CapacityAsset    `json:"pdus,omitempty"`
	Racks               []RackProfile      `json:"racks,omitempty"`
	CurrentRackLoadKW   map[string]float64 `json:"current_rack_load_kw,omitempty"`
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
	RackID                string               `json:"rack_id"`
	BusID                 string               `json:"bus_id,omitempty"`
	FeederID              string               `json:"feeder_id,omitempty"`
	PDUID                 string               `json:"pdu_id,omitempty"`
	CriticalityClass      LoadCriticalityClass `json:"criticality_class"`
	CriticalityRank       int                  `json:"criticality_rank"`
	CriticalityReason     string               `json:"criticality_reason,omitempty"`
	SafetyBlocked         bool                 `json:"safety_blocked"`
	SafetyBlockReason     string               `json:"safety_block_reason,omitempty"`
	CurrentLoadKW         float64              `json:"current_load_kw"`
	NominalCapacityKW     float64              `json:"nominal_capacity_kw"`
	EffectiveCapacityKW   float64              `json:"effective_capacity_kw"`
	ReservedCapacityKW    float64              `json:"reserved_capacity_kw"`
	SafeCapacityKW        float64              `json:"safe_capacity_kw"`
	AvailableCapacityKW   float64              `json:"available_capacity_kw"`
	ThermalDensityLimitKW float64              `json:"thermal_density_limit_kw"`
	ThermalHeadroomKW     float64              `json:"thermal_headroom_kw"`
	RampUpLimitKW         float64              `json:"ramp_up_limit_kw"`
	RampDownLimitKW       float64              `json:"ramp_down_limit_kw"`
	UpRampRemainingKW     float64              `json:"up_ramp_remaining_kw"`
	DownRampRemainingKW   float64              `json:"down_ramp_remaining_kw"`
	SafeDispatchableKW    float64              `json:"safe_dispatchable_kw"`
	ConstraintFlags       []string             `json:"constraint_flags,omitempty"`
}

type SiteBudget struct {
	SiteID              string        `json:"site_id"`
	PolicyMode          string        `json:"policy_mode"`
	CalculatedAt        time.Time     `json:"calculated_at"`
	TelemetryObservedAt *time.Time    `json:"telemetry_observed_at,omitempty"`
	AmbientCelsius      float64       `json:"ambient_celsius"`
	NominalCapacityKW   float64       `json:"nominal_capacity_kw"`
	EffectiveCapacityKW float64       `json:"effective_capacity_kw"`
	ReservedCapacityKW  float64       `json:"reserved_capacity_kw"`
	SafeCapacityKW      float64       `json:"safe_capacity_kw"`
	CurrentLoadKW       float64       `json:"current_load_kw"`
	AvailableCapacityKW float64       `json:"available_capacity_kw"`
	SafeDispatchableKW  float64       `json:"safe_dispatchable_kw"`
	RampPolicy          RampPolicy    `json:"ramp_policy"`
	ConstraintFlags     []string      `json:"constraint_flags,omitempty"`
	Substations         []Substation  `json:"substations,omitempty"`
	Transformers        []AssetBudget `json:"transformers,omitempty"`
	Buses               []AssetBudget `json:"buses,omitempty"`
	Feeders             []AssetBudget `json:"feeders,omitempty"`
	PDUs                []AssetBudget `json:"pdus,omitempty"`
	Racks               []RackBudget  `json:"racks,omitempty"`
}

type DispatchRequest struct {
	RackID  string  `json:"rack_id" binding:"required"`
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
	Explanation      string                `json:"explanation,omitempty"`
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

type ActiveConstraint struct {
	Scope       string  `json:"scope"`
	ScopeID     string  `json:"scope_id"`
	Severity    string  `json:"severity"`
	Code        string  `json:"code"`
	Summary     string  `json:"summary"`
	Explanation string  `json:"explanation"`
	LimitKW     float64 `json:"limit_kw,omitempty"`
	CurrentKW   float64 `json:"current_kw,omitempty"`
	AvailableKW float64 `json:"available_kw,omitempty"`
}

type PendingRecommendation struct {
	RackID             string               `json:"rack_id"`
	Action             string               `json:"action"`
	CriticalityClass   LoadCriticalityClass `json:"criticality_class"`
	PriorityRank       int                  `json:"priority_rank"`
	RequestedDeltaKW   float64              `json:"requested_delta_kw"`
	RecommendedDeltaKW float64              `json:"recommended_delta_kw"`
	Reason             string               `json:"reason"`
	Explanation        string               `json:"explanation"`
	ConstraintCodes    []string             `json:"constraint_codes,omitempty"`
}

type BlockedAction struct {
	RackID           string               `json:"rack_id"`
	AttemptedAction  string               `json:"attempted_action"`
	CriticalityClass LoadCriticalityClass `json:"criticality_class"`
	Code             string               `json:"code"`
	Reason           string               `json:"reason"`
	Explanation      string               `json:"explanation"`
}

type DecisionExplanation struct {
	Scope       string `json:"scope"`
	ScopeID     string `json:"scope_id"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Explanation string `json:"explanation"`
}

type OperationalView struct {
	SiteID                 string                  `json:"site_id"`
	PolicyMode             string                  `json:"policy_mode"`
	CalculatedAt           time.Time               `json:"calculated_at"`
	Budget                 SiteBudget              `json:"budget"`
	ActiveConstraints      []ActiveConstraint      `json:"active_constraints,omitempty"`
	PendingRecommendations []PendingRecommendation `json:"pending_recommendations,omitempty"`
	BlockedActions         []BlockedAction         `json:"blocked_actions,omitempty"`
	Explanations           []DecisionExplanation   `json:"explanations,omitempty"`
}

type HistoricalRackPoint struct {
	Bucket                time.Time            `json:"bucket"`
	RackID                string               `json:"rack_id"`
	MinerModel            string               `json:"miner_model"`
	CriticalityClass      LoadCriticalityClass `json:"criticality_class"`
	CurrentLoadKW         float64              `json:"current_load_kw"`
	AvgHashrateTHs        float64              `json:"avg_hashrate_ths"`
	AvgPowerWatts         float64              `json:"avg_power_watts"`
	AvgTempCelsius        float64              `json:"avg_temp_celsius"`
	MaxTempCelsius        float64              `json:"max_temp_celsius"`
	AvgAmbientCelsius     float64              `json:"avg_ambient_celsius"`
	AvgEfficiencyJTH      float64              `json:"avg_efficiency_jth"`
	NominalHashrateTHs    float64              `json:"nominal_hashrate_ths"`
	NominalPowerWatts     float64              `json:"nominal_power_watts"`
	RampUpKWPerInterval   float64              `json:"ramp_up_kw_per_interval"`
	RampDownKWPerInterval float64              `json:"ramp_down_kw_per_interval"`
	SafetyLocked          bool                 `json:"safety_locked"`
}

type HistoricalReplayInput struct {
	Day                     time.Time             `json:"day"`
	Site                    SiteProfile           `json:"site"`
	ObservedPersistedAlerts int64                 `json:"observed_persisted_alerts"`
	Points                  []HistoricalRackPoint `json:"points"`
}

type ReplayScenarioMetrics struct {
	Name                string  `json:"name"`
	Description         string  `json:"description"`
	AvgJTH              float64 `json:"avg_jth"`
	PeakPowerKW         float64 `json:"peak_power_kw"`
	MaxTempCelsius      float64 `json:"max_temp_celsius"`
	EstimatedAlertCount int64   `json:"estimated_alert_count"`
	ObservedAlertRows   int64   `json:"observed_alert_rows,omitempty"`
	EnergyMWh           float64 `json:"energy_mwh"`
	AvgAmbientCelsius   float64 `json:"avg_ambient_celsius"`
	DeltaAvgJTHPct      float64 `json:"delta_avg_jth_pct"`
	DeltaPeakPowerPct   float64 `json:"delta_peak_power_pct"`
	DeltaMaxTempPct     float64 `json:"delta_max_temp_pct"`
	DeltaAlertCountPct  float64 `json:"delta_alert_count_pct"`
}

type HistoricalReplayResult struct {
	SiteID                  string                  `json:"site_id"`
	PolicyMode              string                  `json:"policy_mode"`
	Day                     time.Time               `json:"day"`
	RampPolicy              RampPolicy              `json:"ramp_policy"`
	ObservedPersistedAlerts int64                   `json:"observed_persisted_alerts"`
	Observed                ReplayScenarioMetrics   `json:"observed"`
	Scenarios               []ReplayScenarioMetrics `json:"scenarios"`
}
