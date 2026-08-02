//go:build integration

package program

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/auth"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/exercise"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/httpserver"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/user"
)

const benchPressID = "00000000-0000-4000-8000-000000000002"

func TestExerciseAndProgramHTTPFlow(t *testing.T) {
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
	defer func() { _, _ = migrator.Close() }()
	if err := migrator.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) && !errors.Is(err, migrate.ErrNilVersion) {
		t.Fatalf("reset database: %v", err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	defer func() {
		if err := migrator.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			t.Errorf("migrate down: %v", err)
		}
	}()

	pool, err := database.OpenPool(context.Background(), programIntegrationDatabaseConfig(databaseURL))
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()
	if _, err := database.SeedSystemExercises(context.Background(), pool); err != nil {
		t.Fatalf("seed exercises: %v", err)
	}
	if _, err := database.SeedSystemExercises(context.Background(), pool); err != nil {
		t.Fatalf("repeat exercise seed: %v", err)
	}
	var systemCount int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM exercises WHERE owner_user_id IS NULL").Scan(&systemCount); err != nil {
		t.Fatalf("count system exercises: %v", err)
	}
	if systemCount < 19 {
		t.Fatalf("system exercise count = %d, want at least 19", systemCount)
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	authConfig := programIntegrationAuthConfig()
	userRepository := user.NewRepository(pool)
	authService, err := auth.NewService(pool, auth.NewRepository(pool), userRepository, auth.NewTokenManager(authConfig))
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	exerciseService := exercise.NewService(pool, exercise.NewRepository(pool))
	programService := NewService(pool, NewRepository(pool), exerciseService)
	authHandler := auth.NewHandler(authService, authConfig, logger)
	exerciseHandler := exercise.NewHandler(exerciseService, logger)
	programHandler := NewHandler(programService, logger)
	readiness := httpserver.NewReadiness(pool, time.Second)
	readiness.SetReady(true)
	server := httptest.NewServer(httpserver.NewHandler(logger, readiness, func(router chi.Router) {
		authHandler.RegisterRoutes(router)
		router.Group(func(private chi.Router) {
			private.Use(authService.Middleware)
			exerciseHandler.RegisterRoutes(private)
			programHandler.RegisterRoutes(private)
		})
	}))
	defer server.Close()
	client := server.Client()

	first := registerProgramUser(t, client, server.URL, "first@example.com")
	second := registerProgramUser(t, client, server.URL, "second@example.com")
	firstHeaders := authorizedProgramHeaders(first.AccessToken)
	secondHeaders := authorizedProgramHeaders(second.AccessToken)

	response := programRequest(t, client, http.MethodGet, server.URL+"/api/v1/exercises?type=strength&limit=2", "", firstHeaders)
	requireProgramStatus(t, response, http.StatusOK)
	var firstPage collectionEnvelope[exercise.Exercise]
	decodeProgramResponse(t, response, &firstPage)
	if len(firstPage.Data) != 2 || firstPage.Meta.NextCursor == nil {
		t.Fatalf("exercise page = %+v", firstPage)
	}
	response = programRequest(t, client, http.MethodGet, server.URL+"/api/v1/exercises?type=strength&limit=2&cursor="+*firstPage.Meta.NextCursor, "", firstHeaders)
	requireProgramStatus(t, response, http.StatusOK)
	response.Body.Close()

	response = programRequest(t, client, http.MethodGet, server.URL+"/api/v1/exercises/"+benchPressID, "", firstHeaders)
	requireProgramStatus(t, response, http.StatusOK)
	systemETag := response.Header.Get("ETag")
	response.Body.Close()
	systemPatchHeaders := firstHeaders.Clone()
	systemPatchHeaders.Set("If-Match", systemETag)
	response = programRequest(t, client, http.MethodPatch, server.URL+"/api/v1/exercises/"+benchPressID, `{"name":"Новый жим"}`, systemPatchHeaders)
	requireProgramStatus(t, response, http.StatusForbidden)
	response.Body.Close()

	customBody := `{"name":"Тяга саней","description":"Пользовательское упражнение","primary_muscle_group":"full_body","exercise_type":"strength","equipment":"other","is_unilateral":false,"tracks_weight":true,"tracks_repetitions":false,"tracks_time":false,"tracks_distance":true}`
	response = programRequest(t, client, http.MethodPost, server.URL+"/api/v1/exercises", customBody, firstHeaders)
	requireProgramStatus(t, response, http.StatusCreated)
	customETag := response.Header.Get("ETag")
	var custom dataEnvelope[exercise.Exercise]
	decodeProgramResponse(t, response, &custom)
	if custom.Data.OwnerUserID == nil || *custom.Data.OwnerUserID != first.UserID {
		t.Fatalf("custom exercise owner = %+v", custom.Data.OwnerUserID)
	}
	response = programRequest(t, client, http.MethodGet, server.URL+"/api/v1/exercises/"+custom.Data.ID, "", secondHeaders)
	requireProgramStatus(t, response, http.StatusNotFound)
	response.Body.Close()

	invalidProgramBody := fmt.Sprintf(`{"name":"Неверный порядок","days":[{"position":2,"name":"День","exercises":[{"exercise_id":%q,"position":1,"working_sets":3}]}]}`, benchPressID)
	response = programRequest(t, client, http.MethodPost, server.URL+"/api/v1/programs", invalidProgramBody, firstHeaders)
	requireProgramStatus(t, response, http.StatusUnprocessableEntity)
	response.Body.Close()

	programBody := fmt.Sprintf(`{"name":"Сила 2 дня","description":"Основная программа","goal":"strength","days":[{"position":1,"name":"Верх","notes":"Тяжёлый день","exercises":[{"exercise_id":%q,"position":1,"working_sets":4,"target_reps_min":5,"target_reps_max":8,"target_rir":2,"rest_seconds":180},{"exercise_id":%q,"position":2,"working_sets":3,"target_reps_min":10,"target_reps_max":15,"target_rir":1.5,"rest_seconds":90}]}]}`, benchPressID, custom.Data.ID)
	response = programRequest(t, client, http.MethodPost, server.URL+"/api/v1/programs", programBody, firstHeaders)
	requireProgramStatus(t, response, http.StatusCreated)
	programETagValue := response.Header.Get("ETag")
	var original dataEnvelope[Program]
	decodeProgramResponse(t, response, &original)
	oldDayID := original.Data.Days[0].ID
	oldItemID := original.Data.Days[0].Exercises[0].ID
	response = programRequest(t, client, http.MethodGet, server.URL+"/api/v1/programs/"+original.Data.ID, "", secondHeaders)
	requireProgramStatus(t, response, http.StatusNotFound)
	response.Body.Close()

	response = programRequest(t, client, http.MethodPost, server.URL+"/api/v1/programs/"+original.Data.ID+"/duplicate", "", firstHeaders)
	requireProgramStatus(t, response, http.StatusCreated)
	duplicateETag := response.Header.Get("ETag")
	var duplicated dataEnvelope[Program]
	decodeProgramResponse(t, response, &duplicated)
	if duplicated.Data.Status != "draft" || duplicated.Data.ID == original.Data.ID ||
		duplicated.Data.Days[0].ID == oldDayID || duplicated.Data.Days[0].Exercises[0].ID == oldItemID {
		t.Fatalf("duplicated program = %+v", duplicated.Data)
	}

	activateOriginalHeaders := firstHeaders.Clone()
	activateOriginalHeaders.Set("If-Match", programETagValue)
	response = programRequest(t, client, http.MethodPost, server.URL+"/api/v1/programs/"+original.Data.ID+"/activate", "", activateOriginalHeaders)
	requireProgramStatus(t, response, http.StatusOK)
	response.Body.Close()
	activateDuplicateHeaders := firstHeaders.Clone()
	activateDuplicateHeaders.Set("If-Match", duplicateETag)
	response = programRequest(t, client, http.MethodPost, server.URL+"/api/v1/programs/"+duplicated.Data.ID+"/activate", "", activateDuplicateHeaders)
	requireProgramStatus(t, response, http.StatusOK)
	activeDuplicateETag := response.Header.Get("ETag")
	response.Body.Close()
	assertOnlyActiveProgram(t, pool, first.UserID, duplicated.Data.ID)
	invalidActivePatchHeaders := firstHeaders.Clone()
	invalidActivePatchHeaders.Set("If-Match", activeDuplicateETag)
	response = programRequest(t, client, http.MethodPatch, server.URL+"/api/v1/programs/"+duplicated.Data.ID, `{"days":[]}`, invalidActivePatchHeaders)
	requireProgramStatus(t, response, http.StatusConflict)
	response.Body.Close()
	assertOnlyActiveProgram(t, pool, first.UserID, duplicated.Data.ID)

	response = programRequest(t, client, http.MethodPost, server.URL+"/api/v1/programs", programBody, firstHeaders)
	requireProgramStatus(t, response, http.StatusCreated)
	candidateETag := response.Header.Get("ETag")
	var candidate dataEnvelope[Program]
	decodeProgramResponse(t, response, &candidate)
	response = programRequest(t, client, http.MethodPost, server.URL+"/api/v1/programs", programBody, firstHeaders)
	requireProgramStatus(t, response, http.StatusCreated)
	var concurrentTarget dataEnvelope[Program]
	decodeProgramResponse(t, response, &concurrentTarget)
	activationErrors := make(chan error, 2)
	for _, targetID := range []string{candidate.Data.ID, concurrentTarget.Data.ID} {
		targetID := targetID
		go func() {
			_, err := programService.Activate(context.Background(), first.UserID, targetID, 1)
			activationErrors <- err
		}()
	}
	for range 2 {
		if err := <-activationErrors; err != nil {
			t.Fatalf("concurrent activation: %v", err)
		}
	}
	assertActiveProgramCount(t, pool, first.UserID, 1)
	currentDuplicate, err := programService.Get(context.Background(), first.UserID, duplicated.Data.ID)
	if err != nil {
		t.Fatalf("get duplicate after concurrent activation: %v", err)
	}
	if _, err := programService.Activate(context.Background(), first.UserID, duplicated.Data.ID, currentDuplicate.Version); err != nil {
		t.Fatalf("restore active duplicate: %v", err)
	}
	assertOnlyActiveProgram(t, pool, first.UserID, duplicated.Data.ID)
	currentCandidate, err := programService.Get(context.Background(), first.UserID, candidate.Data.ID)
	if err != nil {
		t.Fatalf("get candidate after concurrent activation: %v", err)
	}
	candidateETag = programETag(currentCandidate)

	archiveExerciseHeaders := firstHeaders.Clone()
	archiveExerciseHeaders.Set("If-Match", customETag)
	response = programRequest(t, client, http.MethodDelete, server.URL+"/api/v1/exercises/"+custom.Data.ID, "", archiveExerciseHeaders)
	requireProgramStatus(t, response, http.StatusNoContent)
	response.Body.Close()
	response = programRequest(t, client, http.MethodGet, server.URL+"/api/v1/programs/"+original.Data.ID, "", firstHeaders)
	requireProgramStatus(t, response, http.StatusOK)
	response.Body.Close()

	activateCandidateHeaders := firstHeaders.Clone()
	activateCandidateHeaders.Set("If-Match", candidateETag)
	response = programRequest(t, client, http.MethodPost, server.URL+"/api/v1/programs/"+candidate.Data.ID+"/activate", "", activateCandidateHeaders)
	requireProgramStatus(t, response, http.StatusUnprocessableEntity)
	response.Body.Close()
	assertOnlyActiveProgram(t, pool, first.UserID, duplicated.Data.ID)

	response = programRequest(t, client, http.MethodGet, server.URL+"/api/v1/programs/"+original.Data.ID, "", firstHeaders)
	requireProgramStatus(t, response, http.StatusOK)
	currentOriginalETag := response.Header.Get("ETag")
	response.Body.Close()
	replacementBody := fmt.Sprintf(`{"days":[{"position":1,"name":"Новый верх","exercises":[{"exercise_id":%q,"position":1,"working_sets":5,"target_reps_min":3,"target_reps_max":5,"target_rir":2,"rest_seconds":240}]}]}`, benchPressID)
	patchProgramHeaders := firstHeaders.Clone()
	patchProgramHeaders.Set("If-Match", currentOriginalETag)
	response = programRequest(t, client, http.MethodPatch, server.URL+"/api/v1/programs/"+original.Data.ID, replacementBody, patchProgramHeaders)
	requireProgramStatus(t, response, http.StatusOK)
	archivableProgramETag := response.Header.Get("ETag")
	response.Body.Close()
	var archivedDayAt, archivedItemAt *time.Time
	if err := pool.QueryRow(context.Background(), "SELECT archived_at FROM program_days WHERE id = $1", oldDayID).Scan(&archivedDayAt); err != nil {
		t.Fatalf("inspect replaced day: %v", err)
	}
	if err := pool.QueryRow(context.Background(), "SELECT archived_at FROM program_day_exercises WHERE id = $1", oldItemID).Scan(&archivedItemAt); err != nil {
		t.Fatalf("inspect replaced item: %v", err)
	}
	if archivedDayAt == nil || archivedItemAt == nil {
		t.Fatalf("replaced tree was not archived: day=%v item=%v", archivedDayAt, archivedItemAt)
	}

	archiveProgramHeaders := firstHeaders.Clone()
	archiveProgramHeaders.Set("If-Match", archivableProgramETag)
	response = programRequest(t, client, http.MethodDelete, server.URL+"/api/v1/programs/"+original.Data.ID, "", archiveProgramHeaders)
	requireProgramStatus(t, response, http.StatusNoContent)
	response.Body.Close()
	var retainedDays int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM program_days WHERE program_id = $1", original.Data.ID).Scan(&retainedDays); err != nil {
		t.Fatalf("count retained days: %v", err)
	}
	if retainedDays < 2 {
		t.Fatalf("retained program days = %d, want old and current trees", retainedDays)
	}
}

type registeredProgramUser struct {
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

func registerProgramUser(t *testing.T, client *http.Client, baseURL, email string) registeredProgramUser {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":"long-secure-password"}`, email)
	response := programRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/register", body, http.Header{"Content-Type": []string{"application/json"}})
	requireProgramStatus(t, response, http.StatusCreated)
	var envelope dataEnvelope[registeredProgramUser]
	decodeProgramResponse(t, response, &envelope)
	return envelope.Data
}

func authorizedProgramHeaders(accessToken string) http.Header {
	return http.Header{
		"Authorization": []string{"Bearer " + accessToken},
		"Content-Type":  []string{"application/json"},
	}
}

func programRequest(t *testing.T, client *http.Client, method, target, body string, headers http.Header) *http.Response {
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

func requireProgramStatus(t *testing.T, response *http.Response, expected int) {
	t.Helper()
	if response.StatusCode != expected {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, expected, body)
	}
}

func decodeProgramResponse(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func assertOnlyActiveProgram(t *testing.T, pool *pgxpool.Pool, userID, expectedID string) {
	t.Helper()
	var count int
	var activeID string
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*), coalesce(min(id::text), '')
		FROM programs WHERE user_id = $1 AND status = 'active'`, userID).Scan(&count, &activeID); err != nil {
		t.Fatalf("inspect active programs: %v", err)
	}
	if count != 1 || activeID != expectedID {
		t.Fatalf("active programs = %d, id=%q, want one %q", count, activeID, expectedID)
	}
}

func assertActiveProgramCount(t *testing.T, pool *pgxpool.Pool, userID string, expected int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM programs WHERE user_id = $1 AND status = 'active'", userID).Scan(&count); err != nil {
		t.Fatalf("count active programs: %v", err)
	}
	if count != expected {
		t.Fatalf("active program count = %d, want %d", count, expected)
	}
}

func programIntegrationDatabaseConfig(url string) config.DatabaseConfig {
	return config.DatabaseConfig{
		URL: url, ConnectTimeout: 5 * time.Second, PingTimeout: 2 * time.Second,
		MinConnections: 1, MaxConnections: 4, MaxConnLifetime: 10 * time.Minute,
		MaxConnIdleTime: time.Minute, HealthCheckPeriod: 30 * time.Second,
	}
}

func programIntegrationAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		AccessSecret:  "program-integration-access-secret-at-least-32-characters",
		RefreshSecret: "program-integration-refresh-secret-at-least-32-characters",
		Issuer:        "gymtracker-program-test", AccessAudience: "gymtracker-web",
		RefreshAudience: "gymtracker-refresh", AccessTTL: 15 * time.Minute,
		RefreshTTL: 24 * time.Hour, CookieName: "gymtracker_refresh",
		AllowedOrigins: []string{"http://localhost:3000"}, RateLimit: 20, RateWindow: time.Minute,
	}
}
