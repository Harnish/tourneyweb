package webhandler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) ManageCreateGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad division ID", http.StatusBadRequest)
		return
	}
	div := me.DB.ReturnDivisionByID(did)
	if div.ID == 0 || div.TournamentID != t.ID {
		http.Error(w, "Division not found", http.StatusNotFound)
		return
	}
	me.render(w, "manageCreateGame", manageCreateGameData{
		baseData:      newBaseWithTournament(r, t),
		DivisionID:    did,
		Teams:         me.DB.ReturnTeamsByDivisionID(did),
		Games:         me.DB.AllGamesByDivision(did),
		Locations:     me.locationsForTournament(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) ManageCreateGameSubmit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(r.FormValue("divisionid"))
	if err != nil {
		http.Error(w, "Bad DivisionID", http.StatusBadRequest)
		return
	}
	hid, err := strconv.Atoi(r.FormValue("hometeam"))
	if err != nil {
		http.Error(w, "Bad Home team ID", http.StatusBadRequest)
		return
	}
	aid, err := strconv.Atoi(r.FormValue("awayteam"))
	if err != nil {
		http.Error(w, "Bad Away team ID", http.StatusBadRequest)
		return
	}
	if aid == hid {
		http.Error(w, "Must select a different team as an opponent.", http.StatusBadRequest)
		return
	}
	div := me.DB.ReturnDivisionByID(did)
	if div.ID == 0 || div.TournamentID != t.ID {
		http.Error(w, "Division does not belong to this tournament", http.StatusBadRequest)
		return
	}
	homeTeam := me.DB.ReturnTeamByID(hid)
	if homeTeam.ID == 0 || homeTeam.TournamentID != t.ID {
		http.Error(w, "Home team does not belong to this tournament", http.StatusBadRequest)
		return
	}
	awayTeam := me.DB.ReturnTeamByID(aid)
	if awayTeam.ID == 0 || awayTeam.TournamentID != t.ID {
		http.Error(w, "Away team does not belong to this tournament", http.StatusBadRequest)
		return
	}
	startTime, _ := time.ParseInLocation("2006-01-02T15:04", r.FormValue("datetime"), time.Local)
	me.DB.AddGame(t.ID, did, hid, aid, r.FormValue("location"), startTime, r.FormValue("umpire"))
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions/%d/games/new", t.ID, did), http.StatusSeeOther)
}

func (me *Env) ManageGenerateGames(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad division ID", http.StatusBadRequest)
		return
	}
	div := me.DB.ReturnDivisionByID(did)
	if div.ID == 0 || div.TournamentID != t.ID {
		http.Error(w, "Division does not belong to this tournament", http.StatusBadRequest)
		return
	}
	teams := me.DB.ReturnTeamsByDivisionID(did)
	if len(teams) < 2 {
		http.Error(w, "Need at least 2 teams to generate a schedule", http.StatusBadRequest)
		return
	}
	startTime, _ := time.ParseInLocation("2006-01-02T15:04", r.FormValue("start_datetime"), time.Local)
	minutesBetween, _ := strconv.Atoi(r.FormValue("minutes_between"))
	if minutesBetween <= 0 {
		minutesBetween = 120
	}
	location := r.FormValue("location")
	interval := time.Duration(minutesBetween) * time.Minute
	current := startTime

	maxGames, _ := strconv.Atoi(r.FormValue("max_games_per_team"))
	if maxGames > 0 {
		for _, spec := range cappedSchedule(teams, maxGames) {
			gid := me.DB.AddGame(t.ID, did, spec.homeTeamID, spec.awayTeamID, location, current, "")
			if spec.scrimmageFor != 0 {
				me.DB.SetGameScrimmage(gid, spec.scrimmageFor)
			}
			current = current.Add(interval)
		}
	} else {
		double := r.FormValue("round_type") == "double"
		for i := 0; i < len(teams); i++ {
			for j := i + 1; j < len(teams); j++ {
				me.DB.AddGame(t.ID, did, teams[i].ID, teams[j].ID, location, current, "")
				current = current.Add(interval)
				if double {
					me.DB.AddGame(t.ID, did, teams[j].ID, teams[i].ID, location, current, "")
					current = current.Add(interval)
				}
			}
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions/%d/games/new", t.ID, did), http.StatusSeeOther)
}

func (me *Env) ManageEditGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	gid, err := strconv.Atoi(ps.ByName("gid"))
	if err != nil {
		http.Error(w, "Bad Game ID", http.StatusBadRequest)
		return
	}
	game := me.DB.ReturnGameByID(gid)
	if game.ID == 0 || game.TournamentID != t.ID {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPost {
		did, err := strconv.Atoi(r.FormValue("divisionid"))
		if err != nil {
			http.Error(w, "Bad Division ID", http.StatusBadRequest)
			return
		}
		hid, err := strconv.Atoi(r.FormValue("hometeam"))
		if err != nil {
			http.Error(w, "Bad Home Team ID", http.StatusBadRequest)
			return
		}
		aid, err := strconv.Atoi(r.FormValue("awayteam"))
		if err != nil {
			http.Error(w, "Bad Away Team ID", http.StatusBadRequest)
			return
		}
		if hid == aid {
			http.Error(w, "Home and away team must be different", http.StatusBadRequest)
			return
		}
		div := me.DB.ReturnDivisionByID(did)
		if div.ID == 0 || div.TournamentID != t.ID {
			http.Error(w, "Division does not belong to this tournament", http.StatusBadRequest)
			return
		}
		homeTeam := me.DB.ReturnTeamByID(hid)
		if homeTeam.ID == 0 || homeTeam.TournamentID != t.ID {
			http.Error(w, "Home team does not belong to this tournament", http.StatusBadRequest)
			return
		}
		awayTeam := me.DB.ReturnTeamByID(aid)
		if awayTeam.ID == 0 || awayTeam.TournamentID != t.ID {
			http.Error(w, "Away team does not belong to this tournament", http.StatusBadRequest)
			return
		}
		startTime, _ := time.ParseInLocation("2006-01-02T15:04", r.FormValue("datetime"), time.Local)
		me.DB.UpdateGame(gid, did, hid, aid, r.FormValue("location"), startTime, r.FormValue("umpire"))
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "manageEditGame", manageEditGameData{
		baseData:  newBaseWithTournament(r, t),
		Game:      game,
		Teams:     me.DB.ReturnTeamsByTournamentID(t.ID),
		Divisions: me.DB.ReturnDivisions(t.ID),
		Locations: me.locationsForTournament(t.ID),
	})
}

func (me *Env) ManageDeleteGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	gid, err := strconv.Atoi(ps.ByName("gid"))
	if err != nil {
		http.Error(w, "Bad Game ID", http.StatusBadRequest)
		return
	}
	game := me.DB.ReturnGameByID(gid)
	if game.ID == 0 || game.TournamentID != t.ID {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}
	if !me.DisableDelete {
		me.DB.DelGame(gid)
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
}
