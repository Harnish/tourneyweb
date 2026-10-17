package webhandler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
)

type roleFx struct {
	*slotFx
	rid      int
	director TWUser
}

func newRoleFx(t *testing.T) *roleFx {
	t.Helper()
	f := newSlotFx(t)
	rid, _ := f.db.AddExtraRole(f.tid, "Scorekeeper")
	f.db.SetDivisionRoleNeeded(f.did, rid, 2)
	return &roleFx{slotFx: f, rid: rid,
		director: TWUser{ID: 1, Roles: []TournamentRole{{TournamentID: f.tid, Role: "director"}}}}
}

func (f *roleFx) key(prefix string, slot int) string {
	return prefix + strconv.Itoa(f.rid) + "_" + strconv.Itoa(slot)
}

// edit POSTs the game edit form (same time and teams) with extra form fields.
func (f *roleFx) edit(datetime string, extra ...string) *httptest.ResponseRecorder {
	tm := f.db.ReturnTeamsByDivisionID(f.did)
	kv := append([]string{"divisionid", strconv.Itoa(f.did), "hometeam", strconv.Itoa(tm[0].ID), "awayteam", strconv.Itoa(tm[1].ID),
		"location", "Field 1", "datetime", datetime}, extra...)
	req := form(kv...)
	req = req.WithContext(ctxWithUser(req.Context(), f.director))
	w := httptest.NewRecorder()
	f.env.ManageEditGame(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}, {Key: "gid", Value: strconv.Itoa(f.game.ID)}})
	return w
}

const sameTime = "2026-07-10T10:00"

func TestParseRoleChoices(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	r := form(f.key("role_user_", 1), "7", f.key("role_name_", 1), "ignored", f.key("role_name_", 2), "Wes K.", f.key("role_user_", 3), "9")
	got := f.env.parseRoleChoices(r, f.did, 0)
	if len(got) != 2 {
		t.Fatalf("slots beyond the count are ignored: %+v", got)
	}
	if got[0] != (roleChoice{RoleID: f.rid, Slot: 1, UserID: 7}) || got[1] != (roleChoice{RoleID: f.rid, Slot: 2, Name: "Wes K."}) {
		t.Fatalf("user wins over name: %+v", got)
	}
}

func TestAssignRoleOnGameForm(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	if w := f.edit(sameTime, f.key("role_user_", 1), strconv.Itoa(f.ann)); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	s := f.db.GameRoleSlots(f.game.ID)
	if len(s) != 1 || s[0].UserID != f.ann || s[0].Status != "pending" {
		t.Fatalf("slots: %+v", s)
	}
	if n := f.db.ListNotifications(f.ann, 10); len(n) != 1 || n[0].Subject != "You have been scheduled for a role" || !strings.Contains(n[0].Message, "Scorekeeper") {
		t.Fatalf("notification: %+v", n)
	}
	if e := f.emails(); len(e) != 1 || e[0] != "ann@x.com|You have been scheduled for a role" {
		t.Fatalf("email: %v", e)
	}
	// resubmitting unchanged must not notify again
	f.edit(sameTime, f.key("role_user_", 1), strconv.Itoa(f.ann))
	if n := f.db.ListNotifications(f.ann, 10); len(n) != 1 {
		t.Fatalf("unchanged resubmit renotified: %+v", n)
	}
	// clearing the slot tells her
	f.edit(sameTime)
	if len(f.db.GameRoleSlots(f.game.ID)) != 0 {
		t.Fatal("blank form clears the slot")
	}
	if n := f.db.ListNotifications(f.ann, 10); len(n) != 2 || n[0].Subject != "Removed from a game" {
		t.Fatalf("removal: %+v", n)
	}
}

func TestRoleFreeFormGetsNoNotice(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	f.edit(sameTime, f.key("role_name_", 1), "Walk-in Wes")
	s := f.db.GameRoleSlots(f.game.ID)
	if len(s) != 1 || s[0].FreeName != "Walk-in Wes" || s[0].Status != "accepted" {
		t.Fatalf("slots: %+v", s)
	}
	if e := f.emails(); len(e) != 0 {
		t.Fatalf("free-form must send nothing: %v", e)
	}
}

func TestRoleConflictRejected(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	tm := f.db.ReturnTeamsByDivisionID(f.did)
	other := f.db.AddGame(f.tid, f.did, tm[0].ID, tm[1].ID, "F2", f.game.Start.Add(30*time.Minute), "")
	f.db.AssignUmpire(other, 1, f.ann, "")
	w := f.edit(sameTime, f.key("role_user_", 1), strconv.Itoa(f.ann))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "Ann") {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	if len(f.db.GameRoleSlots(f.game.ID)) != 0 {
		t.Fatal("rejected edit must write nothing")
	}
}

func TestRoleSameUserTwiceOrAlsoUmpireRejected(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	if w := f.edit(sameTime, f.key("role_user_", 1), strconv.Itoa(f.ann), f.key("role_user_", 2), strconv.Itoa(f.ann)); w.Code != http.StatusConflict {
		t.Fatalf("twice in roles: got %d", w.Code)
	}
	if w := f.edit(sameTime, "ump_user_1", strconv.Itoa(f.ann), f.key("role_user_", 1), strconv.Itoa(f.ann)); w.Code != http.StatusConflict {
		t.Fatalf("umpire and role same game: got %d", w.Code)
	}
	if len(f.db.GameRoleSlots(f.game.ID)) != 0 {
		t.Fatal("nothing written")
	}
}

func TestDraftRefusesRegisteredRoleUser(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	td := f.db.AddTournament("Draft", "baseball", "L", "", time.Now(), "draft")
	dd := f.db.AddDivision(td, "10U")
	rd, _ := f.db.AddExtraRole(td, "Scorekeeper")
	f.db.SetDivisionRoleNeeded(dd, rd, 1)
	f.db.AddTeam(td, dd, "A", "")
	f.db.AddTeam(td, dd, "B", "")
	tm := f.db.ReturnTeamsByDivisionID(dd)
	gid := f.db.AddGame(td, dd, tm[0].ID, tm[1].ID, "F", f.game.Start, "")
	admin := TWUser{ID: 1, IsAdmin: true}
	post := func(kv ...string) *httptest.ResponseRecorder {
		req := form(append([]string{"divisionid", strconv.Itoa(dd), "hometeam", strconv.Itoa(tm[0].ID), "awayteam", strconv.Itoa(tm[1].ID),
			"location", "F", "datetime", sameTime}, kv...)...)
		req = req.WithContext(ctxWithUser(req.Context(), admin))
		w := httptest.NewRecorder()
		f.env.ManageEditGame(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(td)}, {Key: "gid", Value: strconv.Itoa(gid)}})
		return w
	}
	if w := post("role_user_"+strconv.Itoa(rd)+"_1", strconv.Itoa(f.ann)); w.Code != http.StatusConflict {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	if w := post("role_name_"+strconv.Itoa(rd)+"_1", "Wes"); w.Code != http.StatusSeeOther {
		t.Fatalf("free-form allowed in drafts: %d", w.Code)
	}
}

func TestGameChangeResetsRoleSlots(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	f.db.AssignRoleSlot(f.game.ID, f.rid, 1, f.ann, "")
	f.db.RespondRoleSlot(f.game.ID, f.rid, 1, f.ann, true)
	if w := f.edit("2026-07-10T14:00", f.key("role_user_", 1), strconv.Itoa(f.ann)); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	if s := f.db.GameRoleSlots(f.game.ID); len(s) != 1 || s[0].Status != "pending" {
		t.Fatalf("time change must reset to pending: %+v", s)
	}
	if n := f.db.ListNotifications(f.ann, 10); len(n) != 1 || n[0].Subject != "Game changed — please re-confirm" {
		t.Fatalf("notification: %+v", n)
	}
}

func TestDeleteGameNotifiesRoleAssignee(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	f.db.AssignRoleSlot(f.game.ID, f.rid, 1, f.ann, "")
	req := form()
	req = req.WithContext(ctxWithUser(req.Context(), f.director))
	f.env.ManageDeleteGame(httptest.NewRecorder(), req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}, {Key: "gid", Value: strconv.Itoa(f.game.ID)}})
	if n := f.db.ListNotifications(f.ann, 10); len(n) != 1 || n[0].Subject != "Game cancelled" {
		t.Fatalf("notification: %+v", n)
	}
}

func TestGameFormRendersRoleSlotsEscaped(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	f.db.AssignRoleSlot(f.game.ID, f.rid, 1, 0, "<b>Evil</b>")
	req := httptest.NewRequest("GET", "/x", nil)
	req = req.WithContext(ctxWithUser(req.Context(), f.director))
	w := httptest.NewRecorder()
	f.env.ManageEditGame(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}, {Key: "gid", Value: strconv.Itoa(f.game.ID)}})
	body := w.Body.String()
	if !strings.Contains(body, "Scorekeeper") || strings.Contains(body, "<b>Evil</b>") || !strings.Contains(body, `name="`+f.key("role_name_", 2)+`"`) {
		t.Fatalf("role slots not rendered safely: %s", body)
	}
}

func TestUserSearch(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	f.db.CreateUser("evil@x.com", `<script>x</script> Evil`, "h", "", true, false)
	call := func(user TWUser, q string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/x?q="+q, nil)
		req = req.WithContext(ctxWithUser(req.Context(), user))
		w := httptest.NewRecorder()
		f.env.UserSearch(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}})
		return w
	}
	w := call(f.director, "an")
	var got []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got) != 1 || got[0]["name"] != "Ann" {
		t.Fatalf("search: %v %s", err, w.Body.String())
	}
	if _, hasEmail := got[0]["email"]; hasEmail || strings.Contains(w.Body.String(), "@") {
		t.Fatalf("email must not be returned: %s", w.Body.String())
	}
	if w := call(f.director, "evil"); strings.Contains(w.Body.String(), "<script>") {
		t.Fatalf("json must escape html: %s", w.Body.String())
	}
	if w := call(f.director, "a"); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("short query: %s", w.Body.String())
	}
	if w := call(TWUser{ID: f.ann}, "an"); w.Code != http.StatusForbidden {
		t.Fatalf("non-staff: got %d", w.Code)
	}
	other := f.db.AddTournament("T2", "baseball", "L", "", time.Now(), "published")
	if w := call(TWUser{ID: 5, Roles: []TournamentRole{{TournamentID: other, Role: "director"}}}, "an"); w.Code != http.StatusForbidden {
		t.Fatalf("other tournament's director: got %d", w.Code)
	}
	for i := 0; i < 15; i++ {
		f.db.CreateUser("zed"+strconv.Itoa(i)+"@x.com", "Zed "+strconv.Itoa(i), "h", "", true, false)
	}
	_ = json.Unmarshal(call(f.director, "zed").Body.Bytes(), &got)
	if len(got) != 10 {
		t.Fatalf("limit 10, got %d", len(got))
	}
}

func TestMoveRoleHolderToUmpireSlotSameGame(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	f.db.AssignRoleSlot(f.game.ID, f.rid, 1, f.ann, "")
	if w := f.edit(sameTime, "ump_user_1", strconv.Itoa(f.ann)); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	if len(f.db.GameRoleSlots(f.game.ID)) != 0 {
		t.Fatal("role slot must be cleared")
	}
	if u := f.db.GameUmpires(f.game.ID); len(u) != 1 || u[0].UserID != f.ann {
		t.Fatalf("umpire slot: %+v", u)
	}
}

func TestUserSearchRefusedForDraftTournament(t *testing.T) {
	f := newRoleFx(t)
	defer f.restore()
	td := f.db.AddTournament("Draft", "baseball", "L", "", time.Now(), "draft")
	req := httptest.NewRequest("GET", "/x?q=an", nil)
	req = req.WithContext(ctxWithUser(req.Context(), TWUser{ID: 9, IsAdmin: true}))
	w := httptest.NewRecorder()
	f.env.UserSearch(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(td)}})
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d want 403", w.Code)
	}
}
