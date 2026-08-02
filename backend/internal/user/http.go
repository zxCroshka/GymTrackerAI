package user

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/auth"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/httpserver"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/profile", h.get)
	router.Patch("/profile", h.patch)
	router.Post("/profile/import", h.importProfile)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := auth.PrincipalFromContext(r.Context())
	profile, err := h.service.Get(r.Context(), principal.UserID)
	if err != nil {
		h.writeError(w, r, "get_profile", err)
		return
	}
	w.Header().Set("ETag", profileETag(profile))
	httpserver.WriteData(w, r, http.StatusOK, profile)
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := auth.PrincipalFromContext(r.Context())
	version, ok := expectedProfileVersion(w, r, principal.UserID)
	if !ok {
		return
	}
	var input PatchInput
	if !decodeRequest(w, r, 32<<10, &input) {
		return
	}
	profile, err := h.service.Patch(r.Context(), principal.UserID, version, input)
	if err != nil {
		h.writeError(w, r, "patch_profile", err)
		return
	}
	w.Header().Set("ETag", profileETag(profile))
	httpserver.WriteData(w, r, http.StatusOK, profile)
}

func (h *Handler) importProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	principal, _ := auth.PrincipalFromContext(r.Context())
	version, ok := expectedProfileVersion(w, r, principal.UserID)
	if !ok {
		return
	}
	var input ImportInput
	if !decodeRequest(w, r, 256<<10, &input) {
		return
	}
	result, err := h.service.Import(r.Context(), principal.UserID, version, input)
	if err != nil {
		h.writeError(w, r, "import_profile", err)
		return
	}
	w.Header().Set("ETag", profileETag(result.Profile))
	httpserver.WriteData(w, r, http.StatusOK, result)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", "Profile data does not satisfy the documented constraints")
	case errors.Is(err, ErrVersionConflict):
		httpserver.WriteProblem(w, r, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "Profile has changed; fetch it and retry with the current ETag")
	case errors.Is(err, ErrProfileNotFound):
		httpserver.WriteProblem(w, r, http.StatusNotFound, "not_found", "Not found", "Profile was not found")
	default:
		h.logger.ErrorContext(r.Context(), "user operation failed", slog.String("operation", operation))
		httpserver.WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal server error", "Request could not be processed")
	}
}

func profileETag(profile Profile) string {
	return fmt.Sprintf("\"profile:%s:%d\"", profile.UserID, profile.Version)
}

func expectedProfileVersion(w http.ResponseWriter, r *http.Request, userID string) (int64, bool) {
	value := strings.TrimSpace(r.Header.Get("If-Match"))
	if value == "" {
		httpserver.WriteProblem(w, r, http.StatusPreconditionRequired, "precondition_required", "Precondition required", "If-Match with the current profile ETag is required")
		return 0, false
	}
	prefix := "\"profile:" + userID + ":"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, "\"") {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_if_match", "Invalid If-Match", "If-Match must contain the current profile ETag")
		return 0, false
	}
	version, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(value, prefix), "\""), 10, 64)
	if err != nil || version < 1 {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_if_match", "Invalid If-Match", "If-Match must contain the current profile ETag")
		return 0, false
	}
	return version, true
}

func decodeRequest(w http.ResponseWriter, r *http.Request, limit int64, destination any) bool {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if contentType != "application/json" {
		httpserver.WriteProblem(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Unsupported media type", "Content-Type must be application/json")
		return false
	}
	if err := httpserver.DecodeJSON(w, r, limit, destination); err != nil {
		status, code, title, detail := http.StatusBadRequest, "invalid_json", "Invalid JSON", "Request body must contain one valid JSON object with only supported fields"
		if errors.Is(err, httpserver.ErrBodyTooLarge) {
			status, code, title, detail = http.StatusRequestEntityTooLarge, "body_too_large", "Request body too large", "Request body exceeds the allowed size"
		}
		httpserver.WriteProblem(w, r, status, code, title, detail)
		return false
	}
	return true
}
