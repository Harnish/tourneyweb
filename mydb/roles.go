package mydb

import (
	"log"
	"time"
)

type TournamentRole struct {
	ID           int
	UserID       int
	TournamentID int
	Role         string
	TeamID       int
}

type Invitation struct {
	ID           int
	Email        string
	TournamentID int
	Role         string
	TeamID       int
	Token        string
	ExpiresAt    time.Time
}

func (me *MyDB) GetRolesForUser(userID int) []TournamentRole {
	rows, err := me.DB.Query(
		`SELECT id, user_id, tournament_id, role, COALESCE(team_id,0)
		 FROM tournament_roles WHERE user_id=$1`, userID)
	if err != nil {
		log.Println("GetRolesForUser:", err)
		return nil
	}
	defer rows.Close()
	var out []TournamentRole
	for rows.Next() {
		var r TournamentRole
		rows.Scan(&r.ID, &r.UserID, &r.TournamentID, &r.Role, &r.TeamID)
		out = append(out, r)
	}
	return out
}

func (me *MyDB) GetRolesForTournament(tournamentID int) []TournamentRole {
	rows, err := me.DB.Query(
		`SELECT id, user_id, tournament_id, role, COALESCE(team_id,0)
		 FROM tournament_roles WHERE tournament_id=$1`, tournamentID)
	if err != nil {
		log.Println("GetRolesForTournament:", err)
		return nil
	}
	defer rows.Close()
	var out []TournamentRole
	for rows.Next() {
		var r TournamentRole
		rows.Scan(&r.ID, &r.UserID, &r.TournamentID, &r.Role, &r.TeamID)
		out = append(out, r)
	}
	return out
}

func (me *MyDB) AssignRole(userID, tournamentID int, role string, teamID int) error {
	var teamArg interface{}
	if teamID > 0 {
		teamArg = teamID
	}
	_, err := me.DB.Exec(
		`INSERT INTO tournament_roles (user_id, tournament_id, role, team_id)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (user_id, tournament_id) DO UPDATE SET role=$3, team_id=$4`,
		userID, tournamentID, role, teamArg)
	if err != nil {
		log.Println("AssignRole:", err)
	}
	return err
}

func (me *MyDB) RemoveRole(userID, tournamentID int) {
	_, err := me.DB.Exec(
		`DELETE FROM tournament_roles WHERE user_id=$1 AND tournament_id=$2`,
		userID, tournamentID)
	if err != nil {
		log.Println("RemoveRole:", err)
	}
}

func (me *MyDB) CreateInvitation(email string, tournamentID int, role string, teamID int, token string, expires time.Time) error {
	var teamArg interface{}
	if teamID > 0 {
		teamArg = teamID
	}
	_, err := me.DB.Exec(
		`INSERT INTO invitations (email, tournament_id, role, team_id, token, expires_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		email, tournamentID, role, teamArg, token, expires)
	if err != nil {
		log.Println("CreateInvitation:", err)
	}
	return err
}

func (me *MyDB) GetInvitationByEmail(email string) (Invitation, bool) {
	var inv Invitation
	err := me.DB.QueryRow(
		`SELECT id, email, tournament_id, role, COALESCE(team_id,0), token, expires_at
		 FROM invitations WHERE email=$1 AND expires_at > NOW()
		 ORDER BY created_at DESC LIMIT 1`, email).
		Scan(&inv.ID, &inv.Email, &inv.TournamentID, &inv.Role, &inv.TeamID, &inv.Token, &inv.ExpiresAt)
	if err != nil {
		return Invitation{}, false
	}
	return inv, true
}

func (me *MyDB) GetInvitationByToken(token string) (Invitation, bool) {
	var inv Invitation
	err := me.DB.QueryRow(
		`SELECT id, email, tournament_id, role, COALESCE(team_id,0), token, expires_at
		 FROM invitations WHERE token=$1 AND expires_at > NOW()`, token).
		Scan(&inv.ID, &inv.Email, &inv.TournamentID, &inv.Role, &inv.TeamID, &inv.Token, &inv.ExpiresAt)
	if err != nil {
		return Invitation{}, false
	}
	return inv, true
}

func (me *MyDB) DeleteInvitation(id int) {
	_, err := me.DB.Exec(`DELETE FROM invitations WHERE id=$1`, id)
	if err != nil {
		log.Println("DeleteInvitation:", err)
	}
}
