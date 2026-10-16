package mydb

import (
	"database/sql"
	"log/slog"
	"time"
)

// Notification is one inbox entry. ReadAt is the zero time while unread.
type Notification struct {
	ID, UserID             int
	Subject, Message, Link string
	CreatedAt, ReadAt      time.Time
}

func (me *MyDB) AddNotification(userID int, subject, message, link string) int {
	var id int
	err := me.DB.QueryRow(`INSERT INTO notifications (user_id, subject, message, link) VALUES ($1,$2,$3,$4) RETURNING id`,
		userID, subject, message, link).Scan(&id)
	if err != nil {
		slog.Error("AddNotification", "err", err)
	}
	return id
}

const notificationSelect = `SELECT id, user_id, subject, message, link, created_at, read_at FROM notifications`

func scanNotification(sc interface{ Scan(...any) error }) (Notification, error) {
	var n Notification
	var read sql.NullTime
	err := sc.Scan(&n.ID, &n.UserID, &n.Subject, &n.Message, &n.Link, &n.CreatedAt, &read)
	n.ReadAt = read.Time
	return n, err
}

func (me *MyDB) ListNotifications(userID, limit int) []Notification {
	rows, err := me.DB.Query(notificationSelect+` WHERE user_id=$1 ORDER BY created_at DESC, id DESC LIMIT $2`, userID, limit)
	if err != nil {
		slog.Error("ListNotifications", "err", err)
		return nil
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		if n, err := scanNotification(rows); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func (me *MyDB) CountUnreadNotifications(userID int) int {
	var n int
	if err := me.DB.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id=$1 AND read_at IS NULL`, userID).Scan(&n); err != nil {
		slog.Error("CountUnreadNotifications", "err", err)
	}
	return n
}

// MarkNotificationRead marks one of userID's notifications read; false if it isn't theirs.
func (me *MyDB) MarkNotificationRead(id, userID int) (Notification, bool) {
	n, err := scanNotification(me.DB.QueryRow(
		`UPDATE notifications SET read_at = COALESCE(read_at, NOW()) WHERE id=$1 AND user_id=$2
		 RETURNING id, user_id, subject, message, link, created_at, read_at`, id, userID))
	return n, err == nil
}

func (me *MyDB) MarkAllNotificationsRead(userID int) {
	if _, err := me.DB.Exec(`UPDATE notifications SET read_at=NOW() WHERE user_id=$1 AND read_at IS NULL`, userID); err != nil {
		slog.Error("MarkAllNotificationsRead", "err", err)
	}
}

func (me *MyDB) ListAdminUserIDs() []int {
	rows, err := me.DB.Query(`SELECT id FROM users WHERE is_admin ORDER BY id`)
	if err != nil {
		slog.Error("ListAdminUserIDs", "err", err)
		return nil
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var id int
		if rows.Scan(&id) == nil {
			out = append(out, id)
		}
	}
	return out
}
