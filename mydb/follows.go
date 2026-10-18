package mydb

import (
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"
)

// MaxFollows bounds how many things one user can follow.
const MaxFollows = 50

var ErrFollowCap = errors.New("follow limit reached")

// Follow is one thing a user follows. Kind is "tournament", "division" or "team".
// Name and TournamentID are filled by UserFollows for display and link building.
type Follow struct {
	UserID       int
	Kind         string
	TargetID     int
	Name         string
	TournamentID int
}

func validFollowKind(kind string) bool {
	return kind == "tournament" || kind == "division" || kind == "team"
}

func (me *MyDB) AddFollow(userID int, kind string, targetID int) error {
	if !validFollowKind(kind) {
		return errors.New("unknown follow kind")
	}
	res, err := me.DB.Exec(`INSERT INTO follows (user_id, kind, target_id)
		SELECT $1, $2, $3 WHERE (SELECT COUNT(*) FROM follows WHERE user_id=$1) < $4
		ON CONFLICT DO NOTHING`, userID, kind, targetID, MaxFollows)
	if err != nil {
		slog.Error("AddFollow", "err", err)
		return err
	}
	if n, _ := res.RowsAffected(); n == 1 || me.IsFollowing(userID, kind, targetID) {
		return nil
	}
	return ErrFollowCap
}

func (me *MyDB) DelFollow(userID int, kind string, targetID int) {
	if _, err := me.DB.Exec(`DELETE FROM follows WHERE user_id=$1 AND kind=$2 AND target_id=$3`, userID, kind, targetID); err != nil {
		slog.Error("DelFollow", "err", err)
	}
}

func (me *MyDB) IsFollowing(userID int, kind string, targetID int) bool {
	var one int
	return me.DB.QueryRow(`SELECT 1 FROM follows WHERE user_id=$1 AND kind=$2 AND target_id=$3`, userID, kind, targetID).Scan(&one) == nil
}

func (me *MyDB) UserFollows(userID int) []Follow {
	rows, err := me.DB.Query(`SELECT f.kind, f.target_id, COALESCE(t.name, d.name, tm.name), COALESCE(t.id, d.tournament_id, tm.tournament_id)
		FROM follows f
		LEFT JOIN tournaments t ON f.kind='tournament' AND t.id=f.target_id
		LEFT JOIN divisions d ON f.kind='division' AND d.id=f.target_id
		LEFT JOIN teams tm ON f.kind='team' AND tm.id=f.target_id
		WHERE f.user_id=$1 AND COALESCE(t.name, d.name, tm.name) IS NOT NULL
		ORDER BY CASE f.kind WHEN 'tournament' THEN 0 WHEN 'division' THEN 1 ELSE 2 END, lower(COALESCE(t.name, d.name, tm.name)), f.target_id`, userID)
	if err != nil {
		slog.Error("UserFollows", "err", err)
		return nil
	}
	defer rows.Close()
	var out []Follow
	for rows.Next() {
		f := Follow{UserID: userID}
		if rows.Scan(&f.Kind, &f.TargetID, &f.Name, &f.TournamentID) == nil {
			out = append(out, f)
		}
	}
	return out
}

// GameFollowers maps each user following the game's division or either team to their most specific
// follow (a team follow wins over a division follow). Tournament follows are interest only and excluded.
func (me *MyDB) GameFollowers(g Game) map[int]Follow {
	rows, err := me.DB.Query(`SELECT user_id, kind, target_id FROM follows
		WHERE (kind='division' AND target_id=$1) OR (kind='team' AND target_id IN ($2,$3))`,
		g.Division.ID, g.HomeTeam.ID, g.AwayTeam.ID)
	if err != nil {
		slog.Error("GameFollowers", "err", err)
		return nil
	}
	defer rows.Close()
	out := map[int]Follow{}
	for rows.Next() {
		f := Follow{TournamentID: g.TournamentID}
		if rows.Scan(&f.UserID, &f.Kind, &f.TargetID) != nil {
			continue
		}
		if cur, ok := out[f.UserID]; ok && cur.Kind == "team" {
			continue
		}
		out[f.UserID] = f
	}
	return out
}

// FollowersMatching is SearchUsers limited to users following the tournament or the division.
func (me *MyDB) FollowersMatching(q string, tournamentID, divisionID, limit int) []User {
	q = strings.TrimSpace(q)
	if utf8.RuneCountInString(q) < 2 || limit <= 0 {
		return nil
	}
	rows, err := me.DB.Query(`SELECT id, email, name FROM users u
		WHERE name ILIKE $1 AND EXISTS (SELECT 1 FROM follows f WHERE f.user_id=u.id AND
			((f.kind='tournament' AND f.target_id=$2) OR (f.kind='division' AND f.target_id=$3)))
		ORDER BY lower(name), id LIMIT $4`, "%"+likeEscaper.Replace(q)+"%", tournamentID, divisionID, limit)
	if err != nil {
		slog.Error("FollowersMatching", "err", err)
		return nil
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if rows.Scan(&u.ID, &u.Email, &u.Name) == nil {
			out = append(out, u)
		}
	}
	return out
}

func (me *MyDB) SetMuteFollowAlerts(userID int, mute bool) {
	if _, err := me.DB.Exec(`UPDATE users SET mute_follow_alerts=$1 WHERE id=$2`, mute, userID); err != nil {
		slog.Error("SetMuteFollowAlerts", "err", err)
	}
}

func (me *MyDB) MutesFollowAlerts(userID int) bool {
	var mute bool
	me.DB.QueryRow(`SELECT mute_follow_alerts FROM users WHERE id=$1`, userID).Scan(&mute)
	return mute
}

// SearchTournaments finds published tournaments whose name contains q, newest first, at most 50.
func (me *MyDB) SearchTournaments(q string) []Tournament {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	rows, err := me.DB.Query(`SELECT id, name, sport, location, start_date, notes, extras_html, rules_html, status, default_ranking_criteria, kind, end_date, COALESCE(league_id,0)
		FROM tournaments WHERE status='published' AND name ILIKE $1 ORDER BY start_date DESC, id LIMIT 50`, "%"+likeEscaper.Replace(q)+"%")
	if err != nil {
		slog.Error("SearchTournaments", "err", err)
		return nil
	}
	return scanTournaments(rows)
}

// deleteWithFollows runs deleteSQL together with the follow cleanup in one transaction, so a delete the
// database refuses (team with games, division with teams) leaves its followers' follows alone.
func (me *MyDB) deleteWithFollows(name, followSQL, deleteSQL string, id int) {
	tx, err := me.DB.Begin()
	if err != nil {
		slog.Error(name, "err", err)
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec(followSQL, id); err != nil {
		slog.Error(name, "err", err)
		return
	}
	if _, err := tx.Exec(deleteSQL, id); err != nil {
		slog.Error(name, "err", err)
		return
	}
	if err := tx.Commit(); err != nil {
		slog.Error(name, "err", err)
	}
}

const delTournamentFollowsSQL = `DELETE FROM follows WHERE
	(kind='tournament' AND target_id=$1)
	OR (kind='division' AND target_id IN (SELECT id FROM divisions WHERE tournament_id=$1))
	OR (kind='team' AND target_id IN (SELECT id FROM teams WHERE tournament_id=$1))`
