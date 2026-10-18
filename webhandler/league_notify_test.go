package webhandler

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

func (f *entryFx) leagueGame() mydb.Game {
	f.db.SetTournamentLeague(f.tid, "league", time.Time{}, 0)
	return f.cur()
}

func moved(g mydb.Game) mydb.Game {
	g.Start = g.Start.Add(time.Hour)
	return g
}

func (f *entryFx) resched(u int) []mydb.Notification {
	var out []mydb.Notification
	for _, n := range f.db.ListNotifications(u, 50) {
		if n.Subject == "Game rescheduled" {
			out = append(out, n)
		}
	}
	return out
}

func TestRescheduleNotifiesCoachesAndFollowers(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	g := f.leagueGame()
	divFan := f.follower(t, "divfan", "division", f.did)
	teamFan := f.follower(t, "teamfan", "team", g.AwayTeam.ID)
	stranger := f.follower(t, "stranger", "tournament", f.tid)
	f.env.notifyLeagueReschedule(g, moved(g), f.director)
	for name, id := range map[string]int{"home coach": f.homeCoach.ID, "away coach": f.awayCoach.ID, "division follower": divFan, "team follower": teamFan} {
		got := f.resched(id)
		if len(got) != 1 || !strings.Contains(got[0].Message, "Was:") || !strings.Contains(got[0].Message, "Now:") {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if len(f.resched(stranger)) != 0 || len(f.resched(f.director)) != 0 {
		t.Fatal("tournament-only followers and the editor get nothing")
	}
}

func TestNoRescheduleNoticeWhenNothingMoved(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	g := f.leagueGame()
	f.env.notifyLeagueReschedule(g, g, f.director)
	same := g
	same.Start = g.Start.In(time.FixedZone("x", 3600)) // same instant, different zone
	f.env.notifyLeagueReschedule(g, same, f.director)
	if len(f.resched(f.homeCoach.ID)) != 0 {
		t.Fatal("no notice when time and field are unchanged")
	}
}

func TestNoRescheduleNoticeForPlainTournament(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	g := f.cur()
	f.env.notifyLeagueReschedule(g, moved(g), f.director)
	if len(f.resched(f.homeCoach.ID)) != 0 {
		t.Fatal("tournaments keep today's behavior")
	}
}

func TestFieldOnlyChangeAndScoredGameStillNotify(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	g := f.leagueGame()
	f.db.ScoreGame(g.ID, 3, 2)
	g = f.cur()
	field := g
	field.Location = "Field 9"
	f.env.notifyLeagueReschedule(g, field, f.director)
	if len(f.resched(f.homeCoach.ID)) != 1 {
		t.Fatal("a field change on a scored game still notifies")
	}
}

func TestCoachWhoAlsoFollowsGetsOneNotice(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	g := f.leagueGame()
	f.db.AddFollow(f.homeCoach.ID, "team", g.HomeTeam.ID)
	f.db.AddFollow(f.homeCoach.ID, "division", f.did)
	f.env.notifyLeagueReschedule(g, moved(g), f.director)
	if n := len(f.resched(f.homeCoach.ID)); n != 1 {
		t.Fatalf("got %d notices", n)
	}
}

func TestMutedFollowerSkippedButCoachAlwaysGets(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	g := f.leagueGame()
	fan := f.follower(t, "fan", "division", f.did)
	f.db.SetMuteFollowAlerts(fan, true)
	f.db.AddFollow(f.homeCoach.ID, "team", g.HomeTeam.ID)
	f.db.SetMuteFollowAlerts(f.homeCoach.ID, true)
	f.env.notifyLeagueReschedule(g, moved(g), f.director)
	if len(f.resched(fan)) != 0 || len(f.resched(f.homeCoach.ID)) != 1 {
		t.Fatalf("fan %d coach %d", len(f.resched(fan)), len(f.resched(f.homeCoach.ID)))
	}
}

func TestTeamWithoutCoachAccountNotifiesFollowersOnly(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.leagueGame()
	f.db.AddTeam(f.tid, f.did, "C", "Typed Coach")
	f.db.AddTeam(f.tid, f.did, "D", "")
	var c, d int
	for _, tm := range f.db.ReturnTeamsByDivisionID(f.did) {
		switch tm.Name {
		case "C":
			c = tm.ID
		case "D":
			d = tm.ID
		}
	}
	gid := f.db.AddGame(f.tid, f.did, c, d, "F2", time.Now().Add(48*time.Hour), "")
	g := f.db.ReturnGameByID(gid)
	fan := f.follower(t, "fan", "team", c)
	f.env.notifyLeagueReschedule(g, moved(g), f.director)
	if len(f.resched(fan)) != 1 {
		t.Fatal("followers still hear about it")
	}
}

func TestFollowerRescheduleNoticeCarriesUnfollowLink(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.env.FollowSecret = []byte("0123456789abcdef0123456789abcdef")
	f.env.BaseURL = "https://tw.example"
	g := f.leagueGame()
	fan := f.follower(t, "fan", "division", f.did)
	f.env.notifyLeagueReschedule(g, moved(g), f.director)
	got := f.resched(fan)
	if len(got) != 1 || !strings.Contains(got[0].Message, "https://tw.example/unfollow?t=") {
		t.Fatalf("%+v", got)
	}
}

func TestEditGameSendsRescheduleAndUmpireNotices(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	f.db.SetTournamentLeague(f.tid, "league", time.Time{}, 0)
	coach, _ := f.db.CreateUser("coach@x.com", "Coach", "h", "", true, false)
	f.db.AssignRole(coach.ID, f.tid, "coach", f.game.HomeTeam.ID)
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.db.RespondUmpireSlot(f.game.ID, 1, f.ann, true) // only accepted slots are asked to re-confirm
	if w := f.edit(sameTime, "ump_user_1", strconv.Itoa(f.ann)); w.Code != http.StatusSeeOther {
		t.Fatalf("%d", w.Code)
	}
	for _, n := range f.db.ListNotifications(coach.ID, 10) {
		if n.Subject == "Game rescheduled" {
			t.Fatal("an edit that changes nothing must not notify")
		}
	}
	if w := f.edit("2026-07-10T14:00", "ump_user_1", strconv.Itoa(f.ann)); w.Code != http.StatusSeeOther {
		t.Fatalf("%d", w.Code)
	}
	var coachGot, umpGot bool
	for _, n := range f.db.ListNotifications(coach.ID, 10) {
		coachGot = coachGot || n.Subject == "Game rescheduled"
	}
	for _, n := range f.db.ListNotifications(f.ann, 10) {
		umpGot = umpGot || n.Subject == "Game changed — please re-confirm"
	}
	if !coachGot || !umpGot {
		t.Fatalf("coach %v umpire %v", coachGot, umpGot)
	}
}
