package exercise

import (
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

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
	router.Get("/exercises", h.list)
	router.Post("/exercises", h.create)
	router.Get("/exercises/{id}", h.get)
	router.Patch("/exercises/{id}", h.patch)
	router.Delete("/exercises/{id}", h.archive)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := auth.PrincipalFromContext(r.Context())
	filter, err := listFilter(r)
	if err != nil {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_query", "Invalid query", "Exercise filters, booleans, limit, and cursor must use the documented format")
		return
	}
	result, err := h.service.List(r.Context(), principal.UserID, filter)
	if err != nil {
		h.writeError(w, r, "list_exercises", err)
		return
	}
	httpserver.WriteCollection(w, r, http.StatusOK, result.Items, result.NextCursor)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := auth.PrincipalFromContext(r.Context())
	var input CreateInput
	if !decodeJSON(w, r, 32<<10, &input) {
		return
	}
	value, err := h.service.Create(r.Context(), principal.UserID, input)
	if err != nil {
		h.writeError(w, r, "create_exercise", err)
		return
	}
	w.Header().Set("Location", "/api/v1/exercises/"+value.ID)
	w.Header().Set("ETag", exerciseETag(value))
	httpserver.WriteData(w, r, http.StatusCreated, value)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	exerciseID, ok := exerciseID(w, r)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Get(r.Context(), principal.UserID, exerciseID)
	if err != nil {
		h.writeError(w, r, "get_exercise", err)
		return
	}
	w.Header().Set("ETag", exerciseETag(value))
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	exerciseID, ok := exerciseID(w, r)
	if !ok {
		return
	}
	version, ok := expectedVersion(w, r, "exercise", exerciseID)
	if !ok {
		return
	}
	var input PatchInput
	if !decodeJSON(w, r, 32<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Patch(r.Context(), principal.UserID, exerciseID, version, input)
	if err != nil {
		h.writeError(w, r, "patch_exercise", err)
		return
	}
	w.Header().Set("ETag", exerciseETag(value))
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	exerciseID, ok := exerciseID(w, r)
	if !ok {
		return
	}
	version, ok := expectedVersion(w, r, "exercise", exerciseID)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	if err := h.service.Archive(r.Context(), principal.UserID, exerciseID, version); err != nil {
		h.writeError(w, r, "archive_exercise", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listFilter(r *http.Request) (ListFilter, error) {
	query := r.URL.Query()
	allowed := map[string]struct{}{
		"q": {}, "scope": {}, "muscle_group": {}, "type": {}, "equipment": {},
		"include_archived": {}, "tracks_weight": {}, "tracks_repetitions": {},
		"tracks_time": {}, "tracks_distance": {}, "limit": {}, "cursor": {},
	}
	for key, values := range query {
		if _, ok := allowed[key]; !ok || len(values) != 1 {
			return ListFilter{}, ErrValidation
		}
	}
	limit := 50
	if value := query.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return ListFilter{}, ErrValidation
		}
		limit = parsed
	}
	filter := ListFilter{
		Query: query.Get("q"), Scope: query.Get("scope"),
		MuscleGroup: query.Get("muscle_group"), ExerciseType: query.Get("type"),
		Equipment: query.Get("equipment"), Limit: limit,
	}
	var err error
	if filter.IncludeArchived, err = optionalBool(query.Get("include_archived"), false); err != nil {
		return ListFilter{}, ErrValidation
	}
	for value, destination := range map[string]**bool{
		"tracks_weight": &filter.TracksWeight, "tracks_repetitions": &filter.TracksRepetitions,
		"tracks_time": &filter.TracksTime, "tracks_distance": &filter.TracksDistance,
	} {
		if raw := query.Get(value); raw != "" {
			parsed, err := strconv.ParseBool(raw)
			if err != nil {
				return ListFilter{}, ErrValidation
			}
			*destination = &parsed
		}
	}
	if raw := query.Get("cursor"); raw != "" {
		cursor, err := DecodeCursor(raw)
		if err != nil {
			return ListFilter{}, ErrValidation
		}
		filter.Cursor = &cursor
	}
	return filter, nil
}

func optionalBool(value string, fallback bool) (bool, error) {
	if value == "" {
		return fallback, nil
	}
	return strconv.ParseBool(value)
}

func exerciseID(w http.ResponseWriter, r *http.Request) (string, bool) {
	value := chi.URLParam(r, "id")
	if !id.ValidUUID(value) {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_id", "Invalid identifier", "Exercise id must be a UUID")
		return "", false
	}
	return value, true
}

func exerciseETag(value Exercise) string {
	return fmt.Sprintf("\"exercise:%s:%d\"", value.ID, value.Version)
}

func expectedVersion(w http.ResponseWriter, r *http.Request, resource, resourceID string) (int64, bool) {
	value := strings.TrimSpace(r.Header.Get("If-Match"))
	if value == "" {
		httpserver.WriteProblem(w, r, http.StatusPreconditionRequired, "precondition_required", "Precondition required", "If-Match with the current ETag is required")
		return 0, false
	}
	prefix := "\"" + resource + ":" + resourceID + ":"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, "\"") {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_if_match", "Invalid If-Match", "If-Match must contain the current resource ETag")
		return 0, false
	}
	version, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(value, prefix), "\""), 10, 64)
	if err != nil || version < 1 {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_if_match", "Invalid If-Match", "If-Match must contain the current resource ETag")
		return 0, false
	}
	return version, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, destination any) bool {
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

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", "Exercise data or filters do not satisfy the documented constraints")
	case errors.Is(err, ErrNotFound):
		httpserver.WriteProblem(w, r, http.StatusNotFound, "not_found", "Not found", "Exercise was not found")
	case errors.Is(err, ErrSystemImmutable):
		httpserver.WriteProblem(w, r, http.StatusForbidden, "system_exercise_immutable", "Forbidden", "System exercises cannot be changed")
	case errors.Is(err, ErrArchived):
		httpserver.WriteProblem(w, r, http.StatusConflict, "exercise_archived", "Conflict", "Archived exercise cannot be changed")
	case errors.Is(err, ErrNameConflict):
		httpserver.WriteProblem(w, r, http.StatusConflict, "exercise_name_conflict", "Conflict", "An active custom exercise with this name already exists")
	case errors.Is(err, ErrVersionConflict):
		httpserver.WriteProblem(w, r, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "Exercise changed; fetch it and retry with the current ETag")
	default:
		h.logger.ErrorContext(r.Context(), "exercise operation failed", slog.String("operation", operation))
		httpserver.WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal server error", "Request could not be processed")
	}
}
