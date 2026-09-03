package postgres

import (
	"testing"
	"time"

	"github.com/jamersom/b3-data-hub/internal/application/ports/outbound"
	"github.com/jamersom/b3-data-hub/internal/domain"
)

func TestNewHistoricalQuoteRow(t *testing.T) {
	expirationDate := time.Date(2027, time.June, 21, 0, 0, 0, 0, time.UTC)
	record := outbound.HistoricalQuoteRecord{
		LineNumber: 42, RecordSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Quote: domain.HistoricalQuote{
			TradingDate: time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC),
			Ticker:      "PETR4", OpenPriceCents: 12638, Term: "",
			OptionIndicator: "A", ExpirationDate: &expirationDate,
			StrikePointsScaled: 1234567,
		},
	}

	row := newHistoricalQuoteRow(7, record)

	if row.ImportID != 7 || row.LineNumber != 42 || row.Ticker != "PETR4" {
		t.Fatalf("unexpected row identity: %+v", row)
	}
	if row.Term != nil {
		t.Fatalf("expected empty term to become NULL, got %q", *row.Term)
	}
	if row.OptionIndicator == nil || *row.OptionIndicator != "A" {
		t.Fatalf("unexpected option indicator: %v", row.OptionIndicator)
	}
	if !row.OpenPrice.Valid || row.OpenPrice.Int.Int64() != 12638 || row.OpenPrice.Exp != -2 {
		t.Fatalf("unexpected open price: %+v", row.OpenPrice)
	}
	if !row.StrikePoints.Valid || row.StrikePoints.Int.Int64() != 1234567 || row.StrikePoints.Exp != -6 {
		t.Fatalf("unexpected strike points: %+v", row.StrikePoints)
	}
	if len(row.values()) != len(historicalQuoteColumns) {
		t.Fatalf("columns and values differ: %d columns, %d values", len(historicalQuoteColumns), len(row.values()))
	}
}
