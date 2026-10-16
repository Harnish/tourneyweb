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
	// certification expiry: linked date, expiry date, per-tournament flags cleared by a new link/image
	if u, _ := d.GetUmpire(ann.ID); u.LinkedAt.IsZero() {
		t.Fatal("LinkedAt must be set once a link/image exists")
	}
	d.SetUmpireCertExpiry(ann.ID, time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC))
	if u, _ := d.GetUmpire(ann.ID); u.ExpiresOn.Format("2006-01-02") != "2027-03-01" {
		t.Fatalf("expiry: %v", u.ExpiresOn)
	}
	d.SetUmpireVerified(tid, ann.ID, bob.ID, true)
	d.SetUmpireExpiredFlag(tid, ann.ID, bob.ID, true)
	d.SetUmpireProfile(ann.ID, true, "HTTPS://C.example") // scheme/host case only: no change
	if !d.VerifiedUmpireIDs(tid)[ann.ID] || !d.ExpiredFlagUmpireIDs(tid)[ann.ID] {
		t.Fatal("trivial link edit must keep verification and flag")
	}
	d.SetUmpireProfile(ann.ID, true, "https://d.example")
	if d.VerifiedUmpireIDs(tid)[ann.ID] || !d.ExpiredFlagUmpireIDs(tid)[ann.ID] {
		t.Fatal("a new link clears verification but keeps the flag")
	}
	if u, _ := d.GetUmpire(ann.ID); !u.LinkedAt.After(d.ExpiredFlagTimes(tid)[ann.ID]) {
		t.Fatal("linked date should be after the flag time")
	}
	d.SetUmpireVerified(tid, ann.ID, bob.ID, true)
	d.SetUmpireCertImage(ann.ID, []byte{9, 9}, "image/png")
	if d.VerifiedUmpireIDs(tid)[ann.ID] {
		t.Fatal("a new image clears verification")
	}
	d.SetUmpireVerified(tid, ann.ID, bob.ID, true)
	d.SetUmpireCertImage(ann.ID, []byte{9, 9}, "image/png")
	if !d.VerifiedUmpireIDs(tid)[ann.ID] {
		t.Fatal("identical image keeps verification")
	}
	d.SetUmpireCertExpiry(ann.ID, time.Time{})
	if u, _ := d.GetUmpire(ann.ID); !u.ExpiresOn.IsZero() {
		t.Fatal("zero clears expiry")
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

func TestNotificationsPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	var d mydb.DB = mydb.New(url, false)
	sfx := fmt.Sprint(time.Now().UnixNano())
	a, _ := d.CreateUser("na"+sfx+"@x.com", "A", "h", "", true, true)
	b, _ := d.CreateUser("nb"+sfx+"@x.com", "B", "h", "", true, false)
	id1 := d.AddNotification(a.ID, "first", "m", "/x")
	d.AddNotification(a.ID, "second", "", "")
	if all := d.ListNotifications(a.ID, 0); len(all) != 2 {
		t.Fatalf("limit 0 means no limit, got %d", len(all))
	}
	if got := d.ListNotifications(a.ID, 10); len(got) != 2 || got[0].Subject != "second" {
		t.Fatalf("newest first: %+v", got)
	}
	if d.CountUnreadNotifications(a.ID) != 2 || d.CountUnreadNotifications(b.ID) != 0 {
		t.Fatal("unread counts")
	}
	if _, ok := d.MarkNotificationRead(id1, b.ID); ok {
		t.Fatal("other user must not mark read")
	}
	n, ok := d.MarkNotificationRead(id1, a.ID)
	if !ok || n.Link != "/x" || n.ReadAt.IsZero() || d.CountUnreadNotifications(a.ID) != 1 {
		t.Fatalf("mark read: %+v %v", n, ok)
	}
	if _, ok := d.MarkNotificationRead(id1, a.ID); !ok {
		t.Fatal("idempotent")
	}
	d.MarkAllNotificationsRead(a.ID)
	if d.CountUnreadNotifications(a.ID) != 0 {
		t.Fatal("mark all")
	}
	if d.MutesScoreNotices(a.ID) {
		t.Fatal("default is not muted")
	}
	d.SetMuteScoreNotices(a.ID, true)
	if !d.MutesScoreNotices(a.ID) || d.MutesScoreNotices(b.ID) {
		t.Fatal("mute is per user")
	}
	d.SetMuteScoreNotices(a.ID, false)
	found := false
	for _, id := range d.ListAdminUserIDs() {
		found = found || id == a.ID
		if id == b.ID {
			t.Fatal("non-admin listed")
		}
	}
	if !found {
		t.Fatal("admin missing")
	}
}

func TestAvailabilityPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	var d mydb.DB = mydb.New(url, false)
	sfx := fmt.Sprint(time.Now().UnixNano())
	u, _ := d.CreateUser("av"+sfx+"@x.com", "Av", "h", "", true, false)
	soon := d.AddTournament("soon "+sfx, "baseball", "L", "", time.Now().Add(48*time.Hour), "published")
	old := d.AddTournament("old "+sfx, "baseball", "L", "", time.Now().Add(-30*24*time.Hour), "published")
	draft := d.AddTournament("draft "+sfx, "baseball", "L", "", time.Now().Add(48*time.Hour), "draft")
	if !d.SetUmpireAvailability(u.ID, soon, "weekends") {
		t.Fatal("first volunteer is new")
	}
	if d.SetUmpireAvailability(u.ID, soon, "weekends+Fri") {
		t.Fatal("note edit is not new")
	}
	if a := d.UmpireAvailabilityForUser(u.ID)[soon]; a.Note != "weekends+Fri" || a.CreatedAt.IsZero() {
		t.Fatalf("%+v", a)
	}
	if _, ok := d.UmpireAvailabilityForTournament(soon)[u.ID]; !ok {
		t.Fatal("per-tournament lookup")
	}
	ids := map[int]bool{}
	for _, tr := range d.ReturnTournamentsOpenForUmpires(time.Now().Add(-7 * 24 * time.Hour)) {
		ids[tr.ID] = true
	}
	if !ids[soon] || ids[old] || ids[draft] {
		t.Fatalf("open list wrong: %v", ids)
	}
	d.ClearUmpireAvailability(u.ID, soon)
	if _, ok := d.UmpireAvailabilityForTournament(soon)[u.ID]; ok {
		t.Fatal("withdrawn volunteers are not listed")
	}
	if d.SetUmpireAvailability(u.ID, soon, "") {
		t.Fatal("re-volunteering inside the cooldown must not notify")
	}
	d.ClearUmpireAvailability(u.ID, soon)
	db := d.(*mydb.MyDB)
	db.DB.Exec(`UPDATE umpire_availability SET notified_at = NOW() - INTERVAL '25 hours' WHERE user_id=$1 AND tournament_id=$2`, u.ID, soon)
	if !d.SetUmpireAvailability(u.ID, soon, "") {
		t.Fatal("re-volunteering after the cooldown notifies again")
	}
}

func TestUmpirePayPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	var d mydb.DB = mydb.New(url, false)
	sfx := fmt.Sprint(time.Now().UnixNano())
	tid := d.AddTournament("Pay "+sfx, "baseball", "L", "", time.Now(), "published")
	did := d.AddDivision(tid, "10U")
	d.AddTeam(tid, did, "A", "")
	d.AddTeam(tid, did, "B", "")
	tm := d.ReturnTeamsByDivisionID(did)
	g1 := d.AddGame(tid, did, tm[0].ID, tm[1].ID, "F1", time.Now(), "")
	u, _ := d.CreateUser("pay"+sfx+"@x.com", "Pay", "h", "", true, false)
	if d.DivisionPayPerGameCents(did) != 0 {
		t.Fatal("default no rate")
	}
	d.SetDivisionPayPerGameCents(did, 7550)
	if d.DivisionPayPerGameCents(did) != 7550 {
		t.Fatal("rate")
	}
	d.AssignUmpire(g1, 1, u.ID, "")
	d.AssignUmpire(g1, 2, 0, "Free F.")
	d.SetUmpireSlotAbsent(g1, 2, true)
	slots := d.UmpireSlotsForTournament(tid)
	if len(slots) != 2 || slots[0].Absent || !slots[1].Absent {
		t.Fatalf("slots: %+v", slots)
	}
	d.AssignUmpire(g1, 2, 0, "Other F.") // reassigning resets absent
	if d.UmpireSlotsForTournament(tid)[1].Absent {
		t.Fatal("reassign must reset absent")
	}
}

func TestScoreReportsPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	var d mydb.DB = mydb.New(url, false)
	sfx := fmt.Sprint(time.Now().UnixNano())
	tid := d.AddTournament("SR "+sfx, "baseball", "L", "", time.Now(), "published")
	did := d.AddDivision(tid, "10U")
	d.AddTeam(tid, did, "A", "")
	d.AddTeam(tid, did, "B", "")
	tm := d.ReturnTeamsByDivisionID(did)
	g := d.AddGame(tid, did, tm[0].ID, tm[1].ID, "F", time.Now(), "")
	if d.ReturnGameByID(g).ScoreStatus != "" {
		t.Fatal("default unscored")
	}
	d.SaveScoreReport(g, "coach_away", 1, 2, 4)
	time.Sleep(5 * time.Millisecond)
	d.SaveScoreReport(g, "coach_home", 2, 4, 2)
	time.Sleep(5 * time.Millisecond)
	d.SaveScoreReport(g, "coach_away", 3, 9, 9)
	got := d.ScoreReportsForGame(g)
	if len(got) != 2 || got[0].Reporter != "coach_away" || got[0].HomeScore != 9 || got[0].UserID != 3 {
		t.Fatalf("order/edit: %+v", got)
	}
	if !got[0].UpdatedAt.After(got[0].ReportedAt) {
		t.Fatal("edit sets UpdatedAt, keeps ReportedAt")
	}
	d.SetGameScoreStatus(g, "disputed")
	if d.ReturnGameByID(g).ScoreStatus != "disputed" {
		t.Fatal("status")
	}
	// migration: an already-scored game with no status becomes final, idempotently
	d.ScoreGame(g, 5, 4)
	db := d.(*mydb.MyDB)
	db.DB.Exec(`UPDATE games SET score_status='' WHERE id=$1`, g)
	mydb.New(url, false)
	if d.ReturnGameByID(g).ScoreStatus != "final" {
		t.Fatalf("scored game should migrate to final, got %q", d.ReturnGameByID(g).ScoreStatus)
	}
	d.DelGame(g)
	if len(d.ScoreReportsForGame(g)) != 0 {
		t.Fatal("cascade on delete")
	}
}
