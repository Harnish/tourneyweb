package webhandler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Harnish/tourneyweb/mydb"
	"github.com/julienschmidt/httprouter"
)

// RoleRespond accepts or declines one of the user's own extra-role slots. A decline reopens the slot and notifies the tournament's directors.
func (me *Env) RoleRespond(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	gid, err1 := strconv.Atoi(ps.ByName("gid"))
	rid, err2 := strconv.Atoi(ps.ByName("rid"))
	slot, err3 := strconv.Atoi(ps.ByName("slot"))
	action := r.FormValue("action")
	if err1 != nil || err2 != nil || err3 != nil || (action != "accept" && action != "decline") {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if err := me.DB.RespondRoleSlot(gid, rid, slot, user.ID, action == "accept"); err != nil {
		if errors.Is(err, mydb.ErrUmpireSlotDeclined) {
			http.Error(w, "You already declined this; ask the tournament director to reassign it.", http.StatusConflict)
			return
		}
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if action == "decline" {
		g := me.DB.ReturnGameByID(gid)
		link := fmt.Sprintf("/tournaments/%d/manage/games/%d/edit", g.TournamentID, g.ID)
		for _, id := range me.directorRecipients(g.TournamentID) {
			me.notifyUser(id, "Role declined", user.Name+" declined "+me.DB.ExtraRoleByID(rid).Name+": "+gameLine(g)+". The slot is open again.", link)
		}
	}
	http.Redirect(w, r, "/umpire", http.StatusSeeOther)
}
