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

	budgetCalculationsTotal    *prometheus.CounterVec
	dispatchValidationsTotal   *prometheus.CounterVec
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

func RecordBudgetCalculation(result string) {
	loadMetrics().budgetCalculationsTotal.WithLabelValues(sanitizeLabel(result, "unknown")).Inc()
}

func RecordDispatchValidation(result string) {
	loadMetrics().dispatchValidationsTotal.WithLabelValues(sanitizeLabel(result, "unknown")).Inc()
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
				Namespace: "fractsoul_energy",
				Subsystem: "http",
				Name:      "inflight_requests",
				Help:      "Current number of in-flight HTTP requests.",
			}),
			httpRequestsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "fractsoul_energy",
					Subsystem: "http",
					Name:      "requests_total",
					Help:      "Total HTTP requests handled by method, route path and status.",
				},
				[]string{"method", "path", "status"},
			),
			httpDurationSeconds: prometheus.NewHistogramVec(
				prometheus.HistogramOpts{
					Namespace: "fractsoul_energy",
					Subsystem: "http",
					Name:      "request_duration_seconds",
					Help:      "HTTP request latency in seconds by method, route path and status.",
					Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
				},
				[]string{"method", "path", "status"},
			),
			budgetCalculationsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "fractsoul_energy",
					Subsystem: "budget",
					Name:      "calculations_total",
					Help:      "Budget calculations by result.",
				},
				[]string{"result"},
			),
			dispatchValidationsTotal: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "fractsoul_energy",
					Subsystem: "dispatch",
					Name:      "validations_total",
					Help:      "Dispatch validations by result.",
				},
				[]string{"result"},
			),
		}

		registry.MustRegister(
			metrics.httpInFlight,
			metrics.httpRequestsTotal,
			metrics.httpDurationSeconds,
			metrics.budgetCalculationsTotal,
			metrics.dispatchValidationsTotal,
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
