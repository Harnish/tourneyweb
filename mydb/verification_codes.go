package mydb

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"
)

type VerificationCode struct {
	ID           int
	TournamentID int
	Code         string
	CreatedAt    time.Time
	RedeemedAt   time.Time
	Redeemed     bool
}

func generateVerificationCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}

func (me *MyDB) IssueVerificationCode(tournamentID int) (string, error) {
	code, err := generateVerificationCode()
	if err != nil {
		return "", err
	}
	_, err = me.DB.Exec(
		`INSERT INTO verification_codes (tournament_id, code) VALUES ($1, $2)`,
		tournamentID, code,
	)
	if err != nil {
		slog.Error("IssueVerificationCode", "err", err)
		return "", err
	}
	return code, nil
}

func (me *MyDB) RedeemVerificationCode(code string, tournamentID int) error {
	var id, storedTID int
	var redeemed bool
	err := me.DB.QueryRow(
		`SELECT id, tournament_id, redeemed_at IS NOT NULL FROM verification_codes WHERE code=$1`,
		code,
	).Scan(&id, &storedTID, &redeemed)
	if err == sql.ErrNoRows {
		return errors.New("invalid code")
	}
	if err != nil {
		return err
	}
	if storedTID != tournamentID {
		return errors.New("invalid code")
	}
	if redeemed {
		return errors.New("code already used")
	}
	tx, err := me.DB.Begin()
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE verification_codes SET redeemed_at=NOW() WHERE id=$1`, id); err != nil {
		tx.Rollback()
		return err
	}
	if _, err = tx.Exec(`UPDATE tournaments SET status='published' WHERE id=$1`, tournamentID); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
