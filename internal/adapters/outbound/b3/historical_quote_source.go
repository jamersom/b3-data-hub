package b3

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/jamersom/b3-data-hub/internal/domain"
)

const (
	historicalURLPattern = "https://bvmf.bmfbovespa.com.br/InstDados/SerHist/COTAHIST_A%d.ZIP"
	maxResponseSize      = 512 << 20
)

type HistoricalQuoteSource struct {
	client *http.Client
}

func NewHistoricalQuoteSource(client *http.Client) *HistoricalQuoteSource {
	return &HistoricalQuoteSource{client: client}
}

func (s *HistoricalQuoteSource) Download(ctx context.Context, year int) (domain.HistoricalFile, error) {
	url := fmt.Sprintf(historicalURLPattern, year)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return domain.HistoricalFile{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "b3-data-hub/1.0")
	req.Header.Set("Accept", "application/zip,application/octet-stream,*/*")

	resp, err := s.client.Do(req)
	if err != nil {
		return domain.HistoricalFile{}, fmt.Errorf("request B3: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.HistoricalFile{}, fmt.Errorf("B3 returned HTTP %d", resp.StatusCode)
	}

	file, err := downloadToTemporaryFile(resp.Body, maxResponseSize)
	if err != nil {
		return domain.HistoricalFile{}, fmt.Errorf("read B3 response: %w", err)
	}

	historicalFile := domain.HistoricalFile{
		Year:        year,
		FileName:    fmt.Sprintf("COTAHIST_A%d.ZIP", year),
		ContentType: resp.Header.Get("Content-Type"),
		Path:        file.path,
		Size:        file.size,
		SHA256:      file.sha256,
		SourceURL:   url,
		Header:      file.header,
	}

	if err := historicalFile.Validate(); err != nil {
		_ = os.Remove(file.path)
		return domain.HistoricalFile{}, err
	}

	return historicalFile, nil
}

type temporaryDownload struct {
	path   string
	size   int64
	sha256 string
	header []byte
}

func downloadToTemporaryFile(reader io.Reader, maxSize int64) (result temporaryDownload, err error) {
	temp, err := os.CreateTemp("", "b3-data-hub-*.zip.part")
	if err != nil {
		return result, fmt.Errorf("create temporary file: %w", err)
	}

	path := temp.Name()

	defer func() {
		if closeErr := temp.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(path)
		}
	}()

	hash := sha256.New()
	limited := &io.LimitedReader{R: reader, N: maxSize + 1}
	size, err := io.Copy(io.MultiWriter(temp, hash), limited)

	if err != nil {
		return result, err
	}

	if size > maxSize {
		return result, fmt.Errorf("response exceeds the %d-byte limit", maxSize)
	}

	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return result, fmt.Errorf("seek temporary file: %w", err)
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(temp, header); err != nil {
		return result, fmt.Errorf("read temporary file header: %w", err)
	}

	return temporaryDownload{
		path:   path,
		size:   size,
		sha256: hex.EncodeToString(hash.Sum(nil)),
		header: header}, nil
}
