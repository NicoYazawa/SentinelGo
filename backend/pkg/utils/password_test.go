package utils

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("super-secret-pass")
	if err != nil {
		t.Fatalf("HashPassword error: %v", err)
	}
	if hash == "super-secret-pass" {
		t.Fatal("hash should not equal plaintext")
	}
	if !CheckPasswordHash("super-secret-pass", hash) {
		t.Error("expected matching password to validate")
	}
	if CheckPasswordHash("wrong-pass", hash) {
		t.Error("expected wrong password to fail")
	}
	if CheckPasswordHash("", hash) {
		t.Error("empty password should not validate")
	}
}
