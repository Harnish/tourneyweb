# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## About

TourneyWeb is an open-source Go web application for managing sports tournaments (initially baseball). It tracks divisions, teams, games, and standings/rankings.

## Commands

```bash
# Build
go build

# Run (requires tourneyweb.conf or config.yaml in working directory)
./tourneyweb

# Run tests (none exist yet, but if added)
go test ./...
```

## Configuration

Config is loaded from `tourneyweb.conf`, then `config.yaml`, then `/etc/go-periodical-rack/config.yaml`. YAML format:

```yaml
port: 8989
debug: true
database: mysql://user:pass@tcp(host:3306)/tourneyweb
adminpassword: somepassword
disabledelete: false
bannerimagepath: dawgpoundlogo.jpg
```

**Local dev without MySQL**: If no `database` is set in config, `mydb.New()` falls back to an in-memory MySQL server started by `localdb.StartDB()`. The `localdb` package is a thin wrapper around `go-mysql-server`.

Environment config (`GetEnvironmentConfig()` in `config.go`) reads `TANPORT`, `TANDEBUG`, `TANDBTYPE`/`TANDB`/`TANDB*`, and `TANADMINPASS` — but this function is defined and unused; the app currently only reads from the YAML file.

## Architecture

### Layers

- **`main.go`** — wires everything: loads config, creates `MyDB`, creates `Env` (webhandler), registers all routes with `httprouter`, starts HTTP server
- **`mydb/`** — database layer, direct MySQL queries via `database/sql`
  - `mydb.go` — connection, table auto-creation on startup (`mysqltables`), score helpers
  - `divisions.go`, `teams.go`, `games.go` — CRUD per entity; `Team` struct carries denormalized stats (Wins, Losses, RunsFor, RunsAgainst)
- **`webhandler/`** — HTTP handlers; all HTML is built via string concatenation (no templates)
  - `webhandler.go` — index, login/session, game scoring UI, request logger middleware, CSS/static serving
  - `divisions.go` — division admin forms
  - `teams.go` — team admin forms and team detail page
  - `sortteams.go` — two ranking algorithms (`WinsHead2HeadRunsAgainstRunsEarned`, `WinsRunsAgainstRunsEarnedHead2Head`)

### Database schema

Six tables created automatically on startup:
- `DIVISIONS` — tournament divisions
- `TEAMS` — teams with division FK
- `GAMES` — scheduled games with home/away team IDs, location, starttime, umpire, and raw scores
- `GAMESBYTEAM` — denormalized per-team game results (written by `ScoreGame`); this is what drives wins/losses/runs stats
- `EVENTNEWS` — news items (defined but no UI yet)
- `LOCATION` — field locations (defined but no UI yet)

When a game is scored (`ScoreGame`), it writes to both `GAMES` and `GAMESBYTEAM` (two rows: one per team). Rankings read from `GAMESBYTEAM`.

### Auth

Admin access uses a single shared password from config (`adminpassword`). On login, the username + session cookie is stored via `github.com/rivo/sessions`. The `RequestLogger` middleware checks session for any `/admin/*` path; non-authenticated requests get a "not authorized" error response.

`DisableDelete` config flag locks out all delete operations — intended for use during a live tournament.

### Frontend

Bootstrap 4.5 loaded from CDN. A banner image is served from the path set in `bannerimagepath`. A minimal custom CSS file is served inline from `main.go`.
