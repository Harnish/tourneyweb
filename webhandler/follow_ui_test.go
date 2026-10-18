package webhandler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
)

func (f *entryFx) page(h func(http.ResponseWriter, *http.Request, httprouter.Params), u TWUser, target string, ps httprouter.Params) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", target, nil)
	req = req.WithContext(ctxWithUser(req.Context(), u))
	w := httptest.NewRecorder()
	h(w, req, ps)
	return w
}

func (f *entryFx) tp(extra ...string) httprouter.Params {
	ps := httprouter.Params{{Key: "tid", Value: itoa(f.tid)}}
	for i := 0; i < len(extra); i += 2 {
		ps = append(ps, httprouter.Param{Key: extra[i], Value: extra[i+1]})
	}
	return ps
}

func TestTournamentListSearchFiltersByName(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AddTournament("Spring Slam", "baseball", "L", "", time.Now(), "published")
	f.db.AddTournament("Fall Classic", "baseball", "L", "", time.Now(), "published")
	f.db.AddTournament("Secret Slam", "baseball", "L", "", time.Now(), "draft")
	body := f.page(f.env.TournamentList, anon, "/?q=slam", nil).Body.String()
	if !strings.Contains(body, "Spring Slam") || strings.Contains(body, "Fall Classic") || strings.Contains(body, "Secret Slam") {
		t.Fatalf("results: %s", body)
	}
	if !strings.Contains(body, `value="slam"`) {
		t.Fatal("the search box keeps the query")
	}
	if b := f.page(f.env.TournamentList, anon, "/?q=zzzz", nil).Body.String(); !strings.Contains(b, "No tournaments match") {
		t.Fatalf("empty state: %s", b)
	}
}

func TestTournamentListWithoutQueryUnchanged(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	f.db.AddTournament("Fall Classic", "baseball", "L", "", time.Now(), "published")
	body := f.page(f.env.TournamentList, anon, "/", nil).Body.String()
	if !strings.Contains(body, "Fall Classic") || !strings.Contains(body, `name="q"`) || strings.Contains(body, "No tournaments match") {
		t.Fatalf("list: %s", body)
	}
}

func TestTournamentListShowsFollowButtons(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	id := f.db.AddTournament("Fall Classic", "baseball", "L", "", time.Now(), "published")
	u := fan(f, t)
	body := f.page(f.env.TournamentList, u, "/?q=fall", nil).Body.String()
	if !strings.Contains(body, `action="/follow"`) || !strings.Contains(body, `name="target" value="`+itoa(id)+`"`) {
		t.Fatalf("follow form: %s", body)
	}
	f.db.AddFollow(u.ID, "tournament", id)
	if body := f.page(f.env.TournamentList, u, "/?q=fall", nil).Body.String(); !strings.Contains(body, `action="/unfollow"`) {
		t.Fatalf("following state: %s", body)
	}
}

func TestFollowButtonOnDivisionTeamAndTournamentPages(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	u := fan(f, t)
	cases := []struct {
		name, kind string
		target     int
		h          func(http.ResponseWriter, *http.Request, httprouter.Params)
		ps         httprouter.Params
	}{
		{"division", "division", f.did, f.env.PrintDivision, f.tp("did", itoa(f.did))},
		{"team", "team", f.game.HomeTeam.ID, f.env.ShowTeam, f.tp("teamid", itoa(f.game.HomeTeam.ID))},
		{"tournament", "tournament", f.tid, f.env.TournamentHome, f.tp()},
	}
	for _, c := range cases {
		body := f.page(c.h, u, "/x", c.ps).Body.String()
		if !strings.Contains(body, `action="/follow"`) || !strings.Contains(body, `name="kind" value="`+c.kind+`"`) || !strings.Contains(body, `name="target" value="`+itoa(c.target)+`"`) {
			t.Errorf("%s page lacks follow form: %s", c.name, body)
		}
		f.db.AddFollow(u.ID, c.kind, c.target)
		if body := f.page(c.h, u, "/x", c.ps).Body.String(); !strings.Contains(body, `action="/unfollow"`) {
			t.Errorf("%s page should show following state", c.name)
		}
		f.db.DelFollow(u.ID, c.kind, c.target)
	}
}

func TestFollowButtonLoggedOutLinksToLogin(t *testing.T) {
	f := entryFixture(t)
	defer f.restore()
	body := f.page(f.env.PrintDivision, anon, "/x", f.tp("did", itoa(f.did))).Body.String()
	if !strings.Contains(body, "Log in to follow") || strings.Contains(body, `action="/follow"`) {
		t.Fatalf("logged-out: %s", body)
	}
}
