package ports

import (
	"context"

	"github.com/jamersom/b3-data-hub/internal/domain"
)

type HistoricalQuoteSource interface {
	Download(ctx context.Context, year int) (domain.HistoricalFile, error)
}
