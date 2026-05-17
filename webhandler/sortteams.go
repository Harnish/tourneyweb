package webhandler

import (
	"sort"
	"strings"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

// criterionFn compares two teams on a single criterion.
// Returns -1 if a ranks higher, 1 if b ranks higher, 0 if tied.
type criterionFn func(a, b mydb.Team, db mydb.DB) int

var criteriaRegistry = map[string]criterionFn{
	"wins":                      criterionWins,
	"win_pct":                   criterionWinPct,
	"head_to_head":              criterionHeadToHead,
	"run_differential":          criterionRunDifferential,
	"runs_against":              criterionRunsAgainst,
	"runs_against_per_game":     criterionRunsAgainstPerGame,
	"runs_for":                  criterionRunsFor,
	"head_to_head_runs_against": criterionHeadToHeadRunsAgainst,
	"coin_flip":                 criterionCoinFlip,
}

// orderedCriteriaKeys is the canonical display order for the UI.
var orderedCriteriaKeys = []string{
	"wins",
	"win_pct",
	"head_to_head",
	"run_differential",
	"runs_against",
	"runs_against_per_game",
	"runs_for",
	"head_to_head_runs_against",
	"coin_flip",
}

var criteriaLabels = map[string]string{
	"wins":                      "Most wins",
	"win_pct":                   "Best win percentage",
	"head_to_head":              "Head-to-head record",
	"run_differential":          "Best run differential",
	"runs_against":              "Fewest runs allowed (total)",
	"runs_against_per_game":     "Fewest runs allowed per game",
	"runs_for":                  "Most runs scored",
	"head_to_head_runs_against": "Fewest runs allowed head-to-head",
	"coin_flip":                 "Stable draw (by team ID)",
}

// CriterionUIRow is used by the division edit form.
type CriterionUIRow struct {
	Key     string
	Label   string
	Checked bool
}

// AllCriteriaForUI returns all criteria for the edit form: active ones first
// (in configured order, checked=true), then remaining ones (checked=false).
func AllCriteriaForUI(active []string) []CriterionUIRow {
	activeSet := make(map[string]bool, len(active))
	for _, k := range active {
		activeSet[k] = true
	}
	var rows []CriterionUIRow
	for _, k := range active {
		rows = append(rows, CriterionUIRow{Key: k, Label: criteriaLabels[k], Checked: true})
	}
	for _, k := range orderedCriteriaKeys {
		if !activeSet[k] {
			rows = append(rows, CriterionUIRow{Key: k, Label: criteriaLabels[k], Checked: false})
		}
	}
	return rows
}

// CriteriaRankingLabel formats a criteria slice as "Label1 → Label2 → …"
func CriteriaRankingLabel(criteria []string) string {
	labels := make([]string, 0, len(criteria))
	for _, k := range criteria {
		if l, ok := criteriaLabels[k]; ok {
			labels = append(labels, l)
		}
	}
	return strings.Join(labels, " → ")
}

// SortTeams sorts teams in place by the given criteria applied in sequence.
// The first criterion that differentiates two teams determines their order.
func (me *Env) SortTeams(teams []mydb.Team, criteria []string) []mydb.Team {
	sort.SliceStable(teams, func(i, j int) bool {
		for _, key := range criteria {
			fn, ok := criteriaRegistry[key]
			if !ok {
				continue
			}
			result := fn(teams[i], teams[j], me.DB)
			if result != 0 {
				return result < 0
			}
		}
		return false
	})
	return teams
}

func criterionWins(a, b mydb.Team, _ mydb.DB) int {
	eq, better := Wins(a, b)
	if eq {
		return 0
	}
	if better {
		return -1
	}
	return 1
}

func criterionWinPct(a, b mydb.Team, _ mydb.DB) int {
	pctA := winPct(a)
	pctB := winPct(b)
	if pctA > pctB {
		return -1
	}
	if pctA < pctB {
		return 1
	}
	return 0
}

func winPct(t mydb.Team) float64 {
	total := t.Wins + t.Losses
	if total == 0 {
		return 0
	}
	return float64(t.Wins) / float64(total)
}

func criterionHeadToHead(a, b mydb.Team, db mydb.DB) int {
	played, aWon := db.DidTeamABeatTeamB(a.ID, b.ID)
	if !played {
		return 0
	}
	if aWon {
		return -1
	}
	return 1
}

func criterionRunDifferential(a, b mydb.Team, _ mydb.DB) int {
	diffA := a.RunsFor - a.RunsAgainst
	diffB := b.RunsFor - b.RunsAgainst
	if diffA > diffB {
		return -1
	}
	if diffA < diffB {
		return 1
	}
	return 0
}

func criterionRunsAgainst(a, b mydb.Team, _ mydb.DB) int {
	eq, better := RunsAgainst(a, b)
	if eq {
		return 0
	}
	if better {
		return -1
	}
	return 1
}

func criterionRunsAgainstPerGame(a, b mydb.Team, _ mydb.DB) int {
	rpgA := runsAgainstPerGame(a)
	rpgB := runsAgainstPerGame(b)
	if rpgA < rpgB {
		return -1
	}
	if rpgA > rpgB {
		return 1
	}
	return 0
}

func runsAgainstPerGame(t mydb.Team) float64 {
	total := t.Wins + t.Losses
	if total == 0 {
		return 0
	}
	return float64(t.RunsAgainst) / float64(total)
}

func criterionRunsFor(a, b mydb.Team, _ mydb.DB) int {
	eq, better := RunsFor(a, b)
	if eq {
		return 0
	}
	if better {
		return -1
	}
	return 1
}

func criterionHeadToHeadRunsAgainst(a, b mydb.Team, db mydb.DB) int {
	raA, played := db.RunsAllowedInHeadToHead(a.ID, b.ID)
	if !played {
		return 0
	}
	raB, _ := db.RunsAllowedInHeadToHead(b.ID, a.ID)
	if raA < raB {
		return -1
	}
	if raA > raB {
		return 1
	}
	return 0
}

func criterionCoinFlip(a, b mydb.Team, _ mydb.DB) int {
	if a.ID < b.ID {
		return -1
	}
	if a.ID > b.ID {
		return 1
	}
	return 0
}

// RunsAgainst compares two teams' RunsAgainst.
// Returns (equal bool, aRanksHigher bool).
func RunsAgainst(teama, teamb mydb.Team) (bool, bool) {
	if teama.RunsAgainst < teamb.RunsAgainst {
		return false, true
	} else if teama.RunsAgainst > teamb.RunsAgainst {
		return false, false
	}
	return true, true
}

// Wins compares two teams' Wins.
// Returns (equal bool, aRanksHigher bool).
func Wins(teama, teamb mydb.Team) (bool, bool) {
	if teama.Wins > teamb.Wins {
		return false, true
	} else if teama.Wins < teamb.Wins {
		return false, false
	}
	return true, true
}

// RunsFor compares two teams' RunsFor.
// Returns (equal bool, aRanksHigher bool).
func RunsFor(teama, teamb mydb.Team) (bool, bool) {
	if teama.RunsFor > teamb.RunsFor {
		return false, true
	} else if teama.RunsFor < teamb.RunsFor {
		return false, false
	}
	return true, true
}
