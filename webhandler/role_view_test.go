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

func roleRespond(f *roleFx, user TWUser, gid, slot int, action string) *httptest.ResponseRecorder {
	req := form("action", action)
	req = req.WithContext(ctxWithUser(req.Context(), user))
	w := httptest.NewRecorder()
	f.env.RoleRespond(w, req, httprouter.Params{{Key: "gid", Value: strconv.Itoa(gid)}, {Key: "rid", Value: strconv.Itoa(f.rid)}, {Key: "slot", Value: strconv.Itoa(slot)}})
	return w
}

func TestRoleRespondAccept(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	f.db.AssignRoleSlot(f.game.ID, f.rid, 1, f.ann, "")
	if w := roleRespond(f, TWUser{ID: f.ann}, f.game.ID, 1, "accept"); w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/umpire" {
		t.Fatalf("got %d %s", w.Code, w.Header().Get("Location"))
	}
	if f.db.GameRoleSlots(f.game.ID)[0].Status != "accepted" {
		t.Fatal("want accepted")
	}
}

func TestRoleRespondOtherUser403(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	f.db.AssignRoleSlot(f.game.ID, f.rid, 1, f.ann, "")
	if w := roleRespond(f, TWUser{ID: f.bob}, f.game.ID, 1, "accept"); w.Code != http.StatusForbidden {
		t.Fatalf("got %d want 403", w.Code)
	}
	if f.db.GameRoleSlots(f.game.ID)[0].Status != "pending" {
		t.Fatal("state must not change")
	}
	if w := roleRespond(f, TWUser{}, f.game.ID, 1, "accept"); w.Code != http.StatusSeeOther {
		t.Fatalf("anonymous goes to login, got %d", w.Code)
	}
	if w := roleRespond(f, TWUser{ID: f.ann}, f.game.ID, 1, "maybe"); w.Code != http.StatusBadRequest {
		t.Fatalf("bad action: got %d", w.Code)
	}
}

func TestRoleDeclineNotifiesDirectorsOnceAndSecondIs409(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	dir := addDirector(f.slotFx, "dir@x.com")
	f.db.AssignRoleSlot(f.game.ID, f.rid, 1, f.ann, "")
	if w := roleRespond(f, TWUser{ID: f.ann, Name: "Ann"}, f.game.ID, 1, "decline"); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d", w.Code)
	}
	n := f.db.ListNotifications(dir, 10)
	want := "/tournaments/" + strconv.Itoa(f.tid) + "/manage/games/" + strconv.Itoa(f.game.ID) + "/edit"
	if len(n) != 1 || n[0].Subject != "Role declined" || n[0].Link != want || !strings.Contains(n[0].Message, "Scorekeeper") {
		t.Fatalf("director notice: %+v", n)
	}
	if w := roleRespond(f, TWUser{ID: f.ann, Name: "Ann"}, f.game.ID, 1, "accept"); w.Code != http.StatusConflict {
		t.Fatalf("already declined: got %d want 409", w.Code)
	}
	if len(f.db.ListNotifications(dir, 10)) != 1 {
		t.Fatal("no second notice")
	}
}

func TestDashboardMixesUmpireAndRoles(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	tm := f.db.ReturnTeamsByDivisionID(f.did)
	soon := time.Now().Add(24 * time.Hour)
	later := time.Now().Add(72 * time.Hour)
	gRole := f.db.AddGame(f.tid, f.did, tm[0].ID, tm[1].ID, "Field Role", soon, "")
	gUmp := f.db.AddGame(f.tid, f.did, tm[0].ID, tm[1].ID, "Field Ump", later, "")
	gDec := f.db.AddGame(f.tid, f.did, tm[0].ID, tm[1].ID, "Field Declined", later.Add(24*time.Hour), "")
	f.db.AssignRoleSlot(gRole, f.rid, 1, f.ann, "")
	f.db.AssignUmpire(gUmp, 1, f.ann, "")
	f.db.AssignRoleSlot(gDec, f.rid, 1, f.ann, "")
	f.db.RespondRoleSlot(gDec, f.rid, 1, f.ann, false)

	get := func(user TWUser) (*httptest.ResponseRecorder, string) {
		req := httptest.NewRequest("GET", "/umpire", nil)
		req = req.WithContext(ctxWithUser(req.Context(), user))
		w := httptest.NewRecorder()
		f.env.UmpireGames(w, req, nil)
		return w, w.Body.String()
	}
	w, body := get(TWUser{ID: f.ann, Name: "Ann"})
	if w.Code != 200 {
		t.Fatalf("code %d", w.Code)
	}
	if !strings.Contains(body, "Scorekeeper") || !strings.Contains(body, "Field Role") || !strings.Contains(body, "Field Ump") {
		t.Fatalf("both kinds must show: %s", body)
	}
	if strings.Contains(body, "Field Declined") {
		t.Error("declined role slots are hidden")
	}
	if strings.Index(body, "Field Role") > strings.Index(body, "Field Ump") {
		t.Error("sorted by time across both kinds")
	}
	if !strings.Contains(body, "/umpire/games/"+strconv.Itoa(gRole)+"/roles/"+strconv.Itoa(f.rid)+"/slots/1") {
		t.Error("role rows post to the role respond route")
	}
	// a user who is not an umpire and only holds a role slot still gets the page, without the umpire-only section
	f.db.SetUmpireProfile(f.bob, false, "")
	f.db.AssignRoleSlot(gRole, f.rid, 2, f.bob, "")
	w, body = get(TWUser{ID: f.bob, Name: "Bob"})
	if w.Code != 200 || !strings.Contains(body, "Field Role") || strings.Contains(body, "Upcoming Tournaments") {
		t.Fatalf("role-only user: %d %s", w.Code, body)
	}
}
