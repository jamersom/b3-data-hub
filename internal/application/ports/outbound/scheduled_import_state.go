package outbound

import (
	"context"
	"time"
)

// ScheduledImportState consulta dados publicados e protege a execução inteira.
// A função devolvida por TryAcquireImportLock deve liberar a conexão do bloqueio.
type ScheduledImportState interface {
	HasPublishedTradingDate(ctx context.Context, date time.Time) (bool, error)
	TryAcquireImportLock(ctx context.Context) (release func(), acquired bool, err error)
}
