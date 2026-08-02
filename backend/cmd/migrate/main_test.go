package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateMigrationCreatesForwardAndRollbackFiles(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, time.August, 2, 12, 34, 56, 0, time.FixedZone("test", 3*60*60))

	if err := createMigration(directory, "add_example", now.UTC()); err != nil {
		t.Fatalf("create migration: %v", err)
	}

	for _, suffix := range []string{"up", "down"} {
		path := filepath.Join(directory, "20260802093456_add_example."+suffix+".sql")
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected %s migration: %v", suffix, err)
			continue
		}
		if string(contents) != "SET TIME ZONE 'UTC';\n" {
			t.Errorf("unexpected %s migration contents: %q", suffix, contents)
		}
	}
}

func TestCreateMigrationRejectsUnsafeName(t *testing.T) {
	err := createMigration(t.TempDir(), "../private-value", time.Now())
	if err == nil || !strings.Contains(err.Error(), "snake_case") {
		t.Fatalf("unsafe migration name error = %v", err)
	}
}
