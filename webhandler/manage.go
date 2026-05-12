package webhandler

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) ManageDashboard(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	me.render(w, "manageDashboard", manageDashboardData{
		baseData: newBaseWithTournament(r, t),
		IsDraft:  t.Status == "draft",
	})
}
