package mydb

import (
	"database/sql"
	"log/slog"
)

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
