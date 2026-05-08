# html/template Migration Design

**Date:** 2026-05-07  
**Status:** Approved

## Problem

All HTML is built via Go string concatenation. User-supplied strings (team names, coach names, division names, location, umpire) are injected into HTML without escaping, creating XSS vectors in every handler. The approach is also unmaintainable — every UI change requires editing Go source.

## Goal

Migrate all handlers to `html/template`. Auto-escaping eliminates XSS. UI look is kept identical — this is a mechanical refactor, not a redesign.

## Approach

Embedded templates (`//go:embed`) compiled into the binary. Templates parsed once at startup. Each handler builds a typed data struct and calls a `render()` helper. No disk files needed at runtime — single-binary deployment preserved.

## File Structure

### New: `templates/` directory

```
templates/
  layout.html              ← defines {{header}} and {{footer}} named templates
  index.html               ← {{define "index"}}
  login.html               ← {{define "login"}}
  divisions.html           ← {{define "divisions"}} — public division view (standings + games)
  team.html                ← {{define "team"}} — public team detail
  games.html               ← {{define "games"}} — all games, public
  hrderby.html             ← {{define "hrderby"}} — HR Derby info (content moved as-is)
  admin/
    index.html             ← {{define "adminIndex"}}
    divisions.html         ← {{define "adminDivisions"}} — add/delete division
    division_view.html     ← {{define "adminDivisionView"}}
    teams.html             ← {{define "adminTeams"}} — add/delete team
    create_game.html       ← {{define "createGame"}}
    score_game.html        ← {{define "scoreGame"}}
    games.html             ← {{define "adminGames"}}
```

### New: `webhandler/templates.go`

Owns the `embed.FS`, template parsing at `init()`, all data structs, and the `render()` helper on `Env`.

### Modified: `webhandler/webhandler.go`, `webhandler/teams.go`, `webhandler/divisions.go`

Handlers simplified: remove all string building, build typed data struct, call `me.render()`. Remove `gorilla/csrf` string conversion — CSRF field now lives in the data struct.

### Unchanged

- `webhandler/sortteams.go` — pure logic, no HTML
- `mydb/` — no changes
- `main.go` — CSS stays inline, static file serving unchanged
- `localdb/` — no changes

## Data Structs (all in `webhandler/templates.go`)

```go
type baseData struct {
    IsAdmin   bool
    CSRFField template.HTML // csrf.TemplateField(r)
}

type indexData             struct { baseData; Divisions []mydb.Division; Teams map[int][]mydb.Team }
type divisionData          struct { baseData; Division mydb.Division; Teams []mydb.Team; Games []mydb.Game }
type teamData              struct { baseData; Team mydb.Team; Games []mydb.Game }
type gamesData             struct { baseData; Games []mydb.Game }
type loginData             struct { baseData; Error string }
type adminIndexData        struct { baseData; DisableDelete bool }
type adminDivisionsData    struct { baseData; Divisions []mydb.Division; DisableDelete bool }
type adminDivisionViewData struct { baseData; Division mydb.Division; DivisionID int; Teams []mydb.Team; Games []mydb.Game; DisableDelete bool }
type adminTeamsData        struct { baseData; Divisions []mydb.Division; TeamsByDivision map[int][]mydb.Team; DisableDelete bool }
type createGameData        struct { baseData; DivisionID int; Teams []mydb.Team; Games []mydb.Game }
type scoreGameData         struct { baseData; Game mydb.Game; ScoreOptions []int }
type adminGamesData        struct { baseData; Games []mydb.Game }
```

## Template Rendering

### `webhandler/templates.go`

```go
//go:embed templates
var templateFS embed.FS

var tmpl *template.Template

func init() {
    tmpl = template.Must(template.New("").ParseFS(templateFS,
        "templates/layout.html",
        "templates/*.html",
        "templates/admin/*.html",
    ))
}

func (me *Env) render(w http.ResponseWriter, name string, data any) {
    if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
        http.Error(w, "template error", http.StatusInternalServerError)
        log.Println("template error:", name, err)
    }
}
```

### Layout pattern

`layout.html` defines two named partials — `header` and `footer`:

```html
{{define "header"}}
<!doctype html>
<html lang="en">
<head>
  <title>Battle at the Dawg Pound</title>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
  <link rel="stylesheet" href="/style.css">
  <link rel="stylesheet" href="https://stackpath.bootstrapcdn.com/bootstrap/4.5.0/css/bootstrap.min.css" ...>
  <script src="...jquery..."></script>
  <script src="...popper..."></script>
  <script src="...bootstrap..."></script>
</head>
<body>
<img src="/img/topimage.jpg"><br>
<a href="/">Home</a> | <a href="/hrderbyinfo">Skills &amp; HR derby Info</a>
{{if .IsAdmin}}
| <a href="/admin/">Admin</a> | <a href="/admin/adddivisionform">Divisions</a> | <a href="/admin/teams">Teams</a> | <a href="/admin/games">Games</a>
{{else}}
| <a href="/login">Login</a>
{{end}}
<br><hr>
{{end}}

{{define "footer"}}
<br><hr>Powered by <a href="https://github.com/Harnish/tourneyweb">TourneyWeb</a>
</body></html>
{{end}}
```

Each page template:

```html
{{define "index"}}
{{template "header" .}}
... page content using {{.Divisions}}, {{.Teams}}, etc. ...
{{template "footer" .}}
{{end}}
```

### CSRF in templates

Each data struct that has a form includes `CSRFField template.HTML` set from `csrf.TemplateField(r)`. Templates render it as:

```html
<form method="post" action="/admin/adddivision">
  {{.CSRFField}}
  ...
</form>
```

`template.HTML` is exempt from auto-escaping — gorilla/csrf's output renders as raw HTML. All user data (`{{.Team.Name}}`, etc.) is auto-escaped.

## XSS Fix

`html/template` escapes all `{{.Field}}` substitutions by default. The only `template.HTML` value used is `CSRFField` — generated by gorilla/csrf, never from user input.

`ScoreOptions []int` is a plain integer slice iterated with `{{range .ScoreOptions}}<option value="{{.}}">{{.}}</option>{{end}}` — integers are safe without escaping and are not marked `template.HTML`.

No user-supplied string is ever marked `template.HTML`.

## Handler Simplification Example

**Before** (`CreateGame`, ~25 lines of string building):
```go
form := `<form method=post action=/admin/addgame>...` + divisionidstr + `...` + teamoptions + `...` + string(csrf.TemplateField(r)) + `...</form>`
w.Write([]byte(header))
w.Write([]byte(form))
...
w.Write([]byte(ReturnFooter()))
```

**After** (~8 lines):
```go
func (me *Env) CreateGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
    did, err := strconv.Atoi(ps.ByName("divisionid"))
    if err != nil { PrintError(w, "bad division ID"); return }
    me.render(w, "createGame", createGameData{
        baseData:   baseData{IsAdmin: true, CSRFField: csrf.TemplateField(r)},
        DivisionID: did,
        Teams:      me.DB.ReturnTeamsByDivisionID(did),
        Games:      me.DB.AllGamesByDivision(did),
    })
}
```

## Out of Scope

- CSS (stays inline in `main.go`)
- Any UI or layout changes
- `hrderby.html` content (moved as-is, no changes)
- `mydb/` package
- Improving the `ReturnAllGamesInTable` GET-form-with-side-effects issue (separate TODO)
