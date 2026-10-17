package webhandler

import (
	"errors"
	"strconv"
	"testing"

	"github.com/Harnish/tourneyweb/mydb"
)

type entryFx struct {
	*slotFx
	homeCoach, awayCoach TWUser
	director             int
}

// entryFixture: slot fixture plus a coach for each team of the game and one director.
func entryFixture(t *testing.T) *entryFx {
	t.Helper()
	f := newSlotFx(t)
	mk := func(email string, team int) TWUser {
		u, _ := f.db.CreateUser(email, email, "h", "", true, false)
		f.db.AssignRole(u.ID, f.tid, "coach", team)
		return TWUser{ID: u.ID, Name: email, Roles: []TournamentRole{{TournamentID: f.tid, Role: "coach", TeamID: team}}}
	}
	return &entryFx{slotFx: f,
		homeCoach: mk("homecoach@x.com", f.game.HomeTeam.ID),
		awayCoach: mk("awaycoach@x.com", f.game.AwayTeam.ID),
		director:  addDirector(f, "dir@x.com")}
}

func (f *entryFx) cur() mydb.Game { return f.db.ReturnGameByID(f.game.ID) }

func (f *entryFx) subjects(userID int) []string {
	var out []string
	for _, n := range f.db.ListNotifications(userID, 50) {
		out = append(out, n.Subject)
	}
	return out
}

func count(list []string, subject string) int {
	n := 0
	for _, s := range list {
		if s == subject {
			n++
		}
	}
	return n
}

func (f *entryFx) coachReport(who TWUser, reporter string, h, a int) error {
	return f.env.submitCoachScore(f.cur(), reporter, who, h, a)
}

func TestFirstCoachReportSetsStandingsAndNotifiesOtherCoach(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	if err := f.coachReport(f.homeCoach, "coach_home", 7, 3); err != nil {
		t.Fatal(err)
	}
	g := f.cur()
	if !g.Scored || g.HomeScore != 7 || g.AwayScore != 3 || g.ScoreStatus != "reported" {
		t.Fatalf("standings follow the first coach: %+v", g)
	}
	n := f.db.ListNotifications(f.awayCoach.ID, 10)
	wantLink := "/tournaments/" + strconv.Itoa(f.tid) + "/coach/games/" + strconv.Itoa(f.game.ID) + "/score"
	if len(n) != 1 || n[0].Subject != "Score reported — please confirm" || n[0].Link != wantLink {
		t.Fatalf("other coach notice: %+v", n)
	}
	if len(f.subjects(f.homeCoach.ID)) != 0 {
		t.Fatal("the reporter is not notified")
	}
	if count(f.subjects(f.director), "Game scored") != 1 {
		t.Fatalf("director told standings changed: %v", f.subjects(f.director))
	}
}

func TestMatchingReportsConfirmWithoutDirectorNotice(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.coachReport(f.homeCoach, "coach_home", 7, 3)
	if err := f.coachReport(f.awayCoach, "coach_away", 7, 3); err != nil {
		t.Fatal(err)
	}
	if f.cur().ScoreStatus != "confirmed" {
		t.Fatalf("status %q", f.cur().ScoreStatus)
	}
	if got := f.subjects(f.director); len(got) != 1 {
		t.Fatalf("a plain confirmation sends the director nothing new: %v", got)
	}
}

func TestDifferingReportsDisputeAndNotifyUmpireOnce(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.coachReport(f.homeCoach, "coach_home", 7, 3)
	f.coachReport(f.awayCoach, "coach_away", 7, 5)
	g := f.cur()
	if g.ScoreStatus != "disputed" || g.HomeScore != 7 || g.AwayScore != 3 {
		t.Fatalf("disputed, standings still follow the first coach: %+v", g)
	}
	n := f.db.ListNotifications(f.ann, 10)
	wantLink := "/tournaments/" + strconv.Itoa(f.tid) + "/umpire/games/" + strconv.Itoa(f.game.ID) + "/score"
	if len(n) != 1 || n[0].Subject != "Score disputed" || n[0].Link != wantLink {
		t.Fatalf("umpire notice: %+v", n)
	}
	f.coachReport(f.awayCoach, "coach_away", 6, 5) // still different: no second notice
	if len(f.db.ListNotifications(f.ann, 10)) != 1 {
		t.Fatal("notified once per transition into disputed")
	}
	if got := f.subjects(f.director); count(got, "Game score updated") != 0 {
		t.Fatalf("a second coach's differing report doesn't change standings: %v", got)
	}
}

func TestFirstReporterEditUpdatesStandingsKeepsFirst(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.coachReport(f.homeCoach, "coach_home", 7, 3)
	f.coachReport(f.awayCoach, "coach_away", 7, 5) // disputed
	f.coachReport(f.homeCoach, "coach_home", 8, 3) // first reporter edits
	if g := f.cur(); g.HomeScore != 8 || g.AwayScore != 3 || g.ScoreStatus != "disputed" {
		t.Fatalf("standings follow the first reporter's new value: %+v", g)
	}
	if count(f.subjects(f.director), "Game score updated") != 1 {
		t.Fatalf("director told of the changed standings: %v", f.subjects(f.director))
	}
	f.coachReport(f.awayCoach, "coach_away", 8, 3) // second now agrees
	if f.cur().ScoreStatus != "confirmed" {
		t.Fatalf("agreement confirms: %q", f.cur().ScoreStatus)
	}
}

func TestConfirmedAndFinalAreLockedForCoaches(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.coachReport(f.homeCoach, "coach_home", 7, 3)
	f.coachReport(f.awayCoach, "coach_away", 7, 3)
	if err := f.coachReport(f.homeCoach, "coach_home", 9, 9); !errors.Is(err, ErrScoreLocked) {
		t.Fatalf("confirmed: got %v", err)
	}
	f.env.markFinal(f.cur())
	if err := f.coachReport(f.awayCoach, "coach_away", 1, 0); !errors.Is(err, ErrScoreLocked) {
		t.Fatalf("final: got %v", err)
	}
	if g := f.cur(); g.HomeScore != 7 || g.AwayScore != 3 {
		t.Fatalf("locked scores must not change: %+v", g)
	}
}

func TestUmpireResolvesDisputeToFinal(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.coachReport(f.homeCoach, "coach_home", 7, 3)
	f.coachReport(f.awayCoach, "coach_away", 7, 5)
	if err := f.env.submitUmpireScore(f.cur(), TWUser{ID: f.ann, Name: "Ann"}, 6, 5); err != nil {
		t.Fatal(err)
	}
	if g := f.cur(); g.HomeScore != 6 || g.AwayScore != 5 || g.ScoreStatus != "final" {
		t.Fatalf("umpire's score is final: %+v", g)
	}
	if count(f.subjects(f.director), "Game score updated") != 1 {
		t.Fatalf("director told: %v", f.subjects(f.director))
	}
}

func TestUmpireGuards(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.db.RespondUmpireSlot(f.game.ID, 1, f.ann, false) // declined
	f.db.AssignUmpire(f.game.ID, 2, 0, "Free Ump")
	for name, who := range map[string]int{"unassigned": f.bob, "declined": f.ann} {
		if err := f.env.submitUmpireScore(f.cur(), TWUser{ID: who}, 4, 2); !errors.Is(err, ErrNotYourGame) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if f.cur().Scored {
		t.Fatal("rejected umpire entries store nothing")
	}
	f.db.AssignUmpire(f.game.ID, 1, f.bob, "")
	f.coachReport(f.homeCoach, "coach_home", 7, 3)
	f.coachReport(f.awayCoach, "coach_away", 7, 3) // confirmed
	if err := f.env.submitUmpireScore(f.cur(), TWUser{ID: f.bob}, 1, 0); !errors.Is(err, ErrScoreLocked) {
		t.Fatalf("umpire on a confirmed game: got %v", err)
	}
	if err := f.env.submitUmpireScore(f.cur(), TWUser{ID: f.bob}, 41, 0); err == nil {
		t.Fatal("out-of-range umpire score rejected")
	}
}

func TestDisputeFallsBackToDirectorsThenAdmins(t *testing.T) {
	f := entryFixture(t) // only a free-form umpire on the game
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, 0, "Free Ump")
	f.coachReport(f.homeCoach, "coach_home", 7, 3)
	f.coachReport(f.awayCoach, "coach_away", 7, 5)
	if count(f.subjects(f.director), "Score disputed") != 1 {
		t.Fatalf("directors notified when no registered umpire: %v", f.subjects(f.director))
	}
	// a tournament without directors falls back to admins
	notifyWG.Wait() // the seam is a package global: let the first fixture's mail finish before swapping it
	g2 := newSlotFx(t)
	defer g2.restore()
	admin, _ := g2.db.CreateUser("admin@x.com", "Admin", "h", "", true, true)
	ch, _ := g2.db.CreateUser("c1@x.com", "c1", "h", "", true, false)
	ca, _ := g2.db.CreateUser("c2@x.com", "c2", "h", "", true, false)
	g2.db.AssignRole(ch.ID, g2.tid, "coach", g2.game.HomeTeam.ID)
	g2.db.AssignRole(ca.ID, g2.tid, "coach", g2.game.AwayTeam.ID)
	hc := TWUser{ID: ch.ID, Roles: []TournamentRole{{TournamentID: g2.tid, Role: "coach", TeamID: g2.game.HomeTeam.ID}}}
	ac := TWUser{ID: ca.ID, Roles: []TournamentRole{{TournamentID: g2.tid, Role: "coach", TeamID: g2.game.AwayTeam.ID}}}
	g2.env.submitCoachScore(g2.db.ReturnGameByID(g2.game.ID), "coach_home", hc, 7, 3)
	g2.env.submitCoachScore(g2.db.ReturnGameByID(g2.game.ID), "coach_away", ac, 7, 5)
	var adminSubjects []string
	for _, n := range g2.db.ListNotifications(admin.ID, 10) {
		adminSubjects = append(adminSubjects, n.Subject)
	}
	if count(adminSubjects, "Score disputed") != 1 {
		t.Fatalf("admins notified as the last resort: %v", adminSubjects)
	}
}

func seedBracketGame(f *entryFx) int {
	b := f.db.CreateBracket(f.did, "single_elimination", 2)
	bg := f.db.AddBracketGame(b, 1, 1, "winners")
	f.db.SetBracketGameTeams(bg, f.game.HomeTeam.ID, f.game.AwayTeam.ID, false, false)
	f.db.SetBracketGameGameID(bg, f.game.ID)
	return bg
}

func TestBracketGameAdvancesOnlyOnConfirmedOrFinalAndRejectsTies(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	seedBracketGame(f)
	winner := func() int { return f.db.GetBracketGameByGameID(f.game.ID).WinnerTeamID }
	if err := f.coachReport(f.homeCoach, "coach_home", 4, 4); !errors.Is(err, ErrBracketTie) {
		t.Fatalf("tie in a bracket game: got %v", err)
	}
	f.coachReport(f.homeCoach, "coach_home", 7, 3)
	if winner() != 0 {
		t.Fatal("an unconfirmed report must not advance the bracket")
	}
	f.coachReport(f.awayCoach, "coach_away", 7, 3)
	if winner() != f.game.HomeTeam.ID {
		t.Fatalf("confirmed score advances the winner, got %d", winner())
	}
	// disputed then umpire final also advances
	notifyWG.Wait() // the mail seam is a package global: finish the first fixture's mail before swapping it
	f2 := entryFixture(t)
	defer f2.restore()
	f2.db.AssignUmpire(f2.game.ID, 1, f2.ann, "")
	seedBracketGame(f2)
	f2.coachReport(f2.homeCoach, "coach_home", 7, 3)
	f2.coachReport(f2.awayCoach, "coach_away", 2, 5)
	if f2.db.GetBracketGameByGameID(f2.game.ID).WinnerTeamID != 0 {
		t.Fatal("disputed bracket game must not advance")
	}
	f2.env.submitUmpireScore(f2.cur(), TWUser{ID: f2.ann}, 2, 5)
	if f2.db.GetBracketGameByGameID(f2.game.ID).WinnerTeamID != f2.game.AwayTeam.ID {
		t.Fatal("umpire final advances the winner")
	}
}

func TestCoachReporterIsTeamAndTournamentScoped(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	g := f.cur()
	if r, ok := coachReporter(f.homeCoach, g); !ok || r != "coach_home" {
		t.Fatalf("home coach: %q %v", r, ok)
	}
	if r, ok := coachReporter(f.awayCoach, g); !ok || r != "coach_away" {
		t.Fatalf("away coach: %q %v", r, ok)
	}
	outsider := TWUser{ID: 99, Roles: []TournamentRole{{TournamentID: f.tid, Role: "coach", TeamID: 424242}}}
	elsewhere := TWUser{ID: 98, Roles: []TournamentRole{{TournamentID: f.tid + 1, Role: "coach", TeamID: g.HomeTeam.ID}}}
	staff := TWUser{ID: 97, Roles: []TournamentRole{{TournamentID: f.tid, Role: "staff"}}}
	for name, u := range map[string]TWUser{"other team": outsider, "other tournament": elsewhere, "staff": staff, "anonymous": {}} {
		if _, ok := coachReporter(u, g); ok {
			t.Errorf("%s must not be a reporter", name)
		}
	}
	if err := f.env.submitCoachScore(g, "coach_bogus", f.homeCoach, 1, 0); !errors.Is(err, ErrNotYourGame) {
		t.Fatalf("bad reporter key: got %v", err)
	}
}

func TestValidScoreBounds(t *testing.T) {
	for _, c := range []struct {
		h, a int
		ok   bool
	}{{0, 0, true}, {40, 40, true}, {-1, 0, false}, {0, -1, false}, {41, 0, false}, {0, 41, false}} {
		if validScore(c.h, c.a) != c.ok {
			t.Errorf("validScore(%d,%d) != %v", c.h, c.a, c.ok)
		}
	}
	f := entryFixture(t)
	defer f.restore()
	if err := f.coachReport(f.homeCoach, "coach_home", 41, 0); !errors.Is(err, ErrBadScore) || f.cur().Scored {
		t.Fatalf("out-of-range report: err=%v scored=%v", err, f.cur().Scored)
	}
	if len(f.db.ScoreReportsForGame(f.game.ID)) != 0 {
		t.Fatal("nothing stored for a rejected report")
	}
}

func TestMarkFinalClearsDispute(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.coachReport(f.homeCoach, "coach_home", 7, 3)
	f.coachReport(f.awayCoach, "coach_away", 7, 5)
	f.env.markFinal(f.cur())
	if f.cur().ScoreStatus != "final" {
		t.Fatalf("status %q", f.cur().ScoreStatus)
	}
}


func TestDisputeLinkMatchesWhoGetsIt(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, 0, "Free Ump") // no registered umpire: the director is told instead
	f.coachReport(f.homeCoach, "coach_home", 7, 3)
	f.coachReport(f.awayCoach, "coach_away", 7, 5)
	var link string
	for _, n := range f.db.ListNotifications(f.director, 10) {
		if n.Subject == "Score disputed" {
			link = n.Link
		}
	}
	want := "/tournaments/" + strconv.Itoa(f.tid) + "/score/games/" + strconv.Itoa(f.game.ID)
	if link != want {
		t.Fatalf("a director can't open the umpire form; link %q want %q", link, want)
	}
}

func TestConcurrentCoachReportsEndConfirmed(t *testing.T) {
	for i := 0; i < 30; i++ {
		f := entryFixture(t)
		done := make(chan struct{}, 2)
		go func() { f.coachReport(f.homeCoach, "coach_home", 7, 3); done <- struct{}{} }()
		go func() { f.coachReport(f.awayCoach, "coach_away", 7, 3); done <- struct{}{} }()
		<-done
		<-done
		if got := f.cur().ScoreStatus; got != "confirmed" {
			t.Fatalf("iteration %d: two matching concurrent reports must end confirmed, got %q", i, got)
		}
		if n := count(f.subjects(f.awayCoach.ID), "Score reported — please confirm") + count(f.subjects(f.homeCoach.ID), "Score reported — please confirm"); n != 1 {
			t.Fatalf("iteration %d: exactly one 'please confirm' notice expected, got %d", i, n)
		}
		f.restore()
	}
}

type noReportsDB struct{ *mydb.FakeDB }

func (noReportsDB) ScoreReportsForGame(int) []mydb.ScoreReport { return nil }

func TestFailedReportSaveReturnsErrorInsteadOfPanicking(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	env := &Env{DB: noReportsDB{f.db}}
	err := env.submitCoachScore(f.cur(), "coach_home", f.homeCoach, 7, 3)
	if !errors.Is(err, ErrSaveFailed) {
		t.Fatalf("got %v want ErrSaveFailed", err)
	}
}

func TestCoachOfBothTeamsCannotReport(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	both := TWUser{ID: 55, Roles: []TournamentRole{
		{TournamentID: f.tid, Role: "coach", TeamID: f.game.HomeTeam.ID},
		{TournamentID: f.tid, Role: "coach", TeamID: f.game.AwayTeam.ID},
	}}
	if _, ok := coachReporter(both, f.cur()); ok {
		t.Fatal("a user who coaches both teams must not be able to confirm their own report")
	}
}

func TestCoachWhoIsAlsoTheUmpireCannotScoreFinal(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.homeCoach.ID, "")
	if err := f.env.submitUmpireScore(f.cur(), f.homeCoach, 7, 3); !errors.Is(err, ErrNotYourGame) {
		t.Fatalf("a coach of a team in the game must not enter the umpire's final score: %v", err)
	}
	if f.cur().Scored {
		t.Fatal("nothing stored")
	}
}

func TestStaffScoreIsValidated(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	staff := TWUser{ID: 999, Roles: []TournamentRole{{TournamentID: f.tid, Role: "staff"}}}
	if w := f.call(f.env.RecordScore, "POST", staff, f.game.ID, "homescore", "41", "awayscore", "0"); w.Code != 400 || f.cur().Scored {
		t.Fatalf("out of range: got %d scored=%v", w.Code, f.cur().Scored)
	}
	seedBracketGame(f)
	if w := f.call(f.env.RecordScore, "POST", staff, f.game.ID, "homescore", "4", "awayscore", "4"); w.Code != 400 || f.cur().Scored {
		t.Fatalf("bracket tie: got %d scored=%v", w.Code, f.cur().Scored)
	}
}
