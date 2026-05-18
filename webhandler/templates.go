package webhandler

import (
	"bytes"
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"time"

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
		"htmlSafe": func(s string) template.HTML { return template.HTML(s) },
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("Jan 2, 2006 3:04 PM")
		},
		"formatDateTimeLocal": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02T15:04")
		},
	}).ParseFS(templateFS,
		"templates/*.html",
		"templates/admin/*.html",
		"templates/auth/*.html",
		"templates/manage/*.html",
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
	SiteNews      []mydb.NewsItem
}

type tournamentData struct {
	baseData
	Divisions []mydb.Division
	Teams     map[int][]mydb.Team
	News      []mydb.NewsItem
	Fields    []mydb.Location
}

type tournamentFieldsData struct {
	baseData
	Fields []mydb.Location
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
	Division     mydb.Division
	Teams        []divisionTeamRow
	Games        []mydb.Game
	RankingLabel string
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
	Locations     []mydb.Location
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
	Locations []mydb.Location
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

type adminLocationsData struct {
	baseData
	Locations     []mydb.Location
	LocationsJSON template.JS
	DisableDelete bool
}

type editLocationData struct {
	baseData
	Location     mydb.Location
	LocationJSON template.JS
}

type mapViewData struct {
	baseData
	Locations     []mydb.Location
	LocationsJSON template.JS
}

type errorData struct {
	baseData
	Title   string
	Message string
}

type manageDashboardData struct {
	baseData
	IsDraft bool
}

type managePublishData struct {
	baseData
	Error string
}

type manageDivisionsData struct {
	baseData
	Divisions     []mydb.Division
	Brackets      map[int]mydb.Bracket // division ID → bracket
	DisableDelete bool
}

type manageDivisionEditData struct {
	baseData
	Division    mydb.Division
	AllCriteria []CriterionUIRow
}

type manageBracketSeedData struct {
	baseData
	Division mydb.Division
	Bracket  mydb.Bracket
	Seeds    []mydb.BracketSeed
}

type bracketRound struct {
	Label      string
	Games      []mydb.BracketGame
	Connectors []struct{} // one entry per game; range over to render connector lines
}

type bracketData struct {
	baseData
	Division mydb.Division
	Bracket  mydb.Bracket
	Rounds   []bracketRound
}

type manageTeamsData struct {
	baseData
	Divisions       []mydb.Division
	TeamsByDivision map[int][]mydb.Team
	DisableDelete   bool
}

type manageTeamEditData struct {
	baseData
	Team      mydb.Team
	Divisions []mydb.Division
}

type manageLocationsData struct {
	baseData
	Locations     []mydb.Location
	DisableDelete bool
}

type manageLocationEditData struct {
	baseData
	Location     mydb.Location
	LocationJSON template.JS
}

type manageCreateGameData struct {
	baseData
	DivisionID    int
	Teams         []mydb.Team
	Games         []mydb.Game
	Locations     []mydb.Location
	DisableDelete bool
}

type manageEditGameData struct {
	baseData
	Game      mydb.Game
	Teams     []mydb.Team
	Divisions []mydb.Division
	Locations []mydb.Location
}

type adminQueueData struct {
	baseData
	Drafts     []mydb.Tournament
	IssuedCode string
	IssuedFor  string
}

type newsListData struct {
	baseData
	News          []mydb.NewsItem
	DisableDelete bool
}

type newsEditData struct {
	baseData
	Item mydb.NewsItem
}

func (me *Env) renderError(w http.ResponseWriter, r *http.Request, status int, title, message string) {
	w.WriteHeader(status)
	me.render(w, "error", errorData{
		baseData: newBase(r),
		Title:    title,
		Message:  message,
	})
}

func (me *Env) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		slog.Error("template error", "name", name, "err", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}
