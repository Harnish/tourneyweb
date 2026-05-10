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
	))
}

type baseData struct {
	IsAdmin    bool
	CSRFField  template.HTML
	Tournament mydb.Tournament
}

func newBase(r *http.Request, isAdmin bool) baseData {
	return baseData{IsAdmin: isAdmin, CSRFField: csrf.TemplateField(r)}
}

func newBaseWithTournament(r *http.Request, isAdmin bool, t mydb.Tournament) baseData {
	return baseData{IsAdmin: isAdmin, CSRFField: csrf.TemplateField(r), Tournament: t}
}

type divisionTeamRow struct {
	mydb.Team
	GamesPlayed int
}

// Home page: tournament listing
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

// Public tournament home: divisions + teams overview
type tournamentData struct {
	baseData
	Divisions []mydb.Division
	Teams     map[int][]mydb.Team
}

// Admin tournament list
type adminTournamentsData struct {
	baseData
	Tournaments []mydb.Tournament
}

// Admin tournament home
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
