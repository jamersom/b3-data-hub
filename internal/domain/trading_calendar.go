package domain

import (
	"encoding/json"
	"fmt"
	"time"
	_ "time/tzdata"
)

// TradingCalendar contém os dias sem pregão por ano, conforme o calendário da B3.
// Anos ausentes são rejeitados para não presumir que feriados são dias de negociação.
type TradingCalendar struct {
	years    map[int]map[string]bool
	location *time.Location
}

func NewTradingCalendar(data []byte) (*TradingCalendar, error) {
	var years map[int][]string
	if err := json.Unmarshal(data, &years); err != nil {
		return nil, fmt.Errorf("parse trading calendar: %w", err)
	}
	if len(years) == 0 {
		return nil, fmt.Errorf("trading calendar must contain at least one year")
	}
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return nil, err
	}
	calendar := &TradingCalendar{years: make(map[int]map[string]bool), location: location}
	for year, dates := range years {
		calendar.years[year] = make(map[string]bool)
		for _, value := range dates {
			date, err := time.Parse(time.DateOnly, value)
			if err != nil || date.Year() != year {
				return nil, fmt.Errorf("invalid holiday %q for year %d", value, year)
			}
			calendar.years[year][value] = true
		}
	}
	return calendar, nil
}

// ImportCycle associa a madrugada ao pregão da véspera. Dias sem pregão não
// iniciam ciclos; a manhã de sábado ainda pertence ao ciclo de sexta-feira.
func (c *TradingCalendar) ImportCycle(now time.Time) (date time.Time, active bool, lastAttempt bool, err error) {
	local := now.In(c.location)
	date = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	if local.Hour() < 19 {
		date = date.AddDate(0, 0, -1)
	}
	holidays, found := c.years[date.Year()]
	if !found {
		return date, false, false, fmt.Errorf("trading calendar does not cover %d", date.Year())
	}
	active = date.Weekday() != time.Saturday && date.Weekday() != time.Sunday && !holidays[date.Format(time.DateOnly)]
	return date, active, local.Hour() >= 9 && local.Hour() < 19, nil
}
