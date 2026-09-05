package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jamersom/b3-data-hub/internal/application/ports/outbound"
	"github.com/jamersom/b3-data-hub/internal/domain"
)

type HistoricalImporter interface {
	Execute(ctx context.Context, year int) (ImportResult, error)
}

type ScheduledImportService struct {
	importer HistoricalImporter
	state    outbound.ScheduledImportState
	calendar *domain.TradingCalendar
	logger   *slog.Logger
}

func NewScheduledImportService(importer HistoricalImporter, state outbound.ScheduledImportState, calendar *domain.TradingCalendar, logger *slog.Logger) *ScheduledImportService {
	return &ScheduledImportService{importer: importer, state: state, calendar: calendar, logger: logger}
}

func (s *ScheduledImportService) Execute(ctx context.Context, now time.Time) (string, error) {
	date, active, last, err := s.calendar.ImportCycle(now)
	if err != nil {
		return "", err
	}
	logger := s.logger.With("expected_trading_date", date.Format(time.DateOnly), "last_attempt", last)
	if !active {
		logger.InfoContext(ctx, "scheduled import skipped", "reason", "no_trading_session")
		return "no_trading_session", nil
	}
	release, acquired, err := s.state.TryAcquireImportLock(ctx)
	if err != nil {
		return "", err
	}
	if !acquired {
		logger.InfoContext(ctx, "scheduled import skipped", "reason", "import_in_progress")
		return "import_in_progress", nil
	}
	defer release()
	ready, err := s.state.HasPublishedTradingDate(ctx, date)
	if err != nil {
		return "", err
	}
	if ready {
		logger.InfoContext(ctx, "scheduled import skipped", "reason", "trading_date_already_published")
		return "trading_date_already_published", nil
	}
	logger.InfoContext(ctx, "scheduled import attempt started")
	if _, err := s.importer.Execute(ctx, date.Year()); err != nil {
		logger.ErrorContext(ctx, "scheduled import attempt failed", "error", err)
		if last {
			logger.ErrorContext(ctx, "scheduled import cycle exhausted")
		}
		return "", fmt.Errorf("scheduled import for %s: %w", date.Format(time.DateOnly), err)
	}
	ready, err = s.state.HasPublishedTradingDate(ctx, date)
	if err != nil {
		return "", err
	}
	if !ready {
		logger.WarnContext(ctx, "scheduled import awaiting updated file")
		if last {
			logger.ErrorContext(ctx, "scheduled import cycle exhausted")
		}
		return "awaiting_updated_file", nil
	}
	logger.InfoContext(ctx, "scheduled import cycle completed")
	return "cycle_completed", nil
}
