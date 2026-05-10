# Edit Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add edit forms for Tournaments, Divisions, Teams, and Games so any entity can be corrected after creation.

**Architecture:** Each entity gets `GET /:id/edit` (pre-filled form) and `POST /:id/edit` (submit), following the same GET+POST pattern as `AddDivisionForm`. New `Update*` functions in `mydb/` issue `UPDATE … WHERE id=$N`. Four new templates pre-fill current values using template expressions. Edit links are added to existing admin views.

**Tech Stack:** Go, `html/template`, PostgreSQL (`jackc/pgx/v5/stdlib`), `httprouter`, `gorilla/csrf`

---

## File Map

**New files:**
- `webhandler/templates/admin/edit_tournament.html`
- `webhandler/templates/admin/edit_division.html`
- `webhandler/templates/admin/edit_team.html`
- `webhandler/templates/admin/edit_game.html`

**Modified files:**
- `mydb/tournaments.go` — add `UpdateTournament`
- `mydb/divisions.go` — add `UpdateDivision`
- `mydb/teams.go` — add `UpdateTeam`, `ReturnTeamsByTournamentID`
- `mydb/games.go` — add `UpdateGame`
- `webhandler/templates.go` — add `editDivisionData`, `editTeamData`, `editGameData`
- `webhandler/tournaments.go` — add `EditTournament`
- `webhandler/divisions.go` — add `EditDivision`
- `webhandler/teams.go` — add `EditTeam`
- `webhandler/webhandler.go` — add `EditGame`
- `webhandler/templates/admin/tournament_view.html` — add Edit link
- `webhandler/templates/admin/division_view.html` — add Edit Division link + Edit per game row
- `webhandler/templates/admin/teams.html` — add Edit per team row
- `webhandler/templates/admin/games.html` — add Edit per game row
- `main.go` — register 8 new routes

---

### Task 1: mydb update functions

**Files:**
- Modify: `mydb/tournaments.go`
- Modify: `mydb/divisions.go`
- Modify: `mydb/teams.go`
- Modify: `mydb/games.go`

- [ ] **Step 1: Add `UpdateTournament` to `mydb/tournaments.go`**

Append after `DelTournament`:

```go
func (me *MyDB) UpdateTournament(id int, name, sport, location, notes string, date time.Time) {
	_, err := me.DB.Exec(
		`UPDATE tournaments SET name=$1, sport=$2, location=$3, notes=$4, start_date=$5 WHERE id=$6`,
		name, sport, location, notes, date, id,
	)
	if err != nil {
		log.Println("UpdateTournament:", err)
	}
}
```

- [ ] **Step 2: Add `UpdateDivision` to `mydb/divisions.go`**

Append after `DelDivision`:

```go
func (me *MyDB) UpdateDivision(id int, name string) {
	_, err := me.DB.Exec(
		`UPDATE divisions SET name=$1 WHERE id=$2`,
		name, id,
	)
	if err != nil {
		log.Println("UpdateDivision:", err)
	}
}
```

- [ ] **Step 3: Add `UpdateTeam` and `ReturnTeamsByTournamentID` to `mydb/teams.go`**

`ReturnTeamsByTournamentID` is needed by the game edit form to populate team selects across all divisions. Append both after `DelTeam`:

```go
func (me *MyDB) ReturnTeamsByTournamentID(tournamentID int) []Team {
	rows, err := me.DB.Query(
		`SELECT id, tournament_id, name, coach, division_id FROM teams WHERE tournament_id=$1 ORDER BY name`,
		tournamentID,
	)
	if err != nil {
		log.Println("ReturnTeamsByTournamentID:", err)
		return nil
	}
	var teams []Team
	for rows.Next() {
		var t Team
		var did int
		if err := rows.Scan(&t.ID, &t.TournamentID, &t.Name, &t.Coach, &did); err != nil {
			log.Println("ReturnTeamsByTournamentID scan:", err)
			continue
		}
		t.Division = me.ReturnDivisionByID(did)
		teams = append(teams, t)
	}
	rows.Close()
	return teams
}

func (me *MyDB) UpdateTeam(id, divisionID int, name, coach string) {
	_, err := me.DB.Exec(
		`UPDATE teams SET division_id=$1, name=$2, coach=$3 WHERE id=$4`,
		divisionID, name, coach, id,
	)
	if err != nil {
		log.Println("UpdateTeam:", err)
	}
}
```

- [ ] **Step 4: Add `UpdateGame` to `mydb/games.go`**

`UpdateGame` updates the game row and, if the game was already scored, re-syncs `games_by_team` so standings remain correct after team reassignment. Append after `DelGame`:

```go
func (me *MyDB) UpdateGame(id, divisionID, homeTeamID, awayTeamID int, location, startTime, umpire string) {
	_, err := me.DB.Exec(
		`UPDATE games SET division_id=$1, home_team_id=$2, away_team_id=$3, location=$4, start_time=$5, umpire=$6 WHERE id=$7`,
		divisionID, homeTeamID, awayTeamID, location, startTime, umpire, id,
	)
	if err != nil {
		log.Println("UpdateGame:", err)
		return
	}
	// Re-fetch to get updated team/division references before re-syncing scores.
	game := me.ReturnGameByID(id)
	if game.Scored {
		me.DeleteTeamScore(id)
		me.AddTeamScore(game.Division.TournamentID, game.Division.ID, game.HomeTeam.ID, game.AwayTeam.ID, game.ID, game.HomeScore, game.AwayScore)
		me.AddTeamScore(game.Division.TournamentID, game.Division.ID, game.AwayTeam.ID, game.HomeTeam.ID, game.ID, game.AwayScore, game.HomeScore)
	}
}
```

- [ ] **Step 5: Verify build**

```bash
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 6: Commit**

```bash
git add mydb/tournaments.go mydb/divisions.go mydb/teams.go mydb/games.go
git commit -m "feat: add Update* functions and ReturnTeamsByTournamentID to mydb"
```

---

### Task 2: Data structs + Tournament edit

**Files:**
- Modify: `webhandler/templates.go`
- Modify: `webhandler/tournaments.go`
- Create: `webhandler/templates/admin/edit_tournament.html`
- Modify: `webhandler/templates/admin/tournament_view.html`
- Modify: `main.go`

- [ ] **Step 1: Add edit data structs to `webhandler/templates.go`**

Append after `adminGamesData` (before the `render` function):

```go
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
```

- [ ] **Step 2: Add `EditTournament` to `webhandler/tournaments.go`**

Append after `AdminTournamentView`. The tournament data is in `baseData.Tournament` so no extra struct is needed.

```go
func (me *Env) EditTournament(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		sport := r.FormValue("sport")
		location := r.FormValue("location")
		notes := r.FormValue("notes")
		date, err := time.Parse("2006-01-02", r.FormValue("start_date"))
		if err != nil {
			http.Error(w, "Invalid date format (expected YYYY-MM-DD)", http.StatusBadRequest)
			return
		}
		if name == "" || sport == "" || location == "" {
			http.Error(w, "Name, sport, and location are required", http.StatusBadRequest)
			return
		}
		me.DB.UpdateTournament(t.ID, name, sport, location, notes, date)
		http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "editTournament", newBaseWithTournament(r, true, t))
}
```

- [ ] **Step 3: Create `webhandler/templates/admin/edit_tournament.html`**

```html
{{define "editTournament"}}
{{template "header" .}}
<h1>Edit Tournament</h1>
<form method="post" action="/admin/tournaments/{{.Tournament.ID}}/edit">
<table>
  <tr><td>Name</td><td><input type="text" name="name" value="{{.Tournament.Name}}"></td></tr>
  <tr><td>Sport</td><td><input type="text" name="sport" value="{{.Tournament.Sport}}"></td></tr>
  <tr><td>Location</td><td><input type="text" name="location" value="{{.Tournament.Location}}"></td></tr>
  <tr><td>Start Date</td><td><input type="date" name="start_date" value="{{.Tournament.StartDate.Format "2006-01-02"}}"></td></tr>
  <tr><td>Notes</td><td><textarea name="notes">{{.Tournament.Notes}}</textarea></td></tr>
  <tr><td></td><td><input type="submit" value="Save"></td></tr>
</table>
{{.CSRFField}}
</form>
<p><a href="/admin/tournaments/{{.Tournament.ID}}">Cancel</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 4: Add Edit link to `webhandler/templates/admin/tournament_view.html`**

Replace the current `<ul>` block:

```html
<ul>
  <li><a href="/admin/tournaments/{{.Tournament.ID}}/divisions">Manage Divisions</a></li>
  <li><a href="/admin/tournaments/{{.Tournament.ID}}/teams">Manage Teams</a></li>
  <li><a href="/admin/tournaments/{{.Tournament.ID}}/games">Manage Games</a></li>
  <li><a href="/admin/tournaments/{{.Tournament.ID}}/edit">Edit Tournament</a></li>
</ul>
```

- [ ] **Step 5: Register routes in `main.go`**

After the line `router.GET("/admin/tournaments/:tid/games/:gid/delete", wh.DelGame)`, add:

```go
router.GET("/admin/tournaments/:tid/edit", wh.EditTournament)
router.POST("/admin/tournaments/:tid/edit", wh.EditTournament)
```

- [ ] **Step 6: Verify build**

```bash
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 7: Commit**

```bash
git add webhandler/templates.go webhandler/tournaments.go \
        webhandler/templates/admin/edit_tournament.html \
        webhandler/templates/admin/tournament_view.html \
        main.go
git commit -m "feat: add tournament edit form and handler"
```

---

### Task 3: Division edit

**Files:**
- Modify: `webhandler/divisions.go`
- Create: `webhandler/templates/admin/edit_division.html`
- Modify: `webhandler/templates/admin/division_view.html`
- Modify: `main.go`

- [ ] **Step 1: Add `EditDivision` to `webhandler/divisions.go`**

Append after `AdminDivisionView`:

```go
func (me *Env) EditDivision(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad Division ID", http.StatusBadRequest)
		return
	}
	division := me.DB.ReturnDivisionByID(did)
	if division.ID == 0 {
		http.Error(w, "Division not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		if name == "" {
			http.Error(w, "Division name required", http.StatusBadRequest)
			return
		}
		me.DB.UpdateDivision(did, name)
		http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/divisions/%d", t.ID, did), http.StatusSeeOther)
		return
	}
	me.render(w, "editDivision", editDivisionData{
		baseData: newBaseWithTournament(r, true, t),
		Division: division,
	})
}
```

- [ ] **Step 2: Create `webhandler/templates/admin/edit_division.html`**

```html
{{define "editDivision"}}
{{template "header" .}}
<h1>Edit Division</h1>
<form method="post" action="/admin/tournaments/{{.Tournament.ID}}/divisions/{{.Division.ID}}/edit">
<table>
  <tr><td>Name</td><td><input type="text" name="name" value="{{.Division.Name}}"></td></tr>
  <tr><td></td><td><input type="submit" value="Save"></td></tr>
</table>
{{.CSRFField}}
</form>
<p><a href="/admin/tournaments/{{.Tournament.ID}}/divisions/{{.Division.ID}}">Cancel</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Add Edit Division link to `webhandler/templates/admin/division_view.html`**

After the `<H1>{{.Division.Name}}</H1>` line, add:

```html
<p><a href="/admin/tournaments/{{.Tournament.ID}}/divisions/{{.DivisionID}}/edit">Edit Division</a></p>
```

- [ ] **Step 4: Add Edit Game link per row in `division_view.html`**

In the Games table header, add an `<th>Edit</th>` column after `<th>Score Game</th>`:

```html
<tr><th>Home Team</th><th>Away Team</th><th>Location</th><th>Start time</th><th>Umpire</th><th>Score Game</th><th>Edit</th>{{if not $.DisableDelete}}<th>Delete Game</th>{{end}}<th>Home Score</th><th>Away Score</th></tr>
```

In each game row, add an Edit cell after the Score Game cell:

```html
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/score">Score Game</a></td>
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/edit">Edit</a></td>
  {{if not $.DisableDelete}}<td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/delete">Delete Game</a></td>{{end}}
```

- [ ] **Step 5: Register routes in `main.go`**

After the tournament edit routes added in Task 2, add:

```go
router.GET("/admin/tournaments/:tid/divisions/:did/edit", wh.EditDivision)
router.POST("/admin/tournaments/:tid/divisions/:did/edit", wh.EditDivision)
```

- [ ] **Step 6: Verify build**

```bash
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 7: Commit**

```bash
git add webhandler/divisions.go \
        webhandler/templates/admin/edit_division.html \
        webhandler/templates/admin/division_view.html \
        main.go
git commit -m "feat: add division edit form and handler"
```

---

### Task 4: Team edit

**Files:**
- Modify: `webhandler/teams.go`
- Create: `webhandler/templates/admin/edit_team.html`
- Modify: `webhandler/templates/admin/teams.html`
- Modify: `main.go`

- [ ] **Step 1: Add `EditTeam` to `webhandler/teams.go`**

Append after `DeleteTeam`:

```go
func (me *Env) EditTeam(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		http.Error(w, "Bad Team ID", http.StatusBadRequest)
		return
	}
	team := me.DB.ReturnTeamByID(teamID)
	if team.ID == 0 {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("teamname")
		coach := r.FormValue("teamcoach")
		divisionID, err := strconv.Atoi(r.FormValue("division"))
		if err != nil || name == "" {
			http.Error(w, "Name and valid division required", http.StatusBadRequest)
			return
		}
		me.DB.UpdateTeam(teamID, divisionID, name, coach)
		http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/teams", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "editTeam", editTeamData{
		baseData:  newBaseWithTournament(r, true, t),
		Team:      team,
		Divisions: me.DB.ReturnDivisions(t.ID),
	})
}
```

- [ ] **Step 2: Create `webhandler/templates/admin/edit_team.html`**

The `selected` attribute is applied when the option's ID matches the team's current division ID.

```html
{{define "editTeam"}}
{{template "header" .}}
<h1>Edit Team</h1>
<form method="post" action="/admin/tournaments/{{.Tournament.ID}}/teams/{{.Team.ID}}/edit">
<table>
  <tr><td>Name</td><td><input type="text" name="teamname" value="{{.Team.Name}}"></td></tr>
  <tr><td>Coach</td><td><input type="text" name="teamcoach" value="{{.Team.Coach}}"></td></tr>
  <tr><td>Division</td><td>
    <select name="division">
    {{range .Divisions}}
    <option value="{{.ID}}"{{if eq .ID $.Team.Division.ID}} selected{{end}}>{{.Name}}</option>
    {{end}}
    </select>
  </td></tr>
  <tr><td></td><td><input type="submit" value="Save"></td></tr>
</table>
{{.CSRFField}}
</form>
<p><a href="/admin/tournaments/{{.Tournament.ID}}/teams">Cancel</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Add Edit link per team row in `webhandler/templates/admin/teams.html`**

In each team row inside `{{range index $.TeamsByDivision .ID}}`, add an Edit link cell after the team name/coach cells and before the delete form cell:

```html
{{range index $.TeamsByDivision .ID}}
<tr>
  <td>{{.Name}}</td>
  <td>{{.Coach}}</td>
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/teams/{{.ID}}/edit">Edit</a></td>
  <td>
  {{if not $.DisableDelete}}
  <form method="post" action="/admin/tournaments/{{$.Tournament.ID}}/teams/delete">
    <input type="hidden" name="teamid" value="{{.ID}}">
    {{$.CSRFField}}
    <input type="submit" name="Delete" value="Delete">
  </form>
  {{end}}
  </td>
</tr>
{{end}}
```

- [ ] **Step 4: Register routes in `main.go`**

After the division edit routes, add:

```go
router.GET("/admin/tournaments/:tid/teams/:teamid/edit", wh.EditTeam)
router.POST("/admin/tournaments/:tid/teams/:teamid/edit", wh.EditTeam)
```

- [ ] **Step 5: Verify build**

```bash
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 6: Commit**

```bash
git add webhandler/teams.go \
        webhandler/templates/admin/edit_team.html \
        webhandler/templates/admin/teams.html \
        main.go
git commit -m "feat: add team edit form and handler"
```

---

### Task 5: Game edit

**Files:**
- Modify: `webhandler/webhandler.go`
- Create: `webhandler/templates/admin/edit_game.html`
- Modify: `webhandler/templates/admin/games.html`
- Modify: `main.go`

- [ ] **Step 1: Add `EditGame` to `webhandler/webhandler.go`**

Append after `CreateGameSubmit`:

```go
func (me *Env) EditGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, ps)
	if !ok {
		return
	}
	gid, err := strconv.Atoi(ps.ByName("gid"))
	if err != nil {
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
```

- [ ] **Step 2: Create `webhandler/templates/admin/edit_game.html`**

Both team selects pre-select the current home/away team. The division select pre-selects the current division.

```html
{{define "editGame"}}
{{template "header" .}}
<h1>Edit Game</h1>
<form method="post" action="/admin/tournaments/{{.Tournament.ID}}/games/{{.Game.ID}}/edit">
<table>
  <tr><td>Division</td><td>
    <select name="divisionid">
    {{range .Divisions}}
    <option value="{{.ID}}"{{if eq .ID $.Game.Division.ID}} selected{{end}}>{{.Name}}</option>
    {{end}}
    </select>
  </td></tr>
  <tr><td>Home Team</td><td>
    <select name="hometeam">
    {{range .Teams}}
    <option value="{{.ID}}"{{if eq .ID $.Game.HomeTeam.ID}} selected{{end}}>{{.Name}} - {{.Coach}}</option>
    {{end}}
    </select>
  </td></tr>
  <tr><td>Away Team</td><td>
    <select name="awayteam">
    {{range .Teams}}
    <option value="{{.ID}}"{{if eq .ID $.Game.AwayTeam.ID}} selected{{end}}>{{.Name}} - {{.Coach}}</option>
    {{end}}
    </select>
  </td></tr>
  <tr><td>Location</td><td><input type="text" name="location" value="{{.Game.Location}}"></td></tr>
  <tr><td>Date/Time</td><td><input type="text" name="datetime" value="{{.Game.Start}}"></td></tr>
  <tr><td>Umpire</td><td><input type="text" name="umpire" value="{{.Game.Umpire}}"></td></tr>
  <tr><td></td><td><input type="submit" value="Save"></td></tr>
</table>
{{.CSRFField}}
</form>
<p><a href="/admin/tournaments/{{.Tournament.ID}}/games">Cancel</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Add Edit link per game row in `webhandler/templates/admin/games.html`**

Add `<th>Edit</th>` to the header row after `<th>Score Game</th>`:

```html
<tr><th>Home Team</th><th>Home Score</th><th>Away Team</th><th>Away Score</th><th>Location</th><th>Start Time</th><th>Umpire</th><th>Score Game</th><th>Edit</th>{{if not $.DisableDelete}}<th>Delete</th>{{end}}</tr>
```

Add the Edit cell after the Score Game cell in each row:

```html
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/score">Score Game</a></td>
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/edit">Edit</a></td>
  {{if not $.DisableDelete}}<td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/delete">Delete</a></td>{{end}}
```

- [ ] **Step 4: Register routes in `main.go`**

After the team edit routes, add:

```go
router.GET("/admin/tournaments/:tid/games/:gid/edit", wh.EditGame)
router.POST("/admin/tournaments/:tid/games/:gid/edit", wh.EditGame)
```

- [ ] **Step 5: Verify build and vet**

```bash
go build ./... && go vet ./...
```

Expected: no output, exit 0.

- [ ] **Step 6: Manual smoke test**

Start the server with a valid PostgreSQL config. Verify each edit flow:

1. **Tournament**: Navigate to `/admin/tournaments`, click a tournament, click "Edit Tournament" — form pre-fills name/sport/location/date/notes. Submit change — redirects to admin tournament view with updated values.
2. **Division**: Navigate to a division admin view, click "Edit Division" — form pre-fills name. Submit — redirects back with updated name.
3. **Team**: Navigate to `/admin/tournaments/:tid/teams`, click "Edit" on a team row — form pre-fills name, coach, and selects current division. Submit — redirects to teams list with updated values.
4. **Game**: Navigate to `/admin/tournaments/:tid/games`, click "Edit" on a game row — form pre-fills all fields with current home/away teams pre-selected. Submit — redirects to games list. If game was scored, verify standings still reflect correct team assignments.

- [ ] **Step 7: Commit**

```bash
git add webhandler/webhandler.go \
        webhandler/templates/admin/edit_game.html \
        webhandler/templates/admin/games.html \
        main.go
git commit -m "feat: add game edit form and handler"
```
