package webhandler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) TournamentQueue(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "adminQueue", adminQueueData{
		baseData: newBase(r),
		Drafts:   me.DB.ReturnDraftTournaments(),
	})
}

func (me *Env) IssueCode(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tid, err := strconv.Atoi(ps.ByName("tid"))
	if err != nil {
		http.Error(w, "Bad tournament ID", http.StatusBadRequest)
		return
	}
	t := me.DB.ReturnTournamentByID(tid)
	if t.ID == 0 {
		http.Error(w, "Tournament not found", http.StatusNotFound)
		return
	}
	code, err := me.DB.IssueVerificationCode(tid)
	if err != nil {
		http.Error(w, "Could not issue code", http.StatusInternalServerError)
		return
	}
	me.render(w, "adminQueue", adminQueueData{
		baseData:   newBase(r),
		Drafts:     me.DB.ReturnDraftTournaments(),
		IssuedCode: code,
		IssuedFor:  t.Name,
	})
}

func (me *Env) ManagePublish(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if t.Status == "published" {
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage", t.ID), http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		code := r.FormValue("code")
		if err := me.DB.RedeemVerificationCode(code, t.ID); err != nil {
			me.render(w, "managePublish", managePublishData{
				baseData: newBaseWithTournament(r, t),
				Error:    "Invalid or already-used code.",
			})
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "managePublish", managePublishData{
		baseData: newBaseWithTournament(r, t),
	})
}
