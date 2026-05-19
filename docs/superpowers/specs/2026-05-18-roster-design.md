# Coach Roster Management Design

## Goal

Allow coaches, directors, and admins to manage player rosters for each team. Public team pages show a condensed roster (first initial + last name, number, handed, position). The feature is sport-agnostic — position is free text.

## Tech Stack

Go, PostgreSQL, html/template, no new dependencies.

---

## Section 1: Data Model

### `players` table

```sql
CREATE TABLE IF NOT EXISTS players (
    id        SERIAL PRIMARY KEY,
    team_id   INT  NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    number    TEXT,
    first_name TEXT NOT NULL,
    last_name  TEXT NOT NULL,
    handed    TEXT,   -- "L", "R", "S", or empty string
    position  TEXT
);
```

### `Player` struct (`mydb/players.go`)

```go
type Player struct {
    ID       int
    TeamID   int
    Number   string
    First    string
    Last     string
    Handed   string // "L", "R", "S", or ""
    Position string
}

func (p Player) DisplayName() string {
    return string([]rune(p.First)[:1]) + ". " + p.Last
}
```

`DisplayName()` returns "F. LastName" for public display. Empty First returns ". LastName" — callers must ensure First is non-empty before saving.

---

## Section 2: Routes & Permissions

Six routes, all under `/tournaments/:tid/manage/teams/:teamid/roster`:

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/tournaments/:tid/manage/teams/:teamid/roster` | `RosterList` | List players |
| GET | `/tournaments/:tid/manage/teams/:teamid/roster/new` | `RosterAdd` | Add form |
| POST | `/tournaments/:tid/manage/teams/:teamid/roster/new` | `RosterAdd` | Submit add |
| GET | `/tournaments/:tid/manage/teams/:teamid/roster/:pid/edit` | `RosterEdit` | Edit form |
| POST | `/tournaments/:tid/manage/teams/:teamid/roster/:pid/edit` | `RosterEdit` | Submit edit |
| POST | `/tournaments/:tid/manage/teams/:teamid/roster/:pid/delete` | `RosterDelete` | Delete player |

### Permission check (inline in each handler)

```go
allowed := u.IsAdmin || u.IsDirectorFor(tid) || u.IsCoachFor(teamID)
if !allowed { render 403 }
```

The existing `CanManage` middleware already gates `/tournaments/:tid/manage/*` to admins and directors. Coach access requires an **additional** check because coaches are not covered by that middleware — the handler must verify `u.IsCoachFor(teamID)` and allow them through even if `CanManage` would block them.

**Implementation note:** The `/tournaments/:tid/manage/teams/:teamid/roster*` routes must be registered **outside** the `CanManage` middleware group and use the inline permission check above, so coaches can reach them without being blocked at the middleware layer.

### "Manage Roster" link

Added to the manage teams list template, one per team row.

---

## Section 3: DB Interface

### New methods on `mydb.DB`

```go
AddPlayer(teamID int, number, first, last, handed, position string) int
GetPlayersByTeamID(teamID int) []Player
GetPlayerByID(id int) (Player, bool)
UpdatePlayer(id int, number, first, last, handed, position string)
DeletePlayer(id int)
```

### Migration

Added to `pgmigrations` in `mydb/mydb.go`:

```sql
CREATE TABLE IF NOT EXISTS players (
    id         SERIAL PRIMARY KEY,
    team_id    INT  NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    number     TEXT,
    first_name TEXT NOT NULL,
    last_name  TEXT NOT NULL,
    handed     TEXT,
    position   TEXT
);
```

### Files

| File | Change |
|------|--------|
| `mydb/players.go` | New file: `Player` struct, `DisplayName()`, 5 DB methods |
| `mydb/db.go` | Add 5 player methods to interface |
| `mydb/fakedb.go` | Implement 5 player methods using in-memory map |
| `mydb/mydb.go` | Add `players` table migration |

---

## Section 4: Templates

### `webhandler/templates/manage/roster.html`

Manage view — lists players in a table with edit and delete controls:

| # | Name | Handed | Position | Edit | Delete |
|---|------|--------|----------|------|--------|

- Delete uses a POST form with `confirm()` dialog (same pattern as game/team deletes).
- "Add Player" button at the bottom links to the `/roster/new` route.
- Page title: team name + " — Roster".

### `webhandler/templates/manage/edit_player.html`

Shared for add and edit (hidden `id` field; empty `id` = add):

| Field | Input | Notes |
|-------|-------|-------|
| Jersey # | `<input type="text">` | Optional |
| First Name | `<input type="text" required>` | Required |
| Last Name | `<input type="text" required>` | Required |
| Handed | `<select>`: — / L / R / S | Optional |
| Position | `<input type="text">` | Optional, free text |

Submit button label: "Save Player".

### `webhandler/templates/team.html` addition

Appended after existing team content. Rendered only when `len(.Players) > 0`:

```
Roster
| # | Name | Handed | Position |
```

Uses `DisplayName()` (first initial + last name). No edit links.

### `webhandler/templates/manage/teams.html` modification

Add "Manage Roster" link in each team row, alongside existing edit/delete controls.

---

## Files Summary

| File | Change |
|------|--------|
| `mydb/mydb.go` | Add `players` table migration |
| `mydb/players.go` | New: `Player` struct, `DisplayName()`, 5 DB methods |
| `mydb/db.go` | Add 5 player methods to `DB` interface |
| `mydb/fakedb.go` | Implement 5 player methods |
| `webhandler/roster.go` | New: `RosterList`, `RosterAdd`, `RosterEdit`, `RosterDelete` handlers |
| `webhandler/webhandler.go` | Register 6 roster routes outside CanManage middleware |
| `webhandler/templates/manage/roster.html` | New: manage roster list |
| `webhandler/templates/manage/edit_player.html` | New: add/edit player form |
| `webhandler/templates/team.html` | Add public roster section |
| `webhandler/templates/manage/teams.html` | Add "Manage Roster" link per team |
