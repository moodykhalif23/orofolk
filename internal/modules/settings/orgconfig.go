package settings

import (
	"encoding/json"
	"net/http"
	"strings"

	"b2bcommerce/internal/payments/tenantgw"
	"b2bcommerce/internal/server/response"
	"b2bcommerce/internal/store/gen"
)

// Org-scoped commerce identity (SAAS.md #4): payment gateway + credentials
// (dedicated encrypted table — config_settings would echo secrets from the
// admin list endpoint), and public storefront branding read from the
// branding.* config keys.

// ---- Payments ---------------------------------------------------------------

func (h *Handler) getPayments(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	out := map[string]any{
		"gateway":         "mock",
		"available":       tenantgw.Names(),
		"configured_keys": []string{},
	}
	cfg, err := h.q.GetOrgPaymentConfig(r.Context(), org)
	if err == nil {
		out["gateway"] = cfg.Gateway
		// Credential VALUES never leave the server — only which keys are stored.
		if len(cfg.CredentialsEnc) > 0 && h.box != nil {
			if plain, err := h.box.Open(cfg.CredentialsEnc); err == nil {
				var creds map[string]string
				if json.Unmarshal(plain, &creds) == nil {
					keys := make([]string, 0, len(creds))
					for k := range creds {
						keys = append(keys, k)
					}
					out["configured_keys"] = keys
				}
			}
		}
	}
	response.JSON(w, http.StatusOK, out)
}

func (h *Handler) putPayments(w http.ResponseWriter, r *http.Request) {
	org, ok := orgID(r)
	if !ok {
		response.Fail(w, http.StatusUnauthorized, "unauthorized", "no claims")
		return
	}
	var req struct {
		Gateway     string            `json:"gateway"`
		Credentials map[string]string `json:"credentials"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Fail(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	req.Gateway = strings.ToLower(strings.TrimSpace(req.Gateway))
	if !tenantgw.Known(req.Gateway) {
		response.Fail(w, http.StatusBadRequest, "bad_request",
			"unknown gateway — available: "+strings.Join(tenantgw.Names(), ", "))
		return
	}

	// nil credentials = keep what's stored (the upsert COALESCEs); a non-nil map
	// replaces them, and {} explicitly clears.
	var sealed []byte
	if req.Credentials != nil {
		if h.box == nil {
			response.Fail(w, http.StatusServiceUnavailable, "no_encryption",
				"credential encryption is not configured on this deployment (CONFIG_ENCRYPTION_KEY)")
			return
		}
		plain, err := json.Marshal(req.Credentials)
		if err != nil {
			response.Fail(w, http.StatusBadRequest, "bad_request", "invalid credentials")
			return
		}
		if sealed, err = h.box.Seal(plain); err != nil {
			response.Fail(w, http.StatusInternalServerError, "internal", "could not store credentials")
			return
		}
	}
	cfg, err := h.q.UpsertOrgPaymentConfig(r.Context(), gen.UpsertOrgPaymentConfigParams{
		OrganizationID: org, Gateway: req.Gateway, CredentialsEnc: sealed,
	})
	if err != nil {
		response.Fail(w, http.StatusInternalServerError, "internal", "could not save payment config")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"gateway": cfg.Gateway})
}

// ---- Storefront branding ----------------------------------------------------

// branding is the public storefront identity: resolved by serving host (like
// the catalog), read from the branding.* config keys with the website-scope
// cascade. Missing keys come back as empty strings — the storefront falls back
// to its defaults.
func (h *Handler) branding(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	org := int64(1)
	var website *int64
	if ws, err := h.q.GetWebsiteByDomain(r.Context(), host); err == nil {
		org = ws.OrganizationID
		website = &ws.ID
	}
	out := map[string]any{
		"store_name":  h.resolveString(r, org, "branding.store_name", website),
		"brand_color": h.resolveString(r, org, "branding.brand_color", website),
		"logo_url":    h.resolveString(r, org, "branding.logo_url", website),
		// Cart subtotal that unlocks free shipping (store currency). Empty when
		// the commerce.free_shipping_threshold setting is unset — the storefront
		// then shows no free-shipping meter.
		"free_shipping_threshold": h.resolveString(r, org, "commerce.free_shipping_threshold", website),
	}
	response.JSON(w, http.StatusOK, out)
}

func (h *Handler) resolveString(r *http.Request, org int64, key string, website *int64) string {
	val, _, found := h.resolve(r, org, key, website, nil, nil)
	if !found {
		return ""
	}
	var s string
	if err := json.Unmarshal(val, &s); err != nil {
		return ""
	}
	return s
}
