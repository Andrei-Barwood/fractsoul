package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type WebhookConfig struct {
	URL        string
	AuthHeader string
	AuthToken  string
	Timeout    time.Duration
}

type WebhookNotifier struct {
	url        string
	authHeader string
	authToken  string
	client     *http.Client
}

func NewWebhookNotifier(cfg WebhookConfig) (*WebhookNotifier, error) {
	url := strings.TrimSpace(cfg.URL)
	if url == "" {
		return nil, fmt.Errorf("webhook url is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 3 * time.Second
	}
	authHeader := strings.TrimSpace(cfg.AuthHeader)
	if authHeader == "" {
		authHeader = "Authorization"
	}

	return &WebhookNotifier{
		url:        url,
		authHeader: authHeader,
		authToken:  strings.TrimSpace(cfg.AuthToken),
		client:     &http.Client{Timeout: cfg.Timeout},
	}, nil
}

func (n *WebhookNotifier) Channel() NotificationChannel {
	return ChannelWebhook
}

func (n *WebhookNotifier) Notify(ctx context.Context, alert PersistedAlert) (DeliveryResult, error) {
	payload := map[string]any{
		"alert_id":          alert.AlertID,
		"rule_id":           alert.RuleID,
		"rule_name":         alert.RuleName,
		"severity":          alert.Severity,
		"status":            alert.Status,
		"message":           alert.Message,
		"site_id":           alert.SiteID,
		"rack_id":           alert.RackID,
		"miner_id":          alert.MinerID,
		"event_id":          alert.EventID,
		"miner_model":       alert.MinerModel,
		"metric_name":       alert.MetricName,
		"metric_value":      alert.MetricValue,
		"threshold":         alert.Threshold,
		"occurrences":       alert.Occurrences,
		"first_seen_at":     alert.FirstSeenAt,
		"last_seen_at":      alert.LastSeenAt,
		"suppression_until": alert.SuppressionUntil,
		"details":           alert.Details,
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(encoded))
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if n.authToken != "" {
		req.Header.Set(n.authHeader, n.authToken)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("send webhook request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

	result := DeliveryResult{
		Destination:  n.url,
		ResponseCode: resp.StatusCode,
		Payload:      payload,
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("webhook responded with status %d body=%s", resp.StatusCode, string(responseBody))
	}

	return result, nil
}
