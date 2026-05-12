package webhandler

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) TournamentExtras(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if t.ExtrasHTML == "" {
		http.NotFound(w, r)
		return
	}
	me.render(w, "tournamentExtras", newBaseWithTournament(r, t))
}

func (me *Env) ManageExtras(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		me.DB.SetTournamentExtras(t.ID, r.FormValue("extras_html"))
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/extras", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "editExtras", newBaseWithTournament(r, t))
}
