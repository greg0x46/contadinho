package settings_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"contadinho-go/internal/db"
	"contadinho-go/internal/settings"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestSetupThenVerifyPassword(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()

	configured, err := settings.IsConfigured(ctx, conn)
	if err != nil {
		t.Fatalf("IsConfigured: %v", err)
	}
	if configured {
		t.Fatal("should not be configured before Setup")
	}

	key, err := settings.Setup(ctx, conn, "correct horse battery staple")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if len(key) == 0 {
		t.Fatal("Setup returned an empty key")
	}

	configured, err = settings.IsConfigured(ctx, conn)
	if err != nil {
		t.Fatalf("IsConfigured: %v", err)
	}
	if !configured {
		t.Fatal("should be configured after Setup")
	}

	verifiedKey, err := settings.VerifyPassword(ctx, conn, "correct horse battery staple")
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if string(verifiedKey) != string(key) {
		t.Error("VerifyPassword returned a different key than Setup")
	}

	if _, err := settings.VerifyPassword(ctx, conn, "wrong password"); err == nil || !errors.Is(err, settings.ErrIncorrectPassword) {
		t.Errorf("wrong password: err = %v, want ErrIncorrectPassword", err)
	}
}

func TestSetupTwiceFails(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	if _, err := settings.Setup(ctx, conn, "first"); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if _, err := settings.Setup(ctx, conn, "second"); !errors.Is(err, settings.ErrAlreadyConfigured) {
		t.Errorf("second Setup: err = %v, want ErrAlreadyConfigured", err)
	}
}

func TestVerifyPasswordBeforeSetup(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	if _, err := settings.VerifyPassword(ctx, conn, "anything"); !errors.Is(err, settings.ErrNotConfigured) {
		t.Errorf("err = %v, want ErrNotConfigured", err)
	}
}

func TestSetAndGetEncryptedValue(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	key, err := settings.Setup(ctx, conn, "hunter2hunter2")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	if err := settings.Set(ctx, conn, "pluggy.client_secret", "s3cr3t", true, key); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, ok, err := settings.Get(ctx, conn, "pluggy.client_secret", key)
	if err != nil || !ok {
		t.Fatalf("Get: got=%q ok=%v err=%v", got, ok, err)
	}
	if got != "s3cr3t" {
		t.Errorf("Get = %q, want s3cr3t", got)
	}

	if _, _, err := settings.Get(ctx, conn, "pluggy.client_secret", nil); !errors.Is(err, settings.ErrLocked) {
		t.Errorf("Get without key: err = %v, want ErrLocked", err)
	}

	if err := settings.Set(ctx, conn, "pluggy.client_secret", "s3cr3t", true, nil); !errors.Is(err, settings.ErrLocked) {
		t.Errorf("Set without key: err = %v, want ErrLocked", err)
	}
}

func TestGetPlainValueNeedsNoKey(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	if err := settings.Set(ctx, conn, "sync.poll_interval_seconds", "5", false, nil); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok, err := settings.Get(ctx, conn, "sync.poll_interval_seconds", nil)
	if err != nil || !ok || got != "5" {
		t.Errorf("Get = %q ok=%v err=%v, want 5/true/nil", got, ok, err)
	}
}

func TestGetMissingKey(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	_, ok, err := settings.Get(ctx, conn, "does.not.exist", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Error("ok = true for a missing key")
	}
}

func TestSessionLifecycle(t *testing.T) {
	s := settings.NewSession()
	if _, unlocked := s.Key(); unlocked {
		t.Fatal("new session should start locked")
	}
	s.Unlock([]byte("key-material"))
	key, unlocked := s.Key()
	if !unlocked || string(key) != "key-material" {
		t.Errorf("after Unlock: key=%q unlocked=%v", key, unlocked)
	}
	s.Lock()
	if _, unlocked := s.Key(); unlocked {
		t.Error("session should be locked after Lock")
	}
}
