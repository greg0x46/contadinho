package settings

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"contadinho-go/internal/db"
)

// verifierPlaintext is encrypted under the derived key and stored in
// app_auth.verifier purely so a later unlock attempt can confirm a candidate
// password derives the correct key (via GCM authentication failing on a
// wrong key) before trusting it to decrypt real settings.
const verifierPlaintext = "contadinho-settings-unlock-v1"

// Querier is satisfied by both *sql.DB and *sql.Tx.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

var (
	// ErrAlreadyConfigured is returned by Setup once app_auth already has a
	// row — the setup screen only runs once; changing the passphrase later
	// would need re-encrypting every existing sensitive value, which this
	// package does not implement yet.
	ErrAlreadyConfigured = errors.New("settings: already configured")
	// ErrNotConfigured is returned by VerifyPassword before Setup has run.
	ErrNotConfigured = errors.New("settings: not configured")
	// ErrIncorrectPassword is returned by VerifyPassword when password does
	// not derive the key app_auth was set up with.
	ErrIncorrectPassword = errors.New("settings: incorrect password")
)

// IsConfigured reports whether Setup has already run.
func IsConfigured(ctx context.Context, q Querier) (bool, error) {
	var exists int
	err := q.QueryRowContext(ctx, `SELECT 1 FROM app_auth WHERE id = 1`).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Setup derives a key from password, records a salt and verifier so future
// unlocks can confirm a password before trusting it, and returns the derived
// key so the caller can immediately encrypt initial settings (e.g. Pluggy
// credentials entered on the same setup screen) without asking the user to
// unlock again right after setting up.
func Setup(ctx context.Context, q Querier, password string) ([]byte, error) {
	configured, err := IsConfigured(ctx, q)
	if err != nil {
		return nil, err
	}
	if configured {
		return nil, ErrAlreadyConfigured
	}

	salt, err := generateSalt()
	if err != nil {
		return nil, err
	}
	key := deriveKey(password, salt)
	verifier, err := encrypt(key, []byte(verifierPlaintext))
	if err != nil {
		return nil, err
	}

	now := db.FormatTime(time.Now())
	_, err = q.ExecContext(ctx,
		`INSERT INTO app_auth (id, salt, verifier, created_at, updated_at) VALUES (1, ?, ?, ?, ?)`,
		base64.StdEncoding.EncodeToString(salt), base64.StdEncoding.EncodeToString(verifier), now, now,
	)
	if err != nil {
		return nil, err
	}
	return key, nil
}

// VerifyPassword derives a key from password and confirms it against the
// stored verifier, returning the key for the caller to hold (see Session)
// only when it's actually correct.
func VerifyPassword(ctx context.Context, q Querier, password string) ([]byte, error) {
	var saltB64, verifierB64 string
	err := q.QueryRowContext(ctx, `SELECT salt, verifier FROM app_auth WHERE id = 1`).Scan(&saltB64, &verifierB64)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotConfigured
	}
	if err != nil {
		return nil, err
	}
	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return nil, err
	}
	verifier, err := base64.StdEncoding.DecodeString(verifierB64)
	if err != nil {
		return nil, err
	}

	key := deriveKey(password, salt)
	plaintext, err := decrypt(key, verifier)
	if err != nil || string(plaintext) != verifierPlaintext {
		return nil, ErrIncorrectPassword
	}
	return key, nil
}
