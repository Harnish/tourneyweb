package webhandler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Harnish/tourneyweb/mydb"
)

func (f *entryFx) follower(t *testing.T, name, kind string, target int) int {
	t.Helper()
	u, err := f.db.CreateUser(name+"@x.com", name, "h", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.AddFollow(u.ID, kind, target); err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func (f *entryFx) staffScore(t *testing.T, home, away string) {
	t.Helper()
	w := f.call(f.env.RecordScore, "POST", dirUser(f.director, f.tid), f.game.ID, "homescore", home, "awayscore", away)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("staff score: %d %s", w.Code, w.Body.String())
	}
}

func (f *entryFx) alerts(userID int) []string {
	var out []string
	for _, s := range f.subjects(userID) {
		if s == "Final score" || s == "Game score corrected" {
			out = append(out, s)
		}
	}
	return out
}

func TestFollowerAlertOnStaffScore(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	ann := f.follower(t, "fan", "division", f.did)
	f.staffScore(t, "7", "3")
	if got := f.alerts(ann); len(got) != 1 || got[0] != "Final score" {
		t.Fatalf("alerts: %v", got)
	}
	n := f.db.ListNotifications(ann, 5)[0]
	if !strings.Contains(n.Message, f.game.HomeTeam.Name+" 7") || !strings.Contains(n.Message, f.game.AwayTeam.Name+" 3") {
		t.Fatalf("message must carry teams and score: %q", n.Message)
	}
	if want := "/tournaments/" + strconv.Itoa(f.tid) + "/games"; n.Link != want {
		t.Fatalf("link %q want %q", n.Link, want)
	}
}

func TestFollowerGetsOneAlertForDivisionAndTeam(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	u := f.follower(t, "fan", "division", f.did)
	f.db.AddFollow(u, "team", f.game.HomeTeam.ID)
	f.staffScore(t, "7", "3")
	if got := f.alerts(u); len(got) != 1 {
		t.Fatalf("alerts: %v", got)
	}
}

func TestTeamFollowerOnlyForThatTeam(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AddTeam(f.tid, f.did, "C", "")
	var other int
	for _, tm := range f.db.ReturnTeamsByDivisionID(f.did) {
		if tm.Name == "C" {
			other = tm.ID
		}
	}
	mine := f.follower(t, "mine", "team", f.game.AwayTeam.ID)
	stranger := f.follower(t, "stranger", "team", other)
	f.staffScore(t, "7", "3")
	if len(f.alerts(mine)) != 1 || len(f.alerts(stranger)) != 0 {
		t.Fatalf("mine %v stranger %v", f.alerts(mine), f.alerts(stranger))
	}
}

func TestTournamentFollowerGetsNoAlert(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	u := f.follower(t, "fan", "tournament", f.tid)
	f.staffScore(t, "7", "3")
	if got := f.alerts(u); len(got) != 0 {
		t.Fatalf("alerts: %v", got)
	}
}

func TestCoachReportAndDisputeSendNoFollowerAlert(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	u := f.follower(t, "fan", "division", f.did)
	f.call(f.env.CoachScore, "POST", f.homeCoach, f.game.ID, "homescore", "4", "awayscore", "2")
	if got := f.alerts(u); len(got) != 0 {
		t.Fatalf("first report alerted: %v", got)
	}
	f.call(f.env.CoachScore, "POST", f.awayCoach, f.game.ID, "homescore", "5", "awayscore", "2")
	if f.cur().ScoreStatus != "disputed" || len(f.alerts(u)) != 0 {
		t.Fatalf("status %s alerts %v", f.cur().ScoreStatus, f.alerts(u))
	}
}

func TestCoachConfirmAlertsFollowersExceptSecondCoach(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AddFollow(f.homeCoach.ID, "team", f.game.HomeTeam.ID)
	f.db.AddFollow(f.awayCoach.ID, "team", f.game.AwayTeam.ID)
	fan := f.follower(t, "fan", "division", f.did)
	f.call(f.env.CoachScore, "POST", f.homeCoach, f.game.ID, "homescore", "4", "awayscore", "2")
	f.call(f.env.CoachScore, "POST", f.awayCoach, f.game.ID, "homescore", "4", "awayscore", "2")
	if f.cur().ScoreStatus != "confirmed" {
		t.Fatalf("status %s", f.cur().ScoreStatus)
	}
	if len(f.alerts(fan)) != 1 || len(f.alerts(f.homeCoach.ID)) != 1 {
		t.Fatalf("fan %v first coach %v", f.alerts(fan), f.alerts(f.homeCoach.ID))
	}
	if got := f.alerts(f.awayCoach.ID); len(got) != 0 {
		t.Fatalf("the coach whose report made it official is skipped: %v", got)
	}
}

func TestUmpireFinalAlerts(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	fan := f.follower(t, "fan", "division", f.did)
	if err := f.env.submitUmpireScore(f.game, TWUser{ID: f.ann}, 5, 1); err != nil {
		t.Fatal(err)
	}
	if got := f.alerts(fan); len(got) != 1 {
		t.Fatalf("alerts: %v", got)
	}
}

func TestSameScoreResaveNoAlertAndChangeSendsCorrected(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	fan := f.follower(t, "fan", "division", f.did)
	f.staffScore(t, "7", "3")
	f.staffScore(t, "7", "3")
	if got := f.alerts(fan); len(got) != 1 {
		t.Fatalf("re-save alerted: %v", got)
	}
	f.staffScore(t, "8", "3")
	if got := f.alerts(fan); len(got) != 2 || got[0] != "Game score corrected" {
		t.Fatalf("alerts (newest first): %v", got)
	}
}

func TestScorerAndMutedFollowerSkipped(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AddFollow(f.director, "division", f.did) // the scorer follows too
	muted := f.follower(t, "muted", "division", f.did)
	f.db.SetMuteFollowAlerts(muted, true)
	scoreMuted := f.follower(t, "scoremuted", "division", f.did)
	f.db.SetMuteScoreNotices(scoreMuted, true) // the director-notice mute must not silence follower alerts
	f.staffScore(t, "7", "3")
	if len(f.alerts(f.director)) != 0 || len(f.alerts(muted)) != 0 {
		t.Fatalf("scorer %v muted %v", f.alerts(f.director), f.alerts(muted))
	}
	if len(f.alerts(scoreMuted)) != 1 {
		t.Fatal("mute_score_notices must not suppress follower alerts")
	}
}

func TestTeamChangeThenRescoreAlertsAgain(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	fan := f.follower(t, "fan", "division", f.did)
	f.call(f.env.CoachScore, "POST", f.homeCoach, f.game.ID, "homescore", "7", "awayscore", "3")
	f.call(f.env.CoachScore, "POST", f.awayCoach, f.game.ID, "homescore", "7", "awayscore", "3")
	g := f.cur()
	if g.ScoreStatus != "confirmed" || len(f.alerts(fan)) != 1 {
		t.Fatalf("setup: %s %v", g.ScoreStatus, f.alerts(fan))
	}
	f.db.AddTeam(f.tid, f.did, "C", "")
	var c int
	for _, tm := range f.db.ReturnTeamsByDivisionID(f.did) {
		if tm.Name == "C" {
			c = tm.ID
		}
	}
	f.db.UpdateGame(g.ID, f.did, g.HomeTeam.ID, c, g.Location, g.Start, "")
	f.staffScore(t, "7", "3")
	if got := f.alerts(fan); len(got) != 2 {
		t.Fatalf("alerts: %v", got)
	}
}

func TestProfileSavesMuteFollowAlerts(t *testing.T) {
	env, db, u := profileEnv(t)
	postProfile(env, u, map[string]string{"mute_follow_alerts": "on"}, nil)
	if !db.MutesFollowAlerts(u.ID) || db.MutesScoreNotices(u.ID) {
		t.Fatal("checkbox on should mute follower alerts only")
	}
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req = req.WithContext(ctxWithUser(req.Context(), u))
	w := httptest.NewRecorder()
	env.Profile(w, req, nil)
	body := w.Body.String()
	i := strings.Index(body, `name="mute_follow_alerts"`)
	if i < 0 || !strings.Contains(body[i:i+80], "checked") {
		t.Fatalf("page should show the box checked: %s", body)
	}
	postProfile(env, u, map[string]string{}, nil)
	if db.MutesFollowAlerts(u.ID) {
		t.Fatal("unchecked should unmute")
	}
}

func TestOfficialResult(t *testing.T) {
	cases := []struct {
		g    mydb.Game
		want bool
	}{
		{mydb.Game{Scored: true, ScoreStatus: "final"}, true},
		{mydb.Game{Scored: true, ScoreStatus: "confirmed"}, true},
		{mydb.Game{Scored: true, ScoreStatus: "reported"}, false},
		{mydb.Game{Scored: true, ScoreStatus: "disputed"}, false},
		{mydb.Game{ScoreStatus: "final"}, false},
	}
	for _, c := range cases {
		if got, _, _ := officialResult(c.g); got != c.want {
			t.Errorf("%+v: got %v", c.g, got)
		}
	}
}
