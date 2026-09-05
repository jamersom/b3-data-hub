package usecases

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jamersom/b3-data-hub/internal/domain"
)

type scheduleStateFake struct {
	ready, locked    bool
	checks, releases int
	dates            []time.Time
	err              error
}

func (s *scheduleStateFake) HasPublishedTradingDate(_ context.Context, date time.Time) (bool, error) {
	s.checks++
	s.dates = append(s.dates, date)
	return s.ready, s.err
}
func (s *scheduleStateFake) TryAcquireImportLock(context.Context) (func(), bool, error) {
	if s.locked {
		return nil, false, nil
	}
	s.locked = true
	return func() { s.locked = false; s.releases++ }, true, nil
}

type scheduleImporterFake struct {
	calls, year int
	err         error
	publish     bool
	state       *scheduleStateFake
}

func (s *scheduleImporterFake) Execute(_ context.Context, year int) (ImportResult, error) {
	s.calls++
	s.year = year
	if s.publish && s.err == nil {
		s.state.ready = true
	}
	return ImportResult{}, s.err
}
func scheduleTime(value string) time.Time { date, _ := time.Parse(time.RFC3339, value); return date }
func scheduleService(t *testing.T, state *scheduleStateFake, importer *scheduleImporterFake, logs *bytes.Buffer) *ScheduledImportService {
	t.Helper()
	calendar, err := domain.NewTradingCalendar([]byte(`{"2025":[],"2026":["2026-09-07"]}`))
	if err != nil {
		t.Fatal(err)
	}
	return NewScheduledImportService(importer, state, calendar, slog.New(slog.NewJSONHandler(logs, nil)))
}

func TestScheduleStopsDownloadingAfterSuccessAt21(t *testing.T) {
	state := &scheduleStateFake{}
	importer := &scheduleImporterFake{state: state}
	var logs bytes.Buffer
	service := scheduleService(t, state, importer, &logs)
	for _, hour := range []string{"19", "20", "21", "22", "23"} {
		importer.publish = hour >= "21"
		if _, err := service.Execute(context.Background(), scheduleTime("2026-09-04T"+hour+":00:00-03:00")); err != nil {
			t.Fatal(err)
		}
	}
	// Outra instância representa o reinício do processo após a meia-noite.
	service = scheduleService(t, state, importer, &logs)
	for _, hour := range []string{"00", "03", "06", "09"} {
		outcome, err := service.Execute(context.Background(), scheduleTime("2026-09-05T"+hour+":00:00-03:00"))
		if err != nil || outcome != "trading_date_already_published" {
			t.Fatalf("%s %v", outcome, err)
		}
	}
	if importer.calls != 3 || state.releases != 9 {
		t.Fatalf("downloads=%d liberações=%d", importer.calls, state.releases)
	}
	for _, date := range state.dates {
		if date.Format(time.DateOnly) != "2026-09-04" {
			t.Fatal(date)
		}
	}
}

func TestScheduleFailuresAndSkips(t *testing.T) {
	for _, name := range []string{"stale", "download_error", "database_error", "busy", "holiday"} {
		t.Run(name, func(t *testing.T) {
			state := &scheduleStateFake{}
			importer := &scheduleImporterFake{state: state}
			var logs bytes.Buffer
			service := scheduleService(t, state, importer, &logs)
			now := scheduleTime("2026-09-05T09:00:00-03:00")
			switch name {
			case "download_error":
				importer.err = errors.New("HTTP 404")
			case "database_error":
				state.err = errors.New("database unavailable")
			case "busy":
				state.locked = true
			case "holiday":
				now = scheduleTime("2026-09-07T19:00:00-03:00")
			}
			outcome, err := service.Execute(context.Background(), now)
			switch name {
			case "stale":
				if outcome != "awaiting_updated_file" || !strings.Contains(logs.String(), "cycle exhausted") {
					t.Fatal(logs.String())
				}
			case "download_error":
				if !errors.Is(err, importer.err) || !strings.Contains(logs.String(), "cycle exhausted") {
					t.Fatalf("%v %s", err, logs.String())
				}
			case "database_error":
				if !errors.Is(err, state.err) || importer.calls != 0 {
					t.Fatal(err)
				}
			case "busy":
				if outcome != "import_in_progress" || importer.calls != 0 || state.checks != 0 {
					t.Fatal(outcome)
				}
			case "holiday":
				if outcome != "no_trading_session" || importer.calls != 0 || state.checks != 0 {
					t.Fatal(outcome)
				}
			}
			if name != "busy" && state.locked {
				t.Fatal("bloqueio não liberado")
			}
		})
	}
}

func TestScheduleUsesCycleYear(t *testing.T) {
	state := &scheduleStateFake{}
	importer := &scheduleImporterFake{state: state}
	var logs bytes.Buffer
	_, err := scheduleService(t, state, importer, &logs).Execute(context.Background(), scheduleTime("2026-01-01T00:00:00-03:00"))
	if err != nil || importer.year != 2025 {
		t.Fatalf("ano=%d erro=%v", importer.year, err)
	}
}
