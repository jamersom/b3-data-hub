package ports

import (
	"context"

	"github.com/jamersom/b3-data-hub/internal/core/domain"
)

// HistoricalQuoteSource is the input port used to obtain a COTAHIST file.
type HistoricalQuoteSource interface {
	Download(ctx context.Context, year int) (domain.HistoricalFile, error)
}

// FileStore is the output port used to persist the downloaded source file.
type FileStore interface {
	Save(ctx context.Context, file domain.HistoricalFile) (string, error)
}

type HistoricalQuoteRecord struct {
	LineNumber int
	Quote      domain.HistoricalQuote
}

// HistoricalQuoteParser converts a source file into quote records.
type HistoricalQuoteParser interface {
	Parse(ctx context.Context, file domain.HistoricalFile, consume func(HistoricalQuoteRecord) error) error
}

type HistoricalImportInput struct {
	ReferenceYear int
	FileName      string
	FileSHA256    string
}

type HistoricalImport struct {
	ID               int64
	AlreadyCompleted bool
	TotalRecords     int64
}

// HistoricalQuoteRepository is the output port used by the import service.
type HistoricalQuoteRepository interface {
	BeginImport(ctx context.Context, input HistoricalImportInput) (HistoricalImport, error)
	InsertBatch(ctx context.Context, importID int64, records []HistoricalQuoteRecord) error
	CompleteImport(ctx context.Context, importID, totalRecords int64) error
	FailImport(ctx context.Context, importID int64, cause error) error
}
