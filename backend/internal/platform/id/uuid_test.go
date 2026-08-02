package id

import (
	"regexp"
	"testing"
)

func TestUUID(t *testing.T) {
	first, err := UUID()
	if err != nil {
		t.Fatalf("UUID: %v", err)
	}
	second, err := UUID()
	if err != nil {
		t.Fatalf("UUID: %v", err)
	}
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !pattern.MatchString(first) {
		t.Fatalf("UUID = %q", first)
	}
	if first == second {
		t.Fatal("two UUIDs are equal")
	}
}
