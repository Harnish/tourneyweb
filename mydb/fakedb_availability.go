package mydb

import (
	"sort"
	"time"
)

func (f *FakeDB) SetUmpireAvailability(userID, tournamentID int, note string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := [2]int{userID, tournamentID}
	a, existed := f.availability[k]
	if !existed {
		a = Availability{UserID: userID, TournamentID: tournamentID, CreatedAt: time.Now()}
	}
	a.Note = note
	f.availability[k] = a
	return !existed
}

func (f *FakeDB) ClearUmpireAvailability(userID, tournamentID int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.availability, [2]int{userID, tournamentID})
}

func (f *FakeDB) UmpireAvailabilityForUser(userID int) map[int]Availability {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int]Availability{}
	for _, a := range f.availability {
		if a.UserID == userID {
			out[a.TournamentID] = a
		}
	}
	return out
}

func (f *FakeDB) UmpireAvailabilityForTournament(tournamentID int) map[int]Availability {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int]Availability{}
	for _, a := range f.availability {
		if a.TournamentID == tournamentID {
			out[a.UserID] = a
		}
	}
	return out
}

func (f *FakeDB) ReturnTournamentsOpenForUmpires(from time.Time) []Tournament {
	f.mu.Lock()
	defer f.mu.Unlock()
	cutoff := from.Format("2006-01-02")
	var out []Tournament
	for _, t := range f.tournaments {
		if t.Status != "draft" && t.StartDate.Format("2006-01-02") >= cutoff {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].StartDate.Equal(out[j].StartDate) {
			return out[i].StartDate.Before(out[j].StartDate)
		}
		return out[i].ID < out[j].ID
	})
	return out
}
