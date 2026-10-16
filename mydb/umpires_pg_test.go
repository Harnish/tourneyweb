package mydb_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

// Runs against a real Postgres only when TEST_DATABASE_URL is set, e.g.
// TEST_DATABASE_URL='postgres://postgres:pw@localhost:55432/tourneyweb?sslmode=disable' go test ./mydb -run Postgres
func TestUmpiresPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db := mydb.New(url, false)
	var d mydb.DB = db
	sfx := fmt.Sprint(time.Now().UnixNano())

	tid := d.AddTournament("PG "+sfx, "baseball", "L", "", time.Now(), "published")
	did := d.AddDivision(tid, "10U")
	d.AddTeam(tid, did, "A", "")
	d.AddTeam(tid, did, "B", "")
	tm := d.ReturnTeamsByDivisionID(did)
	start := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	g1 := d.AddGame(tid, did, tm[0].ID, tm[1].ID, "F1", start, "")
	g2 := d.AddGame(tid, did, tm[0].ID, tm[1].ID, "F2", start.Add(time.Hour), "")
	g3 := d.AddGame(tid, did, tm[0].ID, tm[1].ID, "F3", start.Add(2*time.Hour), "")
	ann, _ := d.CreateUser("ann"+sfx+"@x.com", "Ann", "h", "", true, false)
	bob, _ := d.CreateUser("bob"+sfx+"@x.com", "Bob", "h", "", true, false)
	d.SetUmpireProfile(ann.ID, true, "https://c.example")
	d.SetUmpireProfile(bob.ID, true, "")

	if u, ok := d.GetUmpire(ann.ID); !ok || u.CertURL != "https://c.example" {
		t.Fatalf("GetUmpire: %+v %v", u, ok)
	}
	d.SetUmpireCertImage(ann.ID, []byte{1, 2, 3}, "image/png")
	if b, ct, ok := d.GetUmpireCertImage(ann.ID); !ok || ct != "image/png" || len(b) != 3 {
		t.Fatal("cert image round trip")
	}
	d.SetUmpireVerified(tid, ann.ID, bob.ID, true)
	if !d.VerifiedUmpireIDs(tid)[ann.ID] {
		t.Fatal("verified")
	}
	d.SetUmpireVerified(tid, ann.ID, bob.ID, false)
	if d.VerifiedUmpireIDs(tid)[ann.ID] {
		t.Fatal("unverified")
	}
	d.SetDivisionUmpiresNeeded(did, 9)
	if d.DivisionUmpiresNeeded(did) != 4 {
		t.Fatal("umpires needed clamps to 4")
	}

	if err := d.AssignUmpire(g1, 1, ann.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.AssignUmpire(g2, 1, ann.ID, ""); err != mydb.ErrUmpireConflict {
		t.Fatalf("want conflict, got %v", err)
	}
	if err := d.AssignUmpire(g3, 1, ann.ID, ""); err != nil {
		t.Fatalf("back-to-back must be allowed: %v", err)
	}
	d.ClearUmpireSlot(g3, 1) // g3 (+2h) would overlap g2 (+1h) below
	if err := d.AssignUmpire(g1, 2, ann.ID, ""); err != mydb.ErrUmpireDuplicate {
		t.Fatalf("want duplicate, got %v", err)
	}
	if err := d.AssignUmpire(g1, 2, 0, "Free K."); err != nil {
		t.Fatal(err)
	}
	if got := d.ReturnGameByID(g1).Umpire; got != "Ann, Free K." {
		t.Fatalf("cache: %q", got)
	}
	if err := d.RespondUmpireSlot(g1, 1, bob.ID, true); err == nil {
		t.Fatal("wrong user must not respond")
	}
	if err := d.RespondUmpireSlot(g1, 1, ann.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := d.ReturnGameByID(g1).Umpire; got != "Free K." {
		t.Fatalf("after decline: %q", got)
	}
	if err := d.AssignUmpire(g2, 1, ann.ID, ""); err != nil {
		t.Fatalf("declined must not block: %v", err)
	}
	if err := d.RespondUmpireSlot(g1, 1, ann.ID, true); err != mydb.ErrUmpireSlotDeclined {
		t.Fatalf("accept after decline must be refused, got %v", err)
	}
	d.RespondUmpireSlot(g2, 1, ann.ID, true)
	if r := d.ResetUmpireSlotsPending(g2); len(r) != 1 || r[0].UserEmail == "" {
		t.Fatalf("reset: %+v", r)
	}
	if len(d.UmpireSlotsForUser(ann.ID)) != 2 {
		t.Fatalf("slots for user: %d", len(d.UmpireSlotsForUser(ann.ID)))
	}
	d.ClearUmpireSlot(g1, 2)
	if d.ReturnGameByID(g1).Umpire != "" {
		t.Fatal("clear updates cache")
	}
	d.AssignUmpire(g3, 2, 0, "Gone G.")
	d.DelGame(g3)
	if len(d.GameUmpires(g3)) != 0 {
		t.Fatal("delete cascades")
	}

	// legacy migration: umpire text becomes a free-form slot, idempotently
	g4 := d.AddGame(tid, did, tm[0].ID, tm[1].ID, "F4", start.Add(24*time.Hour), "Legacy L.")
	mydb.New(url, false)
	mydb.New(url, false)
	if s := d.GameUmpires(g4); len(s) != 1 || s[0].FreeName != "Legacy L." || s[0].Status != "accepted" {
		t.Fatalf("legacy migration: %+v", s)
	}
	_ = db
}
