package webhandler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
)

func availPost(f *slotFx, user TWUser, tid int, note string) *httptest.ResponseRecorder {
	req := form("note", note)
	req = req.WithContext(ctxWithUser(req.Context(), user))
	w := httptest.NewRecorder()
	f.env.UmpireAvailable(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}})
	return w
}

func withdrawPost(f *slotFx, user TWUser, tid int) *httptest.ResponseRecorder {
	req := form()
	req = req.WithContext(ctxWithUser(req.Context(), user))
	w := httptest.NewRecorder()
	f.env.UmpireWithdraw(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}})
	return w
}

func addDirector(f *slotFx, email string) int {
	d, _ := f.db.CreateUser(email, "Dir", "h", "", true, false)
	f.db.AssignRole(d.ID, f.tid, "director", 0)
	return d.ID
}

func TestUmpireAvailableNotifiesDirectorsOnce(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	dir := addDirector(f, "dir@x.com")
	ann := TWUser{ID: f.ann, Name: "Ann"}
	if w := availPost(f, ann, f.tid, "weekends only"); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	want := "Umpire available|/tournaments/" + strconv.Itoa(f.tid) + "/manage/umpires"
	if got := inboxSubjects(f, dir); len(got) != 1 || got[0] != want {
		t.Fatalf("got %v want [%s]", got, want)
	}
	if e := f.emails(); len(e) != 1 || !strings.HasPrefix(e[0], "dir@x.com|Umpire available") {
		t.Fatalf("emails %v", e)
	}
	// editing only the note must not notify again
	availPost(f, ann, f.tid, "weekends and Fridays")
	if got := inboxSubjects(f, dir); len(got) != 1 {
		t.Fatalf("note edit re-notified: %v", got)
	}
	if a := f.db.UmpireAvailabilityForUser(f.ann)[f.tid]; a.Note != "weekends and Fridays" {
		t.Fatalf("note not updated: %+v", a)
	}
	// withdraw then volunteer again: new volunteer, notifies again
	withdrawPost(f, ann, f.tid)
	availPost(f, ann, f.tid, "")
	if got := inboxSubjects(f, dir); len(got) != 2 {
		t.Fatalf("re-volunteering after a withdrawal should notify: %v", got)
	}
}

func TestAvailableFallsBackToAdminsWhenNoDirector(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	admin, _ := f.db.CreateUser("admin@x.com", "Admin", "h", "", true, true)
	availPost(f, TWUser{ID: f.ann, Name: "Ann"}, f.tid, "")
	if len(inboxSubjects(f, admin.ID)) != 1 {
		t.Fatal("admin fallback must be told about the volunteer")
	}
}

func TestAvailableRejectsDraftPastAndNonUmpire(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	draft := f.db.AddTournament("Draft", "baseball", "L", "", time.Now().Add(48*time.Hour), "draft")
	old := f.db.AddTournament("Old", "baseball", "L", "", time.Now().Add(-30*24*time.Hour), "published")
	plain, _ := f.db.CreateUser("plain@x.com", "Plain", "h", "", true, false)
	if w := availPost(f, TWUser{ID: f.ann}, draft, ""); w.Code != http.StatusNotFound {
		t.Errorf("draft: got %d want 404", w.Code)
	}
	if w := availPost(f, TWUser{ID: f.ann}, old, ""); w.Code != http.StatusNotFound {
		t.Errorf("long-past tournament: got %d want 404", w.Code)
	}
	if w := availPost(f, TWUser{ID: f.ann}, 99999, ""); w.Code != http.StatusNotFound {
		t.Errorf("unknown tournament: got %d want 404", w.Code)
	}
	if w := availPost(f, TWUser{ID: plain.ID}, f.tid, ""); w.Code != http.StatusForbidden {
		t.Errorf("non-umpire: got %d want 403", w.Code)
	}
	if w := availPost(f, TWUser{}, f.tid, ""); w.Code != http.StatusSeeOther {
		t.Errorf("anonymous: got %d want 303 to login", w.Code)
	}
	if len(f.db.UmpireAvailabilityForUser(f.ann))+len(f.db.UmpireAvailabilityForUser(plain.ID)) != 0 {
		t.Error("nothing may be stored by rejected requests")
	}
}

func TestAvailableNoteTooLong400(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	if w := availPost(f, TWUser{ID: f.ann}, f.tid, strings.Repeat("x", 201)); w.Code != http.StatusBadRequest {
		t.Fatalf("got %d want 400", w.Code)
	}
	if w := availPost(f, TWUser{ID: f.ann}, f.tid, strings.Repeat("x", 200)); w.Code != http.StatusSeeOther {
		t.Fatalf("200 chars is allowed, got %d", w.Code)
	}
}

func manageUmpPage(f *slotFx, dirID int) string {
	dir := TWUser{ID: dirID, Roles: []TournamentRole{{TournamentID: f.tid, Role: "director"}}}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req = req.WithContext(ctxWithUser(req.Context(), dir))
	f.env.ManageUmpires(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}})
	return w.Body.String()
}

func TestManageUmpiresShowsVolunteersFirstWithEscapedNote(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	dir := addDirector(f, "dir@x.com")
	availPost(f, TWUser{ID: f.bob, Name: "Bob"}, f.tid, "<b>x</b> can do 2 fields")
	body := manageUmpPage(f, dir)
	if strings.Index(body, "Bob") > strings.Index(body, "Ann") {
		t.Fatal("volunteers sort first (Bob volunteered, Ann didn't)")
	}
	if !strings.Contains(body, "volunteered") || !strings.Contains(body, "&lt;b&gt;x&lt;/b&gt; can do 2 fields") || strings.Contains(body, "<b>x</b>") {
		t.Fatalf("volunteered badge with escaped note expected: %s", body)
	}
}

func TestWithdrawKeepsAssignmentsAndSendsNothing(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	dir := addDirector(f, "dir@x.com")
	ann := TWUser{ID: f.ann, Name: "Ann"}
	availPost(f, ann, f.tid, "")
	f.emails()
	before := len(inboxSubjects(f, dir))
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	withdrawPost(f, ann, f.tid)
	if len(f.db.UmpireAvailabilityForUser(f.ann)) != 0 {
		t.Fatal("availability should be cleared")
	}
	if s := f.db.GameUmpires(f.game.ID); len(s) != 1 || s[0].UserID != f.ann {
		t.Fatal("assignments must be untouched")
	}
	if len(inboxSubjects(f, dir)) != before {
		t.Fatal("withdrawing must not notify directors")
	}
}

func umpPage(f *slotFx, user TWUser) string {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/umpire", nil)
	req = req.WithContext(ctxWithUser(req.Context(), user))
	f.env.UmpireGames(w, req, nil)
	return w.Body.String()
}

func TestUmpirePageListsUpcomingNotDrafts(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.db.AddTournament("Secret Draft Cup", "baseball", "L", "", time.Now().Add(48*time.Hour), "draft")
	f.db.AddTournament("Autumn Classic", "baseball", "Riverside", "", time.Now().Add(72*time.Hour), "published")
	body := umpPage(f, TWUser{ID: f.ann, Name: "Ann"})
	if !strings.Contains(body, "Autumn Classic") || strings.Contains(body, "Secret Draft Cup") {
		t.Fatalf("upcoming list wrong: %s", body)
	}
	if !strings.Contains(body, "available") {
		t.Fatal("expected an Available action")
	}
	plain, _ := f.db.CreateUser("plain@x.com", "Plain", "h", "", true, false)
	if strings.Contains(umpPage(f, TWUser{ID: plain.ID}), "Autumn Classic") {
		t.Fatal("non-umpires don't get the upcoming tournaments section")
	}
	availPost(f, TWUser{ID: f.ann, Name: "Ann"}, f.tid, "")
	if !strings.Contains(umpPage(f, TWUser{ID: f.ann, Name: "Ann"}), "Withdraw") {
		t.Fatal("a tournament already volunteered for offers Withdraw")
	}
}

