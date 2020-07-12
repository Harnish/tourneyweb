package webhandler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func (me *Env) Teams(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	teamname := r.FormValue("teamname")
	teamcoach := r.FormValue("teamcoach")
	teamdivisionstr := r.FormValue("division")
	teamdivision, err1 := strconv.Atoi(teamdivisionstr)
	if teamname != "" && err1 == nil {
		me.DB.AddTeam(teamname, teamcoach, teamdivision)

	}
	teamid := r.FormValue("teamid")
	if teamid != "" {
		log.Println("Deleting teamid  ", teamid)
		did, err := strconv.Atoi(teamid)
		if err != nil {
			log.Println("Bad ID", err)
		} else {
			if !me.DisableDelete {
				me.DB.DelTeam(did)
			}
		}
	}

	header := ReturnHeader(true)
	out2 := `<form method=post action="/admin/addteam">
	<table>
	<tr><td>Team Name</td><td><input type="text" name="teamname"></td><tr>
	<tr><td>Team Coach</td><td><input type="text" name="teamcoach"></td><tr>
	<tr><td>Division</td><td><select name="division">`
	Divs := me.DB.ReturnDivisions()
	for _, div := range Divs {
		out2 = out2 + "<option value=\"" + strconv.Itoa(div.ID) + "\">" + div.Name + "</option>"
	}
	out2 = out2 + `</select></td></tr>
	<tr><td></td><td><input type="submit" name="submit"></td></tr>
	</table>
	</form>
	`

	footer := ""

	for _, div := range Divs {
		out2 = out2 + "<h2>" + div.Name + "</h2>"
		out2 = out2 + "<table>"
		teams := me.DB.ReturnTeamsByDivisionID(div.ID)
		for _, team := range teams {
			out2 = out2 + "<tr><td>" + team.Name + "</td><td>" + team.Coach + "</td><td>"
			if !me.DisableDelete {
				out2 = out2 + "<form method=port action=\"/admin/teams\"><input type=hidden name=\"teamid\" value=\"" + strconv.Itoa(team.ID) + "\"><input type=submit name=Delete value=Delete></form>"
			}
			out2 = out2 + "</td></tr>"
		}

		out2 = out2 + "</table>"
	}

	out := header + out2 + footer
	w.Write([]byte(out))

}

func TeamOptions(teams []mydb.Team) string {
	var out string
	for _, team := range teams {
		out = out + "<option id=" + strconv.Itoa(team.ID) + " value=" + strconv.Itoa(team.ID) + ">" + team.Name + " - " + team.Coach + "</option>"
	}
	return out
}

func (me *Env) ShowTeam(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	teamidstr := ps.ByName("teamid")
	tid, err := strconv.Atoi(teamidstr)
	if err != nil {
		PrintError(w, "Bad Team ID")
		return
	}
	Team := me.DB.ReturnTeamByID(tid)
	Games := me.DB.AllGamesByTeam(tid)

	header := ReturnHeader(false)
	w.Write([]byte(header))
	out := "<br>" + Team.Name + " " + Team.Coach + " " + Team.Division.Name + " <br>\n"
	w.Write([]byte(out))
	out2 := `<table>
			 <tr><th>Home</th><th>Away</th><th>Location</th><th>Start time</th><th>Umpire</th></tr>`
	for _, game := range Games {
		out2 = out2 + "<tr><td>" + game.HomeTeam.Name + "</td><td>" + game.AwayTeam.Name + "</td><td>" + game.Location + "</td><td>" + game.Start + "</td><td>" + game.Umpire + "</td></tr>"
	}
	out = out2 + "</table>\n"
	w.Write([]byte(out2))
	w.Write([]byte(ReturnFooter()))
}
