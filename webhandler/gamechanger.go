package webhandler

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	qrcode "github.com/skip2/go-qrcode"
)

// validGameChangerURL trims raw and confirms it is an http(s) URL whose host is
// gamechanger.io or a subdomain of it. An empty/blank input is valid and means
// "clear the link". Returns the cleaned URL and whether it is acceptable.
func validGameChangerURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	host := strings.ToLower(u.Hostname())
	if host != "gamechanger.io" && !strings.HasSuffix(host, ".gamechanger.io") {
		return "", false
	}
	return raw, true
}

// RosterSetGameChanger lets a team's coach (or a director/admin) set or clear the
// team's GameChanger link from the roster management page.
func (me *Env) RosterSetGameChanger(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		http.Error(w, "Bad team ID", http.StatusBadRequest)
		return
	}
	team := me.DB.ReturnTeamByID(teamID)
	if team.ID == 0 || team.TournamentID != t.ID {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}
	if !me.rosterAccess(w, r, t.ID, teamID) {
		return
	}
	gcURL, valid := validGameChangerURL(r.FormValue("gamechanger_url"))
	if !valid {
		me.renderError(w, r, http.StatusBadRequest, "Invalid Link",
			"The GameChanger link must be a full https://gamechanger.io URL.")
		return
	}
	me.DB.SetTeamGameChanger(teamID, gcURL)
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/teams/%d/roster", t.ID, teamID), http.StatusSeeOther)
}

// TeamGameChangerQR serves a PNG QR code encoding the team's GameChanger link.
// Returns 404 when the team has no link set.
func (me *Env) TeamGameChangerQR(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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
	if team.ID == 0 || team.TournamentID != t.ID || team.GameChangerURL == "" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	png, err := qrcode.Encode(team.GameChangerURL, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(png)
}
