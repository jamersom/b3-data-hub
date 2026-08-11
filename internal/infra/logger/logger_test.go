package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNewCreatesJSONLoggerWithConfiguredLevel(t *testing.T) {
	t.Setenv(levelEnvironment, "debug")
	t.Setenv(formatEnvironment, "json")
	var output bytes.Buffer

	log, err := New(&output)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	log.Debug("import batch persisted", slog.Int64("import_id", 7))

	logged := output.String()
	if !strings.Contains(logged, `"level":"DEBUG"`) || !strings.Contains(logged, `"import_id":7`) {
		t.Fatalf("unexpected structured log: %s", logged)
	}
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	t.Setenv(levelEnvironment, "verbose")
	t.Setenv(formatEnvironment, "json")

	if _, err := New(&bytes.Buffer{}); err == nil {
		t.Fatal("expected invalid log level error")
	}
}
