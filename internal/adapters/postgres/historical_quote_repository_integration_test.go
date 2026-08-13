package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jamersom/b3-data-hub/internal/core/domain"
	"github.com/jamersom/b3-data-hub/internal/core/ports"
	"github.com/jamersom/b3-data-hub/internal/infra/config"
	"github.com/jamersom/b3-data-hub/internal/infra/database"
)

func TestHistoricalQuoteRepositoryIntegration(t *testing.T) {
	if os.Getenv("DATABASE_INTEGRATION_TEST") != "1" {
		t.Skip("set DATABASE_INTEGRATION_TEST=1 to run")
	}
	cfg, err := config.LoadDatabase()
	if err != nil {
		t.Fatalf("load database config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	checksum := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	_, _ = pool.Exec(ctx, "DELETE FROM historical_imports WHERE file_sha256 = $1", checksum)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM historical_imports WHERE file_sha256 = $1", checksum)
	})

	repository := NewHistoricalQuoteRepository(pool)
	batch, err := repository.BeginImport(ctx, ports.HistoricalImportInput{ReferenceYear: 2026, FileName: "test.zip", FileSHA256: checksum})
	if err != nil {
		t.Fatalf("begin import: %v", err)
	}
	date := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)
	record := ports.HistoricalQuoteRecord{LineNumber: 2, Quote: domain.HistoricalQuote{
		TradingDate: date, BDICode: "34", Ticker: "MACY34", MarketType: 10,
		ShortName: "MACY S", Specification: "DRN", Currency: "R$", OpenPriceCents: 12638,
		HighPriceCents: 12638, LowPriceCents: 12300, AveragePriceCents: 12439,
		ClosePriceCents: 12300, BestBidPriceCents: 100, BestAskPriceCents: 13500,
		TradeCount: 3, TradedQuantity: 11, TradedVolumeCents: 136838,
		QuoteFactor: 1, ISIN: "BRMACYBDR000", DistributionNumber: 134,
	}}
	if err := repository.InsertBatch(ctx, batch.ID, []ports.HistoricalQuoteRecord{record}); err != nil {
		t.Fatalf("insert batch: %v", err)
	}
	if err := repository.CompleteImport(ctx, batch.ID, 1); err != nil {
		t.Fatalf("complete import: %v", err)
	}

	var ticker string
	var openPrice string
	if err := pool.QueryRow(ctx, "SELECT ticker, open_price::text FROM historical_quotes WHERE import_id = $1", batch.ID).Scan(&ticker, &openPrice); err != nil {
		t.Fatalf("query quote: %v", err)
	}
	if ticker != "MACY34" || openPrice != "126.38" {
		t.Fatalf("unexpected stored quote: ticker=%q open=%q", ticker, openPrice)
	}

	replayed, err := repository.BeginImport(ctx, ports.HistoricalImportInput{ReferenceYear: 2026, FileName: "test.zip", FileSHA256: checksum})
	if err != nil {
		t.Fatalf("replay import: %v", err)
	}
	if !replayed.AlreadyCompleted || replayed.TotalRecords != 1 {
		t.Fatalf("expected completed replay, got %+v", replayed)
	}
}
