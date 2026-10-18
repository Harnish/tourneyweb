package webhandler

import (
	"log/slog"
	"time"

	"github.com/Harnish/tourneyweb/mydb"
)

// teamAddBlocked enforces the league rule that teams cannot join once the season has started. Only site
// admins and the league's own directors may override it, and only with the explicit late_team confirmation.
func (me *Env) teamAddBlocked(t mydb.Tournament, user TWUser, lateConfirmed bool) (bool, string) {
	if !t.IsLeague() || !t.Started(time.Now()) {
		return false, ""
	}
	if lateConfirmed && (user.IsAdmin || user.IsDirectorFor(t.ID)) {
		return false, ""
	}
	return true, "This league's season has started; teams can no longer be added."
}

// canOverrideRoster reports whether the user may add a late team to a started league.
func canOverrideRoster(t mydb.Tournament, user TWUser) bool {
	return user.IsAdmin || user.IsDirectorFor(t.ID)
}

func logLateTeam(t mydb.Tournament, user TWUser, name string) {
	if t.IsLeague() && t.Started(time.Now()) {
		slog.Info("late team added to started league", "tournament", t.ID, "user", user.ID, "team", name)
	}
}
