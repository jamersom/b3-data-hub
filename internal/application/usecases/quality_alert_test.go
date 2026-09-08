package usecases

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/jamersom/b3-data-hub/internal/application/ports/outbound"
	"github.com/jamersom/b3-data-hub/internal/domain"
)

type qualityParser struct {
	records []outbound.HistoricalQuoteRecord
}

func (p qualityParser) Parse(ctx context.Context, file domain.HistoricalFile, consume func(outbound.HistoricalQuoteRecord) error) error {
	for _, record := range p.records {
		if err := consume(record); err != nil {
			return err
		}
	}
	return nil
}

type qualityRepository struct {
	outbound.HistoricalQuoteRepository
	records []outbound.HistoricalQuoteRecord
}

func (r *qualityRepository) InsertBatch(ctx context.Context, id int64, records []outbound.HistoricalQuoteRecord) error {
	r.records = append(r.records, records...)
	return nil
}

func TestQualityAlertDoesNotStopPersistence(t *testing.T) {
	quote := domain.HistoricalQuote{Ticker: "BCFF11", TradingDate: time.Date(2020, 6, 8, 0, 0, 0, 0, time.UTC), MarketType: 10, LowPriceCents: 9000, HighPriceCents: 9250, ClosePriceCents: 8961}
	bad := outbound.HistoricalQuoteRecord{LineNumber: 113450, RecordSHA256: "source-hash", Quote: quote}
	good := bad
	good.LineNumber++
	good.Quote.ClosePriceCents = 9100
	repo := &qualityRepository{}
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	service := &ImportHistoricalQuotesService{parser: qualityParser{[]outbound.HistoricalQuoteRecord{bad, good}}, repository: repo}
	total, err := service.persistQuotes(context.Background(), domain.HistoricalFile{FileName: "COTAHIST_A2020.ZIP"}, 4, logger)
	if err != nil || total != 2 || len(repo.records) != 2 || repo.records[0].Quote.ClosePriceCents != 8961 {
		t.Fatalf("total=%d err=%v records=%+v", total, err, repo.records)
	}
	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["level"] != "WARN" || entry["quality_code"] != domain.QualityCloseOutsideDailyRange || entry["ticker"] != "BCFF11" || entry["line_number"] != float64(113450) || entry["close_price_cents"] != float64(8961) || entry["import_id"] != float64(4) {
		t.Fatalf("alert: %v", entry)
	}
}
