package webhandler

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func nextPowerOf2(n int) int {
	if n <= 1 {
		return 1
	}
	p := 1
	for p < n {
		p *= 2
	}
	return p
}

// bracketPositions returns the seed order for round 1 of an N-team bracket.
// Pairs are: (positions[0], positions[1]), (positions[2], positions[3]), etc.
// Each pair sums to N+1, ensuring top seeds play bottom seeds.
func bracketPositions(n int) []int {
	if n == 1 {
		return []int{1}
	}
	half := bracketPositions(n / 2)
	out := make([]int, 0, n)
	for _, p := range half {
		out = append(out, p, n+1-p)
	}
	return out
}

func roundLabel(round, totalRounds int) string {
	switch totalRounds - round {
	case 0:
		return "Final"
	case 1:
		return "Semifinals"
	case 2:
		return "Quarterfinals"
	default:
		return fmt.Sprintf("Round %d", round)
	}
}

// generateBracket creates all bracket_game rows for all rounds, assigns round-1
// teams from the seeded bracket_seeds, and auto-advances teams that received byes.
func generateBracket(db mydb.DB, bracketID int) {
	bracket := db.GetBracketByID(bracketID)
	size := bracket.Size
	totalRounds := int(math.Log2(float64(size)))
	seeds := db.GetBracketSeeds(bracketID)

	seedMap := make(map[int]mydb.BracketSeed, len(seeds))
	for _, s := range seeds {
		seedMap[s.Seed] = s
	}

	// Create bracket_game rows for every round and position.
	for r := 1; r <= totalRounds; r++ {
		posCount := size >> r // size / 2^r
		if posCount == 0 {
			posCount = 1
		}
		for p := 1; p <= posCount; p++ {
			db.AddBracketGame(bracketID, r, p)
		}
	}

	// Assign round-1 teams.
	positions := bracketPositions(size)
	for i := 0; i < len(positions)-1; i += 2 {
		gamePos := i/2 + 1
		topEntry := seedMap[positions[i]]
		bottomEntry := seedMap[positions[i+1]]
		bg := db.GetBracketGameByRoundPosition(bracketID, 1, gamePos)
		db.SetBracketGameTeams(bg.ID,
			topEntry.TeamID, bottomEntry.TeamID,
			topEntry.TeamID == 0, bottomEntry.TeamID == 0,
		)
	}

	// Auto-advance any byes in round 1.
	allGames := db.GetBracketGames(bracketID)
	for _, bg := range allGames {
		if bg.Round != 1 {
			continue
		}
		if !bg.TopIsBye && !bg.BottomIsBye {
			continue
		}
		if bg.TopIsBye && bg.BottomIsBye {
			continue // both bye: nothing to advance
		}
		winner := bg.BottomTeamID
		if bg.BottomIsBye {
			winner = bg.TopTeamID
		}
		db.SetBracketGameWinner(bg.ID, winner)

		parentPos := (bg.Position + 1) / 2
		parent := db.GetBracketGameByRoundPosition(bracketID, 2, parentPos)
		if parent.ID == 0 {
			continue
		}
		if bg.Position%2 == 1 {
			db.SetBracketGameTopTeam(parent.ID, winner)
		} else {
			db.SetBracketGameBottomTeam(parent.ID, winner)
		}
	}
}

func (me *Env) ManageBracketStart(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "bad division id", http.StatusBadRequest)
		return
	}
	div := me.DB.ReturnDivisionByID(did)
	if div.TournamentID != t.ID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if div.Phase != "pool" {
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions/%d/bracket/seed", t.ID, did), http.StatusSeeOther)
		return
	}

	teams := me.DB.ReturnTeamsByDivisionIDWithStats(div.ID)
	teams = me.SortTeams(teams, div.RankingCriteria)

	size := nextPowerOf2(len(teams))
	if size < 2 {
		size = 2
	}
	bracketID := me.DB.CreateBracket(div.ID, "single_elimination", size)
	for i, team := range teams {
		me.DB.AddBracketSeed(bracketID, i+1, team.ID)
	}
	for i := len(teams) + 1; i <= size; i++ {
		me.DB.AddBracketSeed(bracketID, i, 0)
	}
	me.DB.SetDivisionPhase(div.ID, "bracket")
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions/%d/bracket/seed", t.ID, did), http.StatusSeeOther)
}

func (me *Env) ManageBracketSeed(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "bad division id", http.StatusBadRequest)
		return
	}
	div := me.DB.ReturnDivisionByID(did)
	if div.TournamentID != t.ID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	bracket := me.DB.GetBracketByDivisionID(did)
	if bracket.ID == 0 {
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodPost {
		if bracket.Status != "seeding" {
			http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
			return
		}
		orderStr := r.FormValue("seed_order")
		var teamIDs []int
		for _, s := range strings.Split(orderStr, ",") {
			s = strings.TrimSpace(s)
			id, err := strconv.Atoi(s)
			if err != nil || id <= 0 {
				continue
			}
			teamIDs = append(teamIDs, id)
		}
		// Clamp to bracket size in case of oversized submission (form tampering).
		byeCount := bracket.Size - len(teamIDs)
		if byeCount < 0 {
			teamIDs = teamIDs[:bracket.Size]
			byeCount = 0
		}
		// Append byes to fill bracket size
		for i := 0; i < byeCount; i++ {
			teamIDs = append(teamIDs, 0)
		}
		me.DB.UpdateBracketSeeds(bracket.ID, teamIDs)
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions/%d/bracket/seed", t.ID, did), http.StatusSeeOther)
		return
	}

	seeds := me.DB.GetBracketSeeds(bracket.ID)
	me.render(w, "bracketSeed", manageBracketSeedData{
		baseData: newBaseWithTournament(r, t),
		Division: div,
		Bracket:  bracket,
		Seeds:    seeds,
	})
}

func (me *Env) ManageBracketLock(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "bad division id", http.StatusBadRequest)
		return
	}
	div := me.DB.ReturnDivisionByID(did)
	if div.TournamentID != t.ID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	bracket := me.DB.GetBracketByDivisionID(did)
	if bracket.ID == 0 || bracket.Status != "seeding" {
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
		return
	}

	me.DB.SetBracketStatus(bracket.ID, "active")
	generateBracket(me.DB, bracket.ID)

	// Create placeholder games for all matchups that are immediately ready.
	for _, bg := range me.DB.GetBracketGames(bracket.ID) {
		if bg.WinnerTeamID != 0 {
			continue // already decided (bye advancement)
		}
		if bg.TopTeamID == 0 || bg.BottomTeamID == 0 {
			continue // TBD
		}
		if bg.GameID != 0 {
			continue // already has a game
		}
		gid := me.DB.AddGame(div.TournamentID, div.ID, bg.TopTeamID, bg.BottomTeamID, "", time.Time{}, "")
		me.DB.SetBracketGameGameID(bg.ID, gid)
	}

	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
}

func (me *Env) AdvanceBracket(gameID, winnerTeamID int) {
	bg := me.DB.GetBracketGameByGameID(gameID)
	if bg.ID == 0 {
		return
	}
	if bg.WinnerTeamID != 0 {
		slog.Error("AdvanceBracket: game already advanced, re-scoring not supported",
			"bracketGameID", bg.ID, "existingWinner", bg.WinnerTeamID, "newWinner", winnerTeamID)
		return
	}

	me.DB.SetBracketGameWinner(bg.ID, winnerTeamID)

	parentRound := bg.Round + 1
	parentPos := (bg.Position + 1) / 2
	parent := me.DB.GetBracketGameByRoundPosition(bg.BracketID, parentRound, parentPos)
	if parent.ID == 0 {
		// This was the final — mark bracket complete.
		me.DB.SetBracketStatus(bg.BracketID, "complete")
		return
	}

	if bg.Position%2 == 1 {
		me.DB.SetBracketGameTopTeam(parent.ID, winnerTeamID)
	} else {
		me.DB.SetBracketGameBottomTeam(parent.ID, winnerTeamID)
	}

	// If parent now has both teams and no game yet, create a placeholder game.
	parent = me.DB.GetBracketGameByID(parent.ID)
	if parent.TopTeamID != 0 && parent.BottomTeamID != 0 && parent.GameID == 0 {
		bracket := me.DB.GetBracketByID(bg.BracketID)
		div := me.DB.ReturnDivisionByID(bracket.DivisionID)
		gid := me.DB.AddGame(div.TournamentID, div.ID, parent.TopTeamID, parent.BottomTeamID, "", time.Time{}, "")
		me.DB.SetBracketGameGameID(parent.ID, gid)
	}
}
