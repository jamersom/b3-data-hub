package ports

import (
	"context"

	"github.com/jamersom/b3-data-hub/internal/domain"
)

type FileStore interface {
	Save(ctx context.Context, file domain.HistoricalFile) (string, error)
}
