package webhandler

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/rivo/sessions"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

type Env struct {
	DB            *mydb.MyDB
	AdminPW       string
	DisableDelete bool
}

func New(db *mydb.MyDB, adminpw string, dd bool) *Env {
	me := &Env{
		DB:            db,
		AdminPW:       adminpw,
		DisableDelete: dd,
	}
	return me
}

func (me *Env) PrintIndex(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	Divs := me.DB.ReturnDivisions()
	header := ReturnHeader(false)
	var out string
	out = out + "<h1>This is being updated for future use.  Information may display differently as we progress development.</h1>\n"
	out = out + "Click the division for division standings, Click team for Upcoming games and results."
	for _, div := range Divs {
		out = out + "<h2><a href=\"/divisions/" + strconv.Itoa(div.ID) + "\">" + div.Name + "</a></h2>\n<ul>"
		teams := me.DB.ReturnTeamsByDivisionID(div.ID)
		for _, team := range teams {
			out = out + "<li><a href=\"/teams/" + strconv.Itoa(team.ID) + "\">" + team.Name + "</a> " + team.Coach + "</li>\n"
		}
		out = out + "\n</ul>"
	}

	var footer string
	full := header + out + footer
	w.Write([]byte(full))
}

func (me *Env) AdminIndex(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	out3 := ReturnHeader(true) + me.ReturnAdminIndex() + ReturnFooter()
	w.Write([]byte(out3))
}

func (me *Env) ReturnAdminIndex() string {
	out2 := `<ul>
	<li><a href="/admin/adddivisionform">Add Division</a></li>
	<li><a href="/admin/addteamform">Add Team</a></li>
	<li><a href="/admin/creategameform">Create Game</a></li>
	<li><a href="/admin/games">Record Score for a game</a></li>
	</ul>
	`
	if me.DisableDelete {
		out2 = out2 + "Deletes have been disabled during the tournament.<br>\n"
	}
	return out2
}

func ReturnHeader(admin bool) string {
	output := `<!doctype html>
	<html lang="en"> 
	<head>
	<title>Battle at the Dawg Pound</title>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
	<link rel="stylesheet" href="/style.css">
	<!-- CSS only -->
	<link rel="stylesheet" href="https://stackpath.bootstrapcdn.com/bootstrap/4.5.0/css/bootstrap.min.css" integrity="sha384-9aIt2nRpC12Uk9gS9baDl411NQApFmC26EwAOH8WgZl5MYYxFfc+NcPb1dKGj7Sk" crossorigin="anonymous">

	<!-- JS, Popper.js, and jQuery -->
	<script src="https://code.jquery.com/jquery-3.5.1.slim.min.js" integrity="sha384-DfXdz2htPH0lsSSs5nCTpuj/zy4C+OGpamoFVy38MVBnE+IbbVYUew+OrCXaRkfj" crossorigin="anonymous"></script>
	<script src="https://cdn.jsdelivr.net/npm/popper.js@1.16.0/dist/umd/popper.min.js" integrity="sha384-Q6E9RHvbIyZFJoft+2mJbHaEWldlvI9IOYy5n3zV9zzTtmI3UksdQRVvoxMfooAo" crossorigin="anonymous"></script>
	<script src="https://stackpath.bootstrapcdn.com/bootstrap/4.5.0/js/bootstrap.min.js" integrity="sha384-OgVRvuATP1z7JjHLkuOU7Xw704+h835Lr+6QL9UvYjZE3Ipu6Tp75j7Bh/kR0JKI" crossorigin="anonymous"></script>
	</head>
	<body>`
	output = output + "<img src=\"/img/topimage.jpg\"> <br><a href=\"/\">Home</a> | <a href=\"/hrderbyinfo\">Skills & HR derby Info</a> "
	if admin {
		output = output + "| <a href=\"/admin\">Admin</a> | <a href=\"/admin/adddivisionform\">Divisions</a> | <a href=\"/admin/teams\">Teams</a> | <a href=\"/admin/games\">Games</a> <br>"
	} else {
		output = output + "| <a href=\"/login\">Login</a> "
	}
	output = output + "<br><hr>"
	return output
}

func (me *Env) RequestLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		userdata := me.MySession(w, r)
		forwardedip := r.Header.Get("X-Forwarded-For")
		//[02/Mar/2016:08:14:04 -0600] "GET /shop HTTP/1.1" 200 615 "-" "Googlebot-Image/1.0"
		log.Println(r.Method, r.URL.Path, r.Proto, forwardedip)
		//FIXME length and code
		//log.Println("\"", r.Method, r.URL.Path, r.Proto, "\"", "200", "0", "\"-\"", "\""+r.UserAgent()+"\"", userdata.Userid, forwardedip)
		//me.DB.LogLine(forwardedip, r.Method, r.URL.Path, userdata.Userid, 0, "")
		if strings.HasPrefix(r.URL.Path, "/admin") && userdata.ID < 1 {
			log.Println(r.RemoteAddr, r.Method, r.URL.Path, userdata.UserName, "Permission Denied ")
			PrintError(w, "notauthorized")
			return

		}

		ctx := context.WithValue(r.Context(), "uid", userdata.ID)
		h.ServeHTTP(w, r.WithContext(ctx))
		//h.ServeHTTP(w, r)
	})
}

func (me *Env) LoginForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	header := ReturnHeader(false)
	out := `<form method=post action="/login"> 
	    <table>
		<tr><td>Username</td><td><input type="text" name="username"></td></tr>
		<tr><td>Password</td><td><input type="password" name="password"></td></tr>
		<tr><td></td><td><input type="submit" name="Login" value="Login"></td></tr>
		</table>
		</form>`

	var footer string
	out2 := header + out + footer
	w.Write([]byte(out2))
}

func (me *Env) Login(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	if password == me.AdminPW {
		session, err := sessions.Start(w, r, true)
		if err != nil {
			log.Println("Session Failed to start ", err)
		}
		session.Set("userid", username)

		out3 := ReturnHeader(true) + me.ReturnAdminIndex() + ReturnFooter()
		w.Write([]byte(out3))
	} else {
		header := ReturnHeader(false)
		header = header + "Login Failed"
		w.Write([]byte(header))
	}
}

func (me *Env) CreateGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	divisionidstr := ps.ByName("divisionid")
	header := ReturnHeader(true)
	did, err := strconv.Atoi(divisionidstr)
	if err != nil {
		w.Write([]byte(header))
		w.Write([]byte("Error, Division ID invalid"))
		return
	}
	teams := me.DB.ReturnTeamsByDivisionID(did)
	teamoptions := TeamOptions(teams)
	form := `
	<form method=post action=/admin/addgame>
	<input type=hidden name=divisionid value=` + divisionidstr + `>
	<table>
		<tr><td>Home Team</td><td><select name=hometeam> ` + teamoptions + `</select></td></tr>
		<tr><td>Away Team</td><td><select name=awayteam> ` + teamoptions + `</select></td></tr>
		<tr><td>Location</td><td><input type=text name="location"></td></tr>
		<tr><td>Date/Time</td><td><input type=text name="datetime"></td></tr>
		<tr><td>Umpire</td><td><input type=text name="umpire"></td></tr>
		<tr><td></td><td><input type=submit name=submit></td></tr>
	</table>
	</form>`

	w.Write([]byte(header))
	w.Write([]byte(form))
	listofgames := me.GamesByDivisionList(did, true, true)
	w.Write([]byte(listofgames))
	w.Write([]byte(ReturnFooter()))
}

func (me *Env) CreateGameSubmit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	divisionidstr := r.FormValue("divisionid")
	hometeamidstr := r.FormValue("hometeam")
	awayteamidstr := r.FormValue("awayteam")
	location := r.FormValue("location")
	dt := r.FormValue("datetime")
	umpire := r.FormValue("umpire")
	did, err := strconv.Atoi(divisionidstr)
	if err != nil {
		PrintError(w, "Bad DivisionID")
		log.Println("Bad divisionid")
		return
	}
	hid, err := strconv.Atoi(hometeamidstr)
	if err != nil {
		PrintError(w, "Bad Hometeam ID")
		log.Println("Bad hometeamid")
		return
	}
	aid, err := strconv.Atoi(awayteamidstr)
	if err != nil {
		PrintError(w, "Bad Away team ID")
		log.Println("Bad away team ID")
		return
	}
	if aid == hid {
		PrintError(w, "Must select a different team as an opponent.")
		log.Println("same ID for both teams.")
		return
	}
	me.DB.AddGame(did, hid, aid, location, dt, umpire)
	header := ReturnHeader(true)
	teams := me.DB.ReturnTeamsByDivisionID(did)
	teamoptions := TeamOptions(teams)
	form := `
	<form method=post action=/admin/addgame>
	<input type=hidden name=divisionid value=` + divisionidstr + `>
	<table>
		<tr><td>Home Team</td><td><select name=hometeam> ` + teamoptions + `</select></td></tr>
		<tr><td>Away Team</td><td><select name=awayteam> ` + teamoptions + `</select></td></tr>
		<tr><td>Location</td><td><input type=text name="location"></td></tr>
		<tr><td>Date/Time</td><td><input type=text name="datetime"></td></tr>
		<tr><td>Umpire</td><td><input type=text name="umpire"></td></tr>
		<tr><td></td><td><input type=submit name=submit></td></tr>
	</table>
	</form>`

	w.Write([]byte(header))
	w.Write([]byte(form))
	listofgames := me.GamesByDivisionList(did, true, true)
	w.Write([]byte(listofgames))
	w.Write([]byte(ReturnFooter()))

}

func (me *Env) GamesByDivisionList(did int, withadmin, withscores bool) string {
	games := me.DB.AllGamesByDivision(did)
	listofgames := `<table><tr><th>Home Team</th><th>Away Team</th><th>Location</th><th>Start time</th><th>Umpire</th>`
	if withadmin {
		listofgames = listofgames + "<th>Score Game</th>"
	}
	if !me.DisableDelete {
		listofgames = listofgames + "<th>Delete Game</th>"
	}
	if withscores {
		listofgames = listofgames + "<th>Home Team Score</th><th>Away Team Score</th>"
	}

	listofgames = listofgames + `<tr>`
	for _, game := range games {
		listofgames = listofgames + "<tr><td>" + game.HomeTeam.Name + " " + game.HomeTeam.Coach + "</td><td>" + game.AwayTeam.Name + " " + game.AwayTeam.Coach + "</td><td>" + game.Location + "</td><td>" + game.Start + "</td><td>" + game.Umpire + "</td>"
		if withadmin {
			listofgames = listofgames + "<td><a href=/admin/scoregame/" + strconv.Itoa(game.ID) + ">Score Game</a></td>"
			if !me.DisableDelete {
				listofgames = listofgames + "<td><a href=/admin/delgame/" + strconv.Itoa(game.ID) + ">Delete Game</a></td>"
			}
		}
		if withscores {
			listofgames = listofgames + "<td>" + strconv.Itoa(game.HomeScore) + "</td><td>" + strconv.Itoa(game.AwayScore) + "</td>"
		}
		listofgames = listofgames + "</tr>"
	}
	listofgames = listofgames + "</table>"
	return listofgames
}

func (me *Env) PrintDivision(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	header := ReturnHeader(false)
	divstr := ps.ByName("id")
	div, err := strconv.Atoi(divstr)
	if err != nil {
		log.Println("bad ID ", divstr, err)
		w.Write([]byte("Error bad ID for division"))
		return
	}
	//out := me.ReturnTeamsByDivisionIDTable(div, false)
	out := me.ReturnTeamsByDivisionIDRankedTable(div, false)
	out = out + "<h2>Games</h2>\n" + me.GamesByDivisionList(div, false, true)
	w.Write([]byte(header))
	w.Write([]byte("<h2>Teams</h2>" + out))
	w.Write([]byte(ReturnFooter()))
}

func (me *Env) ReturnTeamsByDivisionIDTable(div int, admin bool) string {
	division := me.DB.ReturnDivisionByID(div)
	out := "<H1>" + division.Name + "</H1>"
	out = out + "<table><tr><th>Team Name</th><th>Coach</th><th>Wins</th><th>Losses</th>"
	teams := me.DB.ReturnTeamsByDivisionID(div)

	for _, team := range teams {
		out = out + "<tr><td>" + team.Name + "</td><td>" + team.Coach + "</td><td>" + strconv.Itoa(me.DB.TeamWins(team.ID)) + "</td><td>" + strconv.Itoa(me.DB.TeamLosses(team.ID)) + "</td></tr>\n"
	}
	out = out + "</table>"
	return out
}

func (me *Env) ReturnTeamsByDivisionIDRankedTable(div int, admin bool) string {
	division := me.DB.ReturnDivisionByID(div)
	out := "<H1>" + division.Name + "</H1>"
	out = out + "<table><tr><th>Rank</th><th>Team Name</th><th>Coach</th><th>Wins</th><th>Losses</th><th>Runs Against</th><th>Runs For</th><th>Games Played</th></tr>"
	teams := me.DB.ReturnTeamsByDivisionIDWithStats(div)
	teams = me.SortTeams(teams, "WinsRunsAgainstRunsEarnedHead2Head")
	for idx, team := range teams {
		out = out + "<tr><td>" + strconv.Itoa(idx+1) + "</td><td>" + team.Name + "</td><td>" + team.Coach + "</td><td>" + strconv.Itoa(team.Wins) + "</td><td>" + strconv.Itoa(team.Losses) + "</td><td>" + strconv.Itoa(team.RunsAgainst) + "</td><td>" + strconv.Itoa(team.RunsFor) + "</td><td>" + strconv.Itoa(me.DB.GamesPlayedByTeam(team.ID)) + "</td></tr>\n"
	}
	out = out + "</table>"
	return out
}

func PrintError(w http.ResponseWriter, message string) {
	header := ReturnHeader(false)
	w.Write([]byte(header))
	w.Write([]byte(message))
	w.Write([]byte(ReturnFooter()))
}

func ReturnFooter() string {
	out := `<br><hr>Powered by <a href="https://github.com/Harnish/tourneyweb">TourneyWeb</a> </body></html>`
	return out
}

func (me *Env) DelGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	gameidstr := ps.ByName("gameid")
	gid, err := strconv.Atoi(gameidstr)
	if err != nil {
		log.Println("DelGame bad ID", err, gameidstr)
		PrintError(w, "Bad Game ID")
		return
	}
	if !me.DisableDelete {
		me.DB.DelGame(gid)
	}
	//FIXME needs to return to a page.
}

func (me *Env) ScoreGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	gameidstr := ps.ByName("gameid")
	gid, err := strconv.Atoi(gameidstr)
	if err != nil {
		log.Println("DelGame bad ID", err, gameidstr)
		PrintError(w, "Bad Game ID")
		return
	}

	game := me.DB.ReturnGameByID(gid)
	header := ReturnHeader(false)
	w.Write([]byte(header))
	scoreoptions := ReturnScoresOptions(0, 40)
	output := `<form method=post action="/admin/scoregamepost">
	<input type=hidden name=gameid value=` + gameidstr + `>
	<table>
		<tr><td>Home Team</td><td>` + game.HomeTeam.Name + `</td></tr>
		<tr><td>Away Team</td><td>` + game.AwayTeam.Name + `</td></tr>
		<tr><td>Location</td><td>` + game.Location + `</td></tr>
		<tr><td>Start Time</td><td>` + game.Start + `</td></tr>
		<tr><td>Umpire</td><td>` + game.Umpire + `</td></tr>
		<tr><td>Home Team Score</td><td><select name=homescore>` + scoreoptions + `</select></td></tr>
		<tr><td>Away Team Score</td><td><select name=awayscore>` + scoreoptions + `</select></td></tr>
		<tr><td></td><td><input type=submit name="Save" value="Save"></td></tr>
	</table>
	</form>`
	w.Write([]byte(output))
	w.Write([]byte(ReturnFooter()))

}

func (me *Env) Games(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	header := ReturnHeader(false)
	w.Write([]byte(header))
	out := me.ReturnAllGamesInTable(false)
	w.Write([]byte(out))
	w.Write([]byte(ReturnFooter()))

}

func (me *Env) AdminGames(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	admin := true
	header := ReturnHeader(admin)
	w.Write([]byte(header))
	out := me.ReturnAllGamesInTable(admin)
	w.Write([]byte(out))
	w.Write([]byte(ReturnFooter()))
}

func (me *Env) ReturnAllGamesInTable(admin bool) string {
	games := me.DB.AllGames()
	var out string
	out = `<table border=1 cellpadding=1 cellspacing=0>
	<tr><th>Home Team</th><th>Home Team Score</th><th>Away Team</th><th>Away Team Score</th><th>Location</th><th>Start Time</th><th>Umpire</th>`
	if admin {
		out = out + `<th>Score Game</th>`
	}
	out = out + `</tr>`
	for _, game := range games {
		out = out + "<tr><td>" + game.HomeTeam.Name + " - " + game.HomeTeam.Coach + "</td><td>" + strconv.Itoa(game.HomeScore) + "</td><td>" + game.AwayTeam.Name + " - " + game.AwayTeam.Coach + "</td><td>" + strconv.Itoa(game.AwayScore) + "</td><td>" + game.Location + "</td><td>" + game.Start + "</td><td>" + game.Umpire + "</td>"
		if admin {
			out = out + "<td><form action=/admin/scoregame/" + strconv.Itoa(game.ID) + "><input type=submit value=\"Score Game\" name=\"Score Game\"></form></b></td>"
		}
		out = out + "</tr>"
	}
	out = out + "</table>"
	return out
}

func ReturnScoresOptions(min, max int) string {
	var out string
	for i := min; i <= max; i++ {
		istr := strconv.Itoa(i)
		out = out + "<option value=" + istr + ">" + istr + "</option>"
	}
	return out

}

func (me *Env) RecordScore(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	gameidstr := r.FormValue("gameid")
	homescorestr := r.FormValue("homescore")
	awayscorestr := r.FormValue("awayscore")
	gid, err := strconv.Atoi(gameidstr)
	if err != nil {
		PrintError(w, "bad game ID")
		return
	}
	hscore, err := strconv.Atoi(homescorestr)
	if err != nil {
		PrintError(w, "Bad Home score")
		return
	}
	ascore, err := strconv.Atoi(awayscorestr)
	if err != nil {
		PrintError(w, "Bad Away Score")
		return
	}
	me.DB.ScoreGame(gid, hscore, ascore)

	header := ReturnHeader(true)
	w.Write([]byte(header))
	out := me.ReturnAllGamesInTable(true)
	w.Write([]byte(out))
	w.Write([]byte(ReturnFooter()))
}

func (me *Env) PrintHRDerby(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	out := `
	<h1>Home Run Derby and Skills Competition</h1>
We will be having a Home Run Derby and Skills Competition at the tournament this Saturday at the Wayland fields, Dorr fields, and Otsego fields after games are completed for the day. Home Run Derby and Roadrunner are $5 to enter, and Around the Horn is $10 to enter a 5 player team. Your player/team may sign-up before this Saturday via Venmo at <b><a href="https://venmo.com/code?user_id=2716969862168576351"> https://venmo.com/code?user_id=2716969862168576351</a></b><br><br>
OR you may sign-up  the day of at the information table by check made out to Wayland Union Schools, or with cash at the information table at your field location. 
Sign-up before Saturday at this link: <b><a href="https://www.signupgenius.com/go/20F0D44AAAE23A2F94-skills">https://www.signupgenius.com/go/20F0D44AAAE23A2F94-skills</a></b> <br><br>
<b>Road Runner</b> (All Players)<br>
Each player will run around the bases once while being timed. If a player misses a base they must go back and touch the base. The top 3 players will advance to the final round.
<br><br>
<b>Around the Horn</b> (5 players) <br>
Around the horn consists of 5 players from each team. A catcher, 3rd baseman, 2nd baseman, short stop and 1st baseman. The order for this competition will be: C, 3rd, 2nd, SS, 1st, SS, 2nd, 3rd, C. The clock starts when the catcher releases the ball on his throw to third and will stop when the catcher catches the throw back to him from 3rd. Top 3 teams move on to final round.
<br><br>
<b>Home Run Derby</b> (All Players)<br>
Each player will have 5 outs. Any swing that results from something other than a home run out the park is an out. Top three players advance to the final round. <br>
NOTE: Participants must provide their own pitchers please. <br>
`

	w.Write([]byte(ReturnHeader(false)))
	w.Write([]byte(out))
	w.Write([]byte(ReturnFooter()))
}

type TWUser struct {
	ID       int
	UserName string
}

// MySession handles the session information
func (me *Env) MySession(w http.ResponseWriter, r *http.Request) TWUser {
	var user TWUser
	user.ID = -1
	session, err := sessions.Start(w, r, false)
	if err != nil || session == nil {
		//log.Println("Session doesn't exist yet")
		return user
	}
	userid := session.Get("userid", nil)
	if err != nil {
		log.Println("Failed to acccess session ", err)
		//      me.LoginForm(w, r, ps)
		return user
	}
	if session == nil {
		//      me.LoginForm(w, r, ps
		return user
	}
	//log.Println(userid)

	user.ID = 1
	user.UserName = userid.(string)
	return user
}
