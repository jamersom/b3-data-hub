package storage

import (
	"context"
	"fmt"
	"io"
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
	if err := copyAndCommit(ctx, file.Path, path); err != nil {
		return "", err
	}
	if err := os.Remove(file.Path); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("remove source file: %w", err)
	}

	return path, nil
}

func copyAndCommit(ctx context.Context, sourcePath, destinationPath string) (err error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer source.Close()

	temporary, err := os.CreateTemp(filepath.Dir(destinationPath), ".cotahist-*.part")
	if err != nil {
		return fmt.Errorf("create destination temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if err != nil {
			_ = os.Remove(temporaryPath)
		}
	}()

	if _, err = io.Copy(temporary, contextReader{ctx: ctx, reader: source}); err != nil {
		return fmt.Errorf("copy source file: %w", err)
	}
	if err = temporary.Sync(); err != nil {
		return fmt.Errorf("sync destination temporary file: %w", err)
	}
	if err = temporary.Close(); err != nil {
		return fmt.Errorf("close destination temporary file: %w", err)
	}
	if err = os.Rename(temporaryPath, destinationPath); err != nil {
		return fmt.Errorf("commit file: %w", err)
	}

	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
