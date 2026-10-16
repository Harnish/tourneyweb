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
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
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

func expireReq(env *Env, dir TWUser, tid, uid int, v string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	env.ManageUmpireExpire(w, directorReq("POST", url.Values{"expired": {v}}.Encode(), dir),
		httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}, {Key: "uid", Value: strconv.Itoa(uid)}})
	return w
}

func TestExpireFlagScopedToTournament(t *testing.T) {
	env, db, tid, _, uid, dir := umpFixture(t)
	tid2 := db.AddTournament("T2", "baseball", "L", "", time.Now(), "published")
	if w := expireReq(env, dir, tid, uid, "1"); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d", w.Code)
	}
	if !db.ExpiredFlagUmpireIDs(tid)[uid] || db.ExpiredFlagUmpireIDs(tid2)[uid] {
		t.Fatal("flag must apply to this tournament only")
	}
	expireReq(env, dir, tid, uid, "0")
	if db.ExpiredFlagUmpireIDs(tid)[uid] {
		t.Fatal("clear should remove the flag")
	}
}

func TestExpireFlagGuards(t *testing.T) {
	env, db, tid, _, _, dir := umpFixture(t)
	other, _ := db.CreateUser("o@x.com", "Other", "h", "", true, false)
	if w := expireReq(env, dir, tid, other.ID, "1"); w.Code != http.StatusNotFound {
		t.Errorf("non-umpire: got %d want 404", w.Code)
	}
	draft := db.AddTournament("Draft", "baseball", "L", "", time.Now(), "draft")
	dir.Roles = append(dir.Roles, TournamentRole{TournamentID: draft, Role: "director"})
	if w := expireReq(env, dir, draft, other.ID, "1"); w.Code != http.StatusForbidden {
		t.Errorf("draft tournament: got %d want 403", w.Code)
	}
}

func TestManageUmpiresShowsExpiredInsteadOfVerified(t *testing.T) {
	env, db, tid, _, uid, dir := umpFixture(t)
	db.SetUmpireVerified(tid, uid, dir.ID, true)
	db.SetUmpireExpiredFlag(tid, uid, dir.ID, true)
	db.SetUmpireCertExpiry(uid, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	w := httptest.NewRecorder()
	env.ManageUmpires(w, directorReq("GET", "", dir), httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}})
	body := w.Body.String()
	if !strings.Contains(body, "expired") || strings.Contains(body, "✔ verified") {
		t.Fatalf("expired umpire shows expired, not verified: %s", body)
	}
	if !strings.Contains(body, "Clear expired") {
		t.Error("want a button to clear the flag")
	}
}
