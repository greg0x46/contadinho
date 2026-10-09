package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

// Environment variables that tune the Postgres connection pool. SQLite
// ignores them: its pool stays pinned to a single connection.
const (
	envMaxOpenConns    = "JULIUS_DB_MAX_OPEN_CONNS"
	envMaxIdleConns    = "JULIUS_DB_MAX_IDLE_CONNS"
	envConnMaxLifetime = "JULIUS_DB_CONN_MAX_LIFETIME"
	envConnMaxIdleTime = "JULIUS_DB_CONN_MAX_IDLE_TIME"
)

const (
	defaultMaxOpenConns    = 10
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
)

// poolConfig holds the database/sql pool settings applied to Postgres
// connections. Zero durations mean "no limit", as in database/sql.
type poolConfig struct {
	MaxOpen     int
	MaxIdle     int
	MaxLifetime time.Duration
	MaxIdleTime time.Duration
}

// defaultPoolConfig keeps as many idle connections as open ones, so bursts
// reuse warm connections instead of reconnecting, and recycles them
// periodically so server-side restarts or failovers are picked up.
func defaultPoolConfig() poolConfig {
	return poolConfig{
		MaxOpen:     defaultMaxOpenConns,
		MaxIdle:     defaultMaxOpenConns,
		MaxLifetime: defaultConnMaxLifetime,
		MaxIdleTime: defaultConnMaxIdleTime,
	}
}

// poolConfigFromEnv reads the pool settings through getenv, falling back to
// the defaults for unset variables. Max idle defaults to the effective max
// open. Invalid values are errors naming the variable, never silently
// replaced by a default.
func poolConfigFromEnv(getenv func(string) string) (poolConfig, error) {
	cfg := defaultPoolConfig()

	if v := getenv(envMaxOpenConns); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return poolConfig{}, fmt.Errorf("%s: want an integer >= 1, got %q", envMaxOpenConns, v)
		}
		cfg.MaxOpen = n
	}
	cfg.MaxIdle = cfg.MaxOpen

	if v := getenv(envMaxIdleConns); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return poolConfig{}, fmt.Errorf("%s: want an integer >= 0, got %q", envMaxIdleConns, v)
		}
		cfg.MaxIdle = n
	}

	var err error
	if cfg.MaxLifetime, err = parsePoolDuration(getenv, envConnMaxLifetime, cfg.MaxLifetime); err != nil {
		return poolConfig{}, err
	}
	if cfg.MaxIdleTime, err = parsePoolDuration(getenv, envConnMaxIdleTime, cfg.MaxIdleTime); err != nil {
		return poolConfig{}, err
	}
	return cfg, nil
}

func parsePoolDuration(getenv func(string) string, name string, def time.Duration) (time.Duration, error) {
	v := getenv(name)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("%s: want a non-negative Go duration such as 30m (0 = unlimited), got %q", name, v)
	}
	return d, nil
}

// apply sets every pool limit on conn. database/sql caps max idle at max
// open on its own.
func (c poolConfig) apply(conn *sql.DB) {
	conn.SetMaxOpenConns(c.MaxOpen)
	conn.SetMaxIdleConns(c.MaxIdle)
	conn.SetConnMaxLifetime(c.MaxLifetime)
	conn.SetConnMaxIdleTime(c.MaxIdleTime)
}
