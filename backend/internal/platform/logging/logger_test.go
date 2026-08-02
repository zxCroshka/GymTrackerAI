package logging

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNewWritesRepositoryJSONFields(t *testing.T) {
	var output bytes.Buffer
	logger, err := New(&output, "info", "gymtracker-test", "test")
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}

	logger.Info("request completed", "request_id", "request-123")

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}

	for _, key := range []string{"timestamp", "level", "message", "service", "environment", "request_id"} {
		if _, exists := entry[key]; !exists {
			t.Fatalf("log entry is missing %q: %v", key, entry)
		}
	}
	if entry["level"] != "info" {
		t.Fatalf("level = %v, want info", entry["level"])
	}
}

func TestNewRejectsUnknownLevel(t *testing.T) {
	if _, err := New(&bytes.Buffer{}, "trace", "test", "test"); err == nil {
		t.Fatal("expected invalid level error")
	}
}

func TestBootstrapUsesRepositoryFormat(t *testing.T) {
	var output bytes.Buffer
	Bootstrap(&output).Error("startup failed")

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode bootstrap log entry: %v", err)
	}
	if entry["environment"] != "bootstrap" || entry["timestamp"] == nil || entry["message"] != "startup failed" {
		t.Fatalf("unexpected bootstrap log entry: %v", entry)
	}
}
