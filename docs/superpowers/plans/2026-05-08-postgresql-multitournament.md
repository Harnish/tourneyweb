# PostgreSQL + Multi-Tournament Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace MySQL with PostgreSQL, add a `tournaments` table, scope all entities to a tournament, and add a tournament-listing home page.

**Architecture:** In-place rewrite. The `mydb` layer swaps to `jackc/pgx/v5` stdlib driver, gains `tournaments.go`, and all entity functions gain `tournamentID` params. The `webhandler` layer adds tournament handlers/templates and updates all existing handlers to extract `:tid` from routes. No database migrations — PostgreSQL and multi-tournament are the starting point.

**Tech Stack:** `github.com/jackc/pgx/v5/stdlib`, PostgreSQL `$N` placeholders, `html/template` (already in use), Bootstrap 4.5 (already in use), `httprouter` (already in use).

---

### Task 1: Driver swap + schema DDL + config

**Files:**
- Modify: `go.mod` / `go.sum`
- Rewrite: `mydb/mydb.go`
- Modify: `config.go`
- Delete: `localdb/` (entire package)

- [ ] **Step 1: Add pgx/v5 dependency**

```bash
go get github.com/jackc/pgx/v5
```

Expected: module added to go.mod and go.sum.

- [ ] **Step 2: Rewrite `mydb/mydb.go`**

```go
package mydb

import (
	"database/sql"
	"log"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var pgtables = []string{
	`CREATE TABLE IF NOT EXISTS tournaments (
		id         SERIAL PRIMARY KEY,
		name       TEXT NOT NULL,
		sport      TEXT NOT NULL,
		location   TEXT NOT NULL,
		start_date DATE NOT NULL,
		notes      TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS divisions (
		id            SERIAL PRIMARY KEY,
		tournament_id INTEGER NOT NULL REFERENCES tournaments(id),
		name          TEXT NOT NULL,
		UNIQUE(tournament_id, name)
	)`,
	`CREATE TABLE IF NOT EXISTS teams (
		id            SERIAL PRIMARY KEY,
		tournament_id INTEGER NOT NULL REFERENCES tournaments(id),
		division_id   INTEGER NOT NULL REFERENCES divisions(id),
		name          TEXT NOT NULL,
		coach         TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS games (
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
	)`,
	`CREATE TABLE IF NOT EXISTS games_by_team (
		id             SERIAL PRIMARY KEY,
		tournament_id  INTEGER NOT NULL REFERENCES tournaments(id),
		division_id    INTEGER NOT NULL REFERENCES divisions(id),
		team_id        INTEGER NOT NULL REFERENCES teams(id),
		opponent_id    INTEGER NOT NULL REFERENCES teams(id),
		game_id        INTEGER NOT NULL REFERENCES games(id),
		team_score     INTEGER NOT NULL,
		opponent_score INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS event_news (
		id            SERIAL PRIMARY KEY,
		tournament_id INTEGER NOT NULL REFERENCES tournaments(id),
		title         TEXT NOT NULL,
		body          TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS locations (
		id      SERIAL PRIMARY KEY,
		name    TEXT NOT NULL,
		address TEXT NOT NULL
	)`,
}

type MyDB struct {
	DB    *sql.DB
	debug bool
}

func New(path string, debug bool) *MyDB {
	if !strings.HasPrefix(path, "postgres://") {
		log.Fatalf("database: must be a postgres:// URL, got %q", path)
	}
	db, err := sql.Open("pgx", path)
	if err != nil {
		log.Fatalf("database: open: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("database: ping: %v", err)
	}
	for _, ddl := range pgtables {
		if _, err := db.Exec(ddl); err != nil {
			log.Fatalf("database: create table: %v\n%s", err, ddl)
		}
	}
	return &MyDB{DB: db, debug: debug}
}

func (me *MyDB) AddTeamScore(tournamentID, divisionID, teamID, opponentID, gameID, teamScore, opponentScore int) {
	me.DB.Exec(
		`INSERT INTO games_by_team (tournament_id, division_id, team_id, opponent_id, game_id, team_score, opponent_score) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		tournamentID, divisionID, teamID, opponentID, gameID, teamScore, opponentScore,
	)
}
```

- [ ] **Step 3: Simplify `config.go` `GetEnvironmentConfig()`**

Replace the body of `GetEnvironmentConfig()` (lines 60–82) with:

```go
func GetEnvironmentConfig() (config Config) {
	config.Port = os.Getenv("TANPORT")
	tandebug := os.Getenv("TANDEBUG")
	config.Debug = strings.ToLower(tandebug) == "true"
	config.Database = os.Getenv("TANDB")
	config.AdminPassword = os.Getenv("TANADMINPASS")
	return
}
```

Remove unused imports from config.go if any appear after this change.

- [ ] **Step 4: Delete `localdb/` package**

```bash
rm -rf localdb/
```

- [ ] **Step 5: Remove old dependencies and tidy**

```bash
go mod tidy
```

Expected: `go-sql-driver/mysql`, `src-d/go-mysql-server`, and all their transitive deps removed from go.mod/go.sum.

- [ ] **Step 6: Verify mydb package compiles**

```bash
go build ./mydb/...
```

Expected: PASS. (`go build ./...` will fail — webhandler still uses old API, expected at this stage.)

- [ ] **Step 7: Commit**

```bash
git add mydb/mydb.go config.go go.mod go.sum
git rm -r localdb/
git commit -m "feat: swap MySQL for PostgreSQL (pgx/v5), rewrite schema DDL, delete localdb"
```

---

### Task 2: Tournament DB layer

**Files:**
- Create: `mydb/tournaments.go`

- [ ] **Step 1: Create `mydb/tournaments.go`**

```go
package mydb

import (
	"database/sql"
	"log"
	"time"
)

type Tournament struct {
	ID        int
	Name      string
	Sport     string
	Location  string
	StartDate time.Time
	Notes     string
}

func scanTournaments(rows *sql.Rows) []Tournament {
	var out []Tournament
	for rows.Next() {
		var t Tournament
		rows.Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes)
		out = append(out, t)
	}
	rows.Close()
	return out
}

func (me *MyDB) AddTournament(name, sport, location, notes string, date time.Time) int {
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO tournaments (name, sport, location, start_date, notes) VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		name, sport, location, date, notes,
	).Scan(&id)
	if err != nil {
		log.Println("AddTournament:", err)
	}
	return id
}

func (me *MyDB) ReturnTournaments() []Tournament {
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes FROM tournaments ORDER BY start_date DESC`,
	)
	if err != nil {
		log.Println("ReturnTournaments:", err)
		return nil
	}
	return scanTournaments(rows)
}

func (me *MyDB) ReturnTournamentByID(id int) Tournament {
	var t Tournament
	err := me.DB.QueryRow(
		`SELECT id, name, sport, location, start_date, notes FROM tournaments WHERE id=$1`, id,
	).Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes)
	if err != nil && err != sql.ErrNoRows {
		log.Println("ReturnTournamentByID:", err)
	}
	return t
}

func (me *MyDB) ReturnTournamentsComingUp() []Tournament {
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes FROM tournaments WHERE start_date >= CURRENT_DATE AND start_date <= CURRENT_DATE + INTERVAL '7 days' ORDER BY start_date ASC`,
	)
	if err != nil {
		log.Println("ReturnTournamentsComingUp:", err)
		return nil
	}
	return scanTournaments(rows)
}

func (me *MyDB) ReturnTournamentsRecent() []Tournament {
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes FROM tournaments WHERE start_date >= CURRENT_DATE - INTERVAL '7 days' AND start_date < CURRENT_DATE ORDER BY start_date DESC`,
	)
	if err != nil {
		log.Println("ReturnTournamentsRecent:", err)
		return nil
	}
	return scanTournaments(rows)
}

func (me *MyDB) ReturnTournamentsFuture(page int) ([]Tournament, int) {
	if page < 1 {
		page = 1
	}
	var total int
	me.DB.QueryRow(`SELECT COUNT(*) FROM tournaments WHERE start_date > CURRENT_DATE + INTERVAL '7 days'`).Scan(&total)
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes FROM tournaments WHERE start_date > CURRENT_DATE + INTERVAL '7 days' ORDER BY start_date ASC LIMIT 20 OFFSET $1`,
		(page-1)*20,
	)
	if err != nil {
		log.Println("ReturnTournamentsFuture:", err)
		return nil, total
	}
	return scanTournaments(rows), total
}

func (me *MyDB) ReturnTournamentsPast(page int) ([]Tournament, int) {
	if page < 1 {
		page = 1
	}
	var total int
	me.DB.QueryRow(`SELECT COUNT(*) FROM tournaments WHERE start_date < CURRENT_DATE - INTERVAL '7 days'`).Scan(&total)
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes FROM tournaments WHERE start_date < CURRENT_DATE - INTERVAL '7 days' ORDER BY start_date DESC LIMIT 20 OFFSET $1`,
		(page-1)*20,
	)
	if err != nil {
		log.Println("ReturnTournamentsPast:", err)
		return nil, total
	}
	return scanTournaments(rows), total
}

func (me *MyDB) DelTournament(id int) {
	me.DB.Exec(`DELETE FROM tournaments WHERE id=$1`, id)
}
```

- [ ] **Step 2: Verify**

```bash
go build ./mydb/...
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add mydb/tournaments.go
git commit -m "feat: add Tournament struct and CRUD functions"
```

---

### Task 3: Division DB layer

**Files:**
- Rewrite: `mydb/divisions.go`

- [ ] **Step 1: Rewrite `mydb/divisions.go`**

```go
package mydb

import (
	"database/sql"
	"log"
)

type Division struct {
	ID           int
	TournamentID int
	Name         string
}

func (me *MyDB) AddDivision(tournamentID int, name string) {
	_, err := me.DB.Exec(
		`INSERT INTO divisions (tournament_id, name) VALUES ($1,$2)`,
		tournamentID, name,
	)
	if err != nil {
		log.Println("AddDivision:", err)
	}
}

func (me *MyDB) DelDivision(id int) {
	me.DB.Exec(`DELETE FROM divisions WHERE id=$1`, id)
}

func (me *MyDB) ReturnDivisions(tournamentID int) []Division {
	rows, err := me.DB.Query(
		`SELECT id, tournament_id, name FROM divisions WHERE tournament_id=$1 ORDER BY name`,
		tournamentID,
	)
	if err != nil {
		log.Println("ReturnDivisions:", err)
		return nil
	}
	var out []Division
	for rows.Next() {
		var d Division
		rows.Scan(&d.ID, &d.TournamentID, &d.Name)
		out = append(out, d)
	}
	rows.Close()
	return out
}

func (me *MyDB) ReturnDivisionByID(id int) Division {
	var d Division
	err := me.DB.QueryRow(
		`SELECT id, tournament_id, name FROM divisions WHERE id=$1`, id,
	).Scan(&d.ID, &d.TournamentID, &d.Name)
	if err != nil && err != sql.ErrNoRows {
		log.Println("ReturnDivisionByID:", err)
	}
	return d
}
```

- [ ] **Step 2: Verify**

```bash
go build ./mydb/...
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add mydb/divisions.go
git commit -m "feat: add TournamentID to Division, scope queries to tournament, switch to pgx placeholders"
```

---

### Task 4: Team DB layer

**Files:**
- Rewrite: `mydb/teams.go`

- [ ] **Step 1: Rewrite `mydb/teams.go`**

```go
package mydb

import (
	"database/sql"
	"log"
)

type Team struct {
	ID           int
	TournamentID int
	Name         string
	Coach        string
	Division     Division
	Wins         int
	Losses       int
	RunsAgainst  int
	RunsFor      int
}

func (me *MyDB) AddTeam(tournamentID, divisionID int, name, coach string) {
	_, err := me.DB.Exec(
		`INSERT INTO teams (tournament_id, division_id, name, coach) VALUES ($1,$2,$3,$4)`,
		tournamentID, divisionID, name, coach,
	)
	if err != nil {
		log.Println("AddTeam:", err)
	}
}

func (me *MyDB) DelTeam(id int) {
	me.DB.Exec(`DELETE FROM teams WHERE id=$1`, id)
}

func (me *MyDB) ReturnTeamsByDivisionID(divisionID int) []Team {
	rows, err := me.DB.Query(
		`SELECT id, tournament_id, name, coach, division_id FROM teams WHERE division_id=$1`,
		divisionID,
	)
	if err != nil {
		log.Println("ReturnTeamsByDivisionID:", err)
		return nil
	}
	var teams []Team
	for rows.Next() {
		var t Team
		var did int
		rows.Scan(&t.ID, &t.TournamentID, &t.Name, &t.Coach, &did)
		t.Division = me.ReturnDivisionByID(did)
		teams = append(teams, t)
	}
	rows.Close()
	return teams
}

func (me *MyDB) ReturnTeamsByDivisionIDWithStats(divisionID int) []Team {
	teams := me.ReturnTeamsByDivisionID(divisionID)
	for i := range teams {
		teams[i].Wins = me.TeamWins(teams[i].ID)
		teams[i].Losses = me.TeamLosses(teams[i].ID)
		teams[i].RunsAgainst = me.TeamScoredAgainst(teams[i].ID)
		teams[i].RunsFor = me.TeamScoredFor(teams[i].ID)
	}
	return teams
}

func (me *MyDB) ReturnTeamByID(id int) Team {
	var t Team
	var did int
	err := me.DB.QueryRow(
		`SELECT id, tournament_id, name, coach, division_id FROM teams WHERE id=$1`, id,
	).Scan(&t.ID, &t.TournamentID, &t.Name, &t.Coach, &did)
	if err != nil && err != sql.ErrNoRows {
		log.Println("ReturnTeamByID:", err)
		return t
	}
	t.Division = me.ReturnDivisionByID(did)
	return t
}

func (me *MyDB) TeamWins(id int) int {
	var n int
	me.DB.QueryRow(
		`SELECT COUNT(*) FROM games_by_team WHERE team_id=$1 AND team_score > opponent_score`, id,
	).Scan(&n)
	return n
}

func (me *MyDB) TeamLosses(id int) int {
	var n int
	me.DB.QueryRow(
		`SELECT COUNT(*) FROM games_by_team WHERE team_id=$1 AND team_score < opponent_score`, id,
	).Scan(&n)
	return n
}

func (me *MyDB) TeamScoredFor(id int) int {
	var s sql.NullInt64
	me.DB.QueryRow(`SELECT SUM(team_score) FROM games_by_team WHERE team_id=$1`, id).Scan(&s)
	if !s.Valid {
		return 0
	}
	return int(s.Int64)
}

func (me *MyDB) TeamScoredAgainst(id int) int {
	var s sql.NullInt64
	me.DB.QueryRow(`SELECT SUM(opponent_score) FROM games_by_team WHERE team_id=$1`, id).Scan(&s)
	if !s.Valid {
		return 0
	}
	return int(s.Int64)
}

func (me *MyDB) GamesPlayedByTeam(id int) int {
	var n int
	me.DB.QueryRow(`SELECT COUNT(*) FROM games_by_team WHERE team_id=$1`, id).Scan(&n)
	return n
}
```

- [ ] **Step 2: Verify**

```bash
go build ./mydb/...
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add mydb/teams.go
git commit -m "feat: add TournamentID to Team, scope queries, switch to pgx placeholders"
```

---

### Task 5: Game DB layer

**Files:**
- Rewrite: `mydb/games.go`

- [ ] **Step 1: Rewrite `mydb/games.go`**

```go
package mydb

import (
	"database/sql"
	"log"
)

type Game struct {
	ID           int
	TournamentID int
	Division     Division
	HomeTeam     Team
	AwayTeam     Team
	Location     string
	Start        string
	Umpire       string
	AwayScore    int
	HomeScore    int
	Scored       bool
}

const gameSelect = `SELECT id, tournament_id, division_id, home_team_id, away_team_id, location, start_time, umpire, home_score, away_score FROM games`

func (me *MyDB) scanGames(rows *sql.Rows) []Game {
	var out []Game
	for rows.Next() {
		var g Game
		var hid, aid, did int
		var homeScore, awayScore sql.NullInt64
		rows.Scan(&g.ID, &g.TournamentID, &did, &hid, &aid, &g.Location, &g.Start, &g.Umpire, &homeScore, &awayScore)
		g.Division = me.ReturnDivisionByID(did)
		g.HomeTeam = me.ReturnTeamByID(hid)
		g.AwayTeam = me.ReturnTeamByID(aid)
		if homeScore.Valid && awayScore.Valid {
			g.HomeScore = int(homeScore.Int64)
			g.AwayScore = int(awayScore.Int64)
			g.Scored = true
		}
		out = append(out, g)
	}
	rows.Close()
	return out
}

func (me *MyDB) AddGame(tournamentID, divisionID, homeTeamID, awayTeamID int, location, startTime, umpire string) {
	_, err := me.DB.Exec(
		`INSERT INTO games (tournament_id, division_id, home_team_id, away_team_id, location, start_time, umpire) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		tournamentID, divisionID, homeTeamID, awayTeamID, location, startTime, umpire,
	)
	if err != nil {
		log.Println("AddGame:", err)
	}
}

func (me *MyDB) AllGamesByDivision(divisionID int) []Game {
	rows, err := me.DB.Query(gameSelect+` WHERE division_id=$1`, divisionID)
	if err != nil {
		log.Println("AllGamesByDivision:", err)
		return nil
	}
	return me.scanGames(rows)
}

func (me *MyDB) AllGamesByTeam(teamID int) []Game {
	rows, err := me.DB.Query(gameSelect+` WHERE home_team_id=$1 OR away_team_id=$1`, teamID)
	if err != nil {
		log.Println("AllGamesByTeam:", err)
		return nil
	}
	return me.scanGames(rows)
}

func (me *MyDB) ReturnGameByID(gameID int) Game {
	rows, err := me.DB.Query(gameSelect+` WHERE id=$1`, gameID)
	if err != nil {
		log.Println("ReturnGameByID:", err)
		return Game{}
	}
	games := me.scanGames(rows)
	if len(games) == 0 {
		return Game{}
	}
	return games[0]
}

func (me *MyDB) DelGame(id int) {
	me.DB.Exec(`DELETE FROM games WHERE id=$1`, id)
}

func (me *MyDB) AllGames(tournamentID int) []Game {
	rows, err := me.DB.Query(gameSelect+` WHERE tournament_id=$1 ORDER BY start_time`, tournamentID)
	if err != nil {
		log.Println("AllGames:", err)
		return nil
	}
	return me.scanGames(rows)
}

func (me *MyDB) ScoreGame(gid, hscore, ascore int) {
	_, err := me.DB.Exec(
		`UPDATE games SET home_score=$1, away_score=$2 WHERE id=$3`,
		hscore, ascore, gid,
	)
	if err != nil {
		log.Println("ScoreGame:", err)
		return
	}
	game := me.ReturnGameByID(gid)
	me.DeleteTeamScore(game.ID)
	me.AddTeamScore(game.Division.TournamentID, game.Division.ID, game.HomeTeam.ID, game.AwayTeam.ID, game.ID, hscore, ascore)
	me.AddTeamScore(game.Division.TournamentID, game.Division.ID, game.AwayTeam.ID, game.HomeTeam.ID, game.ID, ascore, hscore)
}

func (me *MyDB) DeleteTeamScore(gameID int) {
	me.DB.Exec(`DELETE FROM games_by_team WHERE game_id=$1`, gameID)
}

func (me *MyDB) DidTeamABeatTeamB(teamAID, teamBID int) (bool, bool) {
	var teamAScore, teamBScore int
	err := me.DB.QueryRow(
		`SELECT team_score, opponent_score FROM games_by_team WHERE team_id=$1 AND opponent_id=$2`,
		teamAID, teamBID,
	).Scan(&teamAScore, &teamBScore)
	if err == sql.ErrNoRows {
		return false, false
	}
	if err != nil {
		log.Println("DidTeamABeatTeamB:", err)
		return false, false
	}
	return true, teamAScore > teamBScore
}

func (me *MyDB) IsGameScored(id int) bool {
	var n int
	me.DB.QueryRow(`SELECT COUNT(*) FROM games_by_team WHERE game_id=$1`, id).Scan(&n)
	return n == 2
}
```

- [ ] **Step 2: Verify**

```bash
go build ./mydb/...
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add mydb/games.go
git commit -m "feat: add TournamentID to Game, nullable scores, scanGames helper, pgx placeholders"
```

---

### Task 6: webhandler data structs

**Files:**
- Rewrite: `webhandler/templates.go`

- [ ] **Step 1: Rewrite `webhandler/templates.go`**

```go
package webhandler

import (
	"bytes"
	"embed"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/csrf"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

//go:embed templates
var templateFS embed.FS

var tmpl *template.Template

func init() {
	tmpl = template.Must(template.New("").Funcs(template.FuncMap{
		"inc": func(i int) int { return i + 1 },
		"dec": func(i int) int { return i - 1 },
	}).ParseFS(templateFS,
		"templates/*.html",
		"templates/admin/*.html",
	))
}

type baseData struct {
	IsAdmin    bool
	CSRFField  template.HTML
	Tournament mydb.Tournament
}

func newBase(r *http.Request, isAdmin bool) baseData {
	return baseData{IsAdmin: isAdmin, CSRFField: csrf.TemplateField(r)}
}

func newBaseWithTournament(r *http.Request, isAdmin bool, t mydb.Tournament) baseData {
	return baseData{IsAdmin: isAdmin, CSRFField: csrf.TemplateField(r), Tournament: t}
}

type divisionTeamRow struct {
	mydb.Team
	GamesPlayed int
}

// Home page: tournament listing
type indexData struct {
	baseData
	ComingUp      []mydb.Tournament
	Recent        []mydb.Tournament
	Future        []mydb.Tournament
	Past          []mydb.Tournament
	FuturePage    int
	PastPage      int
	FutureTotal   int
	PastTotal     int
	FutureHasPrev bool
	FutureHasNext bool
	PastHasPrev   bool
	PastHasNext   bool
}

// Public tournament home: divisions + teams overview
type tournamentData struct {
	baseData
	Divisions []mydb.Division
	Teams     map[int][]mydb.Team
}

// Admin tournament list
type adminTournamentsData struct {
	baseData
	Tournaments []mydb.Tournament
}

// Admin tournament home
type adminTournamentViewData struct {
	baseData
	DisableDelete bool
}

type divisionData struct {
	baseData
	Division mydb.Division
	Teams    []divisionTeamRow
	Games    []mydb.Game
}

type teamData struct {
	baseData
	Team  mydb.Team
	Games []mydb.Game
}

type gamesData struct {
	baseData
	Games []mydb.Game
}

type loginData struct {
	baseData
	Error string
}

type adminDivisionsData struct {
	baseData
	Divisions     []mydb.Division
	DisableDelete bool
}

type adminDivisionViewData struct {
	baseData
	Division      mydb.Division
	DivisionID    int
	Teams         []mydb.Team
	Games         []mydb.Game
	DisableDelete bool
}

type adminTeamsData struct {
	baseData
	Divisions       []mydb.Division
	TeamsByDivision map[int][]mydb.Team
	DisableDelete   bool
}

type createGameData struct {
	baseData
	DivisionID    int
	Teams         []mydb.Team
	Games         []mydb.Game
	DisableDelete bool
}

type scoreGameData struct {
	baseData
	Game         mydb.Game
	ScoreOptions []int
}

type adminGamesData struct {
	baseData
	Games         []mydb.Game
	DisableDelete bool
}

func (me *Env) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Println("template error:", name, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}
```

- [ ] **Step 2: Note**

`go build ./...` will fail at this step — webhandler handler files still call old DB signatures. Expected.

- [ ] **Step 3: Commit**

```bash
git add webhandler/templates.go
git commit -m "feat: add Tournament to baseData, new tournament data structs, dec template func"
```

---

### Task 7: Tournament handlers + new templates

**Files:**
- Create: `webhandler/tournaments.go`
- Rewrite: `webhandler/templates/index.html`
- Create: `webhandler/templates/tournament.html`
- Create: `webhandler/templates/admin/tournaments.html`
- Create: `webhandler/templates/admin/tournament_view.html`

- [ ] **Step 1: Create `webhandler/tournaments.go`**

```go
package webhandler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func (me *Env) tournamentFromRoute(w http.ResponseWriter, ps httprouter.Params) (mydb.Tournament, bool) {
	tid, err := strconv.Atoi(ps.ByName("tid"))
	if err != nil {
		http.Error(w, "Bad tournament ID", http.StatusBadRequest)
		return mydb.Tournament{}, false
	}
	t := me.DB.ReturnTournamentByID(tid)
	if t.ID == 0 {
		http.Error(w, "Tournament not found", http.StatusNotFound)
		return mydb.Tournament{}, false
	}
	return t, true
}

func (me *Env) TournamentList(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	futurePage, _ := strconv.Atoi(r.URL.Query().Get("future_page"))
	pastPage, _ := strconv.Atoi(r.URL.Query().Get("past_page"))
	if futurePage < 1 {
		futurePage = 1
	}
	if pastPage < 1 {
		pastPage = 1
	}
	future, futureTotal := me.DB.ReturnTournamentsFuture(futurePage)
	past, pastTotal := me.DB.ReturnTournamentsPast(pastPage)
	me.render(w, "index", indexData{
		baseData:      newBase(r, false),
		ComingUp:      me.DB.ReturnTournamentsComingUp(),
		Recent:        me.DB.ReturnTournamentsRecent(),
		Future:        future,
		Past:          past,
		FuturePage:    futurePage,
		PastPage:      pastPage,
		FutureTotal:   futureTotal,
		PastTotal:     pastTotal,
		FutureHasPrev: futurePage > 1,
		FutureHasNext: futurePage*20 < futureTotal,
		PastHasPrev:   pastPage > 1,
		PastHasNext:   pastPage*20 < pastTotal,
	})
}

func (me *Env) TournamentHome(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	divs := me.DB.ReturnDivisions(t.ID)
	teams := make(map[int][]mydb.Team)
	for _, div := range divs {
		teams[div.ID] = me.DB.ReturnTeamsByDivisionID(div.ID)
	}
	me.render(w, "tournament", tournamentData{
		baseData:  newBaseWithTournament(r, false, t),
		Divisions: divs,
		Teams:     teams,
	})
}

func (me *Env) AdminTournaments(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "adminTournaments", adminTournamentsData{
		baseData:    newBase(r, true),
		Tournaments: me.DB.ReturnTournaments(),
	})
}

func (me *Env) CreateTournament(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := r.FormValue("name")
	sport := r.FormValue("sport")
	location := r.FormValue("location")
	notes := r.FormValue("notes")
	dateStr := r.FormValue("start_date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "Invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
		return
	}
	if name == "" || sport == "" || location == "" {
		http.Error(w, "Name, sport, and location are required", http.StatusBadRequest)
		return
	}
	id := me.DB.AddTournament(name, sport, location, notes, date)
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d", id), http.StatusSeeOther)
}

func (me *Env) AdminTournamentView(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	me.render(w, "adminTournamentView", adminTournamentViewData{
		baseData:      newBaseWithTournament(r, true, t),
		DisableDelete: me.DisableDelete,
	})
}
```

- [ ] **Step 2: Rewrite `webhandler/templates/index.html`**

```html
{{define "index"}}
{{template "header" .}}
<h1>Tournaments</h1>

<form onsubmit="event.preventDefault(); var v=document.getElementById('jump-id').value.trim(); if(v) window.location='/tournaments/'+v;">
  <div class="form-inline mb-3">
    <label class="mr-2">Tournament ID:</label>
    <input id="jump-id" type="number" class="form-control mr-2" style="width:120px">
    <button type="submit" class="btn btn-secondary">Go</button>
  </div>
</form>

{{if and (not .ComingUp) (not .Recent)}}
<p>No active tournaments.</p>
{{end}}

{{if .ComingUp}}
<h2>Coming Up</h2>
{{range .ComingUp}}
<div class="card mb-2">
  <div class="card-body">
    <h5 class="card-title"><a href="/tournaments/{{.ID}}">{{.Name}}</a></h5>
    <p class="card-text">{{.Sport}} | {{.Location}} | {{.StartDate.Format "Jan 2, 2006"}}</p>
  </div>
</div>
{{end}}
{{end}}

{{if .Recent}}
<h2>Recently Happened</h2>
{{range .Recent}}
<div class="card mb-2">
  <div class="card-body">
    <h5 class="card-title"><a href="/tournaments/{{.ID}}">{{.Name}}</a></h5>
    <p class="card-text">{{.Sport}} | {{.Location}} | {{.StartDate.Format "Jan 2, 2006"}}</p>
  </div>
</div>
{{end}}
{{end}}

{{if .Future}}
<button class="btn btn-outline-secondary mb-2" type="button" data-toggle="collapse" data-target="#future-section">
  Future ({{.FutureTotal}})
</button>
<div class="collapse" id="future-section">
  {{range .Future}}
  <div class="card mb-2">
    <div class="card-body">
      <h5 class="card-title"><a href="/tournaments/{{.ID}}">{{.Name}}</a></h5>
      <p class="card-text">{{.Sport}} | {{.Location}} | {{.StartDate.Format "Jan 2, 2006"}}</p>
    </div>
  </div>
  {{end}}
  <p>Showing {{len .Future}} of {{.FutureTotal}}</p>
  {{if .FutureHasPrev}}<a href="?future_page={{dec .FuturePage}}&past_page={{.PastPage}}">Prev</a> {{end}}
  {{if .FutureHasNext}}<a href="?future_page={{inc .FuturePage}}&past_page={{.PastPage}}">Next</a>{{end}}
</div>
{{end}}

{{if .Past}}
<button class="btn btn-outline-secondary mb-2" type="button" data-toggle="collapse" data-target="#past-section">
  Past ({{.PastTotal}})
</button>
<div class="collapse" id="past-section">
  {{range .Past}}
  <div class="card mb-2">
    <div class="card-body">
      <h5 class="card-title"><a href="/tournaments/{{.ID}}">{{.Name}}</a></h5>
      <p class="card-text">{{.Sport}} | {{.Location}} | {{.StartDate.Format "Jan 2, 2006"}}</p>
    </div>
  </div>
  {{end}}
  <p>Showing {{len .Past}} of {{.PastTotal}}</p>
  {{if .PastHasPrev}}<a href="?future_page={{.FuturePage}}&past_page={{dec .PastPage}}">Prev</a> {{end}}
  {{if .PastHasNext}}<a href="?future_page={{.FuturePage}}&past_page={{inc .PastPage}}">Next</a>{{end}}
</div>
{{end}}

{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Create `webhandler/templates/tournament.html`**

```html
{{define "tournament"}}
{{template "header" .}}
<h1>{{.Tournament.Name}}</h1>
<p>{{.Tournament.Sport}} | {{.Tournament.Location}} | {{.Tournament.StartDate.Format "Jan 2, 2006"}}</p>
{{if .Tournament.Notes}}<p>{{.Tournament.Notes}}</p>{{end}}
<p>Click a division for standings. Click a team for their schedule.</p>
{{range .Divisions}}
<h2><a href="/tournaments/{{$.Tournament.ID}}/divisions/{{.ID}}">{{.Name}}</a></h2>
<ul>
{{range index $.Teams .ID}}
<li><a href="/tournaments/{{$.Tournament.ID}}/teams/{{.ID}}">{{.Name}}</a> {{.Coach}}</li>
{{end}}
</ul>
{{end}}
{{template "footer" .}}
{{end}}
```

- [ ] **Step 4: Create `webhandler/templates/admin/tournaments.html`**

```html
{{define "adminTournaments"}}
{{template "header" .}}
<h1>Tournaments</h1>
<h2>Create Tournament</h2>
<form method="post" action="/admin/tournaments">
<table>
<tr><td>Name</td><td><input type="text" name="name" required></td></tr>
<tr><td>Sport</td><td><input type="text" name="sport" required></td></tr>
<tr><td>Location</td><td><input type="text" name="location" required></td></tr>
<tr><td>Start Date</td><td><input type="date" name="start_date" required></td></tr>
<tr><td>Notes</td><td><input type="text" name="notes"></td></tr>
<tr><td></td><td><input type="submit" value="Create Tournament"></td></tr>
</table>
{{.CSRFField}}
</form>
<h2>All Tournaments</h2>
<table border="1" cellpadding="1" cellspacing="0">
<tr><th>Name</th><th>Sport</th><th>Location</th><th>Start Date</th></tr>
{{range .Tournaments}}
<tr>
  <td><a href="/admin/tournaments/{{.ID}}">{{.Name}}</a></td>
  <td>{{.Sport}}</td>
  <td>{{.Location}}</td>
  <td>{{.StartDate.Format "Jan 2, 2006"}}</td>
</tr>
{{end}}
</table>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 5: Create `webhandler/templates/admin/tournament_view.html`**

```html
{{define "adminTournamentView"}}
{{template "header" .}}
<h1>{{.Tournament.Name}}</h1>
<p>{{.Tournament.Sport}} | {{.Tournament.Location}} | {{.Tournament.StartDate.Format "Jan 2, 2006"}}</p>
<ul>
  <li><a href="/admin/tournaments/{{.Tournament.ID}}/divisions">Manage Divisions</a></li>
  <li><a href="/admin/tournaments/{{.Tournament.ID}}/teams">Manage Teams</a></li>
  <li><a href="/admin/tournaments/{{.Tournament.ID}}/games">Manage Games</a></li>
</ul>
{{if .DisableDelete}}<p>Deletes have been disabled during the tournament.</p>{{end}}
<p><a href="/tournaments/{{.Tournament.ID}}">View Public Page</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 6: Commit**

```bash
git add webhandler/tournaments.go \
        webhandler/templates/index.html \
        webhandler/templates/tournament.html \
        webhandler/templates/admin/tournaments.html \
        webhandler/templates/admin/tournament_view.html
git commit -m "feat: add tournament handlers and templates (listing, home, admin)"
```

---

### Task 8: Update existing handlers

**Files:**
- Rewrite: `webhandler/webhandler.go`
- Rewrite: `webhandler/divisions.go`
- Rewrite: `webhandler/teams.go`

- [ ] **Step 1: Rewrite `webhandler/webhandler.go`**

Remove `PrintIndex` and `AdminIndex`. Add `"fmt"` to imports. Update all handlers to use `tournamentFromRoute`, new DB signatures, and `newBaseWithTournament`. Update Login redirect target.

```go
package webhandler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/rivo/sessions"
)

type contextKey struct{}

type Env struct {
	DB            *mydb.MyDB
	AdminPW       string
	DisableDelete bool
}

func New(db *mydb.MyDB, adminpw string, dd bool) *Env {
	return &Env{DB: db, AdminPW: adminpw, DisableDelete: dd}
}

func (me *Env) RequestLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userdata := me.MySession(w, r)
		forwardedip := r.Header.Get("X-Forwarded-For")
		log.Println(r.Method, r.URL.Path, r.Proto, forwardedip)
		if strings.HasPrefix(r.URL.Path, "/admin") && userdata.ID < 1 {
			log.Println(r.RemoteAddr, r.Method, r.URL.Path, userdata.UserName, "Permission Denied")
			http.Error(w, "Not authorized", http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), contextKey{}, userdata.ID)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (me *Env) LoginForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "login", loginData{baseData: newBase(r, false)})
}

func (me *Env) Login(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	password := r.FormValue("password")
	username := r.FormValue("username")
	if password == me.AdminPW {
		session, err := sessions.Start(w, r, true)
		if err != nil {
			log.Println("Session Failed to start", err)
		}
		session.Set("userid", username)
		http.Redirect(w, r, "/admin/tournaments", http.StatusSeeOther)
		return
	}
	me.render(w, "login", loginData{
		baseData: newBase(r, false),
		Error:    "Login Failed",
	})
}

func (me *Env) Logout(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {}

func (me *Env) PrintDivision(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad division ID", http.StatusBadRequest)
		return
	}
	rawTeams := me.DB.ReturnTeamsByDivisionIDWithStats(did)
	rawTeams = me.SortTeams(rawTeams, "WinsRunsAgainstRunsEarnedHead2Head")
	rows := make([]divisionTeamRow, len(rawTeams))
	for i, team := range rawTeams {
		rows[i] = divisionTeamRow{Team: team, GamesPlayed: me.DB.GamesPlayedByTeam(team.ID)}
	}
	me.render(w, "divisions", divisionData{
		baseData: newBaseWithTournament(r, false, t),
		Division: me.DB.ReturnDivisionByID(did),
		Teams:    rows,
		Games:    me.DB.AllGamesByDivision(did),
	})
}

func (me *Env) DelGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	gid, err := strconv.Atoi(ps.ByName("gid"))
	if err != nil {
		http.Error(w, "Bad Game ID", http.StatusBadRequest)
		return
	}
	if !me.DisableDelete {
		me.DB.DelGame(gid)
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/games", t.ID), http.StatusSeeOther)
}

func (me *Env) ScoreGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	gid, err := strconv.Atoi(ps.ByName("gid"))
	if err != nil {
		http.Error(w, "Bad Game ID", http.StatusBadRequest)
		return
	}
	options := make([]int, 41)
	for i := range options {
		options[i] = i
	}
	me.render(w, "scoreGame", scoreGameData{
		baseData:     newBaseWithTournament(r, true, t),
		Game:         me.DB.ReturnGameByID(gid),
		ScoreOptions: options,
	})
}

func (me *Env) RecordScore(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	gid, err := strconv.Atoi(ps.ByName("gid"))
	if err != nil {
		http.Error(w, "Bad game ID", http.StatusBadRequest)
		return
	}
	hscore, err := strconv.Atoi(r.FormValue("homescore"))
	if err != nil {
		http.Error(w, "Bad home score", http.StatusBadRequest)
		return
	}
	ascore, err := strconv.Atoi(r.FormValue("awayscore"))
	if err != nil {
		http.Error(w, "Bad away score", http.StatusBadRequest)
		return
	}
	me.DB.ScoreGame(gid, hscore, ascore)
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/games", t.ID), http.StatusSeeOther)
}

func (me *Env) Games(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	me.render(w, "games", gamesData{
		baseData: newBaseWithTournament(r, false, t),
		Games:    me.DB.AllGames(t.ID),
	})
}

func (me *Env) AdminGames(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	me.render(w, "adminGames", adminGamesData{
		baseData:      newBaseWithTournament(r, true, t),
		Games:         me.DB.AllGames(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) CreateGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad division ID", http.StatusBadRequest)
		return
	}
	me.render(w, "createGame", createGameData{
		baseData:      newBaseWithTournament(r, true, t),
		DivisionID:    did,
		Teams:         me.DB.ReturnTeamsByDivisionID(did),
		Games:         me.DB.AllGamesByDivision(did),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) CreateGameSubmit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(r.FormValue("divisionid"))
	if err != nil {
		http.Error(w, "Bad DivisionID", http.StatusBadRequest)
		return
	}
	hid, err := strconv.Atoi(r.FormValue("hometeam"))
	if err != nil {
		http.Error(w, "Bad Home team ID", http.StatusBadRequest)
		return
	}
	aid, err := strconv.Atoi(r.FormValue("awayteam"))
	if err != nil {
		http.Error(w, "Bad Away team ID", http.StatusBadRequest)
		return
	}
	if aid == hid {
		http.Error(w, "Must select a different team as an opponent.", http.StatusBadRequest)
		return
	}
	me.DB.AddGame(t.ID, did, hid, aid, r.FormValue("location"), r.FormValue("datetime"), r.FormValue("umpire"))
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/divisions/%d/games/new", t.ID, did), http.StatusSeeOther)
}

func (me *Env) PrintHRDerby(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "hrderby", newBase(r, false))
}

type TWUser struct {
	ID       int
	UserName string
}

func (me *Env) MySession(w http.ResponseWriter, r *http.Request) TWUser {
	var user TWUser
	user.ID = -1
	session, err := sessions.Start(w, r, false)
	if err != nil || session == nil {
		return user
	}
	userid := session.Get("userid", nil)
	s, ok := userid.(string)
	if !ok || s == "" {
		return user
	}
	user.ID = 1
	user.UserName = s
	return user
}
```

Note: add `"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"` to the import block — it was implicitly available before via the `mydb` field type but now needs explicit import since we removed the type alias approach.

- [ ] **Step 2: Rewrite `webhandler/divisions.go`**

```go
package webhandler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) AddDivisionForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("divisionname")
		if name == "" {
			http.Error(w, "Division name required", http.StatusBadRequest)
			return
		}
		me.DB.AddDivision(t.ID, name)
		http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/divisions", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "adminDivisions", adminDivisionsData{
		baseData:      newBaseWithTournament(r, true, t),
		Divisions:     me.DB.ReturnDivisions(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) DeleteDivision(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		log.Println("DeleteDivision bad ID:", err)
		http.Error(w, "Bad Division ID", http.StatusBadRequest)
		return
	}
	if !me.DisableDelete {
		me.DB.DelDivision(did)
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/divisions", t.ID), http.StatusSeeOther)
}

func (me *Env) AdminDivisionView(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		log.Println("AdminDivisionView bad ID:", err)
		http.Error(w, "Bad Division ID", http.StatusBadRequest)
		return
	}
	me.render(w, "adminDivisionView", adminDivisionViewData{
		baseData:      newBaseWithTournament(r, true, t),
		Division:      me.DB.ReturnDivisionByID(did),
		DivisionID:    did,
		Teams:         me.DB.ReturnTeamsByDivisionIDWithStats(did),
		Games:         me.DB.AllGamesByDivision(did),
		DisableDelete: me.DisableDelete,
	})
}
```

- [ ] **Step 3: Rewrite `webhandler/teams.go`**

```go
package webhandler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func (me *Env) Teams(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("teamname")
		coach := r.FormValue("teamcoach")
		divisionID, err := strconv.Atoi(r.FormValue("division"))
		if err != nil || name == "" {
			http.Error(w, "Name and valid division required", http.StatusBadRequest)
			return
		}
		me.DB.AddTeam(t.ID, divisionID, name, coach)
		http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/teams", t.ID), http.StatusSeeOther)
		return
	}
	divs := me.DB.ReturnDivisions(t.ID)
	byDiv := make(map[int][]mydb.Team)
	for _, div := range divs {
		byDiv[div.ID] = me.DB.ReturnTeamsByDivisionID(div.ID)
	}
	me.render(w, "adminTeams", adminTeamsData{
		baseData:        newBaseWithTournament(r, true, t),
		Divisions:       divs,
		TeamsByDivision: byDiv,
		DisableDelete:   me.DisableDelete,
	})
}

func (me *Env) DeleteTeam(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(r.FormValue("teamid"))
	if err != nil {
		log.Println("DeleteTeam bad ID:", err)
		http.Error(w, "Bad team ID", http.StatusBadRequest)
		return
	}
	if !me.DisableDelete {
		me.DB.DelTeam(teamID)
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/teams", t.ID), http.StatusSeeOther)
}

func (me *Env) ShowTeam(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	tid, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		http.Error(w, "Bad Team ID", http.StatusBadRequest)
		return
	}
	team := me.DB.ReturnTeamByID(tid)
	if team.ID == 0 {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}
	me.render(w, "team", teamData{
		baseData: newBaseWithTournament(r, false, t),
		Team:     team,
		Games:    me.DB.AllGamesByTeam(tid),
	})
}
```

- [ ] **Step 4: Verify the project compiles**

```bash
go build ./...
```

Expected: PASS. This is the first time the full project compiles since the mydb API changed.

- [ ] **Step 5: Commit**

```bash
git add webhandler/webhandler.go webhandler/divisions.go webhandler/teams.go
git commit -m "feat: update all handlers to be tournament-scoped, split add/delete handlers"
```

---

### Task 9: Update existing templates

**Files:**
- Rewrite: `webhandler/templates/layout.html`
- Modify: `webhandler/templates/divisions.html` (no link changes, but verify renders with new baseData)
- Rewrite: `webhandler/templates/admin/divisions.html`
- Rewrite: `webhandler/templates/admin/division_view.html`
- Rewrite: `webhandler/templates/admin/teams.html`
- Rewrite: `webhandler/templates/admin/games.html`
- Rewrite: `webhandler/templates/admin/create_game.html`
- Rewrite: `webhandler/templates/admin/score_game.html`
- Delete: `webhandler/templates/admin/index.html`

- [ ] **Step 1: Rewrite `webhandler/templates/layout.html`**

```html
{{define "header"}}
<!doctype html>
<html lang="en">
<head>
<title>Battle at the Dawg Pound</title>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
<link rel="stylesheet" href="/style.css">
<link rel="stylesheet" href="https://stackpath.bootstrapcdn.com/bootstrap/4.5.0/css/bootstrap.min.css" integrity="sha384-9aIt2nRpC12Uk9gS9baDl411NQApFmC26EwAOH8WgZl5MYYxFfc+NcPb1dKGj7Sk" crossorigin="anonymous">
<script src="https://code.jquery.com/jquery-3.5.1.slim.min.js" integrity="sha384-DfXdz2htPH0lsSSs5nCTpuj/zy4C+OGpamoFVy38MVBnE+IbbVYUew+OrCXaRkfj" crossorigin="anonymous"></script>
<script src="https://cdn.jsdelivr.net/npm/popper.js@1.16.0/dist/umd/popper.min.js" integrity="sha384-Q6E9RHvbIyZFJoft+2mJbHaEWldlvI9IOYy5n3zV9zzTtmI3UksdQRVvoxMfooAo" crossorigin="anonymous"></script>
<script src="https://stackpath.bootstrapcdn.com/bootstrap/4.5.0/js/bootstrap.min.js" integrity="sha384-OgVRvuATP1z7JjHLkuOU7Xw704+h835Lr+6QL9UvYjZE3Ipu6Tp75j7Bh/kR0JKI" crossorigin="anonymous"></script>
</head>
<body>
<img src="/img/topimage.jpg"><br>
<a href="/">Home</a> | <a href="/hrderbyinfo">Skills &amp; HR derby Info</a>
{{if .IsAdmin}}
  {{if .Tournament.ID}}
  | <a href="/admin/tournaments/{{.Tournament.ID}}">Admin</a>
  | <a href="/admin/tournaments/{{.Tournament.ID}}/divisions">Divisions</a>
  | <a href="/admin/tournaments/{{.Tournament.ID}}/teams">Teams</a>
  | <a href="/admin/tournaments/{{.Tournament.ID}}/games">Games</a>
  {{else}}
  | <a href="/admin/tournaments">Admin</a>
  {{end}}
{{else}}
| <a href="/login">Login</a>
{{end}}
{{if .Tournament.ID}}<br><strong>{{.Tournament.Name}}</strong> — {{.Tournament.Sport}}{{end}}
<br><hr>
{{end}}

{{define "footer"}}
<br><hr>Powered by <a href="https://github.com/Harnish/tourneyweb">TourneyWeb</a>
</body></html>
{{end}}
```

- [ ] **Step 2: Rewrite `webhandler/templates/admin/divisions.html`**

```html
{{define "adminDivisions"}}
{{template "header" .}}
<h2>Add Division</h2>
<form method="post" action="/admin/tournaments/{{.Tournament.ID}}/divisions">
<table>
<tr><td>Division Name</td><td><input type="text" name="divisionname"></td></tr>
<tr><td></td><td><input type="submit" name="submit"></td></tr>
</table>
{{.CSRFField}}
</form>
<table border="1" cellpadding="1" cellspacing="0">
{{range .Divisions}}
<tr>
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/divisions/{{.ID}}">{{.Name}}</a></td>
  {{if not $.DisableDelete}}
  <td valign="top">
    <form method="post" action="/admin/tournaments/{{$.Tournament.ID}}/divisions/{{.ID}}/delete">
      {{$.CSRFField}}
      <input type="submit" name="delete" value="delete">
    </form>
  </td>
  {{end}}
  <td valign="top">
    <form action="/admin/tournaments/{{$.Tournament.ID}}/divisions/{{.ID}}/games/new">
      <input type="submit" value="Add Game">
    </form>
  </td>
</tr>
{{end}}
</table>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Rewrite `webhandler/templates/admin/division_view.html`**

```html
{{define "adminDivisionView"}}
{{template "header" .}}
<H1>{{.Division.Name}}</H1>
<table>
<tr><th>Team Name</th><th>Coach</th><th>Wins</th><th>Losses</th></tr>
{{range .Teams}}
<tr>
  <td>{{.Name}}</td>
  <td>{{.Coach}}</td>
  <td>{{.Wins}}</td>
  <td>{{.Losses}}</td>
</tr>
{{end}}
</table>
<br>
<form action="/admin/tournaments/{{.Tournament.ID}}/teams"><input type="submit" value="Add Team"></form>
<br>
<h2>Games</h2>
<table>
<tr><th>Home Team</th><th>Away Team</th><th>Location</th><th>Start time</th><th>Umpire</th><th>Score Game</th>{{if not $.DisableDelete}}<th>Delete Game</th>{{end}}<th>Home Score</th><th>Away Score</th></tr>
{{range .Games}}
<tr>
  <td>{{.HomeTeam.Name}} {{.HomeTeam.Coach}}</td>
  <td>{{.AwayTeam.Name}} {{.AwayTeam.Coach}}</td>
  <td>{{.Location}}</td>
  <td>{{.Start}}</td>
  <td>{{.Umpire}}</td>
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/score">Score Game</a></td>
  {{if not $.DisableDelete}}<td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/delete">Delete Game</a></td>{{end}}
  <td>{{.HomeScore}}</td>
  <td>{{.AwayScore}}</td>
</tr>
{{end}}
</table>
<br>
<form action="/admin/tournaments/{{.Tournament.ID}}/divisions/{{.DivisionID}}/games/new">
  <input type="submit" value="Add Game">
</form>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 4: Rewrite `webhandler/templates/admin/teams.html`**

```html
{{define "adminTeams"}}
{{template "header" .}}
<h2>Add Team</h2>
<form method="post" action="/admin/tournaments/{{.Tournament.ID}}/teams">
<table>
<tr><td>Team Name</td><td><input type="text" name="teamname"></td></tr>
<tr><td>Team Coach</td><td><input type="text" name="teamcoach"></td></tr>
<tr><td>Division</td><td>
  <select name="division">
  {{range .Divisions}}
  <option value="{{.ID}}">{{.Name}}</option>
  {{end}}
  </select>
</td></tr>
<tr><td></td><td><input type="submit" name="submit"></td></tr>
</table>
{{.CSRFField}}
</form>
{{range .Divisions}}
<h2>{{.Name}}</h2>
<table>
{{range index $.TeamsByDivision .ID}}
<tr>
  <td>{{.Name}}</td>
  <td>{{.Coach}}</td>
  <td>
  {{if not $.DisableDelete}}
  <form method="post" action="/admin/tournaments/{{$.Tournament.ID}}/teams/delete">
    <input type="hidden" name="teamid" value="{{.ID}}">
    {{$.CSRFField}}
    <input type="submit" name="Delete" value="Delete">
  </form>
  {{end}}
  </td>
</tr>
{{end}}
</table>
{{end}}
{{template "footer" .}}
{{end}}
```

- [ ] **Step 5: Rewrite `webhandler/templates/admin/games.html`**

```html
{{define "adminGames"}}
{{template "header" .}}
<table border="1" cellpadding="1" cellspacing="0">
<tr><th>Home Team</th><th>Home Score</th><th>Away Team</th><th>Away Score</th><th>Location</th><th>Start Time</th><th>Umpire</th><th>Score Game</th>{{if not $.DisableDelete}}<th>Delete</th>{{end}}</tr>
{{range .Games}}
<tr>
  <td>{{.HomeTeam.Name}} - {{.HomeTeam.Coach}}</td>
  <td>{{.HomeScore}}</td>
  <td>{{.AwayTeam.Name}} - {{.AwayTeam.Coach}}</td>
  <td>{{.AwayScore}}</td>
  <td>{{.Location}}</td>
  <td>{{.Start}}</td>
  <td>{{.Umpire}}</td>
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/score">Score Game</a></td>
  {{if not $.DisableDelete}}<td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/delete">Delete</a></td>{{end}}
</tr>
{{end}}
</table>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 6: Rewrite `webhandler/templates/admin/create_game.html`**

```html
{{define "createGame"}}
{{template "header" .}}
<form method="post" action="/admin/tournaments/{{.Tournament.ID}}/games">
<input type="hidden" name="divisionid" value="{{.DivisionID}}">
<table>
  <tr><td>Home Team</td><td><select name="hometeam">
    {{range .Teams}}<option value="{{.ID}}">{{.Name}} - {{.Coach}}</option>{{end}}
  </select></td></tr>
  <tr><td>Away Team</td><td><select name="awayteam">
    {{range .Teams}}<option value="{{.ID}}">{{.Name}} - {{.Coach}}</option>{{end}}
  </select></td></tr>
  <tr><td>Location</td><td><input type="text" name="location"></td></tr>
  <tr><td>Date/Time</td><td><input type="text" name="datetime"></td></tr>
  <tr><td>Umpire</td><td><input type="text" name="umpire"></td></tr>
  <tr><td></td><td><input type="submit" name="submit"></td></tr>
</table>
{{.CSRFField}}
</form>
<table>
<tr><th>Home Team</th><th>Away Team</th><th>Location</th><th>Start time</th><th>Umpire</th><th>Score Game</th>{{if not $.DisableDelete}}<th>Delete Game</th>{{end}}<th>Home Score</th><th>Away Score</th></tr>
{{range .Games}}
<tr>
  <td>{{.HomeTeam.Name}} {{.HomeTeam.Coach}}</td>
  <td>{{.AwayTeam.Name}} {{.AwayTeam.Coach}}</td>
  <td>{{.Location}}</td>
  <td>{{.Start}}</td>
  <td>{{.Umpire}}</td>
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/score">Score Game</a></td>
  {{if not $.DisableDelete}}<td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/delete">Delete Game</a></td>{{end}}
  <td>{{.HomeScore}}</td>
  <td>{{.AwayScore}}</td>
</tr>
{{end}}
</table>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 7: Rewrite `webhandler/templates/admin/score_game.html`**

The form no longer needs a hidden `gameid` field — gid is in the route URL.

```html
{{define "scoreGame"}}
{{template "header" .}}
<form method="post" action="/admin/tournaments/{{.Tournament.ID}}/games/{{.Game.ID}}/score">
<table>
  <tr><td>Home Team</td><td>{{.Game.HomeTeam.Name}}</td></tr>
  <tr><td>Away Team</td><td>{{.Game.AwayTeam.Name}}</td></tr>
  <tr><td>Location</td><td>{{.Game.Location}}</td></tr>
  <tr><td>Start Time</td><td>{{.Game.Start}}</td></tr>
  <tr><td>Umpire</td><td>{{.Game.Umpire}}</td></tr>
  <tr><td>Home Team Score</td><td>
    <select name="homescore">
      {{range .ScoreOptions}}<option value="{{.}}">{{.}}</option>{{end}}
    </select>
  </td></tr>
  <tr><td>Away Team Score</td><td>
    <select name="awayscore">
      {{range .ScoreOptions}}<option value="{{.}}">{{.}}</option>{{end}}
    </select>
  </td></tr>
  <tr><td></td><td><input type="submit" name="Save" value="Save"></td></tr>
</table>
{{.CSRFField}}
</form>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 8: Delete the obsolete admin index template**

```bash
git rm webhandler/templates/admin/index.html
```

- [ ] **Step 9: Verify**

```bash
go build ./...
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add webhandler/templates/layout.html \
        webhandler/templates/admin/divisions.html \
        webhandler/templates/admin/division_view.html \
        webhandler/templates/admin/teams.html \
        webhandler/templates/admin/games.html \
        webhandler/templates/admin/create_game.html \
        webhandler/templates/admin/score_game.html
git commit -m "feat: update all templates to tournament-scoped routes"
```

---

### Task 10: Route rewrite (main.go)

**Files:**
- Rewrite routes section of: `main.go`

- [ ] **Step 1: Replace all `router.*` registrations in `main.go`**

Find the block from `router := httprouter.New()` through `csrfKey :=` and replace the router registrations with:

```go
router := httprouter.New()

// Static assets
router.GET("/style.css", PrintCSS)
router.GET("/favicon.ico", PrintFavIco)
router.GET("/img/topimage.jpg", PrintBannerLogo)

// Public
router.GET("/", wh.TournamentList)
router.GET("/login", wh.LoginForm)
router.POST("/login", wh.Login)
router.GET("/hrderbyinfo", wh.PrintHRDerby)
router.GET("/tournaments/:tid", wh.TournamentHome)
router.GET("/tournaments/:tid/divisions/:did", wh.PrintDivision)
router.GET("/tournaments/:tid/teams/:teamid", wh.ShowTeam)
router.GET("/tournaments/:tid/games", wh.Games)

// Admin — tournaments
router.GET("/admin/tournaments", wh.AdminTournaments)
router.POST("/admin/tournaments", wh.CreateTournament)
router.GET("/admin/tournaments/:tid", wh.AdminTournamentView)

// Admin — divisions
router.GET("/admin/tournaments/:tid/divisions", wh.AddDivisionForm)
router.POST("/admin/tournaments/:tid/divisions", wh.AddDivisionForm)
router.GET("/admin/tournaments/:tid/divisions/:did", wh.AdminDivisionView)
router.POST("/admin/tournaments/:tid/divisions/:did/delete", wh.DeleteDivision)

// Admin — teams
router.GET("/admin/tournaments/:tid/teams", wh.Teams)
router.POST("/admin/tournaments/:tid/teams", wh.Teams)
router.POST("/admin/tournaments/:tid/teams/delete", wh.DeleteTeam)

// Admin — games
router.GET("/admin/tournaments/:tid/games", wh.AdminGames)
router.GET("/admin/tournaments/:tid/divisions/:did/games/new", wh.CreateGame)
router.POST("/admin/tournaments/:tid/games", wh.CreateGameSubmit)
router.GET("/admin/tournaments/:tid/games/:gid/score", wh.ScoreGame)
router.POST("/admin/tournaments/:tid/games/:gid/score", wh.RecordScore)
router.GET("/admin/tournaments/:tid/games/:gid/delete", wh.DelGame)
```

- [ ] **Step 2: Remove unused imports from `main.go`**

`go-spew/spew` (used for `spew.Dump(cfg)`) is still referenced — leave it unless the user wants it removed. `ioutil` is deprecated — can replace `ioutil.ReadFile` with `os.ReadFile` but only change if the compiler warns.

- [ ] **Step 3: Final build**

```bash
go build ./...
```

Expected: PASS.

- [ ] **Step 4: Run vet**

```bash
go vet ./...
```

Expected: no issues.

- [ ] **Step 5: Commit**

```bash
git add main.go
git commit -m "feat: rewrite routes for tournament-scoped URLs, remove old single-tournament routes"
```

---

## Config for local dev

Update `tourneyweb.conf` or `config.yaml` before running:

```yaml
port: 8989
debug: true
database: postgres://user:password@localhost:5432/tourneyweb
adminpassword: somepassword
disabledelete: false
bannerimagepath: dawgpoundlogo.jpg
csrfkey: <64 hex chars from: openssl rand -hex 32>
```

The app fatals on startup if `database` is not a `postgres://` URL.
