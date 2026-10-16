package mydb

import (
	"sort"
	"time"
)

func (f *FakeDB) AddNotification(userID int, subject, message, link string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	f.notifications = append(f.notifications, Notification{ID: id, UserID: userID, Subject: subject, Message: message, Link: link, CreatedAt: time.Now()})
	return id
}

func (f *FakeDB) ListNotifications(userID, limit int) []Notification {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Notification
	for _, n := range f.notifications {
		if n.UserID == userID {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID > out[j].ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (f *FakeDB) CountUnreadNotifications(userID int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := 0
	for _, n := range f.notifications {
		if n.UserID == userID && n.ReadAt.IsZero() {
			c++
		}
	}
	return c
}

func (f *FakeDB) MarkNotificationRead(id, userID int) (Notification, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, n := range f.notifications {
		if n.ID == id && n.UserID == userID {
			if n.ReadAt.IsZero() {
				f.notifications[i].ReadAt = time.Now()
			}
			return f.notifications[i], true
		}
	}
	return Notification{}, false
}

func (f *FakeDB) MarkAllNotificationsRead(userID int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, n := range f.notifications {
		if n.UserID == userID && n.ReadAt.IsZero() {
			f.notifications[i].ReadAt = time.Now()
		}
	}
}

func (f *FakeDB) ListAdminUserIDs() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []int
	for id, u := range f.users {
		if u.IsAdmin {
			out = append(out, id)
		}
	}
	sort.Ints(out)
	return out
}
