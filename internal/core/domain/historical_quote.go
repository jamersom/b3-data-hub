package domain

import "time"

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
