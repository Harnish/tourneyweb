package webhandler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) AddDivisionForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
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
		baseData:      newBaseWithTournament(r, t),
		Divisions:     me.DB.ReturnDivisions(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) DeleteDivision(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		slog.Error("DeleteDivision bad ID", "err", err)
		http.Error(w, "Bad Division ID", http.StatusBadRequest)
		return
	}
	if !me.DisableDelete {
		me.DB.DelDivision(did)
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/divisions", t.ID), http.StatusSeeOther)
}

func (me *Env) AdminDivisionView(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		slog.Error("AdminDivisionView bad ID", "err", err)
		http.Error(w, "Bad Division ID", http.StatusBadRequest)
		return
	}
	me.render(w, "adminDivisionView", adminDivisionViewData{
		baseData:      newBaseWithTournament(r, t),
		Division:      me.DB.ReturnDivisionByID(did),
		DivisionID:    did,
		Teams:         me.DB.ReturnTeamsByDivisionIDWithStats(did),
		Games:         me.DB.AllGamesByDivision(did),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) EditDivision(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		slog.Error("EditDivision bad ID", "err", err)
		http.Error(w, "Bad Division ID", http.StatusBadRequest)
		return
	}
	division := me.DB.ReturnDivisionByID(did)
	if division.ID == 0 {
		http.Error(w, "Division not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		if name == "" {
			http.Error(w, "Division name required", http.StatusBadRequest)
			return
		}
		me.DB.UpdateDivision(did, name)
		http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/divisions/%d", t.ID, did), http.StatusSeeOther)
		return
	}
	me.render(w, "editDivision", editDivisionData{
		baseData: newBaseWithTournament(r, t),
		Division: division,
	})
}
