package postgres

import (
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jamersom/b3-data-hub/internal/application/ports/outbound"
)

var historicalQuoteColumns = []string{
	"import_id", "line_number", "record_sha256", "trading_date", "bdi_code", "ticker", "market_type",
	"short_name", "specification", "term", "currency", "open_price", "high_price",
	"low_price", "average_price", "close_price", "best_bid_price", "best_ask_price",
	"trade_count", "traded_quantity", "traded_volume", "strike_price", "option_indicator",
	"expiration_date", "quote_factor", "strike_points", "isin", "distribution_number",
}

type historicalQuoteRow struct {
	ImportID           int64
	LineNumber         int
	RecordSHA256       string
	TradingDate        time.Time
	BDICode            string
	Ticker             string
	MarketType         int
	ShortName          string
	Specification      string
	Term               *string
	Currency           string
	OpenPrice          pgtype.Numeric
	HighPrice          pgtype.Numeric
	LowPrice           pgtype.Numeric
	AveragePrice       pgtype.Numeric
	ClosePrice         pgtype.Numeric
	BestBidPrice       pgtype.Numeric
	BestAskPrice       pgtype.Numeric
	TradeCount         int
	TradedQuantity     int64
	TradedVolume       pgtype.Numeric
	StrikePrice        pgtype.Numeric
	OptionIndicator    *string
	ExpirationDate     *time.Time
	QuoteFactor        int
	StrikePoints       pgtype.Numeric
	ISIN               string
	DistributionNumber int
}

func newHistoricalQuoteRow(importID int64, record outbound.HistoricalQuoteRecord) historicalQuoteRow {
	quote := record.Quote

	return historicalQuoteRow{
		ImportID:           importID,
		LineNumber:         record.LineNumber,
		RecordSHA256:       record.RecordSHA256,
		TradingDate:        quote.TradingDate,
		BDICode:            quote.BDICode,
		Ticker:             quote.Ticker,
		MarketType:         quote.MarketType,
		ShortName:          quote.ShortName,
		Specification:      quote.Specification,
		Term:               nullableString(quote.Term),
		Currency:           quote.Currency,
		OpenPrice:          numeric(quote.OpenPriceCents, -2),
		HighPrice:          numeric(quote.HighPriceCents, -2),
		LowPrice:           numeric(quote.LowPriceCents, -2),
		AveragePrice:       numeric(quote.AveragePriceCents, -2),
		ClosePrice:         numeric(quote.ClosePriceCents, -2),
		BestBidPrice:       numeric(quote.BestBidPriceCents, -2),
		BestAskPrice:       numeric(quote.BestAskPriceCents, -2),
		TradeCount:         quote.TradeCount,
		TradedQuantity:     quote.TradedQuantity,
		TradedVolume:       numeric(quote.TradedVolumeCents, -2),
		StrikePrice:        numeric(quote.StrikePriceCents, -2),
		OptionIndicator:    nullableString(quote.OptionIndicator),
		ExpirationDate:     quote.ExpirationDate,
		QuoteFactor:        quote.QuoteFactor,
		StrikePoints:       numeric(quote.StrikePointsScaled, -6),
		ISIN:               quote.ISIN,
		DistributionNumber: quote.DistributionNumber,
	}
}

func (r historicalQuoteRow) values() []any {
	return []any{
		r.ImportID,
		r.LineNumber,
		r.RecordSHA256,
		r.TradingDate,
		r.BDICode,
		r.Ticker,
		r.MarketType,
		r.ShortName,
		r.Specification,
		r.Term,
		r.Currency,
		r.OpenPrice,
		r.HighPrice,
		r.LowPrice,
		r.AveragePrice,
		r.ClosePrice,
		r.BestBidPrice,
		r.BestAskPrice,
		r.TradeCount,
		r.TradedQuantity,
		r.TradedVolume,
		r.StrikePrice,
		r.OptionIndicator,
		r.ExpirationDate,
		r.QuoteFactor,
		r.StrikePoints,
		r.ISIN,
		r.DistributionNumber,
	}
}

// numeric converts a scaled integer to a PostgreSQL numeric value.
// For example, numeric(12638, -2) represents 126.38.
func numeric(value int64, exponent int32) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   big.NewInt(value),
		Exp:   exponent,
		Valid: true}
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
