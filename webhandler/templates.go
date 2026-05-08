package webhandler

import (
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
	}).ParseFS(templateFS,
		"templates/*.html",
		"templates/admin/*.html",
	))
}

type baseData struct {
	IsAdmin   bool
	CSRFField template.HTML
}

func newBase(r *http.Request, isAdmin bool) baseData {
	return baseData{IsAdmin: isAdmin, CSRFField: csrf.TemplateField(r)}
}

type divisionTeamRow struct {
	mydb.Team
	GamesPlayed int
}

type indexData struct {
	baseData
	Divisions []mydb.Division
	Teams     map[int][]mydb.Team
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

type adminIndexData struct {
	baseData
	DisableDelete bool
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

func (me *Env) render(w http.ResponseWriter, name string, data any) {
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		log.Println("template error:", name, err)
	}
}
