package events

import "context"

type Publisher interface {
	Publish(ctx context.Context, subject string, payload []byte, headers map[string]string) error
	Close() error
}
