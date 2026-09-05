package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jamersom/b3-data-hub/internal/adapters/outbound/b3"
	"github.com/jamersom/b3-data-hub/internal/adapters/outbound/cotahist"
	"github.com/jamersom/b3-data-hub/internal/adapters/outbound/postgres"
	"github.com/jamersom/b3-data-hub/internal/adapters/outbound/storage"
	"github.com/jamersom/b3-data-hub/internal/application/usecases"
	"github.com/jamersom/b3-data-hub/internal/domain"
	"github.com/jamersom/b3-data-hub/internal/infra/config"
	"github.com/jamersom/b3-data-hub/internal/infra/database"
	applicationlogger "github.com/jamersom/b3-data-hub/internal/infra/logger"
)

func main() {
	logger, err := applicationlogger.New(os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configure logger: %v\n", err)
		os.Exit(1)
	}

	if err := run(logger); err != nil {
		logger.Error("application stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	logger.Info("application started", slog.Duration("timeout", 30*time.Minute))

	databaseConfig, err := config.LoadDatabase()
	if err != nil {
		return fmt.Errorf("load database configuration: %w", err)
	}
	databasePool, err := database.NewPool(ctx, databaseConfig)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer databasePool.Close()
	logger.Info("database connection established",
		slog.Int("max_connections", int(databaseConfig.MaxConnections)),
		slog.Int("min_connections", int(databaseConfig.MinConnections)),
	)

	client := &http.Client{Timeout: 2 * time.Minute}
	source := b3.NewHistoricalQuoteSource(client)
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	fileStore := storage.NewLocalFileStore(dataDir)
	parser := cotahist.NewParser()
	repository := postgres.NewHistoricalQuoteRepository(databasePool)
	importer := usecases.NewImportHistoricalQuotesService(source, fileStore, parser, repository, logger)

	year := time.Now().Year()
	if len(os.Args) > 2 {
		return fmt.Errorf("usage: b3-data-hub [year | --scheduled]")
	}
	if len(os.Args) == 2 && os.Args[1] == "--scheduled" {
		calendarPath := os.Getenv("TRADING_CALENDAR_PATH")
		if calendarPath == "" {
			calendarPath = "config/trading-calendar.json"
		}
		data, err := os.ReadFile(calendarPath)
		if err != nil {
			return fmt.Errorf("read trading calendar: %w", err)
		}
		calendar, err := domain.NewTradingCalendar(data)
		if err != nil {
			return err
		}
		outcome, err := usecases.NewScheduledImportService(importer, repository, calendar, logger).Execute(ctx, time.Now())
		if err != nil {
			return err
		}
		logger.Info("application completed", "outcome", outcome)
		return nil
	}
	if len(os.Args) > 1 {
		year, err = strconv.Atoi(os.Args[1])
		if err != nil {
			return fmt.Errorf("invalid year %q: %w", os.Args[1], err)
		}
	}
	release, acquired, err := repository.TryAcquireImportLock(ctx)
	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("another import is already running")
	}
	defer release()

	result, err := importer.Execute(ctx, year)
	if err != nil {
		return err
	}
	abs, _ := filepath.Abs(result.FilePath)
	if result.AlreadyImported {
		logger.Info("application completed",
			slog.String("outcome", "already_imported"),
			slog.String("file_path", abs),
			slog.Int64("records", result.Records),
		)
		return nil
	}
	logger.Info("application completed",
		slog.String("outcome", "imported"),
		slog.String("file_path", abs),
		slog.Int64("size_bytes", result.Size),
		slog.Int64("records", result.Records),
	)
	return nil
}
