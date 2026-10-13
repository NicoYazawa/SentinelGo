package token

import (
	"testing"
	"time"
)

const testSecret = "this-is-a-32-byte-test-secret-key!!"

func TestGenerateAndParseAccessToken(t *testing.T) {
	Init(testSecret, nil)
	tok, err := GenerateAccessToken(1, "alice", []string{"admin"}, "zh-CN", "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 1 || claims.Username != "alice" || claims.TokenType != "access" {
		t.Errorf("claims wrong: %+v", claims)
	}
	if claims.SessionID != "sess-1" {
		t.Errorf("session id wrong: %q", claims.SessionID)
	}
	if time.Until(claims.ExpiresAt.Time) > AccessTokenTTL+time.Minute {
		t.Error("expiry too far")
	}
}

func TestParseInvalidToken(t *testing.T) {
	Init(testSecret, nil)
	if _, err := ParseToken("not-a-token"); err == nil {
		t.Error("expected error for bad token")
	}
}

func TestGenerateRefreshTokenAndPair(t *testing.T) {
	Init(testSecret, nil)
	rt, err := GenerateRefreshToken(2, "bob", nil, "en-US")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken(rt)
	if err != nil {
		t.Fatal(err)
	}
	if claims.TokenType != "refresh" {
		t.Errorf("expected refresh type, got %q", claims.TokenType)
	}

	pair, err := GenerateTokenPair(3, "carol", []string{"user"}, "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Error("token pair missing tokens")
	}
	if pair.ExpiresIn != int64(AccessTokenTTL.Seconds()) {
		t.Errorf("expires_in wrong: %d", pair.ExpiresIn)
	}
}

func TestBlacklistNilRedis(t *testing.T) {
	Init(testSecret, nil)
	if err := BlacklistToken("jti-1", time.Minute); err != nil {
		t.Errorf("nil redis should not error: %v", err)
	}
	if IsBlacklisted("jti-1") {
		t.Error("nil redis should report not blacklisted")
	}
}

func TestSessionIDHelper(t *testing.T) {
	if sessionID(nil) != "" {
		t.Error("empty should be empty")
	}
	if sessionID([]string{"a", "b"}) != "a" {
		t.Error("should take first")
	}
}
