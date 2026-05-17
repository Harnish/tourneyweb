# Configurable Rankings Design

## Goal

Let Tournament Directors choose the tiebreaker order for each division's standings via a per-division ordered criteria list stored in the database.

## Architecture

A `ranking_criteria` JSON column on `divisions` holds an ordered array of criterion keys (e.g. `["wins","head_to_head","runs_against","runs_for"]`). The ranking engine iterates the list and applies independent comparator functions in sequence — first non-zero result wins. Directors configure the order via an ordered list with up/down buttons and checkboxes on the division edit form.

**Tech Stack:** Go, PostgreSQL, html/template, vanilla JS (no new dependencies)

---

## Data Model

### Schema

Add one nullable TEXT column to `divisions`:

```sql
ALTER TABLE divisions ADD COLUMN ranking_criteria TEXT;
```

NULL means use the system default order. The column stores a JSON array of criterion key strings.

### Division struct

```go
type Division struct {
    ID               int
    TournamentID     int
    Name             string
    RulesHTML        string
    RankingCriteria  []string  // parsed from ranking_criteria JSON; never nil after DB read
}
```

When `ranking_criteria` is NULL or empty in the DB, the DB layer returns the system default slice:

```go
var DefaultRankingCriteria = []string{"wins", "head_to_head", "runs_against", "runs_for"}
```

### DB interface changes

`ReturnDivisions` and `ReturnDivisionByID` parse the `ranking_criteria` JSON column into `[]string`, falling back to `DefaultRankingCriteria` on NULL or parse error.

`UpdateDivision` gains a `criteria []string` parameter and serializes it to JSON before writing. If the slice equals `DefaultRankingCriteria`, store NULL (keeps the column clean).

New method on the DB interface:

```go
// RunsAllowedInHeadToHead returns the total runs team B scored against team A
// across all games, and whether they played at all.
RunsAllowedInHeadToHead(teamAID, teamBID int) (int, bool)
```

This is the only new DB method required. All other criteria derive from fields already on `Team` (Wins, Losses, RunsFor, RunsAgainst).

### FakeDB

`FakeDB.UpdateDivision` stores the criteria slice on the in-memory division record. `FakeDB.RunsAllowedInHeadToHead` scans in-memory game records filtered by the two team IDs.

---

## Ranking Engine

### Signature change

```go
// Before
func (me *Env) SortTeams(teams []mydb.Team, sortalgo string) []mydb.Team

// After
func (me *Env) SortTeams(teams []mydb.Team, criteria []string) []mydb.Team
```

The two legacy named algorithm strings (`WinsHead2HeadRunsAgainstRunsEarned`, `WinsRunsAgainstRunsEarnedHead2Head`) are removed entirely. The one hardcoded call site in `PrintDivision` passes `division.RankingCriteria` instead.

### Comparator registry

Each criterion is a function with this signature:

```go
type criterionFn func(a, b mydb.Team, db mydb.DB) int // -1: a wins, 0: tie, +1: b wins
```

Registry in `sortteams.go`:

```go
var criteriaRegistry = map[string]criterionFn{
    "wins":                    criterionWins,
    "win_pct":                 criterionWinPct,
    "head_to_head":            criterionHeadToHead,
    "run_differential":        criterionRunDifferential,
    "runs_against":            criterionRunsAgainst,
    "runs_against_per_game":   criterionRunsAgainstPerGame,
    "runs_for":                criterionRunsFor,
    "head_to_head_runs_against": criterionHeadToHeadRunsAgainst,
    "coin_flip":               criterionCoinFlip,
}
```

### Sort loop

```go
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
```

### Criterion definitions

| Key | Logic | Notes |
|---|---|---|
| `wins` | `a.Wins` vs `b.Wins`, descending | Almost always first |
| `win_pct` | `a.Wins/(a.Wins+a.Losses)` vs same, descending | Handles unequal games played; 0-0 record = 0.0 |
| `head_to_head` | `db.DidTeamABeatTeamB(a.ID, b.ID)` | Existing DB method; returns (played bool, aWon bool) |
| `run_differential` | `(a.RunsFor-a.RunsAgainst)` vs same, descending | Uncapped total |
| `runs_against` | `a.RunsAgainst` vs `b.RunsAgainst`, ascending | Fewer is better |
| `runs_against_per_game` | `a.RunsAgainst/(a.Wins+a.Losses)` vs same, ascending | 0 games played = 0.0 |
| `runs_for` | `a.RunsFor` vs `b.RunsFor`, descending | More is better |
| `head_to_head_runs_against` | `db.RunsAllowedInHeadToHead(a.ID, b.ID)` | Fewer runs allowed vs this opponent is better |
| `coin_flip` | `a.ID` vs `b.ID`, ascending | Stable (deterministic), always breaks ties |

For `head_to_head`: if the two teams never played each other, the comparator returns 0 (no effect). Same for `head_to_head_runs_against`.

For float division in `win_pct` and `runs_against_per_game`: use `float64` arithmetic; a team with 0 games played is treated as 0.0 (sorts to the bottom under win_pct, ties under runs_against_per_game).

---

## Director UI

### Form location

The ranking criteria configurator is added to the existing division edit form at:
`/tournaments/:tid/manage/divisions/:did/edit`

It appears as a collapsible section below the rules editor, labeled "Standings ranking order".

### UI structure

An ordered `<ul>` where each `<li>` contains:
- A hidden `<input name="criteria" value="key">` (submitted only when checked)
- A checkbox labeled with the human-readable criterion name
- ↑ and ↓ buttons that swap the row with its neighbor via JS

On form submit, a JS handler collects all checked `criteria` inputs in DOM order and writes them into a single hidden `<input name="ranking_criteria" value="wins,head_to_head,...">` before the form posts. The POST handler splits on comma, validates each key against the known set, and saves.

Unknown keys are silently dropped. If the resulting list is empty, the system default is stored (NULL in DB).

### Human-readable criterion labels (shown in UI)

| Key | Label |
|---|---|
| `wins` | Most wins |
| `win_pct` | Best win percentage |
| `head_to_head` | Head-to-head record |
| `run_differential` | Best run differential |
| `runs_against` | Fewest runs allowed (total) |
| `runs_against_per_game` | Fewest runs allowed per game |
| `runs_for` | Most runs scored |
| `head_to_head_runs_against` | Fewest runs allowed head-to-head |
| `coin_flip` | Stable draw (by team ID) |

### Public display

The public division standings page (`/tournaments/:tid/divisions/:did`) shows a one-line summary beneath the table:

```
Ranked by: Most wins → Head-to-head record → Fewest runs allowed (total) → Most runs scored
```

This uses the division's active criteria list (filtered to only checked/included ones).

---

## Backward Compatibility

- The two legacy algorithm name strings are deleted from `SortTeams`. Tests that call `SortTeams("WinsRunsAgainstRunsEarnedHead2Head", ...)` are updated to pass the equivalent `[]string` slice.
- No existing division records are affected — NULL `ranking_criteria` falls back to the default, which matches the current `WinsHead2HeadRunsAgainstRunsEarned` behavior.

---

## Testing

All criterion functions are unit-tested independently in `webhandler/sortteams_test.go`. Each test constructs two `mydb.Team` values with known stats and asserts the correct comparator return value (-1, 0, or +1).

Integration tests in `webhandler/sortteams_test.go` verify the full `SortTeams` pipeline for:
- Basic ordering by wins
- Tiebreaker chains (wins tied → head-to-head → runs against)
- Three-way ties that resolve correctly
- `coin_flip` as final fallback produces a stable order

`mydb/fakedb_test.go` covers `RunsAllowedInHeadToHead` with games between three teams.

The DB serialization round-trip (criteria → JSON → []string) is tested in `mydb/divisions_test.go` or inline in `fakedb_test.go`.
