package exercise

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
)

func EncodeCursor(exercise Exercise) (string, error) {
	value, err := json.Marshal(Cursor{Name: strings.ToLower(exercise.Name), ID: exercise.ID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func DecodeCursor(value string) (Cursor, error) {
	if len(value) == 0 || len(value) > 1024 {
		return Cursor{}, errors.New("invalid cursor")
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		return Cursor{}, errors.New("invalid cursor")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var cursor Cursor
	if err := decoder.Decode(&cursor); err != nil || cursor.Name == "" || !id.ValidUUID(cursor.ID) {
		return Cursor{}, errors.New("invalid cursor")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Cursor{}, errors.New("invalid cursor")
	}
	return cursor, nil
}
