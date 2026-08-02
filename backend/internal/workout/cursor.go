package workout

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
)

func EncodeCursor(value Workout, filter ListFilter) (string, error) {
	eventAt := workoutEventAt(value)
	raw, err := json.Marshal(Cursor{EventAt: eventAt, ID: value.ID, FilterKey: listFilterKey(filter)})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeCursor(value string, filter ListFilter) (Cursor, error) {
	if value == "" || len(value) > 1024 {
		return Cursor{}, errors.New("invalid cursor")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		return Cursor{}, errors.New("invalid cursor")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var cursor Cursor
	if err := decoder.Decode(&cursor); err != nil || cursor.EventAt.IsZero() || !id.ValidUUID(cursor.ID) {
		return Cursor{}, errors.New("invalid cursor")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Cursor{}, errors.New("invalid cursor")
	}
	if cursor.FilterKey != listFilterKey(filter) {
		return Cursor{}, errors.New("cursor does not match filters")
	}
	cursor.EventAt = cursor.EventAt.UTC()
	return cursor, nil
}

func listFilterKey(filter ListFilter) string {
	parts := []string{filter.Status, filter.ProgramID, filter.ExerciseID}
	if filter.From != nil {
		parts = append(parts, filter.From.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"))
	} else {
		parts = append(parts, "")
	}
	if filter.To != nil {
		parts = append(parts, filter.To.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"))
	} else {
		parts = append(parts, "")
	}
	parts = append(parts, strconv.Itoa(filter.Limit))
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}

func workoutEventAt(value Workout) time.Time {
	if value.StartedAt != nil {
		return value.StartedAt.UTC()
	}
	if value.ScheduledAt != nil {
		return value.ScheduledAt.UTC()
	}
	return value.CreatedAt.UTC()
}
