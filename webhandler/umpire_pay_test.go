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

type payFx struct {
	*slotFx
	div12 int
}

// payFixture: tournament with a rated 10U division (the fixture's) and an unrated 12U division.
func payFixture(t *testing.T) *payFx {
	t.Helper()
	f := newSlotFx(t)
	f.db.SetDivisionPayPerGameCents(f.did, 7500)
	d12 := f.db.AddDivision(f.tid, "12U")
	f.db.AddTeam(f.tid, d12, "C", "")
	f.db.AddTeam(f.tid, d12, "D", "")
	return &payFx{slotFx: f, div12: d12}
}

func (f *payFx) addGame(did int, hoursFromStart int) int {
	tm := f.db.ReturnTeamsByDivisionID(did)
	return f.db.AddGame(f.tid, did, tm[0].ID, tm[1].ID, "Field", f.game.Start.Add(time.Duration(hoursFromStart)*time.Hour), "")
}

func findUmpire(rep []payUmpire, name string) *payUmpire {
	for i := range rep {
		if rep[i].Name == name {
			return &rep[i]
		}
	}
	return nil
}

func TestPayReportCountsAssignedNotDeclinedNotAbsent(t *testing.T) {
	f := payFixture(t)
	defer f.restore()
	g1, g2, g3, g4 := f.addGame(f.did, 4), f.addGame(f.did, 8), f.addGame(f.did, 12), f.addGame(f.did, 16)
	f.db.AssignUmpire(g1, 1, f.ann, "") // pending counts
	f.db.AssignUmpire(g2, 1, f.ann, "")
	f.db.RespondUmpireSlot(g2, 1, f.ann, true) // accepted counts
	f.db.AssignUmpire(g3, 1, f.ann, "")
	f.db.RespondUmpireSlot(g3, 1, f.ann, false) // declined does not count
	f.db.AssignUmpire(g4, 1, f.ann, "")
	f.db.SetUmpireSlotAbsent(g4, 1, true) // absent does not count
	rep := f.env.buildPayReport(f.tid)
	ann := findUmpire(rep, "Ann")
	if ann == nil || len(ann.Lines) != 1 {
		t.Fatalf("got %+v", rep)
	}
	l := ann.Lines[0]
	if l.Division != "10U" || l.Games != 2 || l.RateCents != 7500 || l.AmountCents != 15000 || !l.HasRate {
		t.Fatalf("line: %+v", l)
	}
	if ann.TotalCents != 15000 || ann.UnratedGames != 0 {
		t.Fatalf("total: %+v", ann)
	}
	var absent, declinedListed int
	for _, g := range ann.Games {
		if g.Absent {
			absent++
		}
		if g.Game.ID == g3 {
			declinedListed++
		}
	}
	if absent != 1 || declinedListed != 0 {
		t.Fatalf("game list: absent=%d declinedListed=%d (declined games are not listed, absent ones are)", absent, declinedListed)
	}
}

func TestPayReportUnratedDivisionAndFreeFormGrouping(t *testing.T) {
	f := payFixture(t)
	defer f.restore()
	g1, g2 := f.addGame(f.did, 4), f.addGame(f.div12, 4)
	g3 := f.addGame(f.div12, 8)
	f.db.AssignUmpire(g1, 1, 0, "Bob K.")
	f.db.AssignUmpire(g2, 1, 0, " bob k. ") // same person, different spelling
	f.db.AssignUmpire(g3, 1, 0, "BOB K.")
	rep := f.env.buildPayReport(f.tid)
	bob := findUmpire(rep, "Bob K.")
	if bob == nil || len(rep) != 1 {
		t.Fatalf("free-form names group case-insensitively: %+v", rep)
	}
	if len(bob.Lines) != 2 {
		t.Fatalf("lines: %+v", bob.Lines)
	}
	var rated, unrated *payLine
	for i := range bob.Lines {
		if bob.Lines[i].HasRate {
			rated = &bob.Lines[i]
		} else {
			unrated = &bob.Lines[i]
		}
	}
	if rated == nil || rated.Games != 1 || rated.AmountCents != 7500 {
		t.Fatalf("rated: %+v", rated)
	}
	if unrated == nil || unrated.Division != "12U" || unrated.Games != 2 || unrated.AmountCents != 0 {
		t.Fatalf("unrated: %+v", unrated)
	}
	if bob.TotalCents != 7500 || bob.UnratedGames != 2 {
		t.Fatalf("total covers rated divisions only: %+v", bob)
	}
}

func TestPayReportIsScopedToTheTournament(t *testing.T) {
	f := payFixture(t)
	defer f.restore()
	other := f.db.AddTournament("Other", "baseball", "L", "", time.Now(), "published")
	od := f.db.AddDivision(other, "10U")
	f.db.AddTeam(other, od, "X", "")
	f.db.AddTeam(other, od, "Y", "")
	tm := f.db.ReturnTeamsByDivisionID(od)
	og := f.db.AddGame(other, od, tm[0].ID, tm[1].ID, "F", f.game.Start, "")
	f.db.AssignUmpire(og, 1, 0, "Elsewhere E.")
	if rep := f.env.buildPayReport(f.tid); len(rep) != 0 {
		t.Fatalf("other tournaments' umpires must not appear: %+v", rep)
	}
}

func payGet(f *payFx, path string, h func(http.ResponseWriter, *http.Request, httprouter.Params)) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	req = req.WithContext(ctxWithUser(req.Context(), dirUser(1, f.tid)))
	w := httptest.NewRecorder()
	h(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(f.tid)}})
	return w
}

func TestPayPageShowsAmountsAndEscapesNames(t *testing.T) {
	f := payFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, 0, "<b>Eve</b>")
	g := f.addGame(f.did, 4)
	f.db.AssignUmpire(g, 1, 0, "<b>Eve</b>")
	w := payGet(f, "/x", f.env.ManageUmpirePay)
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, "$150.00") || !strings.Contains(body, "2 games") {
		t.Fatalf("code=%d body=%s", w.Code, body)
	}
	if strings.Contains(body, "<b>Eve</b>") || !strings.Contains(body, "&lt;b&gt;Eve&lt;/b&gt;") {
		t.Fatal("names must be escaped")
	}
	if !strings.Contains(body, "Mark absent") {
		t.Fatal("each game offers Mark absent")
	}
}

func TestPayPageShowsNoRateSet(t *testing.T) {
	f := payFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.addGame(f.div12, 4), 1, f.bob, "")
	body := payGet(f, "/x", f.env.ManageUmpirePay).Body.String()
	if !strings.Contains(body, "no rate set") || !strings.Contains(body, "1 game") {
		t.Fatalf("unrated divisions show counts only: %s", body)
	}
}

func absentPost(f *payFx, tid, gid, slot int, v string) *httptest.ResponseRecorder {
	req := form("absent", v)
	req = req.WithContext(ctxWithUser(req.Context(), dirUser(1, tid)))
	w := httptest.NewRecorder()
	f.env.ManageUmpireAbsent(w, req, httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}, {Key: "gid", Value: strconv.Itoa(gid)}, {Key: "slot", Value: strconv.Itoa(slot)}})
	return w
}

func TestAbsentToggleAndTournamentGuard(t *testing.T) {
	f := payFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, f.ann, "")
	if w := absentPost(f, f.tid, f.game.ID, 1, "1"); w.Code != http.StatusSeeOther {
		t.Fatalf("got %d", w.Code)
	}
	if ann := findUmpire(f.env.buildPayReport(f.tid), "Ann"); ann == nil || ann.Lines != nil && len(ann.Lines) != 0 || ann.TotalCents != 0 {
		t.Fatalf("absent game must not be paid: %+v", ann)
	}
	absentPost(f, f.tid, f.game.ID, 1, "0")
	if ann := findUmpire(f.env.buildPayReport(f.tid), "Ann"); ann == nil || ann.TotalCents != 7500 {
		t.Fatalf("restore counts again: %+v", ann)
	}
	other := f.db.AddTournament("Other", "baseball", "L", "", time.Now(), "published")
	if w := absentPost(f, other, f.game.ID, 1, "1"); w.Code != http.StatusNotFound {
		t.Fatalf("game from another tournament: got %d want 404", w.Code)
	}
	if w := absentPost(f, f.tid, f.game.ID, 9, "1"); w.Code != http.StatusNotFound {
		t.Fatalf("unknown slot: got %d want 404", w.Code)
	}
}

func TestPayCSVRowsAndFormulaEscaping(t *testing.T) {
	f := payFixture(t)
	defer f.restore()
	f.db.AssignUmpire(f.game.ID, 1, 0, "=cmd|' /C calc'!A0")
	f.db.AssignUmpire(f.addGame(f.div12, 4), 1, f.ann, "")
	w := payGet(f, "/x.csv", f.env.ManageUmpirePayCSV)
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/csv") || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("headers: %v", w.Header())
	}
	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	if lines[0] != "Umpire,Division,Games,Rate,Amount" {
		t.Fatalf("header: %q", lines[0])
	}
	body := w.Body.String()
	if !strings.Contains(body, "'=cmd") || strings.Contains(body, ",=cmd") || strings.HasPrefix(lines[1], "=") {
		t.Fatalf("formula-like names must be neutralised: %s", body)
	}
	if !strings.Contains(body, "10U,1,75.00,75.00") {
		t.Fatalf("rated row: %s", body)
	}
	if !strings.Contains(body, "12U,1,,") {
		t.Fatalf("unrated row has empty rate and amount: %s", body)
	}
}

func TestMoneyParseAndFormat(t *testing.T) {
	if formatCents(7550) != "$75.50" || formatCents(0) != "$0.00" || formatCents(100000) != "$1000.00" {
		t.Fatal("formatCents")
	}
	ok := map[string]int{"": 0, "75": 7500, "75.5": 7550, "75.50": 7550, " $100 ": 10000, "0.05": 5}
	for in, want := range ok {
		if got, err := parseCents(in); err != nil || got != want {
			t.Errorf("parseCents(%q) = %d, %v want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"abc", "-5", "75.555", "1e3", "$", "75,50"} {
		if _, err := parseCents(bad); err == nil {
			t.Errorf("parseCents(%q) should fail", bad)
		}
	}
}

func TestDivisionEditSavesPayRate(t *testing.T) {
	env, db, tid, did, _, dir := umpFixture(t)
	ps := httprouter.Params{{Key: "tid", Value: strconv.Itoa(tid)}, {Key: "did", Value: strconv.Itoa(did)}}
	post := func(v string) int {
		w := httptest.NewRecorder()
		env.ManageDivisionEdit(w, directorReq("POST", url.Values{"name": {"10U"}, "pay_per_game": {v}}.Encode(), dir), ps)
		return w.Code
	}
	if c := post("75.50"); c != http.StatusSeeOther || db.DivisionPayPerGameCents(did) != 7550 {
		t.Fatalf("code=%d rate=%d", c, db.DivisionPayPerGameCents(did))
	}
	if c := post("abc"); c != http.StatusBadRequest || db.DivisionPayPerGameCents(did) != 7550 {
		t.Fatalf("invalid rate must be rejected and change nothing: code=%d rate=%d", c, db.DivisionPayPerGameCents(did))
	}
	if c := post("-5"); c != http.StatusBadRequest {
		t.Fatalf("negative: got %d", c)
	}
	if c := post(""); c != http.StatusSeeOther || db.DivisionPayPerGameCents(did) != 0 {
		t.Fatalf("blank clears the rate: code=%d rate=%d", c, db.DivisionPayPerGameCents(did))
	}
}
