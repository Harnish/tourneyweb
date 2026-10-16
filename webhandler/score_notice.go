package webhandler

import (
	"fmt"

	"github.com/Harnish/tourneyweb/mydb"
)

// notifyGameScored tells the tournament's directors (or admins if none) that a game was scored.
// The person who entered the score and anyone who turned off game-score notices are skipped.
func (me *Env) notifyGameScored(g mydb.Game, wasScored bool, scorerID int) {
	subject := "Game scored"
	if wasScored {
		subject = "Game score updated"
	}
	msg := fmt.Sprintf("%s. Score: %s %d, %s %d.", gameLine(g), g.HomeTeam.Name, g.HomeScore, g.AwayTeam.Name, g.AwayScore)
	link := fmt.Sprintf("/tournaments/%d/manage/divisions/%d/games/new", g.TournamentID, g.Division.ID)
	for _, id := range me.directorRecipients(g.TournamentID) {
		if id == scorerID || me.DB.MutesScoreNotices(id) {
			continue
		}
		me.notifyUser(id, subject, msg, link)
	}
}
