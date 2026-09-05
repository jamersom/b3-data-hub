package domain

import (
	"os"
	"testing"
	"time"
)

func TestImportCycle(t *testing.T) {
	data, err := os.ReadFile("../../config/trading-calendar.json")
	if err != nil {
		t.Fatal(err)
	}
	calendar, err := NewTradingCalendar(data)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		now, date    string
		active, last bool
	}{
		{"2026-09-04T19:00:00-03:00", "2026-09-04", true, false},
		{"2026-09-04T21:00:00-03:00", "2026-09-04", true, false},
		{"2026-09-05T00:00:00-03:00", "2026-09-04", true, false},
		{"2026-09-05T09:00:00-03:00", "2026-09-04", true, true},
		{"2026-09-05T19:00:00-03:00", "2026-09-05", false, false},
		{"2026-09-07T19:00:00-03:00", "2026-09-07", false, false},
		{"2026-09-08T09:00:00-03:00", "2026-09-07", false, true},
		{"2026-09-08T19:00:00-03:00", "2026-09-08", true, false},
		{"2026-02-17T19:00:00-03:00", "2026-02-17", false, false},
		{"2026-02-18T19:00:00-03:00", "2026-02-18", true, false},
		{"2026-07-09T19:00:00-03:00", "2026-07-09", true, false},
		{"2026-12-31T00:00:00-03:00", "2026-12-30", true, false},
		{"2027-01-01T03:00:00Z", "2026-12-31", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.now, func(t *testing.T) {
			now, _ := time.Parse(time.RFC3339, tt.now)
			date, active, last, err := calendar.ImportCycle(now)
			if err != nil || date.Format(time.DateOnly) != tt.date || active != tt.active || last != tt.last {
				t.Fatalf("ciclo: %v %v %v %v", date, active, last, err)
			}
		})
	}
	now, _ := time.Parse(time.RFC3339, "2027-01-04T19:00:00-03:00")
	if _, _, _, err := calendar.ImportCycle(now); err == nil {
		t.Fatal("ano sem calendário deve falhar")
	}
}

func TestInvalidTradingCalendar(t *testing.T) {
	for _, data := range []string{`{}`, `null`, `{`, `{"2026":["2025-01-01"]}`, `{"2026":["2026-02-30"]}`} {
		if _, err := NewTradingCalendar([]byte(data)); err == nil {
			t.Fatalf("aceitou %s", data)
		}
	}
}
