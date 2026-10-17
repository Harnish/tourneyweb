package webhandler

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// UserSearch returns up to 10 registered users (id and name only) matching ?q= for the role-slot picker.
// The /manage prefix middleware already limits it to staff; the check here keeps it safe if the route moves.
func (me *Env) UserSearch(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	user := userFromContext(r.Context())
	if !user.IsAdmin && !user.IsDirectorFor(t.ID) && !user.IsStaffFor(t.ID) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if t.Status == "draft" { // registered users can't be assigned in a draft, so there is nothing to search for
		http.Error(w, "User search is available once the tournament is approved and published.", http.StatusForbidden)
		return
	}
	type hit struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	hits := []hit{}
	for _, u := range me.DB.SearchUsers(r.URL.Query().Get("q"), 10) {
		hits = append(hits, hit{ID: u.ID, Name: u.Name})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	json.NewEncoder(w).Encode(hits)
}
