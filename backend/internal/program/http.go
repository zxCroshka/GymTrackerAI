package program

import (
	"errors"
	"fmt"
	"io"
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
	router.Get("/programs", h.list)
	router.Post("/programs", h.create)
	router.Get("/programs/{id}", h.get)
	router.Patch("/programs/{id}", h.patch)
	router.Delete("/programs/{id}", h.archive)
	router.Post("/programs/{id}/duplicate", h.duplicate)
	router.Post("/programs/{id}/activate", h.activate)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := auth.PrincipalFromContext(r.Context())
	filter, err := programListFilter(r)
	if err != nil {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_query", "Invalid query", "Program status, include_archived, limit, and cursor must use the documented format")
		return
	}
	result, err := h.service.List(r.Context(), principal.UserID, filter)
	if err != nil {
		h.writeError(w, r, "list_programs", err)
		return
	}
	httpserver.WriteCollection(w, r, http.StatusOK, result.Items, result.NextCursor)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input CreateInput
	if !decodeProgramJSON(w, r, 256<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Create(r.Context(), principal.UserID, input)
	if err != nil {
		h.writeError(w, r, "create_program", err)
		return
	}
	w.Header().Set("Location", "/api/v1/programs/"+value.ID)
	w.Header().Set("ETag", programETag(value))
	httpserver.WriteData(w, r, http.StatusCreated, value)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	programID, ok := requestProgramID(w, r)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Get(r.Context(), principal.UserID, programID)
	if err != nil {
		h.writeError(w, r, "get_program", err)
		return
	}
	w.Header().Set("ETag", programETag(value))
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	programID, ok := requestProgramID(w, r)
	if !ok {
		return
	}
	version, ok := programExpectedVersion(w, r, programID)
	if !ok {
		return
	}
	var input PatchInput
	if !decodeProgramJSON(w, r, 256<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Patch(r.Context(), principal.UserID, programID, version, input)
	if err != nil {
		h.writeError(w, r, "patch_program", err)
		return
	}
	w.Header().Set("ETag", programETag(value))
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	programID, ok := requestProgramID(w, r)
	if !ok {
		return
	}
	version, ok := programExpectedVersion(w, r, programID)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	if err := h.service.Archive(r.Context(), principal.UserID, programID, version); err != nil {
		h.writeError(w, r, "archive_program", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type duplicateInput struct {
	Name *string `json:"name"`
}

func (h *Handler) duplicate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	programID, ok := requestProgramID(w, r)
	if !ok {
		return
	}
	var input duplicateInput
	if r.ContentLength != 0 {
		if !decodeProgramJSON(w, r, 8<<10, &input) {
			return
		}
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Duplicate(r.Context(), principal.UserID, programID, input.Name)
	if err != nil {
		h.writeError(w, r, "duplicate_program", err)
		return
	}
	w.Header().Set("Location", "/api/v1/programs/"+value.ID)
	w.Header().Set("ETag", programETag(value))
	httpserver.WriteData(w, r, http.StatusCreated, value)
}

func (h *Handler) activate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	programID, ok := requestProgramID(w, r)
	if !ok {
		return
	}
	version, ok := programExpectedVersion(w, r, programID)
	if !ok {
		return
	}
	if r.Body != nil {
		defer r.Body.Close()
		var oneByte [1]byte
		if count, err := r.Body.Read(oneByte[:]); count != 0 || err != nil && !errors.Is(err, io.EOF) {
			httpserver.WriteProblem(w, r, http.StatusBadRequest, "unexpected_body", "Unexpected body", "Activation request must have an empty body")
			return
		}
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Activate(r.Context(), principal.UserID, programID, version)
	if err != nil {
		h.writeError(w, r, "activate_program", err)
		return
	}
	w.Header().Set("ETag", programETag(value))
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func programListFilter(r *http.Request) (ListFilter, error) {
	query := r.URL.Query()
	allowed := map[string]struct{}{
		"status": {}, "include_archived": {}, "limit": {}, "cursor": {},
	}
	for key, values := range query {
		if _, ok := allowed[key]; !ok || len(values) != 1 {
			return ListFilter{}, ErrValidation
		}
	}
	limit := 50
	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return ListFilter{}, ErrValidation
		}
		limit = parsed
	}
	includeArchived := false
	if raw := query.Get("include_archived"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return ListFilter{}, ErrValidation
		}
		includeArchived = parsed
	}
	filter := ListFilter{
		Status: query.Get("status"), IncludeArchived: includeArchived, Limit: limit,
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

func requestProgramID(w http.ResponseWriter, r *http.Request) (string, bool) {
	value := chi.URLParam(r, "id")
	if !id.ValidUUID(value) {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_id", "Invalid identifier", "Program id must be a UUID")
		return "", false
	}
	return value, true
}

func programETag(value Program) string {
	return fmt.Sprintf("\"program:%s:%d\"", value.ID, value.Version)
}

func programExpectedVersion(w http.ResponseWriter, r *http.Request, programID string) (int64, bool) {
	value := strings.TrimSpace(r.Header.Get("If-Match"))
	if value == "" {
		httpserver.WriteProblem(w, r, http.StatusPreconditionRequired, "precondition_required", "Precondition required", "If-Match with the current ETag is required")
		return 0, false
	}
	prefix := "\"program:" + programID + ":"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, "\"") {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_if_match", "Invalid If-Match", "If-Match must contain the current program ETag")
		return 0, false
	}
	version, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(value, prefix), "\""), 10, 64)
	if err != nil || version < 1 {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_if_match", "Invalid If-Match", "If-Match must contain the current program ETag")
		return 0, false
	}
	return version, true
}

func decodeProgramJSON(w http.ResponseWriter, r *http.Request, limit int64, destination any) bool {
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
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", "Program data or filters do not satisfy the documented constraints")
	case errors.Is(err, ErrNotFound):
		httpserver.WriteProblem(w, r, http.StatusNotFound, "not_found", "Not found", "Program was not found")
	case errors.Is(err, ErrArchived):
		httpserver.WriteProblem(w, r, http.StatusConflict, "program_archived", "Conflict", "Archived program cannot be changed")
	case errors.Is(err, ErrVersionConflict):
		httpserver.WriteProblem(w, r, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "Program changed; fetch it and retry with the current ETag")
	case errors.Is(err, ErrExerciseUnavailable):
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "exercise_unavailable", "Exercise unavailable", "Every exercise must be a visible, non-archived system or user exercise")
	case errors.Is(err, ErrNotActivatable):
		httpserver.WriteProblem(w, r, http.StatusConflict, "program_not_activatable", "Program is not activatable", "An active program must contain at least one day and every day must contain an exercise")
	default:
		h.logger.ErrorContext(r.Context(), "program operation failed", slog.String("operation", operation))
		httpserver.WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal server error", "Request could not be processed")
	}
}
