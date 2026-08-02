//go:build integration

package auth_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
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
	"github.com/jackc/pgx/v5"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/auth"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/measurement"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/httpserver"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/user"
)

func TestAuthAndUserHTTPFlow(t *testing.T) {
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

	pool, err := database.OpenPool(context.Background(), integrationConfig(databaseURL))
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	defer pool.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	authConfig := integrationAuthConfig()
	userRepository := user.NewRepository(pool)
	measurementRepository := measurement.NewRepository()
	writer := user.InitialMeasurementWriterFunc(func(ctx context.Context, tx pgx.Tx, value user.InitialMeasurement) error {
		return measurementRepository.InsertInitial(ctx, tx, measurement.InitialMeasurement{
			ID: value.ID, UserID: value.UserID, MeasuredAt: value.MeasuredAt,
			WeightKG: value.WeightKG, ChestCM: value.ChestCM, WaistCM: value.WaistCM,
			HipsCM: value.HipsCM, NeckCM: value.NeckCM, BicepsCM: value.BicepsCM,
		})
	})
	userService := user.NewService(pool, userRepository, writer)
	authService, err := auth.NewService(pool, auth.NewRepository(pool), userRepository, auth.NewTokenManager(authConfig))
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	authHandler := auth.NewHandler(authService, authConfig, logger)
	userHandler := user.NewHandler(userService, logger)
	readiness := httpserver.NewReadiness(pool, time.Second)
	readiness.SetReady(true)
	handler := httpserver.NewHandler(logger, readiness, func(router chi.Router) {
		authHandler.RegisterRoutes(router)
		router.Group(func(private chi.Router) {
			private.Use(authService.Middleware)
			userHandler.RegisterRoutes(private)
		})
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	client := server.Client()

	response := doRequest(t, client, http.MethodGet, server.URL+"/api/v1/profile", "", nil)
	requireStatus(t, response, http.StatusUnauthorized)
	response.Body.Close()

	registerBody := `{"email":"ruslan@example.com","password":"long-secure-password"}`
	response = doRequest(t, client, http.MethodPost, server.URL+"/api/v1/auth/register", registerBody, jsonHeaders())
	requireStatus(t, response, http.StatusCreated)
	var registered dataEnvelope[auth.AuthResult]
	decodeResponse(t, response, &registered)
	refreshCookie := findCookie(t, response.Cookies(), authConfig.CookieName)
	if refreshCookie.Value == "" || registered.Data.AccessToken == "" {
		t.Fatal("token pair was not issued")
	}
	digest := sha256.Sum256([]byte(refreshCookie.Value))
	var storedHashes, hashLength int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*), min(octet_length(token_hash))
		FROM refresh_tokens WHERE user_id = $1 AND token_hash = $2`,
		registered.Data.UserID, digest[:]).Scan(&storedHashes, &hashLength); err != nil {
		t.Fatalf("inspect refresh hash: %v", err)
	}
	if storedHashes != 1 || hashLength != sha256.Size {
		t.Fatalf("stored refresh hashes = %d, length = %d", storedHashes, hashLength)
	}

	privateHeaders := jsonHeaders()
	privateHeaders.Set("Authorization", "Bearer "+registered.Data.AccessToken)
	response = doRequest(t, client, http.MethodGet, server.URL+"/api/v1/profile", "", privateHeaders)
	requireStatus(t, response, http.StatusOK)
	etag := response.Header.Get("ETag")
	var initialProfile dataEnvelope[user.Profile]
	decodeResponse(t, response, &initialProfile)
	if initialProfile.Data.UserID != registered.Data.UserID || etag == "" {
		t.Fatal("own profile or ETag missing")
	}

	patchHeaders := privateHeaders.Clone()
	patchHeaders.Set("If-Match", etag)
	patchBody := `{"name":"Руслан","sex":"male","birth_date":"1995-01-12","height_cm":170,"goal":"strength","experience_level":"intermediate","training_frequency":4,"timezone":"Europe/Moscow","unit_system":"metric"}`
	response = doRequest(t, client, http.MethodPatch, server.URL+"/api/v1/profile", patchBody, patchHeaders)
	requireStatus(t, response, http.StatusOK)
	currentETag := response.Header.Get("ETag")
	var patched dataEnvelope[user.Profile]
	decodeResponse(t, response, &patched)
	if patched.Data.Goal == nil || *patched.Data.Goal != "strength" || patched.Data.Version != 2 {
		t.Fatalf("patched profile = %+v", patched.Data)
	}

	importHeaders := privateHeaders.Clone()
	importHeaders.Set("If-Match", currentETag)
	response = doRequest(t, client, http.MethodPost, server.URL+"/api/v1/profile/import", `{"name":"Руслан","unexpected":true}`, importHeaders)
	requireStatus(t, response, http.StatusBadRequest)
	response.Body.Close()

	importBody := `{"name":"Руслан","sex":"male","height_cm":170,"weight_kg":66.7,"goal":"recomposition","training_frequency":4,"experience_level":"intermediate","sleep_hours_average":8,"measurements":{"chest_cm":100,"waist_cm":77,"hips_cm":89,"neck_cm":38,"biceps_cm":36},"notes":["Цель — набор мышц с сохранением эстетичной формы","Рабочие подходы выполняются близко к отказу"]}`
	response = doRequest(t, client, http.MethodPost, server.URL+"/api/v1/profile/import", importBody, importHeaders)
	requireStatus(t, response, http.StatusOK)
	var imported dataEnvelope[user.ImportResult]
	decodeResponse(t, response, &imported)
	if imported.Data.InitialMeasurementID == nil || len(imported.Data.Profile.Notes) != 2 || imported.Data.Profile.Version != 3 {
		t.Fatalf("import result = %+v", imported.Data)
	}
	var ownerID, source string
	var leftArm, rightArm float64
	if err := pool.QueryRow(context.Background(), `
		SELECT user_id, source, left_upper_arm_cm, right_upper_arm_cm
		FROM body_measurements WHERE id = $1`, *imported.Data.InitialMeasurementID).
		Scan(&ownerID, &source, &leftArm, &rightArm); err != nil {
		t.Fatalf("get imported measurement: %v", err)
	}
	if ownerID != registered.Data.UserID || source != "import" || leftArm != 36 || rightArm != 36 {
		t.Fatalf("measurement owner/source/arms = %s, %s, %v, %v", ownerID, source, leftArm, rightArm)
	}

	refreshHeaders := http.Header{"Origin": []string{"http://localhost:3000"}}
	refreshHeaders.Set("Cookie", refreshCookie.String())
	response = doRequest(t, client, http.MethodPost, server.URL+"/api/v1/auth/refresh", "", refreshHeaders)
	requireStatus(t, response, http.StatusOK)
	rotatedCookie := findCookie(t, response.Cookies(), authConfig.CookieName)
	var refreshed dataEnvelope[auth.AuthResult]
	decodeResponse(t, response, &refreshed)
	if rotatedCookie.Value == refreshCookie.Value {
		t.Fatal("refresh token was not rotated")
	}

	response = doRequest(t, client, http.MethodPost, server.URL+"/api/v1/auth/refresh", "", refreshHeaders)
	requireStatus(t, response, http.StatusUnauthorized)
	response.Body.Close()
	response = doRequest(t, client, http.MethodGet, server.URL+"/api/v1/profile", "", privateHeaders)
	requireStatus(t, response, http.StatusUnauthorized)
	response.Body.Close()

	wrongEmail := loginProblem(t, client, server.URL, `{"email":"missing@example.com","password":"wrong-password-value"}`)
	wrongPassword := loginProblem(t, client, server.URL, `{"email":"ruslan@example.com","password":"wrong-password-value"}`)
	if wrongEmail.Code != wrongPassword.Code || wrongEmail.Detail != wrongPassword.Detail || wrongEmail.Title != wrongPassword.Title {
		t.Fatalf("credential errors differ: %+v / %+v", wrongEmail, wrongPassword)
	}

	response = doRequest(t, client, http.MethodPost, server.URL+"/api/v1/auth/login", registerBody, jsonHeaders())
	requireStatus(t, response, http.StatusOK)
	var loggedIn dataEnvelope[auth.AuthResult]
	decodeResponse(t, response, &loggedIn)
	loggedInCookie := findCookie(t, response.Cookies(), authConfig.CookieName)
	logoutHeaders := http.Header{"Origin": []string{"http://localhost:3000"}}
	logoutHeaders.Set("Authorization", "Bearer "+loggedIn.Data.AccessToken)
	logoutHeaders.Set("Cookie", loggedInCookie.String())
	response = doRequest(t, client, http.MethodPost, server.URL+"/api/v1/auth/logout", "", logoutHeaders)
	requireStatus(t, response, http.StatusNoContent)
	response.Body.Close()

	refreshAfterLogout := http.Header{"Origin": []string{"http://localhost:3000"}}
	refreshAfterLogout.Set("Cookie", loggedInCookie.String())
	response = doRequest(t, client, http.MethodPost, server.URL+"/api/v1/auth/refresh", "", refreshAfterLogout)
	requireStatus(t, response, http.StatusUnauthorized)
	response.Body.Close()

	concurrentBody := `{"email":"concurrent@example.com","password":"long-secure-password"}`
	response = doRequest(t, client, http.MethodPost, server.URL+"/api/v1/auth/register", concurrentBody, jsonHeaders())
	requireStatus(t, response, http.StatusCreated)
	var concurrentUser dataEnvelope[auth.AuthResult]
	decodeResponse(t, response, &concurrentUser)
	concurrentCookie := findCookie(t, response.Cookies(), authConfig.CookieName)
	importFailure := errors.New("measurement writer failed")
	failingUserService := user.NewService(pool, userRepository, user.InitialMeasurementWriterFunc(
		func(context.Context, pgx.Tx, user.InitialMeasurement) error { return importFailure },
	))
	rollbackName, rollbackWeight := "must roll back", 70.0
	rollbackNotes := []string{"must not persist"}
	if _, err := failingUserService.Import(context.Background(), concurrentUser.Data.UserID, 1, user.ImportInput{
		Name: &rollbackName, WeightKG: &rollbackWeight, Notes: &rollbackNotes,
	}); !errors.Is(err, importFailure) {
		t.Fatalf("transactional import error = %v", err)
	}
	rolledBack, err := failingUserService.Get(context.Background(), concurrentUser.Data.UserID)
	if err != nil {
		t.Fatalf("get rolled-back profile: %v", err)
	}
	if rolledBack.Version != 1 || rolledBack.Name != nil || len(rolledBack.Notes) != 0 {
		t.Fatalf("profile import was not rolled back: %+v", rolledBack)
	}
	var rolledBackMeasurements int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM body_measurements WHERE user_id = $1`,
		concurrentUser.Data.UserID).Scan(&rolledBackMeasurements); err != nil {
		t.Fatalf("count rolled-back measurements: %v", err)
	}
	if rolledBackMeasurements != 0 {
		t.Fatalf("rolled-back measurement count = %d", rolledBackMeasurements)
	}
	start := make(chan struct{})
	statuses := make(chan int, 2)
	requestErrors := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			request, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/auth/refresh", nil)
			if err == nil {
				request.Header.Set("Origin", "http://localhost:3000")
				request.Header.Set("Cookie", concurrentCookie.String())
				var result *http.Response
				result, err = client.Do(request)
				if err == nil {
					_, _ = io.Copy(io.Discard, result.Body)
					result.Body.Close()
					statuses <- result.StatusCode
					return
				}
			}
			requestErrors <- err
		}()
	}
	close(start)
	successes, replays := 0, 0
	for range 2 {
		select {
		case err := <-requestErrors:
			t.Fatalf("concurrent refresh: %v", err)
		case status := <-statuses:
			if status == http.StatusOK {
				successes++
			}
			if status == http.StatusUnauthorized {
				replays++
			}
		}
	}
	if successes != 1 || replays != 1 {
		t.Fatalf("concurrent refresh statuses: success=%d replay=%d", successes, replays)
	}
	var authVersion, activeTokens, audits int
	if err := pool.QueryRow(context.Background(), `
		SELECT u.auth_version,
		       count(*) FILTER (WHERE rt.revoked_at IS NULL),
		       count(DISTINCT ae.id)
		FROM users u
		LEFT JOIN refresh_tokens rt ON rt.user_id = u.id
		LEFT JOIN audit_events ae ON ae.actor_user_id = u.id AND ae.event_type = 'auth.refresh_reuse'
		WHERE u.id = $1
		GROUP BY u.auth_version`, concurrentUser.Data.UserID).
		Scan(&authVersion, &activeTokens, &audits); err != nil {
		t.Fatalf("inspect concurrent replay state: %v", err)
	}
	if authVersion != 2 || activeTokens != 0 || audits != 1 {
		t.Fatalf("replay state: auth_version=%d active=%d audits=%d", authVersion, activeTokens, audits)
	}
}

type dataEnvelope[T any] struct {
	Data T `json:"data"`
}

type problemResponse struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

func loginProblem(t *testing.T, client *http.Client, baseURL, body string) problemResponse {
	t.Helper()
	response := doRequest(t, client, http.MethodPost, baseURL+"/api/v1/auth/login", body, jsonHeaders())
	requireStatus(t, response, http.StatusUnauthorized)
	var problem problemResponse
	decodeResponse(t, response, &problem)
	return problem
}

func doRequest(t *testing.T, client *http.Client, method, target, body string, headers http.Header) *http.Response {
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

func jsonHeaders() http.Header {
	return http.Header{"Content-Type": []string{"application/json"}}
}

func requireStatus(t *testing.T, response *http.Response, expected int) {
	t.Helper()
	if response.StatusCode != expected {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("status = %d, want %d; body=%s", response.StatusCode, expected, body)
	}
}

func decodeResponse(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name && cookie.Value != "" {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found", name)
	return nil
}

func integrationConfig(url string) config.DatabaseConfig {
	return config.DatabaseConfig{
		URL: url, ConnectTimeout: 5 * time.Second, PingTimeout: 2 * time.Second,
		MinConnections: 1, MaxConnections: 4, MaxConnLifetime: 10 * time.Minute,
		MaxConnIdleTime: time.Minute, HealthCheckPeriod: 30 * time.Second,
	}
}

func integrationAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		AccessSecret:  "integration-access-secret-at-least-32-characters",
		RefreshSecret: "integration-refresh-secret-at-least-32-characters",
		Issuer:        "gymtracker-test", AccessAudience: "gymtracker-web",
		RefreshAudience: "gymtracker-refresh", AccessTTL: 15 * time.Minute,
		RefreshTTL: 24 * time.Hour, CookieName: "gymtracker_refresh",
		AllowedOrigins: []string{"http://localhost:3000"}, RateLimit: 20, RateWindow: time.Minute,
	}
}
