package mydb

import (
	"database/sql"
	"log"
	"strconv"
	"strings"

	//Mysql driver
	_ "github.com/go-sql-driver/mysql"

	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/localdb"
)

var mysqltables = [...]string{"CREATE TABLE IF NOT EXISTS DIVISIONS (id INTEGER PRIMARY KEY AUTO_INCREMENT, DivisionName Varchar(255), CONSTRAINT divisionname_uniq UNIQUE(DivisionName));",
	"CREATE TABLE IF NOT EXISTS TEAMS (id INTEGER PRIMARY KEY AUTO_INCREMENT, divisionid INTEGER, TeamName Varchar(255), CoachName Varchar(255));",
	"CREATE TABLE IF NOT EXISTS GAMESBYTEAM (id INTEGER PRIMARY KEY AUTO_INCREMENT, divisionid INTEGER, primaryteamid INTEGER, oppenentid INTEGER, gameid INTEGER, teamscore INTEGER, oppenentscore INTEGER);",
	"CREATE TABLE IF NOT EXISTS GAMES (id INTEGER PRIMARY KEY AUTO_INCREMENT, divisionid INTEGER, hometeamid INTEGER, awayteamid INTEGER, location Varchar(255), starttime Varchar(255), primaryumpire Varchar(255), hometeamscore INTEGER, awayteamscore INTEGER);"}

type MyDB struct {
	DB *sql.DB
}

func New(path string) *MyDB {
	me := &MyDB{}
	if strings.HasPrefix(path, "mysql://") {
		newpath := strings.TrimPrefix(path, "mysql://")
		me.DB = CreateDatabase("mysql", newpath)

	} else {
		// FIXME randomly generate so that it doesn't matter if using a memory based db.
		user := "user"
		password := "password"
		log.Println("No DB defined.  Starting a local one.")
		localdb.StartDB(user, password)
		me.DB = CreateDatabase("mysql", user+":"+password+"tcp(localhost:3306)/tourneyweb")
	}
	return me
}

func CreateDatabase(dbtype, path string) *sql.DB {

	database, err := sql.Open("mysql", path)
	if err != nil {
		log.Println("Opening DB error: ", err)
	}
	for _, table := range mysqltables {
		statement, err := database.Prepare(table)
		if err != nil {
			log.Println("Writing DB error ", err, table)
		}
		statement.Exec()
	}
	return database

	return nil
}

func (me *MyDB) HomeScore(gid int) int {
	return me.TeamScore(gid, "home")
}

func (me *MyDB) AwayScore(gid int) int {
	return me.TeamScore(gid, "away")
}

func (me *MyDB) TeamScore(gid int, team string) int {
	var query string
	if team == "home" {
		query = "select hometeamscore from GAMES where id=" + strconv.Itoa(gid) + ";"
	} else {
		query = "select awayteamscore from GAMES where id=" + strconv.Itoa(gid) + ";"

	}
	rows, err := me.DB.Query(query)

	if err != nil {
		log.Println("Error - TeamScore - Query ", err, query)
		return -1
	}
	for rows.Next() {
		var temp int
		rows.Scan(&temp)
		rows.Close()
		return temp
	}
	return -1
}

func (me *MyDB) AddTeamScore(divisionid, primaryteamid, oppenentid, gameid, teamscore, oppenentscore int) {
	//divisionid INTEGER, primaryteamid INTEGER, oppenentid INTEGER, gameid INTEGER, teamscore INTEGER, oppenentscore INTEGER
	query := "INSERT INTO GAMESBYTEAM (divisionid, primaryteamid, oppenentid, gameid, teamscore, oppenentscore) values (?, ?, ?, ?, ?, ?);"
	statement, err := me.DB.Prepare(query)
	if err != nil {
		log.Println("Error - AddGame - Prepare ", err)
		return
	}
	_, err = statement.Exec(divisionid, primaryteamid, oppenentid, gameid, teamscore, oppenentscore)
	if err != nil {
		log.Println("Error - AddGame writing ", err)
		_, err = statement.Exec(divisionid, primaryteamid, oppenentid, gameid, teamscore, oppenentscore)
	}
}

//func (me *MyDB)
