package webhandler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Harnish/tourneyweb/mydb"
	"github.com/julienschmidt/httprouter"
)

// extraRoleTarget resolves the tournament and one of its extra roles; a role of another tournament is a 404.
func (me *Env) extraRoleTarget(w http.ResponseWriter, r *http.Request, ps httprouter.Params) (mydb.Tournament, mydb.ExtraRole, bool) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return t, mydb.ExtraRole{}, false
	}
	rid, err := strconv.Atoi(ps.ByName("rid"))
	if err != nil {
		http.Error(w, "Bad role ID", http.StatusBadRequest)
		return t, mydb.ExtraRole{}, false
	}
	role := me.DB.ExtraRoleByID(rid)
	if role.ID == 0 || role.TournamentID != t.ID {
		http.Error(w, "Role not found", http.StatusNotFound)
		return t, role, false
	}
	return t, role, true
}

func (me *Env) ManageExtraRoles(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	me.render(w, "manageExtraRoles", manageExtraRolesData{baseData: newBaseWithTournament(r, t), Roles: me.DB.TournamentExtraRoles(t.ID), DisableDelete: me.DisableDelete})
}

// roleNameError maps a role-name error to an HTTP status.
func roleNameError(w http.ResponseWriter, err error) {
	if errors.Is(err, mydb.ErrRoleNameTaken) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if errors.Is(err, mydb.ErrRoleNameInvalid) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Error(w, "Could not save the role", http.StatusInternalServerError)
}

func (me *Env) ManageExtraRoleCreate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if _, err := me.DB.AddExtraRole(t.ID, r.FormValue("name")); err != nil {
		roleNameError(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/extra-roles", t.ID), http.StatusSeeOther)
}

func (me *Env) ManageExtraRoleRename(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, role, ok := me.extraRoleTarget(w, r, ps)
	if !ok {
		return
	}
	if err := me.DB.RenameExtraRole(t.ID, role.ID, r.FormValue("name")); err != nil {
		roleNameError(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/extra-roles", t.ID), http.StatusSeeOther)
}

// ManageExtraRoleDelete removes a role with its counts and slots, telling registered assignees first.
func (me *Env) ManageExtraRoleDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, role, ok := me.extraRoleTarget(w, r, ps)
	if !ok {
		return
	}
	if !me.DisableDelete {
		for _, g := range me.DB.AllGames(t.ID) {
			for _, s := range me.DB.GameRoleSlots(g.ID) {
				if s.RoleID == role.ID && s.UserID > 0 && s.Status != "declined" {
					me.notifyUser(s.UserID, "Role removed", "The role "+role.Name+" was removed, so you are no longer assigned to: "+gameLine(g), "/umpire")
				}
			}
		}
		me.DB.DelExtraRole(t.ID, role.ID)
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/extra-roles", t.ID), http.StatusSeeOther)
}

type roleNeedRow struct {
	Role mydb.ExtraRole
	N    int
}

// applyRoleNeeds saves role_need_<id> inputs for this tournament's roles; absent inputs are left alone.
func (me *Env) applyRoleNeeds(r *http.Request, t mydb.Tournament, divisionID int) {
	for _, role := range me.DB.TournamentExtraRoles(t.ID) {
		if n, err := strconv.Atoi(r.FormValue("role_need_" + strconv.Itoa(role.ID))); err == nil {
			me.DB.SetDivisionRoleNeeded(divisionID, role.ID, n)
		}
	}
}

func (me *Env) roleNeedRows(t mydb.Tournament, divisionID int) []roleNeedRow {
	needs := me.DB.DivisionRoleNeeds(divisionID)
	var rows []roleNeedRow
	for _, role := range me.DB.TournamentExtraRoles(t.ID) {
		rows = append(rows, roleNeedRow{Role: role, N: needs[role.ID]})
	}
	return rows
}
