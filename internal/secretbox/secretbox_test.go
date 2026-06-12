package secretbox

import (
	"bytes"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	b, err := New("a deployment passphrase")
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte(`{"api_key":"sk_live_123","secret":"shhh"}`)
	blob, err := b.Seal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte("sk_live_123")) {
		t.Fatal("ciphertext leaks plaintext")
	}
	out, err := b.Open(blob)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, plain) {
		t.Fatalf("round trip: got %q", out)
	}
}

func TestSealIsNonDeterministic(t *testing.T) {
	b, _ := New("k")
	one, _ := b.Seal([]byte("same"))
	two, _ := b.Seal([]byte("same"))
	if bytes.Equal(one, two) {
		t.Fatal("two seals of the same plaintext must differ (random nonce)")
	}
}

func TestOpenRejectsTamperAndWrongKey(t *testing.T) {
	b, _ := New("right key")
	blob, _ := b.Seal([]byte("secret"))

	tampered := append([]byte{}, blob...)
	tampered[len(tampered)-1] ^= 0xff
	if _, err := b.Open(tampered); err == nil {
		t.Fatal("tampered blob must not open")
	}

	other, _ := New("wrong key")
	if _, err := other.Open(blob); err == nil {
		t.Fatal("wrong key must not open")
	}

	if _, err := b.Open([]byte("xx")); err == nil {
		t.Fatal("short blob must not open")
	}
}

func TestNewRejectsEmptyPassphrase(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("empty passphrase must be refused")
	}
}
