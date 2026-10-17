package mydb

import (
	"sort"
	"time"
)

func (f *FakeDB) SaveScoreReport(gameID int, reporter string, userID, home, away int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	for i, r := range f.scoreReports {
		if r.GameID == gameID && r.Reporter == reporter {
			f.scoreReports[i].UserID, f.scoreReports[i].HomeScore, f.scoreReports[i].AwayScore = userID, home, away
			f.scoreReports[i].UpdatedAt = now
			return
		}
	}
	f.scoreReports = append(f.scoreReports, ScoreReport{GameID: gameID, Reporter: reporter, UserID: userID,
		HomeScore: home, AwayScore: away, ReportedAt: now, UpdatedAt: now})
}

func (f *FakeDB) ScoreReportsForGame(gameID int) []ScoreReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []ScoreReport
	for _, r := range f.scoreReports {
		if r.GameID == gameID {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ReportedAt.Before(out[j].ReportedAt) })
	return out
}

func (f *FakeDB) SetGameScoreStatus(gameID int, status string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if g, ok := f.games[gameID]; ok {
		g.ScoreStatus = status
		f.games[gameID] = g
	}
}

func (f *FakeDB) ClearScoreReports(gameID int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	kept := f.scoreReports[:0]
	for _, r := range f.scoreReports {
		if r.GameID != gameID {
			kept = append(kept, r)
		}
	}
	f.scoreReports = kept
	if g, ok := f.games[gameID]; ok {
		switch g.ScoreStatus {
		case "reported", "confirmed", "disputed":
			g.ScoreStatus = ""
			f.games[gameID] = g
		}
	}
}
