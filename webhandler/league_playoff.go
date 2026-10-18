package webhandler

import (
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/julienschmidt/httprouter"
)

// CreatePlayoff copies a league's levels and teams, in standings order, into a new linked tournament.
// Only site admins and the league's directors may do it (staff reach /manage but not this).
// Games, scores, news, assignments and every role except directors are left behind.
func (me *Env) CreatePlayoff(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !canOverrideRoster(t, user) { // admin or this league's director
		http.Error(w, "Only the league's directors can create a playoff tournament", http.StatusForbidden)
		return
	}
	if !t.IsLeague() {
		http.Error(w, "Playoffs can only be created from a league", http.StatusBadRequest)
		return
	}
	start := t.EndDate
	if start.IsZero() {
		n := time.Now()
		start = time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	}
	status := "draft"
	if user.IsAdmin {
		status = "published"
	}
	id := me.DB.AddTournament(t.Name+" Playoffs", t.Sport, t.Location, "", start, status)
	if id == 0 {
		http.Error(w, "Could not create the playoff tournament", http.StatusInternalServerError)
		return
	}
	me.DB.SetTournamentLeague(id, "tournament", time.Time{}, t.ID)
	if len(t.DefaultRankingCriteria) > 0 {
		me.DB.SetTournamentDefaultRanking(id, t.DefaultRankingCriteria)
	}
	me.DB.AssignRole(user.ID, id, "director", 0)
	for _, role := range me.DB.GetRolesForTournament(t.ID) {
		if role.Role == "director" && role.UserID != user.ID {
			me.DB.AssignRole(role.UserID, id, "director", 0)
		}
	}
	for _, d := range me.DB.ReturnDivisions(t.ID) {
		nd := me.DB.AddDivision(id, d.Name)
		if nd == 0 {
			slog.Error("CreatePlayoff: division", "league", t.ID, "name", d.Name)
			continue
		}
		me.DB.UpdateDivision(nd, d.Name, d.RankingCriteria)
		teams := me.DB.ReturnTeamsByDivisionIDWithStats(d.ID)
		sort.SliceStable(teams, func(i, j int) bool { return teams[i].Name < teams[j].Name }) // ties and unscored levels stay alphabetical
		for _, tm := range me.SortTeams(teams, d.RankingCriteria) {
			me.DB.AddTeam(id, nd, tm.Name, tm.Coach)
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage", id), http.StatusSeeOther)
}
