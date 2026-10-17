package webhandler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
)

func roleParams(tid int, rid int) httprouter.Params {
	ps := httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}}
	if rid != 0 {
		ps = append(ps, httprouter.Param{Key: "rid", Value: strconv.Itoa(rid)})
	}
	return ps
}

func TestCreateExtraRole(t *testing.T) {
	env, db, tid, _, _, dir := umpFixture(t)
	post := func(name string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		env.ManageExtraRoleCreate(w, directorReq("POST", url.Values{"name": {name}}.Encode(), dir), roleParams(tid, 0))
		return w
	}
	if w := post("Scorekeeper"); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	if w := post(" scorekeeper "); w.Code != http.StatusConflict {
		t.Fatalf("duplicate: got %d", w.Code)
	}
	if w := post(""); w.Code != http.StatusBadRequest {
		t.Fatalf("empty: got %d", w.Code)
	}
	if got := db.TournamentExtraRoles(tid); len(got) != 1 {
		t.Fatalf("roles: %+v", got)
	}
	w := httptest.NewRecorder()
	env.ManageExtraRoles(w, directorReq("GET", "", dir), roleParams(tid, 0))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Scorekeeper") {
		t.Fatalf("page: %d %s", w.Code, w.Body.String())
	}
}

func TestExtraRoleOtherTournament404(t *testing.T) {
	env, db, tid, _, _, dir := umpFixture(t)
	tid2 := db.AddTournament("T2", "baseball", "L", "", time.Now(), "published")
	rid, _ := db.AddExtraRole(tid2, "Safety")
	for name, h := range map[string]func(http.ResponseWriter, *http.Request, httprouter.Params){
		"rename": env.ManageExtraRoleRename, "delete": env.ManageExtraRoleDelete} {
		w := httptest.NewRecorder()
		h(w, directorReq("POST", url.Values{"name": {"x"}}.Encode(), dir), roleParams(tid, rid))
		if w.Code != http.StatusNotFound {
			t.Errorf("%s: got %d want 404", name, w.Code)
		}
	}
	if db.ExtraRoleByID(rid).Name != "Safety" {
		t.Fatal("other tournament's role must be untouched")
	}
}

func TestDeleteExtraRoleNotifiesAssignees(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	rid, _ := f.db.AddExtraRole(f.tid, "Scorekeeper")
	f.db.AssignRoleSlot(f.game.ID, rid, 1, f.ann, "")
	f.db.AssignRoleSlot(f.game.ID, rid, 2, 0, "Walk-in")
	f.db.AssignRoleSlot(f.game.ID, rid, 3, f.bob, "")
	f.db.RespondRoleSlot(f.game.ID, rid, 3, f.bob, false) // declined: not told
	dir := TWUser{ID: 1, Roles: []TournamentRole{{TournamentID: f.tid, Role: "director"}}}
	w := httptest.NewRecorder()
	f.env.ManageExtraRoleDelete(w, directorReq("POST", "", dir), roleParams(f.tid, rid))
	if w.Code != http.StatusSeeOther {
		t.Fatalf("got %d", w.Code)
	}
	if len(f.db.GameRoleSlots(f.game.ID)) != 0 || f.db.ExtraRoleByID(rid).ID != 0 {
		t.Fatal("role and slots must be gone")
	}
	if n := f.db.ListNotifications(f.ann, 10); len(n) != 1 || n[0].Subject != "Role removed" {
		t.Fatalf("ann: %+v", n)
	}
	if n := f.db.ListNotifications(f.bob, 10); len(n) != 0 {
		t.Fatalf("declined assignee must not be told: %+v", n)
	}
}

func TestDivisionEditSetsRoleNeeds(t *testing.T) {
	env, db, tid, did, _, dir := umpFixture(t)
	rid, _ := db.AddExtraRole(tid, "Scorekeeper")
	tid2 := db.AddTournament("T2", "baseball", "L", "", time.Now(), "published")
	foreign, _ := db.AddExtraRole(tid2, "Foreign")
	ps := httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}, {Key: "did", Value: strconv.Itoa(did)}}
	post := func(kv ...string) {
		v := url.Values{"name": {"10U"}}
		for i := 0; i < len(kv); i += 2 {
			v.Set(kv[i], kv[i+1])
		}
		w := httptest.NewRecorder()
		env.ManageDivisionEdit(w, directorReq("POST", v.Encode(), dir), ps)
		if w.Code != http.StatusSeeOther {
			t.Fatalf("got %d: %s", w.Code, w.Body.String())
		}
	}
	post("role_need_"+strconv.Itoa(rid), "3", "role_need_"+strconv.Itoa(foreign), "2")
	if got := db.DivisionRoleNeeds(did); got[rid] != 3 || len(got) != 1 {
		t.Fatalf("needs: %v", got)
	}
	post("role_need_"+strconv.Itoa(rid), "9")
	if db.DivisionRoleNeeds(did)[rid] != 5 {
		t.Fatal("clamp to 5")
	}
	post() // field absent: unchanged
	if db.DivisionRoleNeeds(did)[rid] != 5 {
		t.Fatal("absent field must not reset")
	}
	w := httptest.NewRecorder()
	env.ManageDivisionEdit(w, directorReq("GET", "", dir), ps)
	if !strings.Contains(w.Body.String(), `name="role_need_`+strconv.Itoa(rid)+`"`) || strings.Contains(w.Body.String(), "Foreign") {
		t.Fatalf("edit form lists this tournament's roles only: %s", w.Body.String())
	}
}
