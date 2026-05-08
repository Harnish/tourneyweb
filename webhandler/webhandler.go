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

type contextKey struct{}

type Env struct {
	DB            *mydb.MyDB
	AdminPW       string
	DisableDelete bool
}

func New(db *mydb.MyDB, adminpw string, dd bool) *Env {
	return &Env{DB: db, AdminPW: adminpw, DisableDelete: dd}
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

func (me *Env) PrintIndex(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	divs := me.DB.ReturnDivisions()
	teams := make(map[int][]mydb.Team)
	for _, div := range divs {
		teams[div.ID] = me.DB.ReturnTeamsByDivisionID(div.ID)
	}
	me.render(w, "index", indexData{
		baseData:  newBase(r, false),
		Divisions: divs,
		Teams:     teams,
	})
}

func (me *Env) AdminIndex(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "adminIndex", adminIndexData{
		baseData:      newBase(r, true),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) LoginForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "login", loginData{baseData: newBase(r, false)})
}

func (me *Env) Login(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	password := r.FormValue("password")
	username := r.FormValue("username")
	if password == me.AdminPW {
		session, err := sessions.Start(w, r, true)
		if err != nil {
			log.Println("Session Failed to start", err)
		}
		session.Set("userid", username)
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
		return
	}
	me.render(w, "login", loginData{
		baseData: newBase(r, false),
		Error:    "Login Failed",
	})
}

func (me *Env) Logout(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {}

func (me *Env) PrintDivision(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	did, err := strconv.Atoi(ps.ByName("id"))
	if err != nil {
		log.Println("bad ID", ps.ByName("id"), err)
		http.Error(w, "Bad division ID", http.StatusBadRequest)
		return
	}
	rawTeams := me.DB.ReturnTeamsByDivisionIDWithStats(did)
	rawTeams = me.SortTeams(rawTeams, "WinsRunsAgainstRunsEarnedHead2Head")
	rows := make([]divisionTeamRow, len(rawTeams))
	for i, t := range rawTeams {
		rows[i] = divisionTeamRow{Team: t, GamesPlayed: me.DB.GamesPlayedByTeam(t.ID)}
	}
	me.render(w, "divisions", divisionData{
		baseData: newBase(r, false),
		Division: me.DB.ReturnDivisionByID(did),
		Teams:    rows,
		Games:    me.DB.AllGamesByDivision(did),
	})
}

func (me *Env) DelGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	gid, err := strconv.Atoi(ps.ByName("gameid"))
	if err != nil {
		log.Println("DelGame bad ID", err, ps.ByName("gameid"))
		http.Error(w, "Bad Game ID", http.StatusBadRequest)
		return
	}
	if !me.DisableDelete {
		me.DB.DelGame(gid)
	}
	http.Redirect(w, r, "/admin/games", http.StatusSeeOther)
}

func (me *Env) ScoreGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	gid, err := strconv.Atoi(ps.ByName("gameid"))
	if err != nil {
		log.Println("ScoreGame bad ID", err, ps.ByName("gameid"))
		http.Error(w, "Bad Game ID", http.StatusBadRequest)
		return
	}
	options := make([]int, 41)
	for i := range options {
		options[i] = i
	}
	me.render(w, "scoreGame", scoreGameData{
		baseData:     newBase(r, true),
		Game:         me.DB.ReturnGameByID(gid),
		ScoreOptions: options,
	})
}

func (me *Env) RecordScore(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	gid, err := strconv.Atoi(r.FormValue("gameid"))
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
	http.Redirect(w, r, "/admin/games", http.StatusSeeOther)
}

func (me *Env) Games(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "games", gamesData{
		baseData: newBase(r, false),
		Games:    me.DB.AllGames(),
	})
}

func (me *Env) AdminGames(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "adminGames", adminGamesData{
		baseData:      newBase(r, true),
		Games:         me.DB.AllGames(),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) CreateGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	did, err := strconv.Atoi(ps.ByName("divisionid"))
	if err != nil {
		http.Error(w, "Bad division ID", http.StatusBadRequest)
		return
	}
	me.render(w, "createGame", createGameData{
		baseData:      newBase(r, true),
		DivisionID:    did,
		Teams:         me.DB.ReturnTeamsByDivisionID(did),
		Games:         me.DB.AllGamesByDivision(did),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) CreateGameSubmit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	did, err := strconv.Atoi(r.FormValue("divisionid"))
	if err != nil {
		http.Error(w, "Bad DivisionID", http.StatusBadRequest)
		log.Println("Bad divisionid")
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
	me.DB.AddGame(did, hid, aid, r.FormValue("location"), r.FormValue("datetime"), r.FormValue("umpire"))
	me.render(w, "createGame", createGameData{
		baseData:      newBase(r, true),
		DivisionID:    did,
		Teams:         me.DB.ReturnTeamsByDivisionID(did),
		Games:         me.DB.AllGamesByDivision(did),
		DisableDelete: me.DisableDelete,
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
	if userid == nil {
		return user
	}
	user.ID = 1
	user.UserName = userid.(string)
	return user
}
