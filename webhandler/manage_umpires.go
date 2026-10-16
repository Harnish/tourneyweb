package webhandler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
	"github.com/julienschmidt/httprouter"
)

type manageUmpireRow struct {
	mydb.Umpire
	Verified bool
	Flagged  bool
	Expired  bool
	Relinked bool // the umpire changed their certification after the director flagged it
}

func (me *Env) ManageUmpires(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if t.Status == "draft" {
		http.Error(w, "The umpire list is available once the tournament is approved and published.", http.StatusForbidden)
		return
	}
	verified, flagged, flagTimes, now := me.DB.VerifiedUmpireIDs(t.ID), me.DB.ExpiredFlagUmpireIDs(t.ID), me.DB.ExpiredFlagTimes(t.ID), time.Now()
	var rows []manageUmpireRow
	for _, u := range me.DB.ListUmpires() {
		rows = append(rows, manageUmpireRow{Umpire: u, Verified: verified[u.UserID], Flagged: flagged[u.UserID],
			Expired: u.Expired(flagged[u.UserID], now), Relinked: flagged[u.UserID] && u.LinkedAt.After(flagTimes[u.UserID])})
	}
	me.render(w, "manageUmpires", manageUmpiresData{baseData: newBaseWithTournament(r, t), Rows: rows})
}

// umpireTarget resolves the tournament and the flagged-umpire user for the verify/expire POSTs.
func (me *Env) umpireTarget(w http.ResponseWriter, r *http.Request, ps httprouter.Params, what string) (mydb.Tournament, int, bool) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return t, 0, false
	}
	if t.Status == "draft" {
		http.Error(w, "Umpires can be "+what+" once the tournament is approved and published.", http.StatusForbidden)
		return t, 0, false
	}
	uid, err := strconv.Atoi(ps.ByName("uid"))
	if err != nil {
		http.Error(w, "Bad user ID", http.StatusBadRequest)
		return t, 0, false
	}
	if _, isUmpire := me.DB.GetUmpire(uid); !isUmpire {
		http.Error(w, "Umpire not found", http.StatusNotFound)
		return t, 0, false
	}
	return t, uid, true
}

func (me *Env) ManageUmpireVerify(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, uid, ok := me.umpireTarget(w, r, ps, "verified")
	if !ok {
		return
	}
	me.DB.SetUmpireVerified(t.ID, uid, userFromContext(r.Context()).ID, r.FormValue("verified") == "1")
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/umpires", t.ID), http.StatusSeeOther)
}

// ManageUmpireExpire flags (or clears) an umpire's certification as expired for this tournament.
func (me *Env) ManageUmpireExpire(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, uid, ok := me.umpireTarget(w, r, ps, "flagged as expired")
	if !ok {
		return
	}
	me.DB.SetUmpireExpiredFlag(t.ID, uid, userFromContext(r.Context()).ID, r.FormValue("expired") == "1")
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/umpires", t.ID), http.StatusSeeOther)
}
