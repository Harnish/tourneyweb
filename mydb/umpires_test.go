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
