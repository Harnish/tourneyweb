package webhandler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
)

// league makes a published league with one level; started leagues began yesterday.
func (f *entryFx) league(started bool) (tid, did int) {
	start := time.Now().AddDate(0, 0, 5)
	if started {
		start = time.Now().AddDate(0, 0, -1)
	}
	tid = f.db.AddTournament("Lg", "baseball", "L", "", start, "published")
	f.db.SetTournamentLeague(tid, "league", time.Time{}, 0)
	did = f.db.AddDivision(tid, "10U")
	return
}

func roleUser(id int, tid int, role string) TWUser {
	return TWUser{ID: id, Roles: []TournamentRole{{TournamentID: tid, Role: role}}}
}

func (f *entryFx) addTeam(h func(http.ResponseWriter, *http.Request, httprouter.Params), u TWUser, tid, did int, kv ...string) *httptest.ResponseRecorder {
	req := form(append([]string{"teamname", "Late Team", "division", itoa(did)}, kv...)...)
	req = req.WithContext(ctxWithUser(req.Context(), u))
	w := httptest.NewRecorder()
	h(w, req, httprouter.Params{{Key: "tid", Value: itoa(tid)}})
	return w
}

func teamCount(f *entryFx, did int) int { return len(f.db.ReturnTeamsByDivisionID(did)) }

func TestStartedLeagueRefusesTeamOnManageForm(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	tid, did := f.league(true)
	w := f.addTeam(f.env.ManageTeams, roleUser(7, tid, "director"), tid, did)
	if w.Code != http.StatusForbidden || teamCount(f, did) != 0 {
		t.Fatalf("%d teams=%d", w.Code, teamCount(f, did))
	}
}

func TestLateTeamOverrideForLeagueDirectorAndAdmin(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	tid, did := f.league(true)
	for _, u := range []TWUser{roleUser(7, tid, "director"), {ID: 8, IsAdmin: true}} {
		w := f.addTeam(f.env.ManageTeams, u, tid, did, "late_team", "1")
		if w.Code != http.StatusSeeOther {
			t.Fatalf("override for %+v: %d %s", u, w.Code, w.Body.String())
		}
	}
	if teamCount(f, did) != 2 {
		t.Fatalf("teams=%d", teamCount(f, did))
	}
}

func TestLateTeamOverrideRefusedForStaffAndOtherDirectors(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	tid, did := f.league(true)
	for _, u := range []TWUser{roleUser(7, tid, "staff"), roleUser(9, f.tid, "director"), {ID: 10}} {
		if w := f.addTeam(f.env.ManageTeams, u, tid, did, "late_team", "1"); w.Code != http.StatusForbidden {
			t.Errorf("%+v: got %d want 403", u, w.Code)
		}
	}
	if teamCount(f, did) != 0 {
		t.Fatal("no team may be stored")
	}
}

func TestLateTeamFlagIgnoredBeforeSeasonStart(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	tid, did := f.league(false)
	dir := roleUser(7, tid, "director")
	for _, kv := range [][]string{nil, {"late_team", "1"}} {
		if w := f.addTeam(f.env.ManageTeams, dir, tid, did, kv...); w.Code != http.StatusSeeOther {
			t.Fatalf("%v: %d", kv, w.Code)
		}
	}
	if teamCount(f, did) != 2 {
		t.Fatalf("teams=%d", teamCount(f, did))
	}
}

func TestLegacyAdminTeamFormRefusesStartedLeague(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	tid, did := f.league(true)
	w := f.addTeam(f.env.Teams, TWUser{ID: 8, IsAdmin: true}, tid, did, "late_team", "1")
	if w.Code != http.StatusForbidden || teamCount(f, did) != 0 {
		t.Fatalf("%d teams=%d", w.Code, teamCount(f, did))
	}
}

func TestTournamentsAreNeverLocked(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	tid := f.db.AddTournament("Old Cup", "baseball", "L", "", time.Now().AddDate(0, 0, -30), "published")
	did := f.db.AddDivision(tid, "10U")
	if w := f.addTeam(f.env.ManageTeams, roleUser(7, tid, "director"), tid, did); w.Code != http.StatusSeeOther {
		t.Fatalf("%d", w.Code)
	}
}

func TestManageTeamsPageShowsLateTeamButtonOnlyWhenStarted(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	started, _ := f.league(true)
	notStarted, _ := f.league(false)
	get := func(tid int, u TWUser) string {
		return f.page(f.env.ManageTeams, u, "/x", httprouter.Params{{Key: "tid", Value: itoa(tid)}}).Body.String()
	}
	if b := get(started, roleUser(7, started, "director")); !strings.Contains(b, `name="late_team"`) || !strings.Contains(b, "Add late team") {
		t.Fatalf("director on started league: %s", b)
	}
	b := get(started, roleUser(7, started, "staff"))
	if strings.Contains(b, `name="late_team"`) || !strings.Contains(b, "season has started") {
		t.Fatalf("staff gets a note, no button: %s", b)
	}
	if b := get(notStarted, roleUser(7, notStarted, "director")); strings.Contains(b, `name="late_team"`) || !strings.Contains(b, "Add Team") {
		t.Fatalf("not started: %s", b)
	}
}
