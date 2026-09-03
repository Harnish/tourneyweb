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

func gcTestTeam(t *testing.T) (*mydb.FakeDB, int, int) {
	t.Helper()
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	did := db.ReturnDivisions(tid)[0].ID
	db.AddTeam(tid, did, "Eagles", "")
	teamID := db.ReturnTeamsByDivisionID(did)[0].ID
	return db, tid, teamID
}

func gcPost(env *Env, tid, teamID int, gcURL string, user TWUser) *httptest.ResponseRecorder {
	body := strings.NewReader(url.Values{"gamechanger_url": {gcURL}}.Encode())
	req := httptest.NewRequest(http.MethodPost, "/x", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), contextKey{}, user))
	w := httptest.NewRecorder()
	ps := httprouter.Params{
		{Key: "tid", Value: strconv.Itoa(tid)},
		{Key: "teamid", Value: strconv.Itoa(teamID)},
	}
	env.RosterSetGameChanger(w, req, ps)
	return w
}

func TestRosterSetGameChanger_CoachSetsURL(t *testing.T) {
	db, tid, teamID := gcTestTeam(t)
	env := &Env{DB: db, BaseURL: "https://example.com"}
	coach := TWUser{ID: 1, Roles: []TournamentRole{{TournamentID: tid, Role: "coach", TeamID: teamID}}}

	w := gcPost(env, tid, teamID, "https://web.gamechanger.io/teams/abc", coach)

	if w.Result().StatusCode != http.StatusSeeOther {
		t.Fatalf("want 303, got %d", w.Result().StatusCode)
	}
	if got := db.ReturnTeamByID(teamID).GameChangerURL; got != "https://web.gamechanger.io/teams/abc" {
		t.Errorf("URL not saved, got %q", got)
	}
}

func TestRosterSetGameChanger_RejectsForeignHost(t *testing.T) {
	db, tid, teamID := gcTestTeam(t)
	env := &Env{DB: db, BaseURL: "https://example.com"}
	coach := TWUser{ID: 1, Roles: []TournamentRole{{TournamentID: tid, Role: "coach", TeamID: teamID}}}

	w := gcPost(env, tid, teamID, "https://evil.com/x", coach)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Result().StatusCode)
	}
	if got := db.ReturnTeamByID(teamID).GameChangerURL; got != "" {
		t.Errorf("bad URL should not be saved, got %q", got)
	}
}

func TestTeamGameChangerQR(t *testing.T) {
	db, tid, teamID := gcTestTeam(t)
	db.SetTeamGameChanger(teamID, "https://web.gamechanger.io/teams/abc")
	env := &Env{DB: db, BaseURL: "https://example.com"}

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	ps := httprouter.Params{
		{Key: "tid", Value: strconv.Itoa(tid)},
		{Key: "teamid", Value: strconv.Itoa(teamID)},
	}
	env.TeamGameChangerQR(w, req, ps)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Result().StatusCode)
	}
	if ct := w.Result().Header.Get("Content-Type"); ct != "image/png" {
		t.Fatalf("want image/png, got %q", ct)
	}
	if b := w.Body.Bytes(); len(b) < 4 || string(b[:4]) != "\x89PNG" {
		t.Fatal("body is not a PNG")
	}
}

func TestTeamGameChangerQR_404WhenUnset(t *testing.T) {
	db, tid, teamID := gcTestTeam(t)
	env := &Env{DB: db, BaseURL: "https://example.com"}

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	ps := httprouter.Params{
		{Key: "tid", Value: strconv.Itoa(tid)},
		{Key: "teamid", Value: strconv.Itoa(teamID)},
	}
	env.TeamGameChangerQR(w, req, ps)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Result().StatusCode)
	}
}

func TestShowTeam_GameChangerLink(t *testing.T) {
	db, tid, teamID := gcTestTeam(t)
	env := &Env{DB: db, BaseURL: "https://example.com"}
	ps := httprouter.Params{
		{Key: "tid", Value: strconv.Itoa(tid)},
		{Key: "teamid", Value: strconv.Itoa(teamID)},
	}

	// Not set: no GameChanger markup.
	w := httptest.NewRecorder()
	env.ShowTeam(w, httptest.NewRequest(http.MethodGet, "/x", nil), ps)
	if strings.Contains(w.Body.String(), "GameChanger") {
		t.Error("team page should not mention GameChanger when link unset")
	}

	// Set: link + QR image appear.
	db.SetTeamGameChanger(teamID, "https://web.gamechanger.io/teams/abc")
	w = httptest.NewRecorder()
	env.ShowTeam(w, httptest.NewRequest(http.MethodGet, "/x", nil), ps)
	body := w.Body.String()
	if !strings.Contains(body, "https://web.gamechanger.io/teams/abc") {
		t.Error("team page should link to the GameChanger URL")
	}
	if !strings.Contains(body, "/teams/"+strconv.Itoa(teamID)+"/gc-qr.png") {
		t.Error("team page should embed the GameChanger QR image")
	}
}

func TestRosterSetGameChanger_DeniesStranger(t *testing.T) {
	db, tid, teamID := gcTestTeam(t)
	env := &Env{DB: db, BaseURL: "https://example.com"}

	w := gcPost(env, tid, teamID, "https://gamechanger.io/x", TWUser{})

	if w.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Result().StatusCode)
	}
}

func TestValidGameChangerURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"empty clears", "", "", true},
		{"whitespace clears", "   ", "", true},
		{"gamechanger host", "https://gamechanger.io/teams/abc", "https://gamechanger.io/teams/abc", true},
		{"web subdomain", "https://web.gamechanger.io/teams/abc", "https://web.gamechanger.io/teams/abc", true},
		{"trims surrounding space", "  https://web.gamechanger.io/x  ", "https://web.gamechanger.io/x", true},
		{"http allowed", "http://gamechanger.io/t", "http://gamechanger.io/t", true},
		{"other host rejected", "https://evil.com/x", "", false},
		{"suffix spoof rejected", "https://gamechanger.io.evil.com/x", "", false},
		{"substring spoof rejected", "https://notgamechanger.io/x", "", false},
		{"non-http scheme rejected", "ftp://gamechanger.io/x", "", false},
		{"garbage rejected", "not a url", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := validGameChangerURL(c.in)
			if ok != c.ok || got != c.want {
				t.Errorf("validGameChangerURL(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
			}
		})
	}
}
