package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jamersom/b3-data-hub/internal/domain"
)

type LocalFileStore struct {
	baseDir string
}

func NewLocalFileStore(baseDir string) *LocalFileStore {
	return &LocalFileStore{baseDir: baseDir}
}

func (s *LocalFileStore) Save(ctx context.Context, file domain.HistoricalFile) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return "", fmt.Errorf("create data directory: %w", err)
	}

	path := filepath.Join(s.baseDir, file.FileName)
	if err := os.Rename(file.Path, path); err != nil {
		return "", fmt.Errorf("commit file: %w", err)
	}

	return path, nil
}
