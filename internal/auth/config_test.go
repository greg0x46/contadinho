package auth

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestMasterKeySources(t *testing.T) {
	key := bytes.Repeat([]byte{3}, 32)
	encoded := base64.StdEncoding.EncodeToString(key)
	file := filepath.Join(t.TempDir(), "master.key")
	if err := os.WriteFile(file, []byte(encoded+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONTADINHO_MASTER_KEY", "")
	t.Setenv("CONTADINHO_MASTER_KEY_FILE", "")
	if _, err := LoadMasterKey(); err == nil {
		t.Fatal("missing key accepted")
	}
	t.Setenv("CONTADINHO_MASTER_KEY_FILE", file)
	got, err := LoadMasterKey()
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("file source: %v", err)
	}
	t.Setenv("CONTADINHO_MASTER_KEY", encoded)
	if _, err := LoadMasterKey(); err == nil {
		t.Fatal("ambiguous sources accepted")
	}
	t.Setenv("CONTADINHO_MASTER_KEY_FILE", "")
	got, err = LoadMasterKey()
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("env source: %v", err)
	}
	for _, invalid := range []string{"secret", base64.StdEncoding.EncodeToString(make([]byte, 16))} {
		t.Setenv("CONTADINHO_MASTER_KEY", invalid)
		if _, err := LoadMasterKey(); err == nil {
			t.Fatal("invalid key accepted")
		}
	}
}
