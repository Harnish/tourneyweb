package webhandler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Harnish/tourneyweb/mydb"
	"github.com/julienschmidt/httprouter"
)

type payLine struct {
	DivisionID  int
	Division    string
	Games       int
	RateCents   int
	AmountCents int
	HasRate     bool
	RateText    string
	AmountText  string
}

type payGame struct {
	Game   mydb.Game
	Slot   int
	Absent bool
}

type payUmpire struct {
	Name         string
	UserID       int
	Lines        []payLine
	Games        []payGame
	TotalCents   int
	TotalText    string
	UnratedGames int
}

var centsRe = regexp.MustCompile(`^\d{1,9}(\.\d{1,2})?$`)

// parseCents reads a dollar amount like "75", "75.5" or "$75.50" into cents. Blank means 0 (no rate).
func parseCents(s string) (int, error) {
	s = strings.TrimSpace(s)
	hadDollar := strings.HasPrefix(s, "$")
	s = strings.TrimSpace(strings.TrimPrefix(s, "$"))
	if s == "" {
		if hadDollar {
			return 0, fmt.Errorf("amount missing")
		}
		return 0, nil
	}
	if !centsRe.MatchString(s) {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	whole, frac, _ := strings.Cut(s, ".")
	w, _ := strconv.Atoi(whole)
	f := 0
	if frac != "" {
		f, _ = strconv.Atoi((frac + "0")[:2])
	}
	return w*100 + f, nil
}

func formatCents(c int) string { return fmt.Sprintf("$%d.%02d", c/100, c%100) }

func payText(c int) string {
	if c <= 0 {
		return ""
	}
	return plainCents(c)
}

func plainCents(c int) string { return fmt.Sprintf("%d.%02d", c/100, c%100) }

// buildPayReport counts, per umpire, every slot in the tournament that is not declined and not marked absent.
// Pending and accepted slots both count, as do free-form names (grouped by name, ignoring case and spacing).
func (me *Env) buildPayReport(tournamentID int) []payUmpire {
	games := map[int]mydb.Game{}
	for _, g := range me.DB.AllGames(tournamentID) {
		games[g.ID] = g
	}
	rates := map[int]int{}
	byKey := map[string]*payUmpire{}
	counts := map[string]map[int]int{} // key -> division -> games
	var keys []string
	for _, s := range me.DB.UmpireSlotsForTournament(tournamentID) {
		g, ok := games[s.GameID]
		if !ok || s.Status == "declined" {
			continue
		}
		key, name := "", ""
		if s.UserID > 0 {
			key, name = "u"+strconv.Itoa(s.UserID), s.UserName
		} else {
			name = strings.Join(strings.Fields(s.FreeName), " ")
			key = "f:" + strings.ToLower(name)
		}
		u := byKey[key]
		if u == nil {
			u = &payUmpire{Name: name, UserID: s.UserID}
			byKey[key] = u
			counts[key] = map[int]int{}
			keys = append(keys, key)
		}
		u.Games = append(u.Games, payGame{Game: g, Slot: s.Slot, Absent: s.Absent})
		if !s.Absent {
			counts[key][g.Division.ID]++
		}
	}
	var out []payUmpire
	for _, key := range keys {
		u := byKey[key]
		for did, n := range counts[key] {
			rate, seen := rates[did]
			if !seen {
				rate = me.DB.DivisionPayPerGameCents(did)
				rates[did] = rate
			}
			l := payLine{DivisionID: did, Games: n, RateCents: rate, HasRate: rate > 0}
			for _, g := range u.Games {
				if g.Game.Division.ID == did {
					l.Division = g.Game.Division.Name
					break
				}
			}
			if l.HasRate {
				l.AmountCents = n * rate
				l.RateText, l.AmountText = formatCents(rate), formatCents(l.AmountCents)
				u.TotalCents += l.AmountCents
			} else {
				u.UnratedGames += n
			}
			u.Lines = append(u.Lines, l)
		}
		u.TotalText = formatCents(u.TotalCents)
		sort.Slice(u.Lines, func(i, j int) bool { return u.Lines[i].Division < u.Lines[j].Division })
		sort.SliceStable(u.Games, func(i, j int) bool { return u.Games[i].Game.Start.Before(u.Games[j].Game.Start) })
		out = append(out, *u)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

type manageUmpirePayData struct {
	baseData
	Report []payUmpire
}

func (me *Env) ManageUmpirePay(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	me.render(w, "manageUmpirePay", manageUmpirePayData{baseData: newBaseWithTournament(r, t), Report: me.buildPayReport(t.ID)})
}

// csvSafe stops spreadsheets from running a cell as a formula.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func (me *Env) ManageUmpirePayCSV(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="umpire-pay-%d.csv"`, t.ID))
	cw := csv.NewWriter(w)
	cw.Write([]string{"Umpire", "Division", "Games", "Rate", "Amount"})
	for _, u := range me.buildPayReport(t.ID) {
		for _, l := range u.Lines {
			rate, amount := "", ""
			if l.HasRate {
				rate, amount = plainCents(l.RateCents), plainCents(l.AmountCents)
			}
			cw.Write([]string{csvSafe(u.Name), csvSafe(l.Division), strconv.Itoa(l.Games), rate, amount})
		}
	}
	cw.Flush()
}

// ManageUmpireAbsent marks (or restores) one umpire slot as a no-show so it isn't paid.
func (me *Env) ManageUmpireAbsent(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	gid, err1 := strconv.Atoi(ps.ByName("gid"))
	slot, err2 := strconv.Atoi(ps.ByName("slot"))
	if err1 != nil || err2 != nil {
		http.NotFound(w, r)
		return
	}
	if g := me.DB.ReturnGameByID(gid); g.ID == 0 || g.TournamentID != t.ID {
		http.NotFound(w, r)
		return
	}
	found := false
	for _, s := range me.DB.GameUmpires(gid) {
		found = found || s.Slot == slot
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	me.DB.SetUmpireSlotAbsent(gid, slot, r.FormValue("absent") == "1")
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/umpire-pay", t.ID), http.StatusSeeOther)
}
