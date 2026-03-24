package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/storage"
	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
	"github.com/nats-io/nats.go"
)

type Config struct {
	Subject    string
	StreamName string
	Durable    string
	DLQSubject string
	MaxDeliver int
	RetryDelay time.Duration
}

type Consumer struct {
	logger *slog.Logger
	repo   storage.Repository
	conn   *nats.Conn
	js     nats.JetStreamContext
	cfg    Config
	sub    *nats.Subscription
}

type DLQMessage struct {
	Subject       string            `json:"subject"`
	EventID       string            `json:"event_id,omitempty"`
	SiteID        string            `json:"site_id,omitempty"`
	RackID        string            `json:"rack_id,omitempty"`
	MinerID       string            `json:"miner_id,omitempty"`
	FailureReason string            `json:"failure_reason"`
	NumDelivered  uint64            `json:"num_delivered"`
	FailedAt      time.Time         `json:"failed_at"`
	Headers       map[string]string `json:"headers,omitempty"`
	Payload       json.RawMessage   `json:"payload"`
}

func NewConsumer(logger *slog.Logger, repo storage.Repository, natsURL string, cfg Config) (*Consumer, error) {
	if cfg.Subject == "" {
		return nil, fmt.Errorf("consumer subject is required")
	}
	if cfg.StreamName == "" {
		return nil, fmt.Errorf("consumer stream name is required")
	}
	if cfg.Durable == "" {
		return nil, fmt.Errorf("consumer durable name is required")
	}
	if cfg.DLQSubject == "" {
		return nil, fmt.Errorf("consumer dlq subject is required")
	}
	if cfg.MaxDeliver <= 0 {
		cfg.MaxDeliver = 5
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 2 * time.Second
	}

	conn, err := nats.Connect(natsURL, nats.Name("fractsoul-telemetry-processor"))
	if err != nil {
		return nil, fmt.Errorf("connect nats %s: %w", natsURL, err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("init nats jetstream context: %w", err)
	}

	if err := telemetry.EnsureStreamSubjects(js, cfg.StreamName, []string{cfg.Subject, cfg.DLQSubject}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ensure stream %s for consumer: %w", cfg.StreamName, err)
	}

	return &Consumer{
		logger: logger,
		repo:   repo,
		conn:   conn,
		js:     js,
		cfg:    cfg,
	}, nil
}

func (c *Consumer) Start() error {
	ackWait := c.cfg.RetryDelay * 2
	if ackWait < 5*time.Second {
		ackWait = 5 * time.Second
	}

	sub, err := c.js.QueueSubscribe(
		c.cfg.Subject,
		"telemetry-processor",
		func(msg *nats.Msg) {
			c.handleMessage(msg)
		},
		nats.BindStream(c.cfg.StreamName),
		nats.Durable(c.cfg.Durable),
		nats.ManualAck(),
		nats.DeliverNew(),
		nats.AckWait(ackWait),
		nats.MaxDeliver(c.cfg.MaxDeliver),
	)
	if err != nil {
		return fmt.Errorf("subscribe to %s via jetstream: %w", c.cfg.Subject, err)
	}

	if err := c.conn.FlushTimeout(5 * time.Second); err != nil {
		sub.Unsubscribe()
		return fmt.Errorf("flush nats subscription: %w", err)
	}

	if err := c.conn.LastError(); err != nil {
		sub.Unsubscribe()
		return fmt.Errorf("nats subscription health check: %w", err)
	}

	c.sub = sub
	c.logger.Info(
		"telemetry processor subscribed",
		"stream", c.cfg.StreamName,
		"subject", c.cfg.Subject,
		"durable", c.cfg.Durable,
		"max_deliver", c.cfg.MaxDeliver,
		"retry_delay", c.cfg.RetryDelay.String(),
		"dlq_subject", c.cfg.DLQSubject,
	)

	return nil
}

func (c *Consumer) handleMessage(msg *nats.Msg) {
	numDelivered := uint64(1)
	if metadata, err := msg.Metadata(); err == nil && metadata != nil {
		numDelivered = metadata.NumDelivered
	}

	event, err := decodeAndNormalizeEvent(msg.Data)
	if err != nil {
		c.handleTerminalFailure(msg, nil, numDelivered, fmt.Errorf("decode/normalize event: %w", err))
		return
	}

	persistCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.repo.PersistTelemetry(persistCtx, event, msg.Data); err != nil {
		if int(numDelivered) >= c.cfg.MaxDeliver {
			c.handleTerminalFailure(msg, &event, numDelivered, fmt.Errorf("persist telemetry: %w", err))
			return
		}

		c.logger.Warn(
			"telemetry persistence failed; scheduling retry",
			"event_id", event.EventID,
			"site_id", event.SiteID,
			"rack_id", event.RackID,
			"miner_id", event.MinerID,
			"num_delivered", numDelivered,
			"max_deliver", c.cfg.MaxDeliver,
			"retry_delay", c.cfg.RetryDelay.String(),
			"error", err,
		)
		if nakErr := msg.NakWithDelay(c.cfg.RetryDelay); nakErr != nil {
			c.logger.Error("failed to nak message for retry", "error", nakErr)
		}
		return
	}

	if err := msg.Ack(); err != nil {
		c.logger.Error("failed to ack processed telemetry message", "error", err)
	}
}

func (c *Consumer) handleTerminalFailure(msg *nats.Msg, event *telemetry.IngestRequest, numDelivered uint64, reason error) {
	if err := c.publishDLQ(msg, event, numDelivered, reason); err != nil {
		c.logger.Error("failed to publish message to dlq", "error", err)
	}

	if err := msg.Ack(); err != nil {
		c.logger.Error("failed to ack terminal failed message", "error", err)
	}

	attrs := []any{
		"subject", c.cfg.Subject,
		"dlq_subject", c.cfg.DLQSubject,
		"num_delivered", numDelivered,
		"max_deliver", c.cfg.MaxDeliver,
		"error", reason,
	}
	if event != nil {
		attrs = append(attrs,
			"event_id", event.EventID,
			"site_id", event.SiteID,
			"rack_id", event.RackID,
			"miner_id", event.MinerID,
		)
	}

	c.logger.Error("telemetry message moved to dlq", attrs...)
}

func (c *Consumer) publishDLQ(msg *nats.Msg, event *telemetry.IngestRequest, numDelivered uint64, reason error) error {
	headers := map[string]string{}
	for key, value := range msg.Header {
		if len(value) == 0 {
			continue
		}
		headers[key] = value[0]
	}

	dlq := DLQMessage{
		Subject:       msg.Subject,
		FailureReason: reason.Error(),
		NumDelivered:  numDelivered,
		FailedAt:      time.Now().UTC(),
		Headers:       headers,
		Payload:       append([]byte(nil), msg.Data...),
	}
	if event != nil {
		dlq.EventID = event.EventID
		dlq.SiteID = event.SiteID
		dlq.RackID = event.RackID
		dlq.MinerID = event.MinerID
	}

	encoded, err := json.Marshal(dlq)
	if err != nil {
		return fmt.Errorf("encode dlq message: %w", err)
	}

	if _, err := c.js.Publish(c.cfg.DLQSubject, encoded); err != nil {
		return fmt.Errorf("publish dlq message to %s: %w", c.cfg.DLQSubject, err)
	}

	if err := c.conn.FlushTimeout(3 * time.Second); err != nil {
		return fmt.Errorf("flush dlq publish: %w", err)
	}

	return nil
}

func decodeAndNormalizeEvent(payload []byte) (telemetry.IngestRequest, error) {
	var event telemetry.IngestRequest
	if err := json.Unmarshal(payload, &event); err != nil {
		return telemetry.IngestRequest{}, err
	}

	siteID, rackID, minerID, err := telemetry.NormalizeOperationalIDs(event.SiteID, event.RackID, event.MinerID)
	if err != nil {
		return telemetry.IngestRequest{}, err
	}

	event.SiteID = siteID
	event.RackID = rackID
	event.MinerID = minerID

	return event, nil
}

func (c *Consumer) Close() error {
	if c == nil {
		return nil
	}

	if c.sub != nil {
		if err := c.sub.Unsubscribe(); err != nil && err != nats.ErrConnectionClosed {
			return fmt.Errorf("unsubscribe nats: %w", err)
		}
	}

	if c.conn != nil {
		if err := c.conn.Drain(); err != nil && err != nats.ErrConnectionClosed {
			return fmt.Errorf("drain nats connection: %w", err)
		}
		c.conn.Close()
	}

	return nil
}
