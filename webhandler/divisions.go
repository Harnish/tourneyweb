package webhandler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) AddDivisionForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	divisionname := r.FormValue("divisionname")
	if divisionname != "" {
		me.DB.AddDivision(divisionname)
	}
	divisionid := r.FormValue("divisionid")
	if divisionid != "" {
		log.Println("Deleting", divisionid)
		did, err := strconv.Atoi(divisionid)
		if err != nil {
			log.Println("Bad ID", err)
		} else if !me.DisableDelete {
			me.DB.DelDivision(did)
		}
	}
	me.render(w, "adminDivisions", adminDivisionsData{
		baseData:      newBase(r, true),
		Divisions:     me.DB.ReturnDivisions(),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) AdminDivisionView(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	divisionid := ps.ByName("divisionid")
	did, err := strconv.Atoi(divisionid)
	if err != nil {
		log.Println("Bad ID", err)
		http.Error(w, "Bad Division ID", http.StatusBadRequest)
		return
	}
	me.render(w, "adminDivisionView", adminDivisionViewData{
		baseData:      newBase(r, true),
		Division:      me.DB.ReturnDivisionByID(did),
		DivisionID:    did,
		Teams:         me.DB.ReturnTeamsByDivisionIDWithStats(did),
		Games:         me.DB.AllGamesByDivision(did),
		DisableDelete: me.DisableDelete,
	})
}
