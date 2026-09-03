package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// HistoricalQuote represents a type-01 detail record from a COTAHIST file.
// Monetary fields are stored as scaled integers to avoid floating-point loss.
type HistoricalQuote struct {
	TradingDate        time.Time
	BDICode            string
	Ticker             string
	MarketType         int
	ShortName          string
	Specification      string
	Term               string
	Currency           string
	OpenPriceCents     int64
	HighPriceCents     int64
	LowPriceCents      int64
	AveragePriceCents  int64
	ClosePriceCents    int64
	BestBidPriceCents  int64
	BestAskPriceCents  int64
	TradeCount         int
	TradedQuantity     int64
	TradedVolumeCents  int64
	StrikePriceCents   int64
	OptionIndicator    string
	ExpirationDate     *time.Time
	QuoteFactor        int
	StrikePointsScaled int64
	ISIN               string
	DistributionNumber int
}

func (q *HistoricalQuote) Normalize() error {
	q.BDICode = strings.TrimSpace(q.BDICode)
	q.Ticker = strings.ToUpper(strings.TrimSpace(q.Ticker))
	q.ShortName = strings.TrimSpace(q.ShortName)
	q.Specification = strings.TrimSpace(q.Specification)
	q.Term = strings.TrimSpace(q.Term)
	q.OptionIndicator = strings.TrimSpace(q.OptionIndicator)
	q.ISIN = strings.ToUpper(strings.TrimSpace(q.ISIN))

	switch strings.ToUpper(strings.TrimSpace(q.Currency)) {
	case "R$", "BRL":
		q.Currency = "BRL"
	default:
		return fmt.Errorf("unsupported currency %q", q.Currency)
	}

	return q.Validate()
}

func (q HistoricalQuote) Validate() error {
	if q.TradingDate.IsZero() {
		return errors.New("trading date is required")
	}
	if q.Ticker == "" {
		return errors.New("ticker is required")
	}
	if q.BDICode == "" {
		return errors.New("BDI code is required")
	}
	if q.MarketType <= 0 {
		return errors.New("market type must be positive")
	}
	if q.Currency != "BRL" {
		return fmt.Errorf("currency must be BRL, got %q", q.Currency)
	}
	if len(q.ISIN) != 12 {
		return fmt.Errorf("ISIN must contain 12 characters, got %d", len(q.ISIN))
	}
	if q.QuoteFactor <= 0 {
		return errors.New("quote factor must be positive")
	}
	if q.HighPriceCents < q.LowPriceCents {
		return errors.New("high price is below low price")
	}
	if q.OpenPriceCents < q.LowPriceCents || q.OpenPriceCents > q.HighPriceCents {
		return errors.New("open price is outside daily range")
	}
	if q.ClosePriceCents < q.LowPriceCents || q.ClosePriceCents > q.HighPriceCents {
		return errors.New("close price is outside daily range")
	}
	if q.TradeCount < 0 || q.TradedQuantity < 0 || q.TradedVolumeCents < 0 {
		return errors.New("trade totals cannot be negative")
	}

	return nil
}
