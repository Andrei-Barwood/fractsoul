package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fractsoul/mvp/backend/services/ingest-api/internal/telemetry"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const (
	maxFutureTimestampSkew   = 5 * time.Minute
	defaultIngestMaxBodySize = 1 << 20 // 1 MiB
)

type TelemetryHandler struct {
	logger       *slog.Logger
	publisher    telemetry.Publisher
	subject      string
	maxBodyBytes int64
}

func NewTelemetryHandler(logger *slog.Logger, publisher telemetry.Publisher, subject string, maxBodyBytes int64) *TelemetryHandler {
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultIngestMaxBodySize
	}

	return &TelemetryHandler{
		logger:       logger,
		publisher:    publisher,
		subject:      subject,
		maxBodyBytes: maxBodyBytes,
	}
}

func (h *TelemetryHandler) Ingest(c *gin.Context) {
	if !isJSONContentType(c.GetHeader("Content-Type")) {
		WriteError(
			c,
			http.StatusUnsupportedMediaType,
			"unsupported_media_type",
			"content type must be application/json",
			map[string]string{"expected": "application/json"},
		)
		return
	}

	request, ingestErr := h.decodeAndValidateRequest(c)
	if ingestErr != nil {
		WriteError(c, ingestErr.Status, ingestErr.Code, ingestErr.Message, ingestErr.Details)
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

	siteID, rackID, minerID, err := telemetry.NormalizeOperationalIDs(request.SiteID, request.RackID, request.MinerID)
	if err != nil {
		WriteError(
			c,
			http.StatusBadRequest,
			"validation_error",
			"invalid operational identifiers",
			map[string]string{"reason": err.Error()},
		)
		return
	}
	request.SiteID = siteID
	request.RackID = rackID
	request.MinerID = minerID

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

type ingestError struct {
	Status  int
	Code    string
	Message string
	Details any
}

func (h *TelemetryHandler) decodeAndValidateRequest(c *gin.Context) (telemetry.IngestRequest, *ingestError) {
	var request telemetry.IngestRequest

	bodyReader := http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBodyBytes)
	defer bodyReader.Close()

	decoder := json.NewDecoder(bodyReader)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		return telemetry.IngestRequest{}, classifyDecodeError(err, h.maxBodyBytes)
	}

	// Ensure payload contains a single JSON object.
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return telemetry.IngestRequest{}, &ingestError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_json",
			Message: "payload must contain a single JSON object",
		}
	}

	if err := binding.Validator.ValidateStruct(request); err != nil {
		return telemetry.IngestRequest{}, &ingestError{
			Status:  http.StatusBadRequest,
			Code:    "validation_error",
			Message: "payload validation failed",
			Details: ValidationDetails(err),
		}
	}

	return request, nil
}

func classifyDecodeError(err error, maxBodyBytes int64) *ingestError {
	var syntaxErr *json.SyntaxError
	var maxBytesErr *http.MaxBytesError
	var unmarshalTypeErr *json.UnmarshalTypeError

	switch {
	case errors.Is(err, io.EOF):
		return &ingestError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_json",
			Message: "payload body is empty",
		}
	case errors.As(err, &maxBytesErr):
		return &ingestError{
			Status:  http.StatusRequestEntityTooLarge,
			Code:    "payload_too_large",
			Message: "payload exceeds maximum allowed size",
			Details: map[string]string{"max_bytes": strconv.FormatInt(maxBodyBytes, 10)},
		}
	case errors.As(err, &syntaxErr):
		return &ingestError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_json",
			Message: "payload JSON is malformed",
			Details: map[string]string{"offset": strconv.FormatInt(syntaxErr.Offset, 10)},
		}
	case errors.As(err, &unmarshalTypeErr):
		field := strings.TrimSpace(unmarshalTypeErr.Field)
		if field == "" {
			field = "unknown"
		}
		return &ingestError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_json",
			Message: "payload has invalid field types",
			Details: map[string]string{
				"field":    field,
				"expected": unmarshalTypeErr.Type.String(),
			},
		}
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.TrimPrefix(err.Error(), "json: unknown field ")
		field = strings.Trim(field, "\"")
		return &ingestError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_json",
			Message: "payload contains unknown fields",
			Details: map[string]string{"field": field},
		}
	default:
		return &ingestError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_json",
			Message: "payload JSON is invalid",
			Details: map[string]string{"reason": err.Error()},
		}
	}
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json"
}
