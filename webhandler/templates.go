package webhandler

import (
	"bytes"
	"embed"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/csrf"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

//go:embed templates
var templateFS embed.FS

var tmpl *template.Template

func init() {
	tmpl = template.Must(template.New("").Funcs(template.FuncMap{
		"inc": func(i int) int { return i + 1 },
		"dec": func(i int) int { return i - 1 },
	}).ParseFS(templateFS,
		"templates/*.html",
		"templates/admin/*.html",
		"templates/auth/*.html",
	))
}

type baseData struct {
	CSRFField  template.HTML
	Tournament mydb.Tournament
	User       TWUser
}

// IsAdmin is callable from templates as {{if .IsAdmin}}.
func (b baseData) IsAdmin() bool { return b.User.IsAdmin }

func newBase(r *http.Request) baseData {
	return baseData{
		CSRFField: csrf.TemplateField(r),
		User:      userFromContext(r.Context()),
	}
}

func newBaseWithTournament(r *http.Request, t mydb.Tournament) baseData {
	return baseData{
		CSRFField:  csrf.TemplateField(r),
		Tournament: t,
		User:       userFromContext(r.Context()),
	}
}

type divisionTeamRow struct {
	mydb.Team
	GamesPlayed int
}

type indexData struct {
	baseData
	ComingUp      []mydb.Tournament
	Recent        []mydb.Tournament
	Future        []mydb.Tournament
	Past          []mydb.Tournament
	FuturePage    int
	PastPage      int
	FutureTotal   int
	PastTotal     int
	FutureHasPrev bool
	FutureHasNext bool
	PastHasPrev   bool
	PastHasNext   bool
}

type tournamentData struct {
	baseData
	Divisions []mydb.Division
	Teams     map[int][]mydb.Team
}

type adminTournamentsData struct {
	baseData
	Tournaments []mydb.Tournament
}

type adminTournamentViewData struct {
	baseData
	DisableDelete bool
}

type divisionData struct {
	baseData
	Division mydb.Division
	Teams    []divisionTeamRow
	Games    []mydb.Game
}

type teamData struct {
	baseData
	Team  mydb.Team
	Games []mydb.Game
}

type gamesData struct {
	baseData
	Games []mydb.Game
}

type loginData struct {
	baseData
	Error string
}

type adminDivisionsData struct {
	baseData
	Divisions     []mydb.Division
	DisableDelete bool
}

type adminDivisionViewData struct {
	baseData
	Division      mydb.Division
	DivisionID    int
	Teams         []mydb.Team
	Games         []mydb.Game
	DisableDelete bool
}

type adminTeamsData struct {
	baseData
	Divisions       []mydb.Division
	TeamsByDivision map[int][]mydb.Team
	DisableDelete   bool
}

type createGameData struct {
	baseData
	DivisionID    int
	Teams         []mydb.Team
	Games         []mydb.Game
	DisableDelete bool
}

type scoreGameData struct {
	baseData
	Game         mydb.Game
	ScoreOptions []int
}

type adminGamesData struct {
	baseData
	Games         []mydb.Game
	DisableDelete bool
}

type editTournamentData struct {
	baseData
}

type editDivisionData struct {
	baseData
	Division mydb.Division
}

type editTeamData struct {
	baseData
	Team      mydb.Team
	Divisions []mydb.Division
}

type editGameData struct {
	baseData
	Game      mydb.Game
	Teams     []mydb.Team
	Divisions []mydb.Division
}

// Auth template data types
type registerData struct {
	baseData
	Error string
}

type verifyData struct {
	baseData
	Success bool
	Error   string
}

type passwordResetData struct {
	baseData
	Error   string
	Success bool
}

type passwordResetConfirmData struct {
	baseData
	Token string
	Error string
}

// Role management
type manageRolesData struct {
	baseData
	Roles   []mydb.TournamentRole
	Users   map[int]mydb.User
	Teams   []mydb.Team
	Error   string
	Success string
}

func (me *Env) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Println("template error:", name, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}
