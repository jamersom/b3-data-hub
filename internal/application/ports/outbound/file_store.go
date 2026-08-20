package outbound

import (
	"context"

	"github.com/jamersom/b3-data-hub/internal/domain"
)

// FileStore is the output port used to persist the downloaded source file.
type FileStore interface {
	Save(ctx context.Context, file domain.HistoricalFile) (string, error)
}
