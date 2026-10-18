package webhandler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/Harnish/tourneyweb/mydb"
	"github.com/julienschmidt/httprouter"
)

// followTargetName returns the display name of a followable target, or ok=false when it doesn't exist.
// With publishedOnly it also requires the owning tournament to be published (drafts can't be followed).
func (me *Env) followTargetName(kind string, id int, publishedOnly bool) (name string, ok bool) {
	var tid int
	switch kind {
	case "tournament":
		t := me.DB.ReturnTournamentByID(id)
		name, tid = t.Name, t.ID
	case "division":
		d := me.DB.ReturnDivisionByID(id)
		name, tid = d.Name, d.TournamentID
	case "team":
		t := me.DB.ReturnTeamByID(id)
		name, tid = t.Name, t.TournamentID
	default:
		return "", false
	}
	if tid == 0 {
		return "", false
	}
	if publishedOnly && me.DB.ReturnTournamentByID(tid).Status != "published" {
		return "", false
	}
	return name, true
}

// safeNext only lets a same-site absolute path through, so the next= parameter can't redirect off-site.
func safeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n\t") {
		return "/following"
	}
	return next
}

func followForm(r *http.Request) (kind string, target int, err error) {
	target, err = strconv.Atoi(r.FormValue("target"))
	return r.FormValue("kind"), target, err
}

// Follow adds the logged-in user's follow of a published tournament, level or team.
func (me *Env) Follow(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	kind, target, err := followForm(r)
	if _, ok := me.followTargetName(kind, target, true); err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	if err := me.DB.AddFollow(user.ID, kind, target); err != nil {
		if err == mydb.ErrFollowCap {
			http.Error(w, fmt.Sprintf("You can follow up to %d things. Unfollow something first.", mydb.MaxFollows), http.StatusBadRequest)
			return
		}
		http.Error(w, "Could not save", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, safeNext(r.FormValue("next")), http.StatusSeeOther)
}

// Unfollow removes the logged-in user's own follow.
func (me *Env) Unfollow(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	kind, target, err := followForm(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	me.DB.DelFollow(user.ID, kind, target)
	http.Redirect(w, r, safeNext(r.FormValue("next")), http.StatusSeeOther)
}

type followRow struct {
	mydb.Follow
	URL string
}

type followingData struct {
	baseData
	Rows []followRow
}

// Following lists everything the logged-in user follows.
func (me *Env) Following(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	var rows []followRow
	for _, f := range me.DB.UserFollows(user.ID) {
		u := fmt.Sprintf("/tournaments/%d", f.TournamentID)
		switch f.Kind {
		case "division":
			u += fmt.Sprintf("/divisions/%d", f.TargetID)
		case "team":
			u += fmt.Sprintf("/teams/%d", f.TargetID)
		}
		rows = append(rows, followRow{f, u})
	}
	me.render(w, "following", followingData{baseData: newBase(r), Rows: rows})
}

// followToken signs "user|kind|target" so an emailed link can unfollow without a login. Empty when no secret is set.
func (me *Env) followToken(userID int, kind string, targetID int) string {
	if len(me.FollowSecret) == 0 {
		return ""
	}
	payload := fmt.Sprintf("%d|%s|%d", userID, kind, targetID)
	mac := hmac.New(sha256.New, me.FollowSecret)
	mac.Write([]byte(payload))
	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(payload)) + "." + enc.EncodeToString(mac.Sum(nil))
}

func (me *Env) parseFollowToken(t string) (userID int, kind string, targetID int, ok bool) {
	if len(me.FollowSecret) == 0 {
		return
	}
	p, m, found := strings.Cut(t, ".")
	if !found {
		return
	}
	enc := base64.RawURLEncoding
	payload, err1 := enc.DecodeString(p)
	sig, err2 := enc.DecodeString(m)
	if err1 != nil || err2 != nil {
		return
	}
	mac := hmac.New(sha256.New, me.FollowSecret)
	mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return
	}
	parts := strings.Split(string(payload), "|")
	if len(parts) != 3 {
		return
	}
	uid, err1 := strconv.Atoi(parts[0])
	tgt, err2 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil {
		return
	}
	return uid, parts[1], tgt, true
}

type unfollowData struct {
	baseData
	Token string
	Name  string
	Done  bool
}

// UnfollowTokenPage shows a confirm button only. A GET must never change anything because mail scanners prefetch links.
func (me *Env) UnfollowTokenPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	t := r.URL.Query().Get("t")
	_, kind, target, ok := me.parseFollowToken(t)
	if !ok {
		http.Error(w, "This unfollow link is not valid.", http.StatusBadRequest)
		return
	}
	name, _ := me.followTargetName(kind, target, false)
	me.render(w, "unfollowConfirm", unfollowData{baseData: newBase(r), Token: t, Name: name})
}

// UnfollowTokenDo performs the unfollow named by a valid token; no login required.
func (me *Env) UnfollowTokenDo(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	t := r.FormValue("t")
	uid, kind, target, ok := me.parseFollowToken(t)
	if !ok {
		http.Error(w, "This unfollow link is not valid.", http.StatusBadRequest)
		return
	}
	me.DB.DelFollow(uid, kind, target)
	name, _ := me.followTargetName(kind, target, false)
	me.render(w, "unfollowConfirm", unfollowData{baseData: newBase(r), Name: name, Done: true})
}

// FollowState is what the followButton template needs for one target.
type FollowState struct {
	CSRFField template.HTML
	Kind      string
	Target    int
	Next      string
	LoggedIn  bool
	Following bool
}

// Follow is called from templates: {{template "followButton" ($.Follow "division" .ID)}}.
func (b baseData) Follow(kind string, id int) FollowState {
	return FollowState{CSRFField: b.CSRFField, Kind: kind, Target: id, Next: b.Here,
		LoggedIn: b.User.LoggedIn(), Following: b.FollowSet[kind+":"+strconv.Itoa(id)]}
}

// followBase loads the viewer's follows and current URL so pages can render Follow / Unfollow buttons.
func (me *Env) followBase(b baseData, r *http.Request) baseData {
	b.Here = r.URL.RequestURI()
	if b.User.LoggedIn() {
		b.FollowSet = map[string]bool{}
		for _, f := range me.DB.UserFollows(b.User.ID) {
			b.FollowSet[f.Kind+":"+strconv.Itoa(f.TargetID)] = true
		}
	}
	return b
}
