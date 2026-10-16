package webhandler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
	"github.com/julienschmidt/httprouter"
)

type scoreFx struct {
	*slotFx
	dir1, dir2 int
}

// scoreFixture reuses the slot fixture (tournament, division, teams, one unscored game) and adds two directors.
func scoreFixture(t *testing.T) *scoreFx {
	t.Helper()
	f := newSlotFx(t)
	d1 := addDirector(f, "dir1@x.com")
	d2 := addDirector(f, "dir2@x.com")
	return &scoreFx{slotFx: f, dir1: d1, dir2: d2}
}

func (f *scoreFx) score(user TWUser, tid, gid int, home, away string) *httptest.ResponseRecorder {
	req := form("homescore", home, "awayscore", away)
	req = req.WithContext(ctxWithUser(req.Context(), user))
	w := httptest.NewRecorder()
	f.env.RecordScore(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}, {Key: "gid", Value: strconv.Itoa(gid)}})
	return w
}

func dirUser(id, tid int) TWUser {
	return TWUser{ID: id, Roles: []TournamentRole{{TournamentID: tid, Role: "director"}}}
}

func TestScoreNotifiesDirectorsExceptTheScorer(t *testing.T) {
	f := scoreFixture(t)
	defer f.restore()
	if w := f.score(dirUser(f.dir1, f.tid), f.tid, f.game.ID, "7", "3"); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	if got := inboxSubjects(f.slotFx, f.dir1); len(got) != 0 {
		t.Fatalf("the scorer must not be notified of their own action: %v", got)
	}
	got := f.db.ListNotifications(f.dir2, 10)
	if len(got) != 1 || got[0].Subject != "Game scored" {
		t.Fatalf("other director: %+v", got)
	}
	wantLink := "/tournaments/" + strconv.Itoa(f.tid) + "/manage/divisions/" + strconv.Itoa(f.did) + "/games/new"
	if got[0].Link != wantLink {
		t.Fatalf("link %q want %q", got[0].Link, wantLink)
	}
	tm := f.db.ReturnTeamsByDivisionID(f.did)
	if !strings.Contains(got[0].Message, tm[0].Name+" 7, "+tm[1].Name+" 3") {
		t.Fatalf("message should include the score: %q", got[0].Message)
	}
	if e := f.emails(); len(e) != 1 || !strings.HasPrefix(e[0], "dir2@x.com|Game scored") {
		t.Fatalf("emails %v", e)
	}
}

func TestRescoreSaysUpdated(t *testing.T) {
	f := scoreFixture(t)
	defer f.restore()
	scorer := TWUser{ID: 999, Roles: []TournamentRole{{TournamentID: f.tid, Role: "staff"}}}
	f.score(scorer, f.tid, f.game.ID, "7", "3")
	f.score(scorer, f.tid, f.game.ID, "7", "4")
	subjects := []string{}
	for _, n := range f.db.ListNotifications(f.dir1, 10) {
		subjects = append(subjects, n.Subject)
	}
	if len(subjects) != 2 || subjects[0] != "Game score updated" || subjects[1] != "Game scored" {
		t.Fatalf("got %v", subjects)
	}
}

func TestScoreNoticeFallsBackToAdmins(t *testing.T) {
	f := newSlotFx(t) // no director in this tournament
	defer f.restore()
	admin, _ := f.db.CreateUser("admin@x.com", "Admin", "h", "", true, true)
	sf := &scoreFx{slotFx: f}
	sf.score(TWUser{ID: 999, Roles: []TournamentRole{{TournamentID: f.tid, Role: "staff"}}}, f.tid, f.game.ID, "1", "0")
	if len(inboxSubjects(f, admin.ID)) != 1 {
		t.Fatal("admin fallback must be notified")
	}
}

func TestMutedUserGetsNoScoreNoticeButStillGetsOthers(t *testing.T) {
	f := scoreFixture(t)
	defer f.restore()
	f.db.SetMuteScoreNotices(f.dir2, true)
	f.score(TWUser{ID: 999, Roles: []TournamentRole{{TournamentID: f.tid, Role: "staff"}}}, f.tid, f.game.ID, "7", "3")
	if got := inboxSubjects(f.slotFx, f.dir2); len(got) != 0 {
		t.Fatalf("muted: no inbox entry expected, got %v", got)
	}
	if len(inboxSubjects(f.slotFx, f.dir1)) != 1 {
		t.Fatal("unmuted director still notified")
	}
	for _, e := range f.emails() {
		if strings.HasPrefix(e, "dir2@x.com") {
			t.Fatalf("muted: no email expected, got %v", e)
		}
	}
	// the mute is only for game-scored notices
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	respond(f.slotFx, TWUser{ID: f.ann, Name: "Ann"}, f.game.ID, 1, "decline")
	var declined bool
	for _, n := range f.db.ListNotifications(f.dir2, 10) {
		declined = declined || n.Subject == "Umpire declined a game"
	}
	if !declined {
		t.Fatal("other notices must ignore the score-notice mute")
	}
}

func TestScoreForGameInAnotherTournamentIs404(t *testing.T) {
	f := scoreFixture(t)
	defer f.restore()
	other := f.db.AddTournament("Other", "baseball", "L", "", time.Now(), "published")
	od := f.db.AddDivision(other, "10U")
	f.db.AddTeam(other, od, "X", "")
	f.db.AddTeam(other, od, "Y", "")
	tm := f.db.ReturnTeamsByDivisionID(od)
	og := f.db.AddGame(other, od, tm[0].ID, tm[1].ID, "F9", time.Now(), "")
	scorer := TWUser{ID: 999, Roles: []TournamentRole{{TournamentID: f.tid, Role: "staff"}}}
	if w := f.score(scorer, f.tid, og, "5", "4"); w.Code != http.StatusNotFound {
		t.Fatalf("got %d want 404", w.Code)
	}
	if f.db.ReturnGameByID(og).Scored {
		t.Fatal("a scorer for tournament A must not score a game in tournament B")
	}
	if len(inboxSubjects(f.slotFx, f.dir1))+len(inboxSubjects(f.slotFx, f.dir2)) != 0 {
		t.Fatal("no notices for a rejected score")
	}
	if w := f.score(scorer, f.tid, 99999, "1", "1"); w.Code != http.StatusNotFound {
		t.Fatalf("unknown game: got %d want 404", w.Code)
	}
	// viewing the score form for another tournament's game is refused too
	req := httptest.NewRequest("GET", "/x", nil)
	req = req.WithContext(ctxWithUser(req.Context(), scorer))
	w := httptest.NewRecorder()
	f.env.ScoreGame(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}, {Key: "gid", Value: strconv.Itoa(og)}})
	if w.Code != http.StatusNotFound {
		t.Fatalf("score form: got %d want 404", w.Code)
	}
}

func TestProfileSavesMuteScoreNotices(t *testing.T) {
	env, db, u := profileEnv(t)
	postProfile(env, u, map[string]string{"mute_score_notices": "on"}, nil)
	if !db.MutesScoreNotices(u.ID) {
		t.Fatal("checkbox on should mute")
	}
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req = req.WithContext(ctxWithUser(req.Context(), u))
	w := httptest.NewRecorder()
	env.Profile(w, req, nil)
	body := w.Body.String()
	i := strings.Index(body, `name="mute_score_notices"`)
	if i < 0 || !strings.Contains(body[i:i+80], "checked") {
		t.Fatalf("page should show the box checked: %s", body)
	}
	postProfile(env, u, map[string]string{}, nil)
	if db.MutesScoreNotices(u.ID) {
		t.Fatal("unchecked should unmute")
	}
}

var _ mydb.DB = (*mydb.FakeDB)(nil)
