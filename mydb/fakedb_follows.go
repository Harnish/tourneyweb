package mydb

import (
	"errors"
	"sort"
	"strings"
)

func (f *FakeDB) hasFollowLocked(userID int, kind string, targetID int) bool {
	for _, x := range f.follows {
		if x.UserID == userID && x.Kind == kind && x.TargetID == targetID {
			return true
		}
	}
	return false
}

func (f *FakeDB) AddFollow(userID int, kind string, targetID int) error {
	if !validFollowKind(kind) {
		return errors.New("unknown follow kind")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.hasFollowLocked(userID, kind, targetID) {
		return nil
	}
	n := 0
	for _, x := range f.follows {
		if x.UserID == userID {
			n++
		}
	}
	if n >= MaxFollows {
		return ErrFollowCap
	}
	f.follows = append(f.follows, Follow{UserID: userID, Kind: kind, TargetID: targetID})
	return nil
}

func (f *FakeDB) DelFollow(userID int, kind string, targetID int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.delFollowsLocked(func(x Follow) bool { return x.UserID == userID && x.Kind == kind && x.TargetID == targetID })
}

func (f *FakeDB) delFollowsLocked(drop func(Follow) bool) {
	kept := f.follows[:0]
	for _, x := range f.follows {
		if !drop(x) {
			kept = append(kept, x)
		}
	}
	f.follows = kept
}

func (f *FakeDB) IsFollowing(userID int, kind string, targetID int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hasFollowLocked(userID, kind, targetID)
}

func (f *FakeDB) UserFollows(userID int) []Follow {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Follow
	for _, x := range f.follows {
		if x.UserID != userID {
			continue
		}
		switch x.Kind {
		case "tournament":
			t, ok := f.tournaments[x.TargetID]
			if !ok {
				continue
			}
			x.Name, x.TournamentID = t.Name, t.ID
		case "division":
			d, ok := f.divisions[x.TargetID]
			if !ok {
				continue
			}
			x.Name, x.TournamentID = d.Name, d.TournamentID
		case "team":
			tm, ok := f.teams[x.TargetID]
			if !ok {
				continue
			}
			x.Name, x.TournamentID = tm.Name, tm.TournamentID
		}
		out = append(out, x)
	}
	rank := map[string]int{"tournament": 0, "division": 1, "team": 2}
	sort.Slice(out, func(i, j int) bool {
		if rank[out[i].Kind] != rank[out[j].Kind] {
			return rank[out[i].Kind] < rank[out[j].Kind]
		}
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].TargetID < out[j].TargetID
	})
	return out
}

func (f *FakeDB) GameFollowers(g Game) map[int]Follow {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int]Follow{}
	for _, x := range f.follows {
		isTeam := x.Kind == "team" && (x.TargetID == g.HomeTeam.ID || x.TargetID == g.AwayTeam.ID)
		isDiv := x.Kind == "division" && x.TargetID == g.Division.ID
		if !isTeam && !isDiv {
			continue
		}
		if cur, ok := out[x.UserID]; ok && cur.Kind == "team" {
			continue
		}
		x.TournamentID = g.TournamentID
		out[x.UserID] = x
	}
	return out
}

func (f *FakeDB) FollowersMatching(q string, tournamentID, divisionID, limit int) []User {
	q = strings.ToLower(strings.TrimSpace(q))
	if len([]rune(q)) < 2 || limit <= 0 {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []User
	for _, u := range f.users {
		if !strings.Contains(strings.ToLower(u.Name), q) {
			continue
		}
		for _, x := range f.follows {
			if x.UserID == u.ID && ((x.Kind == "tournament" && x.TargetID == tournamentID) || (x.Kind == "division" && x.TargetID == divisionID)) {
				out = append(out, u)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (f *FakeDB) SetMuteFollowAlerts(userID int, mute bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.muteFollow[userID] = mute
}

func (f *FakeDB) MutesFollowAlerts(userID int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.muteFollow[userID]
}

func (f *FakeDB) SearchTournaments(q string) []Tournament {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Tournament
	for _, t := range f.tournaments {
		if t.Status == "published" && strings.Contains(strings.ToLower(t.Name), q) {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].StartDate.Equal(out[j].StartDate) {
			return out[i].StartDate.After(out[j].StartDate)
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}
