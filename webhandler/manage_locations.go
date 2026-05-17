package webhandler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) ManageLocations(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		address := r.FormValue("address")
		lat, _ := strconv.ParseFloat(r.FormValue("latitude"), 64)
		lng, _ := strconv.ParseFloat(r.FormValue("longitude"), 64)
		if name != "" {
			me.DB.AddLocation(name, address, "", lat, lng, t.ID)
		}
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/locations", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "manageLocations", manageLocationsData{
		baseData:      newBaseWithTournament(r, t),
		Locations:     me.DB.GetLocationsByTournamentID(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) ManageLocationEdit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	lid, _ := strconv.Atoi(ps.ByName("lid"))
	loc := me.DB.GetLocationByID(lid)
	if loc.ID == 0 || loc.TournamentID != t.ID {
		me.renderError(w, r, http.StatusNotFound, "Not Found", "Location not found.")
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		address := r.FormValue("address")
		lat, _ := strconv.ParseFloat(r.FormValue("latitude"), 64)
		lng, _ := strconv.ParseFloat(r.FormValue("longitude"), 64)
		me.DB.UpdateLocation(lid, name, address, "", lat, lng)
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/locations", t.ID), http.StatusSeeOther)
		return
	}
	j, _ := json.Marshal(loc)
	me.render(w, "manageLocationEdit", manageLocationEditData{
		baseData:     newBaseWithTournament(r, t),
		Location:     loc,
		LocationJSON: template.JS(j),
	})
}

func (me *Env) ManageLocationDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if me.DisableDelete {
		me.renderError(w, r, http.StatusForbidden, "Deletes Disabled", "Delete operations are disabled.")
		return
	}
	lid, _ := strconv.Atoi(ps.ByName("lid"))
	loc := me.DB.GetLocationByID(lid)
	if loc.ID == 0 || loc.TournamentID != t.ID {
		me.renderError(w, r, http.StatusForbidden, "Forbidden", "That location does not belong to this tournament.")
		return
	}
	me.DB.DelLocation(lid)
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/locations", t.ID), http.StatusSeeOther)
}
