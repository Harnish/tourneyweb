package webhandler

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

// Suppress "imported and not used" errors for imports needed by later tasks.
var (
	_ = http.StatusOK
	_ = strconv.Atoi
	_ = strings.Split
	_ = time.Time{}
	_ httprouter.Params
	_ mydb.Bracket
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
