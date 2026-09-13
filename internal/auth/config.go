package auth

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct{ PublicURL string }

func (c Config) Validate() error {
	u, err := url.Parse(c.PublicURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.Opaque != "" {
		return fmt.Errorf("CONTADINHO_PUBLIC_URL deve ser uma origem, sem caminho")
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") {
		return nil
	}
	return fmt.Errorf("CONTADINHO_PUBLIC_URL exige HTTPS (HTTP somente em localhost)")
}
func (c Config) Secure() bool { return strings.HasPrefix(c.PublicURL, "https://") }

// LoadMasterKey accepts exactly one provider-neutral secret source.
func LoadMasterKey() ([]byte, error) {
	value, file := os.Getenv("CONTADINHO_MASTER_KEY"), os.Getenv("CONTADINHO_MASTER_KEY_FILE")
	if (value == "") == (file == "") {
		return nil, fmt.Errorf("defina exatamente uma de CONTADINHO_MASTER_KEY e CONTADINHO_MASTER_KEY_FILE")
	}
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("ler arquivo da chave: %w", err)
		}
		value = string(b)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("a chave deve conter 32 bytes em Base64")
	}
	return key, nil
}
