package webhandler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func (me *Env) tournamentFromRoute(w http.ResponseWriter, ps httprouter.Params) (mydb.Tournament, bool) {
	tid, err := strconv.Atoi(ps.ByName("tid"))
	if err != nil {
		http.Error(w, "Bad tournament ID", http.StatusBadRequest)
		return mydb.Tournament{}, false
	}
	t := me.DB.ReturnTournamentByID(tid)
	if t.ID == 0 {
		http.Error(w, "Tournament not found", http.StatusNotFound)
		return mydb.Tournament{}, false
	}
	return t, true
}

func (me *Env) TournamentList(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	futurePage, _ := strconv.Atoi(r.URL.Query().Get("future_page"))
	pastPage, _ := strconv.Atoi(r.URL.Query().Get("past_page"))
	if futurePage < 1 {
		futurePage = 1
	}
	if pastPage < 1 {
		pastPage = 1
	}
	future, futureTotal := me.DB.ReturnTournamentsFuture(futurePage)
	past, pastTotal := me.DB.ReturnTournamentsPast(pastPage)
	me.render(w, "index", indexData{
		baseData:      newBase(r),
		ComingUp:      me.DB.ReturnTournamentsComingUp(),
		Recent:        me.DB.ReturnTournamentsRecent(),
		Future:        future,
		Past:          past,
		FuturePage:    futurePage,
		PastPage:      pastPage,
		FutureTotal:   futureTotal,
		PastTotal:     pastTotal,
		FutureHasPrev: futurePage > 1,
		FutureHasNext: futurePage*20 < futureTotal,
		PastHasPrev:   pastPage > 1,
		PastHasNext:   pastPage*20 < pastTotal,
	})
}

func (me *Env) TournamentHome(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	divs := me.DB.ReturnDivisions(t.ID)
	teams := make(map[int][]mydb.Team)
	for _, div := range divs {
		teams[div.ID] = me.DB.ReturnTeamsByDivisionID(div.ID)
	}
	me.render(w, "tournament", tournamentData{
		baseData:  newBaseWithTournament(r, t),
		Divisions: divs,
		Teams:     teams,
	})
}

func (me *Env) AdminTournaments(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "adminTournaments", adminTournamentsData{
		baseData:    newBase(r),
		Tournaments: me.DB.ReturnTournaments(),
	})
}

func (me *Env) CreateTournament(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := r.FormValue("name")
	sport := r.FormValue("sport")
	location := r.FormValue("location")
	notes := r.FormValue("notes")
	dateStr := r.FormValue("start_date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "Invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}
	if name == "" || sport == "" || location == "" {
		http.Error(w, "Name, sport, and location are required", http.StatusBadRequest)
		return
	}
	id := me.DB.AddTournament(name, sport, location, notes, date)
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d", id), http.StatusSeeOther)
}

func (me *Env) AdminTournamentView(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	me.render(w, "adminTournamentView", adminTournamentViewData{
		baseData:      newBaseWithTournament(r, t),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) EditTournament(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		sport := r.FormValue("sport")
		location := r.FormValue("location")
		notes := r.FormValue("notes")
		date, err := time.Parse("2006-01-02", r.FormValue("start_date"))
		if err != nil {
			http.Error(w, "Invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
			return
		}
		if name == "" || sport == "" || location == "" {
			http.Error(w, "Name, sport, and location are required", http.StatusBadRequest)
			return
		}
		me.DB.UpdateTournament(t.ID, name, sport, location, notes, date)
		http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "editTournament", editTournamentData{
		baseData: newBaseWithTournament(r, t),
	})
}
