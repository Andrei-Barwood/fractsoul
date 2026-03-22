package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
	"github.com/gin-gonic/gin"
)

const maxFutureTimestampSkew = 5 * time.Minute

type TelemetryHandler struct {
	logger    *slog.Logger
	publisher telemetry.Publisher
	subject   string
}

func NewTelemetryHandler(logger *slog.Logger, publisher telemetry.Publisher, subject string) *TelemetryHandler {
	return &TelemetryHandler{
		logger:    logger,
		publisher: publisher,
		subject:   subject,
	}
}

func (h *TelemetryHandler) Ingest(c *gin.Context) {
	var request telemetry.IngestRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		WriteError(
			c,
			http.StatusBadRequest,
			"validation_error",
			"payload validation failed",
			ValidationDetails(err),
		)
		return
	}

	now := time.Now().UTC()
	if request.Timestamp.After(now.Add(maxFutureTimestampSkew)) {
		WriteError(
			c,
			http.StatusUnprocessableEntity,
			"timestamp_out_of_range",
			"timestamp is too far in the future",
			map[string]string{"max_skew": maxFutureTimestampSkew.String()},
		)
		return
	}

	response := telemetry.IngestResponse{
		RequestID:  RequestID(c),
		Accepted:   true,
		EventID:    request.EventID,
		QueueTopic: h.subject,
		IngestedAt: now,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		WriteError(
			c,
			http.StatusInternalServerError,
			"internal_error",
			"failed to encode telemetry payload",
			nil,
		)
		return
	}

	if err := h.publisher.Publish(c.Request.Context(), h.subject, payload, map[string]string{
		"X-Request-ID": response.RequestID,
		"X-Event-ID":   request.EventID,
	}); err != nil {
		h.logger.Error(
			"failed to publish telemetry",
			"request_id", response.RequestID,
			"event_id", request.EventID,
			"error", err,
		)
		WriteError(
			c,
			http.StatusServiceUnavailable,
			"dependency_unavailable",
			"failed to publish telemetry event",
			nil,
		)
		return
	}

	h.logger.Info(
		"telemetry accepted",
		"request_id", response.RequestID,
		"event_id", request.EventID,
		"site_id", request.SiteID,
		"rack_id", request.RackID,
		"miner_id", request.MinerID,
		"status", request.Metrics.Status,
		"hashrate_ths", request.Metrics.HashrateTHs,
	)

	c.JSON(http.StatusAccepted, response)
}
