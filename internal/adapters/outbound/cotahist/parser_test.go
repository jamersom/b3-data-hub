package cotahist

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jamersom/b3-data-hub/internal/application/ports/outbound"
	"github.com/jamersom/b3-data-hub/internal/domain"
)

func TestParserRejectsTrailerCountMismatch(t *testing.T) {
	line := "012026010734MACY34      010MACY S      DRN          R$  000000001263800000000126380000000012300000000001243900000000123000000000000001000000001350000003000000000000000011000000000000136838000000000000009999123100000010000000000000BRMACYBDR000134"
	path := writeZIP(t, zipDataWithTotal(t, line, 999))
	file := domain.HistoricalFile{Year: 2026, FileName: "COTAHIST_A2026.ZIP", Path: path}

	err := NewParser().Parse(context.Background(), file, func(outbound.HistoricalQuoteRecord) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "trailer declares") {
		t.Fatalf("expected trailer count error, got %v", err)
	}
}

func TestParserParsesDetailRecord(t *testing.T) {
	line := "012026010734MACY34      010MACY S      DRN          R$  000000001263800000000126380000000012300000000001243900000000123000000000000001000000001350000003000000000000000011000000000000136838000000000000009999123100000010000000000000BRMACYBDR000134"
	path := writeZIP(t, zipData(t, line))
	file := domain.HistoricalFile{Year: 2026, FileName: "COTAHIST_A2026.ZIP", Path: path}

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
	if len(record.RecordSHA256) != 64 {
		t.Fatalf("expected SHA-256 record identity, got %q", record.RecordSHA256)
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
	if quote.Currency != "BRL" {
		t.Fatalf("expected normalized currency BRL, got %q", quote.Currency)
	}
}

func writeZIP(t *testing.T, data []byte) string {
	t.Helper()
	path := t.TempDir() + "/cotahist.zip"
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write ZIP fixture: %v", err)
	}
	return path
}

func zipData(t *testing.T, detail string) []byte {
	return zipDataWithTotal(t, detail, 3)
}

func zipDataWithTotal(t *testing.T, detail string, total int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("COTAHIST_A2026.TXT")
	if err != nil {
		t.Fatalf("create ZIP entry: %v", err)
	}
	header := fixedRecord("00COTAHIST.2026BOVESPA 20260807")
	trailer := fixedRecord("99COTAHIST.2026BOVESPA 20260807" + fmt.Sprintf("%011d", total))
	if _, err := entry.Write([]byte(header + "\n" + detail + "\n" + trailer + "\n")); err != nil {
		t.Fatalf("write ZIP entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return buffer.Bytes()
}

func fixedRecord(value string) string {
	return value + strings.Repeat(" ", detailRecordLength-len(value))
}
