package mydb_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

type followFx struct {
	db           *mydb.FakeDB
	tid, did     int
	teamA, teamB int
	game         mydb.Game
}

func newFollowFx(t *testing.T) *followFx {
	t.Helper()
	db := mydb.NewFakeDB()
	tid := db.AddTournament("Spring Slam", "baseball", "L", "", time.Now(), "published")
	did := db.AddDivision(tid, "10U")
	db.AddTeam(tid, did, "Hawks", "")
	db.AddTeam(tid, did, "Bears", "")
	teams := db.ReturnTeamsByDivisionID(did)
	gid := db.AddGame(tid, did, teams[0].ID, teams[1].ID, "F1", t10, "")
	return &followFx{db: db, tid: tid, did: did, teamA: teams[0].ID, teamB: teams[1].ID, game: db.ReturnGameByID(gid)}
}

func (f *followFx) user(t *testing.T, name string) int {
	t.Helper()
	u, err := f.db.CreateUser(name+"@x.com", name, "h", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func TestAddFollowIdempotent(t *testing.T) {
	f := newFollowFx(t)
	u := f.user(t, "Ann")
	for i := 0; i < 2; i++ {
		if err := f.db.AddFollow(u, "division", f.did); err != nil {
			t.Fatal(err)
		}
	}
	got := f.db.UserFollows(u)
	if len(got) != 1 || got[0].Kind != "division" || got[0].Name != "10U" || got[0].TournamentID != f.tid {
		t.Fatalf("follows: %+v", got)
	}
	if !f.db.IsFollowing(u, "division", f.did) || f.db.IsFollowing(u, "team", f.did) {
		t.Fatal("IsFollowing")
	}
	f.db.DelFollow(u, "division", f.did)
	if len(f.db.UserFollows(u)) != 0 {
		t.Fatal("DelFollow")
	}
}

func TestAddFollowRejectsUnknownKind(t *testing.T) {
	f := newFollowFx(t)
	if err := f.db.AddFollow(f.user(t, "Ann"), "league", 1); err == nil {
		t.Fatal("unknown kind must be refused")
	}
}

func TestFollowCap(t *testing.T) {
	f := newFollowFx(t)
	u := f.user(t, "Ann")
	for i := 0; i < mydb.MaxFollows; i++ {
		if err := f.db.AddFollow(u, "team", 1000+i); err != nil {
			t.Fatalf("follow %d: %v", i, err)
		}
	}
	if err := f.db.AddFollow(u, "team", 5000); err != mydb.ErrFollowCap {
		t.Fatalf("51st follow: %v", err)
	}
	if err := f.db.AddFollow(u, "team", 1000); err != nil {
		t.Fatalf("re-following an existing target at the cap is a no-op: %v", err)
	}
}

func TestGameFollowers(t *testing.T) {
	f := newFollowFx(t)
	a, b, c, d := f.user(t, "Ann"), f.user(t, "Bob"), f.user(t, "Cy"), f.user(t, "Di")
	f.db.AddFollow(a, "division", f.did)
	f.db.AddFollow(b, "team", f.teamB)
	f.db.AddFollow(c, "division", f.did)
	f.db.AddFollow(c, "team", f.teamA)
	f.db.AddFollow(d, "tournament", f.tid)
	got := f.db.GameFollowers(f.game)
	if len(got) != 3 || got[a].Kind != "division" || got[b].Kind != "team" || got[c].Kind != "team" || got[c].TargetID != f.teamA {
		t.Fatalf("followers: %+v", got)
	}
	if _, ok := got[d]; ok {
		t.Fatal("tournament followers get no score alerts")
	}
}

func TestDeletingTargetsRemovesFollows(t *testing.T) {
	f := newFollowFx(t)
	u := f.user(t, "Ann")
	f.db.AddFollow(u, "team", f.teamA)
	f.db.DelTeam(f.teamA)
	if f.db.IsFollowing(u, "team", f.teamA) {
		t.Fatal("DelTeam keeps follow")
	}
	f.db.AddFollow(u, "division", f.did)
	f.db.DelDivision(f.did)
	if f.db.IsFollowing(u, "division", f.did) {
		t.Fatal("DelDivision keeps follow")
	}
	f2 := newFollowFx(t)
	u2 := f2.user(t, "Bob")
	f2.db.AddFollow(u2, "tournament", f2.tid)
	f2.db.AddFollow(u2, "division", f2.did)
	f2.db.AddFollow(u2, "team", f2.teamB)
	f2.db.DelTournament(f2.tid)
	if n := len(f2.db.UserFollows(u2)); n != 0 {
		t.Fatalf("DelTournament leaves %d follows", n)
	}
}

func TestFollowersMatching(t *testing.T) {
	f := newFollowFx(t)
	zed, abe, other := f.user(t, "Zed Follower"), f.user(t, "Abe Follower"), f.user(t, "Zed Stranger")
	f.db.AddFollow(zed, "tournament", f.tid)
	f.db.AddFollow(abe, "division", f.did)
	otherDiv := f.db.AddDivision(f.tid, "12U")
	f.db.AddFollow(other, "division", otherDiv)
	got := f.db.FollowersMatching("follower", f.tid, f.did, 10)
	if len(got) != 2 || got[0].ID != abe || got[1].ID != zed {
		t.Fatalf("alphabetical followers: %+v", got)
	}
	if got := f.db.FollowersMatching("z", f.tid, f.did, 10); len(got) != 0 {
		t.Fatalf("one character must not search: %+v", got)
	}
	if got := f.db.FollowersMatching("zed", f.tid, f.did, 10); len(got) != 1 || got[0].ID != zed {
		t.Fatalf("a follower of another division is not a match: %+v", got)
	}
	if got := f.db.FollowersMatching("follower", f.tid, f.did, 1); len(got) != 1 {
		t.Fatalf("limit: %+v", got)
	}
}

func TestMuteFollowAlertsSeparateFromScoreMute(t *testing.T) {
	f := newFollowFx(t)
	u := f.user(t, "Ann")
	f.db.SetMuteFollowAlerts(u, true)
	if !f.db.MutesFollowAlerts(u) || f.db.MutesScoreNotices(u) {
		t.Fatal("follow mute must not touch the score-notice mute")
	}
	f.db.SetMuteScoreNotices(u, true)
	f.db.SetMuteFollowAlerts(u, false)
	if f.db.MutesFollowAlerts(u) || !f.db.MutesScoreNotices(u) {
		t.Fatal("independent flags")
	}
}

func TestSearchTournaments(t *testing.T) {
	db := mydb.NewFakeDB()
	db.AddTournament("Spring Slam", "baseball", "L", "", time.Now(), "published")
	db.AddTournament("Fall Classic", "baseball", "L", "", time.Now(), "published")
	db.AddTournament("Secret Slam", "baseball", "L", "", time.Now(), "draft")
	got := db.SearchTournaments(" SLAM ")
	if len(got) != 1 || got[0].Name != "Spring Slam" {
		t.Fatalf("search: %+v", got)
	}
	if db.SearchTournaments("  ") != nil {
		t.Fatal("empty query returns nothing")
	}
	for i := 0; i < 60; i++ {
		db.AddTournament(fmt.Sprintf("Cup %d", i), "baseball", "L", "", time.Now(), "published")
	}
	if n := len(db.SearchTournaments("cup")); n != 50 {
		t.Fatalf("cap: %d", n)
	}
}
