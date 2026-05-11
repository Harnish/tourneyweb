package webhandler

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

// Locations handles GET (list + add form) and POST (create) for /admin/locations.
func (me *Env) Locations(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		address := r.FormValue("address")
		lat, _ := strconv.ParseFloat(r.FormValue("latitude"), 64)
		lng, _ := strconv.ParseFloat(r.FormValue("longitude"), 64)
		if name != "" {
			me.DB.AddLocation(name, address, lat, lng)
		}
		http.Redirect(w, r, "/admin/locations", http.StatusSeeOther)
		return
	}
	locs := me.DB.GetLocations()
	j, _ := json.Marshal(locs)
	me.render(w, "adminLocations", adminLocationsData{
		baseData:      newBase(r),
		Locations:     locs,
		LocationsJSON: template.JS(j),
		DisableDelete: me.DisableDelete,
	})
}

// EditLocation handles GET (edit form) and POST (update) for /admin/locations/:lid/edit.
func (me *Env) EditLocation(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	lid, _ := strconv.Atoi(ps.ByName("lid"))
	loc := me.DB.GetLocationByID(lid)
	if loc.ID == 0 {
		me.renderError(w, r, http.StatusNotFound, "Not Found", "Location not found.")
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		address := r.FormValue("address")
		lat, _ := strconv.ParseFloat(r.FormValue("latitude"), 64)
		lng, _ := strconv.ParseFloat(r.FormValue("longitude"), 64)
		me.DB.UpdateLocation(lid, name, address, lat, lng)
		http.Redirect(w, r, "/admin/locations", http.StatusSeeOther)
		return
	}
	j, _ := json.Marshal(loc)
	me.render(w, "editLocation", editLocationData{
		baseData:     newBase(r),
		Location:     loc,
		LocationJSON: template.JS(j),
	})
}

// DeleteLocation handles POST /admin/locations/:lid/delete.
func (me *Env) DeleteLocation(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if me.DisableDelete {
		me.renderError(w, r, http.StatusForbidden, "Deletes Disabled", "Delete operations are disabled.")
		return
	}
	lid, _ := strconv.Atoi(ps.ByName("lid"))
	me.DB.DelLocation(lid)
	http.Redirect(w, r, "/admin/locations", http.StatusSeeOther)
}

// MapView handles GET /map — public view of all field locations.
func (me *Env) MapView(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	all := me.DB.GetLocations()
	var mapped []mydb.Location
	for _, l := range all {
		if l.Latitude != 0 || l.Longitude != 0 {
			mapped = append(mapped, l)
		}
	}
	j, _ := json.Marshal(mapped)
	me.render(w, "mapView", mapViewData{
		baseData:      newBase(r),
		Locations:     all,
		LocationsJSON: template.JS(j),
	})
}
