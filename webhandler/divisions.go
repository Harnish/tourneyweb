package webhandler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) AddDivisionForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	divisionname := r.FormValue("divisionname")
	if divisionname != "" {
		me.DB.AddDivision(divisionname)
	}
	divisionid := r.FormValue("divisionid")
	if divisionid != "" {
		log.Println("Deleting ", divisionid)
		did, err := strconv.Atoi(divisionid)
		if err != nil {
			log.Println("Bad ID", err)
		} else {
			if !me.DisableDelete {
				me.DB.DelDivision(did)
			}
		}
	}
	header := ReturnHeader(true)
	out2 := `<form method=post action="/admin/adddivision">
	<table>
	<tr><td>Division Name</td><td><input type="text" name="divisionname"></td><tr>
	<tr><td></td><td><input type="submit" name="submit"></td></tr>
	</table>
	</form>
	`

	footer := ReturnFooter()
	Divs := me.DB.ReturnDivisions()

	out2 = out2 + "<table border=1 cellpadding=1 cellspacing=0>"
	for _, div := range Divs {
		out2 = out2 + "<tr><td><a href=/admin/divisions/" + strconv.Itoa(div.ID) + ">" + div.Name + "</a></td>"
		if !me.DisableDelete {
			out2 = out2 + "<td valign=top><form method=post action=\"/admin/deldivision\"> <input type=hidden name=divisionid value=\"" + strconv.Itoa(div.ID) + "\"><input type=submit name=\"delete\" value=\"delete\"></form></td>"
		}
		out2 = out2 + "<td valign=top><form action=\"/admin/creategame/" + strconv.Itoa(div.ID) + "\"><input type=Submit name=\"Add Game\" value=\"Add Game\"></td></tr>\n"

	}
	out2 = out2 + "</table>"
	out := header + out2 + footer
	w.Write([]byte(out))
}

func (me *Env) AdminDivisionView(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	header := ReturnHeader(true)
	footer := ReturnFooter()

	divisionid := ps.ByName("divisionid")
	did, err := strconv.Atoi(divisionid)
	if err != nil {
		log.Println("Bad ID", err)
		PrintError(w, "Bad Division ID")
		return
	}
	listofgames := me.GamesByDivisionList(did, true, true)
	teams := me.ReturnTeamsByDivisionIDTable(did, true)
	addTeamButton := `<br><form action=/admin/teams><input type=submit name="Add Team" value="Add Team"></form><br>`
	addGameButton := `<br><form action="/admin/creategame/` + divisionid + `"><input type=submit name="Add Game" value="Add Game"></form><br>`
	w.Write([]byte(header + teams + addTeamButton + "<h2>Games</h2>" + listofgames + addGameButton + footer))
}
