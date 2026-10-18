package webhandler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
	"github.com/julienschmidt/httprouter"
)

var anon = TWUser{}

func (f *entryFx) post(h http.HandlerFunc, u TWUser, kv ...string) *httptest.ResponseRecorder {
	req := form(kv...)
	req = req.WithContext(ctxWithUser(req.Context(), u))
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

func (f *entryFx) get(h http.HandlerFunc, u TWUser, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", target, nil)
	req = req.WithContext(ctxWithUser(req.Context(), u))
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

func (f *entryFx) handler(h func(http.ResponseWriter, *http.Request, httprouter.Params)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { h(w, r, nil) }
}

func itoa(n int) string { return strconv.Itoa(n) }

func fan(f *entryFx, t *testing.T) TWUser {
	t.Helper()
	u, _ := f.db.CreateUser("fan@x.com", "Fan", "h", "", true, false)
	return TWUser{ID: u.ID, Name: "Fan"}
}

func TestFollowRequiresLogin(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	w := f.post(f.handler(f.env.Follow), anon, "kind", "division", "target", itoa(f.did))
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
		t.Fatalf("%d %q", w.Code, w.Header().Get("Location"))
	}
}

func TestFollowDivisionAndTeam(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	u := fan(f, t)
	for _, c := range [][2]string{{"division", itoa(f.did)}, {"team", itoa(f.game.HomeTeam.ID)}, {"tournament", itoa(f.tid)}} {
		w := f.post(f.handler(f.env.Follow), u, "kind", c[0], "target", c[1], "next", "/tournaments/"+itoa(f.tid))
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/tournaments/"+itoa(f.tid) {
			t.Fatalf("%v: %d %q", c, w.Code, w.Header().Get("Location"))
		}
	}
	if n := len(f.db.UserFollows(u.ID)); n != 3 {
		t.Fatalf("follows: %d", n)
	}
	// duplicate is a silent no-op
	if w := f.post(f.handler(f.env.Follow), u, "kind", "division", "target", itoa(f.did)); w.Code != http.StatusSeeOther {
		t.Fatalf("duplicate: %d", w.Code)
	}
	if n := len(f.db.UserFollows(u.ID)); n != 3 {
		t.Fatalf("duplicate follow added a row: %d", n)
	}
}

func TestFollowDraftAndUnknownTargetsAre404(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	u := fan(f, t)
	draft := f.db.AddTournament("D", "baseball", "L", "", time.Now(), "draft")
	dd := f.db.AddDivision(draft, "10U")
	f.db.AddTeam(draft, dd, "X", "")
	dt := f.db.ReturnTeamsByDivisionID(dd)[0].ID
	for _, c := range [][2]string{{"tournament", itoa(draft)}, {"division", itoa(dd)}, {"team", itoa(dt)}, {"team", "999999"}, {"league", "1"}, {"division", "abc"}} {
		if w := f.post(f.handler(f.env.Follow), u, "kind", c[0], "target", c[1]); w.Code != http.StatusNotFound {
			t.Errorf("%v: got %d want 404", c, w.Code)
		}
	}
	if len(f.db.UserFollows(u.ID)) != 0 {
		t.Fatal("nothing should be stored")
	}
}

func TestFollowCapShowsError(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	u := fan(f, t)
	for i := 0; i < 50; i++ {
		f.db.AddFollow(u.ID, "team", 7000+i)
	}
	w := f.post(f.handler(f.env.Follow), u, "kind", "division", "target", itoa(f.did))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "50") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestFollowNextOffSiteIgnored(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	u := fan(f, t)
	for _, next := range []string{"https://evil.example/x", "//evil.example", "/\\evil.example", "javascript:alert(1)", ""} {
		w := f.post(f.handler(f.env.Follow), u, "kind", "division", "target", itoa(f.did), "next", next)
		if loc := w.Header().Get("Location"); loc != "/following" {
			t.Errorf("next %q redirected to %q", next, loc)
		}
	}
}

func TestUnfollowOwnOnly(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	a := fan(f, t)
	bu, _ := f.db.CreateUser("b@x.com", "B", "h", "", true, false)
	f.db.AddFollow(a.ID, "division", f.did)
	f.db.AddFollow(bu.ID, "division", f.did)
	w := f.post(f.handler(f.env.Unfollow), a, "kind", "division", "target", itoa(f.did))
	if w.Code != http.StatusSeeOther {
		t.Fatalf("%d", w.Code)
	}
	if f.db.IsFollowing(a.ID, "division", f.did) || !f.db.IsFollowing(bu.ID, "division", f.did) {
		t.Fatal("unfollow must only touch the caller's follow")
	}
	if w := f.post(f.handler(f.env.Unfollow), anon, "kind", "division", "target", itoa(f.did)); w.Header().Get("Location") != "/login" {
		t.Fatal("unfollow needs login")
	}
}

func TestFollowingPageListsOnlyMine(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	a := fan(f, t)
	bu, _ := f.db.CreateUser("b@x.com", "B", "h", "", true, false)
	f.db.AddFollow(a.ID, "division", f.did)
	f.db.AddFollow(bu.ID, "team", f.game.HomeTeam.ID)
	w := f.get(f.handler(f.env.Following), a, "/following")
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "10U") || strings.Contains(body, "/teams/"+itoa(f.game.HomeTeam.ID)) {
		t.Fatalf("%d %s", w.Code, body)
	}
	if !strings.Contains(body, "/tournaments/"+itoa(f.tid)+"/divisions/"+itoa(f.did)) {
		t.Fatalf("missing link: %s", body)
	}
	if w := f.get(f.handler(f.env.Following), anon, "/following"); w.Header().Get("Location") != "/login" {
		t.Fatal("following needs login")
	}
}

func TestUnfollowTokenFlow(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.env.FollowSecret = []byte("0123456789abcdef0123456789abcdef")
	a := fan(f, t)
	f.db.AddFollow(a.ID, "division", f.did)
	tok := f.env.followToken(a.ID, "division", f.did)
	if tok == "" {
		t.Fatal("token")
	}
	// GET only shows a confirm page.
	w := f.get(f.handler(f.env.UnfollowTokenPage), anon, "/unfollow?t="+url.QueryEscape(tok))
	if w.Code != 200 || !f.db.IsFollowing(a.ID, "division", f.did) || !strings.Contains(w.Body.String(), "10U") {
		t.Fatalf("GET must not unfollow: %d %s", w.Code, w.Body.String())
	}
	// tampered tokens do nothing
	for _, bad := range []string{tok + "x", "x" + tok, "", "a.b", strings.Replace(tok, ".", "A.", 1)} {
		if w := f.post(f.handler(f.env.UnfollowTokenDo), anon, "t", bad); w.Code != http.StatusBadRequest {
			t.Errorf("bad token %q: %d", bad, w.Code)
		}
	}
	if !f.db.IsFollowing(a.ID, "division", f.did) {
		t.Fatal("bad tokens unfollowed")
	}
	// a token minted for another user cannot change this user's follows
	other, _ := f.db.CreateUser("o@x.com", "O", "h", "", true, false)
	f.post(f.handler(f.env.UnfollowTokenDo), anon, "t", f.env.followToken(other.ID, "division", f.did))
	if !f.db.IsFollowing(a.ID, "division", f.did) {
		t.Fatal("another user's token touched my follow")
	}
	if w := f.post(f.handler(f.env.UnfollowTokenDo), anon, "t", tok); w.Code != 200 {
		t.Fatalf("POST: %d %s", w.Code, w.Body.String())
	}
	if f.db.IsFollowing(a.ID, "division", f.did) {
		t.Fatal("valid token should unfollow")
	}
}

func TestTokenDisabledWithoutSecret(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	if f.env.followToken(1, "division", 1) != "" || f.env.unfollowLink(1, mydbFollow("division", 1)) != "" {
		t.Fatal("no secret → no links")
	}
}

func TestAlertEmailContainsUnfollowLink(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.env.FollowSecret = []byte("0123456789abcdef0123456789abcdef")
	f.env.BaseURL = "https://tw.example"
	var gotMsg string
	old := sendNotice
	sendNotice = func(me *Env, to, subject, msg, link string) error {
		if subject == "Final score" {
			gotMsg = msg
		}
		return nil
	}
	defer func() { sendNotice = old }()
	a := fan(f, t)
	f.db.AddFollow(a.ID, "division", f.did)
	f.staffScore(t, "7", "3")
	notifyWG.Wait()
	if !strings.Contains(gotMsg, "https://tw.example/unfollow?t=") {
		t.Fatalf("message: %q", gotMsg)
	}
}

func mydbFollow(kind string, id int) mydb.Follow { return mydb.Follow{Kind: kind, TargetID: id} }

func TestUnfollowPagesPointToOtherFollows(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.env.FollowSecret = []byte("0123456789abcdef0123456789abcdef")
	a := fan(f, t)
	f.db.AddFollow(a.ID, "team", f.game.HomeTeam.ID)
	tok := f.env.followToken(a.ID, "team", f.game.HomeTeam.ID)
	confirm := f.get(f.handler(f.env.UnfollowTokenPage), anon, "/unfollow?t="+url.QueryEscape(tok)).Body.String()
	done := f.post(f.handler(f.env.UnfollowTokenDo), anon, "t", tok).Body.String()
	for name, body := range map[string]string{"confirm": confirm, "done": done} {
		if !strings.Contains(body, `href="/following"`) || !strings.Contains(body, "still") {
			t.Errorf("%s page must say other follows can still alert and link /following: %s", name, body)
		}
	}
}
