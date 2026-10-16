package webhandler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
	"github.com/julienschmidt/httprouter"
)

const (
	maxAvailabilityNote = 200
	// a tournament can still be volunteered for this long after it starts
	availabilityGrace = 7 * 24 * time.Hour
)

type openTournament struct {
	Tournament mydb.Tournament
	Available  bool
	Note       string
}

// openTournamentsFor lists the tournaments an umpire can volunteer for, with their current availability.
func (me *Env) openTournamentsFor(userID int) []openTournament {
	mine := me.DB.UmpireAvailabilityForUser(userID)
	var out []openTournament
	for _, t := range me.DB.ReturnTournamentsOpenForUmpires(time.Now().Add(-availabilityGrace)) {
		a, ok := mine[t.ID]
		out = append(out, openTournament{Tournament: t, Available: ok, Note: a.Note})
	}
	return out
}

// availabilityTarget resolves the logged-in umpire and an open (non-draft, not long past) tournament.
func (me *Env) availabilityTarget(w http.ResponseWriter, r *http.Request, ps httprouter.Params) (user TWUser, t mydb.Tournament, ok bool) {
	user = userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if _, isUmpire := me.DB.GetUmpire(user.ID); !isUmpire {
		http.Error(w, "Only registered umpires can do this. Enable it on your profile.", http.StatusForbidden)
		return
	}
	tid, err := strconv.Atoi(ps.ByName("tid"))
	if err == nil {
		t = me.DB.ReturnTournamentByID(tid)
	}
	if err != nil || t.ID == 0 || t.Status == "draft" ||
		t.StartDate.Format("2006-01-02") < time.Now().Add(-availabilityGrace).Format("2006-01-02") {
		http.NotFound(w, r)
		return
	}
	return user, t, true
}

// UmpireAvailable records "I'm available" for a tournament. A new volunteer notifies the directors once;
// editing only the note does not.
func (me *Env) UmpireAvailable(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user, t, ok := me.availabilityTarget(w, r, ps)
	if !ok {
		return
	}
	note := strings.TrimSpace(r.FormValue("note"))
	if len([]rune(note)) > maxAvailabilityNote {
		http.Error(w, fmt.Sprintf("Note must be %d characters or fewer", maxAvailabilityNote), http.StatusBadRequest)
		return
	}
	if me.DB.SetUmpireAvailability(user.ID, t.ID, note) {
		msg := user.Name + " is available to umpire " + t.Name + "."
		if note != "" {
			msg += " Note: " + note
		}
		link := fmt.Sprintf("/tournaments/%d/manage/umpires", t.ID)
		for _, id := range me.directorRecipients(t.ID) {
			me.notifyUser(id, "Umpire available", msg, link)
		}
	}
	http.Redirect(w, r, "/umpire", http.StatusSeeOther)
}

// UmpireWithdraw clears availability. It never touches existing game assignments and notifies nobody.
// Withdrawing is always allowed (it only ever touches the user's own row), even if they are no longer flagged
// as an umpire or the tournament has since moved or become a draft.
func (me *Env) UmpireWithdraw(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	tid, err := strconv.Atoi(ps.ByName("tid"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	me.DB.ClearUmpireAvailability(user.ID, tid)
	http.Redirect(w, r, "/umpire", http.StatusSeeOther)
}
