# Single Elimination Bracket Design

## Goal

Allow Tournament Directors to run single-elimination playoff brackets per division. Directors configure seeding from standings, lock the bracket, and scores entered through the existing score UI automatically advance winners into the next round.

## Architecture

A `brackets` table tracks one bracket per division. `bracket_seeds` holds the seeded team order (mutable during the seeding phase). `bracket_games` holds every slot in the bracket tree (all rounds, all positions) with parent-child relationships derived by arithmetic — no FK chain needed. When a game is scored, the web handler checks whether it belongs to a bracket game and cascades the winner forward.

## Tech Stack

Go, PostgreSQL, html/template, vanilla JS (no new dependencies)

---

## Data Model

### `divisions.phase` column

```sql
ALTER TABLE divisions ADD COLUMN IF NOT EXISTS phase TEXT NOT NULL DEFAULT 'pool';
```

Values: `'pool'` | `'bracket'`

### `brackets` table

```sql
CREATE TABLE IF NOT EXISTS brackets (
    id            SERIAL PRIMARY KEY,
    division_id   INT NOT NULL REFERENCES divisions(id) ON DELETE CASCADE,
    format        TEXT NOT NULL DEFAULT 'single_elimination',
    status        TEXT NOT NULL DEFAULT 'seeding',
    size          INT  NOT NULL
);
```

`status` values: `'seeding'` | `'active'` | `'complete'`  
`size` is the next power of 2 at or above the team count (4, 8, 16, or 32).

### `bracket_seeds` table

```sql
CREATE TABLE IF NOT EXISTS bracket_seeds (
    id          SERIAL PRIMARY KEY,
    bracket_id  INT NOT NULL REFERENCES brackets(id) ON DELETE CASCADE,
    seed        INT NOT NULL,
    team_id     INT          -- NULL means bye slot
);
```

Rows are reorderable (by updating `seed` values) while `brackets.status = 'seeding'`. Once locked, these rows are read-only.

### `bracket_games` table

```sql
CREATE TABLE IF NOT EXISTS bracket_games (
    id               SERIAL PRIMARY KEY,
    bracket_id       INT  NOT NULL REFERENCES brackets(id) ON DELETE CASCADE,
    round            INT  NOT NULL,
    position         INT  NOT NULL,
    top_team_id      INT,           -- NULL = TBD
    bottom_team_id   INT,           -- NULL = TBD
    top_is_bye       BOOL NOT NULL DEFAULT false,
    bottom_is_bye    BOOL NOT NULL DEFAULT false,
    game_id          INT,           -- FK to games table; NULL until game is created
    winner_team_id   INT            -- NULL until game is complete
);
```

**Round/position arithmetic:**
- `round` is 1-based. Round 1 = first round. `total_rounds = log2(size)`. Round `total_rounds` = final.
- `position` is 1-based within the round. Round 1 has `size/2` positions.
- Parent of `(round R, position P)` = `(round R+1, position ⌈P/2⌉)`.
- A game at position P feeds the parent's **top** slot if P is odd, **bottom** slot if P is even.

### Go structs

```go
// mydb/brackets.go

type Bracket struct {
    ID         int
    DivisionID int
    Format     string
    Status     string
    Size       int
}

type BracketSeed struct {
    ID        int
    BracketID int
    Seed      int
    TeamID    int  // 0 = bye
    TeamName  string
}

type BracketGame struct {
    ID             int
    BracketID      int
    Round          int
    Position       int
    TopTeamID      int
    BottomTeamID   int
    TopIsBye       bool
    BottomIsBye    bool
    GameID         int
    WinnerTeamID   int
    // Populated by join for display:
    TopTeamName    string
    BottomTeamName string
    WinnerTeamName string
    Game           Game  // populated when GameID != 0
}
```

---

## Division Phase Transition

### Starting the bracket

Director clicks **"Start Bracket"** on the manage-divisions page (button only shown when `phase = 'pool'`).

Handler `ManageBracketStart` (POST `/tournaments/:tid/manage/divisions/:did/bracket/start`):

1. Fetch teams via `ReturnTeamsByDivisionIDWithStats` + `SortTeams(div.RankingCriteria)` — same sort as the public standings page.
2. Compute `size` = next power of 2 ≥ team count. Max supported: 32.
3. Insert one `brackets` row (`status='seeding'`).
4. Insert `bracket_seeds` rows: seeds 1–N for real teams (in standings order), seeds N+1–size as bye slots (team_id = NULL).
5. Set `divisions.phase = 'bracket'`.
6. Redirect to the seed review page.

### Seed review page

`GET /tournaments/:tid/manage/divisions/:did/bracket/seed`

Shows an ordered list of all seeds. Real teams have ↑/↓ buttons; bye slots are fixed at the bottom and labeled "Bye". A **"Lock Bracket"** button submits the final order.

`POST /tournaments/:tid/manage/divisions/:did/bracket/seed`

Receives a comma-joined list of `team_id` values in seed order (same hidden-input pattern as ranking criteria). Updates `bracket_seeds.seed` values. Bye slots are reinserted at the end. Redirects back to the seed page.

### Locking the bracket

`POST /tournaments/:tid/manage/divisions/:did/bracket/lock`

Handler `ManageBracketLock`:

1. Set `brackets.status = 'active'`.
2. Generate all `bracket_games` rows for all rounds (see Bracket Generation below).
3. Auto-advance bye matchups: if either side of a round-1 game is a bye, set `winner_team_id` to the real team and populate the parent round-2 game's top or bottom slot.
4. For every round-1 game where both sides are real teams, call `AddGame` to create a placeholder `games` record (empty location, zero time, empty umpire) and store the returned `game_id` in `bracket_games.game_id`.
5. Redirect to the director's division manage page.

---

## Bracket Generation Algorithm

### Seeding pattern

For `size = N` (power of 2), the round-1 seed pairings follow the standard balanced bracket:

```
positions(1) = [1]
positions(N) = interleave(positions(N/2), complement(positions(N/2), N))
  where complement(x, N) = N + 1 - x
  and interleave([a,b,...], [x,y,...]) = [a, x, b, y, ...]
```

Examples:
- N=4: positions = [1,4,2,3] → matchups (1v4), (2v3)
- N=8: positions = [1,8,4,5,2,7,3,6] → matchups (1v8), (4v5), (2v7), (3v6)

The `positions` list is split into pairs top-to-bottom: `(positions[0], positions[1])`, `(positions[2], positions[3])`, etc. Each pair is one round-1 `bracket_games` row.

### Subsequent rounds

Create `bracket_games` rows for rounds 2 through `log2(N)` with `top_team_id = 0` and `bottom_team_id = 0` (TBD). These slots are filled in as prior-round games complete.

---

## Auto-Advance on Scoring

`RecordScore` in `webhandler/webhandler.go` is the single handler used by both the staff score route (`POST /tournaments/:tid/score/games/:gid`) and the admin score route (`POST /admin/tournaments/:tid/games/:gid/score`). After calling `me.DB.ScoreGame(gid, hscore, ascore)`, it calls `me.AdvanceBracket(gid, winnerTeamID)`.

The caller computes `winnerTeamID` from the scores and game record:

```go
// In RecordScore, after me.DB.ScoreGame(gid, hscore, ascore):
if hscore != ascore {
    game := me.DB.ReturnGameByID(gid)
    winnerID := game.HomeTeam.ID
    if ascore > hscore {
        winnerID = game.AwayTeam.ID
    }
    me.AdvanceBracket(gid, winnerID)
}
```

```go
// webhandler/bracket.go

func (me *Env) AdvanceBracket(gameID, winnerTeamID int) {
    bg := me.DB.GetBracketGameByGameID(gameID)
    if bg.ID == 0 {
        return // not a bracket game
    }

    me.DB.SetBracketGameWinner(bg.ID, winnerTeamID)

    // find parent game
    parentRound := bg.Round + 1
    parentPos := (bg.Position + 1) / 2
    parent := me.DB.GetBracketGameByRoundPosition(bg.BracketID, parentRound, parentPos)
    if parent.ID == 0 {
        // this was the final — mark bracket complete
        me.DB.SetBracketStatus(bg.BracketID, "complete")
        return
    }

    // update parent slot (odd position → top, even → bottom)
    if bg.Position%2 == 1 {
        me.DB.SetBracketGameTopTeam(parent.ID, winnerTeamID)
    } else {
        me.DB.SetBracketGameBottomTeam(parent.ID, winnerTeamID)
    }

    // if parent now has both teams, create a placeholder games record
    parent = me.DB.GetBracketGameByID(parent.ID)
    if parent.TopTeamID != 0 && parent.BottomTeamID != 0 && parent.GameID == 0 {
        bracket := me.DB.GetBracketByID(bg.BracketID)
        div := me.DB.ReturnDivisionByID(bracket.DivisionID)
        gid := me.DB.AddGame(div.TournamentID, div.ID, parent.TopTeamID, parent.BottomTeamID, "", time.Time{}, "")
        me.DB.SetBracketGameGameID(parent.ID, gid)
    }
}
```

**Re-scoring:** If a bracket game is re-scored, `AdvanceBracket` re-runs with the new winner. The downstream slot is updated. If the downstream game has already been scored, `AdvanceBracket` logs an error and returns without modifying it — the director must manually correct downstream results.

---

## Public Bracket Display

Route: `GET /tournaments/:tid/divisions/:did/bracket`

The template renders a left-to-right flex layout with one column per round. Each column contains `size / (2^round)` matchup boxes. Connecting lines between rounds are rendered with CSS right-borders and pseudo-element horizontal connectors (no SVG, no JS).

**Team slot states:**
- **Normal**: white background, team name + seed number
- **Winner**: blue border + blue tint, bold team name, score shown
- **TBD**: dashed border, gray italic "TBD"
- **Bye**: dashed border, gray — shown only in round 1

The division's public page (`/tournaments/:tid/divisions/:did`) gains a **"View Bracket"** link when `division.Phase == 'bracket'`.

A "Seeded by: …" line at the bottom reuses `CriteriaRankingLabel(div.RankingCriteria)`.

### Template data struct

```go
type bracketData struct {
    baseData
    Division   mydb.Division
    Bracket    mydb.Bracket
    Rounds     []bracketRound
}

type bracketRound struct {
    Label  string         // "Quarterfinals", "Semifinals", "Final"
    Games  []mydb.BracketGame
}
```

Round labels are derived from total rounds and current round number:
- Last round → "Final"
- Last-1 → "Semifinals"
- Last-2 → "Quarterfinals"
- Earlier rounds → "Round N"

---

## DB Interface Methods

New methods on `mydb.DB`:

```go
// Brackets
CreateBracket(divisionID int, format string, size int) int
GetBracketByID(id int) Bracket
GetBracketByDivisionID(divisionID int) Bracket
SetBracketStatus(bracketID int, status string)

// Seeds
AddBracketSeed(bracketID, seed, teamID int)
GetBracketSeeds(bracketID int) []BracketSeed
UpdateBracketSeeds(bracketID int, teamIDs []int) // replaces seed order

// Bracket games
AddBracketGame(bracketID, round, position int) int
SetBracketGameTeams(id, topTeamID, bottomTeamID int, topIsBye, bottomIsBye bool)
SetBracketGameTopTeam(id, teamID int)
SetBracketGameBottomTeam(id, teamID int)
SetBracketGameWinner(id, winnerTeamID int)
SetBracketGameGameID(id, gameID int)
GetBracketGameByID(id int) BracketGame
GetBracketGameByGameID(gameID int) BracketGame
GetBracketGameByRoundPosition(bracketID, round, position int) BracketGame
GetBracketGames(bracketID int) []BracketGame
```

---

## Routes Summary

| Method | Path | Handler | Auth |
|--------|------|---------|------|
| GET | `/tournaments/:tid/divisions/:did/bracket` | `PrintBracket` | public |
| POST | `/tournaments/:tid/manage/divisions/:did/bracket/start` | `ManageBracketStart` | director |
| GET | `/tournaments/:tid/manage/divisions/:did/bracket/seed` | `ManageBracketSeed` | director |
| POST | `/tournaments/:tid/manage/divisions/:did/bracket/seed` | `ManageBracketSeed` | director |
| POST | `/tournaments/:tid/manage/divisions/:did/bracket/lock` | `ManageBracketLock` | director |

---

## Files

| File | Change |
|------|--------|
| `mydb/brackets.go` | New: `Bracket`, `BracketSeed`, `BracketGame` structs + all DB methods on `*MyDB` |
| `mydb/db.go` | Add all new bracket methods to interface |
| `mydb/fakedb.go` | Implement all bracket methods on `*FakeDB` |
| `mydb/mydb.go` | Add 3 `CREATE TABLE` migrations + `ALTER TABLE divisions ADD COLUMN IF NOT EXISTS phase` |
| `mydb/divisions.go` | Add `Phase string` to `Division` struct; include in SELECT/scan |
| `webhandler/bracket.go` | New: all 5 handlers + `AdvanceBracket` + bracket generation algorithm + `bracketData`/`bracketRound` types |
| `webhandler/webhandler.go` | In `RecordScore`: compute winner from scores, call `me.AdvanceBracket(gid, winnerTeamID)` after `ScoreGame` |
| `webhandler/templates/bracket.html` | New: public bracket tree template |
| `webhandler/templates/manage/bracket_seed.html` | New: seed review/reorder template |
| `webhandler/templates/manage/divisions.html` | Add "Start Bracket" / "Edit Seeds" button per division |
| `webhandler/templates/divisions.html` | Add "View Bracket" link when phase = bracket |
| `main.go` | Register 5 new routes |
