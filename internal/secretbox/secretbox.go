// Package secretbox encrypts small secrets (tenant gateway credentials) for
// at-rest storage with AES-256-GCM. The key is derived from a deployment-wide
// passphrase (CONFIG_ENCRYPTION_KEY) via SHA-256, so any passphrase works while
// the cipher always gets exactly 32 bytes. Each Seal uses a fresh random nonce,
// prepended to the ciphertext.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// Box seals and opens secrets with a fixed key.
type Box struct {
	aead cipher.AEAD
}

// New derives the AEAD key from the passphrase. An empty passphrase is refused —
// the caller decides its fallback (e.g. deriving from JWT_SECRET with a warning).
func New(passphrase string) (*Box, error) {
	if passphrase == "" {
		return nil, errors.New("secretbox: empty passphrase")
	}
	key := sha256.Sum256([]byte(passphrase))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Seal encrypts plaintext; the random nonce is prepended to the returned blob.
func (b *Box) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts a Seal blob; fails on tampering or a wrong key.
func (b *Box) Open(blob []byte) ([]byte, error) {
	ns := b.aead.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("secretbox: blob too short")
	}
	return b.aead.Open(nil, blob[:ns], blob[ns:], nil)
}
