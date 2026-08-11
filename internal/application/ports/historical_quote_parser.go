package ports

import (
	"context"

	"github.com/jamersom/b3-data-hub/internal/domain"
)

type HistoricalQuoteRecord struct {
	LineNumber int
	Quote      domain.HistoricalQuote
}

type HistoricalQuoteParser interface {
	Parse(ctx context.Context, file domain.HistoricalFile, consume func(HistoricalQuoteRecord) error) error
}
