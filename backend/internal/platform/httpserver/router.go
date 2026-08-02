package httpserver

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewHandler wires operational routes and the supplied modular-monolith API.
func NewHandler(logger *slog.Logger, readiness *Readiness, registerAPI func(chi.Router)) http.Handler {
	router := chi.NewRouter()
	router.Use(requestID(logger))
	router.Use(requestLogger(logger))
	router.Use(recoverer(logger))

	router.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeHealth(w, r, http.StatusOK, "ok")
	})
	router.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if !readiness.Ready(r.Context()) {
			writeProblem(w, r, http.StatusServiceUnavailable, "not_ready", "Service unavailable", "Application is not ready")
			return
		}
		writeHealth(w, r, http.StatusOK, "ready")
	})
	if registerAPI != nil {
		router.Route("/api/v1", registerAPI)
	}

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeProblem(w, r, http.StatusNotFound, "not_found", "Not found", "Resource was not found")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeProblem(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "HTTP method is not supported for this resource")
	})

	return router
}

type healthResponse struct {
	Data struct {
		Status string `json:"status"`
	} `json:"data"`
	Meta struct {
		RequestID string `json:"request_id"`
	} `json:"meta"`
}

func writeHealth(w http.ResponseWriter, r *http.Request, status int, state string) {
	response := healthResponse{}
	response.Data.Status = state
	response.Meta.RequestID = requestIDFromContext(r.Context())
	writeJSON(w, status, "application/json", response)
}
