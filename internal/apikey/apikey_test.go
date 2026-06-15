package apikey

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	raw, prefix, hash, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.HasPrefix(raw, Prefix) {
		t.Fatalf("raw key %q missing prefix %q", raw, Prefix)
	}
	if prefix != raw[:prefixLen] {
		t.Fatalf("prefix %q is not the first %d chars of %q", prefix, prefixLen, raw)
	}
	if hash != Hash(raw) {
		t.Fatalf("returned hash does not match Hash(raw)")
	}
	if hash == raw {
		t.Fatalf("hash must not equal the raw key")
	}
}

func TestGenerateUnique(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		raw, _, _, err := Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if _, dup := seen[raw]; dup {
			t.Fatalf("duplicate key generated: %q", raw)
		}
		seen[raw] = struct{}{}
	}
}

func TestHashDeterministic(t *testing.T) {
	const raw = "tgk_deadbeef"
	if Hash(raw) != Hash(raw) {
		t.Fatal("Hash is not deterministic")
	}
	if Hash(raw) == Hash(raw+"x") {
		t.Fatal("distinct inputs collided")
	}
}
