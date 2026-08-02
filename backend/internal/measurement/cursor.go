package measurement

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
)

func encodeCursor(at time.Time, valueID string) (string, error) {
	raw, err := json.Marshal(Cursor{At: at.UTC().Format(time.RFC3339Nano), ID: valueID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeCursor(value string) (Cursor, error) {
	if len(value) == 0 || len(value) > 1024 {
		return Cursor{}, ErrValidation
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		return Cursor{}, ErrValidation
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var cursor Cursor
	if err := decoder.Decode(&cursor); err != nil || !id.ValidUUID(cursor.ID) {
		return Cursor{}, ErrValidation
	}
	parsed, err := time.Parse(time.RFC3339Nano, cursor.At)
	if err != nil || parsed.Location() != time.UTC {
		return Cursor{}, ErrValidation
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Cursor{}, ErrValidation
	}
	return cursor, nil
}
