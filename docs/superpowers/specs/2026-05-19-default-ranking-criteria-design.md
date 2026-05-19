# Tournament Default Ranking Criteria Design

## Goal

Allow tournament directors and admins to set a default division ranking order at tournament creation time. The default is automatically applied when new divisions are created. It is adjustable until the tournament is published, at which point it locks.

## Tech Stack

Go, PostgreSQL, html/template, no new dependencies.

---

## Section 1: Data Model

### `tournaments` table

```sql
ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS default_ranking_criteria TEXT;
```

Format: comma-separated ranking key list, same as `divisions.ranking_criteria` (e.g. `"wins,head_to_head,runs_against"`). NULL or empty string falls back to `mydb.DefaultRankingCriteria` at read time.

### `AddDivision` return type change

`AddDivision` currently returns void. Change it to return `int` (the new division ID), consistent with `AddGame`, `AddPlayer`, `AddLocation`. This allows the handler to immediately initialize the new division's ranking criteria from the tournament default.

**Interface change:**
```go
// Before
AddDivision(tournamentID int, name string)

// After
AddDivision(tournamentID int, name string) int
```

Both `MyDB` and `FakeDB` implementations updated accordingly.

### Division creation flow

When a director or admin creates a new division:
1. Handler calls `me.DB.AddDivision(t.ID, name)` — gets back the new division ID
2. Handler reads `t.DefaultRankingCriteria` (the tournament's stored default)
3. If non-empty, calls `me.DB.UpdateDivision(newDivID, name, criteria)` to set ranking criteria
4. If empty, division inherits `mydb.DefaultRankingCriteria` (already the default behavior)

---

## Section 2: UI

### New tournament form (`/create-tournament` and `/admin/tournaments`)

Add the ranking criteria checklist below the existing fields. Pre-checked with `mydb.DefaultRankingCriteria`. Uses the same `AllCriteriaForUI` function and same JS (move up/down, hidden input) as the division edit form. Submitted as `default_ranking_criteria` (comma-separated string).

Label: **Default Division Ranking Order**
Help text: *Applied automatically when new divisions are created.*

### Manage dashboard (`/tournaments/:tid/manage`)

New "Default Ranking Order" section with the checklist and a Save button (POST to `/tournaments/:tid/manage`).

**When tournament is published:**
- Checklist inputs rendered with `disabled` attribute
- Notice: "Ranking order is locked after publishing."

**When editing while divisions already exist:**
- Notice: "This updates the default for new divisions only — existing divisions must be edited manually."

**When no divisions exist yet:**
- No notice needed.

The manage dashboard already handles GET and POST; the POST handler adds a branch for `default_ranking_criteria` form value.

### Division creation

No UI change to the add-division form. The ranking criteria is initialized silently from the tournament default when the division is created.

---

## Section 3: Locking and Enforcement

### Director manage dashboard

Server-side: if `tournament.Status == "published"`, the POST handler ignores (or rejects with 400) any submitted `default_ranking_criteria` value.

Client-side: checklist inputs rendered `disabled` with the locked notice.

### Admin edit tournament route (`/admin/tournaments/:tid/edit`)

No locking — admins can edit `default_ranking_criteria` regardless of tournament status.

---

## Tournament struct change

Add `DefaultRankingCriteria []string` to the `Tournament` struct (parsed from the TEXT column on load, same as divisions). `mydb.ParseRankingCriteria(s string) []string` already exists; reuse it here.

Add `SetTournamentDefaultRanking(id int, criteria []string)` to MyDB (stores as comma-joined string). Signature consistent with existing setters.

---

## Files

| File | Change |
|------|--------|
| `mydb/mydb.go` | Add `default_ranking_criteria` migration |
| `mydb/divisions.go` | Change `AddDivision` to return `int` |
| `mydb/tournaments.go` | Add `DefaultRankingCriteria []string` to `Tournament` struct; update `scanTournaments` to scan 10th column; add `SetTournamentDefaultRanking` |
| `mydb/db.go` | Update `AddDivision` signature in interface; add `SetTournamentDefaultRanking(id int, criteria []string)` |
| `mydb/fakedb.go` | Update `AddDivision` to return `int`; implement `SetTournamentDefaultRanking` |
| `webhandler/divisions.go` | `AddDivisionForm` POST: use returned ID, apply tournament default |
| `webhandler/manage_divisions.go` | `ManageDivisions` POST: use returned ID, apply tournament default |
| `webhandler/manage.go` | `ManageDashboard` GET+POST: add default ranking section; enforce lock on published |
| `webhandler/templates.go` | Extend `manageDashboardData` with `AllCriteria []CriterionUIRow`; add `newTournamentData` with same |
| `webhandler/templates/manage/new_tournament.html` | Add ranking checklist |
| `webhandler/templates/manage/dashboard.html` | Add default ranking section |
| `webhandler/templates/admin/tournaments.html` | Add ranking checklist to admin create tournament form |
