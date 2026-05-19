package mydb

import (
	"database/sql"
	"log/slog"
	"os"
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
		start_time    TIMESTAMPTZ,
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
	`CREATE TABLE IF NOT EXISTS login_attempts (
		ip           VARCHAR(45) PRIMARY KEY,
		count        INTEGER NOT NULL DEFAULT 0,
		locked_until TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		last_attempt TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
	`CREATE TABLE IF NOT EXISTS verification_codes (
		id            SERIAL PRIMARY KEY,
		tournament_id INTEGER NOT NULL REFERENCES tournaments(id),
		code          TEXT NOT NULL UNIQUE,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		redeemed_at   TIMESTAMPTZ
	)`,
}

type MyDB struct {
	DB    *sql.DB
	debug bool
}

func New(path string, debug bool) *MyDB {
	if !strings.HasPrefix(path, "postgres://") {
		slog.Error("database: must be a postgres:// URL", "got", path)
		os.Exit(1)
	}
	db, err := sql.Open("pgx", path)
	if err != nil {
		slog.Error("database: open", "err", err)
		os.Exit(1)
	}
	if err := db.Ping(); err != nil {
		slog.Error("database: ping", "err", err)
		os.Exit(1)
	}
	for _, ddl := range pgtables {
		if _, err := db.Exec(ddl); err != nil {
			slog.Error("database: create table", "err", err)
			os.Exit(1)
		}
	}
	for i, m := range pgmigrations {
		if _, err := db.Exec(m); err != nil {
			slog.Error("migration failed", "index", i, "err", err)
		} else {
			slog.Info("migration ok", "index", i)
		}
	}
	return &MyDB{DB: db, debug: debug}
}

// pgmigrations are idempotent ALTER TABLE statements run after table creation.
var pgmigrations = []string{
	`ALTER TABLE locations ADD COLUMN IF NOT EXISTS latitude  DOUBLE PRECISION NOT NULL DEFAULT 0`,
	`ALTER TABLE locations ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION NOT NULL DEFAULT 0`,
	`ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS extras_html TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE locations ADD COLUMN IF NOT EXISTS available_for TEXT NOT NULL DEFAULT ''`,
	`DO $$ BEGIN
		IF EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name='games' AND column_name='start_time' AND data_type='text'
		) THEN
			ALTER TABLE games ALTER COLUMN start_time DROP NOT NULL;
			ALTER TABLE games ALTER COLUMN start_time DROP DEFAULT;
			UPDATE games SET start_time = NULL;
			ALTER TABLE games ALTER COLUMN start_time TYPE TIMESTAMPTZ USING NULL::TIMESTAMPTZ;
		END IF;
	END $$`,
	`ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'published'`,
	`ALTER TABLE locations ADD COLUMN IF NOT EXISTS tournament_id INTEGER REFERENCES tournaments(id)`,
	`ALTER TABLE event_news ALTER COLUMN tournament_id DROP NOT NULL`,
	`ALTER TABLE event_news ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
	`ALTER TABLE event_news ADD COLUMN IF NOT EXISTS author_id INTEGER REFERENCES users(id) ON DELETE SET NULL`,
	`ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS rules_html TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE divisions  ADD COLUMN IF NOT EXISTS rules_html TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE divisions ADD COLUMN IF NOT EXISTS ranking_criteria TEXT`,
	`ALTER TABLE divisions ADD COLUMN IF NOT EXISTS phase TEXT NOT NULL DEFAULT 'pool'`,
	`CREATE TABLE IF NOT EXISTS brackets (
		id            SERIAL PRIMARY KEY,
		division_id   INT NOT NULL REFERENCES divisions(id) ON DELETE CASCADE,
		format        TEXT NOT NULL DEFAULT 'single_elimination',
		status        TEXT NOT NULL DEFAULT 'seeding',
		size          INT  NOT NULL,
		UNIQUE (division_id)
	)`,
	`ALTER TABLE brackets ADD CONSTRAINT IF NOT EXISTS brackets_division_id_key UNIQUE (division_id)`,
	`CREATE TABLE IF NOT EXISTS bracket_seeds (
		id          SERIAL PRIMARY KEY,
		bracket_id  INT NOT NULL REFERENCES brackets(id) ON DELETE CASCADE,
		seed        INT NOT NULL,
		team_id     INT
	)`,
	`CREATE TABLE IF NOT EXISTS bracket_games (
		id               SERIAL PRIMARY KEY,
		bracket_id       INT  NOT NULL REFERENCES brackets(id) ON DELETE CASCADE,
		round            INT  NOT NULL,
		position         INT  NOT NULL,
		top_team_id      INT,
		bottom_team_id   INT,
		top_is_bye       BOOL NOT NULL DEFAULT false,
		bottom_is_bye    BOOL NOT NULL DEFAULT false,
		game_id          INT,
		winner_team_id   INT
	)`,
	`ALTER TABLE games ADD COLUMN IF NOT EXISTS scrimmage_team_id INT NULL`,
	`CREATE TABLE IF NOT EXISTS players (
		id         SERIAL PRIMARY KEY,
		team_id    INT  NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
		number     TEXT NOT NULL DEFAULT '',
		first_name TEXT NOT NULL,
		last_name  TEXT NOT NULL,
		handed     TEXT NOT NULL DEFAULT '',
		position   TEXT NOT NULL DEFAULT ''
	)`,
}

func (me *MyDB) AddTeamScore(tournamentID, divisionID, teamID, opponentID, gameID, teamScore, opponentScore int) {
	_, err := me.DB.Exec(
		`INSERT INTO games_by_team (tournament_id, division_id, team_id, opponent_id, game_id, team_score, opponent_score) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		tournamentID, divisionID, teamID, opponentID, gameID, teamScore, opponentScore,
	)
	if err != nil {
		slog.Error("AddTeamScore", "err", err)
	}
}
