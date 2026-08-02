package auth

import "testing"

func TestNormalizeEmailAndPasswordLimits(t *testing.T) {
	email, ok := normalizeEmail(" Person@Example.COM ")
	if !ok || email != "person@example.com" {
		t.Fatalf("normalizeEmail = %q, %v", email, ok)
	}
	for _, invalid := range []string{"", "name only <person@example.com>", "not-an-email", "a @example.com"} {
		if _, ok := normalizeEmail(invalid); ok {
			t.Fatalf("accepted invalid email %q", invalid)
		}
	}
	if !validPassword("123456789012") || validPassword("too-short") {
		t.Fatal("password length validation failed")
	}
}
