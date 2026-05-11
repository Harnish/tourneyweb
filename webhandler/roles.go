package webhandler

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// ManageRoles displays the role management page for a tournament.
// TODO: implement in Task 13.
func (me *Env) ManageRoles(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// AssignRole assigns a role to a user within a tournament.
// TODO: implement in Task 13.
func (me *Env) AssignRole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// InviteUser sends an invitation email to a user for a tournament.
// TODO: implement in Task 13.
func (me *Env) InviteUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// RemoveRole removes a user's role from a tournament.
// TODO: implement in Task 13.
func (me *Env) RemoveRole(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
