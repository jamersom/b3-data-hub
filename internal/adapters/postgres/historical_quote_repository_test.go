package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/jamersom/b3-data-hub/internal/core/ports"
)

func TestBeginImportRejectsReferenceYearOutsideDatabaseRange(t *testing.T) {
	repository := &HistoricalQuoteRepository{}

	_, err := repository.BeginImport(context.Background(), ports.HistoricalImportInput{ReferenceYear: 1985})
	if err == nil || !strings.Contains(err.Error(), "reference year") {
		t.Fatalf("expected reference year validation error, got %v", err)
	}
}
