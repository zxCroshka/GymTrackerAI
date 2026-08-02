//go:build integration

package report

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
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/auth"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/exercise"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/measurement"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/calendartime"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/httpserver"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/program"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/progress"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/user"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/workout"
)

const reportBenchPressID = "00000000-0000-4000-8000-000000000002"

func TestMeasurementProgressAndWeeklyReportFlow(t *testing.T) {
	server, pool, workouts, reports := newAnalyticsIntegrationServer(t)
	client := server.Client()
	first := registerAnalyticsUser(t, client, server.URL, "analytics-first@example.com")
	second := registerAnalyticsUser(t, client, server.URL, "analytics-second@example.com")
	if _, err := pool.Exec(context.Background(), `UPDATE user_profiles SET timezone='Europe/Moscow' WHERE user_id=$1`, first.UserID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	weekStart, weekEnd, err := calendartime.WeekContaining(now, "Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	previousStart, _, _ := calendartime.WeekContaining(weekStart.Add(-time.Nanosecond), "Europe/Moscow")
	previous := completeAnalyticsWorkout(t, workouts, first.UserID, previousStart.Add(12*time.Hour), 80, 10, false)
	_ = previous
	current := completeAnalyticsWorkout(t, workouts, first.UserID, weekStart.Add(12*time.Hour), 100, 10, true)
	plannedName := "Следующая тренировка"
	plannedAt := now.Add(24 * time.Hour)
	planned, err := workouts.Create(context.Background(), first.UserID, workout.CreateInput{Name: &plannedName, Status: "planned", ScheduledAt: &plannedAt})
	if err != nil {
		t.Fatalf("create planned workout: %v", err)
	}

	measurementTimes := []time.Time{now.Add(-31 * 24 * time.Hour), now.Add(-8 * 24 * time.Hour), now.Add(-time.Minute)}
	weights := []float64{70, 71, 72}
	var latestMeasurement measurement.BodyMeasurement
	var latestETag string
	for index, at := range measurementTimes {
		response := analyticsRequest(t, client, http.MethodPost, server.URL+"/api/v1/measurements", fmt.Sprintf(`{"measured_at":%q,"weight_kg":%v}`, at.Format(time.RFC3339), weights[index]), analyticsHeaders(first.AccessToken))
		requireAnalyticsStatus(t, response, http.StatusCreated)
		latestETag = response.Header.Get("ETag")
		decodeAnalytics(t, response, &struct {
			Data *measurement.BodyMeasurement `json:"data"`
		}{Data: &latestMeasurement})
	}
	observed := weekStart.Add(20 * time.Hour)
	response := analyticsRequest(t, client, http.MethodPost, server.URL+"/api/v1/wellness", fmt.Sprintf(`{"observed_at":%q,"sleep_minutes":480,"sleep_quality":5,"energy":4,"steps":10000,"calories_kcal":2400,"protein_g":160,"fat_g":70,"carbs_g":280}`, observed.Format(time.RFC3339)), analyticsHeaders(first.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusCreated)
	response.Body.Close()
	response = analyticsRequest(t, client, http.MethodPost, server.URL+"/api/v1/wellness", fmt.Sprintf(`{"observed_at":%q,"energy":3}`, observed.Add(time.Hour).Format(time.RFC3339)), analyticsHeaders(first.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusConflict)
	response.Body.Close()
	response = analyticsRequest(t, client, http.MethodGet, server.URL+"/api/v1/measurements?from="+now.Add(-40*24*time.Hour).Format(time.RFC3339)+"&to="+now.Add(time.Minute).Format(time.RFC3339), "", analyticsHeaders(first.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusOK)
	var listedMeasurements struct {
		Data []measurement.BodyMeasurement `json:"data"`
	}
	decodeAnalytics(t, response, &listedMeasurements)
	if len(listedMeasurements.Data) != 3 {
		t.Fatalf("listed measurements=%+v", listedMeasurements.Data)
	}
	response = analyticsRequest(t, client, http.MethodGet, server.URL+"/api/v1/wellness", "", analyticsHeaders(first.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusOK)
	var listedWellness struct {
		Data []measurement.WellnessEntry `json:"data"`
	}
	decodeAnalytics(t, response, &listedWellness)
	if len(listedWellness.Data) != 1 {
		t.Fatalf("listed wellness=%+v", listedWellness.Data)
	}

	response = analyticsRequest(t, client, http.MethodGet, server.URL+"/api/v1/progress/dashboard", "", analyticsHeaders(first.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusOK)
	var dashboard struct {
		Data progress.Dashboard `json:"data"`
	}
	decodeAnalytics(t, response, &dashboard)
	if dashboard.Data.WorkoutsThisWeek != 1 || dashboard.Data.WeeklyVolumeKG != 1000 || dashboard.Data.TotalVolumeKG != 1800 || dashboard.Data.Weight.CurrentKG == nil || *dashboard.Data.Weight.CurrentKG != 72 || dashboard.Data.NextPlannedWorkout == nil || dashboard.Data.NextPlannedWorkout.ID != planned.ID {
		t.Fatalf("dashboard=%+v", dashboard.Data)
	}
	response = analyticsRequest(t, client, http.MethodGet, server.URL+"/api/v1/progress/dashboard", "", analyticsHeaders(second.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusOK)
	var emptyDashboard struct {
		Data progress.Dashboard `json:"data"`
	}
	decodeAnalytics(t, response, &emptyDashboard)
	if emptyDashboard.Data.Weight.CurrentKG != nil || emptyDashboard.Data.WorkoutsThisWeek != 0 || emptyDashboard.Data.TotalVolumeKG != 0 || emptyDashboard.Data.WeeklyVolumeKG != 0 || len(emptyDashboard.Data.NewAchievements) != 0 {
		t.Fatalf("empty dashboard=%+v", emptyDashboard.Data)
	}
	response = analyticsRequest(t, client, http.MethodGet, server.URL+"/api/v1/progress/weight?from="+now.Add(-40*24*time.Hour).Format(time.RFC3339)+"&to="+now.Add(time.Minute).Format(time.RFC3339), "", analyticsHeaders(first.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusOK)
	var weightProgress struct {
		Data progress.WeightProgress `json:"data"`
	}
	decodeAnalytics(t, response, &weightProgress)
	if len(weightProgress.Data.Points) != 3 || weightProgress.Data.Summary.CurrentKG == nil || *weightProgress.Data.Summary.CurrentKG != 72 {
		t.Fatalf("weight progress=%+v", weightProgress.Data)
	}
	response = analyticsRequest(t, client, http.MethodGet, server.URL+"/api/v1/progress/exercises/"+reportBenchPressID+"?from="+previousStart.Format(time.RFC3339)+"&to="+weekEnd.Format(time.RFC3339), "", analyticsHeaders(first.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusOK)
	var exerciseProgress struct {
		Data progress.ExerciseProgress `json:"data"`
	}
	decodeAnalytics(t, response, &exerciseProgress)
	if len(exerciseProgress.Data.Points) != 2 || exerciseProgress.Data.Points[1].BestEstimated1RMKG == nil {
		t.Fatalf("exercise progress=%+v", exerciseProgress.Data)
	}
	response = analyticsRequest(t, client, http.MethodGet, server.URL+"/api/v1/progress/personal-records?record_type=max_reps", "", analyticsHeaders(first.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusOK)
	var records struct {
		Data []progress.PersonalRecord `json:"data"`
	}
	decodeAnalytics(t, response, &records)
	if len(records.Data) != 2 {
		t.Fatalf("weight-specific max-reps records=%+v", records.Data)
	}

	response = analyticsRequest(t, client, http.MethodPost, server.URL+"/api/v1/reports/weekly", fmt.Sprintf(`{"week_containing_at":%q}`, now.Format(time.RFC3339)), analyticsHeaders(first.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusCreated)
	var generated struct {
		Data WeeklyReport `json:"data"`
	}
	decodeAnalytics(t, response, &generated)
	if generated.Data.Status != "ready" || generated.Data.Metrics == nil || generated.Data.Metrics.Totals.VolumeKG != 1000 || generated.Data.Metrics.PreviousWeek.VolumeChangePercent == nil || *generated.Data.Metrics.PreviousWeek.VolumeChangePercent != 25 || generated.Data.Metrics.Wellness.AverageSleepHours == nil || *generated.Data.Metrics.Wellness.AverageSleepHours != 8 || len(generated.Data.Metrics.PainMessages) != 1 || len(generated.Data.Metrics.NewRecords) == 0 {
		t.Fatalf("weekly report=%+v", generated.Data)
	}
	reportID := generated.Data.ID
	type generateResult struct {
		report  WeeklyReport
		created bool
		err     error
	}
	generatedConcurrently := make([]generateResult, 8)
	startGenerate := make(chan struct{})
	var generateGroup sync.WaitGroup
	for index := range generatedConcurrently {
		generateGroup.Add(1)
		go func(index int) {
			defer generateGroup.Done()
			<-startGenerate
			generatedConcurrently[index].report, generatedConcurrently[index].created, generatedConcurrently[index].err = reports.GenerateWeekly(context.Background(), second.UserID, GenerateInput{WeekContainingAt: &now})
		}(index)
	}
	close(startGenerate)
	generateGroup.Wait()
	createdCount := 0
	var emptyReport WeeklyReport
	for _, result := range generatedConcurrently {
		if result.err != nil {
			t.Fatalf("concurrent empty report: %v", result.err)
		}
		if result.created {
			createdCount++
		}
		if emptyReport.ID == "" {
			emptyReport = result.report
		} else if result.report.ID != emptyReport.ID {
			t.Fatalf("concurrent report ids differ: %q and %q", emptyReport.ID, result.report.ID)
		}
	}
	if createdCount != 1 || emptyReport.Metrics == nil || emptyReport.Metrics.Totals.CompletedWorkouts != 0 || emptyReport.Metrics.Totals.VolumeKG != 0 || emptyReport.Metrics.Weight.Samples != 0 || emptyReport.Metrics.Wellness.Entries != 0 || len(emptyReport.Metrics.NewRecords) != 0 || len(emptyReport.Metrics.PainMessages) != 0 {
		t.Fatalf("concurrent empty weekly report created=%d report=%+v", createdCount, emptyReport)
	}
	response = analyticsRequest(t, client, http.MethodGet, server.URL+"/api/v1/reports/"+reportID, "", analyticsHeaders(second.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusNotFound)
	response.Body.Close()

	reps := int16(12)
	_, newVersion, err := workouts.PatchSet(context.Background(), first.UserID, current.SetID, current.WorkoutID, current.Version, workout.SetPatchInput{Repetitions: workout.Optional[int16]{Set: true, Value: &reps}})
	if err != nil {
		t.Fatalf("correct completed set: %v", err)
	}
	current.Version = newVersion
	response = analyticsRequest(t, client, http.MethodGet, server.URL+"/api/v1/reports/"+reportID, "", analyticsHeaders(first.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusOK)
	var stale struct {
		Data WeeklyReport `json:"data"`
	}
	decodeAnalytics(t, response, &stale)
	if stale.Data.Status != "stale" {
		t.Fatalf("report after completed correction status=%q", stale.Data.Status)
	}
	response = analyticsRequest(t, client, http.MethodPost, server.URL+"/api/v1/reports/weekly", fmt.Sprintf(`{"week_containing_at":%q}`, now.Format(time.RFC3339)), analyticsHeaders(first.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusCreated)
	decodeAnalytics(t, response, &generated)
	if generated.Data.Revision != 2 || generated.Data.Metrics.Totals.VolumeKG != 1200 {
		t.Fatalf("regenerated report=%+v", generated.Data)
	}
	response = analyticsRequest(t, client, http.MethodGet, server.URL+"/api/v1/reports?include_revisions=true", "", analyticsHeaders(first.AccessToken))
	requireAnalyticsStatus(t, response, http.StatusOK)
	var listedReports struct {
		Data []WeeklyReport `json:"data"`
	}
	decodeAnalytics(t, response, &listedReports)
	if len(listedReports.Data) != 2 || listedReports.Data[0].Revision != 2 || listedReports.Data[1].Status != "stale" {
		t.Fatalf("listed reports=%+v", listedReports.Data)
	}

	foreign := analyticsHeaders(second.AccessToken)
	foreign.Set("If-Match", latestETag)
	response = analyticsRequest(t, client, http.MethodPatch, server.URL+"/api/v1/measurements/"+latestMeasurement.ID, `{"weight_kg":73}`, foreign)
	requireAnalyticsStatus(t, response, http.StatusNotFound)
	response.Body.Close()
	own := analyticsHeaders(first.AccessToken)
	own.Set("If-Match", latestETag)
	response = analyticsRequest(t, client, http.MethodPatch, server.URL+"/api/v1/measurements/"+latestMeasurement.ID, `{"weight_kg":73}`, own)
	requireAnalyticsStatus(t, response, http.StatusOK)
	latestETag = response.Header.Get("ETag")
	decodeAnalytics(t, response, &struct {
		Data *measurement.BodyMeasurement `json:"data"`
	}{Data: &latestMeasurement})
	if latestMeasurement.WeightKG == nil || *latestMeasurement.WeightKG != 73 {
		t.Fatalf("patched measurement=%+v", latestMeasurement)
	}
	own.Set("If-Match", latestETag)
	response = analyticsRequest(t, client, http.MethodDelete, server.URL+"/api/v1/measurements/"+latestMeasurement.ID, "", own)
	requireAnalyticsStatus(t, response, http.StatusNoContent)
	response.Body.Close()
}

type completedAnalyticsWorkout struct {
	WorkoutID, SetID string
	Version          int64
}

func completeAnalyticsWorkout(t *testing.T, service *workout.Service, actorID string, started time.Time, weight float64, repetitions int16, pain bool) completedAnalyticsWorkout {
	t.Helper()
	name := "Analytics workout"
	value, err := service.Create(context.Background(), actorID, workout.CreateInput{Name: &name, Status: "in_progress", StartedAt: &started})
	if err != nil {
		t.Fatal(err)
	}
	item, version, err := service.AddExercise(context.Background(), actorID, value.ID, value.Version, workout.ExerciseCreateInput{ExerciseID: reportBenchPressID})
	if err != nil {
		t.Fatal(err)
	}
	performed := started.Add(30 * time.Minute)
	set, version, err := service.AddSet(context.Background(), actorID, item.ID, value.ID, version, workout.SetCreateInput{WeightKG: &weight, Repetitions: &repetitions, PerformedAt: &performed})
	if err != nil {
		t.Fatal(err)
	}
	completed := started.Add(time.Hour)
	difficulty := int16(8)
	hasPain := pain
	discomfort := "Небольшой дискомфорт в плече"
	input := workout.CompleteInput{CompletedAt: workout.Optional[time.Time]{Set: true, Value: &completed}, Difficulty: workout.Optional[int16]{Set: true, Value: &difficulty}, HasPain: workout.Optional[bool]{Set: true, Value: &hasPain}}
	if pain {
		input.Discomfort = workout.Optional[string]{Set: true, Value: &discomfort}
	}
	done, err := service.Complete(context.Background(), actorID, value.ID, version, input)
	if err != nil {
		t.Fatal(err)
	}
	return completedAnalyticsWorkout{WorkoutID: value.ID, SetID: set.ID, Version: done.Version}
}

func newAnalyticsIntegrationServer(t *testing.T) (*httptest.Server, *pgxpool.Pool, *workout.Service, *Service) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL is required and must point to a disposable PostgreSQL database")
	}
	path, err := filepath.Abs("../../migrations")
	if err != nil {
		t.Fatal(err)
	}
	migrator, err := database.NewMigrator(url, "file://"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := migrator.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) && !errors.Is(err, migrate.ErrNilVersion) {
		t.Fatal(err)
	}
	if err := migrator.Up(); err != nil {
		t.Fatal(err)
	}
	pool, err := database.OpenPool(context.Background(), analyticsDatabaseConfig(url))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users")
		pool.Close()
		if err := migrator.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			t.Errorf("migrate down: %v", err)
		}
		_, _ = migrator.Close()
	})
	if _, err := database.SeedSystemExercises(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	source := NewSourceInvalidator()
	measurementRepo := measurement.NewRepository(pool)
	userRepo := user.NewRepository(pool)
	initial := user.InitialMeasurementWriterFunc(func(ctx context.Context, tx pgx.Tx, value user.InitialMeasurement) error {
		return measurementRepo.InsertInitial(ctx, tx, measurement.InitialMeasurement{ID: value.ID, UserID: value.UserID, MeasuredAt: value.MeasuredAt, WeightKG: value.WeightKG, ChestCM: value.ChestCM, WaistCM: value.WaistCM, HipsCM: value.HipsCM, NeckCM: value.NeckCM, BicepsCM: value.BicepsCM})
	})
	users := user.NewService(pool, userRepo, initial)
	authCfg := analyticsAuthConfig()
	authService, err := auth.NewService(pool, auth.NewRepository(pool), userRepo, auth.NewTokenManager(authCfg))
	if err != nil {
		t.Fatal(err)
	}
	exercises := exercise.NewService(pool, exercise.NewRepository(pool))
	programs := program.NewService(pool, program.NewRepository(pool), exercises)
	workoutRepo := workout.NewRepository(pool)
	progressRepo := progress.NewRepository(pool)
	projector := progress.NewRecordProjector(progressRepo, workoutRepo)
	workouts := workout.NewService(pool, workoutRepo, programs, exercises, projector, source)
	measurements := measurement.NewService(pool, measurementRepo, users, source)
	progressService := progress.NewService(progressRepo, measurements, workouts, users)
	reports := NewService(pool, NewRepository(pool), source, users, workouts, measurements, progressService)
	authHandler := auth.NewHandler(authService, authCfg, logger)
	measurementHandler := measurement.NewHandler(measurements, logger)
	progressHandler := progress.NewHandler(progressService, logger)
	reportHandler := NewHandler(reports, logger)
	readiness := httpserver.NewReadiness(pool, time.Second)
	readiness.SetReady(true)
	server := httptest.NewServer(httpserver.NewHandler(logger, readiness, func(router chi.Router) {
		authHandler.RegisterRoutes(router)
		router.Group(func(private chi.Router) {
			private.Use(authService.Middleware)
			measurementHandler.RegisterRoutes(private)
			progressHandler.RegisterRoutes(private)
			reportHandler.RegisterRoutes(private)
		})
	}))
	t.Cleanup(server.Close)
	return server, pool, workouts, reports
}

type registeredAnalyticsUser struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
}

func registerAnalyticsUser(t *testing.T, client *http.Client, base, email string) registeredAnalyticsUser {
	t.Helper()
	response := analyticsRequest(t, client, http.MethodPost, base+"/api/v1/auth/register", fmt.Sprintf(`{"email":%q,"password":"long-secure-password"}`, email), http.Header{"Content-Type": []string{"application/json"}})
	requireAnalyticsStatus(t, response, http.StatusCreated)
	var value struct {
		Data registeredAnalyticsUser `json:"data"`
	}
	decodeAnalytics(t, response, &value)
	return value.Data
}
func analyticsHeaders(token string) http.Header {
	return http.Header{"Authorization": []string{"Bearer " + token}, "Content-Type": []string{"application/json"}}
}
func analyticsRequest(t *testing.T, client *http.Client, method, target, body string, headers http.Header) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, target, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	for key, values := range headers {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
func requireAnalyticsStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if response.StatusCode != want {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("status=%d,want=%d body=%s", response.StatusCode, want, body)
	}
}
func decodeAnalytics(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatal(err)
	}
}
func analyticsDatabaseConfig(url string) config.DatabaseConfig {
	return config.DatabaseConfig{URL: url, ConnectTimeout: 5 * time.Second, PingTimeout: 2 * time.Second, MinConnections: 1, MaxConnections: 8, MaxConnLifetime: 10 * time.Minute, MaxConnIdleTime: time.Minute, HealthCheckPeriod: 30 * time.Second}
}
func analyticsAuthConfig() config.AuthConfig {
	return config.AuthConfig{AccessSecret: "analytics-integration-access-secret-at-least-32-characters", RefreshSecret: "analytics-integration-refresh-secret-at-least-32-characters", Issuer: "analytics-integration", AccessAudience: "gymtracker-web", RefreshAudience: "gymtracker-refresh", AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour, CookieName: "gymtracker_refresh", AllowedOrigins: []string{"http://localhost:3000"}, RateLimit: 20, RateWindow: time.Minute}
}
