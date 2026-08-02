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

func TestValidUUID(t *testing.T) {
	if !ValidUUID("00000000-0000-4000-8000-000000000001") {
		t.Fatal("valid UUID rejected")
	}
	for _, value := range []string{"", "not-a-uuid", "000000000000-4000-8000-000000000001", "00000000-0000-4000-8000-00000000000z"} {
		if ValidUUID(value) {
			t.Fatalf("invalid UUID accepted: %q", value)
		}
	}
}
