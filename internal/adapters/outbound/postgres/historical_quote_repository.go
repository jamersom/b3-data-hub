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
		err = tx.QueryRow(ctx, `
			INSERT INTO historical_imports (
				reference_year, file_name, file_sha256, file_size,
				source_url, parser_version, layout_version, status
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'processing')
			RETURNING id
		`,
			input.ReferenceYear,
			input.FileName,
			input.FileSHA256,
			input.FileSize,
			input.SourceURL,
			input.ParserVersion,
			input.LayoutVersion,
		).Scan(&existing.ID)
		if err != nil {
			return outbound.HistoricalImport{}, fmt.Errorf("insert historical import: %w", err)
		}
	case err != nil:
		return outbound.HistoricalImport{}, fmt.Errorf("find historical import: %w", err)
	case found.Status == "published":
		existing.ID = found.ID
		existing.TotalRecords = found.TotalRecords
		existing.AlreadyPublished = true
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

func (r *HistoricalQuoteRepository) PublishImport(ctx context.Context, importID, totalRecords int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin publish transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var referenceYear int
	if err := tx.QueryRow(ctx, `
		SELECT reference_year
		FROM historical_imports
		WHERE id = $1 AND status = 'processing'
		FOR UPDATE
	`, importID).Scan(&referenceYear); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("publish import: import %d is not processing", importID)
		}
		return fmt.Errorf("lock import for publication: %w", err)
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", referenceYear); err != nil {
		return fmt.Errorf("lock reference year for publication: %w", err)
	}

	rows, err := tx.Query(ctx, `
		SELECT id
		FROM historical_imports
		WHERE reference_year = $1 AND status = 'published' AND id <> $2
		FOR UPDATE
	`, referenceYear, importID)
	if err != nil {
		return fmt.Errorf("lock previous published imports: %w", err)
	}
	var previousIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan previous published import: %w", err)
		}
		previousIDs = append(previousIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate previous published imports: %w", err)
	}
	rows.Close()

	if len(previousIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			DELETE FROM historical_quotes
			WHERE import_id = ANY($1)
		`, previousIDs); err != nil {
			return fmt.Errorf("delete superseded quotes: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE historical_imports
			SET status = 'superseded', superseded_at = NOW()
			WHERE id = ANY($1)
		`, previousIDs); err != nil {
			return fmt.Errorf("supersede previous imports: %w", err)
		}
	}

	result, err := tx.Exec(ctx, `
		UPDATE historical_imports
		SET status = 'published', total_records = $2,
			completed_at = NOW(), published_at = NOW(), error_message = NULL
		WHERE id = $1 AND status = 'processing'
	`, importID, totalRecords)
	if err != nil {
		return fmt.Errorf("publish historical import: %w", err)
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("publish historical import: import %d is not processing", importID)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit historical import publication: %w", err)
	}
	return nil
}

func (r *HistoricalQuoteRepository) FailImport(ctx context.Context, importID int64, cause error) error {
	message := "unknown import error"
	if cause != nil {
		message = cause.Error()
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin fail import transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := r.queries.WithTx(tx)
	if err := queries.DeleteHistoricalQuotesByImportID(ctx, importID); err != nil {
		return fmt.Errorf("delete partial historical quotes: %w", err)
	}
	err = queries.FailHistoricalImport(ctx, sqlcgen.FailHistoricalImportParams{
		ID:           importID,
		ErrorMessage: &message,
	})
	if err != nil {
		return fmt.Errorf("fail historical import: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed import status: %w", err)
	}
	return nil
}

var _ outbound.HistoricalQuoteRepository = (*HistoricalQuoteRepository)(nil)
