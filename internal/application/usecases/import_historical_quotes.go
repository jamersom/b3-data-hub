package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jamersom/b3-data-hub/internal/application/ports/outbound"
	"github.com/jamersom/b3-data-hub/internal/domain"
)

const quoteBatchSize = 10000

type ImportHistoricalQuotesService struct {
	source     outbound.HistoricalQuoteSource
	fileStore  outbound.FileStore
	parser     outbound.HistoricalQuoteParser
	repository outbound.HistoricalQuoteRepository
	logger     *slog.Logger
}

type ImportResult struct {
	FilePath        string
	Size            int64
	Records         int64
	AlreadyImported bool
}

type preparedHistoricalFile struct {
	file     domain.HistoricalFile
	result   ImportResult
	checksum string
}

func NewImportHistoricalQuotesService(
	source outbound.HistoricalQuoteSource,
	fileStore outbound.FileStore,
	parser outbound.HistoricalQuoteParser,
	repository outbound.HistoricalQuoteRepository,
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

	prepared, err := s.prepareHistoricalFile(ctx, year, logger)
	if err != nil {
		return ImportResult{}, err
	}

	batch, err := s.repository.BeginImport(ctx, outbound.HistoricalImportInput{
		ReferenceYear: prepared.file.Year,
		FileName:      prepared.file.FileName,
		FileSHA256:    prepared.checksum,
	})
	if err != nil {
		return prepared.result, fmt.Errorf("begin historical import: %w", err)
	}
	if batch.AlreadyCompleted {
		prepared.result.Records = batch.TotalRecords
		prepared.result.AlreadyImported = true
		logger.InfoContext(ctx, "historical import skipped",
			slog.Int64("import_id", batch.ID),
			slog.Int64("records", batch.TotalRecords),
			slog.String("reason", "checksum_already_completed"),
			slog.Duration("duration", time.Since(startedAt)),
		)
		return prepared.result, nil
	}
	logger.InfoContext(ctx, "historical import registered", slog.Int64("import_id", batch.ID))

	total, err := s.persistQuotes(ctx, prepared.file, batch.ID, logger)
	if err != nil {
		return prepared.result, s.failImport(ctx, batch.ID, total, startedAt, err, logger)
	}
	if err := s.repository.CompleteImport(ctx, batch.ID, total); err != nil {
		return prepared.result, fmt.Errorf("complete historical import: %w", err)
	}

	prepared.result.Records = total
	logger.InfoContext(ctx, "historical import completed",
		slog.Int64("import_id", batch.ID),
		slog.Int64("records", total),
		slog.Duration("duration", time.Since(startedAt)),
	)
	return prepared.result, nil
}

func (s *ImportHistoricalQuotesService) prepareHistoricalFile(ctx context.Context, year int, logger *slog.Logger) (preparedHistoricalFile, error) {
	downloadStartedAt := time.Now()
	file, err := s.source.Download(ctx, year)
	if err != nil {
		return preparedHistoricalFile{}, fmt.Errorf("download historical quotes: %w", err)
	}
	logger.InfoContext(ctx, "historical file downloaded",
		slog.String("file_name", file.FileName),
		slog.Int64("size_bytes", file.Size),
		slog.Duration("duration", time.Since(downloadStartedAt)),
	)
	if err := file.Validate(); err != nil {
		return preparedHistoricalFile{}, fmt.Errorf("validate historical file: %w", err)
	}

	path, err := s.fileStore.Save(ctx, file)
	if err != nil {
		return preparedHistoricalFile{}, fmt.Errorf("save historical file: %w", err)
	}

	logger.InfoContext(ctx, "historical file stored",
		slog.String("file_name", file.FileName),
		slog.String("file_path", path),
		slog.String("file_sha256", file.SHA256),
	)
	file.Path = path

	return preparedHistoricalFile{
		file:     file,
		result:   ImportResult{FilePath: path, Size: file.Size},
		checksum: file.SHA256,
	}, nil
}

func (s *ImportHistoricalQuotesService) persistQuotes(ctx context.Context, file domain.HistoricalFile, importID int64, logger *slog.Logger) (int64, error) {
	records := make([]outbound.HistoricalQuoteRecord, 0, quoteBatchSize)
	var total int64
	flush := func() error {
		if len(records) == 0 {
			return nil
		}
		batchStartedAt := time.Now()
		if err := s.repository.InsertBatch(ctx, importID, records); err != nil {
			return err
		}
		total += int64(len(records))
		logger.DebugContext(ctx, "historical quote batch persisted",
			slog.Int64("import_id", importID),
			slog.Int("batch_records", len(records)),
			slog.Int64("total_records", total),
			slog.Duration("duration", time.Since(batchStartedAt)),
		)
		records = records[:0]
		return nil
	}

	err := s.parser.Parse(ctx, file, func(record outbound.HistoricalQuoteRecord) error {
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
		return total, err
	}
	return total, nil
}

func (s *ImportHistoricalQuotesService) failImport(
	ctx context.Context,
	importID int64,
	total int64,
	startedAt time.Time,
	importErr error,
	logger *slog.Logger,
) error {
	failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	statusErr := s.repository.FailImport(failCtx, importID, importErr)
	logger.ErrorContext(ctx, "historical import failed",
		slog.Int64("import_id", importID),
		slog.Int64("records", total),
		slog.Duration("duration", time.Since(startedAt)),
		slog.Any("error", importErr),
		slog.Any("status_update_error", statusErr),
	)
	return errors.Join(fmt.Errorf("import historical quotes: %w", importErr), statusErr)
}
