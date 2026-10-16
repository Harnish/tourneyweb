package webhandler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Harnish/tourneyweb/mydb"
	"github.com/julienschmidt/httprouter"
)

type noticeFx struct {
	env  *Env
	db   *mydb.FakeDB
	u    mydb.User
	sent *[]string // "to|subject|link"
	mu   *sync.Mutex
	done func()
}

func newNoticeFx(t *testing.T) *noticeFx {
	t.Helper()
	db := mydb.NewFakeDB()
	u, _ := db.CreateUser("una@x.com", "Una", "h", "", true, false)
	var mu sync.Mutex
	var sent []string
	old := sendNotice
	sendNotice = func(me *Env, to, subject, msg, link string) error {
		mu.Lock()
		sent = append(sent, to+"|"+subject+"|"+link)
		mu.Unlock()
		return nil
	}
	return &noticeFx{env: &Env{DB: db}, db: db, u: u, sent: &sent, mu: &mu, done: func() { notifyWG.Wait(); sendNotice = old }}
}

func (f *noticeFx) emails() []string {
	notifyWG.Wait()
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), *f.sent...)
}

func TestNotifyUserWritesInboxAndEmails(t *testing.T) {
	f := newNoticeFx(t)
	defer f.done()
	f.env.notifyUser(f.u.ID, "Hello", "Body", "/umpire")
	got := f.db.ListNotifications(f.u.ID, 10)
	if len(got) != 1 || got[0].Subject != "Hello" || got[0].Message != "Body" || got[0].Link != "/umpire" {
		t.Fatalf("inbox: %+v", got)
	}
	if e := f.emails(); len(e) != 1 || e[0] != "una@x.com|Hello|/umpire" {
		t.Fatalf("emails: %v", e)
	}
}

func TestNotifyUserNoEmailStillWritesInbox(t *testing.T) {
	f := newNoticeFx(t)
	defer f.done()
	noMail, _ := f.db.CreateUser("", "NoMail", "h", "", true, false)
	f.env.notifyUser(noMail.ID, "Hello", "Body", "")
	if len(f.db.ListNotifications(noMail.ID, 10)) != 1 {
		t.Fatal("in-app row must be written")
	}
	if e := f.emails(); len(e) != 0 {
		t.Fatalf("no email address, nothing sent: %v", e)
	}
}

func TestNotifyUserUnknownUserAndNilEmailService(t *testing.T) {
	db := mydb.NewFakeDB()
	u, _ := db.CreateUser("a@x.com", "A", "h", "", true, false)
	env := &Env{DB: db} // Email nil: default seam is a no-op
	env.notifyUser(99999, "s", "m", "")
	env.notifyUser(0, "s", "m", "")
	env.notifyUser(u.ID, "s", "m", "")
	notifyWG.Wait()
	if len(db.ListNotifications(u.ID, 10)) != 1 {
		t.Fatal("known user still gets the in-app row")
	}
}

func inboxReq(method string, user TWUser) *http.Request {
	req := form()
	req.Method = method
	return req.WithContext(ctxWithUser(req.Context(), user))
}

func TestNotificationsPageNewestFirstAndEscaped(t *testing.T) {
	f := newNoticeFx(t)
	defer f.done()
	f.db.AddNotification(f.u.ID, "older one", "", "")
	f.db.AddNotification(f.u.ID, "<b>x</b> newer", "msg", "/umpire")
	w := httptest.NewRecorder()
	f.env.Notifications(w, inboxReq("GET", TWUser{ID: f.u.ID, Name: "Una"}), nil)
	body := w.Body.String()
	if w.Code != 200 || strings.Index(body, "newer") > strings.Index(body, "older one") {
		t.Fatalf("code=%d newest first expected: %s", w.Code, body)
	}
	if strings.Contains(body, "<b>x</b>") || !strings.Contains(body, "&lt;b&gt;x&lt;/b&gt;") {
		t.Fatal("subject must be escaped")
	}
}

func openNote(f *noticeFx, user TWUser, id int) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	f.env.NotificationOpen(w, inboxReq("GET", user), httprouter.Params{{Key: "id", Value: strconv.Itoa(id)}})
	return w
}

func TestNotificationOpenMarksReadAndRedirects(t *testing.T) {
	f := newNoticeFx(t)
	defer f.done()
	id := f.db.AddNotification(f.u.ID, "s", "", "/umpire")
	w := openNote(f, TWUser{ID: f.u.ID}, id)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/umpire" {
		t.Fatalf("got %d -> %q", w.Code, w.Header().Get("Location"))
	}
	if f.db.CountUnreadNotifications(f.u.ID) != 0 {
		t.Fatal("must be marked read")
	}
}

func TestNotificationOpenOtherUser404(t *testing.T) {
	f := newNoticeFx(t)
	defer f.done()
	other, _ := f.db.CreateUser("o@x.com", "O", "h", "", true, false)
	id := f.db.AddNotification(f.u.ID, "s", "", "/umpire")
	if w := openNote(f, TWUser{ID: other.ID}, id); w.Code != http.StatusNotFound {
		t.Fatalf("got %d want 404", w.Code)
	}
	if f.db.CountUnreadNotifications(f.u.ID) != 1 {
		t.Fatal("state must not change")
	}
}

func TestNotificationOpenRefusesOffsiteLinks(t *testing.T) {
	f := newNoticeFx(t)
	defer f.done()
	for _, link := range []string{"", "https://evil.example/", "//evil.example/x", "/\\evil.example", "javascript:alert(1)"} {
		id := f.db.AddNotification(f.u.ID, "s", "", link)
		if w := openNote(f, TWUser{ID: f.u.ID}, id); w.Header().Get("Location") != "/notifications" {
			t.Errorf("link %q redirected to %q, want /notifications", link, w.Header().Get("Location"))
		}
	}
}

func TestNotificationsReadAll(t *testing.T) {
	f := newNoticeFx(t)
	defer f.done()
	f.db.AddNotification(f.u.ID, "a", "", "")
	f.db.AddNotification(f.u.ID, "b", "", "")
	w := httptest.NewRecorder()
	f.env.NotificationsReadAll(w, inboxReq("POST", TWUser{ID: f.u.ID}), nil)
	if w.Code != http.StatusSeeOther || f.db.CountUnreadNotifications(f.u.ID) != 0 {
		t.Fatalf("code=%d unread=%d", w.Code, f.db.CountUnreadNotifications(f.u.ID))
	}
}

func TestNotificationsRequireLogin(t *testing.T) {
	f := newNoticeFx(t)
	defer f.done()
	for name, call := range map[string]func(http.ResponseWriter){
		"list":    func(w http.ResponseWriter) { f.env.Notifications(w, inboxReq("GET", TWUser{}), nil) },
		"open":    func(w http.ResponseWriter) { f.env.NotificationOpen(w, inboxReq("GET", TWUser{}), httprouter.Params{{Key: "id", Value: "1"}}) },
		"readall": func(w http.ResponseWriter) { f.env.NotificationsReadAll(w, inboxReq("POST", TWUser{}), nil) },
	} {
		w := httptest.NewRecorder()
		call(w)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
			t.Errorf("%s: got %d -> %q", name, w.Code, w.Header().Get("Location"))
		}
	}
}

func TestNavUnreadCountOnlyForLoggedIn(t *testing.T) {
	f := newNoticeFx(t)
	defer f.done()
	render := func(user TWUser) string {
		w := httptest.NewRecorder()
		f.env.TournamentList(w, inboxReq("GET", user), nil)
		return w.Body.String()
	}
	if anon := render(TWUser{}); strings.Contains(anon, "/notifications") {
		t.Fatal("anonymous users get no notifications link")
	}
	if in := render(TWUser{ID: f.u.ID, Name: "Una", Unread: 3}); !strings.Contains(in, "Notifications (3)") {
		t.Fatal("logged-in nav shows the unread count")
	}
	if zero := render(TWUser{ID: f.u.ID, Name: "Una"}); !strings.Contains(zero, "Notifications") || strings.Contains(zero, "Notifications (") {
		t.Fatal("no count in parentheses when zero unread")
	}
}
