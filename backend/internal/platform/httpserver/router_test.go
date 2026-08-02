package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoints(t *testing.T) {
	readiness := NewReadiness()
	handler := NewHandler(discardLogger(), readiness)

	t.Run("liveness", func(t *testing.T) {
		response := performRequest(handler, http.MethodGet, "/health/live", "client-request-1")
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
		}
		if response.Header().Get(requestIDHeader) != "client-request-1" {
			t.Fatalf("request ID header = %q", response.Header().Get(requestIDHeader))
		}

		var body healthResponse
		decodeJSON(t, response.Body, &body)
		if body.Data.Status != "ok" || body.Meta.RequestID != "client-request-1" {
			t.Fatalf("unexpected response body: %+v", body)
		}
	})

	t.Run("readiness before initialization", func(t *testing.T) {
		response := performRequest(handler, http.MethodGet, "/health/ready", "ready-request-1")
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
		}
		assertProblemCode(t, response, "not_ready")
	})

	t.Run("readiness after initialization", func(t *testing.T) {
		readiness.SetReady(true)
		response := performRequest(handler, http.MethodGet, "/health/ready", "ready-request-2")
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
		}
	})
}

func TestRouterUsesUnifiedProblems(t *testing.T) {
	handler := NewHandler(discardLogger(), NewReadiness())

	tests := []struct {
		name     string
		method   string
		path     string
		status   int
		wantCode string
	}{
		{name: "not found", method: http.MethodGet, path: "/missing", status: http.StatusNotFound, wantCode: "not_found"},
		{name: "method not allowed", method: http.MethodPost, path: "/health/live", status: http.StatusMethodNotAllowed, wantCode: "method_not_allowed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := performRequest(handler, test.method, test.path, "problem-request")
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d", response.Code, test.status)
			}
			assertProblemCode(t, response, test.wantCode)
		})
	}
}

func TestRecovererDoesNotExposePanicValue(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := requestID(logger)(requestLogger(logger)(recoverer(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("sensitive panic value")
	}))))

	response := performRequest(handler, http.MethodGet, "/panic", "panic-request")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	assertProblemCode(t, response, "internal_error")
	if strings.Contains(response.Body.String(), "sensitive") || strings.Contains(logs.String(), "sensitive") {
		t.Fatal("panic value leaked to response or logs")
	}
}

func TestInvalidRequestIDIsReplaced(t *testing.T) {
	handler := NewHandler(discardLogger(), NewReadiness())
	response := performRequest(handler, http.MethodGet, "/health/live", "invalid request ID")

	generated := response.Header().Get(requestIDHeader)
	if generated == "invalid request ID" || len(generated) != 32 {
		t.Fatalf("generated request ID = %q", generated)
	}
}

func performRequest(handler http.Handler, method, path, requestID string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set(requestIDHeader, requestID)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertProblemCode(t *testing.T, response *httptest.ResponseRecorder, wantCode string) {
	t.Helper()
	if response.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
	}
	var body problem
	decodeJSON(t, response.Body, &body)
	if body.Code != wantCode || body.RequestID == "" {
		t.Fatalf("unexpected problem: %+v", body)
	}
}

func decodeJSON(t *testing.T, reader io.Reader, target any) {
	t.Helper()
	if err := json.NewDecoder(reader).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
