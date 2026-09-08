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

func TestHistoricalQuoteWarnsOutsideRange(t *testing.T) {
	for _, close := range []int64{4200, 4700} {
		quote := validHistoricalQuote()
		quote.ClosePriceCents = close
		if err := quote.Normalize(); err != nil {
			t.Fatal(err)
		}
		alerts := quote.QualityAlerts()
		if len(alerts) != 1 || alerts[0] != QualityCloseOutsideDailyRange || quote.ClosePriceCents != close {
			t.Fatalf("unexpected quote/alerts: %+v %v", quote, alerts)
		}
	}
	for _, close := range []int64{4300, 4500, 4600} {
		quote := validHistoricalQuote()
		quote.ClosePriceCents = close
		if err := quote.Normalize(); err != nil {
			t.Fatal(err)
		}
		if len(quote.QualityAlerts()) != 0 {
			t.Fatal("unexpected alert for valid close")
		}
	}
}

func TestHistoricalQuoteStillRejectsInvalidData(t *testing.T) {
	for _, change := range []func(*HistoricalQuote){
		func(q *HistoricalQuote) { q.ClosePriceCents = -1 },
		func(q *HistoricalQuote) { q.HighPriceCents = q.LowPriceCents - 1 },
		func(q *HistoricalQuote) { q.OpenPriceCents = q.HighPriceCents + 1 },
	} {
		quote := validHistoricalQuote()
		change(&quote)
		if err := quote.Normalize(); err == nil {
			t.Fatal("expected fatal validation error")
		}
	}
}
