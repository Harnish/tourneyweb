package webhandler

import (
	"fmt"

	"github.com/Harnish/tourneyweb/mydb"
)

// officialResult is a game's result as followers see it: only a confirmed or final score counts.
func officialResult(g mydb.Game) (official bool, home, away int) {
	return g.Scored && (g.ScoreStatus == "confirmed" || g.ScoreStatus == "final"), g.HomeScore, g.AwayScore
}

// notifyFollowers alerts followers of the game's division or teams when its official result changes
// between before and after: "Final score" the first time, "Game score corrected" afterwards.
// The user whose action produced the result and anyone who muted follower alerts are skipped.
func (me *Env) notifyFollowers(before, after mydb.Game, scorerID int) {
	ok, h, a := officialResult(after)
	if !ok {
		return
	}
	wasOK, bh, ba := officialResult(before)
	if wasOK && bh == h && ba == a {
		return
	}
	subject := "Final score"
	if wasOK {
		subject = "Game score corrected"
	}
	msg := fmt.Sprintf("%s. Final: %s %d, %s %d.", gameLine(after), after.HomeTeam.Name, h, after.AwayTeam.Name, a)
	link := fmt.Sprintf("/tournaments/%d/games", after.TournamentID)
	for id, f := range me.DB.GameFollowers(after) {
		if id == scorerID || me.DB.MutesFollowAlerts(id) {
			continue
		}
		m := msg
		if u := me.unfollowLink(id, f); u != "" {
			m += "\nStop these alerts: " + u
		}
		me.notifyUser(id, subject, m, link)
	}
}

// unfollowLink is the emailed one-click unfollow URL; empty when no signing secret is configured.
func (me *Env) unfollowLink(userID int, f mydb.Follow) string {
	t := me.followToken(userID, f.Kind, f.TargetID)
	if t == "" {
		return ""
	}
	return me.BaseURL + "/unfollow?t=" + t
}
