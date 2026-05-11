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

type loginAttempt struct {
	count    int
	lockUntil time.Time
}

type Env struct {
	DB            *mydb.MyDB
	AdminPW       string
	DisableDelete bool
	loginMu       sync.Mutex
	loginAttempts map[string]*loginAttempt
}

func New(db *mydb.MyDB, adminpw string, dd bool) *Env {
	return &Env{
		DB:            db,
		AdminPW:       adminpw,
		DisableDelete: dd,
		loginAttempts: make(map[string]*loginAttempt),
	}
}

// loginDelay blocks until the lockout for ip has expired, then returns whether
// the IP is currently locked out (more than 10 consecutive failures).
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
	// Exponential backoff: 2^count seconds, capped at 5 minutes.
	delay := time.Duration(1<<min(a.count, 8)) * time.Second
	a.lockUntil = time.Now().Add(delay)
}

func (me *Env) loginSucceeded(ip string) {
	me.loginMu.Lock()
	defer me.loginMu.Unlock()
	delete(me.loginAttempts, ip)
}


func (me *Env) RequestLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userdata := me.MySession(w, r)
		forwardedip := r.Header.Get("X-Forwarded-For")
		log.Println(r.Method, r.URL.Path, r.Proto, forwardedip)
		if strings.HasPrefix(r.URL.Path, "/admin") && userdata.ID < 1 {
			log.Println(r.RemoteAddr, r.Method, r.URL.Path, userdata.UserName, "Permission Denied")
			http.Error(w, "Not authorized", http.StatusForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), contextKey{}, userdata.ID)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (me *Env) LoginForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "login", loginData{baseData: newBase(r, false)})
}

func (me *Env) Login(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.RemoteAddr
	}

	if me.loginDelay(ip) {
		http.Error(w, "Too many failed attempts — try again later", http.StatusTooManyRequests)
		return
	}

	password := r.FormValue("password")
	username := r.FormValue("username")
	if password == me.AdminPW {
		me.loginSucceeded(ip)
		session, err := sessions.Start(w, r, true)
		if err != nil {
			log.Println("Session Failed to start", err)
		}
		session.Set("userid", username)
		http.Redirect(w, r, "/admin/tournaments", http.StatusSeeOther)
		return
	}

	me.loginFailed(ip)
	log.Println("login failed for", ip)
	me.render(w, "login", loginData{
		baseData: newBase(r, false),
		Error:    "Login Failed",
	})
}

func (me *Env) Logout(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {}

func (me *Env) PrintDivision(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
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
		baseData: newBaseWithTournament(r, false, t),
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
	t, ok := me.tournamentFromRoute(w, ps)
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
		baseData:     newBaseWithTournament(r, true, t),
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
		baseData: newBaseWithTournament(r, false, t),
		Games:    me.DB.AllGames(t.ID),
	})
}

func (me *Env) AdminGames(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	me.render(w, "adminGames", adminGamesData{
		baseData:      newBaseWithTournament(r, true, t),
		Games:         me.DB.AllGames(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) CreateGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad division ID", http.StatusBadRequest)
		return
	}
	me.render(w, "createGame", createGameData{
		baseData:      newBaseWithTournament(r, true, t),
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
		baseData:  newBaseWithTournament(r, true, t),
		Game:      game,
		Teams:     me.DB.ReturnTeamsByTournamentID(t.ID),
		Divisions: me.DB.ReturnDivisions(t.ID),
	})
}

func (me *Env) PrintHRDerby(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "hrderby", newBase(r, false))
}

type TWUser struct {
	ID       int
	UserName string
}

func (me *Env) MySession(w http.ResponseWriter, r *http.Request) TWUser {
	var user TWUser
	user.ID = -1
	session, err := sessions.Start(w, r, false)
	if err != nil || session == nil {
		return user
	}
	userid := session.Get("userid", nil)
	s, ok := userid.(string)
	if !ok || s == "" {
		return user
	}
	user.ID = 1
	user.UserName = s
	return user
}
