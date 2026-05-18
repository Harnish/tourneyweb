package webhandler

import (
	"math/rand"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

// gameSpec describes a single game to create.
// scrimmageFor is the team ID for whom this game doesn't count; 0 means it counts for both.
type gameSpec struct {
	homeTeamID   int
	awayTeamID   int
	scrimmageFor int
}

// roundRobinRounds generates rounds for n teams using the circle rotation algorithm.
// Each element is a pair of 0-based indices into the team slice.
// For odd n, a phantom "bye" slot at index n is added; callers skip pairs where either index >= n.
func roundRobinRounds(n int) [][][2]int {
	size := n
	if n%2 == 1 {
		size = n + 1
	}
	rounds := make([][][2]int, size-1)
	positions := make([]int, size)
	for i := range positions {
		positions[i] = i
	}
	for r := 0; r < size-1; r++ {
		round := make([][2]int, size/2)
		for i := 0; i < size/2; i++ {
			round[i] = [2]int{positions[i], positions[size-1-i]}
		}
		rounds[r] = round
		// Rotate: fix positions[0], rotate the rest
		last := positions[size-1]
		copy(positions[2:], positions[1:size-1])
		positions[1] = last
	}
	return rounds
}

// cappedSchedule returns randomized game specs where each team plays at most maxGames counting games.
//
// It uses the circle (Berger) round-robin rotation to guarantee perfect balance per round,
// then greedily accepts rounds until each team reaches maxGames. Shuffling both the round
// order and team assignment produces varied schedules.
//
// When len(teams)*maxGames is odd, perfect even distribution is mathematically impossible
// (a fractional game), so exactly one team will be one game short after the greedy pass.
// cappedSchedule appends one scrimmage game for that team: the game counts toward their
// standings but not toward the opponent's (scrimmageFor == opponentID).
func cappedSchedule(teams []mydb.Team, maxGames int) []gameSpec {
	n := len(teams)
	if n < 2 {
		return nil
	}

	// Shuffle teams so round-robin positions are assigned randomly.
	shuffled := make([]mydb.Team, n)
	copy(shuffled, teams)
	rand.Shuffle(n, func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	rounds := roundRobinRounds(n)

	// Shuffle round order for schedule variety.
	rand.Shuffle(len(rounds), func(i, j int) { rounds[i], rounds[j] = rounds[j], rounds[i] })

	counts := make(map[int]int, n)
	played := make(map[[2]int]bool) // canonical (smaller, larger) pairs already scheduled
	var specs []gameSpec

	for _, round := range rounds {
		rand.Shuffle(len(round), func(i, j int) { round[i], round[j] = round[j], round[i] })
		for _, pair := range round {
			idxA, idxB := pair[0], pair[1]
			// Skip bye slots (index >= n means no real team).
			if idxA >= n || idxB >= n {
				continue
			}
			a, b := shuffled[idxA].ID, shuffled[idxB].ID
			if counts[a] < maxGames && counts[b] < maxGames {
				specs = append(specs, gameSpec{homeTeamID: a, awayTeamID: b})
				counts[a]++
				counts[b]++
				pa, pb := a, b
				if pa > pb {
					pa, pb = pb, pa
				}
				played[[2]int{pa, pb}] = true
			}
		}
	}

	// Odd-total handling: find the one team that is one game short, if any.
	var shortTeam int
	for _, t := range shuffled {
		if counts[t.ID] < maxGames {
			if shortTeam != 0 {
				// More than one short team — cannot fix with a single scrimmage.
				shortTeam = 0
				break
			}
			shortTeam = t.ID
		}
	}

	if shortTeam != 0 {
		// Prefer a fresh opponent (not yet played); fall back to any at-cap opponent.
		var freshOpponents []int
		var atCap []int
		for _, t := range shuffled {
			if t.ID == shortTeam {
				continue
			}
			if counts[t.ID] >= maxGames {
				atCap = append(atCap, t.ID)
				pa, pb := shortTeam, t.ID
				if pa > pb {
					pa, pb = pb, pa
				}
				if !played[[2]int{pa, pb}] {
					freshOpponents = append(freshOpponents, t.ID)
				}
			}
		}
		candidates := freshOpponents
		if len(candidates) == 0 {
			candidates = atCap
		}
		if len(candidates) > 0 {
			opponent := candidates[rand.Intn(len(candidates))]
			// Short team is home (counts for them); opponent's game is a scrimmage.
			specs = append(specs, gameSpec{
				homeTeamID:   shortTeam,
				awayTeamID:   opponent,
				scrimmageFor: opponent,
			})
		}
	}

	return specs
}
