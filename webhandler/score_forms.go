package webhandler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Harnish/tourneyweb/mydb"
	"github.com/julienschmidt/httprouter"
)

type scoreReportView struct {
	Team       string
	Home, Away int
	Mine       bool
}

type scoreEntryData struct {
	baseData
	Game         mydb.Game
	Status       string
	Reports      []scoreReportView
	HasOther     bool // the other team has reported: offer a one-click Confirm
	OtherHome    int
	OtherAway    int
	ScoreOptions []int
	Locked       bool
}

// scoreFormTarget does the checks shared by the coach and umpire forms: login, tournament, and a game that
// belongs to that tournament.
func (me *Env) scoreFormTarget(w http.ResponseWriter, r *http.Request, ps httprouter.Params) (user TWUser, t mydb.Tournament, g mydb.Game, ok bool) {
	user = userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	t, found := me.tournamentFromRoute(w, r, ps)
	if !found {
		return
	}
	gid, err := strconv.Atoi(ps.ByName("gid"))
	if err == nil {
		g = me.DB.ReturnGameByID(gid)
	}
	if err != nil || g.ID == 0 || g.TournamentID != t.ID {
		http.NotFound(w, r)
		return
	}
	return user, t, g, true
}

func scoreOptions() []int {
	o := make([]int, 41)
	for i := range o {
		o[i] = i
	}
	return o
}

// writeScoreError maps the score-entry errors to HTTP statuses.
func writeScoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrScoreLocked):
		http.Error(w, "This score is already confirmed or final. Ask the tournament director to change it.", http.StatusConflict)
	case errors.Is(err, ErrNotYourGame):
		http.Error(w, "Forbidden", http.StatusForbidden)
	case errors.Is(err, ErrBadScore), errors.Is(err, ErrBracketTie):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, "Could not save the score", http.StatusInternalServerError)
	}
}

func parseScores(r *http.Request) (int, int, bool) {
	h, err1 := strconv.Atoi(r.FormValue("homescore"))
	a, err2 := strconv.Atoi(r.FormValue("awayscore"))
	return h, a, err1 == nil && err2 == nil
}

// CoachScore lets a coach report (or confirm) the score of a game their team played.
func (me *Env) CoachScore(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user, t, g, ok := me.scoreFormTarget(w, r, ps)
	if !ok {
		return
	}
	reporter, isCoach := coachReporter(user, g)
	if !isCoach {
		http.Error(w, "Only a coach of one of the two teams can report this score.", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost {
		home, away, valid := parseScores(r)
		if !valid {
			http.Error(w, "Bad score", http.StatusBadRequest)
			return
		}
		if err := me.submitCoachScore(g, reporter, user, home, away); err != nil {
			writeScoreError(w, err)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/games", t.ID), http.StatusSeeOther)
		return
	}
	data := scoreEntryData{baseData: newBaseWithTournament(r, t), Game: g, Status: g.ScoreStatus,
		ScoreOptions: scoreOptions(), Locked: g.ScoreStatus == "confirmed" || g.ScoreStatus == "final"}
	for _, rep := range me.DB.ScoreReportsForGame(g.ID) {
		team := g.HomeTeam.Name
		if rep.Reporter == reporterAway {
			team = g.AwayTeam.Name
		}
		mine := rep.Reporter == reporter
		data.Reports = append(data.Reports, scoreReportView{Team: team, Home: rep.HomeScore, Away: rep.AwayScore, Mine: mine})
		if !mine {
			data.HasOther, data.OtherHome, data.OtherAway = true, rep.HomeScore, rep.AwayScore
		}
	}
	me.render(w, "coachScore", data)
}

// UmpireScoreEntry lets an umpire assigned to the game enter its final score.
func (me *Env) UmpireScoreEntry(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user, t, g, ok := me.scoreFormTarget(w, r, ps)
	if !ok {
		return
	}
	if !me.umpireAssigned(g.ID, user.ID) {
		http.Error(w, "You are not assigned to umpire this game.", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost {
		home, away, valid := parseScores(r)
		if !valid {
			http.Error(w, "Bad score", http.StatusBadRequest)
			return
		}
		if err := me.submitUmpireScore(g, user, home, away); err != nil {
			writeScoreError(w, err)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/games", t.ID), http.StatusSeeOther)
		return
	}
	data := scoreEntryData{baseData: newBaseWithTournament(r, t), Game: g, Status: g.ScoreStatus,
		ScoreOptions: scoreOptions(), Locked: g.ScoreStatus == "confirmed" || g.ScoreStatus == "final"}
	for _, rep := range me.DB.ScoreReportsForGame(g.ID) {
		team := g.HomeTeam.Name
		if rep.Reporter == reporterAway {
			team = g.AwayTeam.Name
		}
		data.Reports = append(data.Reports, scoreReportView{Team: team, Home: rep.HomeScore, Away: rep.AwayScore})
	}
	me.render(w, "umpireScore", data)
}
