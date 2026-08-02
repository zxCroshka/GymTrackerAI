package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ErrBodyTooLarge identifies a route body limit violation.
var ErrBodyTooLarge = errors.New("request body is too large")

// DecodeJSON decodes exactly one JSON object, rejects unknown fields and
// enforces the route's byte limit.
func DecodeJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return ErrBodyTooLarge
		}
		return fmt.Errorf("decode JSON object")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request body must contain exactly one JSON object")
	}
	return nil
}
