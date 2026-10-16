package mydb_test

import (
	"testing"
	"time"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func newUmpire(t *testing.T, db *mydb.FakeDB, email, name string) int {
	t.Helper()
	u, err := db.CreateUser(email, name, "hash", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func TestUmpireProfile(t *testing.T) {
	db := mydb.NewFakeDB()
	id := newUmpire(t, db, "u@x.com", "Una")
	if _, ok := db.GetUmpire(id); ok {
		t.Fatal("unflagged user must not be an umpire")
	}
	db.SetUmpireProfile(id, true, "http://cert.example/una")
	u, ok := db.GetUmpire(id)
	if !ok || u.CertURL != "http://cert.example/una" || u.Name != "Una" {
		t.Fatalf("got %+v ok=%v", u, ok)
	}
	if len(db.ListUmpires()) != 1 {
		t.Fatal("flagged umpire should be listed")
	}
	db.SetUmpireProfile(id, false, "")
	if _, ok := db.GetUmpire(id); ok || len(db.ListUmpires()) != 0 {
		t.Fatal("unflagged umpire must disappear from GetUmpire/ListUmpires")
	}
}

func TestUmpireCertImage(t *testing.T) {
	db := mydb.NewFakeDB()
	id := newUmpire(t, db, "u@x.com", "Una")
	db.SetUmpireProfile(id, true, "")
	if _, _, ok := db.GetUmpireCertImage(id); ok {
		t.Fatal("no image yet")
	}
	db.SetUmpireCertImage(id, []byte{1, 2, 3}, "image/png")
	data, ct, ok := db.GetUmpireCertImage(id)
	if !ok || ct != "image/png" || len(data) != 3 {
		t.Fatalf("got %v %q %v", data, ct, ok)
	}
	if u, _ := db.GetUmpire(id); !u.HasCertImage {
		t.Fatal("HasCertImage should be true")
	}
}

func TestUmpireVerifiedPerTournament(t *testing.T) {
	db := mydb.NewFakeDB()
	id := newUmpire(t, db, "u@x.com", "Una")
	db.SetUmpireVerified(1, id, 99, true)
	if !db.VerifiedUmpireIDs(1)[id] {
		t.Fatal("verified in t1")
	}
	if db.VerifiedUmpireIDs(2)[id] {
		t.Fatal("must not be verified in t2")
	}
	db.SetUmpireVerified(1, id, 99, false)
	if db.VerifiedUmpireIDs(1)[id] {
		t.Fatal("unverify should clear")
	}
}

func TestDivisionUmpiresNeeded(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "published")
	did := db.AddDivision(tid, "10U")
	if got := db.DivisionUmpiresNeeded(did); got != 1 {
		t.Fatalf("default: got %d want 1", got)
	}
	db.SetDivisionUmpiresNeeded(did, 2)
	if got := db.DivisionUmpiresNeeded(did); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
	db.SetDivisionUmpiresNeeded(did, 0)
	if got := db.DivisionUmpiresNeeded(did); got != 1 {
		t.Fatalf("0 clamps to 1, got %d", got)
	}
	db.SetDivisionUmpiresNeeded(did, 9)
	if got := db.DivisionUmpiresNeeded(did); got != 4 {
		t.Fatalf("9 clamps to 4, got %d", got)
	}
}

// slotFixture: one tournament, one division, two umpires, helper to add games.
func slotFixture(t *testing.T) (*mydb.FakeDB, int, func(start time.Time) int, int, int) {
	t.Helper()
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "published")
	did := db.AddDivision(tid, "10U")
	db.AddTeam(tid, did, "A", "")
	db.AddTeam(tid, did, "B", "")
	teams := db.ReturnTeamsByDivisionID(did)
	add := func(start time.Time) int {
		return db.AddGame(tid, did, teams[0].ID, teams[1].ID, "F1", start, "")
	}
	u1 := newUmpire(t, db, "a@x.com", "Ann")
	u2 := newUmpire(t, db, "b@x.com", "Bob")
	db.SetUmpireProfile(u1, true, "")
	db.SetUmpireProfile(u2, true, "")
	return db, tid, add, u1, u2
}

var t10 = time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)

func TestAssignConflictAcrossTournaments(t *testing.T) {
	db, _, add, u1, _ := slotFixture(t)
	g1 := add(t10)
	tid2 := db.AddTournament("T2", "baseball", "L", "", time.Now(), "published")
	did2 := db.AddDivision(tid2, "12U")
	db.AddTeam(tid2, did2, "C", "")
	db.AddTeam(tid2, did2, "D", "")
	tm := db.ReturnTeamsByDivisionID(did2)
	g2 := db.AddGame(tid2, did2, tm[0].ID, tm[1].ID, "F9", t10.Add(time.Hour+59*time.Minute), "")
	if err := db.AssignUmpire(g1, 1, u1, ""); err != nil {
		t.Fatal(err)
	}
	if err := db.AssignUmpire(g2, 1, u1, ""); err != mydb.ErrUmpireConflict {
		t.Fatalf("want ErrUmpireConflict, got %v", err)
	}
}

func TestAssignBackToBackOK(t *testing.T) {
	db, _, add, u1, _ := slotFixture(t)
	g1, g2 := add(t10), add(t10.Add(2*time.Hour))
	if err := db.AssignUmpire(g1, 1, u1, ""); err != nil {
		t.Fatal(err)
	}
	if err := db.AssignUmpire(g2, 1, u1, ""); err != nil {
		t.Fatalf("10:00 and 12:00 must not conflict: %v", err)
	}
}

func TestDeclinedDoesNotBlock(t *testing.T) {
	db, _, add, u1, _ := slotFixture(t)
	g1, g2 := add(t10), add(t10.Add(time.Hour))
	db.AssignUmpire(g1, 1, u1, "")
	if err := db.RespondUmpireSlot(g1, 1, u1, false); err != nil {
		t.Fatal(err)
	}
	if err := db.AssignUmpire(g2, 1, u1, ""); err != nil {
		t.Fatalf("declined slot must not block: %v", err)
	}
}

func TestZeroStartNeverConflicts(t *testing.T) {
	db, _, add, u1, _ := slotFixture(t)
	g1, g2 := add(time.Time{}), add(time.Time{})
	if err := db.AssignUmpire(g1, 1, u1, ""); err != nil {
		t.Fatal(err)
	}
	if err := db.AssignUmpire(g2, 1, u1, ""); err != nil {
		t.Fatalf("no start time = no conflict: %v", err)
	}
}

func TestDuplicateSameGame(t *testing.T) {
	db, _, add, u1, _ := slotFixture(t)
	g := add(t10)
	db.AssignUmpire(g, 1, u1, "")
	if err := db.AssignUmpire(g, 2, u1, ""); err != mydb.ErrUmpireDuplicate {
		t.Fatalf("want ErrUmpireDuplicate, got %v", err)
	}
}

func TestRespondWrongUser(t *testing.T) {
	db, _, add, u1, u2 := slotFixture(t)
	g := add(t10)
	db.AssignUmpire(g, 1, u1, "")
	if err := db.RespondUmpireSlot(g, 1, u2, true); err == nil {
		t.Fatal("other user must not be able to respond")
	}
	if s := db.GameUmpires(g)[0]; s.Status != "pending" {
		t.Fatalf("status changed to %q", s.Status)
	}
}

func TestDeclineReopensAndCache(t *testing.T) {
	db, _, add, u1, _ := slotFixture(t)
	g := add(t10)
	db.AssignUmpire(g, 1, u1, "")
	db.AssignUmpire(g, 2, 0, "Bob K.")
	if got := db.ReturnGameByID(g).Umpire; got != "Ann, Bob K." {
		t.Fatalf("cache: got %q", got)
	}
	db.RespondUmpireSlot(g, 1, u1, false)
	if got := db.ReturnGameByID(g).Umpire; got != "Bob K." {
		t.Fatalf("after decline: got %q", got)
	}
	s := db.GameUmpires(g)[0]
	if s.Status != "declined" || s.UserID != u1 {
		t.Fatalf("declined slot should be kept for management: %+v", s)
	}
}

func TestFreeFormAcceptedAndClear(t *testing.T) {
	db, _, add, _, _ := slotFixture(t)
	g := add(t10)
	db.AssignUmpire(g, 1, 0, "Bob K.")
	if s := db.GameUmpires(g)[0]; s.Status != "accepted" || s.FreeName != "Bob K." {
		t.Fatalf("%+v", s)
	}
	db.ClearUmpireSlot(g, 1)
	if len(db.GameUmpires(g)) != 0 || db.ReturnGameByID(g).Umpire != "" {
		t.Fatal("clear should remove slot and cache")
	}
}

func TestResetPending(t *testing.T) {
	db, _, add, u1, _ := slotFixture(t)
	g := add(t10)
	db.AssignUmpire(g, 1, u1, "")
	db.AssignUmpire(g, 2, 0, "Bob K.")
	db.RespondUmpireSlot(g, 1, u1, true)
	got := db.ResetUmpireSlotsPending(g)
	if len(got) != 1 || got[0].UserID != u1 {
		t.Fatalf("only the accepted registered slot resets: %+v", got)
	}
	if db.GameUmpires(g)[0].Status != "pending" || db.GameUmpires(g)[1].Status != "accepted" {
		t.Fatal("wrong statuses after reset")
	}
}

func TestDelGameRemovesSlots(t *testing.T) {
	db, _, add, u1, _ := slotFixture(t)
	g := add(t10)
	db.AssignUmpire(g, 1, u1, "")
	db.DelGame(g)
	if len(db.GameUmpires(g)) != 0 || len(db.UmpireSlotsForUser(u1)) != 0 {
		t.Fatal("slots should be removed with the game")
	}
}

func TestUmpireWindowsOverlap(t *testing.T) {
	if !mydb.UmpireWindowsOverlap(t10, t10.Add(119*time.Minute)) {
		t.Fatal("10:00 vs 11:59 overlap")
	}
	if mydb.UmpireWindowsOverlap(t10, t10.Add(2*time.Hour)) || mydb.UmpireWindowsOverlap(t10.Add(2*time.Hour), t10) {
		t.Fatal("back-to-back must not overlap")
	}
}

func TestDeclinedSlotIsTerminal(t *testing.T) {
	db, _, add, u1, _ := slotFixture(t)
	g1, g2 := add(t10), add(t10.Add(time.Hour))
	db.AssignUmpire(g1, 1, u1, "")
	if err := db.RespondUmpireSlot(g1, 1, u1, false); err != nil {
		t.Fatal(err)
	}
	// declined freed the window; the umpire takes an overlapping game
	if err := db.AssignUmpire(g2, 1, u1, ""); err != nil {
		t.Fatal(err)
	}
	// re-accepting the declined slot would double-book; it must be refused
	if err := db.RespondUmpireSlot(g1, 1, u1, true); err == nil {
		t.Fatal("accepting a declined slot must fail")
	}
	if err := db.RespondUmpireSlot(g1, 1, u1, false); err == nil {
		t.Fatal("declining twice must fail (no repeat notifications)")
	}
	if db.GameUmpires(g1)[0].Status != "declined" {
		t.Fatal("status must stay declined")
	}
}

func TestCertLinkedAtSetWhenLinkChanges(t *testing.T) {
	db := mydb.NewFakeDB()
	id := newUmpire(t, db, "u@x.com", "Una")
	db.SetUmpireProfile(id, true, "")
	if u, _ := db.GetUmpire(id); !u.LinkedAt.IsZero() {
		t.Fatal("nothing linked yet")
	}
	db.SetUmpireProfile(id, true, "https://a.example")
	u, _ := db.GetUmpire(id)
	if u.LinkedAt.IsZero() {
		t.Fatal("linking a URL must set LinkedAt")
	}
	first := u.LinkedAt
	time.Sleep(2 * time.Millisecond)
	db.SetUmpireProfile(id, true, "https://a.example") // unchanged
	if u, _ := db.GetUmpire(id); !u.LinkedAt.Equal(first) {
		t.Fatal("re-saving the same link must not reset LinkedAt")
	}
	db.SetUmpireProfile(id, true, "https://b.example")
	if u, _ := db.GetUmpire(id); !u.LinkedAt.After(first) {
		t.Fatal("changing the link must reset LinkedAt")
	}
}

func TestCertChangeClearsExpiredFlags(t *testing.T) {
	db := mydb.NewFakeDB()
	id := newUmpire(t, db, "u@x.com", "Una")
	db.SetUmpireProfile(id, true, "https://a.example")
	db.SetUmpireExpiredFlag(1, id, 99, true)
	db.SetUmpireExpiredFlag(2, id, 99, true)
	db.SetUmpireProfile(id, true, "https://a.example")
	if !db.ExpiredFlagUmpireIDs(1)[id] {
		t.Fatal("unchanged link keeps flags")
	}
	db.SetUmpireCertImage(id, []byte{1, 2, 3}, "image/png")
	if db.ExpiredFlagUmpireIDs(1)[id] || db.ExpiredFlagUmpireIDs(2)[id] {
		t.Fatal("a new image clears flags in every tournament")
	}
	db.SetUmpireExpiredFlag(1, id, 99, true)
	db.SetUmpireProfile(id, true, "https://c.example")
	if db.ExpiredFlagUmpireIDs(1)[id] {
		t.Fatal("a new link clears flags")
	}
}

func TestExpiredFlagPerTournament(t *testing.T) {
	db := mydb.NewFakeDB()
	id := newUmpire(t, db, "u@x.com", "Una")
	db.SetUmpireExpiredFlag(1, id, 99, true)
	if !db.ExpiredFlagUmpireIDs(1)[id] || db.ExpiredFlagUmpireIDs(2)[id] {
		t.Fatal("flag applies to one tournament only")
	}
	db.SetUmpireExpiredFlag(1, id, 99, false)
	if db.ExpiredFlagUmpireIDs(1)[id] {
		t.Fatal("clear should remove the flag")
	}
}

func TestCertExpiryDate(t *testing.T) {
	db := mydb.NewFakeDB()
	id := newUmpire(t, db, "u@x.com", "Una")
	db.SetUmpireProfile(id, true, "")
	d := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	db.SetUmpireCertExpiry(id, d)
	if u, _ := db.GetUmpire(id); !u.ExpiresOn.Equal(d) {
		t.Fatalf("got %v", u.ExpiresOn)
	}
	db.SetUmpireCertExpiry(id, time.Time{})
	if u, _ := db.GetUmpire(id); !u.ExpiresOn.IsZero() {
		t.Fatal("zero clears the date")
	}
}

func TestUmpireCertExpired(t *testing.T) {
	today := time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		exp     time.Time
		flagged bool
		want    bool
	}{
		{"no date, no flag", time.Time{}, false, false},
		{"date yesterday", time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), false, true},
		{"date today is still valid", time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC), false, false},
		{"date tomorrow", time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC), false, false},
		{"flagged, no date", time.Time{}, true, true},
		{"flagged and future date", time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), true, true},
	}
	for _, c := range cases {
		u := mydb.Umpire{ExpiresOn: c.exp}
		if got := u.Expired(c.flagged, today); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
