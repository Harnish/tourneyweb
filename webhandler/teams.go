package webhandler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func (me *Env) Teams(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	teamname := r.FormValue("teamname")
	teamcoach := r.FormValue("teamcoach")
	teamdivision, err1 := strconv.Atoi(r.FormValue("division"))
	if teamname != "" && err1 == nil {
		me.DB.AddTeam(teamname, teamcoach, teamdivision)
	}
	teamid := r.FormValue("teamid")
	if teamid != "" {
		log.Println("Deleting teamid", teamid)
		did, err := strconv.Atoi(teamid)
		if err != nil {
			log.Println("Bad ID", err)
		} else if !me.DisableDelete {
			me.DB.DelTeam(did)
		}
	}

	divs := me.DB.ReturnDivisions()
	byDiv := make(map[int][]mydb.Team)
	for _, div := range divs {
		byDiv[div.ID] = me.DB.ReturnTeamsByDivisionID(div.ID)
	}
	me.render(w, "adminTeams", adminTeamsData{
		baseData:        newBase(r, true),
		Divisions:       divs,
		TeamsByDivision: byDiv,
		DisableDelete:   me.DisableDelete,
	})
}

func (me *Env) ShowTeam(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tid, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		http.Error(w, "Bad Team ID", http.StatusBadRequest)
		return
	}
	me.render(w, "team", teamData{
		baseData: newBase(r, false),
		Team:     me.DB.ReturnTeamByID(tid),
		Games:    me.DB.AllGamesByTeam(tid),
	})
}
