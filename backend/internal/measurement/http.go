package measurement

import (
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/auth"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/httpserver"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/measurements", h.createBody)
	router.Get("/measurements", h.listBody)
	router.Patch("/measurements/{id}", h.patchBody)
	router.Delete("/measurements/{id}", h.deleteBody)
	router.Post("/wellness", h.createWellness)
	router.Get("/wellness", h.listWellness)
}

func (h *Handler) createBody(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var input BodyCreateInput
	if !decodeMeasurementJSON(w, r, 64<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.CreateBody(r.Context(), principal.UserID, input)
	if err != nil {
		h.writeError(w, r, "create_measurement", err)
		return
	}
	w.Header().Set("Location", "/api/v1/measurements/"+value.ID)
	w.Header().Set("ETag", measurementETag(value.ID, value.Version))
	httpserver.WriteData(w, r, http.StatusCreated, value)
}

func (h *Handler) listBody(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	filter, err := measurementListFilter(r)
	if err != nil {
		writeMeasurementInvalidQuery(w, r)
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := h.service.ListBody(r.Context(), principal.UserID, filter)
	if err != nil {
		h.writeError(w, r, "list_measurements", err)
		return
	}
	httpserver.WriteCollection(w, r, http.StatusOK, result.Items, result.NextCursor)
}

func (h *Handler) patchBody(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	valueID, ok := measurementID(w, r)
	if !ok {
		return
	}
	version, ok := measurementExpectedVersion(w, r, valueID)
	if !ok {
		return
	}
	var input BodyPatchInput
	if !decodeMeasurementJSON(w, r, 64<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.PatchBody(r.Context(), principal.UserID, valueID, version, input)
	if err != nil {
		h.writeError(w, r, "patch_measurement", err)
		return
	}
	w.Header().Set("ETag", measurementETag(value.ID, value.Version))
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func (h *Handler) deleteBody(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	valueID, ok := measurementID(w, r)
	if !ok {
		return
	}
	version, ok := measurementExpectedVersion(w, r, valueID)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	if err := h.service.DeleteBody(r.Context(), principal.UserID, valueID, version); err != nil {
		h.writeError(w, r, "delete_measurement", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createWellness(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var input WellnessCreateInput
	if !decodeMeasurementJSON(w, r, 64<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.CreateWellness(r.Context(), principal.UserID, input)
	if err != nil {
		h.writeError(w, r, "create_wellness", err)
		return
	}
	httpserver.WriteData(w, r, http.StatusCreated, value)
}

func (h *Handler) listWellness(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	filter, err := measurementListFilter(r)
	if err != nil {
		writeMeasurementInvalidQuery(w, r)
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := h.service.ListWellness(r.Context(), principal.UserID, filter)
	if err != nil {
		h.writeError(w, r, "list_wellness", err)
		return
	}
	httpserver.WriteCollection(w, r, http.StatusOK, result.Items, result.NextCursor)
}

func measurementListFilter(r *http.Request) (ListFilter, error) {
	query := r.URL.Query()
	allowed := map[string]struct{}{"from": {}, "to": {}, "limit": {}, "cursor": {}}
	for key, values := range query {
		if _, ok := allowed[key]; !ok || len(values) != 1 {
			return ListFilter{}, ErrValidation
		}
	}
	filter := ListFilter{Limit: 50}
	if raw := query.Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return ListFilter{}, ErrValidation
		}
		filter.Limit = value
	}
	var err error
	if raw := query.Get("from"); raw != "" {
		value, parseErr := parseUTCInstant(raw)
		if parseErr != nil {
			return ListFilter{}, parseErr
		}
		filter.From = &value
	}
	if raw := query.Get("to"); raw != "" {
		value, parseErr := parseUTCInstant(raw)
		if parseErr != nil {
			return ListFilter{}, parseErr
		}
		filter.To = &value
	}
	if raw := query.Get("cursor"); raw != "" {
		value, parseErr := decodeCursor(raw)
		if parseErr != nil {
			return ListFilter{}, parseErr
		}
		filter.Cursor = &value
	}
	err = validateFilter(filter)
	return filter, err
}

func parseUTCInstant(value string) (time.Time, error) {
	if !strings.HasSuffix(value, "Z") {
		return time.Time{}, ErrValidation
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, ErrValidation
	}
	return parsed.UTC(), nil
}

func measurementID(w http.ResponseWriter, r *http.Request) (string, bool) {
	value := chi.URLParam(r, "id")
	if !id.ValidUUID(value) {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_id", "Invalid identifier", "Measurement id must be a UUID")
		return "", false
	}
	return value, true
}

func measurementExpectedVersion(w http.ResponseWriter, r *http.Request, valueID string) (int64, bool) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	if raw == "" {
		httpserver.WriteProblem(w, r, http.StatusPreconditionRequired, "precondition_required", "Precondition required", "If-Match with the current measurement ETag is required")
		return 0, false
	}
	prefix := "\"measurement:" + valueID + ":"
	if !strings.HasPrefix(raw, prefix) || !strings.HasSuffix(raw, "\"") {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_if_match", "Invalid If-Match", "If-Match must contain the current measurement ETag")
		return 0, false
	}
	version, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(raw, prefix), "\""), 10, 64)
	if err != nil || version < 1 {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_if_match", "Invalid If-Match", "If-Match must contain the current measurement ETag")
		return 0, false
	}
	return version, true
}

func measurementETag(valueID string, version int64) string {
	return fmt.Sprintf("\"measurement:%s:%d\"", valueID, version)
}
func noStore(w http.ResponseWriter) { w.Header().Set("Cache-Control", "no-store") }

func decodeMeasurementJSON(w http.ResponseWriter, r *http.Request, limit int64, destination any) bool {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		httpserver.WriteProblem(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Unsupported media type", "Content-Type must be application/json")
		return false
	}
	if err := httpserver.DecodeJSON(w, r, limit, destination); err != nil {
		status, code, title, detail := http.StatusBadRequest, "invalid_json", "Invalid JSON", "Body must contain one valid JSON object with only supported fields"
		if errors.Is(err, httpserver.ErrBodyTooLarge) {
			status, code, title, detail = http.StatusRequestEntityTooLarge, "body_too_large", "Request body too large", "Request body exceeds the allowed size"
		}
		httpserver.WriteProblem(w, r, status, code, title, detail)
		return false
	}
	return true
}

func writeMeasurementInvalidQuery(w http.ResponseWriter, r *http.Request) {
	httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_query", "Invalid query", "Range, pagination limit, and cursor must use the documented format")
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", "Measurement or wellness data does not satisfy the documented constraints")
	case errors.Is(err, ErrNotFound):
		httpserver.WriteProblem(w, r, http.StatusNotFound, "not_found", "Not found", "Measurement was not found")
	case errors.Is(err, ErrVersionConflict):
		httpserver.WriteProblem(w, r, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "Measurement changed; fetch the list and retry with its current ETag")
	case errors.Is(err, ErrInstantConflict):
		httpserver.WriteProblem(w, r, http.StatusConflict, "measurement_already_exists", "Conflict", "A measurement already exists at this instant")
	case errors.Is(err, ErrWellnessExists):
		httpserver.WriteProblem(w, r, http.StatusConflict, "wellness_already_exists", "Conflict", "A wellness entry already exists for this local day")
	default:
		h.logger.ErrorContext(r.Context(), "measurement operation failed", slog.String("operation", operation))
		httpserver.WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal server error", "Request could not be processed")
	}
}
