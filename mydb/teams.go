package mydb

import (
	"log"
	"strconv"
)

type Team struct {
	ID          int
	Name        string
	Coach       string
	Division    Division
	Wins        int
	Losses      int
	RunsAgainst int
	RunsFor     int
}

func (me *MyDB) AddTeam(name, coach string, divisionid int) {
	query := "Insert into TEAMS (teamname, coachname, divisionid) values (?, ?, ?);"

	statement, err := me.DB.Prepare(query)
	if err != nil {
		log.Println("Error - AddTeam - Prepare ", err)
		return
	}
	_, err = statement.Exec(name, coach, divisionid)
	if err != nil {
		log.Println("Error - AddTeam writing ", err)
		_, err = statement.Exec(name, coach, divisionid)
	}
}

func (me *MyDB) DelTeam(id int) {

	me.DB.Exec("delete from TEAMS where id=" + strconv.Itoa(id) + ";")

}

func (me *MyDB) ReturnTeamsByDivisionID(id int) (teams []Team) {

	query := "Select id, teamname, coachname, divisionid from TEAMS where divisionid=" + strconv.Itoa(id) + ";"

	rows, err := me.DB.Query(query)

	if err != nil {
		log.Println("Error - ReturnTeamsByDivision - Query ", err, query)
		return
	}
	for rows.Next() {
		var temp Team
		var divisionid int
		rows.Scan(&temp.ID, &temp.Name, &temp.Coach, &divisionid)
		temp.Division = me.ReturnDivisionByID(divisionid)
		teams = append(teams, temp)

	}
	rows.Close()

	return
}

func (me *MyDB) ReturnTeamsByDivisionIDWithStats(id int) (teams []Team) {

	query := "Select id, teamname, coachname, divisionid from TEAMS where divisionid=" + strconv.Itoa(id) + ";"

	rows, err := me.DB.Query(query)

	if err != nil {
		log.Println("Error - ReturnTeamsByDivision - Query ", err, query)
		return
	}
	for rows.Next() {
		var temp Team
		var divisionid int
		rows.Scan(&temp.ID, &temp.Name, &temp.Coach, &divisionid)
		temp.Division = me.ReturnDivisionByID(divisionid)
		temp.Wins = me.TeamWins(temp.ID)
		temp.Losses = me.TeamLosses(temp.ID)
		temp.RunsAgainst = me.TeamScoredAgainst(temp.ID)
		temp.RunsFor = me.TeamScoredFor(temp.ID)
		teams = append(teams, temp)

	}
	rows.Close()

	return
}

func (me *MyDB) TeamWins(id int) int {
	query := "Select count(*) from GAMESBYTEAM where primaryteamid=" + strconv.Itoa(id) + " and teamscore > oppenentscore;"

	rows, err := me.DB.Query(query)
	if err != nil {
		log.Println("Error - TeamWins - Query ", err, query)
		return 0
	}

	for rows.Next() {

		var wins int
		rows.Scan(&wins)
		rows.Close()
		return wins

	}
	return 0
}

func (me *MyDB) TeamLosses(id int) int {
	query := "Select count(*) from GAMESBYTEAM where primaryteamid=" + strconv.Itoa(id) + " and teamscore < oppenentscore;"

	rows, err := me.DB.Query(query)
	if err != nil {
		log.Println("Error - TeamLosses - Query ", err, query)
		return 0
	}

	for rows.Next() {

		var losses int
		rows.Scan(&losses)
		rows.Close()
		return losses

	}
	return 0
}

func (me *MyDB) ReturnTeamByID(id int) Team {

	query := "Select id, teamname, coachname, divisionid from TEAMS where id=" + strconv.Itoa(id) + ";"

	rows, err := me.DB.Query(query)
	var temp Team
	if err != nil {
		log.Println("Error - ReturnTeamByID - Query ", err, query)
		return temp
	}

	for rows.Next() {

		var divisionid int
		rows.Scan(&temp.ID, &temp.Name, &temp.Coach, &divisionid)
		temp.Division = me.ReturnDivisionByID(divisionid)
		rows.Close()
		return temp

	}

	return temp
}

func (me *MyDB) TeamScoredAgainst(id int) int {
	query := "Select sum(oppenentscore) from GAMESBYTEAM where primaryteamid=" + strconv.Itoa(id) + ";"

	rows, err := me.DB.Query(query)
	if err != nil {
		log.Println("Error - TeamScoredAgainst - Query ", err, query)
		return 0
	}

	for rows.Next() {

		var score int
		rows.Scan(&score)
		rows.Close()
		return score

	}
	return 0
}

func (me *MyDB) TeamScoredFor(id int) int {
	query := "Select sum(teamscore) from GAMESBYTEAM where primaryteamid=" + strconv.Itoa(id) + ";"

	rows, err := me.DB.Query(query)
	if err != nil {
		log.Println("Error - TeamScoredFor - Query ", err, query)
		return 0
	}

	for rows.Next() {

		var score int
		rows.Scan(&score)
		rows.Close()
		return score

	}
	return 0
}
