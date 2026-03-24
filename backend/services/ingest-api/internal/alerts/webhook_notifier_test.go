package alerts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWebhookNotifierSendsPayload(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("expected Authorization header, got %s", got)
		}

		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	notifier, err := NewWebhookNotifier(WebhookConfig{
		URL:        server.URL,
		AuthHeader: "Authorization",
		AuthToken:  "Bearer test-token",
		Timeout:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("build notifier: %v", err)
	}

	result, err := notifier.Notify(context.Background(), PersistedAlert{
		AlertID:          "alert-1",
		RuleID:           "overheat",
		RuleName:         "Sobretemperatura",
		Severity:         SeverityCritical,
		Status:           StatusOpen,
		Message:          "Temperatura alta",
		SiteID:           "site-cl-01",
		RackID:           "rack-cl-01-01",
		MinerID:          "asic-000001",
		EventID:          "550e8400-e29b-41d4-a716-446655440000",
		MinerModel:       "S21",
		MetricName:       "temp_celsius",
		MetricValue:      101.2,
		Threshold:        95,
		Occurrences:      1,
		FirstSeenAt:      time.Now().UTC(),
		LastSeenAt:       time.Now().UTC(),
		SuppressionUntil: time.Now().UTC().Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("notify webhook: %v", err)
	}

	if result.ResponseCode != http.StatusAccepted {
		t.Fatalf("expected response code %d, got %d", http.StatusAccepted, result.ResponseCode)
	}
	if received["alert_id"] != "alert-1" {
		t.Fatalf("expected alert_id alert-1, got %#v", received["alert_id"])
	}
	if received["rule_id"] != "overheat" {
		t.Fatalf("expected rule_id overheat, got %#v", received["rule_id"])
	}
}
