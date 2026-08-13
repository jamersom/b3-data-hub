package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jamersom/b3-data-hub/internal/core/ports"
)

const quoteBatchSize = 10000

type ImportHistoricalQuotesService struct {
	source     ports.HistoricalQuoteSource
	fileStore  ports.FileStore
	parser     ports.HistoricalQuoteParser
	repository ports.HistoricalQuoteRepository
	logger     *slog.Logger
}

type ImportResult struct {
	FilePath        string
	Size            int64
	Records         int64
	AlreadyImported bool
}

func NewImportHistoricalQuotesService(
	source ports.HistoricalQuoteSource,
	fileStore ports.FileStore,
	parser ports.HistoricalQuoteParser,
	repository ports.HistoricalQuoteRepository,
	logger *slog.Logger,
) *ImportHistoricalQuotesService {
	return &ImportHistoricalQuotesService{
		source: source, fileStore: fileStore, parser: parser, repository: repository, logger: logger,
	}
}

func (s *ImportHistoricalQuotesService) Execute(ctx context.Context, year int) (ImportResult, error) {
	startedAt := time.Now()
	logger := s.logger.With(slog.Int("reference_year", year))
	logger.InfoContext(ctx, "historical import started")

	downloadStartedAt := time.Now()
	file, err := s.source.Download(ctx, year)
	if err != nil {
		return ImportResult{}, fmt.Errorf("download historical quotes: %w", err)
	}
	logger.InfoContext(ctx, "historical file downloaded",
		slog.String("file_name", file.FileName),
		slog.Int("size_bytes", len(file.Data)),
		slog.Duration("duration", time.Since(downloadStartedAt)),
	)
	if err := file.Validate(); err != nil {
		return ImportResult{}, fmt.Errorf("validate historical file: %w", err)
	}

	path, err := s.fileStore.Save(ctx, file)
	if err != nil {
		return ImportResult{}, fmt.Errorf("save historical file: %w", err)
	}
	result := ImportResult{FilePath: path, Size: int64(len(file.Data))}

	checksum := sha256.Sum256(file.Data)
	checksumText := hex.EncodeToString(checksum[:])
	logger.InfoContext(ctx, "historical file stored",
		slog.String("file_name", file.FileName),
		slog.String("file_path", path),
		slog.String("file_sha256", checksumText),
	)
	batch, err := s.repository.BeginImport(ctx, ports.HistoricalImportInput{
		ReferenceYear: file.Year,
		FileName:      file.FileName,
		FileSHA256:    checksumText,
	})
	if err != nil {
		return result, fmt.Errorf("begin historical import: %w", err)
	}
	if batch.AlreadyCompleted {
		result.Records = batch.TotalRecords
		result.AlreadyImported = true
		logger.InfoContext(ctx, "historical import skipped",
			slog.Int64("import_id", batch.ID),
			slog.Int64("records", batch.TotalRecords),
			slog.String("reason", "checksum_already_completed"),
			slog.Duration("duration", time.Since(startedAt)),
		)
		return result, nil
	}
	logger.InfoContext(ctx, "historical import registered", slog.Int64("import_id", batch.ID))

	records := make([]ports.HistoricalQuoteRecord, 0, quoteBatchSize)
	var total int64
	flush := func() error {
		if len(records) == 0 {
			return nil
		}
		batchStartedAt := time.Now()
		if err := s.repository.InsertBatch(ctx, batch.ID, records); err != nil {
			return err
		}
		total += int64(len(records))
		logger.DebugContext(ctx, "historical quote batch persisted",
			slog.Int64("import_id", batch.ID),
			slog.Int("batch_records", len(records)),
			slog.Int64("total_records", total),
			slog.Duration("duration", time.Since(batchStartedAt)),
		)
		records = records[:0]
		return nil
	}

	err = s.parser.Parse(ctx, file, func(record ports.HistoricalQuoteRecord) error {
		records = append(records, record)
		if len(records) == quoteBatchSize {
			return flush()
		}
		return nil
	})
	if err == nil {
		err = flush()
	}
	if err != nil {
		failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		failErr := s.repository.FailImport(failCtx, batch.ID, err)
		logger.ErrorContext(ctx, "historical import failed",
			slog.Int64("import_id", batch.ID),
			slog.Int64("records", total),
			slog.Duration("duration", time.Since(startedAt)),
			slog.Any("error", err),
			slog.Any("status_update_error", failErr),
		)
		return result, errors.Join(fmt.Errorf("import historical quotes: %w", err), failErr)
	}

	if err := s.repository.CompleteImport(ctx, batch.ID, total); err != nil {
		return result, fmt.Errorf("complete historical import: %w", err)
	}
	result.Records = total
	logger.InfoContext(ctx, "historical import completed",
		slog.Int64("import_id", batch.ID),
		slog.Int64("records", total),
		slog.Duration("duration", time.Since(startedAt)),
	)
	return result, nil
}
