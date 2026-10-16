package webhandler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) ManageUmpires(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if t.Status == "draft" {
		http.Error(w, "The umpire list is available once the tournament is approved and published.", http.StatusForbidden)
		return
	}
	me.render(w, "manageUmpires", manageUmpiresData{
		baseData: newBaseWithTournament(r, t),
		Umpires:  me.DB.ListUmpires(),
		Verified: me.DB.VerifiedUmpireIDs(t.ID),
	})
}

func (me *Env) ManageUmpireVerify(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	uid, err := strconv.Atoi(ps.ByName("uid"))
	if err != nil {
		http.Error(w, "Bad user ID", http.StatusBadRequest)
		return
	}
	if _, isUmpire := me.DB.GetUmpire(uid); !isUmpire {
		http.Error(w, "Umpire not found", http.StatusNotFound)
		return
	}
	me.DB.SetUmpireVerified(t.ID, uid, userFromContext(r.Context()).ID, r.FormValue("verified") == "1")
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/umpires", t.ID), http.StatusSeeOther)
}
