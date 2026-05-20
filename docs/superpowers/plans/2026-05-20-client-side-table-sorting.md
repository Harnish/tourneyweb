# Client-Side Table Sorting Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add click-to-sort column headers to all data tables across public and admin/director views, with no server round-trips and no new dependencies.

**Architecture:** A single vanilla-JS IIFE (`sort.js`) is embedded in the binary alongside `style.css` and served at `GET /sort.js`. Tables opt in with `class="tw-sortable"`; numeric columns carry `data-sort="number"` on their `<th>`. The script is included globally via `layout.html`.

**Tech Stack:** Go (`//go:embed`, `mime.TypeByExtension`), vanilla JavaScript (ES5-compatible), Go html/template

---

## File Map

| Action | File | Purpose |
|--------|------|---------|
| Create | `sort.js` | Vanilla-JS sort utility (project root, alongside `style.css`) |
| Modify | `main.go` | Embed `sort.js`, add `PrintJS` handler, register `GET /sort.js` |
| Modify | `webhandler/templates/layout.html` | Include `<script src="/sort.js" defer></script>` |
| Modify | `webhandler/templates/divisions.html` | Standings table + games table |
| Modify | `webhandler/templates/games.html` | All-games table |
| Modify | `webhandler/templates/team.html` | Games table + roster table |
| Modify | `webhandler/templates/admin/games.html` | Admin games table |
| Modify | `webhandler/templates/admin/division_view.html` | Teams table + games table |
| Modify | `webhandler/templates/admin/teams.html` | Per-division team tables (add headers) |
| Modify | `webhandler/templates/manage/teams.html` | Per-division team tables |
| Modify | `webhandler/templates/manage/roster.html` | Roster table |
| Modify | `webhandler/templates/admin/locations.html` | Locations table |
| Modify | `webhandler/templates/admin/queue.html` | Queue table |

---

### Task 1: sort.js file, serving infrastructure, and global include

**Files:**
- Create: `sort.js` (project root)
- Modify: `main.go`
- Modify: `webhandler/templates/layout.html`

- [ ] **Step 1: Create `sort.js` in the project root**

```javascript
(function () {
  function makeSortable(table) {
    var ths = Array.from(table.querySelectorAll('tr:first-child th'));
    if (!ths.length) return;
    ths.forEach(function (th) { th._label = th.textContent.trim(); });

    var activeCol = -1, ascending = true;

    ths.forEach(function (th, col) {
      th.style.cursor = 'pointer';
      th.addEventListener('click', function () {
        if (activeCol === col) {
          ascending = !ascending;
        } else {
          activeCol = col;
          ascending = true;
        }
        var isNum = th.dataset.sort === 'number';
        var rows = Array.from(table.rows).slice(1);
        rows.sort(function (a, b) {
          var av = a.cells[col] ? a.cells[col].textContent.trim() : '';
          var bv = b.cells[col] ? b.cells[col].textContent.trim() : '';
          if (isNum) return (parseFloat(av) || 0) - (parseFloat(bv) || 0);
          return av.localeCompare(bv, undefined, {sensitivity: 'base'});
        });
        if (!ascending) rows.reverse();
        rows.forEach(function (r) { r.parentNode.appendChild(r); });
        ths.forEach(function (h) { h.textContent = h._label; });
        th.textContent = th._label + (ascending ? ' ↑' : ' ↓');
      });
    });
  }

  document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('table.tw-sortable').forEach(makeSortable);
  });
}());
```

- [ ] **Step 2: Add embed variable and `PrintJS` handler to `main.go`**

After the existing `//go:embed style.css` block (around line 25-26), add:

```go
//go:embed sort.js
var defaultJS []byte
```

After the existing `PrintCSS` function (around line 212-215), add:

```go
func PrintJS(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-type", mime.TypeByExtension(".js"))
	w.Write(defaultJS)
}
```

- [ ] **Step 3: Register the route in `main.go`**

After the existing `router.GET("/style.css", PrintCSS)` line (around line 52), add:

```go
router.GET("/sort.js", PrintJS)
```

- [ ] **Step 4: Add script tag to `webhandler/templates/layout.html`**

After the last existing `<script>` tag (the Bootstrap JS `<script>` on line 12), add:

```html
<script src="/sort.js" defer></script>
```

The `<head>` section should end up looking like:

```html
<script src="https://stackpath.bootstrapcdn.com/bootstrap/4.5.0/js/bootstrap.min.js" ...></script>
<script src="/sort.js" defer></script>
</head>
```

- [ ] **Step 5: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no output, exit code 0.

- [ ] **Step 6: Verify `/sort.js` is served**

```bash
./tourneyweb &
sleep 1
curl -s http://localhost:8989/sort.js | head -3
kill %1
```

Expected: first line is `(function () {`

- [ ] **Step 7: Commit**

```bash
git add sort.js main.go webhandler/templates/layout.html
git commit -m "feat: add sort.js embed and global script include"
```

---

### Task 2: Mark public templates as sortable

**Files:**
- Modify: `webhandler/templates/divisions.html`
- Modify: `webhandler/templates/games.html`
- Modify: `webhandler/templates/team.html`

#### `divisions.html` — Standings table

The standings table currently opens with `<table>`. Change it to:

```html
<table class="tw-sortable">
<tr><th data-sort="number">Rank</th><th>Team Name</th><th>Coach</th><th data-sort="number">Wins</th><th data-sort="number">Losses</th><th data-sort="number">Runs Against</th><th data-sort="number">Runs For</th><th data-sort="number">Games Played</th></tr>
```

#### `divisions.html` — Games table

The games table currently opens with `<table>`. Change it to:

```html
<table class="tw-sortable">
<tr><th>Home Team</th><th>Away Team</th><th>Location</th><th>Start time</th><th>Umpire</th><th data-sort="number">Home Team Score</th><th data-sort="number">Away Team Score</th></tr>
```

#### `games.html`

The table currently opens with `<table border="1" cellpadding="1" cellspacing="0">`. Change it to:

```html
<table border="1" cellpadding="1" cellspacing="0" class="tw-sortable">
```

Add `data-sort="number"` to the two score `<th>` elements. The full header row should be:

```html
<tr><th>Division</th><th>Home Team</th><th data-sort="number">Home Team Score</th><th>Away Team</th><th data-sort="number">Away Team Score</th><th>Location</th><th>Start Time</th><th>Umpire</th>{{if $.User.CanScore $.Tournament.ID}}<th></th>{{end}}</tr>
```

#### `team.html` — Games table

The games table currently opens with `<table>`. This table has no scores in it (just Home/Away/Location/Start time/Umpire) — it's text-only sort. Change it to:

```html
<table class="tw-sortable">
```

The header row is unchanged (all text columns).

#### `team.html` — Roster table

The roster table currently opens with `<table>`. Change it to:

```html
<table class="tw-sortable">
```

Add `data-sort="number"` to the `#` header. The full header row:

```html
<tr><th data-sort="number">#</th><th>Name</th><th>Handed</th><th>Position</th></tr>
```

- [ ] **Step 1: Apply all changes to `divisions.html`**

Full file after edits (only the two table opening tags and their header rows change):

```html
{{define "divisions"}}
{{template "header" .}}
<H1>{{.Division.Name}}</H1>
{{if eq .Division.Phase "bracket"}}
<p><a href="/tournaments/{{.Tournament.ID}}/divisions/{{.Division.ID}}/bracket">View Bracket →</a></p>
{{end}}
<h2>Teams</h2>
<table class="tw-sortable">
<tr><th data-sort="number">Rank</th><th>Team Name</th><th>Coach</th><th data-sort="number">Wins</th><th data-sort="number">Losses</th><th data-sort="number">Runs Against</th><th data-sort="number">Runs For</th><th data-sort="number">Games Played</th></tr>
{{range $i, $row := .Teams}}
<tr>
  <td>{{inc $i}}</td>
  <td><a href="/tournaments/{{$.Tournament.ID}}/teams/{{$row.ID}}">{{$row.Name}}</a></td>
  <td>{{$row.Coach}}</td>
  <td>{{$row.Wins}}</td>
  <td>{{$row.Losses}}</td>
  <td>{{$row.RunsAgainst}}</td>
  <td>{{$row.RunsFor}}</td>
  <td>{{$row.GamesPlayed}}</td>
</tr>
{{end}}
</table>
{{if .RankingLabel}}
<p style="font-size:0.85em;color:#888;margin-top:0.25em;">Ranked by: {{.RankingLabel}}</p>
{{end}}
<h2>Games</h2>
<table class="tw-sortable">
<tr><th>Home Team</th><th>Away Team</th><th>Location</th><th>Start time</th><th>Umpire</th><th data-sort="number">Home Team Score</th><th data-sort="number">Away Team Score</th></tr>
{{range .Games}}
<tr>
  <td>{{.HomeTeam.Name}} {{.HomeTeam.Coach}}{{if and .ScrimmageTeamID (eq .ScrimmageTeamID .HomeTeam.ID)}} <em style="color:#888;font-size:0.85em;">(non-counting)</em>{{end}}</td>
  <td>{{.AwayTeam.Name}} {{.AwayTeam.Coach}}{{if and .ScrimmageTeamID (eq .ScrimmageTeamID .AwayTeam.ID)}} <em style="color:#888;font-size:0.85em;">(non-counting)</em>{{end}}</td>
  <td>{{.Location}}</td>
  <td>{{formatTime .Start}}</td>
  <td>{{.Umpire}}</td>
  <td>{{.HomeScore}}</td>
  <td>{{.AwayScore}}</td>
</tr>
{{end}}
</table>
{{if .Division.RulesHTML}}
<hr>
<h2>Rules</h2>
{{.Division.RulesHTML | htmlSafe}}
{{else if .Tournament.RulesHTML}}
<hr>
<h2>Rules</h2>
<p><em>Tournament rules (no division-specific rules set)</em></p>
{{.Tournament.RulesHTML | htmlSafe}}
{{end}}
{{template "footer" .}}
{{end}}
```

- [ ] **Step 2: Apply changes to `games.html`**

```html
{{define "games"}}
{{template "header" .}}
<table border="1" cellpadding="1" cellspacing="0" class="tw-sortable">
<tr><th>Division</th><th>Home Team</th><th data-sort="number">Home Team Score</th><th>Away Team</th><th data-sort="number">Away Team Score</th><th>Location</th><th>Start Time</th><th>Umpire</th>{{if $.User.CanScore $.Tournament.ID}}<th></th>{{end}}</tr>
{{range .Games}}
<tr>
  <td>{{.Division.Name}}</td>
  <td>{{.HomeTeam.Name}} - {{.HomeTeam.Coach}}{{if and .ScrimmageTeamID (eq .ScrimmageTeamID .HomeTeam.ID)}} <em style="color:#888;font-size:0.85em;">(non-counting)</em>{{end}}</td>
  <td>{{.HomeScore}}</td>
  <td>{{.AwayTeam.Name}} - {{.AwayTeam.Coach}}{{if and .ScrimmageTeamID (eq .ScrimmageTeamID .AwayTeam.ID)}} <em style="color:#888;font-size:0.85em;">(non-counting)</em>{{end}}</td>
  <td>{{.AwayScore}}</td>
  <td>{{.Location}}</td>
  <td>{{formatTime .Start}}</td>
  <td>{{.Umpire}}</td>
  {{if $.User.CanScore $.Tournament.ID}}<td><a href="/tournaments/{{$.Tournament.ID}}/score/games/{{.ID}}">Score</a></td>{{end}}
</tr>
{{end}}
</table>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Apply changes to `team.html`**

```html
{{define "team"}}
{{template "header" .}}
<br>{{.Team.Name}} {{.Team.Coach}} {{.Team.Division.Name}}<br>
<table class="tw-sortable">
<tr><th>Home</th><th>Away</th><th>Location</th><th>Start time</th><th>Umpire</th></tr>
{{range .Games}}
<tr>
  <td>{{.HomeTeam.Name}}{{if and .ScrimmageTeamID (eq .ScrimmageTeamID .HomeTeam.ID)}} <em style="color:#888;font-size:0.85em;">(non-counting)</em>{{end}}</td>
  <td>{{.AwayTeam.Name}}{{if and .ScrimmageTeamID (eq .ScrimmageTeamID .AwayTeam.ID)}} <em style="color:#888;font-size:0.85em;">(non-counting)</em>{{end}}</td>
  <td>{{.Location}}</td>
  <td>{{formatTime .Start}}</td>
  <td>{{.Umpire}}</td>
</tr>
{{end}}
</table>
{{if .Players}}
<h2>Roster</h2>
<table class="tw-sortable">
<tr><th data-sort="number">#</th><th>Name</th><th>Handed</th><th>Position</th></tr>
{{range .Players}}
<tr>
  <td>{{.Number}}</td>
  <td>{{.DisplayName}}</td>
  <td>{{.Handed}}</td>
  <td>{{.Position}}</td>
</tr>
{{end}}
</table>
{{end}}
{{template "footer" .}}
{{end}}
```

- [ ] **Step 4: Build to verify no template/compile errors**

```bash
go build ./...
```

Expected: no output, exit code 0.

- [ ] **Step 5: Commit**

```bash
git add webhandler/templates/divisions.html webhandler/templates/games.html webhandler/templates/team.html
git commit -m "feat: add tw-sortable to public tables"
```

---

### Task 3: Mark admin/director templates as sortable

**Files:**
- Modify: `webhandler/templates/admin/games.html`
- Modify: `webhandler/templates/admin/division_view.html`
- Modify: `webhandler/templates/admin/teams.html`
- Modify: `webhandler/templates/manage/teams.html`
- Modify: `webhandler/templates/manage/roster.html`
- Modify: `webhandler/templates/admin/locations.html`
- Modify: `webhandler/templates/admin/queue.html`

- [ ] **Step 1: Apply changes to `admin/games.html`**

Add `class="tw-sortable"` to the table and `data-sort="number"` to the two score `<th>` elements:

```html
{{define "adminGames"}}
{{template "header" .}}
<table border="1" cellpadding="1" cellspacing="0" class="tw-sortable">
<tr><th>Home Team</th><th data-sort="number">Home Score</th><th>Away Team</th><th data-sort="number">Away Score</th><th>Location</th><th>Start Time</th><th>Umpire</th><th>Score Game</th><th>Edit</th>{{if not $.DisableDelete}}<th>Delete</th>{{end}}</tr>
{{range .Games}}
<tr>
  <td>{{.HomeTeam.Name}} - {{.HomeTeam.Coach}}{{if and .ScrimmageTeamID (eq .ScrimmageTeamID .HomeTeam.ID)}} <em style="color:#888;font-size:0.85em;">(non-counting)</em>{{end}}</td>
  <td>{{.HomeScore}}</td>
  <td>{{.AwayTeam.Name}} - {{.AwayTeam.Coach}}{{if and .ScrimmageTeamID (eq .ScrimmageTeamID .AwayTeam.ID)}} <em style="color:#888;font-size:0.85em;">(non-counting)</em>{{end}}</td>
  <td>{{.AwayScore}}</td>
  <td>{{.Location}}</td>
  <td>{{formatTime .Start}}</td>
  <td>{{.Umpire}}</td>
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/score">Score Game</a></td>
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/edit">Edit</a></td>
  {{if not $.DisableDelete}}<td>
    <form method="post" action="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/delete" onsubmit="return confirm('Delete this game?')">
      {{$.CSRFField}}
      <input type="submit" value="Delete">
    </form>
  </td>{{end}}
</tr>
{{end}}
</table>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 2: Apply changes to `admin/division_view.html`**

Teams table: add `tw-sortable` and `data-sort="number"` on Wins and Losses.
Games table: add `tw-sortable` and `data-sort="number"` on Home Score and Away Score.

```html
{{define "adminDivisionView"}}
{{template "header" .}}
<H1>{{.Division.Name}}</H1>
<p><a href="/admin/tournaments/{{.Tournament.ID}}/divisions/{{.DivisionID}}/edit">Edit Division</a></p>
<table class="tw-sortable">
<tr><th>Team Name</th><th>Coach</th><th data-sort="number">Wins</th><th data-sort="number">Losses</th></tr>
{{range .Teams}}
<tr>
  <td>{{.Name}}</td>
  <td>{{.Coach}}</td>
  <td>{{.Wins}}</td>
  <td>{{.Losses}}</td>
</tr>
{{end}}
</table>
<br>
<form action="/admin/tournaments/{{.Tournament.ID}}/teams"><input type="submit" value="Add Team"></form>
<br>
<h2>Games</h2>
<table class="tw-sortable">
<tr><th>Home Team</th><th>Away Team</th><th>Location</th><th>Start time</th><th>Umpire</th><th>Score Game</th><th>Edit</th>{{if not $.DisableDelete}}<th>Delete Game</th>{{end}}<th data-sort="number">Home Score</th><th data-sort="number">Away Score</th></tr>
{{range .Games}}
<tr>
  <td>{{.HomeTeam.Name}} {{.HomeTeam.Coach}}{{if and .ScrimmageTeamID (eq .ScrimmageTeamID .HomeTeam.ID)}} <em style="color:#888;font-size:0.85em;">(non-counting)</em>{{end}}</td>
  <td>{{.AwayTeam.Name}} {{.AwayTeam.Coach}}{{if and .ScrimmageTeamID (eq .ScrimmageTeamID .AwayTeam.ID)}} <em style="color:#888;font-size:0.85em;">(non-counting)</em>{{end}}</td>
  <td>{{.Location}}</td>
  <td>{{formatTime .Start}}</td>
  <td>{{.Umpire}}</td>
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/score">Score Game</a></td>
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/edit">Edit</a></td>
  {{if not $.DisableDelete}}<td><a href="/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/delete">Delete Game</a></td>{{end}}
  <td>{{.HomeScore}}</td>
  <td>{{.AwayScore}}</td>
</tr>
{{end}}
</table>
<br>
<form action="/admin/tournaments/{{.Tournament.ID}}/divisions/{{.DivisionID}}/games/new">
  <input type="submit" value="Add Game">
</form>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Apply changes to `admin/teams.html`**

The per-division data tables currently have no `<th>` headers. Add a header row and `tw-sortable`. The form table at the top (the Add Team form) is a layout table — leave it unchanged.

```html
{{define "adminTeams"}}
{{template "header" .}}
<h2>Add Team</h2>
<form method="post" action="/admin/tournaments/{{.Tournament.ID}}/teams">
<table>
<tr><td>Team Name</td><td><input type="text" name="teamname"></td></tr>
<tr><td>Team Coach</td><td><input type="text" name="teamcoach"></td></tr>
<tr><td>Division</td><td>
  <select name="division">
  {{range .Divisions}}
  <option value="{{.ID}}">{{.Name}}</option>
  {{end}}
  </select>
</td></tr>
<tr><td></td><td><input type="submit" name="submit"></td></tr>
</table>
{{.CSRFField}}
</form>
{{range .Divisions}}
<h2>{{.Name}}</h2>
<table class="tw-sortable">
<tr><th>Name</th><th>Coach</th><th></th>{{if not $.DisableDelete}}<th></th>{{end}}</tr>
{{range index $.TeamsByDivision .ID}}
<tr>
  <td>{{.Name}}</td>
  <td>{{.Coach}}</td>
  <td><a href="/admin/tournaments/{{$.Tournament.ID}}/teams/{{.ID}}/edit">Edit</a></td>
  <td>
  {{if not $.DisableDelete}}
  <form method="post" action="/admin/tournaments/{{$.Tournament.ID}}/teams/{{.ID}}/delete" onsubmit="return confirm('Delete this team?')">
    {{$.CSRFField}}
    <input type="submit" value="Delete">
  </form>
  {{end}}
  </td>
</tr>
{{end}}
</table>
{{end}}
{{template "footer" .}}
{{end}}
```

- [ ] **Step 4: Apply changes to `manage/teams.html`**

The per-division data tables already have `<th>` headers. Add `tw-sortable` to each one:

```html
{{define "manageTeams"}}
{{template "header" .}}
<h1>Teams — {{.Tournament.Name}}</h1>
<h2>Add Team</h2>
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/teams">
{{.CSRFField}}
<table>
  <tr><td>Division</td><td>
    <select name="division">
      {{range .Divisions}}<option value="{{.ID}}">{{.Name}}</option>{{end}}
    </select>
  </td></tr>
  <tr><td>Team Name</td><td><input type="text" name="teamname" required></td></tr>
  <tr><td>Coach</td><td><input type="text" name="teamcoach"></td></tr>
  <tr><td></td><td><input type="submit" value="Add Team"></td></tr>
</table>
</form>
<h2>Existing Teams</h2>
{{range .Divisions}}
  <h3>{{.Name}}</h3>
  {{$did := .ID}}
  {{$teams := index $.TeamsByDivision $did}}
  {{if $teams}}
  <table border="1" cellpadding="4" cellspacing="0" class="tw-sortable">
  <tr><th>Name</th><th>Coach</th><th>Edit</th><th>Roster</th>{{if not $.DisableDelete}}<th>Delete</th>{{end}}</tr>
  {{range $teams}}
  <tr>
    <td>{{.Name}}</td>
    <td>{{.Coach}}</td>
    <td><a href="/tournaments/{{$.Tournament.ID}}/manage/teams/{{.ID}}/edit">Edit</a></td>
    <td><a href="/tournaments/{{$.Tournament.ID}}/manage/teams/{{.ID}}/roster">Roster</a></td>
    {{if not $.DisableDelete}}<td>
      <form method="post" action="/tournaments/{{$.Tournament.ID}}/manage/teams/{{.ID}}/delete" onsubmit="return confirm('Delete this team?')">
        {{$.CSRFField}}
        <input type="submit" value="Delete">
      </form>
    </td>{{end}}
  </tr>
  {{end}}
  </table>
  {{else}}<p>No teams in this division.</p>{{end}}
{{end}}
<p><a href="/tournaments/{{.Tournament.ID}}/manage">Back to manage</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 5: Apply changes to `manage/roster.html`**

Add `tw-sortable` and `data-sort="number"` on the `#` column:

```html
{{define "roster"}}
{{template "header" .}}
<h1>Roster — {{.Team.Name}}</h1>
{{if .Players}}
<table border="1" cellpadding="4" cellspacing="0" class="tw-sortable">
<tr><th data-sort="number">#</th><th>Name</th><th>Handed</th><th>Position</th><th>Edit</th>{{if not $.DisableDelete}}<th>Delete</th>{{end}}</tr>
{{range .Players}}
<tr>
  <td>{{.Number}}</td>
  <td>{{.First}} {{.Last}}</td>
  <td>{{.Handed}}</td>
  <td>{{.Position}}</td>
  <td><a href="/tournaments/{{$.Tournament.ID}}/manage/teams/{{.TeamID}}/roster/{{.ID}}/edit">Edit</a></td>
  {{if not $.DisableDelete}}<td>
    <form method="post" action="/tournaments/{{$.Tournament.ID}}/manage/teams/{{.TeamID}}/roster/{{.ID}}/delete" onsubmit="return confirm('Delete this player?')">
      {{$.CSRFField}}
      <input type="submit" value="Delete">
    </form>
  </td>{{end}}
</tr>
{{end}}
</table>
{{else}}
<p>No players on roster yet.</p>
{{end}}
<p><a href="/tournaments/{{.Tournament.ID}}/manage/teams/{{.Team.ID}}/roster/new">Add Player</a></p>
<p><a href="/tournaments/{{.Tournament.ID}}/manage/teams">Back to Teams</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 6: Apply changes to `admin/locations.html`**

Add `tw-sortable` to the Locations data table only (not the Add Location form table at the top). Find the line that currently reads:

```html
<table border="1" cellpadding="4" cellspacing="0">
```

inside the `<h2>Locations</h2>` section (not the form table) and change it to:

```html
<table border="1" cellpadding="4" cellspacing="0" class="tw-sortable">
```

All other content in `admin/locations.html` stays unchanged.

- [ ] **Step 7: Apply changes to `admin/queue.html`**

Add `tw-sortable` to the drafts table. Find the line that currently reads:

```html
<table border="1" cellpadding="4" cellspacing="0">
```

inside the `{{if .Drafts}}` block and change it to:

```html
<table border="1" cellpadding="4" cellspacing="0" class="tw-sortable">
```

All other content in `admin/queue.html` stays unchanged.

- [ ] **Step 8: Build to verify no compile errors**

```bash
go build ./...
```

Expected: no output, exit code 0.

- [ ] **Step 9: Commit**

```bash
git add webhandler/templates/admin/games.html \
        webhandler/templates/admin/division_view.html \
        webhandler/templates/admin/teams.html \
        webhandler/templates/manage/teams.html \
        webhandler/templates/manage/roster.html \
        webhandler/templates/admin/locations.html \
        webhandler/templates/admin/queue.html
git commit -m "feat: add tw-sortable to admin and director tables"
```
