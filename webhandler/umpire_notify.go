package webhandler

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/Harnish/tourneyweb/mydb"
)

// sendNotice is a test seam; the default sends through me.Email and is a no-op when mail isn't configured.
var sendNotice = func(me *Env, to, subject, msg, link string) error {
	if me.Email == nil {
		return nil
	}
	return me.Email.SendUmpireNotice(to, subject, msg, link)
}

// notifyWG lets tests wait for in-flight notifications.
var notifyWG sync.WaitGroup

// notifyUmpire emails in the background; failures are logged and never block the request.
func (me *Env) notifyUmpire(toEmail, subject, message string) {
	if toEmail == "" {
		return
	}
	notifyWG.Add(1)
	go func() {
		defer notifyWG.Done()
		if err := sendNotice(me, toEmail, subject, message, "/umpire"); err != nil {
			slog.Error("umpire notice failed", "to", toEmail, "err", err)
		}
	}()
}

// notifyUser writes an in-app notification and emails the user. The inbox row is written synchronously;
// the email goes out in the background and failures are only logged. Unknown users get nothing;
// a user without an email address still gets the inbox row.
func (me *Env) notifyUser(userID int, subject, message, link string) {
	if userID <= 0 {
		return
	}
	u, err := me.DB.GetUserByID(userID)
	if err != nil || u.ID == 0 {
		return
	}
	me.DB.AddNotification(userID, subject, message, link)
	if u.Email == "" {
		return
	}
	notifyWG.Add(1)
	go func() {
		defer notifyWG.Done()
		if err := sendNotice(me, u.Email, subject, message, link); err != nil {
			slog.Error("notification email failed", "to", u.Email, "err", err)
		}
	}()
}

// gameLine describes a game for emails: "10U · Home vs Away · Jul 10, 2026 9:00 AM · Field 1".
func gameLine(g mydb.Game) string {
	parts := []string{g.Division.Name, g.HomeTeam.Name + " vs " + g.AwayTeam.Name}
	if !g.Start.IsZero() {
		parts = append(parts, g.Start.Format("Jan 2, 2006 3:04 PM"))
	}
	if g.Location != "" {
		parts = append(parts, g.Location)
	}
	return strings.Join(parts, " · ")
}
