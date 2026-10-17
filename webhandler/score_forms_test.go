package webhandler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
)

func (f *entryFx) ps(gid int) httprouter.Params {
	return httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}, {Key: "gid", Value: strconv.Itoa(gid)}}
}

type scoreHandler func(http.ResponseWriter, *http.Request, httprouter.Params)

func (f *entryFx) call(h scoreHandler, method string, user TWUser, gid int, kv ...string) *httptest.ResponseRecorder {
	req := form(kv...)
	req.Method = method
	req = req.WithContext(ctxWithUser(req.Context(), user))
	w := httptest.NewRecorder()
	h(w, req, f.ps(gid))
	return w
}

func TestCoachFormGetAndPost(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	w := f.call(f.env.CoachScore, "GET", f.homeCoach, f.game.ID)
	if w.Code != 200 || !strings.Contains(w.Body.String(), f.game.HomeTeam.Name) || !strings.Contains(w.Body.String(), f.game.AwayTeam.Name) {
		t.Fatalf("GET: %d %s", w.Code, w.Body.String())
	}
	w = f.call(f.env.CoachScore, "POST", f.homeCoach, f.game.ID, "homescore", "7", "awayscore", "3")
	if w.Code != http.StatusSeeOther {
		t.Fatalf("POST: %d %s", w.Code, w.Body.String())
	}
	if g := f.cur(); g.HomeScore != 7 || g.AwayScore != 3 || g.ScoreStatus != "reported" {
		t.Fatalf("%+v", g)
	}
}

func TestCoachFormRejectsOutsiderAndOtherTournamentGame(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	outsider := TWUser{ID: 99, Roles: []TournamentRole{{TournamentID: f.tid, Role: "coach", TeamID: 424242}}}
	for _, m := range []string{"GET", "POST"} {
		w := f.call(f.env.CoachScore, m, outsider, f.game.ID, "homescore", "1", "awayscore", "0")
		if w.Code != http.StatusForbidden {
			t.Errorf("%s outsider: got %d want 403", m, w.Code)
		}
	}
	if f.cur().Scored {
		t.Fatal("outsider stored a score")
	}
	other := f.db.AddTournament("Other", "baseball", "L", "", time.Now(), "published")
	od := f.db.AddDivision(other, "10U")
	f.db.AddTeam(other, od, "X", "")
	f.db.AddTeam(other, od, "Y", "")
	tm := f.db.ReturnTeamsByDivisionID(od)
	og := f.db.AddGame(other, od, tm[0].ID, tm[1].ID, "F", time.Now(), "")
	if w := f.call(f.env.CoachScore, "GET", f.homeCoach, og); w.Code != http.StatusNotFound {
		t.Fatalf("other tournament's game: got %d want 404", w.Code)
	}
	if w := f.call(f.env.CoachScore, "GET", f.homeCoach, 99999); w.Code != http.StatusNotFound {
		t.Fatalf("unknown game: got %d want 404", w.Code)
	}
}

func TestCoachPostErrorsMapToStatusCodes(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	post := func(u TWUser, h, a string) int {
		return f.call(f.env.CoachScore, "POST", u, f.game.ID, "homescore", h, "awayscore", a).Code
	}
	if c := post(f.homeCoach, "41", "0"); c != http.StatusBadRequest {
		t.Errorf("out of range: %d", c)
	}
	if c := post(f.homeCoach, "abc", "0"); c != http.StatusBadRequest {
		t.Errorf("non-numeric: %d", c)
	}
	if c := post(f.homeCoach, "-1", "0"); c != http.StatusBadRequest {
		t.Errorf("negative: %d", c)
	}
	post(f.homeCoach, "7", "3")
	post(f.awayCoach, "7", "3") // confirmed
	if c := post(f.homeCoach, "9", "9"); c != http.StatusConflict {
		t.Errorf("locked: got %d want 409", c)
	}
}

func TestCoachConfirmOneClick(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.call(f.env.CoachScore, "POST", f.homeCoach, f.game.ID, "homescore", "7", "awayscore", "3")
	w := f.call(f.env.CoachScore, "GET", f.awayCoach, f.game.ID)
	body := w.Body.String()
	if !strings.Contains(body, `name="homescore" value="7"`) || !strings.Contains(body, `name="awayscore" value="3"`) || !strings.Contains(body, "Confirm") {
		t.Fatalf("one-click confirm form expected: %s", body)
	}
	f.call(f.env.CoachScore, "POST", f.awayCoach, f.game.ID, "homescore", "7", "awayscore", "3")
	if f.cur().ScoreStatus != "confirmed" {
		t.Fatalf("status %q", f.cur().ScoreStatus)
	}
}

func TestUmpireFormGuards(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.db.AssignUmpire(f.game.ID, 2, f.bob, "")
	f.db.RespondUmpireSlot(f.game.ID, 2, f.bob, false)
	stranger, _ := f.db.CreateUser("stranger@x.com", "S", "h", "", true, false)
	for name, id := range map[string]int{"unassigned": stranger.ID, "declined": f.bob} {
		for _, m := range []string{"GET", "POST"} {
			if w := f.call(f.env.UmpireScoreEntry, m, TWUser{ID: id}, f.game.ID, "homescore", "1", "awayscore", "0"); w.Code != http.StatusForbidden {
				t.Errorf("%s %s: got %d want 403", name, m, w.Code)
			}
		}
	}
	f.call(f.env.CoachScore, "POST", f.homeCoach, f.game.ID, "homescore", "7", "awayscore", "3")
	f.call(f.env.CoachScore, "POST", f.awayCoach, f.game.ID, "homescore", "7", "awayscore", "3")
	if w := f.call(f.env.UmpireScoreEntry, "POST", TWUser{ID: f.ann}, f.game.ID, "homescore", "1", "awayscore", "0"); w.Code != http.StatusConflict {
		t.Fatalf("confirmed game: got %d want 409", w.Code)
	}
}

func TestUmpirePostResolvesDispute(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.call(f.env.CoachScore, "POST", f.homeCoach, f.game.ID, "homescore", "7", "awayscore", "3")
	f.call(f.env.CoachScore, "POST", f.awayCoach, f.game.ID, "homescore", "7", "awayscore", "5")
	if w := f.call(f.env.UmpireScoreEntry, "GET", TWUser{ID: f.ann, Name: "Ann"}, f.game.ID); w.Code != 200 {
		t.Fatalf("GET: %d", w.Code)
	}
	if w := f.call(f.env.UmpireScoreEntry, "POST", TWUser{ID: f.ann, Name: "Ann"}, f.game.ID, "homescore", "6", "awayscore", "5"); w.Code != http.StatusSeeOther {
		t.Fatalf("POST: %d", w.Code)
	}
	if g := f.cur(); g.HomeScore != 6 || g.AwayScore != 5 || g.ScoreStatus != "final" {
		t.Fatalf("%+v", g)
	}
}

func TestStaffRecordScoreSetsFinalAndClearsDispute(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.call(f.env.CoachScore, "POST", f.homeCoach, f.game.ID, "homescore", "7", "awayscore", "3")
	f.call(f.env.CoachScore, "POST", f.awayCoach, f.game.ID, "homescore", "7", "awayscore", "5")
	if f.cur().ScoreStatus != "disputed" {
		t.Fatal("setup: disputed")
	}
	staff := TWUser{ID: 999, Roles: []TournamentRole{{TournamentID: f.tid, Role: "staff"}}}
	if w := f.call(f.env.RecordScore, "POST", staff, f.game.ID, "homescore", "8", "awayscore", "3"); w.Code != http.StatusSeeOther {
		t.Fatalf("staff score: %d", w.Code)
	}
	if g := f.cur(); g.ScoreStatus != "final" || g.HomeScore != 8 {
		t.Fatalf("staff overwrite is final: %+v", g)
	}
	if w := f.call(f.env.CoachScore, "POST", f.homeCoach, f.game.ID, "homescore", "1", "awayscore", "0"); w.Code != http.StatusConflict {
		t.Fatalf("coaches locked after a staff fix: %d", w.Code)
	}
}

func TestFormsRequireLogin(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	for name, h := range map[string]scoreHandler{"coach": f.env.CoachScore, "umpire": f.env.UmpireScoreEntry} {
		w := f.call(h, "GET", TWUser{}, f.game.ID)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
			t.Errorf("%s: %d -> %q", name, w.Code, w.Header().Get("Location"))
		}
	}
}

func TestCoachFormEscapesNames(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AddTeam(f.tid, f.did, "<b>Evil</b>", "")
	var evil int
	for _, tm := range f.db.ReturnTeamsByDivisionID(f.did) {
		if tm.Name == "<b>Evil</b>" {
			evil = tm.ID
		}
	}
	g := f.db.AddGame(f.tid, f.did, evil, f.game.AwayTeam.ID, "F", time.Now(), "")
	coach := TWUser{ID: 77, Roles: []TournamentRole{{TournamentID: f.tid, Role: "coach", TeamID: evil}}}
	body := f.call(f.env.CoachScore, "GET", coach, g).Body.String()
	if strings.Contains(body, "<b>Evil</b>") || !strings.Contains(body, "&lt;b&gt;Evil&lt;/b&gt;") {
		t.Fatalf("team names must be escaped: %s", body)
	}
}

func (f *entryFx) gamesPage(user TWUser) string {
	req := form()
	req.Method = "GET"
	req = req.WithContext(ctxWithUser(req.Context(), user))
	w := httptest.NewRecorder()
	f.env.Games(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}})
	return w.Body.String()
}

func TestGamesPageShowsScoreLinkToCoachAndUmpire(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	coachLink := "/tournaments/" + strconv.Itoa(f.tid) + "/coach/games/" + strconv.Itoa(f.game.ID) + "/score"
	umpLink := "/tournaments/" + strconv.Itoa(f.tid) + "/umpire/games/" + strconv.Itoa(f.game.ID) + "/score"
	if body := f.gamesPage(f.homeCoach); !strings.Contains(body, coachLink) || strings.Contains(body, umpLink) {
		t.Fatalf("coach sees only the coach link: %s", body)
	}
	if body := f.gamesPage(TWUser{ID: f.ann, Name: "Ann"}); !strings.Contains(body, umpLink) || strings.Contains(body, coachLink) {
		t.Fatalf("umpire sees only the umpire link: %s", body)
	}
	outsider := TWUser{ID: 99, Roles: []TournamentRole{{TournamentID: f.tid, Role: "coach", TeamID: 424242}}}
	for name, u := range map[string]TWUser{"outsider": outsider, "unassigned umpire": {ID: f.bob}, "anonymous": {}} {
		if body := f.gamesPage(u); strings.Contains(body, coachLink) || strings.Contains(body, umpLink) {
			t.Errorf("%s must see no score link", name)
		}
	}
}

func TestGamesPageHidesLinkWhenLockedAndShowsDisputedBadge(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.call(f.env.CoachScore, "POST", f.homeCoach, f.game.ID, "homescore", "7", "awayscore", "3")
	f.call(f.env.CoachScore, "POST", f.awayCoach, f.game.ID, "homescore", "7", "awayscore", "5")
	if body := f.gamesPage(f.homeCoach); !strings.Contains(body, "disputed") || !strings.Contains(body, "/coach/games/") {
		t.Fatalf("disputed game: badge and link expected: %s", body)
	}
	f.env.markFinal(f.cur())
	body := f.gamesPage(f.homeCoach)
	if strings.Contains(body, "/coach/games/") || strings.Contains(body, "disputed") {
		t.Fatalf("final game: no link, no badge: %s", body)
	}
	if strings.Contains(f.gamesPage(TWUser{ID: f.ann}), "/umpire/games/") {
		t.Fatal("umpire link hidden once final")
	}
}
