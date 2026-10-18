package webhandler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

func dayStr(offset int) string { return time.Now().AddDate(0, 0, offset).Format("2006-01-02") }

func (f *entryFx) createT(t *testing.T, u TWUser, kv ...string) (*mydb.Tournament, int) {
	t.Helper()
	base := []string{"name", "Spring Lg", "sport", "baseball", "location", "Park"}
	w := f.post(f.handler(f.env.NewTournament), u, append(base, kv...)...)
	for _, tt := range f.db.ReturnTournaments() {
		if tt.Name == "Spring Lg" {
			return &tt, w.Code
		}
	}
	return nil, w.Code
}

func TestCreateLeagueStoresKindAndEndDate(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	tt, code := f.createT(t, fan(f, t), "kind", "league", "start_date", dayStr(3), "end_date", dayStr(60))
	if code != http.StatusSeeOther || tt == nil || !tt.IsLeague() || tt.EndDate.Format("2006-01-02") != dayStr(60) {
		t.Fatalf("%d %+v", code, tt)
	}
}

func TestCreateLeagueDateRules(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	u := fan(f, t)
	if tt, code := f.createT(t, u, "kind", "league", "start_date", dayStr(-1)); code != http.StatusBadRequest || tt != nil {
		t.Fatalf("past start: %d", code)
	}
	if tt, code := f.createT(t, u, "kind", "league", "start_date", dayStr(5), "end_date", dayStr(4)); code != http.StatusBadRequest || tt != nil {
		t.Fatalf("end before start: %d", code)
	}
	if tt, code := f.createT(t, u, "kind", "league", "start_date", dayStr(5), "end_date", "garbage"); code != http.StatusBadRequest || tt != nil {
		t.Fatalf("bad end date: %d", code)
	}
}

func TestCreateLeagueAcceptsStartToday(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	if tt, code := f.createT(t, fan(f, t), "kind", "league", "start_date", dayStr(0)); code != http.StatusSeeOther || tt == nil {
		t.Fatalf("%d", code)
	}
}

func TestCreateTournamentIgnoresEndDate(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	tt, _ := f.createT(t, fan(f, t), "start_date", dayStr(-3), "end_date", dayStr(60))
	if tt == nil || tt.IsLeague() || !tt.EndDate.IsZero() {
		t.Fatalf("%+v", tt)
	}
}

func TestCreateRefusesUnknownKind(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	if tt, code := f.createT(t, fan(f, t), "kind", "bogus", "start_date", dayStr(1)); code != http.StatusBadRequest || tt != nil {
		t.Fatalf("unknown kind: %d", code)
	}
}

func TestNonAdminLeagueIsDraftAndAdminLeagueIsPublished(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	u := fan(f, t)
	tt, _ := f.createT(t, u, "kind", "league", "start_date", dayStr(2))
	if tt == nil || tt.Status != "draft" {
		t.Fatalf("%+v", tt)
	}
	f2 := entryFixture(t)
	defer f2.restore()
	admin := TWUser{ID: 1, IsAdmin: true}
	tt, _ = f2.createT(t, admin, "kind", "league", "start_date", dayStr(2))
	if tt == nil || tt.Status != "published" {
		t.Fatalf("%+v", tt)
	}
}

func TestHomeShowsLeaguesSection(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	mk := func(name string, start, end time.Time, status string) int {
		id := f.db.AddTournament(name, "baseball", "L", "", start, status)
		f.db.SetTournamentLeague(id, "league", end, 0)
		return id
	}
	run := mk("Running League", time.Now().AddDate(0, 0, -10), time.Now().AddDate(0, 0, 40), "published")
	mk("Ended League", time.Now().AddDate(0, 0, -90), time.Now().AddDate(0, 0, -2), "published")
	mk("Draft League", time.Now().AddDate(0, 0, -10), time.Now().AddDate(0, 0, 40), "draft")
	f.db.AddTournament("Fall Classic", "baseball", "L", "", time.Now().AddDate(0, 0, 2), "published")
	u := fan(f, t)
	body := f.page(f.env.TournamentList, u, "/", nil).Body.String()
	if !strings.Contains(body, "Leagues") || !strings.Contains(body, "Running League") || !strings.Contains(body, "League</span>") {
		t.Fatalf("leagues section: %s", body)
	}
	if strings.Contains(body, "Ended League") || strings.Contains(body, "Draft League") {
		t.Fatal("ended and draft leagues must be hidden")
	}
	if !strings.Contains(body, `name="target" value="`+itoa(run)+`"`) {
		t.Fatal("league follow button")
	}
	if i, j := strings.Index(body, "Running League"), strings.Index(body, "Fall Classic"); i < 0 || j < 0 {
		t.Fatal("both listed")
	}
}

func TestSearchResultsBadgeLeagues(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	id := f.db.AddTournament("Summer Lg", "baseball", "L", "", time.Now(), "published")
	f.db.SetTournamentLeague(id, "league", time.Time{}, 0)
	body := f.page(f.env.TournamentList, anon, "/?q=summer", nil).Body.String()
	if !strings.Contains(body, "Summer Lg") || !strings.Contains(body, "League</span>") {
		t.Fatalf("%s", body)
	}
}

func TestLeaguePageShowsBadgeAndSeasonDates(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	id := f.db.AddTournament("Summer Lg", "baseball", "L", "", time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "published")
	f.db.SetTournamentLeague(id, "league", time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC), 0)
	ps := f.tp()
	ps[0].Value = itoa(id)
	body := f.page(f.env.TournamentHome, anon, "/x", ps).Body.String()
	if !strings.Contains(body, "League</span>") || !strings.Contains(body, "Season:") || !strings.Contains(body, "Aug 30, 2026") {
		t.Fatalf("%s", body)
	}
	plain := f.page(f.env.TournamentHome, anon, "/x", f.tp()).Body.String()
	if strings.Contains(plain, "League</span>") || strings.Contains(plain, "Season:") {
		t.Fatal("plain tournament page must stay unchanged")
	}
}

func TestNewTournamentFormOffersTypeAndEndDate(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	body := f.page(f.env.NewTournamentForm, fan(f, t), "/create-tournament", nil).Body.String()
	if !strings.Contains(body, `name="kind"`) || !strings.Contains(body, `value="league"`) || !strings.Contains(body, `name="end_date"`) {
		t.Fatalf("%s", body)
	}
}
