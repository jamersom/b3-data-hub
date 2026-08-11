package domain

import "fmt"

type HistoricalFile struct {
	Year        int
	FileName    string
	ContentType string
	Data        []byte
}

func (f HistoricalFile) Validate() error {
	if f.Year < 1986 {
		return fmt.Errorf("historical data is unavailable for year %d", f.Year)
	}
	if len(f.Data) < 4 {
		return fmt.Errorf("historical file is too small")
	}
	if !isZIP(f.Data[:4]) {
		return fmt.Errorf("historical file is not a valid ZIP payload")
	}
	return nil
}

func isZIP(header []byte) bool {
	return len(header) >= 4 &&
		header[0] == 0x50 &&
		header[1] == 0x4B &&
		(header[2] == 0x03 || header[2] == 0x05 || header[2] == 0x07) &&
		(header[3] == 0x04 || header[3] == 0x06 || header[3] == 0x08)
}
