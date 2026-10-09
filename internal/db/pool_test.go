package db

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestPoolConfigDefaults(t *testing.T) {
	cfg, err := poolConfigFromEnv(envFrom(nil))
	if err != nil {
		t.Fatalf("poolConfigFromEnv: %v", err)
	}
	want := poolConfig{MaxOpen: 10, MaxIdle: 10, MaxLifetime: 30 * time.Minute, MaxIdleTime: 5 * time.Minute}
	if cfg != want {
		t.Fatalf("defaults = %+v, want %+v", cfg, want)
	}
}

func TestPoolConfigFromEnvOverrides(t *testing.T) {
	cfg, err := poolConfigFromEnv(envFrom(map[string]string{
		envMaxOpenConns:    "20",
		envMaxIdleConns:    "4",
		envConnMaxLifetime: "1h",
		envConnMaxIdleTime: "90s",
	}))
	if err != nil {
		t.Fatalf("poolConfigFromEnv: %v", err)
	}
	want := poolConfig{MaxOpen: 20, MaxIdle: 4, MaxLifetime: time.Hour, MaxIdleTime: 90 * time.Second}
	if cfg != want {
		t.Fatalf("config = %+v, want %+v", cfg, want)
	}

	cfg, err = poolConfigFromEnv(envFrom(map[string]string{envMaxOpenConns: "25"}))
	if err != nil {
		t.Fatalf("poolConfigFromEnv: %v", err)
	}
	if cfg.MaxIdle != 25 {
		t.Fatalf("MaxIdle = %d, want it to follow MaxOpen (25)", cfg.MaxIdle)
	}
}

func TestPoolConfigFromEnvRejectsInvalid(t *testing.T) {
	cases := []struct {
		name, key, value string
	}{
		{"non-numeric max open", envMaxOpenConns, "abc"},
		{"zero max open", envMaxOpenConns, "0"},
		{"negative max open", envMaxOpenConns, "-1"},
		{"negative max idle", envMaxIdleConns, "-1"},
		{"non-numeric max idle", envMaxIdleConns, "many"},
		{"bad lifetime", envConnMaxLifetime, "30 minutes"},
		{"negative lifetime", envConnMaxLifetime, "-1m"},
		{"bad idle time", envConnMaxIdleTime, "5"},
		{"negative idle time", envConnMaxIdleTime, "-5m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := poolConfigFromEnv(envFrom(map[string]string{tc.key: tc.value}))
			if err == nil {
				t.Fatalf("%s=%q: expected an error", tc.key, tc.value)
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("error %q does not name %s", err, tc.key)
			}
		})
	}
}

func TestPoolConfigZeroDurationsDisable(t *testing.T) {
	cfg, err := poolConfigFromEnv(envFrom(map[string]string{
		envConnMaxLifetime: "0",
		envConnMaxIdleTime: "0s",
		envMaxIdleConns:    "0",
	}))
	if err != nil {
		t.Fatalf("poolConfigFromEnv: %v", err)
	}
	if cfg.MaxLifetime != 0 || cfg.MaxIdleTime != 0 || cfg.MaxIdle != 0 {
		t.Fatalf("config = %+v, want zero lifetime, idle time and idle conns", cfg)
	}
}

func TestPoolConfigApply(t *testing.T) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer conn.Close()

	poolConfig{MaxOpen: 7, MaxIdle: 3, MaxLifetime: time.Minute, MaxIdleTime: time.Second}.apply(conn)
	if got := conn.Stats().MaxOpenConnections; got != 7 {
		t.Fatalf("MaxOpenConnections = %d, want 7", got)
	}
}
