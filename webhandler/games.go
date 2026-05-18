package webhandler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
)

// GenerateGames POST /admin/tournaments/:tid/divisions/:did/games/generate
// Creates a round-robin schedule for all teams in the division.
func (me *Env) GenerateGames(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad division ID", http.StatusBadRequest)
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

	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/divisions/%d/games/new", t.ID, did), http.StatusSeeOther)
}
