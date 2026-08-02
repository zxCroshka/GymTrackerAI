//go:build integration

package workout

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/auth"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/exercise"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/httpserver"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/program"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/user"
)

const (
	integrationBenchPressID = "00000000-0000-4000-8000-000000000002"
	integrationRunningID    = "00000000-0000-4000-8000-000000000017"
)

func TestWorkoutHTTPAndPostgreSQLFlow(t *testing.T) {
	server, _, service := newWorkoutIntegrationServer(t)
	client := server.Client()
	first := registerWorkoutUser(t, client, server.URL, "workout-first@example.com")
	second := registerWorkoutUser(t, client, server.URL, "workout-second@example.com")
	firstHeaders := workoutAuthHeaders(first.AccessToken)
	secondHeaders := workoutAuthHeaders(second.AccessToken)

	programBody := fmt.Sprintf(`{"name":"Workout source","days":[{"position":1,"name":"Upper A","exercises":[{"exercise_id":%q,"position":1,"working_sets":2,"target_reps_min":5,"target_reps_max":8,"target_rir":2,"rest_seconds":180}]}]}`, integrationBenchPressID)
	response := workoutRequest(t, client, http.MethodPost, server.URL+"/api/v1/programs", programBody, firstHeaders)
	requireWorkoutStatus(t, response, http.StatusCreated)
	programETag := response.Header.Get("ETag")
	var source dataEnvelope[program.Program]
	decodeWorkoutResponse(t, response, &source)
	activateHeaders := firstHeaders.Clone()
	activateHeaders.Set("If-Match", programETag)
	response = workoutRequest(t, client, http.MethodPost, server.URL+"/api/v1/programs/"+source.Data.ID+"/activate", "", activateHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	response.Body.Close()

	response = workoutRequest(t, client, http.MethodPost, server.URL+"/api/v1/workouts", fmt.Sprintf(`{"program_day_id":%q}`, source.Data.Days[0].ID), firstHeaders)
	requireWorkoutStatus(t, response, http.StatusCreated)
	workoutETagValue := response.Header.Get("ETag")
	var created dataEnvelope[Workout]
	decodeWorkoutResponse(t, response, &created)
	if created.Data.Status != "in_progress" || created.Data.SourceProgramID == nil || len(created.Data.Exercises) != 1 || len(created.Data.Exercises[0].Sets) != 2 {
		t.Fatalf("program workout = %+v", created.Data)
	}
	workoutID := created.Data.ID
	firstSetID := created.Data.Exercises[0].Sets[0].ID
	benchWorkoutExerciseID := created.Data.Exercises[0].ID

	response = workoutRequest(t, client, http.MethodGet, server.URL+"/api/v1/workouts/active", "", firstHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	if response.Header.Get("ETag") != workoutETagValue {
		t.Fatalf("active ETag = %q, want %q", response.Header.Get("ETag"), workoutETagValue)
	}
	response.Body.Close()
	response = workoutRequest(t, client, http.MethodPost, server.URL+"/api/v1/workouts", `{"name":"second active"}`, firstHeaders)
	requireWorkoutStatus(t, response, http.StatusConflict)
	response.Body.Close()

	response = workoutRequest(t, client, http.MethodGet, server.URL+"/api/v1/workouts/"+workoutID, "", secondHeaders)
	requireWorkoutStatus(t, response, http.StatusNotFound)
	response.Body.Close()
	foreignPatchHeaders := secondHeaders.Clone()
	foreignPatchHeaders.Set("If-Match", workoutETagValue)
	response = workoutRequest(t, client, http.MethodPatch, server.URL+"/api/v1/workouts/"+workoutID, `{"comment":"forbidden"}`, foreignPatchHeaders)
	requireWorkoutStatus(t, response, http.StatusNotFound)
	response.Body.Close()

	performedAt := created.Data.StartedAt.Add(time.Minute)
	setPatchHeaders := firstHeaders.Clone()
	setPatchHeaders.Set("If-Match", workoutETagValue)
	setPatchBody := fmt.Sprintf(`{"weight_kg":100,"reps":10,"rir":2,"performed_at":%q}`, performedAt.Format(time.RFC3339Nano))
	response = workoutRequest(t, client, http.MethodPatch, server.URL+"/api/v1/workout-sets/"+firstSetID, setPatchBody, setPatchHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	workoutETagValue = response.Header.Get("ETag")
	var recorded dataEnvelope[WorkoutSet]
	decodeWorkoutResponse(t, response, &recorded)
	if recorded.Data.VolumeKG == nil || *recorded.Data.VolumeKG != 1000 || recorded.Data.Estimated1RMKG == nil {
		t.Fatalf("recorded set metrics = %+v", recorded.Data)
	}

	warmupHeaders := firstHeaders.Clone()
	warmupHeaders.Set("If-Match", workoutETagValue)
	warmupBody := fmt.Sprintf(`{"set_number":1,"weight_kg":20,"reps":10,"warmup":true,"performed_at":%q}`, performedAt.Add(time.Minute).Format(time.RFC3339Nano))
	response = workoutRequest(t, client, http.MethodPost, server.URL+"/api/v1/workout-exercises/"+benchWorkoutExerciseID+"/sets", warmupBody, warmupHeaders)
	requireWorkoutStatus(t, response, http.StatusCreated)
	workoutETagValue = response.Header.Get("ETag")
	var warmup dataEnvelope[WorkoutSet]
	decodeWorkoutResponse(t, response, &warmup)

	addRunningHeaders := firstHeaders.Clone()
	addRunningHeaders.Set("If-Match", workoutETagValue)
	response = workoutRequest(t, client, http.MethodPost, server.URL+"/api/v1/workouts/"+workoutID+"/exercises", fmt.Sprintf(`{"exercise_id":%q,"position":1}`, integrationRunningID), addRunningHeaders)
	requireWorkoutStatus(t, response, http.StatusCreated)
	workoutETagValue = response.Header.Get("ETag")
	var running dataEnvelope[WorkoutExercise]
	decodeWorkoutResponse(t, response, &running)
	moveRunningHeaders := firstHeaders.Clone()
	moveRunningHeaders.Set("If-Match", workoutETagValue)
	response = workoutRequest(t, client, http.MethodPatch, server.URL+"/api/v1/workout-exercises/"+running.Data.ID, `{"position":2,"comment":"Easy pace"}`, moveRunningHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	workoutETagValue = response.Header.Get("ETag")
	decodeWorkoutResponse(t, response, &running)
	if running.Data.Position != 2 {
		t.Fatalf("moved exercise position = %d", running.Data.Position)
	}

	cardioHeaders := firstHeaders.Clone()
	cardioHeaders.Set("If-Match", workoutETagValue)
	cardioBody := fmt.Sprintf(`{"duration_seconds":600,"distance_meters":2000,"performed_at":%q}`, performedAt.Add(2*time.Minute).Format(time.RFC3339Nano))
	response = workoutRequest(t, client, http.MethodPost, server.URL+"/api/v1/workout-exercises/"+running.Data.ID+"/sets", cardioBody, cardioHeaders)
	requireWorkoutStatus(t, response, http.StatusCreated)
	workoutETagValue = response.Header.Get("ETag")
	response.Body.Close()

	unsupportedHeaders := firstHeaders.Clone()
	unsupportedHeaders.Set("If-Match", workoutETagValue)
	response = workoutRequest(t, client, http.MethodPost, server.URL+"/api/v1/workout-exercises/"+running.Data.ID+"/sets", `{"weight_kg":1}`, unsupportedHeaders)
	requireWorkoutStatus(t, response, http.StatusUnprocessableEntity)
	response.Body.Close()

	response = workoutRequest(t, client, http.MethodGet, server.URL+"/api/v1/workout-exercises/"+benchWorkoutExerciseID+"/previous-result", "", firstHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	var noPrevious dataEnvelope[*PreviousResult]
	decodeWorkoutResponse(t, response, &noPrevious)
	if noPrevious.Data != nil {
		t.Fatalf("previous result before history = %+v", noPrevious.Data)
	}

	completeHeaders := firstHeaders.Clone()
	completeHeaders.Set("If-Match", workoutETagValue)
	completedAt := created.Data.StartedAt.Add(time.Hour)
	completeBody := fmt.Sprintf(`{"completed_at":%q,"difficulty":8,"energy":7,"mood":9}`, completedAt.Format(time.RFC3339Nano))
	response = workoutRequest(t, client, http.MethodPost, server.URL+"/api/v1/workouts/"+workoutID+"/complete", completeBody, completeHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	completedETag := response.Header.Get("ETag")
	var completed dataEnvelope[Workout]
	decodeWorkoutResponse(t, response, &completed)
	if completed.Data.Status != "completed" || completed.Data.CompletedAt == nil || completed.Data.VolumeKG != 1000 || completed.Data.WorkingSetCount != 2 {
		t.Fatalf("completed workout = %+v", completed.Data)
	}

	staleCompletionHeaders := firstHeaders.Clone()
	staleCompletionHeaders.Set("If-Match", workoutETag(created.Data.ID, 1))
	response = workoutRequest(t, client, http.MethodPost, server.URL+"/api/v1/workouts/"+workoutID+"/complete", `{"difficulty":1}`, staleCompletionHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	if response.Header.Get("ETag") != completedETag {
		t.Fatalf("completion replay changed ETag: %q -> %q", completedETag, response.Header.Get("ETag"))
	}
	response.Body.Close()
	deleteWarmupHeaders := firstHeaders.Clone()
	deleteWarmupHeaders.Set("If-Match", completedETag)
	response = workoutRequest(t, client, http.MethodDelete, server.URL+"/api/v1/workout-sets/"+warmup.Data.ID, "", deleteWarmupHeaders)
	requireWorkoutStatus(t, response, http.StatusNoContent)
	completedETag = response.Header.Get("ETag")
	response.Body.Close()

	correctionHeaders := firstHeaders.Clone()
	correctionHeaders.Set("If-Match", completedETag)
	response = workoutRequest(t, client, http.MethodPatch, server.URL+"/api/v1/workout-sets/"+firstSetID, `{"reps":16}`, correctionHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	completedETag = response.Header.Get("ETag")
	decodeWorkoutResponse(t, response, &recorded)
	if recorded.Data.Estimated1RMKG != nil || recorded.Data.VolumeKG == nil || *recorded.Data.VolumeKG != 1600 {
		t.Fatalf("corrected set metrics = %+v", recorded.Data)
	}

	commentHeaders := firstHeaders.Clone()
	commentHeaders.Set("If-Match", completedETag)
	response = workoutRequest(t, client, http.MethodPatch, server.URL+"/api/v1/workouts/"+workoutID, `{"comment":"=HYPERLINK(\"bad\")"}`, commentHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	completedETag = response.Header.Get("ETag")
	response.Body.Close()

	response = workoutRequest(t, client, http.MethodGet, server.URL+"/api/v1/workouts?status=completed&exercise_id="+integrationBenchPressID, "", firstHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	var history collectionEnvelope[Workout]
	decodeWorkoutResponse(t, response, &history)
	if len(history.Data) != 1 || history.Data[0].ID != workoutID || history.Data[0].VolumeKG != 1600 {
		t.Fatalf("history = %+v", history.Data)
	}
	response = workoutRequest(t, client, http.MethodGet, server.URL+"/api/v1/workouts?status=completed", "", secondHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	decodeWorkoutResponse(t, response, &history)
	if len(history.Data) != 0 {
		t.Fatalf("second user's history leaked rows: %+v", history.Data)
	}

	response = workoutRequest(t, client, http.MethodGet, server.URL+"/api/v1/workouts/export.csv?status=completed", "", firstHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	if contentType := response.Header.Get("Content-Type"); contentType != "text/csv; charset=utf-8" {
		t.Fatalf("CSV content type = %q", contentType)
	}
	csvRows, err := csv.NewReader(response.Body).ReadAll()
	response.Body.Close()
	if err != nil {
		t.Fatalf("read CSV: %v", err)
	}
	commentColumn := csvColumn(t, csvRows[0], "comment")
	if len(csvRows) < 2 || csvRows[1][commentColumn] == "" || csvRows[1][commentColumn][0] != '\'' {
		t.Fatalf("CSV comment was not formula-safe: %+v", csvRows)
	}

	response = workoutRequest(t, client, http.MethodPost, server.URL+"/api/v1/workouts", `{"name":"Ad hoc follow-up"}`, firstHeaders)
	requireWorkoutStatus(t, response, http.StatusCreated)
	adHocETag := response.Header.Get("ETag")
	var adHoc dataEnvelope[Workout]
	decodeWorkoutResponse(t, response, &adHoc)
	adHocExerciseHeaders := firstHeaders.Clone()
	adHocExerciseHeaders.Set("If-Match", adHocETag)
	response = workoutRequest(t, client, http.MethodPost, server.URL+"/api/v1/workouts/"+adHoc.Data.ID+"/exercises", fmt.Sprintf(`{"exercise_id":%q}`, integrationBenchPressID), adHocExerciseHeaders)
	requireWorkoutStatus(t, response, http.StatusCreated)
	adHocETag = response.Header.Get("ETag")
	var adHocBench dataEnvelope[WorkoutExercise]
	decodeWorkoutResponse(t, response, &adHocBench)
	response = workoutRequest(t, client, http.MethodGet, server.URL+"/api/v1/workout-exercises/"+adHocBench.Data.ID+"/previous-result", "", firstHeaders)
	requireWorkoutStatus(t, response, http.StatusOK)
	var previous dataEnvelope[*PreviousResult]
	decodeWorkoutResponse(t, response, &previous)
	if previous.Data == nil || previous.Data.SourceWorkoutID != workoutID || len(previous.Data.Sets) != 1 {
		t.Fatalf("previous result = %+v", previous.Data)
	}

	foreignDeleteHeaders := secondHeaders.Clone()
	foreignDeleteHeaders.Set("If-Match", adHocETag)
	response = workoutRequest(t, client, http.MethodDelete, server.URL+"/api/v1/workouts/"+adHoc.Data.ID, "", foreignDeleteHeaders)
	requireWorkoutStatus(t, response, http.StatusNotFound)
	response.Body.Close()
	deleteAdHocExerciseHeaders := firstHeaders.Clone()
	deleteAdHocExerciseHeaders.Set("If-Match", adHocETag)
	response = workoutRequest(t, client, http.MethodDelete, server.URL+"/api/v1/workout-exercises/"+adHocBench.Data.ID, "", deleteAdHocExerciseHeaders)
	requireWorkoutStatus(t, response, http.StatusNoContent)
	adHocETag = response.Header.Get("ETag")
	response.Body.Close()
	deleteHeaders := firstHeaders.Clone()
	deleteHeaders.Set("If-Match", adHocETag)
	response = workoutRequest(t, client, http.MethodDelete, server.URL+"/api/v1/workouts/"+adHoc.Data.ID, "", deleteHeaders)
	requireWorkoutStatus(t, response, http.StatusNoContent)
	response.Body.Close()

	testConcurrentActiveWorkoutCreation(t, service, first.UserID)
}

func testConcurrentActiveWorkoutCreation(t *testing.T, service *Service, actorID string) {
	t.Helper()
	var wait sync.WaitGroup
	wait.Add(2)
	errorsChannel := make(chan error, 2)
	for _, name := range []string{"Concurrent A", "Concurrent B"} {
		name := name
		go func() {
			defer wait.Done()
			_, err := service.Create(context.Background(), actorID, CreateInput{Name: &name})
			errorsChannel <- err
		}()
	}
	wait.Wait()
	close(errorsChannel)
	successes, conflicts := 0, 0
	for err := range errorsChannel {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrActiveExists):
			conflicts++
		default:
			t.Fatalf("concurrent create error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent creates: successes=%d conflicts=%d", successes, conflicts)
	}
}

func newWorkoutIntegrationServer(t *testing.T) (*httptest.Server, *pgxpool.Pool, *Service) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required and must point to a disposable PostgreSQL database")
	}
	migrationsPath, err := filepath.Abs("../../migrations")
	if err != nil {
		t.Fatalf("resolve migrations: %v", err)
	}
	migrator, err := database.NewMigrator(databaseURL, "file://"+filepath.ToSlash(migrationsPath))
	if err != nil {
		t.Fatalf("new migrator: %v", err)
	}
	if err := migrator.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) && !errors.Is(err, migrate.ErrNilVersion) {
		t.Fatalf("reset database: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	pool, err := database.OpenPool(context.Background(), workoutIntegrationDatabaseConfig(databaseURL))
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM users"); err != nil {
			t.Errorf("clean users before rollback: %v", err)
		}
		pool.Close()
		if err := migrator.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			t.Errorf("migrate down: %v", err)
		}
		_, _ = migrator.Close()
	})
	if _, err := database.SeedSystemExercises(context.Background(), pool); err != nil {
		t.Fatalf("seed system exercises: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	authConfig := workoutIntegrationAuthConfig()
	userRepository := user.NewRepository(pool)
	authService, err := auth.NewService(pool, auth.NewRepository(pool), userRepository, auth.NewTokenManager(authConfig))
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	exerciseService := exercise.NewService(pool, exercise.NewRepository(pool))
	programService := program.NewService(pool, program.NewRepository(pool), exerciseService)
	derived := workoutIntegrationDerived{}
	workoutService := NewService(pool, NewRepository(pool), programService, exerciseService, derived, derived)
	authHandler := auth.NewHandler(authService, authConfig, logger)
	programHandler := program.NewHandler(programService, logger)
	workoutHandler := NewHandler(workoutService, logger)
	readiness := httpserver.NewReadiness(pool, time.Second)
	readiness.SetReady(true)
	server := httptest.NewServer(httpserver.NewHandler(logger, readiness, func(router chi.Router) {
		authHandler.RegisterRoutes(router)
		router.Group(func(private chi.Router) {
			private.Use(authService.Middleware)
			programHandler.RegisterRoutes(private)
			workoutHandler.RegisterRoutes(private)
		})
	}))
	t.Cleanup(server.Close)
	return server, pool, workoutService
}

type workoutIntegrationDerived struct{}

func (workoutIntegrationDerived) RebuildUser(context.Context, pgx.Tx, string, time.Time) error {
	return nil
}
func (workoutIntegrationDerived) LockUser(context.Context, pgx.Tx, string) error { return nil }
func (workoutIntegrationDerived) MarkPeriodsStale(context.Context, pgx.Tx, string, []time.Time, time.Time) error {
	return nil
}

type registeredWorkoutUser struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
}

type dataEnvelope[T any] struct {
	Data T `json:"data"`
}

type collectionEnvelope[T any] struct {
	Data []T `json:"data"`
	Meta struct {
		NextCursor *string `json:"next_cursor"`
	} `json:"meta"`
}

func registerWorkoutUser(t *testing.T, client *http.Client, baseURL, email string) registeredWorkoutUser {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":"long-secure-password"}`, email)
	response := workoutRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", body, http.Header{"Content-Type": []string{"application/json"}})
	requireWorkoutStatus(t, response, http.StatusCreated)
	var envelope dataEnvelope[registeredWorkoutUser]
	decodeWorkoutResponse(t, response, &envelope)
	return envelope.Data
}

func workoutAuthHeaders(accessToken string) http.Header {
	return http.Header{"Authorization": []string{"Bearer " + accessToken}, "Content-Type": []string{"application/json"}}
}

func workoutRequest(t *testing.T, client *http.Client, method, target, body string, headers http.Header) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, target, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	return response
}

func requireWorkoutStatus(t *testing.T, response *http.Response, expected int) {
	t.Helper()
	if response.StatusCode != expected {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, expected, body)
	}
}

func decodeWorkoutResponse(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func csvColumn(t *testing.T, header []string, name string) int {
	t.Helper()
	for index, value := range header {
		if value == name {
			return index
		}
	}
	t.Fatalf("CSV column %q missing from %+v", name, header)
	return -1
}

func workoutIntegrationDatabaseConfig(url string) config.DatabaseConfig {
	return config.DatabaseConfig{
		URL: url, ConnectTimeout: 5 * time.Second, PingTimeout: 2 * time.Second,
		MinConnections: 1, MaxConnections: 8, MaxConnLifetime: 10 * time.Minute,
		MaxConnIdleTime: time.Minute, HealthCheckPeriod: 30 * time.Second,
	}
}

func workoutIntegrationAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		AccessSecret:  "workout-integration-access-secret-at-least-32-characters",
		RefreshSecret: "workout-integration-refresh-secret-at-least-32-characters",
		Issuer:        "gymtracker-workout-test", AccessAudience: "gymtracker-web",
		RefreshAudience: "gymtracker-refresh", AccessTTL: 15 * time.Minute,
		RefreshTTL: 24 * time.Hour, CookieName: "gymtracker_refresh",
		AllowedOrigins: []string{"http://localhost:3000"}, RateLimit: 20, RateWindow: time.Minute,
	}
}
