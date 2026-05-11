package mydb

import (
	"database/sql"
	"time"
)

type LoginAttempt struct {
	IP          string
	Count       int
	LockedUntil time.Time
}

// GetLoginAttempt returns the current attempt record for ip, or a zero-value record if none exists.
func (me *MyDB) GetLoginAttempt(ip string) (LoginAttempt, error) {
	var a LoginAttempt
	err := me.DB.QueryRow(
		`SELECT ip, count, locked_until FROM login_attempts WHERE ip = $1`, ip,
	).Scan(&a.IP, &a.Count, &a.LockedUntil)
	if err == sql.ErrNoRows {
		return LoginAttempt{IP: ip}, nil
	}
	return a, err
}

// RecordLoginFailure increments the failure count for ip and sets an exponential lockout:
// locked_until = NOW() + 2^min(count,8) seconds (2s → 256s cap).
func (me *MyDB) RecordLoginFailure(ip string) error {
	_, err := me.DB.Exec(`
		INSERT INTO login_attempts (ip, count, locked_until, last_attempt)
		VALUES ($1, 1, NOW() + make_interval(secs => 2), NOW())
		ON CONFLICT (ip) DO UPDATE
		SET count        = login_attempts.count + 1,
		    locked_until = NOW() + make_interval(secs => POWER(2, LEAST(login_attempts.count + 1, 8))::int),
		    last_attempt = NOW()
	`, ip)
	return err
}

// ClearLoginAttempts removes the attempt record for ip (call on successful login).
func (me *MyDB) ClearLoginAttempts(ip string) error {
	_, err := me.DB.Exec(`DELETE FROM login_attempts WHERE ip = $1`, ip)
	return err
}

// PruneLoginAttempts deletes records that haven't been touched in 24 hours.
func (me *MyDB) PruneLoginAttempts() error {
	_, err := me.DB.Exec(`DELETE FROM login_attempts WHERE last_attempt < NOW() - INTERVAL '24 hours'`)
	return err
}
