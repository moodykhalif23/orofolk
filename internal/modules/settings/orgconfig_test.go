package settings_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"b2bcommerce/internal/auth"
	"b2bcommerce/internal/queue/jobs"
	"b2bcommerce/internal/secretbox"
	"b2bcommerce/internal/server"
	"b2bcommerce/internal/store"
	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/testsupport"

	"b2bcommerce/internal/email"
)

// newServerWithSecrets is newServer plus the credential-encryption box, as
// cmd/api wires it.
func newServerWithSecrets(t *testing.T) (http.Handler, *auth.Issuer, *gen.Queries) {
	t.Helper()
	pool := testsupport.NewDB(t)
	box, err := secretbox.New("test-config-encryption-key")
	if err != nil {
		t.Fatal(err)
	}
	issuer := auth.NewIssuer(testSecret, time.Hour)
	return server.New(store.New(pool), issuer, server.WithSecrets(box)), issuer, gen.New(pool)
}

// TestPaymentConfigSealedAndMasked: credentials round-trip through the API
// without ever being echoed back, and land encrypted in the database.
func TestPaymentConfigSealedAndMasked(t *testing.T) {
	h, issuer, q := newServerWithSecrets(t)
	admin, _ := issuer.Issue("1", 1, "admin", []string{"settings.view", "settings.manage"})

	// Unknown gateway names are refused with the available list.
	if rr := do(t, h, http.MethodPut, "/admin/settings/payments", admin,
		map[string]any{"gateway": "stripe", "credentials": map[string]string{"k": "v"}}); rr.Code != http.StatusBadRequest {
		t.Fatalf("unknown gateway: want 400, got %d (%s)", rr.Code, rr.Body.String())
	}

	put := do(t, h, http.MethodPut, "/admin/settings/payments", admin, map[string]any{
		"gateway":     "mock",
		"credentials": map[string]string{"api_key": "sk_live_supersecret", "till": "174379"},
	})
	if put.Code != http.StatusOK {
		t.Fatalf("put payments: %d (%s)", put.Code, put.Body.String())
	}
	if strings.Contains(put.Body.String(), "supersecret") {
		t.Fatal("PUT response echoed a credential value")
	}

	// GET shows the gateway + which keys exist — never the values.
	get := do(t, h, http.MethodGet, "/admin/settings/payments", admin, nil)
	var got struct {
		Gateway        string   `json:"gateway"`
		Available      []string `json:"available"`
		ConfiguredKeys []string `json:"configured_keys"`
	}
	_ = json.Unmarshal(get.Body.Bytes(), &got)
	if got.Gateway != "mock" || len(got.ConfiguredKeys) != 2 {
		t.Fatalf("get payments: %s", get.Body.String())
	}
	if strings.Contains(get.Body.String(), "supersecret") {
		t.Fatal("GET response echoed a credential value")
	}

	// At rest the blob is ciphertext, not plaintext JSON.
	cfg, err := q.GetOrgPaymentConfig(context.Background(), 1)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(cfg.CredentialsEnc), "supersecret") {
		t.Fatal("credentials stored in plaintext")
	}

	// Updating the gateway WITHOUT credentials keeps the stored ones.
	if rr := do(t, h, http.MethodPut, "/admin/settings/payments", admin, map[string]any{"gateway": "mock"}); rr.Code != http.StatusOK {
		t.Fatalf("gateway-only put: %d", rr.Code)
	}
	get2 := do(t, h, http.MethodGet, "/admin/settings/payments", admin, nil)
	_ = json.Unmarshal(get2.Body.Bytes(), &got)
	if len(got.ConfiguredKeys) != 2 {
		t.Fatalf("credentials lost on gateway-only update: %s", get2.Body.String())
	}

	// settings.manage gates writes.
	viewer, _ := issuer.Issue("1", 1, "admin", []string{"settings.view"})
	if rr := do(t, h, http.MethodPut, "/admin/settings/payments", viewer, map[string]any{"gateway": "mock"}); rr.Code != http.StatusForbidden {
		t.Fatalf("viewer put: want 403, got %d", rr.Code)
	}
}

// TestStorefrontBranding: the public endpoint resolves branding by serving host
// and falls back to empty strings when nothing is configured.
func TestStorefrontBranding(t *testing.T) {
	h, issuer, _ := newServerWithSecrets(t)
	admin, _ := issuer.Issue("1", 1, "admin", []string{"settings.manage"})

	for key, val := range map[string]string{
		"branding.store_name":  "Acme Industrial Supply",
		"branding.brand_color": "#0f766e",
		"branding.logo_url":    "/media/logo.png",
	} {
		rr := do(t, h, http.MethodPut, "/admin/settings", admin, map[string]any{
			"scope": "org", "key": key, "value": val,
		})
		if rr.Code != http.StatusOK {
			t.Fatalf("seed %s: %d (%s)", key, rr.Code, rr.Body.String())
		}
	}

	// demo.localhost is org 1's seeded website domain.
	req := httptest.NewRequest(http.MethodGet, "/storefront/branding", nil)
	req.Host = "demo.localhost"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	var b struct {
		StoreName  string `json:"store_name"`
		BrandColor string `json:"brand_color"`
		LogoURL    string `json:"logo_url"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &b)
	if b.StoreName != "Acme Industrial Supply" || b.BrandColor != "#0f766e" || b.LogoURL != "/media/logo.png" {
		t.Fatalf("branding: %s", rr.Body.String())
	}
}

// captureSender records sent messages for the From-identity assertions.
type captureSender struct{ last email.Message }

func (c *captureSender) Send(_ context.Context, m email.Message) error {
	c.last = m
	return nil
}

// TestEmailSenderIdentityPerOrg: the send_email worker applies the org's
// configured From identity, and leaves the platform default when none is set.
func TestEmailSenderIdentityPerOrg(t *testing.T) {
	h, issuer, q := newServerWithSecrets(t)
	admin, _ := issuer.Issue("1", 1, "admin", []string{"settings.manage"})
	ctx := context.Background()

	for key, val := range map[string]string{
		"email.from_name":    "Acme Industrial",
		"email.from_address": "orders@acme.test",
	} {
		if rr := do(t, h, http.MethodPut, "/admin/settings", admin, map[string]any{
			"scope": "org", "key": key, "value": val,
		}); rr.Code != http.StatusOK {
			t.Fatalf("seed %s: %d", key, rr.Code)
		}
	}

	args := jobs.SendEmailArgs{
		To: "buyer@x.test", Template: "order_confirmation",
		Data:           []byte(`{"name":"Ada","order_number":"ORD-1","total":"1","currency":"USD"}`),
		OrganizationID: 1,
	}
	sender := &captureSender{}
	if err := jobs.SendEmail(ctx, sender, q, args); err != nil {
		t.Fatalf("send: %v", err)
	}
	if sender.last.FromName != "Acme Industrial" || sender.last.FromAddress != "orders@acme.test" {
		t.Fatalf("org identity not applied: %+v", sender.last)
	}

	// No org → platform identity (empty overrides).
	args.OrganizationID = 0
	sender2 := &captureSender{}
	if err := jobs.SendEmail(ctx, sender2, q, args); err != nil {
		t.Fatalf("send: %v", err)
	}
	if sender2.last.FromName != "" || sender2.last.FromAddress != "" {
		t.Fatalf("platform send must not carry overrides: %+v", sender2.last)
	}
}
