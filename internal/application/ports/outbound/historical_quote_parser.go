package outbound

import (
	"context"

	"github.com/jamersom/b3-data-hub/internal/domain"
)

type HistoricalQuoteRecord struct {
	LineNumber int
	Quote      domain.HistoricalQuote
}

// HistoricalQuoteParser converts a source file into quote records.
type HistoricalQuoteParser interface {
	Parse(ctx context.Context, file domain.HistoricalFile, consume func(HistoricalQuoteRecord) error) error
}
