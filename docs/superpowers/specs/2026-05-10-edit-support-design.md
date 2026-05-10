# Edit Support Design

**Date:** 2026-05-10  
**Status:** Approved

## Problem

No entity in TourneyWeb can be edited after creation. Mistakes in tournament details, division names, team rosters, or game scheduling require deleting and recreating the record.

## Goal

Add edit forms for all four entity types: Tournaments, Divisions, Teams, and Games. Game edits cover all scheduling fields (division, teams, location, start time, umpire) but not scores — scores remain in the existing `ScoreGame` flow.

## Approach

Separate edit pages: each entity gets `GET /:id/edit` (pre-filled form) and `POST /:id/edit` (submit). This mirrors the existing create pattern and keeps templates simple. No inline editing, no generic dispatch handler.

## Routes

Eight new routes added to `main.go`. No httprouter conflicts — `edit` is a static segment after the entity param, distinct from existing static segments (`score`, `delete`, `delete`).

```
GET  /admin/tournaments/:tid/edit                     → EditTournament
POST /admin/tournaments/:tid/edit                     → EditTournament

GET  /admin/tournaments/:tid/divisions/:did/edit      → EditDivision
POST /admin/tournaments/:tid/divisions/:did/edit      → EditDivision

GET  /admin/tournaments/:tid/teams/:teamid/edit       → EditTeam
POST /admin/tournaments/:tid/teams/:teamid/edit       → EditTeam

GET  /admin/tournaments/:tid/games/:gid/edit          → EditGame
POST /admin/tournaments/:tid/games/:gid/edit          → EditGame
```

### Redirects after POST

| Entity     | Redirects to                                    |
|------------|-------------------------------------------------|
| Tournament | `/admin/tournaments/:tid`                       |
| Division   | `/admin/tournaments/:tid/divisions/:did`        |
| Team       | `/admin/tournaments/:tid/teams`                 |
| Game       | `/admin/tournaments/:tid/games`                 |

## mydb API

Four new functions, one per entity. All use `UPDATE … WHERE id=$N` with PostgreSQL `$N` placeholders and log errors. `tournamentID` is not updatable on child entities — moving across tournaments would orphan related data.

```go
// mydb/tournaments.go
func (me *MyDB) UpdateTournament(id int, name, sport, location, notes string, date time.Time)

// mydb/divisions.go
func (me *MyDB) UpdateDivision(id int, name string)

// mydb/teams.go
func (me *MyDB) UpdateTeam(id, divisionID int, name, coach string)

// mydb/games.go
// Does NOT update home_score / away_score — use ScoreGame for that.
func (me *MyDB) UpdateGame(id, divisionID, homeTeamID, awayTeamID int, location, startTime, umpire string)
```

## Handler Pattern

Each handler serves both GET and POST on the same route, matching the `AddDivisionForm` pattern:

```go
func (me *Env) EditTournament(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
    t, ok := me.tournamentFromRoute(w, ps)
    if !ok {
        return
    }
    if r.Method == http.MethodPost {
        r.ParseForm()
        // parse and validate fields
        // call me.DB.UpdateTournament(...)
        http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d", t.ID), http.StatusSeeOther)
        return
    }
    base := newBaseWithTournament(r, true, t)
    renderTemplate(w, "editTournament", base)
}
```

`EditTeam` and `EditGame` fetch supporting data (divisions list, teams list) needed to render the selects on the edit form.

## Data Structs

Added to `webhandler/templates.go`:

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
    Game  mydb.Game
    Teams []mydb.Team
}
```

Tournament edit uses no new struct — `baseData` already carries `Tournament` via `newBaseWithTournament`.

## Templates

Four new templates in `webhandler/templates/admin/`:

| File                    | Template name      | Pre-fills from          |
|-------------------------|--------------------|-------------------------|
| `edit_tournament.html`  | `editTournament`   | `.Tournament`           |
| `edit_division.html`    | `editDivision`     | `.Division`             |
| `edit_team.html`        | `editTeam`         | `.Team`, `.Divisions`   |
| `edit_game.html`        | `editGame`         | `.Game`, `.Teams`       |

All forms use `{{.CSRFField}}` and `method="POST"`. Date fields use `value='{{.Tournament.StartDate.Format "2006-01-02"}}'` to pre-fill the date input.

## Edit Links in Existing Templates

Edit links are added to existing admin templates:

| Template                           | Where                        | Link target                                          |
|------------------------------------|------------------------------|------------------------------------------------------|
| `admin/tournament_view.html`       | Tournament admin home        | `/admin/tournaments/{{.Tournament.ID}}/edit`         |
| `admin/division_view.html`         | Division admin view header   | `/admin/tournaments/{{$.Tournament.ID}}/divisions/{{.Division.ID}}/edit` |
| `admin/teams.html`                 | Per-team row                 | `/admin/tournaments/{{$.Tournament.ID}}/teams/{{.ID}}/edit` |
| `admin/games.html`                 | Per-game row                 | `/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/edit` |
| `admin/division_view.html`         | Per-game row in division     | `/admin/tournaments/{{$.Tournament.ID}}/games/{{.ID}}/edit` |

## File Summary

### New files
- `webhandler/templates/admin/edit_tournament.html`
- `webhandler/templates/admin/edit_division.html`
- `webhandler/templates/admin/edit_team.html`
- `webhandler/templates/admin/edit_game.html`

### Modified files
- `mydb/tournaments.go` — add `UpdateTournament`
- `mydb/divisions.go` — add `UpdateDivision`
- `mydb/teams.go` — add `UpdateTeam`
- `mydb/games.go` — add `UpdateGame`
- `webhandler/templates.go` — add `editDivisionData`, `editTeamData`, `editGameData`
- `webhandler/tournaments.go` — add `EditTournament`
- `webhandler/divisions.go` — add `EditDivision`
- `webhandler/teams.go` — add `EditTeam`
- `webhandler/webhandler.go` — add `EditGame`
- `webhandler/templates/admin/tournament_view.html` — add Edit Tournament link
- `webhandler/templates/admin/division_view.html` — add Edit Division link + Edit links per game row
- `webhandler/templates/admin/teams.html` — add Edit link per team row
- `webhandler/templates/admin/games.html` — add Edit link per game row
- `main.go` — register eight new routes

## Out of Scope

- Bulk editing
- Edit history / audit log
- Moving a division, team, or game to a different tournament
- Editing scores (handled by existing `ScoreGame` flow)
