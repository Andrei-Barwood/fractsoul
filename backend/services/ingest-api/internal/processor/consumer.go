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

type Consumer struct {
	logger  *slog.Logger
	repo    storage.Repository
	conn    *nats.Conn
	subject string
	sub     *nats.Subscription
}

func NewConsumer(logger *slog.Logger, repo storage.Repository, natsURL, subject string) (*Consumer, error) {
	conn, err := nats.Connect(natsURL, nats.Name("fractsoul-telemetry-processor"))
	if err != nil {
		return nil, fmt.Errorf("connect nats %s: %w", natsURL, err)
	}

	return &Consumer{
		logger:  logger,
		repo:    repo,
		conn:    conn,
		subject: subject,
	}, nil
}

func (c *Consumer) Start() error {
	sub, err := c.conn.QueueSubscribe(c.subject, "telemetry-processor", func(msg *nats.Msg) {
		c.handleMessage(msg)
	})
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", c.subject, err)
	}

	if err := c.conn.Flush(); err != nil {
		sub.Unsubscribe()
		return fmt.Errorf("flush nats subscription: %w", err)
	}

	if err := c.conn.LastError(); err != nil {
		sub.Unsubscribe()
		return fmt.Errorf("nats subscription health check: %w", err)
	}

	c.sub = sub
	c.logger.Info("telemetry processor subscribed", "subject", c.subject)

	return nil
}

func (c *Consumer) handleMessage(msg *nats.Msg) {
	var event telemetry.IngestRequest
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		c.logger.Error("failed to decode telemetry message", "error", err)
		return
	}

	siteID, rackID, minerID, err := telemetry.NormalizeOperationalIDs(event.SiteID, event.RackID, event.MinerID)
	if err != nil {
		c.logger.Error(
			"failed to normalize telemetry ids",
			"error", err,
			"site_id", event.SiteID,
			"rack_id", event.RackID,
			"miner_id", event.MinerID,
		)
		return
	}

	event.SiteID = siteID
	event.RackID = rackID
	event.MinerID = minerID

	persistCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.repo.PersistTelemetry(persistCtx, event, msg.Data); err != nil {
		c.logger.Error(
			"failed to persist telemetry event",
			"event_id", event.EventID,
			"site_id", event.SiteID,
			"rack_id", event.RackID,
			"miner_id", event.MinerID,
			"error", err,
		)
		return
	}
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
