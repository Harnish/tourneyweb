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

func respond(f *slotFx, user TWUser, gid, slot int, action string) *httptest.ResponseRecorder {
	req := form("action", action)
	req = req.WithContext(ctxWithUser(req.Context(), user))
	w := httptest.NewRecorder()
	f.env.UmpireRespond(w, req, httprouter.Params{{Key: "gid", Value: strconv.Itoa(gid)}, {Key: "slot", Value: strconv.Itoa(slot)}})
	return w
}

func TestRespondWrongUser403(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	if w := respond(f, TWUser{ID: f.bob}, f.game.ID, 1, "accept"); w.Code != http.StatusForbidden {
		t.Fatalf("got %d want 403", w.Code)
	}
	if f.db.GameUmpires(f.game.ID)[0].Status != "pending" {
		t.Fatal("state must not change")
	}
}

func TestAcceptSetsAccepted(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	if w := respond(f, TWUser{ID: f.ann}, f.game.ID, 1, "accept"); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d", w.Code)
	}
	if f.db.GameUmpires(f.game.ID)[0].Status != "accepted" {
		t.Fatal("want accepted")
	}
}

func TestRespondBadAction(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	if w := respond(f, TWUser{ID: f.ann}, f.game.ID, 1, "maybe"); w.Code != http.StatusBadRequest {
		t.Fatalf("got %d want 400", w.Code)
	}
}

func TestDeclineReopensAndDirectorsNotified(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	d1, _ := f.db.CreateUser("dir1@x.com", "Dir1", "h", "", true, false)
	st, _ := f.db.CreateUser("staff@x.com", "Staff", "h", "", true, false)
	ad, _ := f.db.CreateUser("admin@x.com", "Admin", "h", "", true, true)
	f.db.AssignRole(d1.ID, f.tid, "director", 0)
	f.db.AssignRole(st.ID, f.tid, "staff", 0)
	_ = ad
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.db.AssignUmpire(f.game.ID, 2, 0, "Free")
	if w := respond(f, TWUser{ID: f.ann}, f.game.ID, 1, "decline"); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d", w.Code)
	}
	if got := f.db.ReturnGameByID(f.game.ID).Umpire; got != "Free" {
		t.Fatalf("declined name must drop from public display, got %q", got)
	}
	e := f.emails()
	if len(e) != 1 || !strings.HasPrefix(e[0], "dir1@x.com|") {
		t.Fatalf("only the tournament's directors are emailed: %v", e)
	}
}

func TestUmpireGamesGroupsAndHidesDeclined(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	tm := f.db.ReturnTeamsByDivisionID(f.did)
	future := time.Now().Add(48 * time.Hour)
	g1 := f.db.AddGame(f.tid, f.did, tm[0].ID, tm[1].ID, "Field 7", future, "")
	past := f.db.AddGame(f.tid, f.did, tm[0].ID, tm[1].ID, "Field 8", time.Now().Add(-72*time.Hour), "")
	declined := f.db.AddGame(f.tid, f.did, tm[0].ID, tm[1].ID, "Field 9", future.Add(24*time.Hour), "")
	f.db.AssignUmpire(g1, 1, f.ann, "")
	f.db.AssignUmpire(g1, 2, 0, "Partner P.")
	f.db.AssignUmpire(past, 1, f.ann, "")
	f.db.AssignUmpire(declined, 1, f.ann, "")
	f.db.RespondUmpireSlot(declined, 1, f.ann, false)

	req := httptest.NewRequest("GET", "/umpire", nil)
	req = req.WithContext(ctxWithUser(req.Context(), TWUser{ID: f.ann, Name: "Ann"}))
	w := httptest.NewRecorder()
	f.env.UmpireGames(w, req, nil)
	body := w.Body.String()
	if w.Code != 200 {
		t.Fatalf("code %d", w.Code)
	}
	for _, want := range []string{"Field 7", "Partner P.", "pending", "Accept", "Decline", "Field 8"} {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(body, "Field 9") {
		t.Error("declined games must be hidden")
	}
	if strings.Index(body, "Field 7") > strings.Index(body, "Field 8") {
		t.Error("upcoming games come before past games")
	}
}

func TestUmpireGamesRequiresLogin(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	req := httptest.NewRequest("GET", "/umpire", nil)
	w := httptest.NewRecorder()
	f.env.UmpireGames(w, req, nil)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("got %d want 303", w.Code)
	}
}

func TestSecondDeclineDoesNotReEmailDirectors(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	d1, _ := f.db.CreateUser("dir1@x.com", "Dir1", "h", "", true, false)
	f.db.AssignRole(d1.ID, f.tid, "director", 0)
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	respond(f, TWUser{ID: f.ann}, f.game.ID, 1, "decline")
	if w := respond(f, TWUser{ID: f.ann}, f.game.ID, 1, "decline"); w.Code != http.StatusConflict {
		t.Fatalf("repeat decline: got %d want 409", w.Code)
	}
	if w := respond(f, TWUser{ID: f.ann}, f.game.ID, 1, "accept"); w.Code != http.StatusConflict {
		t.Fatalf("accept after decline: got %d want 409", w.Code)
	}
	if e := f.emails(); len(e) != 1 {
		t.Fatalf("directors must be emailed once, got %v", e)
	}
}

func inboxSubjects(f *slotFx, userID int) []string {
	var out []string
	for _, n := range f.db.ListNotifications(userID, 50) {
		out = append(out, n.Subject+"|"+n.Link)
	}
	return out
}

func TestDirectorRecipientsFallsBackToAdmins(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	admin, _ := f.db.CreateUser("admin@x.com", "Admin", "h", "", true, true)
	staff, _ := f.db.CreateUser("staff@x.com", "Staff", "h", "", true, false)
	f.db.AssignRole(staff.ID, f.tid, "staff", 0)
	if got := f.env.directorRecipients(f.tid); len(got) != 1 || got[0] != admin.ID {
		t.Fatalf("no director: want admins only, got %v", got)
	}
	dir, _ := f.db.CreateUser("dir@x.com", "Dir", "h", "", true, false)
	f.db.AssignRole(dir.ID, f.tid, "director", 0)
	if got := f.env.directorRecipients(f.tid); len(got) != 1 || got[0] != dir.ID {
		t.Fatalf("with a director: want directors only, got %v", got)
	}
}

func TestDeclineNotifiesDirectorsInboxWithEditLink(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	dir, _ := f.db.CreateUser("dir@x.com", "Dir", "h", "", true, false)
	f.db.AssignRole(dir.ID, f.tid, "director", 0)
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	respond(f, TWUser{ID: f.ann, Name: "Ann"}, f.game.ID, 1, "decline")
	want := "Umpire declined a game|/tournaments/" + strconv.Itoa(f.tid) + "/manage/games/" + strconv.Itoa(f.game.ID) + "/edit"
	if got := inboxSubjects(f, dir.ID); len(got) != 1 || got[0] != want {
		t.Fatalf("got %v want [%s]", got, want)
	}
}

func TestDeclineWithNoDirectorReachesAdmins(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	admin, _ := f.db.CreateUser("admin@x.com", "Admin", "h", "", true, true)
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	respond(f, TWUser{ID: f.ann, Name: "Ann"}, f.game.ID, 1, "decline")
	if len(inboxSubjects(f, admin.ID)) != 1 {
		t.Fatal("admin fallback must be notified")
	}
}

func TestAssignedRemovedChangedCancelledGetInboxRows(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.env.applySlots(f.game, []slotChoice{{1, f.ann, ""}})
	if got := inboxSubjects(f, f.ann); len(got) != 1 || got[0] != "You have been scheduled to umpire|/umpire" {
		t.Fatalf("assigned: %v", got)
	}
	f.db.RespondUmpireSlot(f.game.ID, 1, f.ann, true)
	moved := f.game
	moved.Start = moved.Start.Add(3 * time.Hour)
	f.env.notifyGameChanged(f.game, moved)
	f.env.applySlots(f.game, []slotChoice{{1, 0, ""}})
	f.env.applySlots(f.game, []slotChoice{{1, f.bob, ""}})
	f.env.notifyGameDeleted(f.game)
	all := strings.Join(inboxSubjects(f, f.ann), ";")
	for _, want := range []string{"Game changed — please re-confirm|/umpire", "Removed from a game|/umpire"} {
		if !strings.Contains(all, want) {
			t.Errorf("Ann is missing %q in %s", want, all)
		}
	}
	if b := strings.Join(inboxSubjects(f, f.bob), ";"); !strings.Contains(b, "Game cancelled|/umpire") {
		t.Errorf("Bob should be told the game was cancelled: %s", b)
	}
}

func TestFreeFormUmpireNotNotified(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.env.applySlots(f.game, []slotChoice{{1, 0, "Free Ump"}})
	f.env.applySlots(f.game, []slotChoice{{1, 0, ""}})
	f.env.notifyGameDeleted(f.game)
	for _, id := range []int{f.ann, f.bob} {
		if n := f.db.CountUnreadNotifications(id); n != 0 {
			t.Fatalf("user %d got %d notifications", id, n)
		}
	}
	if e := f.emails(); len(e) != 0 {
		t.Fatalf("no emails: %v", e)
	}
}
