package webhandler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/Harnish/tourneyweb/mydb"
)

func directorReq(method, body string, user TWUser) *http.Request {
	req := httptest.NewRequest(method, "/x", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req.WithContext(context.WithValue(req.Context(), contextKey{}, user))
}

func umpFixture(t *testing.T) (*Env, *mydb.FakeDB, int, int, int, TWUser) {
	t.Helper()
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T1", "baseball", "L", "", time.Now(), "published")
	did := db.AddDivision(tid, "10U")
	u, _ := db.CreateUser("u@x.com", "Una", "h", "", true, false)
	d, _ := db.CreateUser("d@x.com", "Dir", "h", "", true, false)
	db.SetUmpireProfile(u.ID, true, "https://assoc.example/una")
	dir := TWUser{ID: d.ID, Name: "Dir", Roles: []TournamentRole{{TournamentID: tid, Role: "director"}}}
	return &Env{DB: db}, db, tid, did, u.ID, dir
}

func TestVerifyUmpireScopedToTournament(t *testing.T) {
	env, db, tid, _, uid, dir := umpFixture(t)
	tid2 := db.AddTournament("T2", "baseball", "L", "", time.Now(), "published")
	w := httptest.NewRecorder()
	env.ManageUmpireVerify(w, directorReq("POST", url.Values{"verified": {"1"}}.Encode(), dir),
		httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}, {Key: "uid", Value: strconv.Itoa(uid)}})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("got %d", w.Code)
	}
	if !db.VerifiedUmpireIDs(tid)[uid] || db.VerifiedUmpireIDs(tid2)[uid] {
		t.Fatal("verification must apply to this tournament only")
	}
	env.ManageUmpireVerify(httptest.NewRecorder(), directorReq("POST", url.Values{"verified": {"0"}}.Encode(), dir),
		httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}, {Key: "uid", Value: strconv.Itoa(uid)}})
	if db.VerifiedUmpireIDs(tid)[uid] {
		t.Fatal("unverify should clear")
	}
}

func TestVerifyNonUmpire404(t *testing.T) {
	env, db, tid, _, _, dir := umpFixture(t)
	other, _ := db.CreateUser("o@x.com", "Other", "h", "", true, false)
	w := httptest.NewRecorder()
	env.ManageUmpireVerify(w, directorReq("POST", "verified=1", dir),
		httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}, {Key: "uid", Value: strconv.Itoa(other.ID)}})
	if w.Code != http.StatusNotFound {
		t.Fatalf("got %d want 404", w.Code)
	}
}

func TestManageUmpiresPageRenders(t *testing.T) {
	env, db, tid, _, uid, dir := umpFixture(t)
	db.SetUmpireVerified(tid, uid, dir.ID, true)
	w := httptest.NewRecorder()
	env.ManageUmpires(w, directorReq("GET", "", dir), httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}})
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "Una") || !strings.Contains(body, "https://assoc.example/una") || !strings.Contains(body, "verified") {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
}

func TestDivisionEditSavesUmpiresNeeded(t *testing.T) {
	env, db, tid, did, _, dir := umpFixture(t)
	ps := httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}, {Key: "did", Value: strconv.Itoa(did)}}
	post := func(n string) {
		env.ManageDivisionEdit(httptest.NewRecorder(),
			directorReq("POST", url.Values{"name": {"10U"}, "umpires_needed": {n}}.Encode(), dir), ps)
	}
	post("2")
	if got := db.DivisionUmpiresNeeded(did); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
	post("abc")
	if got := db.DivisionUmpiresNeeded(did); got != 2 {
		t.Fatalf("garbage must leave value unchanged, got %d", got)
	}
}

func TestManageUmpiresBlockedForDraftTournament(t *testing.T) {
	env, db, _, _, _, dir := umpFixture(t)
	draft := db.AddTournament("Draft", "baseball", "L", "", time.Now(), "draft")
	dir.Roles = append(dir.Roles, TournamentRole{TournamentID: draft, Role: "director"})
	w := httptest.NewRecorder()
	env.ManageUmpires(w, directorReq("GET", "", dir), httprouter.Params{{Key: "tid", Value: strconv.Itoa(draft)}})
	if w.Code != http.StatusForbidden || strings.Contains(w.Body.String(), "https://assoc.example/una") {
		t.Fatalf("draft tournament must not expose the umpire directory: %d", w.Code)
	}
}
