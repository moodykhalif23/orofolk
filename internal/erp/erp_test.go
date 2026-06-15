package erp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignVerify(t *testing.T) {
	body := []byte(`{"hello":"world"}`)
	sig := Sign("s3cr3t", body)
	if !Verify("s3cr3t", body, sig) {
		t.Error("valid signature should verify")
	}
	if Verify("s3cr3t", body, "deadbeef") {
		t.Error("bad signature must not verify")
	}
	if Verify("wrong", body, sig) {
		t.Error("wrong secret must not verify")
	}
}

func TestIdempotencyKeyStable(t *testing.T) {
	a := IdempotencyKey("order", 5, "upsert")
	b := IdempotencyKey("order", 5, "upsert")
	if a != b {
		t.Error("idempotency key must be stable")
	}
	if IdempotencyKey("order", 5, "upsert") == IdempotencyKey("invoice", 5, "upsert") {
		t.Error("different entities must differ")
	}
}

func TestPushSignsAndParses(t *testing.T) {
	var gotSig string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get(SignatureHeader)
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"external_id":"ERP-99"}`))
	}))
	defer srv.Close()

	body := []byte(`{"public_id":"abc"}`)
	res, err := NewConnector().Push(context.Background(), srv.URL, "shh", "key-1", body)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if res.ExternalID != "ERP-99" {
		t.Errorf("external id = %q", res.ExternalID)
	}
	if !Verify("shh", gotBody, gotSig) {
		t.Error("server should receive a valid signature")
	}
}

func TestPushNon2xxErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := NewConnector().Push(context.Background(), srv.URL, "x", "k", []byte("{}")); err == nil {
		t.Error("non-2xx should error")
	}
}
