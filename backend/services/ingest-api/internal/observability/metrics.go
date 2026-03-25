package observability

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry *prometheus.Registry

	httpInFlight        prometheus.Gauge
	httpRequestsTotal   *prometheus.CounterVec
	httpDurationSeconds *prometheus.HistogramVec

	ingestEventsTotal *prometheus.CounterVec
	ingestPayloadSize prometheus.Histogram

	processorEventsTotal   *prometheus.CounterVec
	processorDuration      *prometheus.HistogramVec
	alertEvaluationsTotal  *prometheus.CounterVec
	alertNotifications     *prometheus.CounterVec
	alertNotifyDurationSec *prometheus.HistogramVec
}

var (
	metricsOnce sync.Once
	metricsInst *Metrics
)

func MetricsHandler() http.Handler {
	metrics := loadMetrics()
	return promhttp.HandlerFor(metrics.registry, promhttp.HandlerOpts{})
}

func IncHTTPInFlight() {
	loadMetrics().httpInFlight.Inc()
}

func DecHTTPInFlight() {
	loadMetrics().httpInFlight.Dec()
}

func ObserveHTTPRequest(method, path string, status int, duration time.Duration) {
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	if normalizedMethod == "" {
		normalizedMethod = "UNKNOWN"
	}

	statusCode := strconv.Itoa(status)
	if statusCode == "" {
		statusCode = "0"
	}

	normalizedPath := normalizePath(path)
	seconds := duration.Seconds()
	if seconds < 0 {
		seconds = 0
	}

	metrics := loadMetrics()
	metrics.httpRequestsTotal.WithLabelValues(normalizedMethod, normalizedPath, statusCode).Inc()
	metrics.httpDurationSeconds.WithLabelValues(normalizedMethod, normalizedPath, statusCode).Observe(seconds)
}

func RecordIngestEvent(result, reason string, payloadBytes int) {
	metrics := loadMetrics()
	metrics.ingestEventsTotal.WithLabelValues(
		sanitizeLabel(result, "unknown"),
		sanitizeLabel(reason, "none"),
	).Inc()

	if payloadBytes > 0 {
		metrics.ingestPayloadSize.Observe(float64(payloadBytes))
	}
}

func RecordProcessorEvent(result string, duration time.Duration) {
	seconds := duration.Seconds()
	if seconds < 0 {
		seconds = 0
	}

	normalizedResult := sanitizeLabel(result, "unknown")
	metrics := loadMetrics()
	metrics.processorEventsTotal.WithLabelValues(normalizedResult).Inc()
	metrics.processorDuration.WithLabelValues(normalizedResult).Observe(seconds)
}

func RecordAlertEvaluation(ruleID, severity, status string, shouldNotify, suppressed bool) {
	metrics := loadMetrics()
	metrics.alertEvaluationsTotal.WithLabelValues(
		sanitizeLabel(ruleID, "unknown"),
		sanitizeLabel(severity, "unknown"),
		sanitizeLabel(status, "unknown"),
		boolLabel(shouldNotify),
		boolLabel(suppressed),
	).Inc()
}

func RecordAlertNotification(channel, status string, duration time.Duration) {
	seconds := duration.Seconds()
	if seconds < 0 {
		seconds = 0
	}

	normalizedChannel := sanitizeLabel(channel, "unknown")
	normalizedStatus := sanitizeLabel(status, "unknown")

	metrics := loadMetrics()
	metrics.alertNotifications.WithLabelValues(normalizedChannel, normalizedStatus).Inc()
	metrics.alertNotifyDurationSec.WithLabelValues(normalizedChannel, normalizedStatus).Observe(seconds)
}

func loadMetrics() *Metrics {
	metricsOnce.Do(func() {
		registry := prometheus.NewRegistry()
		registry.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		)

		metrics := &Metrics{
			registry: registry,
			httpInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: "fractsoul",
				Subsystem: "http",
				Name:      "inflight_requests",
				Help:      "Current number of in-flight HTTP requests.",
			}),
			httpRequestsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "fractsoul",
					Subsystem: "http",
					Name:      "requests_total",
					Help:      "Total HTTP requests handled by method, route path and status.",
				},
				[]string{"method", "path", "status"},
			),
			httpDurationSeconds: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: "fractsoul",
					Subsystem: "http",
					Name:      "request_duration_seconds",
					Help:      "HTTP request latency in seconds by method, route path and status.",
					Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
				},
				[]string{"method", "path", "status"},
			),
			ingestEventsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "fractsoul",
					Subsystem: "ingest",
					Name:      "events_total",
					Help:      "Telemetry ingest events by result and reason.",
				},
				[]string{"result", "reason"},
			),
			ingestPayloadSize: prometheus.NewHistogram(
				prometheus.HistogramOpts{
					Namespace: "fractsoul",
					Subsystem: "ingest",
					Name:      "payload_size_bytes",
					Help:      "Telemetry ingest payload size in bytes.",
					Buckets:   prometheus.ExponentialBuckets(128, 2, 8),
				},
			),
			processorEventsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "fractsoul",
					Subsystem: "processor",
					Name:      "events_total",
					Help:      "Telemetry processor events by result.",
				},
				[]string{"result"},
			),
			processorDuration: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: "fractsoul",
					Subsystem: "processor",
					Name:      "event_duration_seconds",
					Help:      "Telemetry processor handling time in seconds by result.",
					Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2},
				},
				[]string{"result"},
			),
			alertEvaluationsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "fractsoul",
					Subsystem: "alerts",
					Name:      "evaluations_total",
					Help:      "Alert evaluations by rule, severity, resulting status and notify/suppressed flags.",
				},
				[]string{"rule_id", "severity", "status", "notify", "suppressed"},
			),
			alertNotifications: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "fractsoul",
					Subsystem: "alerts",
					Name:      "notifications_total",
					Help:      "Alert notification outcomes by channel and status.",
				},
				[]string{"channel", "status"},
			),
			alertNotifyDurationSec: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: "fractsoul",
					Subsystem: "alerts",
					Name:      "notification_duration_seconds",
					Help:      "Alert notification time in seconds by channel and status.",
					Buckets:   []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
				},
				[]string{"channel", "status"},
			),
		}

		registry.MustRegister(
			metrics.httpInFlight,
			metrics.httpRequestsTotal,
			metrics.httpDurationSeconds,
			metrics.ingestEventsTotal,
			metrics.ingestPayloadSize,
			metrics.processorEventsTotal,
			metrics.processorDuration,
			metrics.alertEvaluationsTotal,
			metrics.alertNotifications,
			metrics.alertNotifyDurationSec,
		)

		metricsInst = metrics
	})

	return metricsInst
}

func normalizePath(path string) string {
	value := strings.TrimSpace(path)
	if value == "" {
		return "unmatched"
	}
	if !strings.HasPrefix(value, "/") {
		return "unmatched"
	}
	return value
}

func sanitizeLabel(value, fallback string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return fallback
	}
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	return normalized
}

func boolLabel(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
