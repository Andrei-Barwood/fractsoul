package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/observability"
)

type DispatchConfig struct {
	Timeout      time.Duration
	MaxRetries   int
	RetryBackoff time.Duration
	QueueSize    int
	WorkerCount  int
}

type Dispatcher struct {
	logger    *slog.Logger
	repo      Repository
	notifiers []Notifier
	cfg       DispatchConfig

	jobs   chan PersistedAlert
	stopCh chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
}

func NewDispatcher(logger *slog.Logger, repo Repository, cfg DispatchConfig, notifiers []Notifier) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 3 * time.Second
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = 500 * time.Millisecond
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 256
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 2
	}

	cloned := make([]Notifier, 0, len(notifiers))
	for _, notifier := range notifiers {
		if notifier == nil {
			continue
		}
		cloned = append(cloned, notifier)
	}

	return &Dispatcher{
		logger:    logger,
		repo:      repo,
		notifiers: cloned,
		cfg:       cfg,
		jobs:      make(chan PersistedAlert, cfg.QueueSize),
		stopCh:    make(chan struct{}),
	}
}

func (d *Dispatcher) Start() {
	if d == nil || len(d.notifiers) == 0 {
		return
	}

	for i := 0; i < d.cfg.WorkerCount; i++ {
		d.wg.Add(1)
		go d.worker(i + 1)
	}
}

func (d *Dispatcher) Enqueue(alert PersistedAlert) error {
	if d == nil || len(d.notifiers) == 0 {
		return nil
	}

	select {
	case <-d.stopCh:
		return fmt.Errorf("dispatcher is closed")
	default:
	}

	select {
	case d.jobs <- alert:
		return nil
	default:
		return fmt.Errorf("notification queue is full")
	}
}

func (d *Dispatcher) worker(workerID int) {
	defer d.wg.Done()

	for {
		select {
		case <-d.stopCh:
			return
		case alert, ok := <-d.jobs:
			if !ok {
				return
			}
			d.dispatchAlert(workerID, alert)
		}
	}
}

func (d *Dispatcher) dispatchAlert(workerID int, alert PersistedAlert) {
	for _, notifier := range d.notifiers {
		delivered := false
		for attempt := 1; attempt <= d.cfg.MaxRetries; attempt++ {
			attemptStartedAt := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), d.cfg.Timeout)
			result, err := notifier.Notify(ctx, alert)
			cancel()
			attemptDuration := time.Since(attemptStartedAt)

			if err == nil {
				delivered = true
				observability.RecordAlertNotification(string(notifier.Channel()), "sent", attemptDuration)
				d.logger.Info(
					"alert notification sent",
					"worker", workerID,
					"alert_id", alert.AlertID,
					"rule_id", alert.RuleID,
					"channel", notifier.Channel(),
					"destination", result.Destination,
					"attempt", attempt,
				)
				d.recordNotification(NotificationRecord{
					AlertID:      alert.AlertID,
					Channel:      notifier.Channel(),
					Destination:  result.Destination,
					Status:       NotificationSent,
					Attempt:      attempt,
					ResponseCode: result.ResponseCode,
					Payload:      result.Payload,
					NotifiedAt:   time.Now().UTC(),
				})
				break
			}

			d.logger.Error(
				"alert notification failed",
				"worker", workerID,
				"alert_id", alert.AlertID,
				"rule_id", alert.RuleID,
				"channel", notifier.Channel(),
				"attempt", attempt,
				"error", err,
			)

			if attempt == d.cfg.MaxRetries {
				observability.RecordAlertNotification(string(notifier.Channel()), "failed", attemptDuration)
				d.recordNotification(NotificationRecord{
					AlertID:      alert.AlertID,
					Channel:      notifier.Channel(),
					Destination:  result.Destination,
					Status:       NotificationFailed,
					Attempt:      attempt,
					ErrorMessage: err.Error(),
					ResponseCode: result.ResponseCode,
					Payload:      result.Payload,
					NotifiedAt:   time.Now().UTC(),
				})
				break
			}

			select {
			case <-d.stopCh:
				return
			case <-time.After(backoffForAttempt(d.cfg.RetryBackoff, attempt)):
			}
		}

		if !delivered {
			continue
		}
	}
}

func (d *Dispatcher) recordNotification(record NotificationRecord) {
	if d.repo == nil {
		return
	}
	if err := d.repo.RecordAlertNotification(context.Background(), record); err != nil {
		observability.RecordAlertNotification(string(record.Channel), "persist_error", 0)
		d.logger.Error(
			"failed to persist alert notification",
			"alert_id", record.AlertID,
			"channel", record.Channel,
			"error", err,
		)
	}
}

func backoffForAttempt(base time.Duration, attempt int) time.Duration {
	if attempt <= 1 {
		return base
	}
	return time.Duration(attempt) * base
}

func (d *Dispatcher) Close() {
	if d == nil {
		return
	}

	d.once.Do(func() {
		close(d.stopCh)
		close(d.jobs)
		d.wg.Wait()
	})
}
