package webhandler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/rivo/sessions"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

type contextKey struct{}

// TWUser is the authenticated user injected into every request context.
// ID == 0 means unauthenticated.
type TWUser struct {
	ID      int
	Email   string
	Name    string
	IsAdmin bool
	Roles   []TournamentRole
}

type TournamentRole struct {
	TournamentID int
	Role         string // "director", "staff", "coach"
	TeamID       int    // non-zero for coach only
}

func (u TWUser) LoggedIn() bool { return u.ID > 0 }

func (u TWUser) IsDirectorFor(tid int) bool {
	for _, r := range u.Roles {
		if r.TournamentID == tid && r.Role == "director" {
			return true
		}
	}
	return false
}

func (u TWUser) IsStaffFor(tid int) bool {
	for _, r := range u.Roles {
		if r.TournamentID == tid && r.Role == "staff" {
			return true
		}
	}
	return false
}

func (u TWUser) IsCoachFor(tid int) bool {
	for _, r := range u.Roles {
		if r.TournamentID == tid && r.Role == "coach" {
			return true
		}
	}
	return false
}

func (u TWUser) CanManage(tid int) bool { return u.IsAdmin || u.IsDirectorFor(tid) }
func (u TWUser) CanScore(tid int) bool {
	return u.IsAdmin || u.IsDirectorFor(tid) || u.IsStaffFor(tid)
}

func userFromContext(ctx context.Context) TWUser {
	user, _ := ctx.Value(contextKey{}).(TWUser)
	return user
}

// loginAttempt tracks failed login attempts per IP.
type loginAttempt struct {
	count     int
	lockUntil time.Time
}

type Env struct {
	DB                       *mydb.MyDB
	Email                    *EmailService
	DisableDelete            bool
	DisableEmailVerification bool
	loginMu                  sync.Mutex
	loginAttempts            map[string]*loginAttempt
}

func New(db *mydb.MyDB, email *EmailService, dd, disableEmailVerification bool) *Env {
	return &Env{
		DB:                       db,
		Email:                    email,
		DisableDelete:            dd,
		DisableEmailVerification: disableEmailVerification,
		loginAttempts:            make(map[string]*loginAttempt),
	}
}

func (me *Env) loginDelay(ip string) bool {
	me.loginMu.Lock()
	a := me.loginAttempts[ip]
	if a == nil {
		me.loginMu.Unlock()
		return false
	}
	until := a.lockUntil
	locked := a.count > 10
	me.loginMu.Unlock()
	if wait := time.Until(until); wait > 0 {
		time.Sleep(wait)
	}
	return locked
}

func (me *Env) loginFailed(ip string) {
	me.loginMu.Lock()
	defer me.loginMu.Unlock()
	a := me.loginAttempts[ip]
	if a == nil {
		a = &loginAttempt{}
		me.loginAttempts[ip] = a
	}
	a.count++
	delay := time.Duration(1<<min(a.count, 8)) * time.Second
	a.lockUntil = time.Now().Add(delay)
}

func (me *Env) loginSucceeded(ip string) {
	me.loginMu.Lock()
	defer me.loginMu.Unlock()
	delete(me.loginAttempts, ip)
}

// RequestLogger logs every request, loads the authenticated user into context,
// and enforces prefix-level route guards.
func (me *Env) RequestLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedip := r.Header.Get("X-Forwarded-For")
		log.Println(r.Method, r.URL.Path, r.Proto, forwardedip)

		user := me.loadUser(w, r)
		ctx := context.WithValue(r.Context(), contextKey{}, user)
		r = r.WithContext(ctx)

		// /admin/* requires global admin.
		if strings.HasPrefix(r.URL.Path, "/admin") && !user.IsAdmin {
			log.Println(r.RemoteAddr, r.Method, r.URL.Path, user.Email, "Permission Denied")
			if !user.LoggedIn() {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			} else {
				http.Error(w, "Not authorized", http.StatusForbidden)
			}
			return
		}

		// /tournaments/:tid/manage/* requires CanManage for that tournament.
		if strings.Contains(r.URL.Path, "/manage/") {
			tidStr := extractTID(r.URL.Path)
			if tidStr != "" {
				tid, err := strconv.Atoi(tidStr)
				if err != nil || !user.CanManage(tid) {
					if !user.LoggedIn() {
						http.Redirect(w, r, "/login", http.StatusSeeOther)
					} else {
						http.Error(w, "Not authorized", http.StatusForbidden)
					}
					return
				}
			}
		}

		// /tournaments/:tid/score/* requires CanScore for that tournament.
		if strings.Contains(r.URL.Path, "/score/") {
			tidStr := extractTID(r.URL.Path)
			if tidStr != "" {
				tid, err := strconv.Atoi(tidStr)
				if err != nil || !user.CanScore(tid) {
					if !user.LoggedIn() {
						http.Redirect(w, r, "/login", http.StatusSeeOther)
					} else {
						http.Error(w, "Not authorized", http.StatusForbidden)
					}
					return
				}
			}
		}

		h.ServeHTTP(w, r)
	})
}

// extractTID pulls the tournament ID string from paths like /tournaments/42/...
func extractTID(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "tournaments" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// loadUser reads the session, fetches user + roles from DB, returns TWUser.
// Returns zero TWUser if not authenticated.
func (me *Env) loadUser(w http.ResponseWriter, r *http.Request) TWUser {
	session, err := sessions.Start(w, r, false)
	if err != nil || session == nil {
		return TWUser{}
	}
	raw := session.Get("user_id", nil)
	userID, ok := raw.(int)
	if !ok || userID == 0 {
		return TWUser{}
	}
	u, err := me.DB.GetUserByID(userID)
	if err != nil || u.ID == 0 {
		return TWUser{}
	}
	dbRoles := me.DB.GetRolesForUser(u.ID)
	roles := make([]TournamentRole, len(dbRoles))
	for i, r := range dbRoles {
		roles[i] = TournamentRole{TournamentID: r.TournamentID, Role: r.Role, TeamID: r.TeamID}
	}
	return TWUser{ID: u.ID, Email: u.Email, Name: u.Name, IsAdmin: u.IsAdmin, Roles: roles}
}

func (me *Env) PrintDivision(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	_, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad division ID", http.StatusBadRequest)
		return
	}
	rawTeams := me.DB.ReturnTeamsByDivisionIDWithStats(did)
	rawTeams = me.SortTeams(rawTeams, "WinsRunsAgainstRunsEarnedHead2Head")
	rows := make([]divisionTeamRow, len(rawTeams))
	for i, team := range rawTeams {
		rows[i] = divisionTeamRow{Team: team, GamesPlayed: me.DB.GamesPlayedByTeam(team.ID)}
	}
	me.render(w, "divisions", divisionData{
		baseData: newBase(r),
		Division: me.DB.ReturnDivisionByID(did),
		Teams:    rows,
		Games:    me.DB.AllGamesByDivision(did),
	})
}

func (me *Env) DelGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	gid, err := strconv.Atoi(ps.ByName("gid"))
	if err != nil {
		http.Error(w, "Bad Game ID", http.StatusBadRequest)
		return
	}
	if !me.DisableDelete {
		me.DB.DelGame(gid)
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/games", t.ID), http.StatusSeeOther)
}

func (me *Env) ScoreGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	_, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	gid, err := strconv.Atoi(ps.ByName("gid"))
	if err != nil {
		http.Error(w, "Bad Game ID", http.StatusBadRequest)
		return
	}
	options := make([]int, 41)
	for i := range options {
		options[i] = i
	}
	me.render(w, "scoreGame", scoreGameData{
		baseData:     newBase(r),
		Game:         me.DB.ReturnGameByID(gid),
		ScoreOptions: options,
	})
}

func (me *Env) RecordScore(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	gid, err := strconv.Atoi(ps.ByName("gid"))
	if err != nil {
		http.Error(w, "Bad game ID", http.StatusBadRequest)
		return
	}
	hscore, err := strconv.Atoi(r.FormValue("homescore"))
	if err != nil {
		http.Error(w, "Bad home score", http.StatusBadRequest)
		return
	}
	ascore, err := strconv.Atoi(r.FormValue("awayscore"))
	if err != nil {
		http.Error(w, "Bad away score", http.StatusBadRequest)
		return
	}
	me.DB.ScoreGame(gid, hscore, ascore)
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/games", t.ID), http.StatusSeeOther)
}

func (me *Env) Games(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	me.render(w, "games", gamesData{
		baseData: newBase(r),
		Games:    me.DB.AllGames(t.ID),
	})
}

func (me *Env) AdminGames(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	me.render(w, "adminGames", adminGamesData{
		baseData:      newBase(r),
		Games:         me.DB.AllGames(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) CreateGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	_, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad division ID", http.StatusBadRequest)
		return
	}
	me.render(w, "createGame", createGameData{
		baseData:      newBase(r),
		DivisionID:    did,
		Teams:         me.DB.ReturnTeamsByDivisionID(did),
		Games:         me.DB.AllGamesByDivision(did),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) CreateGameSubmit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(r.FormValue("divisionid"))
	if err != nil {
		http.Error(w, "Bad DivisionID", http.StatusBadRequest)
		return
	}
	hid, err := strconv.Atoi(r.FormValue("hometeam"))
	if err != nil {
		http.Error(w, "Bad Home team ID", http.StatusBadRequest)
		return
	}
	aid, err := strconv.Atoi(r.FormValue("awayteam"))
	if err != nil {
		http.Error(w, "Bad Away team ID", http.StatusBadRequest)
		return
	}
	if aid == hid {
		http.Error(w, "Must select a different team as an opponent.", http.StatusBadRequest)
		return
	}
	me.DB.AddGame(t.ID, did, hid, aid, r.FormValue("location"), r.FormValue("datetime"), r.FormValue("umpire"))
	http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/divisions/%d/games/new", t.ID, did), http.StatusSeeOther)
}

func (me *Env) EditGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	gid, err := strconv.Atoi(ps.ByName("gid"))
	if err != nil {
		log.Println("EditGame bad ID:", err)
		http.Error(w, "Bad Game ID", http.StatusBadRequest)
		return
	}
	game := me.DB.ReturnGameByID(gid)
	if game.ID == 0 {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPost {
		did, err := strconv.Atoi(r.FormValue("divisionid"))
		if err != nil {
			http.Error(w, "Bad Division ID", http.StatusBadRequest)
			return
		}
		hid, err := strconv.Atoi(r.FormValue("hometeam"))
		if err != nil {
			http.Error(w, "Bad Home Team ID", http.StatusBadRequest)
			return
		}
		aid, err := strconv.Atoi(r.FormValue("awayteam"))
		if err != nil {
			http.Error(w, "Bad Away Team ID", http.StatusBadRequest)
			return
		}
		if hid == aid {
			http.Error(w, "Home and away team must be different", http.StatusBadRequest)
			return
		}
		div := me.DB.ReturnDivisionByID(did)
		if div.ID == 0 || div.TournamentID != t.ID {
			http.Error(w, "Division does not belong to this tournament", http.StatusBadRequest)
			return
		}
		homeTeam := me.DB.ReturnTeamByID(hid)
		if homeTeam.ID == 0 || homeTeam.TournamentID != t.ID {
			http.Error(w, "Home team does not belong to this tournament", http.StatusBadRequest)
			return
		}
		awayTeam := me.DB.ReturnTeamByID(aid)
		if awayTeam.ID == 0 || awayTeam.TournamentID != t.ID {
			http.Error(w, "Away team does not belong to this tournament", http.StatusBadRequest)
			return
		}
		me.DB.UpdateGame(gid, did, hid, aid, r.FormValue("location"), r.FormValue("datetime"), r.FormValue("umpire"))
		http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/games", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "editGame", editGameData{
		baseData:  newBase(r),
		Game:      game,
		Teams:     me.DB.ReturnTeamsByTournamentID(t.ID),
		Divisions: me.DB.ReturnDivisions(t.ID),
	})
}

func (me *Env) PrintHRDerby(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "hrderby", newBase(r))
}
