// Package blob is the storage abstraction for binary assets (DAM originals and
// renditions). The interface is deliberately S3-shaped so a local-filesystem
// store today can be swapped for an object store later without touching callers.
package blob

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PublicPrefix is the API route under which stored blobs are served. FSStore
// returns URLs of the form "<PublicPrefix>/<key>".
const PublicPrefix = "/media/file"

// Store persists and retrieves opaque blobs by key.
type Store interface {
	// Put writes r under key and returns the public URL the API serves it at.
	Put(ctx context.Context, key string, r io.Reader, contentType string) (url string, err error)
	// Get opens the blob at key for reading.
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete removes the blob at key (no error if absent).
	Delete(ctx context.Context, key string) error
}

// ErrInvalidKey is returned when a key would escape the store root.
var ErrInvalidKey = errors.New("blob: invalid key")

// FSStore is a filesystem-backed Store rooted at a directory. In a multi-node
// deployment the root must be a shared volume (or swap in an S3 implementation).
type FSStore struct {
	root string
}

// NewFSStore returns an FSStore rooted at dir, creating it if necessary.
func NewFSStore(dir string) (*FSStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	return &FSStore{root: abs}, nil
}

// resolve maps a key to an absolute path under root, rejecting traversal.
func (s *FSStore) resolve(key string) (string, error) {
	clean := filepath.Clean("/" + strings.TrimPrefix(key, "/")) // force-rooted, strips ..
	p := filepath.Join(s.root, clean)
	if p != s.root && !strings.HasPrefix(p, s.root+string(os.PathSeparator)) {
		return "", ErrInvalidKey
	}
	return p, nil
}

func (s *FSStore) Put(_ context.Context, key string, r io.Reader, _ string) (string, error) {
	p, err := s.resolve(key)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return PublicPrefix + "/" + strings.TrimPrefix(key, "/"), nil
}

func (s *FSStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

func (s *FSStore) Delete(_ context.Context, key string) error {
	p, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
