package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jamersom/b3-data-hub/internal/infra/config"
	"github.com/jamersom/b3-data-hub/internal/infra/database"
	"github.com/joho/godotenv"
)

func TestScheduledImportStateIntegration(t *testing.T) {
	if os.Getenv("DATABASE_INTEGRATION_TEST") != "1" {
		t.Skip("defina DATABASE_INTEGRATION_TEST=1")
	}
	_ = godotenv.Load("../../../../.env")
	cfg, err := config.LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repository := NewHistoricalQuoteRepository(pool)
	release, acquired, err := repository.TryAcquireImportLock(ctx)
	if err != nil || !acquired {
		t.Fatalf("primeiro bloqueio: %v %v", acquired, err)
	}
	defer release()
	second, acquired, err := repository.TryAcquireImportLock(ctx)
	if second != nil {
		second()
	}
	if err != nil || acquired {
		t.Fatalf("bloqueio concorrente: %v %v", acquired, err)
	}
	release()
	third, acquired, err := repository.TryAcquireImportLock(ctx)
	if err != nil || !acquired {
		t.Fatalf("bloqueio após liberação: %v %v", acquired, err)
	}
	defer third()
	var date time.Time
	if err := pool.QueryRow(ctx, "SELECT trading_date FROM published_historical_quotes ORDER BY trading_date DESC LIMIT 1").Scan(&date); err != nil {
		t.Fatal(err)
	}
	found, err := repository.HasPublishedTradingDate(ctx, date)
	if err != nil || !found {
		t.Fatalf("pregão publicado: %v %v", found, err)
	}
	found, err = repository.HasPublishedTradingDate(ctx, time.Date(1985, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || found {
		t.Fatalf("pregão ausente: %v %v", found, err)
	}
}
