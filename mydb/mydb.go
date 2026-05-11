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
	`CREATE TABLE IF NOT EXISTS users (
		id                  SERIAL PRIMARY KEY,
		email               TEXT UNIQUE NOT NULL,
		name                TEXT NOT NULL,
		password_hash       TEXT NOT NULL,
		email_verified      BOOLEAN NOT NULL DEFAULT false,
		verification_token  TEXT,
		reset_token         TEXT,
		reset_expires       TIMESTAMPTZ,
		is_admin            BOOLEAN NOT NULL DEFAULT false,
		created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS tournament_roles (
		id             SERIAL PRIMARY KEY,
		user_id        INTEGER NOT NULL REFERENCES users(id),
		tournament_id  INTEGER NOT NULL REFERENCES tournaments(id),
		role           TEXT NOT NULL CHECK (role IN ('director','staff','coach')),
		team_id        INTEGER REFERENCES teams(id),
		UNIQUE(user_id, tournament_id)
	)`,
	`CREATE TABLE IF NOT EXISTS invitations (
		id             SERIAL PRIMARY KEY,
		email          TEXT NOT NULL,
		tournament_id  INTEGER NOT NULL REFERENCES tournaments(id),
		role           TEXT NOT NULL CHECK (role IN ('director','staff','coach')),
		team_id        INTEGER REFERENCES teams(id),
		token          TEXT NOT NULL UNIQUE,
		expires_at     TIMESTAMPTZ NOT NULL,
		created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
	_, err := me.DB.Exec(
		`INSERT INTO games_by_team (tournament_id, division_id, team_id, opponent_id, game_id, team_score, opponent_score) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		tournamentID, divisionID, teamID, opponentID, gameID, teamScore, opponentScore,
	)
	if err != nil {
		log.Println("AddTeamScore:", err)
	}
}
