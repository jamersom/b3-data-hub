package b3

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/jamersom/b3-data-hub/internal/domain"
)

const historicalURLPattern = "https://bvmf.bmfbovespa.com.br/InstDados/SerHist/COTAHIST_A%d.ZIP"

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

	data, err := io.ReadAll(io.LimitReader(resp.Body, 512<<20))
	if err != nil {
		return domain.HistoricalFile{}, fmt.Errorf("read B3 response: %w", err)
	}

	return domain.HistoricalFile{
		Year:        year,
		FileName:    fmt.Sprintf("COTAHIST_A%d.ZIP", year),
		ContentType: resp.Header.Get("Content-Type"),
		Data:        data,
	}, nil
}
