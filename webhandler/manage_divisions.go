package webhandler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) ManageDivisions(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "manageDivisions", manageDivisionsData{
		baseData:      newBaseWithTournament(r, t),
		Divisions:     me.DB.ReturnDivisions(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) ManageDivisionEdit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		slog.Error("ManageDivisionEdit bad ID", "err", err)
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
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "manageDivisionEdit", manageDivisionEditData{
		baseData: newBaseWithTournament(r, t),
		Division: division,
	})
}

func (me *Env) ManageDivisionDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad Division ID", http.StatusBadRequest)
		return
	}
	if !me.DisableDelete {
		me.DB.DelDivision(did)
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
}
