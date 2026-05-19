package webhandler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) rosterAccess(w http.ResponseWriter, r *http.Request, tid, teamID int) bool {
	user := userFromContext(r.Context())
	if user.IsAdmin || user.IsDirectorFor(tid) || user.IsCoachForTeam(teamID) {
		return true
	}
	me.renderError(w, r, http.StatusForbidden, "Not Authorized", "You must be the team coach, a director, or an admin to manage this roster.")
	return false
}

func (me *Env) RosterList(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		http.Error(w, "Bad team ID", http.StatusBadRequest)
		return
	}
	team := me.DB.ReturnTeamByID(teamID)
	if team.ID == 0 || team.TournamentID != t.ID {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}
	if !me.rosterAccess(w, r, t.ID, teamID) {
		return
	}
	me.render(w, "roster", rosterData{
		baseData:      newBaseWithTournament(r, t),
		Team:          team,
		Players:       me.DB.GetPlayersByTeamID(teamID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) RosterAdd(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		http.Error(w, "Bad team ID", http.StatusBadRequest)
		return
	}
	team := me.DB.ReturnTeamByID(teamID)
	if team.ID == 0 || team.TournamentID != t.ID {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}
	if !me.rosterAccess(w, r, t.ID, teamID) {
		return
	}
	if r.Method == http.MethodPost {
		first := r.FormValue("first")
		last := r.FormValue("last")
		if first == "" || last == "" {
			http.Error(w, "First and last name required", http.StatusBadRequest)
			return
		}
		me.DB.AddPlayer(teamID, r.FormValue("number"), first, last, r.FormValue("handed"), r.FormValue("position"))
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/teams/%d/roster", t.ID, teamID), http.StatusSeeOther)
		return
	}
	me.render(w, "rosterEdit", rosterEditData{
		baseData: newBaseWithTournament(r, t),
		Team:     team,
		IsNew:    true,
	})
}

func (me *Env) RosterEdit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		http.Error(w, "Bad team ID", http.StatusBadRequest)
		return
	}
	team := me.DB.ReturnTeamByID(teamID)
	if team.ID == 0 || team.TournamentID != t.ID {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}
	if !me.rosterAccess(w, r, t.ID, teamID) {
		return
	}
	pid, err := strconv.Atoi(ps.ByName("pid"))
	if err != nil {
		http.Error(w, "Bad player ID", http.StatusBadRequest)
		return
	}
	player, found := me.DB.GetPlayerByID(pid)
	if !found || player.TeamID != teamID {
		http.Error(w, "Player not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPost {
		first := r.FormValue("first")
		last := r.FormValue("last")
		if first == "" || last == "" {
			http.Error(w, "First and last name required", http.StatusBadRequest)
			return
		}
		me.DB.UpdatePlayer(pid, r.FormValue("number"), first, last, r.FormValue("handed"), r.FormValue("position"))
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/teams/%d/roster", t.ID, teamID), http.StatusSeeOther)
		return
	}
	me.render(w, "rosterEdit", rosterEditData{
		baseData: newBaseWithTournament(r, t),
		Team:     team,
		Player:   player,
		IsNew:    false,
	})
}

func (me *Env) RosterDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		http.Error(w, "Bad team ID", http.StatusBadRequest)
		return
	}
	team := me.DB.ReturnTeamByID(teamID)
	if team.ID == 0 || team.TournamentID != t.ID {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}
	if !me.rosterAccess(w, r, t.ID, teamID) {
		return
	}
	pid, err := strconv.Atoi(ps.ByName("pid"))
	if err != nil {
		http.Error(w, "Bad player ID", http.StatusBadRequest)
		return
	}
	player, found := me.DB.GetPlayerByID(pid)
	if !found || player.TeamID != teamID {
		http.Error(w, "Player not found", http.StatusNotFound)
		return
	}
	me.DB.DeletePlayer(pid)
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/teams/%d/roster", t.ID, teamID), http.StatusSeeOther)
}
