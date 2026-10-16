package mydb_test

import (
	"testing"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

func TestNotificationAddListNewestFirst(t *testing.T) {
	db := mydb.NewFakeDB()
	u := newUmpire(t, db, "u@x.com", "Una")
	a := db.AddNotification(u, "first", "m1", "/a")
	time.Sleep(2 * time.Millisecond)
	b := db.AddNotification(u, "second", "m2", "/b")
	time.Sleep(2 * time.Millisecond)
	c := db.AddNotification(u, "third", "m3", "")
	if a == 0 || b == 0 || c == 0 {
		t.Fatal("ids must be non-zero")
	}
	got := db.ListNotifications(u, 100)
	if len(got) != 3 || got[0].Subject != "third" || got[2].Subject != "first" {
		t.Fatalf("want newest first, got %+v", got)
	}
	if got[2].Link != "/a" || got[2].Message != "m1" || !got[2].ReadAt.IsZero() || got[2].CreatedAt.IsZero() {
		t.Fatalf("fields: %+v", got[2])
	}
	if got := db.ListNotifications(u, 2); len(got) != 2 || got[0].Subject != "third" {
		t.Fatalf("limit not honored: %+v", got)
	}
}

func TestNotificationsAreScopedToUser(t *testing.T) {
	db := mydb.NewFakeDB()
	u1 := newUmpire(t, db, "a@x.com", "A")
	u2 := newUmpire(t, db, "b@x.com", "B")
	db.AddNotification(u1, "s", "m", "")
	if len(db.ListNotifications(u2, 100)) != 0 || db.CountUnreadNotifications(u2) != 0 {
		t.Fatal("another user's notifications must not leak")
	}
}

func TestNotificationUnreadCount(t *testing.T) {
	db := mydb.NewFakeDB()
	u := newUmpire(t, db, "u@x.com", "Una")
	db.AddNotification(u, "a", "", "")
	db.AddNotification(u, "b", "", "")
	if got := db.CountUnreadNotifications(u); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
}

func TestMarkNotificationReadOwnerOnly(t *testing.T) {
	db := mydb.NewFakeDB()
	owner := newUmpire(t, db, "a@x.com", "A")
	other := newUmpire(t, db, "b@x.com", "B")
	id := db.AddNotification(owner, "s", "m", "/go")
	if _, ok := db.MarkNotificationRead(id, other); ok {
		t.Fatal("another user must not mark it read")
	}
	if db.CountUnreadNotifications(owner) != 1 {
		t.Fatal("state must not change")
	}
	n, ok := db.MarkNotificationRead(id, owner)
	if !ok || n.Link != "/go" || db.CountUnreadNotifications(owner) != 0 {
		t.Fatalf("owner mark failed: %+v %v", n, ok)
	}
	if _, ok := db.MarkNotificationRead(id, owner); !ok {
		t.Fatal("marking again must stay ok (idempotent)")
	}
	if _, ok := db.MarkNotificationRead(99999, owner); ok {
		t.Fatal("unknown id must not be ok")
	}
}

func TestMarkAllRead(t *testing.T) {
	db := mydb.NewFakeDB()
	u := newUmpire(t, db, "a@x.com", "A")
	other := newUmpire(t, db, "b@x.com", "B")
	db.AddNotification(u, "a", "", "")
	db.AddNotification(u, "b", "", "")
	db.AddNotification(other, "c", "", "")
	db.MarkAllNotificationsRead(u)
	if db.CountUnreadNotifications(u) != 0 || db.CountUnreadNotifications(other) != 1 {
		t.Fatal("mark-all affects only that user")
	}
}

func TestListAdminUserIDs(t *testing.T) {
	db := mydb.NewFakeDB()
	db.CreateUser("n@x.com", "Normal", "h", "", true, false)
	a1, _ := db.CreateUser("a1@x.com", "Admin1", "h", "", true, true)
	a2, _ := db.CreateUser("a2@x.com", "Admin2", "h", "", true, true)
	got := db.ListAdminUserIDs()
	if len(got) != 2 || !(got[0] == a1.ID && got[1] == a2.ID) {
		t.Fatalf("got %v want [%d %d]", got, a1.ID, a2.ID)
	}
}

func TestMuteScoreNotices(t *testing.T) {
	db := mydb.NewFakeDB()
	u := newUmpire(t, db, "u@x.com", "Una")
	if db.MutesScoreNotices(u) {
		t.Fatal("default is not muted")
	}
	db.SetMuteScoreNotices(u, true)
	if !db.MutesScoreNotices(u) {
		t.Fatal("muted")
	}
	db.SetMuteScoreNotices(u, false)
	if db.MutesScoreNotices(u) {
		t.Fatal("unmuted")
	}
	if db.MutesScoreNotices(99999) {
		t.Fatal("unknown user is not muted")
	}
}
