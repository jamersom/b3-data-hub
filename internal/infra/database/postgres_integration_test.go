package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jamersom/b3-data-hub/internal/infra/config"
)

func TestNewPoolIntegration(t *testing.T) {
	if os.Getenv("DATABASE_INTEGRATION_TEST") != "1" {
		t.Skip("set DATABASE_INTEGRATION_TEST=1 to run")
	}

	cfg, err := config.LoadDatabase()
	if err != nil {
		t.Fatalf("load database config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	var value int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&value); err != nil {
		t.Fatalf("query database: %v", err)
	}
	if value != 1 {
		t.Fatalf("expected 1, got %d", value)
	}
}
