package blob

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestFSStorePutGetDelete(t *testing.T) {
	s, err := NewFSStore(t.TempDir())
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	ctx := context.Background()

	url, err := s.Put(ctx, "org/1/orig/a.txt", strings.NewReader("hello"), "text/plain")
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if url != PublicPrefix+"/org/1/orig/a.txt" {
		t.Errorf("url = %q", url)
	}

	rc, err := s.Get(ctx, "org/1/orig/a.txt")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "hello" {
		t.Errorf("content = %q", b)
	}

	if err := s.Delete(ctx, "org/1/orig/a.txt"); err != nil {
		t.Errorf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "org/1/orig/a.txt"); err == nil {
		t.Error("expected get after delete to fail")
	}
	// Deleting a missing key is not an error.
	if err := s.Delete(ctx, "org/1/orig/a.txt"); err != nil {
		t.Errorf("delete missing: %v", err)
	}
}

func TestFSStoreRejectsTraversal(t *testing.T) {
	s, _ := NewFSStore(t.TempDir())
	ctx := context.Background()
	// A traversal key is cleaned to stay within root; reading it must not reach
	// outside files (it resolves to a path under root that doesn't exist).
	if _, err := s.Get(ctx, "../../../etc/passwd"); err == nil {
		t.Error("expected traversal key to fail (no such file under root)")
	}
}
