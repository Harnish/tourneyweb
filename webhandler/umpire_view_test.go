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
