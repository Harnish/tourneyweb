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
