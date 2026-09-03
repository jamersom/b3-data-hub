package outbound

import (
	"context"

	"github.com/jamersom/b3-data-hub/internal/domain"
)

// HistoricalQuoteSource is the output port used to obtain a COTAHIST file.
type HistoricalQuoteSource interface {
	Download(ctx context.Context, year int) (domain.HistoricalFile, error)
}
