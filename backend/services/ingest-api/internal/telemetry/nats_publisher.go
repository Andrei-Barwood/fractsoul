package telemetry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

type NATSPublisher struct {
	conn       *nats.Conn
	js         nats.JetStreamContext
	streamName string
}

func NewNATSPublisher(url, streamName string, subjects []string) (*NATSPublisher, error) {
	conn, err := nats.Connect(url, nats.Name("fractsoul-ingest-api"))
	if err != nil {
		return nil, fmt.Errorf("connect nats %s: %w", url, err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("init nats jetstream context: %w", err)
	}

	if err := EnsureStreamSubjects(js, streamName, subjects); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ensure stream %s: %w", streamName, err)
	}

	return &NATSPublisher{
		conn:       conn,
		js:         js,
		streamName: streamName,
	}, nil
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

	if _, err := p.js.PublishMsg(msg, nats.Context(ctx)); err != nil {
		return fmt.Errorf("publish message to stream %s: %w", p.streamName, err)
	}

	if err := p.conn.FlushTimeout(3 * time.Second); err != nil {
		return fmt.Errorf("flush nats connection: %w", err)
	}

	if err := p.conn.LastError(); err != nil {
		return fmt.Errorf("flush nats connection: %w", err)
	}

	return nil
}

func EnsureStreamSubjects(js nats.JetStreamContext, streamName string, subjects []string) error {
	if streamName == "" {
		return fmt.Errorf("stream name is required")
	}

	normalizedSubjects := dedupeNonEmpty(subjects)
	if len(normalizedSubjects) == 0 {
		return fmt.Errorf("at least one stream subject is required")
	}

	config := &nats.StreamConfig{
		Name:      streamName,
		Subjects:  normalizedSubjects,
		Retention: nats.LimitsPolicy,
		Storage:   nats.FileStorage,
		Discard:   nats.DiscardOld,
		MaxAge:    24 * time.Hour,
	}

	if _, err := js.AddStream(config); err == nil {
		return nil
	}

	info, infoErr := js.StreamInfo(streamName)
	if infoErr != nil {
		return fmt.Errorf("lookup stream %s: %w", streamName, infoErr)
	}

	merged := mergeSubjects(info.Config.Subjects, normalizedSubjects)
	if sameSubjectSet(info.Config.Subjects, merged) {
		return nil
	}

	updated := info.Config
	updated.Subjects = merged
	if _, err := js.UpdateStream(&updated); err != nil {
		return fmt.Errorf("update stream %s subjects: %w", streamName, err)
	}

	return nil
}

func mergeSubjects(existing, incoming []string) []string {
	merged := make([]string, 0, len(existing)+len(incoming))
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	for _, subject := range append(existing, incoming...) {
		s := strings.TrimSpace(subject)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		merged = append(merged, s)
	}

	return merged
}

func sameSubjectSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	lookup := make(map[string]int, len(a))
	for _, item := range a {
		lookup[strings.TrimSpace(item)]++
	}
	for _, item := range b {
		key := strings.TrimSpace(item)
		lookup[key]--
		if lookup[key] < 0 {
			return false
		}
	}

	for _, count := range lookup {
		if count != 0 {
			return false
		}
	}

	return true
}

func dedupeNonEmpty(items []string) []string {
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
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
