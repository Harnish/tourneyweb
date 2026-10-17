package mydb_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

func TestExtraRolesPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	var d mydb.DB = mydb.New(url, false)
	sfx := fmt.Sprint(time.Now().UnixNano())
	tid := d.AddTournament("PGR "+sfx, "baseball", "L", "", time.Now(), "published")
	did := d.AddDivision(tid, "10U")
	rid, err := d.AddExtraRole(tid, "Scorekeeper")
	if err != nil || rid == 0 {
		t.Fatalf("add: %v", err)
	}
	if _, err := d.AddExtraRole(tid, " scorekeeper "); err != mydb.ErrRoleNameTaken {
		t.Fatalf("unique index: %v", err)
	}
	other, _ := d.AddExtraRole(tid, "Safety")
	if err := d.RenameExtraRole(tid, other, "SCOREKEEPER"); err != mydb.ErrRoleNameTaken {
		t.Fatalf("rename onto existing: %v", err)
	}
	d.SetDivisionRoleNeeded(did, rid, 9)
	if d.DivisionRoleNeeds(did)[rid] != 5 {
		t.Fatalf("clamp: %v", d.DivisionRoleNeeds(did))
	}
	ann, _ := d.CreateUser("annr"+sfx+"@x.com", "Annr"+sfx, "h", "", true, false)
	if got := d.SearchUsers("annr"+sfx, 10); len(got) != 1 || got[0].ID != ann.ID {
		t.Fatalf("search: %+v", got)
	}
	if got := d.SearchUsers("100%_", 10); len(got) != 0 {
		t.Fatalf("LIKE wildcards must be escaped: %+v", got)
	}
	d.DelExtraRole(tid, rid)
	if len(d.DivisionRoleNeeds(did)) != 0 || d.ExtraRoleByID(rid).ID != 0 {
		t.Fatal("role delete cascades needs")
	}
}

func TestRoleSlotsPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	var d mydb.DB = mydb.New(url, false)
	sfx := fmt.Sprint(time.Now().UnixNano())
	tid := d.AddTournament("PGS "+sfx, "baseball", "L", "", time.Now(), "published")
	did := d.AddDivision(tid, "10U")
	d.AddTeam(tid, did, "A", "")
	d.AddTeam(tid, did, "B", "")
	tm := d.ReturnTeamsByDivisionID(did)
	start := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	g1 := d.AddGame(tid, did, tm[0].ID, tm[1].ID, "F1", start, "")
	g2 := d.AddGame(tid, did, tm[0].ID, tm[1].ID, "F2", start.Add(time.Hour), "")
	g3 := d.AddGame(tid, did, tm[0].ID, tm[1].ID, "F3", start.Add(2*time.Hour), "")
	ann, _ := d.CreateUser("anns"+sfx+"@x.com", "Ann", "h", "", true, false)
	rid, _ := d.AddExtraRole(tid, "Scorekeeper")

	if err := d.AssignUmpire(g1, 1, ann.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.AssignRoleSlot(g2, rid, 1, ann.ID, ""); err != mydb.ErrUmpireConflict {
		t.Fatalf("role vs umpire overlap: %v", err)
	}
	if err := d.AssignRoleSlot(g1, rid, 1, ann.ID, ""); err != mydb.ErrUmpireDuplicate {
		t.Fatalf("same game: %v", err)
	}
	if err := d.AssignRoleSlot(g3, rid, 1, ann.ID, ""); err != nil {
		t.Fatalf("back-to-back ok: %v", err)
	}
	if err := d.AssignUmpire(g2, 1, ann.ID, ""); err != mydb.ErrUmpireConflict {
		t.Fatalf("umpire vs role overlap: %v", err)
	}
	if err := d.RespondRoleSlot(g3, rid, 1, ann.ID, true); err != nil {
		t.Fatal(err)
	}
	if s := d.RoleSlotsForUser(ann.ID); len(s) != 1 || s[0].Status != "accepted" || s[0].RoleName != "Scorekeeper" {
		t.Fatalf("for user: %+v", s)
	}
	if got := d.ResetRoleSlotsPending(g3); len(got) != 1 || d.GameRoleSlots(g3)[0].Status != "pending" {
		t.Fatalf("reset: %+v", got)
	}
	d.DelGame(g3)
	if len(d.GameRoleSlots(g3)) != 0 {
		t.Fatal("DelGame cascades role slots")
	}
	d.AssignRoleSlot(g2, rid, 1, 0, "Wes")
	d.DelExtraRole(tid, rid)
	if len(d.GameRoleSlots(g2)) != 0 {
		t.Fatal("role delete cascades slots")
	}
}
