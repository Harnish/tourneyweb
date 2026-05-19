package webhandler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
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

	if r.Method == http.MethodPost {
		if t.Status == "published" && !user.IsAdmin {
			http.Error(w, "Ranking order is locked after publishing", http.StatusBadRequest)
			return
		}
		criteriaStr := r.FormValue("default_ranking_criteria")
		var criteria []string
		for _, k := range strings.Split(criteriaStr, ",") {
			k = strings.TrimSpace(k)
			if _, ok := criteriaRegistry[k]; ok {
				criteria = append(criteria, k)
			}
		}
		if len(criteria) == 0 {
			criteria = mydb.DefaultRankingCriteria
		}
		me.DB.SetTournamentDefaultRanking(t.ID, criteria)
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage", t.ID), http.StatusSeeOther)
		return
	}

	divisions := me.DB.ReturnDivisions(t.ID)
	active := t.DefaultRankingCriteria
	if len(active) == 0 {
		active = mydb.DefaultRankingCriteria
	}
	me.render(w, "manageDashboard", manageDashboardData{
		baseData:     newBaseWithTournament(r, t),
		IsDraft:      t.Status == "draft",
		AllCriteria:  AllCriteriaForUI(active),
		HasDivisions: len(divisions) > 0,
	})
}
