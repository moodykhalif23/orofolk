// Package erp is the ERP/accounting sync connector (Pack 2 §4.6). The built-in
// provider is a generic signed-webhook connector: outbound documents are HMAC
// signed and POSTed to the connection endpoint; inbound webhooks are verified
// with the same secret. A bespoke ERP (NetSuite, QuickBooks, …) implements the
// same small surface behind a connection row.
package erp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SignatureHeader carries the HMAC-SHA256 of the body, hex-encoded.
const SignatureHeader = "X-Teggo-Signature"

// Sign returns the hex HMAC-SHA256 of body under secret.
func Sign(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// Verify checks a provided signature against body (constant-time).
func Verify(secret string, body []byte, sig string) bool {
	want := Sign(secret, body)
	return hmac.Equal([]byte(want), []byte(sig))
}

// PushResult is the outcome of an outbound document push.
type PushResult struct {
	ExternalID string
	Status     int
}

// Connector pushes signed JSON documents to an ERP endpoint.
type Connector struct {
	client *http.Client
}

// NewConnector builds a connector with a sane HTTP timeout.
func NewConnector() *Connector {
	return &Connector{client: &http.Client{Timeout: 15 * time.Second}}
}

// NewConnectorWithClient injects a client (tests use httptest's).
func NewConnectorWithClient(c *http.Client) *Connector { return &Connector{client: c} }

// Push POSTs body to endpoint with an HMAC signature header and an idempotency
// key. A non-2xx is an error (the caller logs + retries on the next sweep). A
// JSON response may carry {"external_id": "..."} which is recorded in
// external_refs; absent that, the caller falls back to our public_id.
func (c *Connector) Push(ctx context.Context, endpoint, secret, idempotencyKey string, body []byte) (PushResult, error) {
	if endpoint == "" {
		return PushResult{}, fmt.Errorf("connection has no endpoint")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return PushResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, Sign(secret, body))
	req.Header.Set("Idempotency-Key", idempotencyKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return PushResult{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PushResult{Status: resp.StatusCode}, fmt.Errorf("erp returned %d", resp.StatusCode)
	}
	var parsed struct {
		ExternalID string `json:"external_id"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	return PushResult{ExternalID: parsed.ExternalID, Status: resp.StatusCode}, nil
}

// IdempotencyKey is a stable hash of (entity, operation) — safe to retry.
func IdempotencyKey(entityType string, entityID int64, operation string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", entityType, entityID, operation)))
	return hex.EncodeToString(sum[:])[:32]
}
