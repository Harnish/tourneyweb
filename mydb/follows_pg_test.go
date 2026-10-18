package mydb_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

func TestFollowsPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	var d mydb.DB = mydb.New(url, false)
	sfx := fmt.Sprint(time.Now().UnixNano())
	tid := d.AddTournament("Follow "+sfx, "baseball", "L", "", time.Now(), "published")
	did := d.AddDivision(tid, "10U")
	d.AddTeam(tid, did, "Hawks", "")
	d.AddTeam(tid, did, "Bears", "")
	teams := d.ReturnTeamsByDivisionID(did)
	gid := d.AddGame(tid, did, teams[0].ID, teams[1].ID, "F1", t10, "")
	g := d.ReturnGameByID(gid)
	mk := func(n string) int {
		u, err := d.CreateUser(n+sfx+"@x.com", n+sfx, "h", "", true, false)
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	ann, bob, cy := mk("Ann"), mk("Bob"), mk("Cy")

	for i := 0; i < 2; i++ {
		if err := d.AddFollow(ann, "division", did); err != nil {
			t.Fatal(err)
		}
	}
	d.AddFollow(bob, "team", teams[1].ID)
	d.AddFollow(ann, "team", teams[0].ID)
	d.AddFollow(cy, "tournament", tid)
	if got := d.UserFollows(ann); len(got) != 2 || got[0].Kind != "division" || got[0].Name != "10U" || got[0].TournamentID != tid || got[1].Name != "Hawks" {
		t.Fatalf("UserFollows: %+v", got)
	}
	got := d.GameFollowers(g)
	if len(got) != 2 || got[ann].Kind != "team" || got[bob].Kind != "team" {
		t.Fatalf("GameFollowers: %+v", got)
	}
	if m := d.FollowersMatching(sfx, tid, did, 10); len(m) != 2 || m[0].ID != ann || m[1].ID != cy {
		t.Fatalf("FollowersMatching: %+v", m)
	}
	if !d.IsFollowing(cy, "tournament", tid) {
		t.Fatal("IsFollowing")
	}

	capUser := mk("Cap")
	for i := 0; i < mydb.MaxFollows; i++ {
		if err := d.AddFollow(capUser, "team", 900000+i); err != nil {
			t.Fatalf("follow %d: %v", i, err)
		}
	}
	if err := d.AddFollow(capUser, "team", 999999); err != mydb.ErrFollowCap {
		t.Fatalf("cap: %v", err)
	}
	if err := d.AddFollow(capUser, "team", 900000); err != nil {
		t.Fatalf("re-follow at cap: %v", err)
	}

	d.SetMuteFollowAlerts(ann, true)
	if !d.MutesFollowAlerts(ann) || d.MutesScoreNotices(ann) {
		t.Fatal("mute flags")
	}
	if s := d.SearchTournaments("follow " + sfx); len(s) != 1 || s[0].ID != tid {
		t.Fatalf("SearchTournaments: %+v", s)
	}

	d.DelTeam(teams[0].ID)
	if d.IsFollowing(ann, "team", teams[0].ID) {
		t.Fatal("DelTeam keeps follow")
	}
	d.DelTournament(tid)
	if d.IsFollowing(ann, "division", did) || d.IsFollowing(cy, "tournament", tid) || d.IsFollowing(bob, "team", teams[1].ID) {
		t.Fatal("DelTournament keeps follows")
	}
}
