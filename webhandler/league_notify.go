package webhandler

import (
	"fmt"

	"github.com/Harnish/tourneyweb/mydb"
)

// notifyLeagueReschedule tells both teams' coaches (always) and followers of either team or the level
// (unless muted) that a league game moved. It does nothing for plain tournaments or when neither the
// start instant nor the field changed. The editor is skipped and nobody is told twice.
func (me *Env) notifyLeagueReschedule(old, updated mydb.Game, editorID int) {
	if old.Start.Equal(updated.Start) && old.Location == updated.Location {
		return
	}
	if !me.DB.ReturnTournamentByID(updated.TournamentID).IsLeague() {
		return
	}
	msg := "Was: " + gameLine(old) + ". Now: " + gameLine(updated) + "."
	link := fmt.Sprintf("/tournaments/%d/games", updated.TournamentID)
	told := map[int]bool{editorID: true}
	for _, role := range me.DB.GetRolesForTournament(updated.TournamentID) {
		if role.Role == "coach" && (role.TeamID == updated.HomeTeam.ID || role.TeamID == updated.AwayTeam.ID) && !told[role.UserID] {
			told[role.UserID] = true
			me.notifyUser(role.UserID, "Game rescheduled", msg, link)
		}
	}
	for id, f := range me.DB.GameFollowers(updated) {
		if told[id] || me.DB.MutesFollowAlerts(id) {
			continue
		}
		told[id] = true
		m := msg
		if u := me.unfollowLink(id, f); u != "" {
			m += "\nStop these alerts: " + u
		}
		me.notifyUser(id, "Game rescheduled", m, link)
	}
}
