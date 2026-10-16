package webhandler

import (
	"log/slog"
	"strings"
	"sync"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

// sendNotice is a test seam; the default sends through me.Email and is a no-op when mail isn't configured.
var sendNotice = func(me *Env, to, subject, msg string) error {
	if me.Email == nil {
		return nil
	}
	return me.Email.SendUmpireNotice(to, subject, msg)
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
		if err := sendNotice(me, toEmail, subject, message); err != nil {
			slog.Error("umpire notice failed", "to", toEmail, "err", err)
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
