package mydb_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

func TestExtraRoleNames(t *testing.T) {
	db := mydb.NewFakeDB()
	t1 := db.AddTournament("T1", "baseball", "L", "", time.Now(), "published")
	t2 := db.AddTournament("T2", "baseball", "L", "", time.Now(), "published")
	id, err := db.AddExtraRole(t1, "Scorekeeper")
	if err != nil || id == 0 {
		t.Fatalf("add: %v", err)
	}
	if _, err := db.AddExtraRole(t1, " scorekeeper "); err != mydb.ErrRoleNameTaken {
		t.Fatalf("case/space duplicate: %v", err)
	}
	if _, err := db.AddExtraRole(t1, "  "); err != mydb.ErrRoleNameInvalid {
		t.Fatalf("empty: %v", err)
	}
	if _, err := db.AddExtraRole(t1, strings.Repeat("x", 41)); err != mydb.ErrRoleNameInvalid {
		t.Fatalf("41 chars: %v", err)
	}
	if _, err := db.AddExtraRole(t2, "Scorekeeper"); err != nil {
		t.Fatalf("same name in another tournament: %v", err)
	}
	if err := db.RenameExtraRole(t1, id, "SCOREKEEPER"); err != nil {
		t.Fatalf("rename to own name, new case: %v", err)
	}
	other, _ := db.AddExtraRole(t1, "Safety")
	if err := db.RenameExtraRole(t1, other, "scorekeeper"); err != mydb.ErrRoleNameTaken {
		t.Fatalf("rename onto existing: %v", err)
	}
	if got := db.TournamentExtraRoles(t1); len(got) != 2 || got[0].Name != "Safety" || got[1].Name != "SCOREKEEPER" {
		t.Fatalf("list sorted by name: %+v", got)
	}
}

func TestDivisionRoleNeedsClamp(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T1", "baseball", "L", "", time.Now(), "published")
	did := db.AddDivision(tid, "10U")
	rid, _ := db.AddExtraRole(tid, "Scorekeeper")
	db.SetDivisionRoleNeeded(did, rid, 7)
	if got := db.DivisionRoleNeeds(did); got[rid] != 5 {
		t.Fatalf("clamp to 5: %v", got)
	}
	db.SetDivisionRoleNeeded(did, rid, 0)
	if len(db.DivisionRoleNeeds(did)) != 0 {
		t.Fatal("0 removes the row")
	}
	db.SetDivisionRoleNeeded(did, rid, 2)
	db.DelExtraRole(tid, rid)
	if len(db.DivisionRoleNeeds(did)) != 0 || db.ExtraRoleByID(rid).ID != 0 {
		t.Fatal("deleting a role removes its needs")
	}
}

func TestSearchUsers(t *testing.T) {
	db := mydb.NewFakeDB()
	for _, n := range []string{"Ann Lee", "Anna Bell", "Bob Ray"} {
		db.CreateUser(strings.ToLower(strings.Fields(n)[0])+"@x.com", n, "h", "", true, false)
	}
	if got := db.SearchUsers("an", 10); len(got) != 2 || got[0].Name != "Ann Lee" {
		t.Fatalf("name match sorted: %+v", got)
	}
	if got := db.SearchUsers("BOB@", 10); len(got) != 1 {
		t.Fatalf("email match: %+v", got)
	}
	if db.SearchUsers("a", 10) != nil {
		t.Fatal("1 char returns nothing")
	}
	if got := db.SearchUsers("an", 1); len(got) != 1 {
		t.Fatalf("limit: %+v", got)
	}
}
