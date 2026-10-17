package mydb_test

import (
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

func roleFixture(t *testing.T) (db *mydb.FakeDB, tid int, add func(time.Time) int, rid, ann, bob int) {
	t.Helper()
	db, tid, add, ann, bob = slotFixture(t)
	rid, err := db.AddExtraRole(tid, "Scorekeeper")
	if err != nil {
		t.Fatal(err)
	}
	return db, tid, add, rid, ann, bob
}

func TestRoleSlotLifecycle(t *testing.T) {
	db, _, add, rid, ann, bob := roleFixture(t)
	g := add(t10)
	if err := db.AssignRoleSlot(g, rid, 1, ann, ""); err != nil {
		t.Fatal(err)
	}
	s := db.GameRoleSlots(g)
	if len(s) != 1 || s[0].Status != "pending" || s[0].RoleName != "Scorekeeper" || s[0].UserName != "Ann" {
		t.Fatalf("assigned: %+v", s)
	}
	if err := db.RespondRoleSlot(g, rid, 1, bob, true); err == nil {
		t.Fatal("another user must not respond")
	}
	if err := db.RespondRoleSlot(g, rid, 1, ann, true); err != nil || db.GameRoleSlots(g)[0].Status != "accepted" {
		t.Fatalf("accept: %v", err)
	}
	if err := db.RespondRoleSlot(g, rid, 1, ann, false); err != nil || db.GameRoleSlots(g)[0].Status != "declined" {
		t.Fatalf("decline: %v", err)
	}
	if err := db.RespondRoleSlot(g, rid, 1, ann, true); err != mydb.ErrUmpireSlotDeclined {
		t.Fatalf("second respond: %v", err)
	}
	if err := db.AssignRoleSlot(g, rid, 2, 0, "Walk-in Wes"); err != nil {
		t.Fatal(err)
	}
	if s := db.GameRoleSlots(g); len(s) != 2 || s[1].Status != "accepted" || s[1].FreeName != "Walk-in Wes" {
		t.Fatalf("free-form accepted: %+v", s)
	}
	if got := db.RoleSlotsForUser(ann); len(got) != 1 || got[0].GameID != g {
		t.Fatalf("for user: %+v", got)
	}
	db.ClearRoleSlot(g, rid, 2)
	if len(db.GameRoleSlots(g)) != 1 {
		t.Fatal("clear")
	}
}

func TestOverlapAcrossUmpireAndRole(t *testing.T) {
	db, _, add, rid, ann, bob := roleFixture(t)
	a, b, c := add(t10), add(t10.Add(time.Hour)), add(t10.Add(2*time.Hour))
	if err := db.AssignUmpire(a, 1, ann, ""); err != nil {
		t.Fatal(err)
	}
	if err := db.AssignRoleSlot(b, rid, 1, ann, ""); err != mydb.ErrUmpireConflict {
		t.Fatalf("role after umpire overlap: %v", err)
	}
	if err := db.AssignRoleSlot(c, rid, 1, ann, ""); err != nil {
		t.Fatalf("back-to-back is fine: %v", err)
	}
	if err := db.AssignUmpire(b, 1, ann, ""); err != mydb.ErrUmpireConflict {
		t.Fatalf("umpire after role overlap: %v", err)
	}
	db.AssignRoleSlot(a, rid, 1, bob, "")
	db.RespondRoleSlot(a, rid, 1, bob, false) // declined no longer blocks
	if err := db.AssignUmpire(b, 1, bob, ""); err != nil {
		t.Fatalf("declined slot must not block: %v", err)
	}
}

func TestSameGameDoubleSlotRefused(t *testing.T) {
	db, _, add, rid, ann, _ := roleFixture(t)
	g := add(t10)
	db.AssignUmpire(g, 1, ann, "")
	if err := db.AssignRoleSlot(g, rid, 1, ann, ""); err != mydb.ErrUmpireDuplicate {
		t.Fatalf("role: %v", err)
	}
	g2 := add(t10.Add(5 * time.Hour))
	db.AssignRoleSlot(g2, rid, 1, ann, "")
	if err := db.AssignUmpire(g2, 1, ann, ""); err != mydb.ErrUmpireDuplicate {
		t.Fatalf("umpire: %v", err)
	}
	rid2, _ := db.AddExtraRole(1, "Safety")
	if err := db.AssignRoleSlot(g2, rid2, 1, ann, ""); err != mydb.ErrUmpireDuplicate {
		t.Fatalf("second role same game: %v", err)
	}
}

func TestResetRoleSlotsPending(t *testing.T) {
	db, _, add, rid, ann, bob := roleFixture(t)
	g := add(t10)
	db.AssignRoleSlot(g, rid, 1, ann, "")
	db.RespondRoleSlot(g, rid, 1, ann, true)
	db.AssignRoleSlot(g, rid, 2, bob, "") // still pending
	got := db.ResetRoleSlotsPending(g)
	if len(got) != 1 || got[0].UserID != ann {
		t.Fatalf("only accepted registered slots reset: %+v", got)
	}
	if db.GameRoleSlots(g)[0].Status != "pending" {
		t.Fatal("status not reset")
	}
}

func TestDelGameAndRoleRemoveRoleSlots(t *testing.T) {
	db, tid, add, rid, ann, _ := roleFixture(t)
	g, g2 := add(t10), add(t10.Add(5*time.Hour))
	db.AssignRoleSlot(g, rid, 1, ann, "")
	db.AssignRoleSlot(g2, rid, 1, ann, "")
	db.DelGame(g)
	if len(db.GameRoleSlots(g)) != 0 || len(db.GameRoleSlots(g2)) != 1 {
		t.Fatal("DelGame removes only that game's role slots")
	}
	db.DelExtraRole(tid, rid)
	if len(db.GameRoleSlots(g2)) != 0 {
		t.Fatal("DelExtraRole removes its slots")
	}
}
