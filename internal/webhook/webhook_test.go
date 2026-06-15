package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSign(t *testing.T) {
	// Deterministic and key-dependent.
	a := Sign("secret", []byte("body"))
	if a != Sign("secret", []byte("body")) {
		t.Fatal("Sign is not deterministic")
	}
	if a == Sign("other", []byte("body")) {
		t.Fatal("signature did not change with the secret")
	}
	if a == Sign("secret", []byte("body2")) {
		t.Fatal("signature did not change with the body")
	}
}

func TestDeliverSignsBodyAndSetsHeaders(t *testing.T) {
	const secret = "whsec_test"
	const event = "order.placed"
	body := []byte(`{"event":"order.placed"}`)

	var gotSig, gotEvent string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get(SignatureHeader)
		gotEvent = r.Header.Get(EventHeader)
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDelivererWithClient(srv.Client())
	status, err := d.Deliver(context.Background(), srv.URL, secret, event, body)
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if gotEvent != event {
		t.Fatalf("event header = %q, want %q", gotEvent, event)
	}
	if want := Sign(secret, gotBody); gotSig != want {
		t.Fatalf("signature = %q, want %q", gotSig, want)
	}
}

func TestDeliverNon2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := NewDelivererWithClient(srv.Client())
	status, err := d.Deliver(context.Background(), srv.URL, "s", "e", []byte("{}"))
	if err == nil {
		t.Fatal("expected an error on 500")
	}
	if status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", status)
	}
}
