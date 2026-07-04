package webhandler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func TestTeamQRCode(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	did := db.ReturnDivisions(tid)[0].ID
	db.AddTeam(tid, did, "Eagles", "")
	teamID := db.ReturnTeamsByDivisionID(did)[0].ID

	env := &Env{DB: db, BaseURL: "https://example.com"}
	req := httptest.NewRequest(http.MethodGet, "/tournaments/1/teams/1/qr.png", nil)
	w := httptest.NewRecorder()
	ps := httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}, {Key: "teamid", Value: strconv.Itoa(teamID)}}

	env.TeamQRCode(w, req, ps)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Fatalf("want Content-Type image/png, got %q", ct)
	}
	body := w.Body.Bytes()
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47}
	if len(body) < 4 || string(body[:4]) != string(pngMagic) {
		t.Fatal("response body is not a valid PNG")
	}
}

func TestTeamQRCode_WrongTournament(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	otherTid := db.AddTournament("Other", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	did := db.ReturnDivisions(tid)[0].ID
	db.AddTeam(tid, did, "Eagles", "")
	teamID := db.ReturnTeamsByDivisionID(did)[0].ID

	env := &Env{DB: db, BaseURL: "https://example.com"}
	req := httptest.NewRequest(http.MethodGet, "/tournaments/2/teams/1/qr.png", nil)
	w := httptest.NewRecorder()
	ps := httprouter.Params{{Key: "tid", Value: strconv.Itoa(otherTid)}, {Key: "teamid", Value: strconv.Itoa(teamID)}}

	env.TeamQRCode(w, req, ps)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 for team belonging to a different tournament, got %d", w.Result().StatusCode)
	}
}

func TestTeamSheet(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "here", "", time.Time{}, "active")
	db.AddDivision(tid, "12U")
	did := db.ReturnDivisions(tid)[0].ID
	db.AddTeam(tid, did, "Eagles", "Coach Smith")
	teamID := db.ReturnTeamsByDivisionID(did)[0].ID
	db.AddPlayer(teamID, "7", "Jane", "Doe", "L", "SS")

	env := &Env{DB: db, BaseURL: "https://example.com"}
	req := httptest.NewRequest(http.MethodGet, "/tournaments/"+strconv.Itoa(tid)+"/teams/"+strconv.Itoa(teamID)+"/sheet", nil)
	w := httptest.NewRecorder()
	ps := httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}, {Key: "teamid", Value: strconv.Itoa(teamID)}}

	env.TeamSheet(w, req, ps)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Eagles") {
		t.Error("team sheet should include team name")
	}
	if !strings.Contains(body, "J. Doe") {
		t.Error("team sheet should include roster player display name")
	}
	wantImg := "/tournaments/" + strconv.Itoa(tid) + "/teams/" + strconv.Itoa(teamID) + "/qr.png"
	if !strings.Contains(body, wantImg) {
		t.Errorf("team sheet should embed the QR code image at %s", wantImg)
	}
}
