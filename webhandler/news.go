package webhandler

import (
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

// AdminNews handles GET (list+form) and POST (create) for /admin/news.
func (me *Env) AdminNews(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	if r.Method == http.MethodPost {
		title := r.FormValue("title")
		body := r.FormValue("news_html")
		user := userFromContext(r.Context())
		if title != "" && body != "" {
			me.DB.AddNews(0, title, body, user.ID)
		}
		http.Redirect(w, r, "/admin/news", http.StatusSeeOther)
		return
	}
	me.render(w, "adminNews", newsListData{
		baseData:      newBase(r),
		News:          me.DB.GetSiteNews(),
		DisableDelete: me.DisableDelete,
	})
}

// AdminNewsEdit handles GET (form) and POST (update) for /admin/news/:nid/edit.
func (me *Env) AdminNewsEdit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	nid, err := strconv.Atoi(ps.ByName("nid"))
	if err != nil {
		me.renderError(w, r, http.StatusBadRequest, "Bad Request", "Invalid news ID.")
		return
	}
	item, ok := me.DB.GetNewsByID(nid)
	if !ok || item.TournamentID != 0 {
		me.renderError(w, r, http.StatusNotFound, "Not Found", "News item not found.")
		return
	}
	if r.Method == http.MethodPost {
		title := r.FormValue("title")
		body := r.FormValue("news_html")
		if title != "" && body != "" {
			me.DB.UpdateNews(nid, title, body)
		}
		http.Redirect(w, r, "/admin/news", http.StatusSeeOther)
		return
	}
	me.render(w, "adminNewsEdit", newsEditData{
		baseData: newBase(r),
		Item:     item,
	})
}

// AdminNewsDelete handles POST /admin/news/:nid/delete.
func (me *Env) AdminNewsDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	nid, err := strconv.Atoi(ps.ByName("nid"))
	if err != nil {
		me.renderError(w, r, http.StatusBadRequest, "Bad Request", "Invalid news ID.")
		return
	}
	item, ok := me.DB.GetNewsByID(nid)
	if !ok || item.TournamentID != 0 {
		me.renderError(w, r, http.StatusNotFound, "Not Found", "News item not found.")
		return
	}
	me.DB.DeleteNews(nid)
	http.Redirect(w, r, "/admin/news", http.StatusSeeOther)
}

// ManageNews handles GET (list+form) and POST (create) for /tournaments/:tid/manage/news.
// Accessible by directors and staff (enforced by manage middleware).
func (me *Env) ManageNews(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		title := r.FormValue("title")
		body := r.FormValue("news_html")
		user := userFromContext(r.Context())
		if title != "" && body != "" {
			me.DB.AddNews(t.ID, title, body, user.ID)
		}
		http.Redirect(w, r, "/tournaments/"+strconv.Itoa(t.ID)+"/manage/news", http.StatusSeeOther)
		return
	}
	me.render(w, "manageNews", newsListData{
		baseData:      newBaseWithTournament(r, t),
		News:          me.DB.GetTournamentNews(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

// ManageNewsEdit handles GET/POST for /tournaments/:tid/manage/news/:nid/edit.
// Directors only — staff are rejected with 403.
func (me *Env) ManageNewsEdit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	user := userFromContext(r.Context())
	if !user.IsAdmin && !user.IsDirectorFor(t.ID) {
		me.renderError(w, r, http.StatusForbidden, "Not Authorized", "Only directors can edit news items.")
		return
	}
	nid, err := strconv.Atoi(ps.ByName("nid"))
	if err != nil {
		me.renderError(w, r, http.StatusBadRequest, "Bad Request", "Invalid news ID.")
		return
	}
	item, ok2 := me.DB.GetNewsByID(nid)
	if !ok2 || item.TournamentID != t.ID {
		me.renderError(w, r, http.StatusNotFound, "Not Found", "News item not found.")
		return
	}
	if r.Method == http.MethodPost {
		title := r.FormValue("title")
		body := r.FormValue("news_html")
		if title != "" && body != "" {
			me.DB.UpdateNews(nid, title, body)
		}
		http.Redirect(w, r, "/tournaments/"+strconv.Itoa(t.ID)+"/manage/news", http.StatusSeeOther)
		return
	}
	me.render(w, "manageNewsEdit", newsEditData{
		baseData: newBaseWithTournament(r, t),
		Item:     item,
	})
}

// ManageNewsDelete handles POST /tournaments/:tid/manage/news/:nid/delete.
// Directors only.
func (me *Env) ManageNewsDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	user := userFromContext(r.Context())
	if !user.IsAdmin && !user.IsDirectorFor(t.ID) {
		me.renderError(w, r, http.StatusForbidden, "Not Authorized", "Only directors can delete news items.")
		return
	}
	nid, err := strconv.Atoi(ps.ByName("nid"))
	if err != nil {
		me.renderError(w, r, http.StatusBadRequest, "Bad Request", "Invalid news ID.")
		return
	}
	item, ok2 := me.DB.GetNewsByID(nid)
	if !ok2 || item.TournamentID != t.ID {
		me.renderError(w, r, http.StatusNotFound, "Not Found", "News item not found.")
		return
	}
	me.DB.DeleteNews(nid)
	http.Redirect(w, r, "/tournaments/"+strconv.Itoa(t.ID)+"/manage/news", http.StatusSeeOther)
}
