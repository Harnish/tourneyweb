package webhandler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func (me *Env) ManageTeams(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("teamname")
		coach := r.FormValue("teamcoach")
		divisionID, err := strconv.Atoi(r.FormValue("division"))
		if err != nil || name == "" {
			http.Error(w, "Name and valid division required", http.StatusBadRequest)
			return
		}
		div := me.DB.ReturnDivisionByID(divisionID)
		if div.ID == 0 || div.TournamentID != t.ID {
			http.Error(w, "Division does not belong to this tournament", http.StatusBadRequest)
			return
		}
		me.DB.AddTeam(t.ID, divisionID, name, coach)
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/teams", t.ID), http.StatusSeeOther)
		return
	}
	divs := me.DB.ReturnDivisions(t.ID)
	byDiv := make(map[int][]mydb.Team)
	for _, div := range divs {
		byDiv[div.ID] = me.DB.ReturnTeamsByDivisionID(div.ID)
	}
	me.render(w, "manageTeams", manageTeamsData{
		baseData:        newBaseWithTournament(r, t),
		Divisions:       divs,
		TeamsByDivision: byDiv,
		DisableDelete:   me.DisableDelete,
	})
}

func (me *Env) ManageTeamEdit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		slog.Error("ManageTeamEdit bad ID", "err", err)
		http.Error(w, "Bad Team ID", http.StatusBadRequest)
		return
	}
	team := me.DB.ReturnTeamByID(teamID)
	if team.ID == 0 || team.TournamentID != t.ID {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("teamname")
		coach := r.FormValue("teamcoach")
		divisionID, err := strconv.Atoi(r.FormValue("division"))
		if err != nil || name == "" {
			http.Error(w, "Name and valid division required", http.StatusBadRequest)
			return
		}
		div := me.DB.ReturnDivisionByID(divisionID)
		if div.ID == 0 || div.TournamentID != t.ID {
			http.Error(w, "Division does not belong to this tournament", http.StatusBadRequest)
			return
		}
		me.DB.UpdateTeam(teamID, divisionID, name, coach)
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/teams", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "manageTeamEdit", manageTeamEditData{
		baseData:  newBaseWithTournament(r, t),
		Team:      team,
		Divisions: me.DB.ReturnDivisions(t.ID),
	})
}

func (me *Env) ManageTeamDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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
	if !me.DisableDelete {
		me.DB.DelTeam(teamID)
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/teams", t.ID), http.StatusSeeOther)
}
