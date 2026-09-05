package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jamersom/b3-data-hub/internal/application/ports/outbound"
)

var _ outbound.ScheduledImportState = (*HistoricalQuoteRepository)(nil)

func (r *HistoricalQuoteRepository) HasPublishedTradingDate(ctx context.Context, date time.Time) (bool, error) {
	var found bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (
 SELECT 1 FROM published_historical_quotes q
 JOIN historical_imports i ON i.id = q.import_id
 WHERE q.trading_date = $1::date AND i.reference_year = $2
 )`, date, date.Year()).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("check published trading date: %w", err)
	}
	return found, nil
}

func (r *HistoricalQuoteRepository) TryAcquireImportLock(ctx context.Context) (func(), bool, error) {
	// Conexão exclusiva: não ocupa o pool durante o download e não devolve
	// uma sessão com bloqueio ativo ao pool. Fechá-la libera o advisory lock.
	connection, err := pgx.ConnectConfig(ctx, r.pool.Config().ConnConfig.Copy())
	if err != nil {
		return nil, false, fmt.Errorf("connect import lock: %w", err)
	}
	release := func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = connection.Close(closeCtx)
	}
	var acquired bool
	if err := connection.QueryRow(ctx, "SELECT pg_try_advisory_lock(42330001::bigint)").Scan(&acquired); err != nil {
		release()
		return nil, false, fmt.Errorf("acquire import lock: %w", err)
	}
	if !acquired {
		release()
		return nil, false, nil
	}
	return release, true, nil
}
