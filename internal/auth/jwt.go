package auth

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload for both admin and storefront contexts.
type Claims struct {
	OrgID       int64    `json:"org_id"`
	Audience    string   `json:"aud"` // "admin" or "storefront"
	Permissions []string `json:"perms,omitempty"`
	// CustomerID is set for storefront tokens: the buying company the
	// authenticated customer-user belongs to. Subject holds the customer_user id.
	CustomerID int64 `json:"cust_id,omitempty"`
	jwt.RegisteredClaims
}

// Issuer mints and verifies tokens with a shared secret.
type Issuer struct {
	secret []byte
	ttl    time.Duration
}

func NewIssuer(secret string, ttl time.Duration) *Issuer {
	return &Issuer{secret: []byte(secret), ttl: ttl}
}

// Issue creates a signed token for a subject (user id or customer-user id).
func (i *Issuer) Issue(subject string, orgID int64, audience string, perms []string) (string, error) {
	now := time.Now()
	claims := Claims{
		OrgID:       orgID,
		Audience:    audience,
		Permissions: perms,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(i.secret)
}

// IssueStorefront mints a storefront token for a customer-user. Subject is the
// customer_user id; CustomerID carries the buying company.
func (i *Issuer) IssueStorefront(customerUserID, orgID, customerID int64) (string, error) {
	now := time.Now()
	claims := Claims{
		OrgID:      orgID,
		Audience:   "storefront",
		CustomerID: customerID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(customerUserID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(i.secret)
}

// Parse verifies a token string and returns its claims.
func (i *Issuer) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}
