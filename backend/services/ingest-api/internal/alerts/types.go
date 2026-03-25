package alerts

import (
	"context"
	"time"
)

type Severity string

const (
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Status string

const (
	StatusOpen       Status = "open"
	StatusSuppressed Status = "suppressed"
	StatusResolved   Status = "resolved"
)

type NotificationChannel string

const (
	ChannelWebhook NotificationChannel = "webhook"
	ChannelEmail   NotificationChannel = "email"
)

type NotificationStatus string

const (
	NotificationSent   NotificationStatus = "sent"
	NotificationFailed NotificationStatus = "failed"
)

type EvaluatedAlert struct {
	RuleID      string
	RuleName    string
	Severity    Severity
	Message     string
	MetricName  string
	MetricValue float64
	Threshold   float64
	ObservedAt  time.Time
	SiteID      string
	RackID      string
	MinerID     string
	EventID     string
	MinerModel  string
	Firmware    string
	Details     map[string]any
}

type PersistInput struct {
	EvaluatedAlert
	Fingerprint       string
	DedupeKey         string
	SuppressionWindow time.Duration
}

type PersistedAlert struct {
	AlertID          string
	RuleID           string
	RuleName         string
	Severity         Severity
	Status           Status
	Message          string
	Fingerprint      string
	DedupeKey        string
	SiteID           string
	RackID           string
	MinerID          string
	EventID          string
	MinerModel       string
	Firmware         string
	MetricName       string
	MetricValue      float64
	Threshold        float64
	FirstSeenAt      time.Time
	LastSeenAt       time.Time
	SuppressionUntil time.Time
	Occurrences      int
	NotifyCount      int
	LastNotifiedAt   *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Details          map[string]any
}

type UpsertResult struct {
	Alert        PersistedAlert
	ShouldNotify bool
	Suppressed   bool
	IsNew        bool
}

type NotificationRecord struct {
	AlertID      string
	Channel      NotificationChannel
	Destination  string
	Status       NotificationStatus
	Attempt      int
	ErrorMessage string
	ResponseCode int
	Payload      map[string]any
	NotifiedAt   time.Time
}

type DeliveryResult struct {
	Destination  string
	ResponseCode int
	Payload      map[string]any
}

type Repository interface {
	UpsertAlert(ctx context.Context, input PersistInput) (UpsertResult, error)
	RecordAlertNotification(ctx context.Context, record NotificationRecord) error
}

type Notifier interface {
	Channel() NotificationChannel
	Notify(ctx context.Context, alert PersistedAlert) (DeliveryResult, error)
}
