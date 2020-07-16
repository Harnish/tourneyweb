package mydb

import (
	"log"
	"strconv"
)

type Game struct {
	ID        int
	Division  Division
	HomeTeam  Team
	AwayTeam  Team
	Location  string
	Start     string
	Umpire    string
	AwayScore int
	HomeScore int
	Scored    bool
}

func (me *MyDB) AddGame(divisionid, hometeamid, awayteamid int, location, dt, umpire string) {
	query := "INSERT INTO GAMES (divisionid, hometeamid, awayteamid, location, starttime, PrimaryUmpire) values (?, ?, ?, ?, ?, ?);"

	statement, err := me.DB.Prepare(query)
	if err != nil {
		log.Println("Error - AddGame - Prepare ", err)
		return
	}
	_, err = statement.Exec(divisionid, hometeamid, awayteamid, location, dt, umpire)
	if err != nil {
		log.Println("Error - AddGame writing ", err)
		_, err = statement.Exec(divisionid, hometeamid, awayteamid, location, dt, umpire)
	}
}

func (me *MyDB) AllGamesByDivision(divisionid int) (games []Game) {
	query := "SELECT id, hometeamid, awayteamid, location, starttime, primaryumpire from GAMES where divisionid=" + strconv.Itoa(divisionid) + ";"
	rows, err := me.DB.Query(query)

	if err != nil {
		log.Println("Error - AllGamesByDivision - Query ", err, query)
		return
	}
	for rows.Next() {
		var g Game
		var aid int
		var hid int
		rows.Scan(&g.ID, &hid, &aid, &g.Location, &g.Start, &g.Umpire)
		g.Division = me.ReturnDivisionByID(divisionid)
		g.HomeTeam = me.ReturnTeamByID(hid)
		g.AwayTeam = me.ReturnTeamByID(aid)
		g.HomeScore = me.HomeScore(g.ID)
		g.AwayScore = me.AwayScore(g.ID)
		games = append(games, g)

	}
	rows.Close()

	return
}

func (me *MyDB) AllGamesByTeam(teamid int) (games []Game) {
	query := "SELECT id, divisionid, hometeamid, awayteamid, location, starttime, primaryumpire from GAMES where hometeamid=" + strconv.Itoa(teamid) + " or awayteamid=" + strconv.Itoa(teamid) + ";"
	rows, err := me.DB.Query(query)

	if err != nil {
		log.Println("Error - AllGamesByTeam - Query ", err, query)
		return
	}
	for rows.Next() {
		var g Game
		var aid int
		var hid int
		var did int
		rows.Scan(&g.ID, &did, &hid, &aid, &g.Location, &g.Start, &g.Umpire)
		g.Division = me.ReturnDivisionByID(did)
		g.HomeTeam = me.ReturnTeamByID(hid)
		g.AwayTeam = me.ReturnTeamByID(aid)
		g.HomeScore = me.HomeScore(g.ID)
		g.AwayScore = me.AwayScore(g.ID)
		games = append(games, g)

	}
	rows.Close()

	return
}
func (me *MyDB) ReturnGameByID(gameid int) Game {
	query := "SELECT id, divisionid, hometeamid, awayteamid, location, starttime, primaryumpire from GAMES where id=" + strconv.Itoa(gameid) + ";"
	rows, err := me.DB.Query(query)
	var g Game
	if err != nil {
		log.Println("Error - ReturnGameByID - Query ", err, query)
		return g
	}

	for rows.Next() {

		var aid int
		var hid int
		var did int
		rows.Scan(&g.ID, &did, &hid, &aid, &g.Location, &g.Start, &g.Umpire)
		g.Division = me.ReturnDivisionByID(did)
		g.HomeTeam = me.ReturnTeamByID(hid)
		g.AwayTeam = me.ReturnTeamByID(aid)
		g.HomeScore = me.HomeScore(g.ID)
		g.AwayScore = me.AwayScore(g.ID)
		rows.Close()
		return g
	}

	return g
}

func (me *MyDB) DelGame(id int) {

	me.DB.Exec("delete from GAMES where id=" + strconv.Itoa(id) + ";")

}

func (me *MyDB) AllGames() (games []Game) {
	query := "SELECT id, divisionid, hometeamid, awayteamid, location, starttime, primaryumpire from GAMES;"
	rows, err := me.DB.Query(query)

	if err != nil {
		log.Println("Error - AllGames - Query ", err, query)
		return
	}
	for rows.Next() {
		var g Game
		var aid int
		var hid int
		var did int
		rows.Scan(&g.ID, &did, &hid, &aid, &g.Location, &g.Start, &g.Umpire)
		g.Division = me.ReturnDivisionByID(did)
		g.HomeTeam = me.ReturnTeamByID(hid)
		g.AwayTeam = me.ReturnTeamByID(aid)
		g.HomeScore = me.HomeScore(g.ID)
		g.AwayScore = me.AwayScore(g.ID)
		games = append(games, g)

	}
	rows.Close()

	return
}

func (me *MyDB) ScoreGame(gid, hscore, ascore int) {
	query := "update GAMES set hometeamscore=?, awayteamscore=? where id=" + strconv.Itoa(gid) + ";"
	statement, err := me.DB.Prepare(query)
	if err != nil {
		log.Println("Error - ScoreGame - Prepare ", err)
		return
	}
	_, err = statement.Exec(hscore, ascore)
	if err != nil {
		log.Println("Error - ScoreGame writing ", err)
		_, err = statement.Exec(hscore, ascore)
	}
	// FIXME need to apply the games to each team.
	game := me.ReturnGameByID(gid)
	//Delete any previous game score
	me.DeleteTeamScore(game.ID)
	//Score for home team:
	me.AddTeamScore(game.Division.ID, game.HomeTeam.ID, game.AwayTeam.ID, game.ID, hscore, ascore)
	//Score for away team:
	me.AddTeamScore(game.Division.ID, game.AwayTeam.ID, game.HomeTeam.ID, game.ID, ascore, hscore)

}

func (me *MyDB) DeleteTeamScore(gameid int) {
	me.DB.Exec("delete from GAMESBYTEAM where gameid=" + strconv.Itoa(gameid) + ";")
}

// Returns 2 bools, first one is if they played second is if they won.
func (me *MyDB) DidTeamABeatTeamB(teamaid, teambid int) (bool, bool) {
	query := "select teamscore, oppenentscore from GAMESBYTEAM where  primaryteamid=" + strconv.Itoa(teamaid) + " and oppenentid=" + strconv.Itoa(teambid) + ";"
	rows, err := me.DB.Query(query)

	if err != nil {
		log.Println("Error - DidTeamABeatTeamB - Query ", err, query)
		return false, false
	}
	for rows.Next() {
		var teamascore, teambscore int
		rows.Scan(&teamascore, &teambscore)
		rows.Close()
		if teamascore > teambscore {
			if me.debug {
				log.Println(teamaid, teambid, " played ", teamaid, " won ")
			}
			return true, true

		}
		if me.debug {
			log.Println(teamaid, teambid, " played ", teamaid, " lost ")
		}
		return true, false
	}
	if me.debug {
		log.Println(teamaid, teambid, " didn't play")
	}
	return false, false
}

func (me *MyDB) GamesPlayedByTeam(id int) int {
	query := "select count(*) from GAMESBYTEAM where  primaryteamid=" + strconv.Itoa(id) + ";"
	rows, err := me.DB.Query(query)

	if err != nil {
		log.Println("Error - GamesPlayedByTeam - Query ", err, query)
		return -1
	}
	out := 0
	for rows.Next() {
		rows.Scan(&out)
	}
	rows.Close()
	return out
}

func (me *MyDB) IsGameScored(id int) bool {
	query := "select count(*) from GAMESBYTEAM where gameid=" + strconv.Itoa(id) + ";"
	rows, err := me.DB.Query(query)

	if err != nil {
		log.Println("Error - GamesPlayedByTeam - Query ", err, query)
		return false
	}
	out := 0
	for rows.Next() {
		rows.Scan(&out)
	}
	rows.Close()
	if out == 2 {
		return true
	}
	return false
}
