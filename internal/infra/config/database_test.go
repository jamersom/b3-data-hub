package config

import (
	"errors"
	"testing"
	"time"
)

func TestDatabaseFromEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/database")
	t.Setenv("DB_MAX_CONNECTIONS", "20")
	t.Setenv("DB_MIN_CONNECTIONS", "3")
	t.Setenv("DB_MAX_CONN_LIFETIME", "45m")
	t.Setenv("DB_CONNECT_TIMEOUT", "7s")

	cfg, err := databaseFromEnvironment()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.MaxConnections != 20 || cfg.MinConnections != 3 {
		t.Fatalf("unexpected pool sizes: %+v", cfg)
	}
	if cfg.MaxConnLifetime != 45*time.Minute || cfg.ConnectTimeout != 7*time.Second {
		t.Fatalf("unexpected durations: %+v", cfg)
	}
}

func TestDatabaseURLIsRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := databaseFromEnvironment()
	if !errors.Is(err, ErrDatabaseURLRequired) {
		t.Fatalf("expected ErrDatabaseURLRequired, got %v", err)
	}
}

func TestMinimumConnectionsCannotExceedMaximum(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/database")
	t.Setenv("DB_MAX_CONNECTIONS", "2")
	t.Setenv("DB_MIN_CONNECTIONS", "3")

	if _, err := databaseFromEnvironment(); err == nil {
		t.Fatal("expected invalid pool size error")
	}
}
