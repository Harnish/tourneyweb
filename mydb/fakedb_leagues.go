package mydb

import (
	"sort"
	"time"
)

func (f *FakeDB) SetTournamentLeague(id int, kind string, endDate time.Time, leagueID int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.tournaments[id]; ok {
		t.Kind, t.EndDate, t.LeagueID = kind, endDate, leagueID
		f.tournaments[id] = t
	}
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (f *FakeDB) ReturnLeagues(today time.Time) []Tournament {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := dateOnly(today)
	var out []Tournament
	for _, t := range f.tournaments {
		if !t.IsLeague() || t.Status != "published" {
			continue
		}
		running := !t.EndDate.IsZero() && !dateOnly(t.EndDate).Before(d)
		openEnded := t.EndDate.IsZero() && !dateOnly(t.StartDate).Before(d.AddDate(0, -12, 0))
		if running || openEnded {
			out = append(out, t)
		}
	}
	sortTournamentsNewestFirst(out)
	return out
}

func (f *FakeDB) LeaguePlayoffs(leagueID int) []Tournament {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Tournament
	for _, t := range f.tournaments {
		if t.LeagueID == leagueID && leagueID != 0 {
			out = append(out, t)
		}
	}
	sortTournamentsNewestFirst(out)
	return out
}

func sortTournamentsNewestFirst(ts []Tournament) {
	sort.Slice(ts, func(i, j int) bool {
		if !ts[i].StartDate.Equal(ts[j].StartDate) {
			return ts[i].StartDate.After(ts[j].StartDate)
		}
		return ts[i].ID < ts[j].ID
	})
}
