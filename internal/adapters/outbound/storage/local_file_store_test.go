package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jamersom/b3-data-hub/internal/domain"
)

func TestLocalFileStoreCopiesAndCommitsFile(t *testing.T) {
	sourceDir := t.TempDir()
	destinationDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "download.zip")
	if err := os.WriteFile(sourcePath, []byte("zip-content"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	store := NewLocalFileStore(destinationDir)
	path, err := store.Save(context.Background(), domain.HistoricalFile{
		Path: sourcePath, FileName: "COTAHIST_A2026.ZIP",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(content) != "zip-content" {
		t.Fatalf("unexpected content %q", content)
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		t.Fatalf("expected source removal, got %v", err)
	}
}
