package webhandler

import (
	"testing"
	"time"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func TestMonthGrid_Structure(t *testing.T) {
	weeks := monthGrid(2026, time.July, nil)

	if len(weeks) == 0 {
		t.Fatal("expected at least one week")
	}
	for _, week := range weeks {
		if len(week) != 7 {
			t.Fatalf("week should have 7 days, got %d", len(week))
		}
		if week[0].Date.Weekday() != time.Sunday {
			t.Errorf("week should start on Sunday, got %s", week[0].Date.Weekday())
		}
		if week[6].Date.Weekday() != time.Saturday {
			t.Errorf("week should end on Saturday, got %s", week[6].Date.Weekday())
		}
	}

	first := weeks[0][0].Date
	last := weeks[len(weeks)-1][6].Date
	if first.After(time.Date(2026, time.July, 1, 0, 0, 0, 0, time.Local)) {
		t.Error("grid should start on or before the 1st of the month")
	}
	if last.Before(time.Date(2026, time.July, 31, 0, 0, 0, 0, time.Local)) {
		t.Error("grid should end on or after the last day of the month")
	}

	var prev time.Time
	for _, week := range weeks {
		for _, day := range week {
			if !prev.IsZero() && day.Date.Sub(prev) != 24*time.Hour {
				t.Fatalf("days should be consecutive: %v then %v", prev, day.Date)
			}
			prev = day.Date
		}
	}

	sawInMonth := false
	for _, week := range weeks {
		for _, day := range week {
			if day.Date.Month() == time.July && day.Date.Year() == 2026 {
				if !day.InMonth {
					t.Errorf("day %v is in July but InMonth is false", day.Date)
				}
				sawInMonth = true
			} else if day.InMonth {
				t.Errorf("day %v is not in July but InMonth is true", day.Date)
			}
		}
	}
	if !sawInMonth {
		t.Error("expected at least one in-month day")
	}
}

func TestMonthGrid_GamesPlacedOnCorrectDay(t *testing.T) {
	gamesByDay := map[string][]mydb.Game{
		"2026-07-15": {{ID: 42}},
	}
	weeks := monthGrid(2026, time.July, gamesByDay)

	found := false
	for _, week := range weeks {
		for _, day := range week {
			if day.Date.Format("2006-01-02") == "2026-07-15" {
				if len(day.Games) != 1 || day.Games[0].ID != 42 {
					t.Fatalf("expected game 42 on 2026-07-15, got %+v", day.Games)
				}
				found = true
			} else if len(day.Games) != 0 {
				t.Errorf("day %v should have no games, got %+v", day.Date, day.Games)
			}
		}
	}
	if !found {
		t.Fatal("did not find 2026-07-15 in the grid")
	}
}
