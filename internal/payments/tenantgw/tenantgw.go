// Package tenantgw resolves the payment gateway per organization :
// tenants are their own merchants of record, so the processor used at charge
// time follows the org's stored config (org_payment_configs) instead of the
// process-wide env default. Credentials are sealed at rest (internal/secretbox)
// and only decrypted here, at construction time.
package tenantgw

import (
	"context"
	"encoding/json"
	"log/slog"

	"b2bcommerce/internal/payments/gateway"
	"b2bcommerce/internal/secretbox"
	"b2bcommerce/internal/store/gen"
)

// Factory builds a gateway from a tenant's decrypted credentials.
type Factory func(creds map[string]string) gateway.Gateway

var factories = map[string]Factory{
	"mock": func(map[string]string) gateway.Gateway { return gateway.Mock{} },
}

// Resolver answers "which gateway charges this org?".
type Resolver struct {
	Q       *gen.Queries
	Box     *secretbox.Box
	Default gateway.Gateway
	Logger  *slog.Logger
}

// For returns the org's configured gateway, or the platform default when the
// org has no config, the configured name has no adapter, or credentials fail to
// decrypt (key rotation gone wrong should degrade, not take checkout down).
func (r *Resolver) For(ctx context.Context, orgID int64) gateway.Gateway {
	cfg, err := r.Q.GetOrgPaymentConfig(ctx, orgID)
	if err != nil {
		return r.Default
	}
	factory, ok := factories[cfg.Gateway]
	if !ok {
		r.warn("org payment gateway has no adapter; using platform default", orgID, cfg.Gateway)
		return r.Default
	}
	creds := map[string]string{}
	if len(cfg.CredentialsEnc) > 0 && r.Box != nil {
		plain, err := r.Box.Open(cfg.CredentialsEnc)
		if err != nil {
			r.warn("org payment credentials failed to decrypt; using platform default", orgID, cfg.Gateway)
			return r.Default
		}
		if err := json.Unmarshal(plain, &creds); err != nil {
			r.warn("org payment credentials are not a JSON object; using platform default", orgID, cfg.Gateway)
			return r.Default
		}
	}
	return factory(creds)
}

func (r *Resolver) warn(msg string, orgID int64, gw string) {
	if r.Logger != nil {
		r.Logger.Warn(msg, "org_id", orgID, "gateway", gw)
	}
}

// Known reports whether a gateway name has a registered adapter (used by the
// settings endpoint to validate input).
func Known(name string) bool {
	_, ok := factories[name]
	return ok
}

// Names lists the registered adapter names (for the admin UI's gateway picker).
func Names() []string {
	out := make([]string, 0, len(factories))
	for n := range factories {
		out = append(out, n)
	}
	return out
}
