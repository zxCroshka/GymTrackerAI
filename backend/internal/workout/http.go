package workout

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
	router.Post("/workouts", h.create)
	router.Get("/workouts", h.list)
	router.Get("/workouts/active", h.active)
	router.Get("/workouts/export.csv", h.exportCSV)
	router.Get("/workouts/{id}", h.get)
	router.Patch("/workouts/{id}", h.patch)
	router.Post("/workouts/{id}/complete", h.complete)
	router.Delete("/workouts/{id}", h.delete)
	router.Post("/workouts/{id}/exercises", h.addExercise)

	router.Get("/workout-exercises/{id}/previous-result", h.previousResult)
	router.Patch("/workout-exercises/{id}", h.patchExercise)
	router.Delete("/workout-exercises/{id}", h.deleteExercise)
	router.Post("/workout-exercises/{id}/sets", h.addSet)

	router.Patch("/workout-sets/{id}", h.patchSet)
	router.Delete("/workout-sets/{id}", h.deleteSet)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	var input CreateInput
	if !decodeWorkoutJSON(w, r, 64<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Create(r.Context(), principal.UserID, input)
	if err != nil {
		h.writeError(w, r, "create_workout", err)
		return
	}
	w.Header().Set("Location", "/api/v1/workouts/"+value.ID)
	w.Header().Set("ETag", workoutETag(value.ID, value.Version))
	httpserver.WriteData(w, r, http.StatusCreated, value)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	filter, err := workoutListFilter(r, false)
	if err != nil {
		writeInvalidQuery(w, r)
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	result, err := h.service.List(r.Context(), principal.UserID, filter)
	if err != nil {
		h.writeError(w, r, "list_workouts", err)
		return
	}
	httpserver.WriteCollection(w, r, http.StatusOK, result.Items, result.NextCursor)
}

func (h *Handler) active(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Active(r.Context(), principal.UserID)
	if err != nil {
		h.writeError(w, r, "get_active_workout", err)
		return
	}
	if value != nil {
		w.Header().Set("ETag", workoutETag(value.ID, value.Version))
	}
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func (h *Handler) exportCSV(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	filter, err := workoutListFilter(r, true)
	if err != nil {
		writeInvalidQuery(w, r)
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		`attachment; filename="gymtracker-workouts-%s.csv"`, time.Now().UTC().Format("20060102T150405Z")))
	if err := h.service.ExportCSV(r.Context(), principal.UserID, filter, w); err != nil {
		h.writeError(w, r, "export_workouts", err)
	}
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	workoutID, ok := requestUUID(w, r, "Workout")
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Get(r.Context(), principal.UserID, workoutID)
	if err != nil {
		h.writeError(w, r, "get_workout", err)
		return
	}
	w.Header().Set("ETag", workoutETag(value.ID, value.Version))
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	workoutID, ok := requestUUID(w, r, "Workout")
	if !ok {
		return
	}
	version, ok := expectedRootVersion(w, r, workoutID)
	if !ok {
		return
	}
	var input PatchInput
	if !decodeWorkoutJSON(w, r, 64<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Patch(r.Context(), principal.UserID, workoutID, version, input)
	if err != nil {
		h.writeError(w, r, "patch_workout", err)
		return
	}
	w.Header().Set("ETag", workoutETag(value.ID, value.Version))
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	workoutID, ok := requestUUID(w, r, "Workout")
	if !ok {
		return
	}
	version, ok := expectedRootVersion(w, r, workoutID)
	if !ok {
		return
	}
	var input CompleteInput
	if r.ContentLength != 0 && !decodeWorkoutJSON(w, r, 64<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.Complete(r.Context(), principal.UserID, workoutID, version, input)
	if err != nil {
		h.writeError(w, r, "complete_workout", err)
		return
	}
	w.Header().Set("ETag", workoutETag(value.ID, value.Version))
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	workoutID, ok := requestUUID(w, r, "Workout")
	if !ok {
		return
	}
	version, ok := expectedRootVersion(w, r, workoutID)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	if err := h.service.Delete(r.Context(), principal.UserID, workoutID, version); err != nil {
		h.writeError(w, r, "delete_workout", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) addExercise(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	workoutID, ok := requestUUID(w, r, "Workout")
	if !ok {
		return
	}
	version, ok := expectedRootVersion(w, r, workoutID)
	if !ok {
		return
	}
	var input ExerciseCreateInput
	if !decodeWorkoutJSON(w, r, 32<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, nextVersion, err := h.service.AddExercise(r.Context(), principal.UserID, workoutID, version, input)
	if err != nil {
		h.writeError(w, r, "add_workout_exercise", err)
		return
	}
	w.Header().Set("Location", "/api/v1/workout-exercises/"+value.ID)
	w.Header().Set("ETag", workoutETag(workoutID, nextVersion))
	httpserver.WriteData(w, r, http.StatusCreated, value)
}

func (h *Handler) patchExercise(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	itemID, ok := requestUUID(w, r, "Workout exercise")
	if !ok {
		return
	}
	workoutID, version, ok := expectedWorkoutVersion(w, r)
	if !ok {
		return
	}
	var input ExercisePatchInput
	if !decodeWorkoutJSON(w, r, 32<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, nextVersion, err := h.service.PatchExercise(r.Context(), principal.UserID, itemID, workoutID, version, input)
	if err != nil {
		h.writeError(w, r, "patch_workout_exercise", err)
		return
	}
	w.Header().Set("ETag", workoutETag(workoutID, nextVersion))
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func (h *Handler) deleteExercise(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	itemID, ok := requestUUID(w, r, "Workout exercise")
	if !ok {
		return
	}
	workoutID, version, ok := expectedWorkoutVersion(w, r)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	nextVersion, err := h.service.DeleteExercise(r.Context(), principal.UserID, itemID, workoutID, version)
	if err != nil {
		h.writeError(w, r, "delete_workout_exercise", err)
		return
	}
	w.Header().Set("ETag", workoutETag(workoutID, nextVersion))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) previousResult(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	itemID, ok := requestUUID(w, r, "Workout exercise")
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, err := h.service.PreviousResult(r.Context(), principal.UserID, itemID)
	if err != nil {
		h.writeError(w, r, "get_previous_exercise_result", err)
		return
	}
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func (h *Handler) addSet(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	itemID, ok := requestUUID(w, r, "Workout exercise")
	if !ok {
		return
	}
	workoutID, version, ok := expectedWorkoutVersion(w, r)
	if !ok {
		return
	}
	var input SetCreateInput
	if !decodeWorkoutJSON(w, r, 32<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, nextVersion, err := h.service.AddSet(r.Context(), principal.UserID, itemID, workoutID, version, input)
	if err != nil {
		h.writeError(w, r, "add_workout_set", err)
		return
	}
	w.Header().Set("Location", "/api/v1/workout-sets/"+value.ID)
	w.Header().Set("ETag", workoutETag(workoutID, nextVersion))
	httpserver.WriteData(w, r, http.StatusCreated, value)
}

func (h *Handler) patchSet(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	setID, ok := requestUUID(w, r, "Workout set")
	if !ok {
		return
	}
	workoutID, version, ok := expectedWorkoutVersion(w, r)
	if !ok {
		return
	}
	var input SetPatchInput
	if !decodeWorkoutJSON(w, r, 32<<10, &input) {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	value, nextVersion, err := h.service.PatchSet(r.Context(), principal.UserID, setID, workoutID, version, input)
	if err != nil {
		h.writeError(w, r, "patch_workout_set", err)
		return
	}
	w.Header().Set("ETag", workoutETag(workoutID, nextVersion))
	httpserver.WriteData(w, r, http.StatusOK, value)
}

func (h *Handler) deleteSet(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	setID, ok := requestUUID(w, r, "Workout set")
	if !ok {
		return
	}
	workoutID, version, ok := expectedWorkoutVersion(w, r)
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	nextVersion, err := h.service.DeleteSet(r.Context(), principal.UserID, setID, workoutID, version)
	if err != nil {
		h.writeError(w, r, "delete_workout_set", err)
		return
	}
	w.Header().Set("ETag", workoutETag(workoutID, nextVersion))
	w.WriteHeader(http.StatusNoContent)
}

func workoutListFilter(r *http.Request, export bool) (ListFilter, error) {
	allowed := map[string]struct{}{"status": {}, "from": {}, "to": {}, "program_id": {}, "exercise_id": {}}
	if !export {
		allowed["limit"] = struct{}{}
		allowed["cursor"] = struct{}{}
	}
	query := r.URL.Query()
	for key, values := range query {
		if _, ok := allowed[key]; !ok || len(values) != 1 || values[0] == "" {
			return ListFilter{}, ErrValidation
		}
	}
	filter := ListFilter{Status: query.Get("status"), ProgramID: query.Get("program_id"), ExerciseID: query.Get("exercise_id"), Limit: 50}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			return ListFilter{}, ErrValidation
		}
		filter.Limit = limit
	}
	for raw, destination := range map[string]**time.Time{"from": &filter.From, "to": &filter.To} {
		if value := query.Get(raw); value != "" {
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return ListFilter{}, ErrValidation
			}
			utc := parsed.UTC()
			*destination = &utc
		}
	}
	if err := validateListFilter(filter); err != nil {
		return ListFilter{}, err
	}
	if raw := query.Get("cursor"); raw != "" {
		cursor, err := DecodeCursor(raw, filter)
		if err != nil {
			return ListFilter{}, ErrValidation
		}
		filter.Cursor = &cursor
	}
	return filter, nil
}

func requestUUID(w http.ResponseWriter, r *http.Request, resource string) (string, bool) {
	value := chi.URLParam(r, "id")
	if !id.ValidUUID(value) {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_id", "Invalid identifier", resource+" id must be a UUID")
		return "", false
	}
	return value, true
}

func workoutETag(workoutID string, version int64) string {
	return fmt.Sprintf(`"workout:%s:%d"`, workoutID, version)
}

func expectedRootVersion(w http.ResponseWriter, r *http.Request, workoutID string) (int64, bool) {
	etagWorkoutID, version, ok := expectedWorkoutVersion(w, r)
	if !ok {
		return 0, false
	}
	if etagWorkoutID != workoutID {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_if_match", "Invalid If-Match", "If-Match must contain the current workout ETag")
		return 0, false
	}
	return version, true
}

func expectedWorkoutVersion(w http.ResponseWriter, r *http.Request) (string, int64, bool) {
	value := strings.TrimSpace(r.Header.Get("If-Match"))
	if value == "" {
		httpserver.WriteProblem(w, r, http.StatusPreconditionRequired, "precondition_required", "Precondition required", "If-Match with the current workout ETag is required")
		return "", 0, false
	}
	if !strings.HasPrefix(value, `"workout:`) || !strings.HasSuffix(value, `"`) || strings.Contains(value, ",") {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_if_match", "Invalid If-Match", "If-Match must contain one current workout ETag")
		return "", 0, false
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(value, `"workout:`), `"`), ":")
	if len(parts) != 2 || !id.ValidUUID(parts[0]) {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_if_match", "Invalid If-Match", "If-Match must contain one current workout ETag")
		return "", 0, false
	}
	version, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || version < 1 {
		httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_if_match", "Invalid If-Match", "If-Match must contain one current workout ETag")
		return "", 0, false
	}
	return parts[0], version, true
}

func decodeWorkoutJSON(w http.ResponseWriter, r *http.Request, limit int64, destination any) bool {
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
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", "Workout data does not satisfy the documented constraints")
	case errors.Is(err, ErrNotFound):
		httpserver.WriteProblem(w, r, http.StatusNotFound, "workout_not_found", "Not found", "Workout was not found")
	case errors.Is(err, ErrExerciseNotFound):
		httpserver.WriteProblem(w, r, http.StatusNotFound, "workout_exercise_not_found", "Not found", "Workout exercise was not found")
	case errors.Is(err, ErrSetNotFound):
		httpserver.WriteProblem(w, r, http.StatusNotFound, "workout_set_not_found", "Not found", "Workout set was not found")
	case errors.Is(err, ErrVersionConflict):
		httpserver.WriteProblem(w, r, http.StatusPreconditionFailed, "precondition_failed", "Precondition failed", "Workout changed; fetch it and retry with the current ETag")
	case errors.Is(err, ErrActiveExists):
		httpserver.WriteProblem(w, r, http.StatusConflict, "workout_already_in_progress", "Conflict", "Only one in-progress workout is allowed")
	case errors.Is(err, ErrInvalidState):
		httpserver.WriteProblem(w, r, http.StatusConflict, "invalid_workout_state", "Conflict", "Operation is not allowed in the current workout state")
	case errors.Is(err, ErrProgramUnavailable):
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "program_day_unavailable", "Program day unavailable", "Workout can only start from a current day of the active program")
	case errors.Is(err, ErrExerciseUnavailable):
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "exercise_unavailable", "Exercise unavailable", "Exercise must be visible and available to the current user")
	case errors.Is(err, ErrMetricNotTracked):
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "metric_not_tracked", "Metric not tracked", "Set contains a metric not supported by its exercise snapshot")
	case errors.Is(err, ErrExportTooLarge):
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "export_too_large", "Export too large", "Narrow the filters before exporting workout history")
	default:
		h.logger.ErrorContext(r.Context(), "workout operation failed", slog.String("operation", operation))
		httpserver.WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal server error", "Request could not be processed")
	}
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func writeInvalidQuery(w http.ResponseWriter, r *http.Request) {
	httpserver.WriteProblem(w, r, http.StatusBadRequest, "invalid_query", "Invalid query", "Workout filters must use the documented format")
}
