package domain

import (
	"testing"
	"time"
)

func validHistoricalQuote() HistoricalQuote {
	return HistoricalQuote{
		TradingDate:       time.Date(2026, time.January, 7, 0, 0, 0, 0, time.UTC),
		BDICode:           "02",
		Ticker:            "PETR4",
		MarketType:        10,
		Currency:          "R$  ",
		OpenPriceCents:    4400,
		HighPriceCents:    4600,
		LowPriceCents:     4300,
		AveragePriceCents: 4450,
		ClosePriceCents:   4500,
		QuoteFactor:       1,
		ISIN:              "BRPETRACNPR6",
	}
}

func TestHistoricalQuoteNormalize(t *testing.T) {
	quote := validHistoricalQuote()
	quote.Ticker = " petr4 "

	if err := quote.Normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if quote.Ticker != "PETR4" || quote.Currency != "BRL" {
		t.Fatalf("unexpected normalized quote: %+v", quote)
	}
}

func TestHistoricalQuoteRejectsInvalidPriceRange(t *testing.T) {
	quote := validHistoricalQuote()
	quote.ClosePriceCents = 4700

	if err := quote.Normalize(); err == nil {
		t.Fatal("expected price range validation error")
	}
}
