package webhandler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func (me *Env) Teams(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
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
		me.DB.AddTeam(t.ID, divisionID, name, coach)
		http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/teams", t.ID), http.StatusSeeOther)
		return
	}
	divs := me.DB.ReturnDivisions(t.ID)
	byDiv := make(map[int][]mydb.Team)
	for _, div := range divs {
		byDiv[div.ID] = me.DB.ReturnTeamsByDivisionID(div.ID)
	}
	me.render(w, "adminTeams", adminTeamsData{
		baseData:        newBaseWithTournament(r, true, t),
		Divisions:       divs,
		TeamsByDivision: byDiv,
		DisableDelete:   me.DisableDelete,
	})
}

func (me *Env) DeleteTeam(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		log.Println("DeleteTeam bad ID:", err)
		http.Error(w, "Bad team ID", http.StatusBadRequest)
		return
	}
	if !me.DisableDelete {
		me.DB.DelTeam(teamID)
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/teams", t.ID), http.StatusSeeOther)
}

func (me *Env) EditTeam(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		log.Println("EditTeam bad ID:", err)
		http.Error(w, "Bad Team ID", http.StatusBadRequest)
		return
	}
	team := me.DB.ReturnTeamByID(teamID)
	if team.ID == 0 {
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
		http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/teams", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "editTeam", editTeamData{
		baseData:  newBaseWithTournament(r, true, t),
		Team:      team,
		Divisions: me.DB.ReturnDivisions(t.ID),
	})
}

func (me *Env) ShowTeam(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	tid, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		http.Error(w, "Bad Team ID", http.StatusBadRequest)
		return
	}
	team := me.DB.ReturnTeamByID(tid)
	if team.ID == 0 {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}
	me.render(w, "team", teamData{
		baseData: newBaseWithTournament(r, false, t),
		Team:     team,
		Games:    me.DB.AllGamesByTeam(tid),
	})
}
