package webhandler

import (
	"errors"
	"fmt"

	"github.com/Harnish/tourneyweb/mydb"
)

var (
	ErrScoreLocked = errors.New("score is already confirmed or final")
	ErrNotYourGame = errors.New("not your game")
	ErrBadScore    = errors.New("scores must be 0-40")
	ErrBracketTie  = errors.New("a bracket game cannot end in a tie")
)

const (
	reporterHome = "coach_home"
	reporterAway = "coach_away"
)

func validScore(h, a int) bool { return h >= 0 && h <= 40 && a >= 0 && a <= 40 }

// coachReporter says which team a coach reports for in this game: a coach role on the game's home or away
// team, in the game's own tournament.
func coachReporter(user TWUser, g mydb.Game) (string, bool) {
	for _, r := range user.Roles {
		if r.Role != "coach" || r.TournamentID != g.TournamentID {
			continue
		}
		if r.TeamID == g.HomeTeam.ID {
			return reporterHome, true
		}
		if r.TeamID == g.AwayTeam.ID {
			return reporterAway, true
		}
	}
	return "", false
}

func (me *Env) isBracketGame(gameID int) bool { return me.DB.GetBracketGameByGameID(gameID).ID != 0 }

// advanceBracketFor moves the winner of a (non-tied) game forward; AdvanceBracket ignores a repeat.
func (me *Env) advanceBracketFor(g mydb.Game) {
	if g.HomeScore == g.AwayScore {
		return
	}
	winner := g.HomeTeam.ID
	if g.AwayScore > g.HomeScore {
		winner = g.AwayTeam.ID
	}
	me.AdvanceBracket(g.ID, winner)
}

// submitCoachScore records a coach team's report and drives the game's score status:
// one report → reported, two equal → confirmed, two different → disputed. Standings follow the
// first reporter's current value. Confirmed and final games are locked for coaches.
func (me *Env) submitCoachScore(g mydb.Game, reporter string, user TWUser, home, away int) error {
	if reporter != reporterHome && reporter != reporterAway {
		return ErrNotYourGame
	}
	cur := me.DB.ReturnGameByID(g.ID)
	if cur.ID == 0 || cur.TournamentID != g.TournamentID {
		return ErrNotYourGame
	}
	if cur.ScoreStatus == "confirmed" || cur.ScoreStatus == "final" {
		return ErrScoreLocked
	}
	if !validScore(home, away) {
		return ErrBadScore
	}
	if home == away && me.isBracketGame(g.ID) {
		return ErrBracketTie
	}
	hadReports := len(me.DB.ScoreReportsForGame(g.ID)) > 0
	me.DB.SaveScoreReport(g.ID, reporter, user.ID, home, away)
	reports := me.DB.ScoreReportsForGame(g.ID)
	first := reports[0]

	changed := !cur.Scored || cur.HomeScore != first.HomeScore || cur.AwayScore != first.AwayScore
	if changed {
		me.DB.ScoreGame(g.ID, first.HomeScore, first.AwayScore)
	}
	status := "reported"
	if len(reports) == 2 {
		status = "disputed"
		if reports[0].HomeScore == reports[1].HomeScore && reports[0].AwayScore == reports[1].AwayScore {
			status = "confirmed"
		}
	}
	me.DB.SetGameScoreStatus(g.ID, status)
	updated := me.DB.ReturnGameByID(g.ID)

	if status == "confirmed" {
		me.advanceBracketFor(updated)
	}
	if !hadReports {
		me.notifyOtherTeamCoaches(updated, reporter, user.ID)
	}
	if status == "disputed" && cur.ScoreStatus != "disputed" {
		me.notifyDisputed(updated)
	}
	if changed {
		me.notifyGameScored(updated, cur.Scored, user.ID)
	}
	return nil
}

// submitUmpireScore lets a registered umpire assigned to the game settle it. Allowed while the game is
// unscored, reported or disputed; the result is final.
func (me *Env) submitUmpireScore(g mydb.Game, user TWUser, home, away int) error {
	cur := me.DB.ReturnGameByID(g.ID)
	if cur.ID == 0 || cur.TournamentID != g.TournamentID || !me.umpireAssigned(g.ID, user.ID) {
		return ErrNotYourGame
	}
	if cur.ScoreStatus == "confirmed" || cur.ScoreStatus == "final" {
		return ErrScoreLocked
	}
	if !validScore(home, away) {
		return ErrBadScore
	}
	if home == away && me.isBracketGame(g.ID) {
		return ErrBracketTie
	}
	me.DB.ScoreGame(g.ID, home, away)
	me.DB.SetGameScoreStatus(g.ID, "final")
	updated := me.DB.ReturnGameByID(g.ID)
	me.advanceBracketFor(updated)
	me.notifyGameScored(updated, cur.Scored, user.ID)
	return nil
}

// markFinal is the staff/director path: their score is final and clears any dispute.
func (me *Env) markFinal(g mydb.Game) { me.DB.SetGameScoreStatus(g.ID, "final") }

func (me *Env) umpireAssigned(gameID, userID int) bool {
	if userID <= 0 {
		return false
	}
	for _, s := range me.DB.GameUmpires(gameID) {
		if s.UserID == userID && s.Status != "declined" {
			return true
		}
	}
	return false
}

// notifyOtherTeamCoaches tells the opposing team's coaches a score was reported.
func (me *Env) notifyOtherTeamCoaches(g mydb.Game, reporter string, reporterID int) {
	other := g.AwayTeam.ID
	if reporter == reporterAway {
		other = g.HomeTeam.ID
	}
	msg := fmt.Sprintf("%s. Reported score: %s %d, %s %d. Please confirm or correct it.", gameLine(g),
		g.HomeTeam.Name, g.HomeScore, g.AwayTeam.Name, g.AwayScore)
	link := fmt.Sprintf("/tournaments/%d/coach/games/%d/score", g.TournamentID, g.ID)
	seen := map[int]bool{reporterID: true}
	for _, role := range me.DB.GetRolesForTournament(g.TournamentID) {
		if role.Role == "coach" && role.TeamID == other && !seen[role.UserID] {
			seen[role.UserID] = true
			me.notifyUser(role.UserID, "Score reported — please confirm", msg, link)
		}
	}
}

// disputeRecipients: registered umpires on the game's non-declined slots; else the directors (else admins).
func (me *Env) disputeRecipients(g mydb.Game) []int {
	seen := map[int]bool{}
	var ids []int
	for _, s := range me.DB.GameUmpires(g.ID) {
		if s.UserID > 0 && s.Status != "declined" && !seen[s.UserID] {
			seen[s.UserID] = true
			ids = append(ids, s.UserID)
		}
	}
	if len(ids) > 0 {
		return ids
	}
	return me.directorRecipients(g.TournamentID)
}

func (me *Env) notifyDisputed(g mydb.Game) {
	msg := fmt.Sprintf("The coaches reported different scores for: %s. Please enter the final score.", gameLine(g))
	link := fmt.Sprintf("/tournaments/%d/umpire/games/%d/score", g.TournamentID, g.ID)
	for _, id := range me.disputeRecipients(g) {
		me.notifyUser(id, "Score disputed", msg, link)
	}
}
