package mydb

import (
	"database/sql"
	"log/slog"
)

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
	TeamID    int // 0 = bye
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
	TopTeamName    string
	BottomTeamName string
	WinnerTeamName string
	Game           Game
}

func (me *MyDB) CreateBracket(divisionID int, format string, size int) int {
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO brackets (division_id, format, status, size) VALUES ($1,$2,'seeding',$3) RETURNING id`,
		divisionID, format, size,
	).Scan(&id)
	if err != nil {
		slog.Error("CreateBracket", "err", err)
	}
	return id
}

func (me *MyDB) GetBracketByID(id int) Bracket {
	var b Bracket
	err := me.DB.QueryRow(
		`SELECT id, division_id, format, status, size FROM brackets WHERE id=$1`, id,
	).Scan(&b.ID, &b.DivisionID, &b.Format, &b.Status, &b.Size)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("GetBracketByID", "err", err)
	}
	return b
}

func (me *MyDB) GetBracketByDivisionID(divisionID int) Bracket {
	var b Bracket
	err := me.DB.QueryRow(
		`SELECT id, division_id, format, status, size FROM brackets WHERE division_id=$1`, divisionID,
	).Scan(&b.ID, &b.DivisionID, &b.Format, &b.Status, &b.Size)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("GetBracketByDivisionID", "err", err)
	}
	return b
}

func (me *MyDB) SetBracketStatus(bracketID int, status string) {
	_, err := me.DB.Exec(`UPDATE brackets SET status=$1 WHERE id=$2`, status, bracketID)
	if err != nil {
		slog.Error("SetBracketStatus", "err", err)
	}
}

func (me *MyDB) AddBracketSeed(bracketID, seed, teamID int) {
	var tid interface{}
	if teamID != 0 {
		tid = teamID
	}
	_, err := me.DB.Exec(
		`INSERT INTO bracket_seeds (bracket_id, seed, team_id) VALUES ($1,$2,$3)`,
		bracketID, seed, tid,
	)
	if err != nil {
		slog.Error("AddBracketSeed", "err", err)
	}
}

func (me *MyDB) GetBracketSeeds(bracketID int) []BracketSeed {
	rows, err := me.DB.Query(
		`SELECT bs.id, bs.bracket_id, bs.seed, COALESCE(bs.team_id,0), COALESCE(t.name,'')
		 FROM bracket_seeds bs LEFT JOIN teams t ON t.id=bs.team_id
		 WHERE bs.bracket_id=$1 ORDER BY bs.seed`,
		bracketID,
	)
	if err != nil {
		slog.Error("GetBracketSeeds", "err", err)
		return nil
	}
	var out []BracketSeed
	for rows.Next() {
		var s BracketSeed
		if err := rows.Scan(&s.ID, &s.BracketID, &s.Seed, &s.TeamID, &s.TeamName); err != nil {
			slog.Error("GetBracketSeeds scan", "err", err)
			continue
		}
		out = append(out, s)
	}
	rows.Close()
	return out
}

func (me *MyDB) UpdateBracketSeeds(bracketID int, teamIDs []int) {
	if _, err := me.DB.Exec(`DELETE FROM bracket_seeds WHERE bracket_id=$1`, bracketID); err != nil {
		slog.Error("UpdateBracketSeeds delete", "err", err)
		return
	}
	for i, tid := range teamIDs {
		me.AddBracketSeed(bracketID, i+1, tid)
	}
}

func (me *MyDB) AddBracketGame(bracketID, round, position int) int {
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO bracket_games (bracket_id, round, position) VALUES ($1,$2,$3) RETURNING id`,
		bracketID, round, position,
	).Scan(&id)
	if err != nil {
		slog.Error("AddBracketGame", "err", err)
	}
	return id
}

func (me *MyDB) SetBracketGameTeams(id, topTeamID, bottomTeamID int, topIsBye, bottomIsBye bool) {
	var top, bottom interface{}
	if topTeamID != 0 {
		top = topTeamID
	}
	if bottomTeamID != 0 {
		bottom = bottomTeamID
	}
	_, err := me.DB.Exec(
		`UPDATE bracket_games SET top_team_id=$1, bottom_team_id=$2, top_is_bye=$3, bottom_is_bye=$4 WHERE id=$5`,
		top, bottom, topIsBye, bottomIsBye, id,
	)
	if err != nil {
		slog.Error("SetBracketGameTeams", "err", err)
	}
}

func (me *MyDB) SetBracketGameTopTeam(id, teamID int) {
	_, err := me.DB.Exec(`UPDATE bracket_games SET top_team_id=$1 WHERE id=$2`, teamID, id)
	if err != nil {
		slog.Error("SetBracketGameTopTeam", "err", err)
	}
}

func (me *MyDB) SetBracketGameBottomTeam(id, teamID int) {
	_, err := me.DB.Exec(`UPDATE bracket_games SET bottom_team_id=$1 WHERE id=$2`, teamID, id)
	if err != nil {
		slog.Error("SetBracketGameBottomTeam", "err", err)
	}
}

func (me *MyDB) SetBracketGameWinner(id, winnerTeamID int) {
	_, err := me.DB.Exec(`UPDATE bracket_games SET winner_team_id=$1 WHERE id=$2`, winnerTeamID, id)
	if err != nil {
		slog.Error("SetBracketGameWinner", "err", err)
	}
}

func (me *MyDB) SetBracketGameGameID(id, gameID int) {
	_, err := me.DB.Exec(`UPDATE bracket_games SET game_id=$1 WHERE id=$2`, gameID, id)
	if err != nil {
		slog.Error("SetBracketGameGameID", "err", err)
	}
}

const bracketGameSelect = `
	SELECT bg.id, bg.bracket_id, bg.round, bg.position,
	       COALESCE(bg.top_team_id,0), COALESCE(bg.bottom_team_id,0),
	       bg.top_is_bye, bg.bottom_is_bye,
	       COALESCE(bg.game_id,0), COALESCE(bg.winner_team_id,0),
	       COALESCE(tt.name,''), COALESCE(bt.name,''), COALESCE(wt.name,'')
	FROM bracket_games bg
	LEFT JOIN teams tt ON tt.id = bg.top_team_id
	LEFT JOIN teams bt ON bt.id = bg.bottom_team_id
	LEFT JOIN teams wt ON wt.id = bg.winner_team_id`

func scanBracketGame(row *sql.Row) (BracketGame, error) {
	var bg BracketGame
	err := row.Scan(
		&bg.ID, &bg.BracketID, &bg.Round, &bg.Position,
		&bg.TopTeamID, &bg.BottomTeamID,
		&bg.TopIsBye, &bg.BottomIsBye,
		&bg.GameID, &bg.WinnerTeamID,
		&bg.TopTeamName, &bg.BottomTeamName, &bg.WinnerTeamName,
	)
	return bg, err
}

func (me *MyDB) GetBracketGameByID(id int) BracketGame {
	bg, err := scanBracketGame(me.DB.QueryRow(bracketGameSelect+` WHERE bg.id=$1`, id))
	if err != nil && err != sql.ErrNoRows {
		slog.Error("GetBracketGameByID", "err", err)
	}
	if bg.GameID != 0 {
		bg.Game = me.ReturnGameByID(bg.GameID)
	}
	return bg
}

func (me *MyDB) GetBracketGameByGameID(gameID int) BracketGame {
	bg, err := scanBracketGame(me.DB.QueryRow(bracketGameSelect+` WHERE bg.game_id=$1`, gameID))
	if err != nil && err != sql.ErrNoRows {
		slog.Error("GetBracketGameByGameID", "err", err)
	}
	if bg.GameID != 0 {
		bg.Game = me.ReturnGameByID(bg.GameID)
	}
	return bg
}

func (me *MyDB) GetBracketGameByRoundPosition(bracketID, round, position int) BracketGame {
	bg, err := scanBracketGame(me.DB.QueryRow(
		bracketGameSelect+` WHERE bg.bracket_id=$1 AND bg.round=$2 AND bg.position=$3`,
		bracketID, round, position,
	))
	if err != nil && err != sql.ErrNoRows {
		slog.Error("GetBracketGameByRoundPosition", "err", err)
	}
	if bg.GameID != 0 {
		bg.Game = me.ReturnGameByID(bg.GameID)
	}
	return bg
}

func (me *MyDB) GetBracketGames(bracketID int) []BracketGame {
	rows, err := me.DB.Query(
		bracketGameSelect+` WHERE bg.bracket_id=$1 ORDER BY bg.round, bg.position`,
		bracketID,
	)
	if err != nil {
		slog.Error("GetBracketGames", "err", err)
		return nil
	}
	var out []BracketGame
	for rows.Next() {
		var bg BracketGame
		if err := rows.Scan(
			&bg.ID, &bg.BracketID, &bg.Round, &bg.Position,
			&bg.TopTeamID, &bg.BottomTeamID,
			&bg.TopIsBye, &bg.BottomIsBye,
			&bg.GameID, &bg.WinnerTeamID,
			&bg.TopTeamName, &bg.BottomTeamName, &bg.WinnerTeamName,
		); err != nil {
			slog.Error("GetBracketGames scan", "err", err)
			continue
		}
		if bg.GameID != 0 {
			bg.Game = me.ReturnGameByID(bg.GameID)
		}
		out = append(out, bg)
	}
	rows.Close()
	return out
}
