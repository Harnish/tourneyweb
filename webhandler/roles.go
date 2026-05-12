package webhandler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func (me *Env) ManageRoles(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	roles := me.DB.GetRolesForTournament(t.ID)
	users := make(map[int]mydb.User)
	for _, role := range roles {
		u, err := me.DB.GetUserByID(role.UserID)
		if err == nil {
			users[role.UserID] = u
		}
	}
	me.render(w, "manageRoles", manageRolesData{
		baseData: newBaseWithTournament(r, t),
		Roles:    roles,
		Users:    users,
		Teams:    me.DB.ReturnTeamsByTournamentID(t.ID),
	})
}

func (me *Env) AssignRole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	email := r.FormValue("email")
	role := r.FormValue("role")
	teamID, _ := strconv.Atoi(r.FormValue("team_id"))

	if role != "director" && role != "staff" && role != "coach" {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}
	if role == "coach" && teamID == 0 {
		http.Error(w, "Coach role requires selecting a team", http.StatusBadRequest)
		return
	}
	user, err := me.DB.GetUserByEmail(email)
	if err != nil || user.ID == 0 {
		http.Error(w, "No registered user found with that email", http.StatusBadRequest)
		return
	}
	me.DB.AssignRole(user.ID, t.ID, role, teamID)
	http.Redirect(w, r, "/tournaments/"+ps.ByName("tid")+"/manage/roles", http.StatusSeeOther)
}

func (me *Env) InviteUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	email := r.FormValue("email")
	role := r.FormValue("role")
	teamID, _ := strconv.Atoi(r.FormValue("team_id"))

	if role != "director" && role != "staff" && role != "coach" {
		http.Error(w, "Invalid role", http.StatusBadRequest)
		return
	}
	token, err := GenerateToken()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if err := me.DB.CreateInvitation(email, t.ID, role, teamID, token, time.Now().Add(7*24*time.Hour)); err != nil {
		http.Error(w, "Could not create invitation", http.StatusInternalServerError)
		return
	}
	if me.Email != nil {
		go func() { me.Email.SendInvitationEmail(email, t.Name, role) }()
	}
	http.Redirect(w, r, "/tournaments/"+ps.ByName("tid")+"/manage/roles", http.StatusSeeOther)
}

func (me *Env) RemoveRole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	uid, err := strconv.Atoi(ps.ByName("uid"))
	if err != nil {
		http.Error(w, "Bad user ID", http.StatusBadRequest)
		return
	}
	me.DB.RemoveRole(uid, t.ID)
	http.Redirect(w, r, "/tournaments/"+ps.ByName("tid")+"/manage/roles", http.StatusSeeOther)
}
