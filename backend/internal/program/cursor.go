package program

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
)

func EncodeCursor(value Program) (string, error) {
	raw, err := json.Marshal(Cursor{UpdatedAt: value.UpdatedAt.UTC(), ID: value.ID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
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
	if err := decoder.Decode(&cursor); err != nil || cursor.UpdatedAt.IsZero() || !id.ValidUUID(cursor.ID) {
		return Cursor{}, errors.New("invalid cursor")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Cursor{}, errors.New("invalid cursor")
	}
	cursor.UpdatedAt = cursor.UpdatedAt.UTC()
	return cursor, nil
}
