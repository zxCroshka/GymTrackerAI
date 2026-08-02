package progress

import (
	"errors"
	"log/slog"
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
	router.Get("/progress/dashboard", h.dashboard)
	router.Get("/progress/weight", h.weight)
	router.Get("/progress/exercises/{exerciseId}", h.exercise)
	router.Get("/progress/personal-records", h.records)
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if len(r.URL.Query()) != 0 {
		writeProgressInvalidQuery(w, r)
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Dashboard(r.Context(), principal.UserID)
	if err != nil {
		h.writeError(w, r, "progress_dashboard", err)
		return
	}
	httpserver.WriteData(w, r, http.StatusOK, value)
}
func (h *Handler) weight(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	from, to, err := progressRange(r, 30*24*time.Hour)
	if err != nil {
		writeProgressInvalidQuery(w, r)
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Weight(r.Context(), principal.UserID, from, to)
	if err != nil {
		h.writeError(w, r, "progress_weight", err)
		return
	}
	httpserver.WriteData(w, r, http.StatusOK, value)
}
func (h *Handler) exercise(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	exerciseID := chi.URLParam(r, "exerciseId")
	if !id.ValidUUID(exerciseID) {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_id", "Invalid identifier", "Exercise id must be a UUID")
		return
	}
	from, to, err := progressRange(r, 365*24*time.Hour)
	if err != nil {
		writeProgressInvalidQuery(w, r)
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Exercise(r.Context(), principal.UserID, exerciseID, from, to)
	if err != nil {
		h.writeError(w, r, "progress_exercise", err)
		return
	}
	httpserver.WriteData(w, r, http.StatusOK, value)
}
func (h *Handler) records(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	query := r.URL.Query()
	allowed := map[string]struct{}{"exercise_id": {}, "record_type": {}, "from": {}, "to": {}, "limit": {}}
	for key, values := range query {
		if _, ok := allowed[key]; !ok || len(values) != 1 {
			writeProgressInvalidQuery(w, r)
			return
		}
	}
	filter := RecordFilter{ExerciseID: query.Get("exercise_id"), RecordType: query.Get("record_type"), Limit: 50}
	if raw := query.Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeProgressInvalidQuery(w, r)
			return
		}
		filter.Limit = value
	}
	if raw := query.Get("from"); raw != "" {
		value, err := parseProgressUTC(raw)
		if err != nil {
			writeProgressInvalidQuery(w, r)
			return
		}
		filter.From = &value
	}
	if raw := query.Get("to"); raw != "" {
		value, err := parseProgressUTC(raw)
		if err != nil {
			writeProgressInvalidQuery(w, r)
			return
		}
		filter.To = &value
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.PersonalRecords(r.Context(), principal.UserID, filter)
	if err != nil {
		h.writeError(w, r, "progress_records", err)
		return
	}
	httpserver.WriteCollection(w, r, http.StatusOK, value, nil)
}

func progressRange(r *http.Request, defaultDuration time.Duration) (time.Time, time.Time, error) {
	query := r.URL.Query()
	for key, values := range query {
		if key != "from" && key != "to" || len(values) != 1 {
			return time.Time{}, time.Time{}, ErrValidation
		}
	}
	to := time.Now().UTC()
	from := to.Add(-defaultDuration)
	var err error
	if raw := query.Get("to"); raw != "" {
		to, err = parseProgressUTC(raw)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if raw := query.Get("from"); raw != "" {
		from, err = parseProgressUTC(raw)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if !from.Before(to) || to.Sub(from) > 2*365*24*time.Hour {
		return time.Time{}, time.Time{}, ErrValidation
	}
	return from, to, nil
}
func parseProgressUTC(value string) (time.Time, error) {
	if !strings.HasSuffix(value, "Z") {
		return time.Time{}, ErrValidation
	}
	return time.Parse(time.RFC3339Nano, value)
}
func writeProgressInvalidQuery(w http.ResponseWriter, r *http.Request) {
	httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_query", "Invalid query", "Progress range and filters must use the documented bounded format")
}
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	if errors.Is(err, ErrValidation) {
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", "Progress query does not satisfy the documented constraints")
		return
	}
	h.logger.ErrorContext(r.Context(), "progress operation failed", slog.String("operation", operation))
	httpserver.WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal server error", "Request could not be processed")
}
