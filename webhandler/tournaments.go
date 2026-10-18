package webhandler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/Harnish/tourneyweb/mydb"
)

func (me *Env) tournamentFromRoute(w http.ResponseWriter, r *http.Request, ps httprouter.Params) (mydb.Tournament, bool) {
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
	if t.Status == "draft" {
		user := userFromContext(r.Context())
		if !user.IsAdmin && !user.IsDirectorFor(t.ID) && !user.IsStaffFor(t.ID) {
			http.NotFound(w, r)
			return mydb.Tournament{}, false
		}
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
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var leagues []mydb.Tournament
	if q == "" {
		leagues = me.DB.ReturnLeagues(time.Now())
	}
	me.render(w, "index", indexData{
		baseData:      me.followBase(newBase(r), r),
		Q:             q,
		Leagues:       leagues,
		Results:       me.DB.SearchTournaments(q),
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
		SiteNews:      me.DB.GetSiteNews(),
	})
}

func (me *Env) TournamentHome(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	divs := me.DB.ReturnDivisions(t.ID)
	teams := make(map[int][]mydb.Team)
	for _, div := range divs {
		teams[div.ID] = me.DB.ReturnTeamsByDivisionID(div.ID)
	}
	var leagueOf mydb.Tournament
	if t.LeagueID != 0 {
		if l := me.DB.ReturnTournamentByID(t.LeagueID); l.Status == "published" {
			leagueOf = l
		}
	}
	var playoffs []mydb.Tournament
	if t.IsLeague() {
		for _, p := range me.DB.LeaguePlayoffs(t.ID) {
			if p.Status == "published" {
				playoffs = append(playoffs, p)
			}
		}
	}
	me.render(w, "tournament", tournamentData{
		LeagueOf:  leagueOf,
		Playoffs:  playoffs,
		baseData:  me.followBase(newBaseWithTournament(r, t), r),
		Divisions: divs,
		Teams:     teams,
		News:      me.DB.GetTournamentNews(t.ID),
		Fields:    me.DB.GetLocationsByTournamentID(t.ID),
	})
}

func (me *Env) TournamentFields(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	me.render(w, "tournamentFields", tournamentFieldsData{
		baseData: newBaseWithTournament(r, t),
		Fields:   me.DB.GetLocationsByTournamentID(t.ID),
	})
}

func (me *Env) AdminTournaments(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "adminTournaments", adminTournamentsData{
		baseData:    newBase(r),
		Tournaments: me.DB.ReturnTournaments(),
		AllCriteria: AllCriteriaForUI(mydb.DefaultRankingCriteria),
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
	id := me.DB.AddTournament(name, sport, location, notes, date, "published")
	user := userFromContext(r.Context())
	if user.LoggedIn() {
		me.DB.AssignRole(user.ID, id, "director", 0)
		criteriaStr := r.FormValue("default_ranking_criteria")
		var criteria []string
		for _, k := range strings.Split(criteriaStr, ",") {
			k = strings.TrimSpace(k)
			if _, ok := criteriaRegistry[k]; ok {
				criteria = append(criteria, k)
			}
		}
		if len(criteria) > 0 {
			me.DB.SetTournamentDefaultRanking(id, criteria)
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d", id), http.StatusSeeOther)
}

func (me *Env) AdminTournamentView(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	me.render(w, "adminTournamentView", adminTournamentViewData{
		baseData:      newBaseWithTournament(r, t),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) NewTournamentForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	me.render(w, "newTournament", newTournamentData{
		baseData:    newBase(r),
		AllCriteria: AllCriteriaForUI(mydb.DefaultRankingCriteria),
	})
}

func (me *Env) NewTournament(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
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
	kind := r.FormValue("kind")
	if kind == "" {
		kind = "tournament"
	}
	var endDate time.Time
	switch kind {
	case "tournament":
	case "league":
		now := time.Now()
		if date.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)) {
			http.Error(w, "A league's start date must be today or later", http.StatusBadRequest)
			return
		}
		if es := r.FormValue("end_date"); es != "" {
			endDate, err = time.Parse("2006-01-02", es)
			if err != nil || endDate.Before(date) {
				http.Error(w, "The end date must be a valid date on or after the start date", http.StatusBadRequest)
				return
			}
		}
	default:
		http.Error(w, "Unknown type", http.StatusBadRequest)
		return
	}
	status := "draft"
	if user.IsAdmin {
		status = "published"
	}
	id := me.DB.AddTournament(name, sport, location, notes, date, status)
	if kind == "league" {
		me.DB.SetTournamentLeague(id, kind, endDate, 0)
	}
	me.DB.AssignRole(user.ID, id, "director", 0)
	criteriaStr := r.FormValue("default_ranking_criteria")
	var criteria []string
	for _, k := range strings.Split(criteriaStr, ",") {
		k = strings.TrimSpace(k)
		if _, ok := criteriaRegistry[k]; ok {
			criteria = append(criteria, k)
		}
	}
	if len(criteria) > 0 {
		me.DB.SetTournamentDefaultRanking(id, criteria)
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage", id), http.StatusSeeOther)
}

func (me *Env) EditTournament(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
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
