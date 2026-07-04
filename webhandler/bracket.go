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

func isPowerOfTwo(n int) bool {
	return n >= 2 && n&(n-1) == 0
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

// generateBracket creates all bracket_game rows for a locked bracket, dispatching
// on format.
func generateBracket(db mydb.DB, bracketID int) {
	bracket := db.GetBracketByID(bracketID)
	if bracket.Format == "double_elimination" {
		generateDoubleEliminationBracket(db, bracket)
		return
	}
	generateSingleEliminationBracket(db, bracket)
}

// generateSingleEliminationBracket creates all bracket_game rows for all rounds,
// assigns round-1 teams from the seeded bracket_seeds, and auto-advances teams
// that received byes.
func generateSingleEliminationBracket(db mydb.DB, bracket mydb.Bracket) {
	bracketID := bracket.ID
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
			db.AddBracketGame(bracketID, r, p, "winners")
		}
	}

	// Assign round-1 teams.
	positions := bracketPositions(size)
	for i := 0; i < len(positions)-1; i += 2 {
		gamePos := i/2 + 1
		topEntry := seedMap[positions[i]]
		bottomEntry := seedMap[positions[i+1]]
		bg := db.GetBracketGameByRoundPosition(bracketID, "winners", 1, gamePos)
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
		parent := db.GetBracketGameByRoundPosition(bracketID, "winners", 2, parentPos)
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

// generateDoubleEliminationBracket creates the winners bracket (identical shape
// to single elimination, no byes — double elimination requires an exact power-of-2
// team count), the losers bracket, and the grand-final slot. See
// docs/superpowers/specs/2026-05-17-brackets-design.md for the round/position math.
func generateDoubleEliminationBracket(db mydb.DB, bracket mydb.Bracket) {
	bracketID := bracket.ID
	size := bracket.Size
	winnersRounds := int(math.Log2(float64(size)))
	seeds := db.GetBracketSeeds(bracketID)

	seedMap := make(map[int]mydb.BracketSeed, len(seeds))
	for _, s := range seeds {
		seedMap[s.Seed] = s
	}

	for r := 1; r <= winnersRounds; r++ {
		posCount := size >> r
		if posCount == 0 {
			posCount = 1
		}
		for p := 1; p <= posCount; p++ {
			db.AddBracketGame(bracketID, r, p, "winners")
		}
	}

	positions := bracketPositions(size)
	for i := 0; i < len(positions)-1; i += 2 {
		gamePos := i/2 + 1
		topEntry := seedMap[positions[i]]
		bottomEntry := seedMap[positions[i+1]]
		bg := db.GetBracketGameByRoundPosition(bracketID, "winners", 1, gamePos)
		db.SetBracketGameTeams(bg.ID, topEntry.TeamID, bottomEntry.TeamID, false, false)
	}

	// Grand final slot always exists, even for a 2-team bracket.
	db.AddBracketGame(bracketID, 1, 1, "final")

	if winnersRounds == 1 {
		// 2-team bracket: the sole winners-bracket game's loser IS the losers-bracket
		// champion, no losers-bracket games are needed.
		return
	}

	totalLBRounds := 2 * (winnersRounds - 1)
	for r := 1; r <= totalLBRounds; r++ {
		k := (r + 1) / 2
		posCount := size >> (k + 1)
		if posCount == 0 {
			posCount = 1
		}
		for p := 1; p <= posCount; p++ {
			db.AddBracketGame(bracketID, r, p, "losers")
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

	format := r.FormValue("format")
	if format != "double_elimination" {
		format = "single_elimination"
	}

	var size int
	if format == "double_elimination" {
		if !isPowerOfTwo(len(teams)) {
			http.Error(w, "Double elimination requires a team count that is exactly a power of 2 (4, 8, 16, or 32); use single elimination instead", http.StatusBadRequest)
			return
		}
		size = len(teams)
	} else {
		size = nextPowerOf2(len(teams))
		if size < 2 {
			size = 2
		}
	}
	bracketID := me.DB.CreateBracket(div.ID, format, size)
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

	bracket := me.DB.GetBracketByID(bg.BracketID)
	if bracket.Format == "double_elimination" {
		me.advanceDoubleElimination(bracket, bg, winnerTeamID)
		return
	}
	me.advanceSingleElimination(bg, winnerTeamID)
}

// maybeCreateBracketGame creates the placeholder games record for a bracket_game
// once both its slots are filled, if one doesn't already exist.
func (me *Env) maybeCreateBracketGame(id int) {
	bg := me.DB.GetBracketGameByID(id)
	if bg.TopTeamID != 0 && bg.BottomTeamID != 0 && bg.GameID == 0 {
		bracket := me.DB.GetBracketByID(bg.BracketID)
		div := me.DB.ReturnDivisionByID(bracket.DivisionID)
		gid := me.DB.AddGame(div.TournamentID, div.ID, bg.TopTeamID, bg.BottomTeamID, "", time.Time{}, "")
		me.DB.SetBracketGameGameID(bg.ID, gid)
	}
}

func (me *Env) advanceSingleElimination(bg mydb.BracketGame, winnerTeamID int) {
	parentRound := bg.Round + 1
	parentPos := (bg.Position + 1) / 2
	parent := me.DB.GetBracketGameByRoundPosition(bg.BracketID, "winners", parentRound, parentPos)
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
	me.maybeCreateBracketGame(parent.ID)
}

// advanceDoubleElimination cascades a decided bracket_game's winner (and, for
// winners-bracket games, its loser) forward. See
// docs/superpowers/specs/2026-05-17-brackets-design.md for the round/position math.
func (me *Env) advanceDoubleElimination(bracket mydb.Bracket, bg mydb.BracketGame, winnerTeamID int) {
	size := bracket.Size
	winnersRounds := int(math.Log2(float64(size)))
	loserTeamID := bg.TopTeamID
	if winnerTeamID == bg.TopTeamID {
		loserTeamID = bg.BottomTeamID
	}

	switch bg.Side {
	case "winners":
		if bg.Round < winnersRounds {
			parentPos := (bg.Position + 1) / 2
			parent := me.DB.GetBracketGameByRoundPosition(bracket.ID, "winners", bg.Round+1, parentPos)
			if bg.Position%2 == 1 {
				me.DB.SetBracketGameTopTeam(parent.ID, winnerTeamID)
			} else {
				me.DB.SetBracketGameBottomTeam(parent.ID, winnerTeamID)
			}
			me.maybeCreateBracketGame(parent.ID)
		} else {
			// Winners-bracket final: winner goes straight to the grand final's top slot.
			final := me.DB.GetBracketGameByRoundPosition(bracket.ID, "final", 1, 1)
			me.DB.SetBracketGameTopTeam(final.ID, winnerTeamID)
			me.maybeCreateBracketGame(final.ID)
		}

		if winnersRounds == 1 {
			// No losers bracket: the sole loser is the losers-bracket champion.
			final := me.DB.GetBracketGameByRoundPosition(bracket.ID, "final", 1, 1)
			me.DB.SetBracketGameBottomTeam(final.ID, loserTeamID)
			me.maybeCreateBracketGame(final.ID)
			return
		}

		if bg.Round == 1 {
			// Round-1 losers pair directly into losers-bracket round 1.
			lbPos := (bg.Position + 1) / 2
			lb := me.DB.GetBracketGameByRoundPosition(bracket.ID, "losers", 1, lbPos)
			if bg.Position%2 == 1 {
				me.DB.SetBracketGameTopTeam(lb.ID, loserTeamID)
			} else {
				me.DB.SetBracketGameBottomTeam(lb.ID, loserTeamID)
			}
			me.maybeCreateBracketGame(lb.ID)
		} else {
			// Later-round losers drop into the bottom (new-blood) slot of the
			// matching losers-bracket cross round.
			lbRound := 2 * (bg.Round - 1)
			lb := me.DB.GetBracketGameByRoundPosition(bracket.ID, "losers", lbRound, bg.Position)
			me.DB.SetBracketGameBottomTeam(lb.ID, loserTeamID)
			me.maybeCreateBracketGame(lb.ID)
		}

	case "losers":
		totalLBRounds := 2 * (winnersRounds - 1)
		if bg.Round == totalLBRounds {
			// Losers-bracket champion advances to the grand final's bottom slot.
			final := me.DB.GetBracketGameByRoundPosition(bracket.ID, "final", 1, 1)
			me.DB.SetBracketGameBottomTeam(final.ID, winnerTeamID)
			me.maybeCreateBracketGame(final.ID)
			return
		}
		if bg.Round%2 == 1 {
			// Odd round: winner carries forward into the top (survivor) slot of the
			// next (cross) round, same position — counts match, no merge.
			next := me.DB.GetBracketGameByRoundPosition(bracket.ID, "losers", bg.Round+1, bg.Position)
			me.DB.SetBracketGameTopTeam(next.ID, winnerTeamID)
			me.maybeCreateBracketGame(next.ID)
		} else {
			// Even (cross) round: winner merges with an adjacent winner into the
			// next (consolidation) round, like a single-elimination parent.
			nextPos := (bg.Position + 1) / 2
			next := me.DB.GetBracketGameByRoundPosition(bracket.ID, "losers", bg.Round+1, nextPos)
			if bg.Position%2 == 1 {
				me.DB.SetBracketGameTopTeam(next.ID, winnerTeamID)
			} else {
				me.DB.SetBracketGameBottomTeam(next.ID, winnerTeamID)
			}
			me.maybeCreateBracketGame(next.ID)
		}

	case "final":
		if bg.Round == 1 {
			if winnerTeamID == bg.TopTeamID {
				// Winners-bracket champion won outright.
				me.DB.SetBracketStatus(bracket.ID, "complete")
				return
			}
			// Losers-bracket entrant forced a bracket reset: replay the same two teams.
			resetID := me.DB.AddBracketGame(bracket.ID, 2, 1, "final")
			me.DB.SetBracketGameTeams(resetID, bg.TopTeamID, bg.BottomTeamID, false, false)
			me.maybeCreateBracketGame(resetID)
			return
		}
		// Bracket-reset game: whoever wins is the overall champion.
		me.DB.SetBracketStatus(bracket.ID, "complete")
	}
}

func (me *Env) PrintBracket(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/divisions/%d", t.ID, did), http.StatusSeeOther)
		return
	}

	size := bracket.Size
	winnersRounds := int(math.Log2(float64(size)))
	if winnersRounds == 0 {
		winnersRounds = 1
	}
	allGames := me.DB.GetBracketGames(bracket.ID)

	gamesBySide := make(map[string]map[int][]mydb.BracketGame)
	for _, bg := range allGames {
		if gamesBySide[bg.Side] == nil {
			gamesBySide[bg.Side] = make(map[int][]mydb.BracketGame)
		}
		gamesBySide[bg.Side][bg.Round] = append(gamesBySide[bg.Side][bg.Round], bg)
	}

	winnersLabel := roundLabel
	if bracket.Format == "double_elimination" {
		winnersLabel = func(rnd, total int) string { return "Winners " + roundLabel(rnd, total) }
	}
	rounds := buildBracketRounds(gamesBySide["winners"], winnersRounds, winnersLabel)

	var losersRounds, finalRounds []bracketRound
	if bracket.Format == "double_elimination" {
		totalLBRounds := 2 * (winnersRounds - 1)
		losersRounds = buildBracketRounds(gamesBySide["losers"], totalLBRounds, func(rnd, total int) string {
			if rnd == total {
				return "Losers Final"
			}
			return fmt.Sprintf("Losers Round %d", rnd)
		})

		totalFinalRounds := 1
		if len(gamesBySide["final"][2]) > 0 {
			totalFinalRounds = 2
		}
		finalRounds = buildBracketRounds(gamesBySide["final"], totalFinalRounds, func(rnd, total int) string {
			if rnd == 1 {
				return "Grand Final"
			}
			return "Bracket Reset"
		})
	}

	me.render(w, "bracket", bracketData{
		baseData:     newBaseWithTournament(r, t),
		Division:     div,
		Bracket:      bracket,
		Rounds:       rounds,
		LosersRounds: losersRounds,
		FinalRounds:  finalRounds,
		SeedLabel:    CriteriaRankingLabel(div.RankingCriteria),
	})
}

// buildBracketRounds assembles a []bracketRound for one side (winners, losers,
// or final) of a bracket, given its games keyed by round number.
func buildBracketRounds(gamesByRound map[int][]mydb.BracketGame, totalRounds int, label func(round, total int) string) []bracketRound {
	rounds := make([]bracketRound, totalRounds)
	for rnd := 1; rnd <= totalRounds; rnd++ {
		games := gamesByRound[rnd]
		var connectors []struct{}
		if rnd > 1 {
			connectors = make([]struct{}, len(games))
		}
		rounds[rnd-1] = bracketRound{
			Label:      label(rnd, totalRounds),
			Games:      games,
			Connectors: connectors,
		}
	}
	return rounds
}
