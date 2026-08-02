package report

import (
	"errors"
	"fmt"
	"io"
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
	router.Post("/reports/weekly", h.generate)
	router.Get("/reports", h.list)
	router.Get("/reports/{id}", h.get)
}

func (h *Handler) generate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var input GenerateInput
	if r.ContentLength != 0 {
		contentType, _, mediaErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if mediaErr != nil || contentType != "application/json" {
			httpserver.WriteProblem(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Unsupported media type", "Content-Type must be application/json")
			return
		}
		if err := httpserver.DecodeJSON(w, r, 16<<10, &input); err != nil {
			status := http.StatusBadRequest
			code := "invalid_json"
			if errors.Is(err, httpserver.ErrBodyTooLarge) {
				status = http.StatusRequestEntityTooLarge
				code = "body_too_large"
			}
			httpserver.WriteProblem(w, r, status, code, "Invalid request", "Body must contain one valid JSON object with only supported fields")
			return
		}
	} else if r.Body != nil {
		defer r.Body.Close()
		_, _ = io.Copy(io.Discard, r.Body)
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, created, err := h.service.GenerateWeekly(r.Context(), principal.UserID, input)
	if err != nil {
		h.writeError(w, r, "generate_weekly_report", err)
		return
	}
	w.Header().Set("ETag", reportETag(value))
	status := http.StatusOK
	if created {
		status = http.StatusCreated
		w.Header().Set("Location", "/api/v1/reports/"+value.ID)
	}
	httpserver.WriteData(w, r, status, value)
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	filter, err := reportListFilter(r)
	if err != nil {
		writeReportInvalidQuery(w, r)
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	values, err := h.service.List(r.Context(), principal.UserID, filter)
	if err != nil {
		h.writeError(w, r, "list_reports", err)
		return
	}
	httpserver.WriteCollection(w, r, http.StatusOK, values, nil)
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	reportID := chi.URLParam(r, "id")
	if !id.ValidUUID(reportID) {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_id", "Invalid identifier", "Report id must be a UUID")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Get(r.Context(), principal.UserID, reportID)
	if err != nil {
		h.writeError(w, r, "get_report", err)
		return
	}
	w.Header().Set("ETag", reportETag(value))
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func reportListFilter(r *http.Request) (ListFilter, error) {
	query := r.URL.Query()
	allowed := map[string]struct{}{"from": {}, "to": {}, "status": {}, "include_revisions": {}, "limit": {}}
	for key, values := range query {
		if _, ok := allowed[key]; !ok || len(values) != 1 {
			return ListFilter{}, ErrValidation
		}
	}
	filter := ListFilter{Status: query.Get("status"), Limit: 50}
	if raw := query.Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return ListFilter{}, ErrValidation
		}
		filter.Limit = value
	}
	if raw := query.Get("include_revisions"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return ListFilter{}, ErrValidation
		}
		filter.IncludeRevisions = value
	}
	if raw := query.Get("from"); raw != "" {
		if !strings.HasSuffix(raw, "Z") {
			return ListFilter{}, ErrValidation
		}
		value, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return ListFilter{}, ErrValidation
		}
		filter.From = &value
	}
	if raw := query.Get("to"); raw != "" {
		if !strings.HasSuffix(raw, "Z") {
			return ListFilter{}, ErrValidation
		}
		value, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return ListFilter{}, ErrValidation
		}
		filter.To = &value
	}
	return filter, nil
}
func reportETag(value WeeklyReport) string {
	return fmt.Sprintf("\"report:%s:%d\"", value.ID, value.Version)
}
func writeReportInvalidQuery(w http.ResponseWriter, r *http.Request) {
	httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_query", "Invalid query", "Report filters and limit must use the documented format")
}
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", "Report request does not satisfy the documented constraints")
	case errors.Is(err, ErrNotFound):
		httpserver.WriteProblem(w, r, http.StatusNotFound, "not_found", "Not found", "Report was not found")
	default:
		h.logger.ErrorContext(r.Context(), "report operation failed", slog.String("operation", operation))
		httpserver.WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal server error", "Request could not be processed")
	}
}
