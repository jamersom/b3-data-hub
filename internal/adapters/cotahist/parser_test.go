package cotahist

import (
	"archive/zip"
	"bytes"
	"context"
	"testing"

	"github.com/jamersom/b3-data-hub/internal/application/ports/outbound"
	"github.com/jamersom/b3-data-hub/internal/domain"
)

func TestParserParsesDetailRecord(t *testing.T) {
	line := "012026010734MACY34      010MACY S      DRN          R$  000000001263800000000126380000000012300000000001243900000000123000000000000001000000001350000003000000000000000011000000000000136838000000000000009999123100000010000000000000BRMACYBDR000134"
	file := domain.HistoricalFile{Year: 2026, FileName: "COTAHIST_A2026.ZIP", Data: zipData(t, line)}

	var records []outbound.HistoricalQuoteRecord
	err := NewParser().Parse(context.Background(), file, func(record outbound.HistoricalQuoteRecord) error {
		records = append(records, record)
		return nil
	})
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record, got %d", len(records))
	}

	record := records[0]
	if record.LineNumber != 2 {
		t.Fatalf("expected source line 2, got %d", record.LineNumber)
	}
	quote := record.Quote
	if quote.Ticker != "MACY34" || quote.TradingDate.Format("2006-01-02") != "2026-01-07" {
		t.Fatalf("unexpected quote identity: %+v", quote)
	}
	if quote.OpenPriceCents != 12638 || quote.ClosePriceCents != 12300 {
		t.Fatalf("unexpected prices: open=%d close=%d", quote.OpenPriceCents, quote.ClosePriceCents)
	}
	if quote.ExpirationDate != nil {
		t.Fatalf("expected sentinel expiration date to become nil, got %v", quote.ExpirationDate)
	}
	if quote.ISIN != "BRMACYBDR000" {
		t.Fatalf("unexpected ISIN %q", quote.ISIN)
	}
}

func zipData(t *testing.T, detail string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("COTAHIST_A2026.TXT")
	if err != nil {
		t.Fatalf("create ZIP entry: %v", err)
	}
	header := "00COTAHIST.2026BOVESPA 20260807"
	if _, err := entry.Write([]byte(header + "\n" + detail + "\n")); err != nil {
		t.Fatalf("write ZIP entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return buffer.Bytes()
}
