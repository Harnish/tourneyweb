package mydb

import (
	"log/slog"
	"time"
)

// ScoreReport is one coach team's reported score for a game. Reporter is "coach_home" or "coach_away".
// ReportedAt is set once and never changes (it defines who reported first); edits set UpdatedAt.
type ScoreReport struct {
	GameID               int
	Reporter             string
	UserID               int
	HomeScore, AwayScore int
	ReportedAt           time.Time
	UpdatedAt            time.Time
}

func (me *MyDB) SaveScoreReport(gameID int, reporter string, userID, home, away int) {
	_, err := me.DB.Exec(`INSERT INTO game_score_reports (game_id, reporter, user_id, home_score, away_score)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (game_id, reporter) DO UPDATE SET user_id=$3, home_score=$4, away_score=$5, updated_at=NOW()`,
		gameID, reporter, userID, home, away)
	if err != nil {
		slog.Error("SaveScoreReport", "err", err)
	}
}

func (me *MyDB) ScoreReportsForGame(gameID int) []ScoreReport {
	rows, err := me.DB.Query(`SELECT game_id, reporter, user_id, home_score, away_score, reported_at, updated_at
		FROM game_score_reports WHERE game_id=$1 ORDER BY reported_at ASC, reporter ASC`, gameID)
	if err != nil {
		slog.Error("ScoreReportsForGame", "err", err)
		return nil
	}
	defer rows.Close()
	var out []ScoreReport
	for rows.Next() {
		var r ScoreReport
		if rows.Scan(&r.GameID, &r.Reporter, &r.UserID, &r.HomeScore, &r.AwayScore, &r.ReportedAt, &r.UpdatedAt) == nil {
			out = append(out, r)
		}
	}
	return out
}

func (me *MyDB) SetGameScoreStatus(gameID int, status string) {
	if _, err := me.DB.Exec(`UPDATE games SET score_status=$1 WHERE id=$2`, status, gameID); err != nil {
		slog.Error("SetGameScoreStatus", "err", err)
	}
}
