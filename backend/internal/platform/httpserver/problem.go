package httpserver

import (
	"encoding/json"
	"net/http"
)

type problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail"`
	Instance  string `json:"instance"`
	Code      string `json:"code"`
	RequestID string `json:"request_id"`
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	writeJSON(w, status, "application/problem+json", problem{
		Type:      "https://gymtracker.example/problems/" + code,
		Title:     title,
		Status:    status,
		Detail:    detail,
		Instance:  r.URL.Path,
		Code:      code,
		RequestID: requestIDFromContext(r.Context()),
	})
}

func writeJSON(w http.ResponseWriter, status int, contentType string, body any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
