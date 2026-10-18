package webhandler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Harnish/tourneyweb/mydb"

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
	// Followers of the tournament or of the game's level come first; everyone else follows, alphabetically.
	// The division only counts if it belongs to this tournament.
	q := r.URL.Query().Get("q")
	did, _ := strconv.Atoi(r.URL.Query().Get("division"))
	if d := me.DB.ReturnDivisionByID(did); d.ID == 0 || d.TournamentID != t.ID {
		did = 0
	}
	hits := []hit{}
	seen := map[int]bool{}
	add := func(users []mydb.User) {
		for _, u := range users {
			if len(hits) < 10 && !seen[u.ID] {
				seen[u.ID] = true
				hits = append(hits, hit{ID: u.ID, Name: u.Name})
			}
		}
	}
	add(me.DB.FollowersMatching(q, t.ID, did, 10))
	add(me.DB.SearchUsers(q, 10+len(hits)))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	json.NewEncoder(w).Encode(hits)
}
