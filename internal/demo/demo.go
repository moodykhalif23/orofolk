// Package demo provisions instant, self-serve demo tenants for the marketing
// landing page. A prospect submits a short form; Provision reuses
// tenant.Provision to create a fresh, isolated organization, activates it
// immediately (no email verification), seeds representative data so the
// dashboards and insights look alive, and mints an admin token so the visitor is
// logged straight in. Demos are time-limited — a daily job suspends them once
// they expire (see SuspendExpiredDemoOrgs).
package demo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/auth"
	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/tenant"
)

// TTL is how long a demo org stays live before the expiry sweep suspends it.
const TTL = 7 * 24 * time.Hour

// Input is one demo request from the landing page.
type Input struct {
	Name    string
	Email   string
	Company string
}

// Result is what the caller returns to the browser: a ready-to-use admin token
// (auto-login) plus the demo's domain and expiry.
type Result struct {
	Token     string    `json:"token"`
	Domain    string    `json:"domain"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Provision creates and activates a demo org, seeds it, and returns an admin
// token for immediate login.
func Provision(ctx context.Context, pool *pgxpool.Pool, issuer *auth.Issuer, in Input) (Result, error) {
	company := strings.TrimSpace(in.Company)
	if company == "" {
		company = "Demo Co"
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "Demo User"
	}
	// Email shape is validated by tenant.Validate inside Provision (it returns a
	// tenant.ValidationError the handler maps to 400).
	email := strings.ToLower(strings.TrimSpace(in.Email))

	expiresAt := time.Now().Add(TTL)

	// A unique subdomain per demo; retry a few times on the rare collision.
	var res tenant.Result
	var err error
	for attempt := 0; attempt < 6; attempt++ {
		pw, perr := randString(14)
		if perr != nil {
			return Result{}, perr
		}
		res, err = tenant.Provision(ctx, pool, tenant.Input{
			OrgName: company, FullName: name, Email: email, Password: pw,
			Subdomain: demoSubdomain(company), Currency: "USD", VerifyTTL: TTL,
		})
		if err == nil {
			break
		}
		if errors.Is(err, tenant.ErrDomainTaken) {
			continue
		}
		return Result{}, err
	}
	if err != nil {
		return Result{}, err
	}

	q := gen.New(pool)
	// Go live immediately (skip email verification) and stamp the demo expiry.
	if aerr := q.ActivateDemoOrg(ctx, gen.ActivateDemoOrgParams{ID: res.OrgID, DemoExpiresAt: tsz(expiresAt)}); aerr != nil {
		return Result{}, fmt.Errorf("activate demo: %w", aerr)
	}
	// Best-effort: a seeding hiccup must not deny the prospect access — they'd
	// just land in an emptier org.
	seed(ctx, pool, res.OrgID, res.WebsiteID)

	perms, perr := q.GetUserPermissions(ctx, res.UserID)
	if perr != nil {
		return Result{}, fmt.Errorf("load permissions: %w", perr)
	}
	token, terr := issuer.Issue(strconv.FormatInt(res.UserID, 10), res.OrgID, "admin", perms)
	if terr != nil {
		return Result{}, fmt.Errorf("issue token: %w", terr)
	}
	return Result{Token: token, Domain: res.Domain, Email: email, ExpiresAt: expiresAt}, nil
}

// demoSubdomain builds "demo-<slug>-<rand>" within the subdomain rules.
func demoSubdomain(company string) string {
	slug := slugify(company)
	if slug == "" {
		slug = "co"
	}
	if len(slug) > 30 {
		slug = slug[:30]
	}
	suffix, _ := randHex(4)
	return "demo-" + slug + "-" + suffix
}

func slugify(s string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '-' || r == '_':
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func randHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randString(n int) (string, error) { return randHex(n) }

func tsz(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
