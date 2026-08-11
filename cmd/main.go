package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jamersom/b3-data-hub/internal/adapters/b3"
	"github.com/jamersom/b3-data-hub/internal/adapters/cotahist"
	"github.com/jamersom/b3-data-hub/internal/adapters/postgres"
	"github.com/jamersom/b3-data-hub/internal/adapters/storage"
	"github.com/jamersom/b3-data-hub/internal/application"
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
	fileStore := storage.NewLocalFileStore("./data")
	parser := cotahist.NewParser()
	repository := postgres.NewHistoricalQuoteRepository(databasePool)
	importer := application.NewImportHistoricalQuotesService(source, fileStore, parser, repository, logger)

	year := time.Now().Year()
	if len(os.Args) > 1 {
		if _, err := fmt.Sscanf(os.Args[1], "%d", &year); err != nil {
			return fmt.Errorf("invalid year %q: %w", os.Args[1], err)
		}
	}

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
