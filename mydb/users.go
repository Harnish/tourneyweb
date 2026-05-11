package mydb

import (
	"database/sql"
	"log"
	"time"
)

type User struct {
	ID                int
	Email             string
	Name              string
	PasswordHash      string
	EmailVerified     bool
	VerificationToken string
	ResetToken        string
	ResetExpires      time.Time
	IsAdmin           bool
	CreatedAt         time.Time
}

func (me *MyDB) CountUsers() int {
	var n int
	me.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n
}

func (me *MyDB) CreateUser(email, name, passwordHash, verificationToken string, emailVerified, isAdmin bool) (User, error) {
	var u User
	err := me.DB.QueryRow(
		`INSERT INTO users (email, name, password_hash, verification_token, email_verified, is_admin)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, email, name, password_hash, email_verified, COALESCE(verification_token,''), is_admin, created_at`,
		email, name, passwordHash, nullableString(verificationToken), emailVerified, isAdmin,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.EmailVerified, &u.VerificationToken, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		log.Println("CreateUser:", err)
	}
	return u, err
}

func (me *MyDB) GetUserByEmail(email string) (User, error) {
	return me.scanUser(me.DB.QueryRow(
		`SELECT id, email, name, password_hash, email_verified,
		        COALESCE(verification_token,''), COALESCE(reset_token,''),
		        COALESCE(reset_expires, '1970-01-01'::timestamptz), is_admin, created_at
		 FROM users WHERE email = $1`, email))
}

func (me *MyDB) GetUserByID(id int) (User, error) {
	return me.scanUser(me.DB.QueryRow(
		`SELECT id, email, name, password_hash, email_verified,
		        COALESCE(verification_token,''), COALESCE(reset_token,''),
		        COALESCE(reset_expires, '1970-01-01'::timestamptz), is_admin, created_at
		 FROM users WHERE id = $1`, id))
}

func (me *MyDB) GetUserByVerificationToken(token string) (User, error) {
	return me.scanUser(me.DB.QueryRow(
		`SELECT id, email, name, password_hash, email_verified,
		        COALESCE(verification_token,''), COALESCE(reset_token,''),
		        COALESCE(reset_expires, '1970-01-01'::timestamptz), is_admin, created_at
		 FROM users WHERE verification_token = $1`, token))
}

func (me *MyDB) GetUserByResetToken(token string) (User, error) {
	return me.scanUser(me.DB.QueryRow(
		`SELECT id, email, name, password_hash, email_verified,
		        COALESCE(verification_token,''), COALESCE(reset_token,''),
		        COALESCE(reset_expires, '1970-01-01'::timestamptz), is_admin, created_at
		 FROM users WHERE reset_token = $1`, token))
}

func (me *MyDB) MarkEmailVerified(userID int) {
	_, err := me.DB.Exec(
		`UPDATE users SET email_verified=true, verification_token=NULL WHERE id=$1`, userID)
	if err != nil {
		log.Println("MarkEmailVerified:", err)
	}
}

func (me *MyDB) SetVerificationToken(userID int, token string) {
	_, err := me.DB.Exec(
		`UPDATE users SET verification_token=$1 WHERE id=$2`, token, userID)
	if err != nil {
		log.Println("SetVerificationToken:", err)
	}
}

func (me *MyDB) SetResetToken(userID int, token string, expires time.Time) {
	_, err := me.DB.Exec(
		`UPDATE users SET reset_token=$1, reset_expires=$2 WHERE id=$3`, token, expires, userID)
	if err != nil {
		log.Println("SetResetToken:", err)
	}
}

func (me *MyDB) UpdatePassword(userID int, passwordHash string) {
	_, err := me.DB.Exec(
		`UPDATE users SET password_hash=$1, reset_token=NULL, reset_expires=NULL WHERE id=$2`,
		passwordHash, userID)
	if err != nil {
		log.Println("UpdatePassword:", err)
	}
}

func (me *MyDB) scanUser(row *sql.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash,
		&u.EmailVerified, &u.VerificationToken, &u.ResetToken, &u.ResetExpires,
		&u.IsAdmin, &u.CreatedAt)
	if err != nil && err != sql.ErrNoRows {
		log.Println("scanUser:", err)
	}
	return u, err
}

// nullableString returns nil for empty strings (stored as NULL).
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
