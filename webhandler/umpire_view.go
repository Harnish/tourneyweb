package webhandler

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
	"github.com/julienschmidt/httprouter"
)

type umpireRow struct {
	Game           mydb.Game
	TournamentName string
	Slot           int
	Status         string
	Partner        string
}

type umpireDay struct {
	Label string
	Rows  []umpireRow
}

type umpireSection struct {
	Days []umpireDay
	CSRF template.HTML
}

type umpireGamesData struct {
	baseData
	Upcoming umpireSection
	Past     umpireSection
}

func groupByDay(rows []umpireRow, desc bool) []umpireDay {
	sort.SliceStable(rows, func(i, j int) bool {
		if desc {
			return rows[i].Game.Start.After(rows[j].Game.Start)
		}
		return rows[i].Game.Start.Before(rows[j].Game.Start)
	})
	var days []umpireDay
	for _, r := range rows {
		label := "Date TBD"
		if !r.Game.Start.IsZero() {
			label = r.Game.Start.Format("Mon Jan 2, 2006")
		}
		if n := len(days); n > 0 && days[n-1].Label == label {
			days[n-1].Rows = append(days[n-1].Rows, r)
		} else {
			days = append(days, umpireDay{Label: label, Rows: []umpireRow{r}})
		}
	}
	return days
}

// UmpireGames lists the logged-in user's umpire assignments (declined ones are hidden).
func (me *Env) UmpireGames(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	var upcoming, past []umpireRow
	now := time.Now()
	for _, s := range me.DB.UmpireSlotsForUser(user.ID) {
		if s.Status == "declined" {
			continue
		}
		g := me.DB.ReturnGameByID(s.GameID)
		if g.ID == 0 {
			continue
		}
		var partners []string
		for _, o := range me.DB.GameUmpires(g.ID) {
			if o.Slot == s.Slot || o.Status == "declined" {
				continue
			}
			if o.UserID > 0 {
				partners = append(partners, o.UserName)
			} else {
				partners = append(partners, o.FreeName)
			}
		}
		row := umpireRow{Game: g, TournamentName: me.DB.ReturnTournamentByID(g.TournamentID).Name,
			Slot: s.Slot, Status: s.Status, Partner: strings.Join(partners, ", ")}
		if g.Start.IsZero() || !g.Start.Before(now) {
			upcoming = append(upcoming, row)
		} else {
			past = append(past, row)
		}
	}
	base := newBase(r)
	me.render(w, "umpireGames", umpireGamesData{
		baseData: base,
		Upcoming: umpireSection{Days: groupByDay(upcoming, false), CSRF: base.CSRFField},
		Past:     umpireSection{Days: groupByDay(past, true), CSRF: base.CSRFField},
	})
}

// UmpireRespond accepts or declines one of the user's own slots. A decline reopens the slot and emails the tournament's directors.
func (me *Env) UmpireRespond(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	gid, err1 := strconv.Atoi(ps.ByName("gid"))
	slot, err2 := strconv.Atoi(ps.ByName("slot"))
	action := r.FormValue("action")
	if err1 != nil || err2 != nil || (action != "accept" && action != "decline") {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if err := me.DB.RespondUmpireSlot(gid, slot, user.ID, action == "accept"); err != nil {
		if errors.Is(err, mydb.ErrUmpireSlotDeclined) {
			http.Error(w, "You already declined this game; ask the tournament director to reassign it.", http.StatusConflict)
			return
		}
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if action == "decline" {
		g := me.DB.ReturnGameByID(gid)
		link := fmt.Sprintf("/tournaments/%d/manage/games/%d/edit", g.TournamentID, g.ID)
		for _, id := range me.directorRecipients(g.TournamentID) {
			me.notifyUser(id, "Umpire declined a game", user.Name+" declined: "+gameLine(g)+". The slot is open again.", link)
		}
	}
	http.Redirect(w, r, "/umpire", http.StatusSeeOther)
}
