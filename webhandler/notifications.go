package webhandler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

const inboxLimit = 100

func (me *Env) Notifications(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	me.render(w, "notifications", notificationsData{
		baseData: newBase(r),
		Items:    me.DB.ListNotifications(user.ID, inboxLimit),
	})
}

// NotificationOpen marks one of the user's notifications read and sends them to its link.
func (me *Env) NotificationOpen(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	id, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	n, ok := me.DB.MarkNotificationRead(id, user.ID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	dest := n.Link
	if !siteRelative(dest) {
		dest = "/notifications"
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

func (me *Env) NotificationsReadAll(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	me.DB.MarkAllNotificationsRead(user.ID)
	http.Redirect(w, r, "/notifications", http.StatusSeeOther)
}

// siteRelative accepts only plain same-site paths: no scheme or host, no "//host", no backslash tricks.
func siteRelative(dest string) bool {
	if !strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, "//") || strings.Contains(dest, "\\") {
		return false
	}
	u, err := url.Parse(dest)
	return err == nil && u.Scheme == "" && u.Host == ""
}
