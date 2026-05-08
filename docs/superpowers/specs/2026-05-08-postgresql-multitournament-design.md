# PostgreSQL + Multi-Tournament Design

**Date:** 2026-05-08  
**Status:** Approved

## Problem

TourneyWeb uses MySQL with a single implicit tournament. All data (divisions, teams, games) is unscoped — there is no way to run multiple tournaments or view historical ones. The MySQL driver (`go-sql-driver/mysql`) and in-memory fallback (`localdb/`) are dead weight now that PostgreSQL is the target from the start.

## Goal

- Replace MySQL with PostgreSQL (`jackc/pgx/v5` in stdlib mode)
- Add a `tournaments` table; scope all entities to a tournament
- Rewrite all public and admin routes to be tournament-scoped
- Add a home page that lists tournaments grouped by recency
- Delete `localdb/` entirely — a real PostgreSQL instance is required

No database migrations are needed. PostgreSQL and multi-tournament are the starting point.

## Driver

`github.com/jackc/pgx/v5/stdlib` satisfies `database/sql` — all existing `*sql.DB` code stays compatible. Only the driver import and DDL change. Placeholders switch from MySQL `?` to PostgreSQL `$1, $2, ...`.

## Schema

All table and column names use lowercase snake_case (idiomatic PostgreSQL). Tables are created at startup via `CREATE TABLE IF NOT EXISTS`.

```sql
CREATE TABLE IF NOT EXISTS tournaments (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    sport      TEXT NOT NULL,
    location   TEXT NOT NULL,
    start_date DATE NOT NULL,
    notes      TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS divisions (
    id            SERIAL PRIMARY KEY,
    tournament_id INTEGER NOT NULL REFERENCES tournaments(id),
    name          TEXT NOT NULL,
    UNIQUE(tournament_id, name)
);

CREATE TABLE IF NOT EXISTS teams (
    id            SERIAL PRIMARY KEY,
    tournament_id INTEGER NOT NULL REFERENCES tournaments(id),
    division_id   INTEGER NOT NULL REFERENCES divisions(id),
    name          TEXT NOT NULL,
    coach         TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS games (
    id            SERIAL PRIMARY KEY,
    tournament_id INTEGER NOT NULL REFERENCES tournaments(id),
    division_id   INTEGER NOT NULL REFERENCES divisions(id),
    home_team_id  INTEGER NOT NULL REFERENCES teams(id),
    away_team_id  INTEGER NOT NULL REFERENCES teams(id),
    location      TEXT NOT NULL DEFAULT '',
    start_time    TEXT NOT NULL DEFAULT '',
    umpire        TEXT NOT NULL DEFAULT '',
    home_score    INTEGER,
    away_score    INTEGER
);

CREATE TABLE IF NOT EXISTS games_by_team (
    id             SERIAL PRIMARY KEY,
    tournament_id  INTEGER NOT NULL REFERENCES tournaments(id),
    division_id    INTEGER NOT NULL REFERENCES divisions(id),
    team_id        INTEGER NOT NULL REFERENCES teams(id),
    opponent_id    INTEGER NOT NULL REFERENCES teams(id),
    game_id        INTEGER NOT NULL REFERENCES games(id),
    team_score     INTEGER NOT NULL,
    opponent_score INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS event_news (
    id            SERIAL PRIMARY KEY,
    tournament_id INTEGER NOT NULL REFERENCES tournaments(id),
    title         TEXT NOT NULL,
    body          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS locations (
    id      SERIAL PRIMARY KEY,
    name    TEXT NOT NULL,
    address TEXT NOT NULL
);
```

`home_score` and `away_score` in `games` are nullable (`INTEGER` with no DEFAULT) — NULL means unscored.

## Config

Connection string format changes from `mysql://user:pass@tcp(host:3306)/db` to `postgres://user:pass@host:5432/db`. `mydb.New()` accepts only a `postgres://` URL; if empty or missing, it fatals with a clear message. The `mysql://` prefix-stripping logic in `config.go` is removed.

## File Structure

### New files
- `mydb/tournaments.go` — `Tournament` struct, CRUD functions
- `webhandler/tournaments.go` — public and admin tournament handlers
- `webhandler/templates/index.html` — rewritten for tournament listing
- `webhandler/templates/tournament.html` — `{{define "tournament"}}` tournament home (divisions overview)
- `webhandler/templates/admin/tournaments.html` — `{{define "adminTournaments"}}` admin tournament list
- `webhandler/templates/admin/tournament_form.html` — `{{define "tournamentForm"}}` create tournament form
- `webhandler/templates/admin/tournament_view.html` — `{{define "adminTournamentView"}}` tournament admin home

### Modified files
- `go.mod` / `go.sum` — replace `go-sql-driver/mysql` with `jackc/pgx/v5`
- `mydb/mydb.go` — driver swap, schema DDL rewrite, remove localdb import
- `mydb/divisions.go` — add `tournamentID` param to scoped queries, `$N` placeholders
- `mydb/teams.go` — same
- `mydb/games.go` — same
- `webhandler/templates.go` — updated data structs, new data types for tournaments
- `webhandler/webhandler.go` — handlers extract `:tid`, pass to DB; update PrintIndex
- `webhandler/divisions.go` — extract `:tid`, pass to DB calls
- `webhandler/teams.go` — same
- `webhandler/templates/layout.html` — nav links updated (tournament-scoped); shows tournament name in context
- All existing page templates — links updated to include `/:tid/` prefix where needed
- All existing admin templates — links updated for new route structure
- `main.go` — route registration rewritten
- `config.go` — remove `mysql://` prefix logic

### Deleted
- `localdb/` — entire package removed

## Routing

### Public
```
GET  /                                          → tournament listing (home page)
GET  /tournaments/:tid                          → tournament home: divisions overview
GET  /tournaments/:tid/divisions/:did           → division standings + games
GET  /tournaments/:tid/teams/:teamid            → team detail
GET  /tournaments/:tid/games                    → all games in tournament
GET  /login
POST /login
GET  /hrderbyinfo                               → unchanged
```

### Admin
```
GET  /admin/tournaments                         → admin tournament list
GET  /admin/tournaments/new                     → create tournament form
POST /admin/tournaments                         → submit new tournament
GET  /admin/tournaments/:tid                    → tournament admin home
GET  /admin/tournaments/:tid/divisions          → manage divisions
POST /admin/tournaments/:tid/divisions          → add division
POST /admin/tournaments/:tid/divisions/delete   → delete division (posts divisionid)
GET  /admin/tournaments/:tid/divisions/:did     → division admin view
GET  /admin/tournaments/:tid/teams              → manage teams
POST /admin/tournaments/:tid/teams              → add team
POST /admin/tournaments/:tid/teams/delete       → delete team (posts teamid)
GET  /admin/tournaments/:tid/games              → list all games
GET  /admin/tournaments/:tid/games/new/:did     → create game form for a division
POST /admin/tournaments/:tid/games              → submit new game
GET  /admin/tournaments/:tid/games/:gid/score   → score game form
POST /admin/tournaments/:tid/games/:gid/score   → submit score
GET  /admin/tournaments/:tid/games/:gid/delete  → delete game
```

The `RequestLogger` middleware auth check is unchanged — any path under `/admin` requires a valid session.

## Home Page Tournament Listing (`/`)

Sections (based on `start_date` relative to today):

1. **Quick Jump** — always shown: `Tournament ID: [____] [Go]` form; uses a small `onsubmit` JS snippet to navigate to `/tournaments/{entered-id}` (avoids a redirect handler that would conflict with the `:tid` param route in httprouter)
2. **Coming Up** — start_date in [today, today+7], sorted soonest first
3. **Recently Happened** — start_date in [today-7, yesterday], sorted most recent first
4. **Future** — start_date > today+7, sorted soonest first — Bootstrap collapse, paginated 20/page via `?future_page=N`
5. **Past** — start_date < today-7, sorted most recent first — Bootstrap collapse, paginated 20/page via `?past_page=N`

If both Coming Up and Recently Happened are empty, show "No active tournaments."

Collapse toggle buttons show counts: "Future (14)" / "Past (32)".

Tournament cards show: name, sport, location, start_date.

### Data struct
```go
type indexData struct {
    baseData
    ComingUp    []mydb.Tournament
    Recent      []mydb.Tournament
    Future      []mydb.Tournament
    Past        []mydb.Tournament
    FuturePage  int
    PastPage    int
    FutureTotal int
    PastTotal   int
}
```

Pagination uses SQL `LIMIT 20 OFFSET (page-1)*20`. FutureTotal and PastTotal are used to render "showing X of Y" and next/prev links.

## Data Structs (in `webhandler/templates.go`)

`baseData` gains a `Tournament` field for use in tournament-scoped pages:

```go
type baseData struct {
    IsAdmin    bool
    CSRFField  template.HTML
    Tournament mydb.Tournament // zero value when not in tournament context
}
```

New data structs:
```go
type tournamentData      struct { baseData; Divisions []mydb.Division; Teams map[int][]mydb.Team }
type adminTournamentsData struct { baseData; Tournaments []mydb.Tournament }
type tournamentFormData  struct { baseData }
type adminTournamentViewData struct { baseData; DisableDelete bool }
```

Existing data structs (`divisionData`, `adminDivisionsData`, etc.) keep their entity fields unchanged. The only structural change is that `baseData` now carries `Tournament` — tournament-scoped pages populate it; non-scoped pages (login, home) leave it as the zero value.

## Handler Pattern

Every tournament-scoped handler opens with:

```go
tid, err := strconv.Atoi(ps.ByName("tid"))
if err != nil {
    http.Error(w, "Bad tournament ID", http.StatusBadRequest)
    return
}
tournament := me.DB.ReturnTournamentByID(tid)
if tournament.ID == 0 {
    http.Error(w, "Tournament not found", http.StatusNotFound)
    return
}
```

`newBase` gains an optional tournament parameter:

```go
func newBaseWithTournament(r *http.Request, isAdmin bool, t mydb.Tournament) baseData {
    return baseData{IsAdmin: isAdmin, CSRFField: csrf.TemplateField(r), Tournament: t}
}
```

## mydb API Changes

### New: `mydb/tournaments.go`
```go
type Tournament struct {
    ID        int
    Name      string
    Sport     string
    Location  string
    StartDate time.Time
    Notes     string
}

func (me *MyDB) AddTournament(name, sport, location, notes string, date time.Time) int // returns new tournament ID for redirect
func (me *MyDB) ReturnTournaments() []Tournament
func (me *MyDB) ReturnTournamentByID(id int) Tournament
func (me *MyDB) ReturnTournamentsComingUp() []Tournament   // start_date in [today, today+7]
func (me *MyDB) ReturnTournamentsRecent() []Tournament     // start_date in [today-7, yesterday]
func (me *MyDB) ReturnTournamentsFuture(page int) ([]Tournament, int)  // start_date > today+7, returns (slice, total)
func (me *MyDB) ReturnTournamentsPast(page int) ([]Tournament, int)    // start_date < today-7, returns (slice, total)
func (me *MyDB) DelTournament(id int)
```

### Modified functions (gain `tournamentID int` parameter)
- `AddDivision(tournamentID int, name string)`
- `DelDivision(id int)` — unchanged (ID is globally unique)
- `ReturnDivisions(tournamentID int) []Division`
- `ReturnDivisionByID(id int) Division` — unchanged
- `AddTeam(tournamentID, divisionID int, name, coach string)`
- `DelTeam(id int)` — unchanged
- `ReturnTeamsByDivisionID(divisionID int) []Team` — unchanged (division scopes it)
- `ReturnTeamsByDivisionIDWithStats(divisionID int) []Team` — unchanged
- `ReturnTeamByID(id int) Team` — unchanged
- `AddGame(tournamentID, divisionID, homeTeamID, awayTeamID int, location, startTime, umpire string)`
- `DelGame(id int)` — unchanged
- `AllGamesByDivision(divisionID int) []Game` — unchanged
- `AllGamesByTeam(teamID int) []Game` — unchanged
- `AllGames(tournamentID int) []Game` — gains tournamentID
- `ReturnGameByID(id int) Game` — unchanged
- `ScoreGame(gid, homeScore, awayScore int)` — unchanged
- `AddTeamScore(tournamentID, divisionID, teamID, opponentID, gameID, teamScore, opponentScore int)`
- `DeleteTeamScore(gameID int)` — unchanged
- Stats functions (`TeamWins`, `TeamLosses`, `TeamScoredFor`, `TeamScoredAgainst`, `GamesPlayedByTeam`, `DidTeamABeatTeamB`) — unchanged (team IDs are globally unique)

## Out of Scope

- Authentication overhaul (single admin password unchanged)
- EVENTNEWS and LOCATION UI (tables defined, no routes)
- Edit support for tournaments, divisions, teams, games
- HR Derby page content (unchanged)
- Any UI redesign
