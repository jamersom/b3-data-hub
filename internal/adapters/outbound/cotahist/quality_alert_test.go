package cotahist

import (
	"context"
	"os"
	"testing"

	"github.com/jamersom/b3-data-hub/internal/application/ports/outbound"
	"github.com/jamersom/b3-data-hub/internal/domain"
)

func TestBCFF2020SourceRangeAnomaly(t *testing.T) {
	// Original line 113450: retain the source's 89.61 last price, below 90.00 low.
	line := "012020060812BCFF11      010FII BC FFII CI  ER       R$  000000000919000000000092500000000009000000000000916400000000089610000000008961000000000899006269000000000000080127000000000734339985000000000000009999123100000010000000000000BRBCFFCTF000228"
	quote, err := parseDetail(line)
	if err != nil {
		t.Fatal(err)
	}
	if err := quote.Normalize(); err != nil {
		t.Fatal(err)
	}
	if quote.ClosePriceCents != 8961 || quote.LowPriceCents != 9000 || len(quote.QualityAlerts()) != 1 {
		t.Fatalf("quote: %+v", quote)
	}
}

func TestLocal2020QualityScan(t *testing.T) {
	if os.Getenv("COTAHIST_LOCAL_SCAN") != "1" {
		t.Skip("set COTAHIST_LOCAL_SCAN=1 for a read-only scan of the local 2020 ZIP")
	}
	var count, alerts int
	err := NewParser().Parse(context.Background(), domain.HistoricalFile{Year: 2020, Path: "../../../../data/COTAHIST_A2020.ZIP"}, func(record outbound.HistoricalQuoteRecord) error {
		count++
		alerts += len(record.Quote.QualityAlerts())
		return nil
	})
	if err != nil {
		t.Fatalf("records=%d alerts=%d: %v", count, alerts, err)
	}
	t.Logf("records=%d quality_alerts=%d", count, alerts)
}
