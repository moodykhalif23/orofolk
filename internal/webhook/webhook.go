// Package webhook delivers signed outbound event notifications to tenant-
// registered endpoints (Platform roadmap, Phase 0). The body is signed with the
// endpoint's secret (HMAC-SHA256, hex) in the X-Teggo-Signature header so the
// receiver can verify authenticity; X-Teggo-Event names the event. The transport
// mirrors the generic signed-webhook connector in internal/erp.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Headers carried on every delivery.
const (
	SignatureHeader = "X-Teggo-Signature"
	EventHeader     = "X-Teggo-Event"
)

// Sign returns the hex HMAC-SHA256 of body under secret.
func Sign(secret string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// Deliverer POSTs signed event bodies to endpoint URLs.
type Deliverer struct {
	client *http.Client
}

// NewDeliverer builds a deliverer with a sane HTTP timeout.
func NewDeliverer() *Deliverer {
	return &Deliverer{client: &http.Client{Timeout: 15 * time.Second}}
}

// NewDelivererWithClient injects a client (tests use httptest's).
func NewDelivererWithClient(c *http.Client) *Deliverer { return &Deliverer{client: c} }

// Deliver POSTs body to url with the signature + event headers. It returns the
// HTTP status; a non-2xx is an error (the caller records the attempt and lets
// the queue retry with backoff). A transport error returns status 0.
func (d *Deliverer) Deliver(ctx context.Context, url, secret, event string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SignatureHeader, Sign(secret, body))
	req.Header.Set(EventHeader, event)

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("webhook endpoint returned %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}
