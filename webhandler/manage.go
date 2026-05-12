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
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !user.CanManage(t.ID) {
		me.renderError(w, r, http.StatusForbidden, "Not Authorized", "You must be a tournament director to access this page.")
		return
	}
	me.render(w, "manageDashboard", manageDashboardData{
		baseData: newBaseWithTournament(r, t),
		IsDraft:  t.Status == "draft",
	})
}
