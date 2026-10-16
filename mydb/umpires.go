package mydb

import (
	"database/sql"
	"errors"
	"log/slog"
	"time"
)

var (
	ErrUmpireConflict  = errors.New("umpire has an overlapping game")
	ErrUmpireDuplicate = errors.New("umpire already assigned to this game")
	// ErrUmpireSlotDeclined: a declined slot is final until management reassigns it.
	ErrUmpireSlotDeclined = errors.New("slot was already declined")
)

// umpireWindow is how long a game blocks its umpires, starting at the game's start time.
const umpireWindow = 2 * time.Hour

// UmpireWindowsOverlap reports whether games starting at a and b overlap. Half-open: back-to-back games do not.
func UmpireWindowsOverlap(a, b time.Time) bool {
	return a.After(b.Add(-umpireWindow)) && a.Before(b.Add(umpireWindow))
}

// Umpire is a user flagged as an umpire, with their credentials.
type Umpire struct {
	UserID       int
	Name         string
	Email        string
	CertURL      string
	HasCertImage bool
}

// UmpireSlot is one umpire position on a game. UserID 0 means a free-form name.
type UmpireSlot struct {
	ID, GameID, Slot, UserID int
	FreeName, Status         string
	UserName, UserEmail      string
}

func clampUmpiresNeeded(n int) int {
	if n < 1 {
		return 1
	}
	if n > 4 {
		return 4
	}
	return n
}

func (me *MyDB) SetUmpireProfile(userID int, isUmpire bool, certURL string) {
	if _, err := me.DB.Exec(`UPDATE users SET is_umpire=$1, cert_url=$2 WHERE id=$3`, isUmpire, certURL, userID); err != nil {
		slog.Error("SetUmpireProfile", "err", err)
	}
}

func (me *MyDB) SetUmpireCertImage(userID int, data []byte, contentType string) {
	if _, err := me.DB.Exec(`UPDATE users SET cert_image=$1, cert_image_type=$2 WHERE id=$3`, data, contentType, userID); err != nil {
		slog.Error("SetUmpireCertImage", "err", err)
	}
}

func (me *MyDB) GetUmpireCertImage(userID int) ([]byte, string, bool) {
	var data []byte
	var ct string
	err := me.DB.QueryRow(`SELECT cert_image, cert_image_type FROM users WHERE id=$1`, userID).Scan(&data, &ct)
	if err != nil || len(data) == 0 {
		return nil, "", false
	}
	return data, ct, true
}

const umpireSelect = `SELECT id, name, email, cert_url, cert_image IS NOT NULL FROM users WHERE is_umpire`

func (me *MyDB) GetUmpire(userID int) (Umpire, bool) {
	var u Umpire
	err := me.DB.QueryRow(umpireSelect+` AND id=$1`, userID).Scan(&u.UserID, &u.Name, &u.Email, &u.CertURL, &u.HasCertImage)
	if err == sql.ErrNoRows {
		return Umpire{}, false
	}
	if err != nil {
		slog.Error("GetUmpire", "err", err)
		return Umpire{}, false
	}
	return u, true
}

func (me *MyDB) ListUmpires() []Umpire {
	rows, err := me.DB.Query(umpireSelect + ` ORDER BY name`)
	if err != nil {
		slog.Error("ListUmpires", "err", err)
		return nil
	}
	defer rows.Close()
	var out []Umpire
	for rows.Next() {
		var u Umpire
		if err := rows.Scan(&u.UserID, &u.Name, &u.Email, &u.CertURL, &u.HasCertImage); err != nil {
			slog.Error("ListUmpires scan", "err", err)
			continue
		}
		out = append(out, u)
	}
	return out
}

func (me *MyDB) SetUmpireVerified(tournamentID, userID, byUserID int, verified bool) {
	var err error
	if verified {
		_, err = me.DB.Exec(`INSERT INTO umpire_verifications (user_id, tournament_id, verified_by) VALUES ($1,$2,$3)
			ON CONFLICT (user_id, tournament_id) DO UPDATE SET verified_by=$3, verified_at=NOW()`, userID, tournamentID, byUserID)
	} else {
		_, err = me.DB.Exec(`DELETE FROM umpire_verifications WHERE user_id=$1 AND tournament_id=$2`, userID, tournamentID)
	}
	if err != nil {
		slog.Error("SetUmpireVerified", "err", err)
	}
}

func (me *MyDB) VerifiedUmpireIDs(tournamentID int) map[int]bool {
	out := map[int]bool{}
	rows, err := me.DB.Query(`SELECT user_id FROM umpire_verifications WHERE tournament_id=$1`, tournamentID)
	if err != nil {
		slog.Error("VerifiedUmpireIDs", "err", err)
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			out[id] = true
		}
	}
	return out
}

func (me *MyDB) DivisionUmpiresNeeded(divisionID int) int {
	n := 1
	me.DB.QueryRow(`SELECT umpires_needed FROM divisions WHERE id=$1`, divisionID).Scan(&n)
	return clampUmpiresNeeded(n)
}

func (me *MyDB) SetDivisionUmpiresNeeded(divisionID, n int) {
	if _, err := me.DB.Exec(`UPDATE divisions SET umpires_needed=$1 WHERE id=$2`, clampUmpiresNeeded(n), divisionID); err != nil {
		slog.Error("SetDivisionUmpiresNeeded", "err", err)
	}
}

// --- Slots ---

type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

const slotSelect = `SELECT gu.id, gu.game_id, gu.slot, COALESCE(gu.user_id,0), gu.free_name, gu.status,
	COALESCE(u.name,''), COALESCE(u.email,'')
	FROM game_umpires gu LEFT JOIN users u ON u.id = gu.user_id`

func (me *MyDB) querySlots(where string, args ...any) []UmpireSlot {
	rows, err := me.DB.Query(slotSelect+" "+where, args...)
	if err != nil {
		slog.Error("querySlots", "err", err)
		return nil
	}
	defer rows.Close()
	var out []UmpireSlot
	for rows.Next() {
		var s UmpireSlot
		if err := rows.Scan(&s.ID, &s.GameID, &s.Slot, &s.UserID, &s.FreeName, &s.Status, &s.UserName, &s.UserEmail); err != nil {
			slog.Error("querySlots scan", "err", err)
			continue
		}
		out = append(out, s)
	}
	return out
}

func (me *MyDB) GameUmpires(gameID int) []UmpireSlot {
	return me.querySlots(`WHERE gu.game_id=$1 ORDER BY gu.slot`, gameID)
}

func (me *MyDB) UmpireSlotsForUser(userID int) []UmpireSlot {
	return me.querySlots(`WHERE gu.user_id=$1 ORDER BY gu.game_id, gu.slot`, userID)
}

func umpireConflict(q rowQuerier, userID int, start time.Time, excludeGameID int) bool {
	if start.IsZero() {
		return false
	}
	var found bool
	err := q.QueryRow(`SELECT EXISTS (
		SELECT 1 FROM game_umpires gu JOIN games g ON g.id = gu.game_id
		WHERE gu.user_id=$1 AND gu.status <> 'declined' AND g.id <> $4
		  AND g.start_time > $2 AND g.start_time < $3)`,
		userID, start.Add(-umpireWindow), start.Add(umpireWindow), excludeGameID).Scan(&found)
	if err != nil {
		slog.Error("umpireConflict", "err", err)
	}
	return found
}

func (me *MyDB) UmpireConflict(userID int, start time.Time, excludeGameID int) bool {
	return umpireConflict(me.DB, userID, start, excludeGameID)
}

// refreshUmpireCache rewrites games.umpire from the non-declined slots.
func refreshUmpireCache(ex interface {
	Exec(query string, args ...any) (sql.Result, error)
}, gameID int) {
	_, err := ex.Exec(`UPDATE games SET umpire = COALESCE((
		SELECT string_agg(COALESCE(u.name, gu.free_name), ', ' ORDER BY gu.slot)
		FROM game_umpires gu LEFT JOIN users u ON u.id = gu.user_id
		WHERE gu.game_id=$1 AND gu.status <> 'declined'), '') WHERE id=$1`, gameID)
	if err != nil {
		slog.Error("refreshUmpireCache", "err", err)
	}
}

func (me *MyDB) AssignUmpire(gameID, slot, userID int, freeName string) error {
	tx, err := me.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if userID > 0 {
		// Serialize concurrent assignments of the same umpire.
		if _, err := tx.Exec(`SELECT 1 FROM users WHERE id=$1 FOR UPDATE`, userID); err != nil {
			return err
		}
		var dup bool
		if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM game_umpires WHERE game_id=$1 AND user_id=$2 AND slot <> $3)`,
			gameID, userID, slot).Scan(&dup); err != nil {
			return err
		}
		if dup {
			return ErrUmpireDuplicate
		}
		var start sql.NullTime
		if err := tx.QueryRow(`SELECT start_time FROM games WHERE id=$1`, gameID).Scan(&start); err != nil {
			return err
		}
		if start.Valid && umpireConflict(tx, userID, start.Time, gameID) {
			return ErrUmpireConflict
		}
		_, err = tx.Exec(`INSERT INTO game_umpires (game_id, slot, user_id, free_name, status) VALUES ($1,$2,$3,'','pending')
			ON CONFLICT (game_id, slot) DO UPDATE SET user_id=$3, free_name='', status='pending'`, gameID, slot, userID)
	} else {
		_, err = tx.Exec(`INSERT INTO game_umpires (game_id, slot, user_id, free_name, status) VALUES ($1,$2,NULL,$3,'accepted')
			ON CONFLICT (game_id, slot) DO UPDATE SET user_id=NULL, free_name=$3, status='accepted'`, gameID, slot, freeName)
	}
	if err != nil {
		return err
	}
	refreshUmpireCache(tx, gameID)
	return tx.Commit()
}

func (me *MyDB) ClearUmpireSlot(gameID, slot int) {
	if _, err := me.DB.Exec(`DELETE FROM game_umpires WHERE game_id=$1 AND slot=$2`, gameID, slot); err != nil {
		slog.Error("ClearUmpireSlot", "err", err)
		return
	}
	refreshUmpireCache(me.DB, gameID)
}

func (me *MyDB) RespondUmpireSlot(gameID, slot, userID int, accept bool) error {
	status := "declined"
	if accept {
		status = "accepted"
	}
	res, err := me.DB.Exec(`UPDATE game_umpires SET status=$1 WHERE game_id=$2 AND slot=$3 AND user_id=$4 AND status <> 'declined'`, status, gameID, slot, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var declined bool
		me.DB.QueryRow(`SELECT EXISTS (SELECT 1 FROM game_umpires WHERE game_id=$1 AND slot=$2 AND user_id=$3 AND status='declined')`,
			gameID, slot, userID).Scan(&declined)
		if declined {
			return ErrUmpireSlotDeclined
		}
		return errors.New("slot not assigned to this user")
	}
	refreshUmpireCache(me.DB, gameID)
	return nil
}

func (me *MyDB) ResetUmpireSlotsPending(gameID int) []UmpireSlot {
	affected := me.querySlots(`WHERE gu.game_id=$1 AND gu.user_id IS NOT NULL AND gu.status='accepted' ORDER BY gu.slot`, gameID)
	if len(affected) == 0 {
		return nil
	}
	if _, err := me.DB.Exec(`UPDATE game_umpires SET status='pending' WHERE game_id=$1 AND user_id IS NOT NULL AND status='accepted'`, gameID); err != nil {
		slog.Error("ResetUmpireSlotsPending", "err", err)
		return nil
	}
	return affected
}
