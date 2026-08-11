package cotahist

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jamersom/b3-data-hub/internal/application/ports"
	"github.com/jamersom/b3-data-hub/internal/domain"
)

const detailRecordLength = 245

var ErrTXTEntryNotFound = errors.New("COTAHIST TXT entry not found in ZIP")

type Parser struct{}

func NewParser() *Parser { return &Parser{} }

func (p *Parser) Parse(ctx context.Context, file domain.HistoricalFile, consume func(ports.HistoricalQuoteRecord) error) error {
	archive, err := zip.NewReader(bytes.NewReader(file.Data), int64(len(file.Data)))
	if err != nil {
		return fmt.Errorf("open COTAHIST ZIP: %w", err)
	}

	entry := findTXTEntry(archive.File)
	if entry == nil {
		return ErrTXTEntryNotFound
	}

	reader, err := entry.Open()
	if err != nil {
		return fmt.Errorf("open COTAHIST TXT: %w", err)
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if err := ctx.Err(); err != nil {
			return err
		}

		line := scanner.Text()
		if len(line) < 2 || line[:2] != "01" {
			continue
		}
		quote, err := parseDetail(line)
		if err != nil {
			return fmt.Errorf("parse COTAHIST line %d: %w", lineNumber, err)
		}
		if quote.TradingDate.Year() != file.Year {
			return fmt.Errorf("line %d belongs to year %d, expected %d", lineNumber, quote.TradingDate.Year(), file.Year)
		}
		if err := consume(ports.HistoricalQuoteRecord{LineNumber: lineNumber, Quote: quote}); err != nil {
			return fmt.Errorf("consume COTAHIST line %d: %w", lineNumber, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan COTAHIST TXT: %w", err)
	}
	return nil
}

func findTXTEntry(files []*zip.File) *zip.File {
	for _, file := range files {
		if strings.HasSuffix(strings.ToUpper(file.Name), ".TXT") {
			return file
		}
	}
	return nil
}

func parseDetail(line string) (domain.HistoricalQuote, error) {
	if len(line) != detailRecordLength {
		return domain.HistoricalQuote{}, fmt.Errorf("invalid record length: got %d, expected %d", len(line), detailRecordLength)
	}

	tradingDate, err := parseDate(field(line, 2, 10), false)
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("trading date: %w", err)
	}
	expirationDate, err := parseDate(field(line, 202, 210), true)
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("expiration date: %w", err)
	}

	marketType, err := parseInt(field(line, 24, 27))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("market type: %w", err)
	}
	openPrice, err := parseInt64(field(line, 56, 69))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("open price: %w", err)
	}
	highPrice, err := parseInt64(field(line, 69, 82))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("high price: %w", err)
	}
	lowPrice, err := parseInt64(field(line, 82, 95))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("low price: %w", err)
	}
	averagePrice, err := parseInt64(field(line, 95, 108))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("average price: %w", err)
	}
	closePrice, err := parseInt64(field(line, 108, 121))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("close price: %w", err)
	}
	bestBid, err := parseInt64(field(line, 121, 134))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("best bid: %w", err)
	}
	bestAsk, err := parseInt64(field(line, 134, 147))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("best ask: %w", err)
	}
	tradeCount, err := parseInt(field(line, 147, 152))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("trade count: %w", err)
	}
	quantity, err := parseInt64(field(line, 152, 170))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("traded quantity: %w", err)
	}
	volume, err := parseInt64(field(line, 170, 188))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("traded volume: %w", err)
	}
	strikePrice, err := parseInt64(field(line, 188, 201))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("strike price: %w", err)
	}
	quoteFactor, err := parseInt(field(line, 210, 217))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("quote factor: %w", err)
	}
	strikePoints, err := parseInt64(field(line, 217, 230))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("strike points: %w", err)
	}
	distribution, err := parseInt(field(line, 242, 245))
	if err != nil {
		return domain.HistoricalQuote{}, fmt.Errorf("distribution number: %w", err)
	}

	return domain.HistoricalQuote{
		TradingDate: *tradingDate, BDICode: trim(field(line, 10, 12)), Ticker: trim(field(line, 12, 24)),
		MarketType: marketType, ShortName: trim(field(line, 27, 39)), Specification: trim(field(line, 39, 49)),
		Term: trim(field(line, 49, 52)), Currency: trim(field(line, 52, 56)), OpenPriceCents: openPrice,
		HighPriceCents: highPrice, LowPriceCents: lowPrice, AveragePriceCents: averagePrice, ClosePriceCents: closePrice,
		BestBidPriceCents: bestBid, BestAskPriceCents: bestAsk, TradeCount: tradeCount, TradedQuantity: quantity,
		TradedVolumeCents: volume, StrikePriceCents: strikePrice, OptionIndicator: trim(field(line, 201, 202)),
		ExpirationDate: expirationDate, QuoteFactor: quoteFactor, StrikePointsScaled: strikePoints,
		ISIN: trim(field(line, 230, 242)), DistributionNumber: distribution,
	}, nil
}

func field(line string, start, end int) string { return line[start:end] }
func trim(value string) string                 { return strings.TrimSpace(value) }

func parseInt(value string) (int, error) {
	parsed, err := strconv.Atoi(trim(value))
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
func parseInt64(value string) (int64, error) {
	parsed, err := strconv.ParseInt(trim(value), 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
func parseDate(value string, nullable bool) (*time.Time, error) {
	value = trim(value)
	if nullable && (value == "" || value == "00000000" || value == "99991231") {
		return nil, nil
	}
	parsed, err := time.Parse("20060102", value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

var _ ports.HistoricalQuoteParser = (*Parser)(nil)
