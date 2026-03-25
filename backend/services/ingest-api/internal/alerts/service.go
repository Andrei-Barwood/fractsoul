package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
)

type ServiceConfig struct {
	Enabled           bool
	SuppressionWindow time.Duration
	NotifyTimeout     time.Duration
	NotifyRetries     int
	RetryBackoff      time.Duration
	QueueSize         int
	WorkerCount       int
}

type Service struct {
	logger     *slog.Logger
	repo       Repository
	engine     *Engine
	dispatcher *Dispatcher
	cfg        ServiceConfig
}

func NewService(
	logger *slog.Logger,
	repo Repository,
	cfg ServiceConfig,
	rules []Rule,
	notifiers []Notifier,
) (*Service, error) {
	if repo == nil {
		return nil, fmt.Errorf("alerts repository is required")
	}
	if logger == nil {
		logger = slog.Default()
	}

	cfg = withServiceDefaults(cfg)
	service := &Service{
		logger: logger,
		repo:   repo,
		engine: NewEngine(rules),
		cfg:    cfg,
	}

	if len(notifiers) > 0 {
		dispatcher := NewDispatcher(logger, repo, DispatchConfig{
			Timeout:      cfg.NotifyTimeout,
			MaxRetries:   cfg.NotifyRetries,
			RetryBackoff: cfg.RetryBackoff,
			QueueSize:    cfg.QueueSize,
			WorkerCount:  cfg.WorkerCount,
		}, notifiers)
		dispatcher.Start()
		service.dispatcher = dispatcher
	}

	return service, nil
}

func withServiceDefaults(cfg ServiceConfig) ServiceConfig {
	if cfg.SuppressionWindow <= 0 {
		cfg.SuppressionWindow = 10 * time.Minute
	}
	if cfg.NotifyTimeout <= 0 {
		cfg.NotifyTimeout = 3 * time.Second
	}
	if cfg.NotifyRetries <= 0 {
		cfg.NotifyRetries = 3
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
	return cfg
}

func (s *Service) ProcessEvent(ctx context.Context, event telemetry.IngestRequest) error {
	if s == nil || !s.cfg.Enabled {
		return nil
	}

	candidates := s.engine.Evaluate(event)
	if len(candidates) == 0 {
		return nil
	}

	var firstErr error
	for _, candidate := range candidates {
		input := PersistInput{
			EvaluatedAlert:    candidate,
			Fingerprint:       buildFingerprint(candidate.RuleID, candidate.MinerID),
			DedupeKey:         buildDedupeKey(candidate.RuleID, candidate.MinerID),
			SuppressionWindow: s.cfg.SuppressionWindow,
		}

		result, err := s.repo.UpsertAlert(ctx, input)
		if err != nil {
			s.logger.Error(
				"failed to upsert alert",
				"rule_id", candidate.RuleID,
				"miner_id", candidate.MinerID,
				"event_id", candidate.EventID,
				"error", err,
			)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		s.logger.Warn(
			"alert evaluated",
			"alert_id", result.Alert.AlertID,
			"rule_id", result.Alert.RuleID,
			"severity", result.Alert.Severity,
			"status", result.Alert.Status,
			"should_notify", result.ShouldNotify,
			"suppressed", result.Suppressed,
			"miner_id", result.Alert.MinerID,
			"event_id", result.Alert.EventID,
		)

		if result.ShouldNotify && s.dispatcher != nil {
			if err := s.dispatcher.Enqueue(result.Alert); err != nil {
				s.logger.Error(
					"failed to enqueue alert notification",
					"alert_id", result.Alert.AlertID,
					"rule_id", result.Alert.RuleID,
					"error", err,
				)
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}

	return firstErr
}

func buildFingerprint(ruleID, minerID string) string {
	return strings.ToLower(strings.TrimSpace(ruleID) + "|" + strings.TrimSpace(minerID))
}

func buildDedupeKey(ruleID, minerID string) string {
	return buildFingerprint(ruleID, minerID)
}

func (s *Service) Close() {
	if s == nil || s.dispatcher == nil {
		return
	}
	s.dispatcher.Close()
}
