package webhandler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

type slotFx struct {
	env     *Env
	db      *mydb.FakeDB
	tid     int
	did     int
	game    mydb.Game
	ann     int
	bob     int
	sent    *[]string // "to|subject"
	sentMu  *sync.Mutex
	restore func()
}

func newSlotFx(t *testing.T) *slotFx {
	t.Helper()
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T1", "baseball", "L", "", time.Now(), "published")
	did := db.AddDivision(tid, "10U")
	db.SetDivisionUmpiresNeeded(did, 2)
	db.AddTeam(tid, did, "A", "")
	db.AddTeam(tid, did, "B", "")
	tm := db.ReturnTeamsByDivisionID(did)
	start := time.Date(2026, 7, 10, 10, 0, 0, 0, time.Local)
	gid := db.AddGame(tid, did, tm[0].ID, tm[1].ID, "Field 1", start, "")
	ann, _ := db.CreateUser("ann@x.com", "Ann", "h", "", true, false)
	bob, _ := db.CreateUser("bob@x.com", "Bob", "h", "", true, false)
	db.SetUmpireProfile(ann.ID, true, "")
	db.SetUmpireProfile(bob.ID, true, "")

	var mu sync.Mutex
	var sent []string
	old := sendNotice
	sendNotice = func(me *Env, to, subject, msg string) error {
		mu.Lock()
		sent = append(sent, to+"|"+subject)
		mu.Unlock()
		return nil
	}
	return &slotFx{env: &Env{DB: db}, db: db, tid: tid, did: did, game: db.ReturnGameByID(gid),
		ann: ann.ID, bob: bob.ID, sent: &sent, sentMu: &mu, restore: func() { sendNotice = old }}
}

func (f *slotFx) emails() []string {
	notifyWG.Wait()
	f.sentMu.Lock()
	defer f.sentMu.Unlock()
	return append([]string(nil), *f.sent...)
}

func form(kv ...string) *http.Request {
	v := url.Values{}
	for i := 0; i < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	req := httptest.NewRequest("POST", "/x", strings.NewReader(v.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestParseSlotChoices(t *testing.T) {
	r := form("ump_user_1", "7", "ump_name_1", "ignored", "ump_name_2", "Bob K.", "ump_user_3", "9")
	got := parseSlotChoices(r, 2)
	if len(got) != 2 {
		t.Fatalf("N beyond needed ignored; got %+v", got)
	}
	if got[0] != (slotChoice{Slot: 1, UserID: 7}) {
		t.Errorf("user wins over name: %+v", got[0])
	}
	if got[1] != (slotChoice{Slot: 2, Name: "Bob K."}) {
		t.Errorf("free name: %+v", got[1])
	}
	if got := parseSlotChoices(form(), 2); len(got) != 2 || got[0].UserID != 0 || got[0].Name != "" {
		t.Errorf("blank slots are returned as blank choices: %+v", got)
	}
}

func TestValidateSlots(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	g := f.game
	if msg := f.env.validateSlots([]slotChoice{{1, f.ann, ""}, {2, f.ann, ""}}, g.Start, g.ID); msg == "" {
		t.Error("same umpire twice must be rejected")
	}
	if msg := f.env.validateSlots([]slotChoice{{1, 99999, ""}}, g.Start, g.ID); msg == "" {
		t.Error("non-umpire user id must be rejected")
	}
	// overlapping game elsewhere
	tm := f.db.ReturnTeamsByDivisionID(f.did)
	other := f.db.AddGame(f.tid, f.did, tm[0].ID, tm[1].ID, "F2", g.Start.Add(time.Hour), "")
	if err := f.db.AssignUmpire(other, 1, f.ann, ""); err != nil {
		t.Fatal(err)
	}
	if msg := f.env.validateSlots([]slotChoice{{1, f.ann, ""}}, g.Start, g.ID); msg == "" || !strings.Contains(msg, "Ann") {
		t.Errorf("conflict message should name the umpire, got %q", msg)
	}
	if msg := f.env.validateSlots([]slotChoice{{1, f.bob, ""}, {2, 0, "Free Ump"}}, g.Start, g.ID); msg != "" {
		t.Errorf("valid choices rejected: %q", msg)
	}
}

func TestValidateSlotsKeepsUnflaggedHolder(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.db.SetUmpireProfile(f.ann, false, "")
	if msg := f.env.validateSlots([]slotChoice{{1, f.ann, ""}}, f.game.Start, f.game.ID); msg != "" {
		t.Errorf("existing holder who un-flagged must be kept, got %q", msg)
	}
}

func TestApplySlotsAssignAndRemove(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.env.applySlots(f.game, []slotChoice{{1, f.ann, ""}, {2, 0, "Bob K."}})
	if got := f.db.ReturnGameByID(f.game.ID).Umpire; got != "Ann, Bob K." {
		t.Fatalf("cache %q", got)
	}
	if e := f.emails(); len(e) != 1 || !strings.HasPrefix(e[0], "ann@x.com|") {
		t.Fatalf("only the registered umpire is emailed: %v", e)
	}
	// replace Ann with Bob; Ann is told she was removed, Bob assigned
	f.env.applySlots(f.game, []slotChoice{{1, f.bob, ""}, {2, 0, "Bob K."}})
	e := f.emails()
	if len(e) != 3 {
		t.Fatalf("want assigned+removed emails, got %v", e)
	}
	// blank clears
	f.env.applySlots(f.game, []slotChoice{{1, 0, ""}, {2, 0, ""}})
	if f.db.ReturnGameByID(f.game.ID).Umpire != "" || len(f.db.GameUmpires(f.game.ID)) != 0 {
		t.Fatal("blank choices should clear")
	}
}

func TestApplySlotsUnchangedSendsNothingAndKeepsDeclined(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.env.applySlots(f.game, []slotChoice{{1, f.ann, ""}})
	f.emails()
	n := len(f.emails())
	f.env.applySlots(f.game, []slotChoice{{1, f.ann, ""}})
	if len(f.emails()) != n {
		t.Fatal("re-saving the same assignment must not re-email")
	}
	f.db.RespondUmpireSlot(f.game.ID, 1, f.ann, false)
	f.env.applySlots(f.game, []slotChoice{{1, 0, ""}})
	if s := f.db.GameUmpires(f.game.ID); len(s) != 1 || s[0].Status != "declined" {
		t.Fatalf("blank must not erase a declined slot: %+v", s)
	}
}

func TestNotifyGameChangedResetsPending(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.db.RespondUmpireSlot(f.game.ID, 1, f.ann, true)
	same := f.game
	f.env.notifyGameChanged(f.game, same)
	if f.db.GameUmpires(f.game.ID)[0].Status != "accepted" || len(f.emails()) != 0 {
		t.Fatal("unchanged time/location must not reset or email")
	}
	moved := f.game
	moved.Start = moved.Start.Add(3 * time.Hour)
	f.env.notifyGameChanged(f.game, moved)
	if f.db.GameUmpires(f.game.ID)[0].Status != "pending" {
		t.Fatal("moved game should reset accepted slot to pending")
	}
	if e := f.emails(); len(e) != 1 || !strings.HasPrefix(e[0], "ann@x.com|") {
		t.Fatalf("got %v", e)
	}
}

func TestNotifyGameDeleted(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.db.AssignUmpire(f.game.ID, 2, 0, "Free")
	f.env.notifyGameDeleted(f.game)
	if e := f.emails(); len(e) != 1 || !strings.HasPrefix(e[0], "ann@x.com|") {
		t.Fatalf("got %v", e)
	}
}

func TestEditGameKeepsUmpireCacheAndRejectsConflict(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	director := TWUser{ID: 1, Roles: []TournamentRole{{TournamentID: f.tid, Role: "director"}}}
	ps := httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}, {Key: "gid", Value: strconv.Itoa(f.game.ID)}}
	tm := f.db.ReturnTeamsByDivisionID(f.did)
	base := []string{"divisionid", strconv.Itoa(f.did), "hometeam", strconv.Itoa(tm[0].ID), "awayteam", strconv.Itoa(tm[1].ID),
		"location", "Field 1", "datetime", "2026-07-10T10:00"}
	post := func(extra ...string) *httptest.ResponseRecorder {
		req := form(append(base, extra...)...)
		req = req.WithContext(ctxWithUser(req.Context(), director))
		w := httptest.NewRecorder()
		f.env.ManageEditGame(w, req, ps)
		return w
	}
	if w := post("ump_user_1", strconv.Itoa(f.ann), "ump_name_2", "Free Ump"); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	if got := f.db.ReturnGameByID(f.game.ID).Umpire; got != "Ann, Free Ump" {
		t.Fatalf("cache after edit: %q", got)
	}
	other := f.db.AddGame(f.tid, f.did, tm[0].ID, tm[1].ID, "F2", f.game.Start.Add(30*time.Minute), "")
	f.db.AssignUmpire(other, 1, f.bob, "")
	if w := post("ump_user_1", strconv.Itoa(f.bob)); w.Code != http.StatusConflict {
		t.Fatalf("conflict: got %d want 409", w.Code)
	}
	if got := f.db.ReturnGameByID(f.game.ID).Umpire; got != "Ann, Free Ump" {
		t.Fatalf("rejected edit must change nothing, got %q", got)
	}
}

func TestGameFormsRenderSlotPickers(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.db.SetUmpireVerified(f.tid, f.ann, 1, true)
	f.db.AssignUmpire(f.game.ID, 2, 0, "<b>Evil</b>")
	admin := TWUser{ID: 1, IsAdmin: true}
	tp := httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}, {Key: "did", Value: strconv.Itoa(f.did)}}
	gp := httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}, {Key: "gid", Value: strconv.Itoa(f.game.ID)}}
	run := func(h func(http.ResponseWriter, *http.Request, httprouter.Params), ps httprouter.Params) string {
		req := httptest.NewRequest("GET", "/x", nil)
		req = req.WithContext(ctxWithUser(req.Context(), admin))
		w := httptest.NewRecorder()
		h(w, req, ps)
		if w.Code != 200 {
			t.Fatalf("code %d: %s", w.Code, w.Body.String())
		}
		return w.Body.String()
	}
	for name, body := range map[string]string{
		"manage create": run(f.env.ManageCreateGame, tp),
		"admin create":  run(f.env.CreateGame, tp),
		"manage edit":   run(f.env.ManageEditGame, gp),
		"admin edit":    run(f.env.EditGame, gp),
	} {
		if !strings.Contains(body, `name="ump_user_1"`) || !strings.Contains(body, `name="ump_user_2"`) {
			t.Errorf("%s: want two slot pickers", name)
		}
		if !strings.Contains(body, "Ann") || !strings.Contains(body, "verified") {
			t.Errorf("%s: want verified Ann in picker", name)
		}
		if strings.Contains(body, `name="umpire"`) {
			t.Errorf("%s: legacy umpire text input should be gone", name)
		}
	}
	edit := run(f.env.ManageEditGame, gp)
	if strings.Contains(edit, "<b>Evil</b>") || !strings.Contains(edit, "&lt;b&gt;Evil&lt;/b&gt;") {
		t.Error("free-form name must be HTML-escaped")
	}
}

func ctxWithUser(ctx context.Context, u TWUser) context.Context {
	return context.WithValue(ctx, contextKey{}, u)
}

func TestDraftTournamentRejectsRegisteredUmpires(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	draft := f.db.AddTournament("D", "baseball", "L", "", time.Now(), "draft")
	tr := f.db.ReturnTournamentByID(draft)
	if msg := f.env.validateSlotsFor(tr, []slotChoice{{1, f.ann, ""}}, f.game.Start, 0); msg == "" {
		t.Error("draft tournaments must not assign registered umpires")
	}
	if msg := f.env.validateSlotsFor(tr, []slotChoice{{1, 0, "Free Ump"}}, f.game.Start, 0); msg != "" {
		t.Errorf("free-form names are fine in drafts: %q", msg)
	}
	if got := f.env.slotBlock(tr, f.did, nil); len(got.Umpires) != 0 {
		t.Error("draft tournaments get no registered-umpire list")
	}
}

func slotHolders(f *slotFx) map[int]string {
	out := map[int]string{}
	for _, s := range f.db.GameUmpires(f.game.ID) {
		switch {
		case s.Status == "declined":
			out[s.Slot] = "declined:" + s.UserName
		case s.UserID > 0:
			out[s.Slot] = s.UserName
		default:
			out[s.Slot] = s.FreeName
		}
	}
	return out
}

func TestApplySlotsMoveUmpireBetweenSlots(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, 0, "Free")
	f.db.AssignUmpire(f.game.ID, 2, f.ann, "")
	if msg := f.env.applySlots(f.game, []slotChoice{{1, f.ann, ""}, {2, 0, ""}}); msg != "" {
		t.Fatal(msg)
	}
	if got := slotHolders(f); len(got) != 1 || got[1] != "Ann" {
		t.Fatalf("Ann must end up in slot 1, got %v", got)
	}
}

func TestApplySlotsSwapUmpires(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.db.AssignUmpire(f.game.ID, 2, f.bob, "")
	if msg := f.env.applySlots(f.game, []slotChoice{{1, f.bob, ""}, {2, f.ann, ""}}); msg != "" {
		t.Fatal(msg)
	}
	if got := slotHolders(f); got[1] != "Bob" || got[2] != "Ann" {
		t.Fatalf("swap failed: %v", got)
	}
}

func TestApplySlotsDeclinedUmpireReassignedToOtherSlot(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	f.db.RespondUmpireSlot(f.game.ID, 1, f.ann, false)
	if msg := f.env.applySlots(f.game, []slotChoice{{1, 0, ""}, {2, f.ann, ""}}); msg != "" {
		t.Fatal(msg)
	}
	if got := slotHolders(f); got[2] != "Ann" || got[1] != "" && got[1] != "declined:Ann" {
		t.Fatalf("got %v", got)
	}
	if s := f.db.GameUmpires(f.game.ID); len(s) != 1 || s[0].Slot != 2 {
		t.Fatalf("the stale declined row must be cleared when the umpire is reassigned: %+v", s)
	}
}

func TestApplySlotsReportsAssignFailure(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	tm := f.db.ReturnTeamsByDivisionID(f.did)
	other := f.db.AddGame(f.tid, f.did, tm[0].ID, tm[1].ID, "F2", f.game.Start.Add(30*time.Minute), "")
	f.db.AssignUmpire(other, 1, f.ann, "")
	if msg := f.env.applySlots(f.game, []slotChoice{{1, f.ann, ""}}); msg == "" {
		t.Fatal("a failed assignment must be reported, not swallowed")
	}
}

func TestVerifyBlockedForDraftTournament(t *testing.T) {
	env, db, _, _, uid, dir := umpFixture(t)
	draft := db.AddTournament("Draft", "baseball", "L", "", time.Now(), "draft")
	dir.Roles = append(dir.Roles, TournamentRole{TournamentID: draft, Role: "director"})
	w := httptest.NewRecorder()
	env.ManageUmpireVerify(w, directorReq("POST", "verified=1", dir),
		httprouter.Params{{Key: "tid", Value: strconv.Itoa(draft)}, {Key: "uid", Value: strconv.Itoa(uid)}})
	if w.Code != http.StatusForbidden || db.VerifiedUmpireIDs(draft)[uid] {
		t.Fatalf("draft tournaments must not verify umpires: %d", w.Code)
	}
}

func TestPickerShowsExpiredWarningButStaysSelectable(t *testing.T) {
	f := newSlotFx(t)
	defer f.restore()
	f.db.SetUmpireVerified(f.tid, f.ann, 1, true)
	f.db.SetUmpireExpiredFlag(f.tid, f.ann, 1, true)
	tr := f.db.ReturnTournamentByID(f.tid)
	var ann umpireOption
	for _, o := range f.env.slotBlock(tr, f.did, nil).Umpires {
		if o.UserID == f.ann {
			ann = o
		}
	}
	if !strings.Contains(ann.Label, "expired") || strings.Contains(ann.Label, "verified") || ann.Blocked {
		t.Fatalf("expired umpire: warn, drop verified badge, stay assignable: %+v", ann)
	}
	if msg := f.env.validateSlots([]slotChoice{{1, f.ann, ""}}, f.game.Start, f.game.ID); msg != "" {
		t.Fatalf("expired umpires must still be assignable: %q", msg)
	}
	f.db.SetUmpireCertExpiry(f.bob, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	for _, o := range f.env.slotBlock(tr, f.did, nil).Umpires {
		if o.UserID == f.bob && !strings.Contains(o.Label, "expired") {
			t.Fatalf("a passed expiration date warns too: %+v", o)
		}
	}
}
