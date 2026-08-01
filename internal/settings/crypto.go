// Package settings stores application configuration — including Pluggy
// credentials — in the SQLite `settings` table instead of a .env file, per
// the project goal's "config e segredos" requirement. Values marked
// sensitive are encrypted at rest with a key derived (Argon2id) from a
// passphrase the user sets up once from the frontend; the derived key lives
// only in server process memory (see Session) and is never itself persisted.
package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	saltSize = 16
	keySize  = 32 // AES-256

	// Argon2id parameters. These sit within the RFC 9106 "recommended"
	// range for when a dedicated hashing accelerator isn't available: 64 MiB
	// of memory makes GPU/ASIC brute force meaningfully more expensive while
	// staying fast enough (well under a second) for a one-time unlock on
	// ordinary desktop hardware.
	argonTime    = 1
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 4
)

func generateSalt() ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return salt, nil
}

// deriveKey derives a 32-byte AES-256 key from password and salt. The same
// (password, salt) pair always yields the same key, so salt must be
// persisted (in app_auth) to unlock again later.
func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, keySize)
}

var errCiphertextTooShort = errors.New("ciphertext shorter than nonce")

// encrypt returns nonce||ciphertext, authenticated with AES-256-GCM under key.
func encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decrypt reverses encrypt. Returns an error (never a partial result) if key
// is wrong or data was tampered with — GCM authentication catches both.
func decrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	if len(data) < gcm.NonceSize() {
		return nil, errCiphertextTooShort
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
