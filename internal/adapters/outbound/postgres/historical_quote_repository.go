package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jamersom/b3-data-hub/internal/adapters/outbound/postgres/sqlcgen"
	"github.com/jamersom/b3-data-hub/internal/application/ports/outbound"
)

type HistoricalQuoteRepository struct {
	pool    *pgxpool.Pool
	queries *sqlcgen.Queries
}

func NewHistoricalQuoteRepository(pool *pgxpool.Pool) *HistoricalQuoteRepository {
	return &HistoricalQuoteRepository{
		pool:    pool,
		queries: sqlcgen.New(pool),
	}
}

func (r *HistoricalQuoteRepository) BeginImport(ctx context.Context, input outbound.HistoricalImportInput) (outbound.HistoricalImport, error) {
	if input.ReferenceYear < 1986 || input.ReferenceYear > math.MaxInt16 {
		return outbound.HistoricalImport{}, fmt.Errorf("reference year must be between 1986 and %d", math.MaxInt16)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return outbound.HistoricalImport{}, fmt.Errorf("begin import transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := r.queries.WithTx(tx)

	var existing outbound.HistoricalImport
	found, err := queries.GetHistoricalImportForUpdate(ctx, input.FileSHA256)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		existing.ID, err = queries.CreateHistoricalImport(ctx, sqlcgen.CreateHistoricalImportParams{
			ReferenceYear: int16(input.ReferenceYear),
			FileName:      input.FileName,
			FileSha256:    input.FileSHA256,
		})
		if err != nil {
			return outbound.HistoricalImport{}, fmt.Errorf("insert historical import: %w", err)
		}
	case err != nil:
		return outbound.HistoricalImport{}, fmt.Errorf("find historical import: %w", err)
	case found.Status == "completed":
		existing.ID = found.ID
		existing.TotalRecords = found.TotalRecords
		existing.AlreadyCompleted = true
		if err := tx.Commit(ctx); err != nil {
			return outbound.HistoricalImport{}, fmt.Errorf("commit existing import: %w", err)
		}
		return existing, nil
	default:
		existing.ID = found.ID
		existing.TotalRecords = found.TotalRecords
		if err := queries.DeleteHistoricalQuotesByImportID(ctx, existing.ID); err != nil {
			return outbound.HistoricalImport{}, fmt.Errorf("clear previous import quotes: %w", err)
		}
		if err := queries.RestartHistoricalImport(ctx, existing.ID); err != nil {
			return outbound.HistoricalImport{}, fmt.Errorf("restart historical import: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return outbound.HistoricalImport{}, fmt.Errorf("commit import start: %w", err)
	}
	return existing, nil
}

func (r *HistoricalQuoteRepository) InsertBatch(ctx context.Context, importID int64, records []outbound.HistoricalQuoteRecord) error {
	if len(records) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(records))
	for _, record := range records {
		row := newHistoricalQuoteRow(importID, record)
		rows = append(rows, row.values())
	}

	count, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"historical_quotes"},
		historicalQuoteColumns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("copy historical quotes: %w", err)
	}
	if count != int64(len(records)) {
		return fmt.Errorf("copy historical quotes: inserted %d of %d records", count, len(records))
	}
	return nil
}

func (r *HistoricalQuoteRepository) CompleteImport(ctx context.Context, importID, totalRecords int64) error {
	rowsAffected, err := r.queries.CompleteHistoricalImport(ctx, sqlcgen.CompleteHistoricalImportParams{
		ID:           importID,
		TotalRecords: totalRecords,
	})
	if err != nil {
		return fmt.Errorf("complete historical import: %w", err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("complete historical import: import %d is not processing", importID)
	}
	return nil
}

func (r *HistoricalQuoteRepository) FailImport(ctx context.Context, importID int64, cause error) error {
	message := "unknown import error"
	if cause != nil {
		message = cause.Error()
	}
	err := r.queries.FailHistoricalImport(ctx, sqlcgen.FailHistoricalImportParams{
		ID:           importID,
		ErrorMessage: &message,
	})
	if err != nil {
		return fmt.Errorf("fail historical import: %w", err)
	}
	return nil
}

var _ outbound.HistoricalQuoteRepository = (*HistoricalQuoteRepository)(nil)
