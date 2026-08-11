package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

const (
	levelEnvironment  = "LOG_LEVEL"
	formatEnvironment = "LOG_FORMAT"
)

// New creates the application logger from LOG_LEVEL and LOG_FORMAT.
func New(output io.Writer) (*slog.Logger, error) {
	level, err := parseLevel(os.Getenv(levelEnvironment))
	if err != nil {
		return nil, err
	}

	options := &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
	}

	var handler slog.Handler
	switch format := strings.ToLower(strings.TrimSpace(os.Getenv(formatEnvironment))); format {
	case "", "json":
		handler = slog.NewJSONHandler(output, options)
	case "text":
		handler = slog.NewTextHandler(output, options)
	default:
		return nil, fmt.Errorf("%s must be json or text", formatEnvironment)
	}

	return slog.New(handler).With("service", "b3-data-hub"), nil
}

func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%s must be debug, info, warn or error", levelEnvironment)
	}
}
