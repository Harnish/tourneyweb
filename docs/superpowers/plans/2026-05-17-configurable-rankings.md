# Configurable Rankings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let Tournament Directors configure the tiebreaker order for each division's standings via an ordered criteria list stored as JSON in the database.

**Architecture:** A `ranking_criteria TEXT` column on `divisions` stores an ordered JSON array of criterion keys. The ranking engine in `webhandler/sortteams.go` iterates the list and applies independent comparator functions in sequence — first non-zero wins. Directors configure the order on the division edit form with up/down buttons and checkboxes.

**Tech Stack:** Go, PostgreSQL, html/template, vanilla JS (no new dependencies)

---

## File Map

| File | Change |
|---|---|
| `mydb/divisions.go` | Add `RankingCriteria []string` to `Division` struct; update SELECT/UPDATE queries; add `parseCriteria` helper; `DefaultRankingCriteria` var |
| `mydb/games.go` | Add `RunsAllowedInHeadToHead(teamAID, teamBID int) (int, bool)` on `*MyDB` |
| `mydb/db.go` | Update `UpdateDivision` signature; add `RunsAllowedInHeadToHead` to interface |
| `mydb/fakedb.go` | Update `UpdateDivision` to store `RankingCriteria`; add `FakeDB.RunsAllowedInHeadToHead` |
| `mydb/mydb.go` | Add `ALTER TABLE divisions ADD COLUMN IF NOT EXISTS ranking_criteria TEXT` to migrations |
| `mydb/fakedb_test.go` | Add `TestFakeDB_RankingCriteria` and `TestFakeDB_RunsAllowedInHeadToHead` |
| `webhandler/sortteams.go` | Rewrite: new `SortTeams([]string)` signature, criterion registry, 9 criterion functions, `AllCriteriaForUI`, `CriteriaRankingLabel`; keep `Wins`/`RunsAgainst`/`RunsFor` helpers |
| `webhandler/sortteams_test.go` | Update all `SortTeams` calls from string algo names to `[]string` slices; add new criterion tests |
| `webhandler/templates.go` | Add `RankingLabel string` to `divisionData`; add `AllCriteria []CriterionUIRow` to `manageDivisionEditData`; `CriterionUIRow` type |
| `webhandler/webhandler.go` | Update `PrintDivision`: pass `div.RankingCriteria` to `SortTeams`; populate `RankingLabel` |
| `webhandler/manage_divisions.go` | POST handler: parse `ranking_criteria` form field; call `UpdateDivision` with criteria; import `strings` |
| `webhandler/templates/manage/edit_division.html` | Add ranking criteria ordered list with checkboxes and up/down buttons; merge submit handler |
| `webhandler/templates/divisions.html` | Add "Ranked by: …" line below standings table |

---

## Task 1: DB layer — Division.RankingCriteria column

**Files:**
- Modify: `mydb/mydb.go`
- Modify: `mydb/divisions.go`
- Modify: `mydb/db.go`
- Modify: `mydb/fakedb.go`
- Test: `mydb/fakedb_test.go`

- [ ] **Step 1: Write the failing test**

Add `TestFakeDB_RankingCriteria` to `mydb/fakedb_test.go`:

```go
func TestFakeDB_RankingCriteria(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "published")

	// AddDivision creates a division with the default criteria
	db.AddDivision(tid, "12U")
	divs := db.ReturnDivisions(tid)
	if len(divs) != 1 {
		t.Fatalf("expected 1 division, got %d", len(divs))
	}
	did := divs[0].ID

	// Default criteria should be populated (not nil/empty)
	got := db.ReturnDivisionByID(did)
	if len(got.RankingCriteria) == 0 {
		t.Error("expected non-empty default RankingCriteria")
	}
	if got.RankingCriteria[0] != "wins" {
		t.Errorf("first default criterion: got %q, want \"wins\"", got.RankingCriteria[0])
	}

	// UpdateDivision stores custom criteria
	custom := []string{"wins", "run_differential", "coin_flip"}
	db.UpdateDivision(did, "12U", custom)
	got = db.ReturnDivisionByID(did)
	if len(got.RankingCriteria) != 3 {
		t.Fatalf("after update: got %d criteria, want 3", len(got.RankingCriteria))
	}
	if got.RankingCriteria[1] != "run_differential" {
		t.Errorf("second criterion: got %q, want \"run_differential\"", got.RankingCriteria[1])
	}

	// ReturnDivisions also returns RankingCriteria
	divs = db.ReturnDivisions(tid)
	if len(divs[0].RankingCriteria) != 3 {
		t.Errorf("ReturnDivisions RankingCriteria: got %d criteria, want 3", len(divs[0].RankingCriteria))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./mydb/... -run TestFakeDB_RankingCriteria -v
```

Expected: FAIL — `RankingCriteria` field does not exist yet.

- [ ] **Step 3: Add migration to mydb/mydb.go**

In `mydb/mydb.go`, find the migrations slice (the `[]string` of ALTER TABLE statements near the bottom). Add after the existing `rules_html` migration:

```go
`ALTER TABLE divisions ADD COLUMN IF NOT EXISTS ranking_criteria TEXT`,
```

- [ ] **Step 4: Rewrite mydb/divisions.go**

Replace the entire file with:

```go
package mydb

import (
	"database/sql"
	"encoding/json"
	"log/slog"
)

// DefaultRankingCriteria is the fallback order used when a division has no
// stored ranking configuration.
var DefaultRankingCriteria = []string{"wins", "head_to_head", "runs_against", "runs_for"}

type Division struct {
	ID              int
	TournamentID    int
	Name            string
	RulesHTML       string
	RankingCriteria []string
}

func parseCriteria(s sql.NullString) []string {
	if !s.Valid || s.String == "" {
		return DefaultRankingCriteria
	}
	var out []string
	if err := json.Unmarshal([]byte(s.String), &out); err != nil || len(out) == 0 {
		return DefaultRankingCriteria
	}
	return out
}

func (me *MyDB) AddDivision(tournamentID int, name string) {
	_, err := me.DB.Exec(
		`INSERT INTO divisions (tournament_id, name) VALUES ($1,$2)`,
		tournamentID, name,
	)
	if err != nil {
		slog.Error("AddDivision", "err", err)
	}
}

func (me *MyDB) DelDivision(id int) {
	me.DB.Exec(`DELETE FROM divisions WHERE id=$1`, id)
}

func (me *MyDB) UpdateDivision(id int, name string, criteria []string) {
	b, _ := json.Marshal(criteria)
	_, err := me.DB.Exec(
		`UPDATE divisions SET name=$1, ranking_criteria=$2 WHERE id=$3`,
		name, string(b), id,
	)
	if err != nil {
		slog.Error("UpdateDivision", "err", err)
	}
}

func (me *MyDB) ReturnDivisions(tournamentID int) []Division {
	rows, err := me.DB.Query(
		`SELECT id, tournament_id, name, rules_html, ranking_criteria FROM divisions WHERE tournament_id=$1 ORDER BY name`,
		tournamentID,
	)
	if err != nil {
		slog.Error("ReturnDivisions", "err", err)
		return nil
	}
	var out []Division
	for rows.Next() {
		var d Division
		var crit sql.NullString
		if err := rows.Scan(&d.ID, &d.TournamentID, &d.Name, &d.RulesHTML, &crit); err != nil {
			slog.Error("ReturnDivisions scan", "err", err)
			continue
		}
		d.RankingCriteria = parseCriteria(crit)
		out = append(out, d)
	}
	rows.Close()
	return out
}

func (me *MyDB) ReturnDivisionByID(id int) Division {
	var d Division
	var crit sql.NullString
	err := me.DB.QueryRow(
		`SELECT id, tournament_id, name, rules_html, ranking_criteria FROM divisions WHERE id=$1`, id,
	).Scan(&d.ID, &d.TournamentID, &d.Name, &d.RulesHTML, &crit)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("ReturnDivisionByID", "err", err)
	}
	d.RankingCriteria = parseCriteria(crit)
	return d
}

func (me *MyDB) SetDivisionRules(id int, html string) {
	_, err := me.DB.Exec(`UPDATE divisions SET rules_html=$1 WHERE id=$2`, html, id)
	if err != nil {
		slog.Error("SetDivisionRules", "err", err)
	}
}
```

- [ ] **Step 5: Update DB interface in mydb/db.go**

Change the `UpdateDivision` line from:
```go
UpdateDivision(id int, name string)
```
to:
```go
UpdateDivision(id int, name string, criteria []string)
```

Also add `RunsAllowedInHeadToHead` to the Games section (will be implemented in Task 2):
```go
RunsAllowedInHeadToHead(teamAID, teamBID int) (int, bool)
```

- [ ] **Step 6: Update FakeDB in mydb/fakedb.go**

Replace the `UpdateDivision` method:

```go
func (f *FakeDB) UpdateDivision(id int, name string, criteria []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d, ok := f.divisions[id]; ok {
		d.Name = name
		if len(criteria) == 0 {
			d.RankingCriteria = DefaultRankingCriteria
		} else {
			d.RankingCriteria = make([]string, len(criteria))
			copy(d.RankingCriteria, criteria)
		}
		f.divisions[id] = d
	}
}
```

Also update `AddDivision` in `fakedb.go` to initialise `RankingCriteria`:

```go
func (f *FakeDB) AddDivision(tournamentID int, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	f.divisions[id] = Division{
		ID:              id,
		TournamentID:    tournamentID,
		Name:            name,
		RankingCriteria: DefaultRankingCriteria,
	}
}
```

Add the stub for `RunsAllowedInHeadToHead` so the interface is satisfied (full implementation in Task 2):

```go
func (f *FakeDB) RunsAllowedInHeadToHead(teamAID, teamBID int) (int, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	total := 0
	found := false
	for _, r := range f.gamesByTeam {
		if r.teamID == teamAID && r.opponentID == teamBID {
			total += r.opponentScore
			found = true
		}
	}
	return total, found
}
```

- [ ] **Step 7: Fix the compile error in webhandler/manage_divisions.go**

The `UpdateDivision` call signature changed. Temporarily pass the default criteria so it compiles while Task 3 adds proper parsing. In `manage_divisions.go`, find the POST handler in `ManageDivisionEdit` and replace:

```go
me.DB.UpdateDivision(did, name)
```

with:

```go
me.DB.UpdateDivision(did, name, mydb.DefaultRankingCriteria)
```

Add the import for `mydb` if not already present:
```go
"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
```

- [ ] **Step 8: Run test and build**

```bash
go build ./... && go test ./mydb/... -run TestFakeDB_RankingCriteria -v
```

Expected: build succeeds, `TestFakeDB_RankingCriteria` PASS.

- [ ] **Step 9: Run all tests**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 10: Commit**

```bash
git add mydb/divisions.go mydb/db.go mydb/fakedb.go mydb/mydb.go mydb/fakedb_test.go webhandler/manage_divisions.go
git commit -m "feat: add ranking_criteria column to divisions; update DB interface"
```

---

## Task 2: RunsAllowedInHeadToHead DB method

**Files:**
- Modify: `mydb/games.go`
- Test: `mydb/fakedb_test.go`

- [ ] **Step 1: Write the failing test**

Add `TestFakeDB_RunsAllowedInHeadToHead` to `mydb/fakedb_test.go`:

```go
func TestFakeDB_RunsAllowedInHeadToHead(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "published")
	db.AddDivision(tid, "12U")
	divs := db.ReturnDivisions(tid)
	did := divs[0].ID

	db.AddTeam(tid, did, "Red", "")
	db.AddTeam(tid, did, "Blue", "")
	db.AddTeam(tid, did, "Green", "")
	teams := db.ReturnTeamsByDivisionID(did)
	var red, blue, green mydb.Team
	for _, t2 := range teams {
		switch t2.Name {
		case "Red":
			red = t2
		case "Blue":
			blue = t2
		case "Green":
			green = t2
		}
	}

	db.AddGame(tid, did, red.ID, blue.ID, "", time.Time{}, "")
	games := db.AllGamesByDivision(did)
	db.ScoreGame(games[0].ID, 5, 3) // Red 5, Blue 3

	// Red allowed 3 runs vs Blue; Blue allowed 5 runs vs Red
	ra, ok := db.RunsAllowedInHeadToHead(red.ID, blue.ID)
	if !ok {
		t.Error("expected found=true for Red vs Blue")
	}
	if ra != 3 {
		t.Errorf("Red allowed vs Blue: got %d, want 3", ra)
	}

	ra, ok = db.RunsAllowedInHeadToHead(blue.ID, red.ID)
	if !ok {
		t.Error("expected found=true for Blue vs Red")
	}
	if ra != 5 {
		t.Errorf("Blue allowed vs Red: got %d, want 5", ra)
	}

	// Teams that never played: found=false
	_, ok = db.RunsAllowedInHeadToHead(red.ID, green.ID)
	if ok {
		t.Error("expected found=false for teams that never played")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./mydb/... -run TestFakeDB_RunsAllowedInHeadToHead -v
```

Expected: FAIL — method not yet on `*MyDB`.

- [ ] **Step 3: Add RunsAllowedInHeadToHead to mydb/games.go**

Open `mydb/games.go` and append at the bottom:

```go
// RunsAllowedInHeadToHead returns the total runs teamBID scored against teamAID
// across all scored games between them, and whether they played at all.
func (me *MyDB) RunsAllowedInHeadToHead(teamAID, teamBID int) (int, bool) {
	var total sql.NullInt64
	err := me.DB.QueryRow(
		`SELECT SUM(opponent_score) FROM games_by_team WHERE team_id=$1 AND opponent_id=$2`,
		teamAID, teamBID,
	).Scan(&total)
	if err != nil || !total.Valid {
		return 0, false
	}
	return int(total.Int64), true
}
```

`sql` is already imported in `mydb/games.go`. If it is not, add `"database/sql"` to the import block.

- [ ] **Step 4: Run test**

```bash
go test ./mydb/... -run TestFakeDB_RunsAllowedInHeadToHead -v
```

Expected: PASS.

- [ ] **Step 5: Run all tests**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add mydb/games.go mydb/fakedb_test.go
git commit -m "feat: add RunsAllowedInHeadToHead DB method"
```

---

## Task 3: Ranking engine rewrite

**Files:**
- Modify: `webhandler/sortteams.go`
- Modify: `webhandler/sortteams_test.go`
- Modify: `webhandler/webhandler.go`
- Modify: `webhandler/templates.go`

- [ ] **Step 1: Write failing tests for new criteria and the new SortTeams signature**

Add these tests to `webhandler/sortteams_test.go` (after existing tests). The existing tests will also need updating in Step 4 — do NOT change them yet, just add these:

```go
// makeTeamL builds a Team with wins, losses, and run stats.
func makeTeamL(id, wins, losses, runsFor, runsAgainst int) mydb.Team {
	return mydb.Team{ID: id, Wins: wins, Losses: losses, RunsFor: runsFor, RunsAgainst: runsAgainst}
}

func TestCriterionWinPct(t *testing.T) {
	env := makeEnv()
	// Team 1: 2W 0L = 1.0; Team 2: 1W 1L = 0.5
	teams := []mydb.Team{
		makeTeamL(2, 1, 1, 5, 5),
		makeTeamL(1, 2, 0, 5, 5),
	}
	got := ids(env.SortTeams(teams, []string{"win_pct"}))
	if got[0] != 1 {
		t.Errorf("higher win_pct should rank first; got %v", got)
	}
}

func TestCriterionRunDifferential(t *testing.T) {
	env := makeEnv()
	// Team 1: +10 diff; Team 2: +3 diff
	teams := []mydb.Team{
		makeTeamL(2, 2, 0, 8, 5),
		makeTeamL(1, 2, 0, 15, 5),
	}
	got := ids(env.SortTeams(teams, []string{"run_differential"}))
	if got[0] != 1 {
		t.Errorf("higher run diff should rank first; got %v", got)
	}
}

func TestCriterionRunsAgainstPerGame(t *testing.T) {
	env := makeEnv()
	// Team 1: 6 RA / 2G = 3.0; Team 2: 12 RA / 2G = 6.0
	teams := []mydb.Team{
		makeTeamL(2, 1, 1, 10, 12),
		makeTeamL(1, 1, 1, 10, 6),
	}
	got := ids(env.SortTeams(teams, []string{"runs_against_per_game"}))
	if got[0] != 1 {
		t.Errorf("fewer RA/game should rank first; got %v", got)
	}
}

func TestCriterionHeadToHeadRunsAgainst(t *testing.T) {
	db := mydb.NewFakeDB()
	env := &Env{DB: db}
	// Team 1 allowed 2 runs vs Team 2; Team 2 allowed 5 runs vs Team 1
	db.AddTeamScore(0, 0, 1, 2, 999, 5, 2) // game 999: T1 scored 5, T2 scored 2
	db.AddTeamScore(0, 0, 2, 1, 999, 2, 5)
	teams := []mydb.Team{
		makeTeam(2, 2, 10, 5),
		makeTeam(1, 2, 10, 5),
	}
	got := ids(env.SortTeams(teams, []string{"head_to_head_runs_against"}))
	if got[0] != 1 {
		t.Errorf("fewer H2H runs allowed should rank first; got %v", got)
	}
}

func TestCriterionCoinFlip(t *testing.T) {
	env := makeEnv()
	// Stable sort by team ID ascending
	teams := []mydb.Team{makeTeam(3, 2, 10, 5), makeTeam(1, 2, 10, 5), makeTeam(2, 2, 10, 5)}
	got := ids(env.SortTeams(teams, []string{"coin_flip"}))
	want := []int{1, 2, 3}
	for i, id := range want {
		if got[i] != id {
			t.Errorf("coin_flip position %d: got %d, want %d (full: %v)", i, got[i], id, got)
		}
	}
}

func TestSortTeams_MultiCriteria(t *testing.T) {
	db := mydb.NewFakeDB()
	env := &Env{DB: db}
	// Team 1 and 2 tied on wins; 1 beat 2 head-to-head
	recordH2H(db, 1, 2)
	teams := []mydb.Team{
		makeTeam(2, 3, 10, 5),
		makeTeam(1, 3, 10, 5),
	}
	got := ids(env.SortTeams(teams, []string{"wins", "head_to_head", "runs_against"}))
	if got[0] != 1 {
		t.Errorf("h2h winner should rank first; got %v", got)
	}
}

func TestSortTeams_UnknownKeyIgnored(t *testing.T) {
	env := makeEnv()
	teams := []mydb.Team{makeTeam(2, 3, 10, 5), makeTeam(1, 2, 10, 5)}
	// Unknown key should be silently skipped; falls through to no result → stable order
	got := ids(env.SortTeams(teams, []string{"wins", "nonexistent_key"}))
	if got[0] != 2 {
		t.Errorf("wins criterion should still apply; got %v", got)
	}
}
```

- [ ] **Step 2: Run new tests to verify they fail**

```bash
go test ./webhandler/... -run "TestCriterionWinPct|TestCriterionRunDiff|TestCriterionRunsAgainstPerGame|TestCriterionHeadToHeadRunsAgainst|TestCriterionCoinFlip|TestSortTeams_MultiCriteria|TestSortTeams_UnknownKeyIgnored" -v
```

Expected: FAIL — `SortTeams` still takes a string, not `[]string`.

- [ ] **Step 3: Rewrite webhandler/sortteams.go**

Replace the entire file with:

```go
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
```

- [ ] **Step 4: Update existing tests in webhandler/sortteams_test.go to use []string**

The tests that call `SortTeams` with a string must be updated. The mapping is:

- `"WinsRunsAgainstRunsEarnedHead2Head"` → `[]string{"wins", "runs_against", "runs_for", "head_to_head"}`
- `"WinsHead2HeadRunsAgainstRunsEarned"` → `[]string{"wins", "head_to_head", "runs_against", "runs_for"}`

Find every occurrence in `webhandler/sortteams_test.go` and replace:

```go
// Replace all 5 occurrences of:
env.SortTeams(teams, "WinsRunsAgainstRunsEarnedHead2Head")
// with:
env.SortTeams(teams, []string{"wins", "runs_against", "runs_for", "head_to_head"})

// Replace all 6 occurrences of:
env.SortTeams(teams, "WinsHead2HeadRunsAgainstRunsEarned")
// with:
env.SortTeams(teams, []string{"wins", "head_to_head", "runs_against", "runs_for"})
```

- [ ] **Step 5: Update webhandler/templates.go**

Add `CriterionUIRow` type and update the two division data structs. Find `divisionData` and `manageDivisionEditData` and replace them:

```go
type divisionData struct {
	baseData
	Division     mydb.Division
	Teams        []divisionTeamRow
	Games        []mydb.Game
	RankingLabel string
}

type manageDivisionEditData struct {
	baseData
	Division    mydb.Division
	AllCriteria []CriterionUIRow
}
```

- [ ] **Step 6: Update PrintDivision in webhandler/webhandler.go**

Find the `PrintDivision` function. It currently calls:
```go
rawTeams := me.DB.ReturnTeamsByDivisionIDWithStats(did)
rawTeams = me.SortTeams(rawTeams, "WinsRunsAgainstRunsEarnedHead2Head")
```
and renders with:
```go
me.render(w, "divisions", divisionData{
    baseData: newBaseWithTournament(r, t),
    Division: me.DB.ReturnDivisionByID(did),
    Teams:    rows,
    Games:    me.DB.AllGamesByDivision(did),
})
```

Replace this block with:

```go
rawTeams := me.DB.ReturnTeamsByDivisionIDWithStats(did)
div := me.DB.ReturnDivisionByID(did)
rawTeams = me.SortTeams(rawTeams, div.RankingCriteria)
rows := make([]divisionTeamRow, len(rawTeams))
for i, team := range rawTeams {
    rows[i] = divisionTeamRow{Team: team, GamesPlayed: me.DB.GamesPlayedByTeam(team.ID)}
}
me.render(w, "divisions", divisionData{
    baseData:     newBaseWithTournament(r, t),
    Division:     div,
    Teams:        rows,
    Games:        me.DB.AllGamesByDivision(did),
    RankingLabel: CriteriaRankingLabel(div.RankingCriteria),
})
```

Remove the old `rows` construction that follows (it was already building `rows` before the render call; this replacement includes the rows loop so the old one is now duplicate — delete it).

- [ ] **Step 7: Build and run all tests**

```bash
go build ./... && go test ./...
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

```bash
git add webhandler/sortteams.go webhandler/sortteams_test.go webhandler/templates.go webhandler/webhandler.go
git commit -m "feat: configurable ranking engine with 9 criteria"
```

---

## Task 4: Director UI and public display

**Files:**
- Modify: `webhandler/manage_divisions.go`
- Modify: `webhandler/templates/manage/edit_division.html`
- Modify: `webhandler/templates/divisions.html`

- [ ] **Step 1: Update manage_divisions.go POST handler**

In `ManageDivisionEdit`, the POST branch currently ends with:

```go
me.DB.UpdateDivision(did, name, mydb.DefaultRankingCriteria)
me.DB.SetDivisionRules(did, r.FormValue("rules_html"))
```

Replace it with:

```go
criteriaStr := r.FormValue("ranking_criteria")
var criteria []string
for _, k := range strings.Split(criteriaStr, ",") {
    k = strings.TrimSpace(k)
    if _, ok := criteriaRegistry[k]; ok {
        criteria = append(criteria, k)
    }
}
if len(criteria) == 0 {
    criteria = mydb.DefaultRankingCriteria
}
me.DB.UpdateDivision(did, name, criteria)
me.DB.SetDivisionRules(did, r.FormValue("rules_html"))
```

Add `"strings"` to the import block in `manage_divisions.go`:

```go
import (
    "fmt"
    "log/slog"
    "net/http"
    "strconv"
    "strings"

    "github.com/julienschmidt/httprouter"
    "gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)
```

Update the GET render call in `ManageDivisionEdit` to populate `AllCriteria`:

```go
me.render(w, "manageDivisionEdit", manageDivisionEditData{
    baseData:    newBaseWithTournament(r, t),
    Division:    division,
    AllCriteria: AllCriteriaForUI(division.RankingCriteria),
})
```

- [ ] **Step 2: Build to verify**

```bash
go build ./...
```

Expected: builds cleanly.

- [ ] **Step 3: Update webhandler/templates/manage/edit_division.html**

Replace the entire file with:

```html
{{define "manageDivisionEdit"}}
{{template "header" .}}
<link rel="stylesheet" href="https://cdn.quilljs.com/1.3.7/quill.snow.css">
<h1>Edit Division</h1>
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/divisions/{{.Division.ID}}/edit" id="division-form">
{{.CSRFField}}
<table>
  <tr><td>Name</td><td><input type="text" name="name" value="{{.Division.Name}}" required></td></tr>
</table>

<h3>Standings Ranking Order</h3>
<p>Check criteria to include them. Use ↑/↓ to set the tiebreaker order (top = first tiebreaker).</p>
<ul id="ranking-list" style="list-style:none;padding:0;max-width:500px;">
{{range .AllCriteria}}
<li style="display:flex;align-items:center;gap:0.5em;margin-bottom:0.3em;padding:0.5em 0.75em;background:var(--tw-bg-card);border:1px solid var(--tw-border);border-radius:4px;">
  <input type="checkbox" value="{{.Key}}"{{if .Checked}} checked{{end}}>
  <span style="flex:1">{{.Label}}</span>
  <button type="button" onclick="moveUp(this)" style="padding:0 6px;background:none;border:1px solid var(--tw-border);border-radius:3px;color:var(--tw-text);cursor:pointer;">↑</button>
  <button type="button" onclick="moveDown(this)" style="padding:0 6px;background:none;border:1px solid var(--tw-border);border-radius:3px;color:var(--tw-text);cursor:pointer;">↓</button>
</li>
{{end}}
</ul>
<input type="hidden" name="ranking_criteria" id="ranking_criteria_input">

<h3>Division Rules</h3>
<p>Optional. If left empty, the tournament rules (if any) will be shown on this division's public page.</p>
<div id="editor" style="height:300px;margin-bottom:1em;"></div>
<textarea id="existing_content" style="display:none">{{.Division.RulesHTML}}</textarea>
<input type="hidden" name="rules_html" id="rules_html_input">
<input type="submit" value="Save">
&nbsp;<a href="/tournaments/{{.Tournament.ID}}/manage/divisions">Back</a>
</form>
<script src="https://cdn.quilljs.com/1.3.7/quill.js"></script>
<script>
function moveUp(btn) {
  var li = btn.closest('li');
  var prev = li.previousElementSibling;
  if (prev) li.parentNode.insertBefore(li, prev);
}
function moveDown(btn) {
  var li = btn.closest('li');
  var next = li.nextElementSibling;
  if (next) li.parentNode.insertBefore(next, li);
}

var quill = new Quill('#editor', {
  theme: 'snow',
  modules: { toolbar: [
    [{ 'header': [1, 2, 3, false] }],
    ['bold', 'italic', 'underline'],
    ['link'],
    [{ 'list': 'ordered' }, { 'list': 'bullet' }],
    ['clean']
  ]}
});
var existing = document.getElementById('existing_content').value;
if (existing.trim()) {
  quill.clipboard.dangerouslyPasteHTML(existing);
}

document.getElementById('division-form').addEventListener('submit', function() {
  // Collect checked criteria in DOM order
  var checks = document.querySelectorAll('#ranking-list input[type=checkbox]');
  var active = [];
  checks.forEach(function(cb) {
    if (cb.checked) active.push(cb.value);
  });
  document.getElementById('ranking_criteria_input').value = active.join(',');

  // Collect Quill content
  var html = quill.root.innerHTML;
  if (html === '<p><br></p>') html = '';
  document.getElementById('rules_html_input').value = html;
});
</script>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 4: Update webhandler/templates/divisions.html**

Add the "Ranked by" line after the closing `</table>` of the standings table (after `{{end}}` that closes the `{{range .Teams}}` block). Find this section:

```html
</table>
<h2>Games</h2>
```

Replace with:

```html
</table>
{{if .RankingLabel}}
<p style="font-size:0.85em;color:var(--tw-muted);margin-top:0.25em;">Ranked by: {{.RankingLabel}}</p>
{{end}}
<h2>Games</h2>
```

- [ ] **Step 5: Build and run all tests**

```bash
go build ./... && go test ./...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add webhandler/manage_divisions.go webhandler/templates/manage/edit_division.html webhandler/templates/divisions.html
git commit -m "feat: ranking criteria UI on division edit form; public ranked-by display"
```

---

## Self-Review Checklist

After all tasks are complete, verify:

- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes with no failures
- [ ] Edit a division → ranking criteria section visible with all 9 criteria
- [ ] Reorder and save → public division page shows updated "Ranked by:" line
- [ ] Division with no custom criteria shows default "Most wins → Head-to-head record → Fewest runs allowed (total) → Most runs scored"
- [ ] Unknown criteria keys in DB are silently skipped by the engine
