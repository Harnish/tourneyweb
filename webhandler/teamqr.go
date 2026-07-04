package webhandler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	qrcode "github.com/skip2/go-qrcode"
)

// TeamQRCode serves a PNG QR code encoding the team's public page URL.
func (me *Env) TeamQRCode(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		http.Error(w, "Bad Team ID", http.StatusBadRequest)
		return
	}
	team := me.DB.ReturnTeamByID(teamID)
	if team.ID == 0 || team.TournamentID != t.ID {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	url := fmt.Sprintf("%s/tournaments/%d/teams/%d", me.BaseURL, t.ID, team.ID)
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}

// TeamSheet renders a printable team sheet: roster plus a QR code linking to
// the team's public page.
func (me *Env) TeamSheet(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		http.Error(w, "Bad Team ID", http.StatusBadRequest)
		return
	}
	team := me.DB.ReturnTeamByID(teamID)
	if team.ID == 0 || team.TournamentID != t.ID {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}
	me.render(w, "teamSheet", teamSheetData{
		baseData: newBaseWithTournament(r, t),
		Team:     team,
		Players:  me.DB.GetPlayersByTeamID(teamID),
	})
}
