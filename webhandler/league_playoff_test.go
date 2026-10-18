package webhandler

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
	"github.com/julienschmidt/httprouter"
)

type poFx struct {
	*entryFx
	tid, did, did2 int
	dir, staff     TWUser
}

// poFixture: a started league with a scored 10U level (A, B, C) and an unscored 12U level (Zed, Amy, Mid),
// plus a game, news, an extra role, a coach account, staff and two directors.
func poFixture(t *testing.T) *poFx {
	t.Helper()
	f := entryFixture(t)
	tid, did := f.league(true)
	for _, n := range []string{"C", "A", "B"} {
		f.db.AddTeam(tid, did, n, n+" Coach")
	}
	by := map[string]int{}
	for _, tm := range f.db.ReturnTeamsByDivisionID(did) {
		by[tm.Name] = tm.ID
	}
	g := f.db.AddGame(tid, did, by["B"], by["A"], "F1", time.Now().AddDate(0, 0, -3), "")
	f.db.ScoreGame(g, 6, 1)
	did2 := f.db.AddDivision(tid, "12U")
	for _, n := range []string{"Zed", "Amy", "Mid"} {
		f.db.AddTeam(tid, did2, n, "")
	}
	f.db.AddNews(tid, "Season news", "body", 1)
	f.db.AddExtraRole(tid, "Scorekeeper")
	coach, _ := f.db.CreateUser("lgcoach@x.com", "LC", "h", "", true, false)
	f.db.AssignRole(coach.ID, tid, "coach", by["A"])
	dirB, _ := f.db.CreateUser("dirb@x.com", "DirB", "h", "", true, false)
	f.db.AssignRole(dirB.ID, tid, "director", 0)
	staff, _ := f.db.CreateUser("lgstaff@x.com", "LS", "h", "", true, false)
	f.db.AssignRole(staff.ID, tid, "staff", 0)
	dir, _ := f.db.CreateUser("lgdir@x.com", "LD", "h", "", true, false)
	f.db.AssignRole(dir.ID, tid, "director", 0)
	return &poFx{entryFx: f, tid: tid, did: did, did2: did2,
		dir: roleUser(dir.ID, tid, "director"), staff: roleUser(staff.ID, tid, "staff")}
}

func (f *poFx) create(u TWUser, tid int) *httptest.ResponseRecorder {
	req := form()
	req = req.WithContext(ctxWithUser(req.Context(), u))
	w := httptest.NewRecorder()
	f.env.CreatePlayoff(w, req, httprouter.Params{{Key: "tid", Value: itoa(tid)}})
	return w
}

func (f *poFx) playoffs() []mydb.Tournament { return f.db.LeaguePlayoffs(f.tid) }

// teamNames lists a level's teams in creation order (by id): the copy order is the seeding order.
func teamNames(db mydb.DB, did int) []string {
	teams := db.ReturnTeamsByDivisionID(did)
	sort.Slice(teams, func(i, j int) bool { return teams[i].ID < teams[j].ID })
	var out []string
	for _, tm := range teams {
		out = append(out, tm.Name)
	}
	return out
}

func (f *poFx) newDivisions(t *testing.T, po int) map[string]int {
	t.Helper()
	m := map[string]int{}
	for _, d := range f.db.ReturnDivisions(po) {
		m[d.Name] = d.ID
	}
	return m
}

func TestPlayoffCopiesLevelsAndTeamsInStandingsOrder(t *testing.T) {
	f := poFixture(t)
	defer f.restore()
	if w := f.create(f.dir, f.tid); w.Code != http.StatusSeeOther {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	po := f.playoffs()
	if len(po) != 1 {
		t.Fatalf("playoffs: %+v", po)
	}
	divs := f.newDivisions(t, po[0].ID)
	if len(divs) != 2 || divs["10U"] == 0 || divs["12U"] == 0 {
		t.Fatalf("divisions: %v", divs)
	}
	// expected order is the league division's own standings
	orig := f.db.ReturnTeamsByDivisionIDWithStats(f.did)
	sortedByName := append([]mydb.Team(nil), orig...)
	for i := range sortedByName {
		for j := i + 1; j < len(sortedByName); j++ {
			if sortedByName[j].Name < sortedByName[i].Name {
				sortedByName[i], sortedByName[j] = sortedByName[j], sortedByName[i]
			}
		}
	}
	var want []string
	for _, tm := range f.env.SortTeams(sortedByName, f.db.ReturnDivisionByID(f.did).RankingCriteria) {
		want = append(want, tm.Name)
	}
	if want[0] != "B" {
		t.Fatalf("test setup: the winner should lead the standings: %v", want)
	}
	got := teamNames(f.db, divs["10U"])
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("copied order %v want standings %v", got, want)
	}
	for _, tm := range f.db.ReturnTeamsByDivisionID(divs["10U"]) {
		if tm.Name == "C" && tm.Coach != "C Coach" {
			t.Fatalf("coach text must be copied: %+v", tm)
		}
	}
}

func TestPlayoffTiesAndUnscoredLevelsAreAlphabetical(t *testing.T) {
	f := poFixture(t)
	defer f.restore()
	f.create(f.dir, f.tid)
	divs := f.newDivisions(t, f.playoffs()[0].ID)
	if got := strings.Join(teamNames(f.db, divs["12U"]), ","); got != "Amy,Mid,Zed" {
		t.Fatalf("got %s", got)
	}
}

func TestPlayoffCopiesNoGamesScoresNewsOrAssignments(t *testing.T) {
	f := poFixture(t)
	defer f.restore()
	f.create(f.dir, f.tid)
	po := f.playoffs()[0].ID
	if len(f.db.AllGames(po)) != 0 || len(f.db.GetTournamentNews(po)) != 0 || len(f.db.TournamentExtraRoles(po)) != 0 {
		t.Fatal("games, news and extra roles must not be copied")
	}
	for _, d := range f.db.ReturnDivisions(po) {
		for _, tm := range f.db.ReturnTeamsByDivisionIDWithStats(d.ID) {
			if tm.Wins != 0 || tm.Losses != 0 {
				t.Fatalf("no standings carry over: %+v", tm)
			}
		}
	}
	for _, r := range f.db.GetRolesForTournament(po) {
		if r.Role != "director" {
			t.Fatalf("only directors are copied: %+v", r)
		}
	}
}

func TestPlayoffDirectors(t *testing.T) {
	f := poFixture(t)
	defer f.restore()
	f.create(f.dir, f.tid)
	po := f.playoffs()[0].ID
	want := map[int]bool{}
	for _, r := range f.db.GetRolesForTournament(f.tid) {
		if r.Role == "director" {
			want[r.UserID] = true
		}
	}
	got := map[int]bool{}
	for _, r := range f.db.GetRolesForTournament(po) {
		got[r.UserID] = true
	}
	if len(got) != len(want) || len(want) < 2 {
		t.Fatalf("league directors %v playoff directors %v", want, got)
	}
	for id := range want {
		if !got[id] {
			t.Fatalf("missing director %d", id)
		}
	}
	if got[f.staff.ID] {
		t.Fatal("staff must not be copied")
	}
}

func TestPlayoffIsLinkedAndNamed(t *testing.T) {
	f := poFixture(t)
	defer f.restore()
	f.db.SetTournamentLeague(f.tid, "league", time.Date(2030, 8, 30, 0, 0, 0, 0, time.UTC), 0)
	f.db.SetTournamentDefaultRanking(f.tid, []string{"wins", "runs_against"})
	f.create(f.dir, f.tid)
	po := f.playoffs()[0]
	if po.IsLeague() || po.LeagueID != f.tid || po.Name != "Lg Playoffs" || po.Sport != "baseball" || po.Location != "L" {
		t.Fatalf("%+v", po)
	}
	if po.StartDate.Format("2006-01-02") != "2030-08-30" {
		t.Fatalf("start date = league end date: %v", po.StartDate)
	}
	if len(po.DefaultRankingCriteria) != 2 {
		t.Fatalf("default ranking copied: %v", po.DefaultRankingCriteria)
	}
	f2 := poFixture(t)
	defer f2.restore()
	f2.create(f2.dir, f2.tid)
	if got := f2.playoffs()[0].StartDate.Format("2006-01-02"); got != time.Now().Format("2006-01-02") {
		t.Fatalf("no end date: starts today, got %s", got)
	}
}

func TestPlayoffDraftForDirectorPublishedForAdmin(t *testing.T) {
	f := poFixture(t)
	defer f.restore()
	f.create(f.dir, f.tid)
	f.create(TWUser{ID: 1, IsAdmin: true}, f.tid)
	po := f.playoffs()
	if len(po) != 2 {
		t.Fatalf("%d playoffs", len(po))
	}
	status := map[string]int{}
	for _, p := range po {
		status[p.Status]++
	}
	if status["draft"] != 1 || status["published"] != 1 {
		t.Fatalf("%v", status)
	}
}

func TestPlayoffRefusedForStaffOthersAndAnonymous(t *testing.T) {
	f := poFixture(t)
	defer f.restore()
	for name, u := range map[string]TWUser{"staff": f.staff, "other director": roleUser(99, f.entryFx.tid, "director"), "plain user": {ID: 98}} {
		if w := f.create(u, f.tid); w.Code != http.StatusForbidden {
			t.Errorf("%s: got %d want 403", name, w.Code)
		}
	}
	if w := f.create(anon, f.tid); w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
		t.Errorf("anonymous: %d %q", w.Code, w.Header().Get("Location"))
	}
	if len(f.playoffs()) != 0 {
		t.Fatal("nothing may be created")
	}
}

func TestPlayoffRefusedForPlainTournament(t *testing.T) {
	f := poFixture(t)
	defer f.restore()
	if w := f.create(roleUser(7, f.entryFx.tid, "director"), f.entryFx.tid); w.Code != http.StatusBadRequest {
		t.Fatalf("got %d", w.Code)
	}
}

func TestLeagueCanHaveSeveralPlayoffs(t *testing.T) {
	f := poFixture(t)
	defer f.restore()
	f.create(f.dir, f.tid)
	f.create(f.dir, f.tid)
	if len(f.playoffs()) != 2 {
		t.Fatalf("%d", len(f.playoffs()))
	}
}

func TestPlayoffLinksShown(t *testing.T) {
	f := poFixture(t)
	defer f.restore()
	f.create(f.dir, f.tid)
	f.create(TWUser{ID: 1, IsAdmin: true}, f.tid)
	var draft, pub mydb.Tournament
	for _, p := range f.playoffs() {
		if p.Status == "draft" {
			draft = p
		} else {
			pub = p
		}
	}
	f.db.SetTournamentLeague(pub.ID, "tournament", time.Time{}, f.tid)
	ps := httprouter.Params{{Key: "tid", Value: itoa(f.tid)}}
	dash := f.page(f.env.ManageDashboard, f.dir, "/x", ps).Body.String()
	if !strings.Contains(dash, `action="/tournaments/`+itoa(f.tid)+`/manage/playoff"`) || !strings.Contains(dash, "/tournaments/"+itoa(draft.ID)) || !strings.Contains(dash, "/tournaments/"+itoa(pub.ID)) {
		t.Fatalf("manage page lists the button and every playoff: %s", dash)
	}
	if staffDash := f.page(f.env.ManageDashboard, f.staff, "/x", ps).Body.String(); strings.Contains(staffDash, "manage/playoff") {
		t.Fatal("staff must not see the playoff button")
	}
	pub2 := f.page(f.env.TournamentHome, anon, "/x", ps).Body.String()
	if !strings.Contains(pub2, "/tournaments/"+itoa(pub.ID)) || strings.Contains(pub2, "/tournaments/"+itoa(draft.ID)) {
		t.Fatalf("public league page lists published playoffs only: %s", pub2)
	}
	pg := f.page(f.env.TournamentHome, anon, "/x", httprouter.Params{{Key: "tid", Value: itoa(pub.ID)}}).Body.String()
	if !strings.Contains(pg, "Playoffs of") || !strings.Contains(pg, "/tournaments/"+itoa(f.tid)) {
		t.Fatalf("playoff page links back: %s", pg)
	}
}
