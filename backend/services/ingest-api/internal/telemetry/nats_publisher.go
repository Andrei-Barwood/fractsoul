package telemetry

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
)

type NATSPublisher struct {
	conn *nats.Conn
}

func NewNATSPublisher(url string) (*NATSPublisher, error) {
	conn, err := nats.Connect(url, nats.Name("fractsoul-ingest-api"))
	if err != nil {
		return nil, fmt.Errorf("connect nats %s: %w", url, err)
	}

	return &NATSPublisher{conn: conn}, nil
}

func (p *NATSPublisher) Publish(ctx context.Context, subject string, payload []byte, headers map[string]string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	msg := nats.NewMsg(subject)
	msg.Data = payload
	for key, value := range headers {
		msg.Header.Set(key, value)
	}

	if err := p.conn.PublishMsg(msg); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}

	if err := p.conn.Flush(); err != nil {
		return fmt.Errorf("flush nats connection: %w", err)
	}

	if err := p.conn.LastError(); err != nil {
		return fmt.Errorf("flush nats connection: %w", err)
	}

	return nil
}

func (p *NATSPublisher) Close() error {
	if p == nil || p.conn == nil {
		return nil
	}

	if err := p.conn.Drain(); err != nil && !errors.Is(err, nats.ErrConnectionClosed) {
		return fmt.Errorf("drain nats connection: %w", err)
	}
	p.conn.Close()

	return nil
}
