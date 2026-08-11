package ports

import (
	"context"
)

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

type HistoricalQuoteRepository interface {
	BeginImport(ctx context.Context, input HistoricalImportInput) (HistoricalImport, error)
	InsertBatch(ctx context.Context, importID int64, records []HistoricalQuoteRecord) error
	CompleteImport(ctx context.Context, importID, totalRecords int64) error
	FailImport(ctx context.Context, importID int64, cause error) error
}
