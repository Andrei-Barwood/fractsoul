package telemetry

import "time"

type Status string

const (
	StatusOK       Status = "ok"
	StatusWarning  Status = "warning"
	StatusCritical Status = "critical"
	StatusOffline  Status = "offline"
)

type Metrics struct {
	HashrateTHs   float64 `json:"hashrate_ths" binding:"required,gte=0,lte=2000"`
	PowerWatts    float64 `json:"power_watts" binding:"required,gte=0,lte=10000"`
	TempCelsius   float64 `json:"temp_celsius" binding:"required,gte=-40,lte=130"`
	FanRPM        int     `json:"fan_rpm" binding:"required,gte=0,lte=30000"`
	EfficiencyJTH float64 `json:"efficiency_jth" binding:"required,gte=0,lte=1000"`
	Status        Status  `json:"status" binding:"required,oneof=ok warning critical offline"`
}

type IngestRequest struct {
	EventID         string            `json:"event_id" binding:"required,uuid4"`
	Timestamp       time.Time         `json:"timestamp" binding:"required"`
	SiteID          string            `json:"site_id" binding:"required,min=2,max=64"`
	RackID          string            `json:"rack_id" binding:"required,min=2,max=64"`
	MinerID         string            `json:"miner_id" binding:"required,min=2,max=64"`
	FirmwareVersion string            `json:"firmware_version,omitempty" binding:"omitempty,max=64"`
	Metrics         Metrics           `json:"metrics" binding:"required"`
	Tags            map[string]string `json:"tags,omitempty" binding:"omitempty"`
}

type IngestResponse struct {
	RequestID  string    `json:"request_id"`
	Accepted   bool      `json:"accepted"`
	EventID    string    `json:"event_id"`
	QueueTopic string    `json:"queue_topic"`
	IngestedAt time.Time `json:"ingested_at"`
}
