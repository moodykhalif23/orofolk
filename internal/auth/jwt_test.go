package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseRoundTrip(t *testing.T) {
	iss := NewIssuer("test-secret", time.Hour)
	tok, err := iss.Issue("42", 1, "admin", []string{"order.view"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claims, err := iss.Parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Subject != "42" || claims.Audience != "admin" || claims.OrgID != 1 {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	tok, _ := NewIssuer("secret-a", time.Hour).Issue("1", 1, "admin", nil)
	if _, err := NewIssuer("secret-b", time.Hour).Parse(tok); err == nil {
		t.Fatal("expected parse to reject token signed with a different secret")
	}
}

func TestParseRejectsExpired(t *testing.T) {
	// Negative TTL mints an already-expired token.
	tok, _ := NewIssuer("secret", -time.Hour).Issue("1", 1, "admin", nil)
	if _, err := NewIssuer("secret", time.Hour).Parse(tok); err == nil {
		t.Fatal("expected parse to reject an expired token")
	}
}

func TestParseRejectsMissingExpiration(t *testing.T) {
	// Hand-mint a token with no exp claim; WithExpirationRequired must reject it.
	raw := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "1", "aud": "admin"})
	signed, err := raw.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := NewIssuer("secret", time.Hour).Parse(signed); err == nil {
		t.Fatal("expected parse to reject a token without expiration")
	}
}

func TestParseRejectsNoneAlg(t *testing.T) {
	// alg=none must be refused by the method allow-list.
	raw := jwt.NewWithClaims(jwt.SigningMethodNone,
		jwt.RegisteredClaims{Subject: "1", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))})
	signed, err := raw.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := NewIssuer("secret", time.Hour).Parse(signed); err == nil {
		t.Fatal("expected parse to reject alg=none token")
	}
}
