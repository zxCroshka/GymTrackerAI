package auth

import "testing"

func TestPasswordHasher(t *testing.T) {
	hasher := PasswordHasher{}
	encoded, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !hasher.Compare(encoded, "correct horse battery staple") {
		t.Fatal("valid password was rejected")
	}
	if hasher.Compare(encoded, "wrong password") {
		t.Fatal("invalid password was accepted")
	}
	if hasher.Compare("malformed", "correct horse battery staple") {
		t.Fatal("malformed hash was accepted")
	}
}
