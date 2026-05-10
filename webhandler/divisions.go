package webhandler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) AddDivisionForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("divisionname")
		if name == "" {
			http.Error(w, "Division name required", http.StatusBadRequest)
			return
		}
		me.DB.AddDivision(t.ID, name)
		http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/divisions", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "adminDivisions", adminDivisionsData{
		baseData:      newBaseWithTournament(r, true, t),
		Divisions:     me.DB.ReturnDivisions(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) DeleteDivision(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		log.Println("DeleteDivision bad ID:", err)
		http.Error(w, "Bad Division ID", http.StatusBadRequest)
		return
	}
	if !me.DisableDelete {
		me.DB.DelDivision(did)
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/divisions", t.ID), http.StatusSeeOther)
}

func (me *Env) AdminDivisionView(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		log.Println("AdminDivisionView bad ID:", err)
		http.Error(w, "Bad Division ID", http.StatusBadRequest)
		return
	}
	me.render(w, "adminDivisionView", adminDivisionViewData{
		baseData:      newBaseWithTournament(r, true, t),
		Division:      me.DB.ReturnDivisionByID(did),
		DivisionID:    did,
		Teams:         me.DB.ReturnTeamsByDivisionIDWithStats(did),
		Games:         me.DB.AllGamesByDivision(did),
		DisableDelete: me.DisableDelete,
	})
}
