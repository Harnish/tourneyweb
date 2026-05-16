# Division Rules Editor Design

**Date:** 2026-05-15  
**Status:** Approved

## Overview

Two-tier rules system: tournament-level rules managed by directors/staff (shown on the public tournament page and as a fallback on each division page) and division-level rules managed by directors/staff (shown on the public division page, overriding the tournament fallback). Both use a Quill rich-text editor.

---

## Data Layer

### Schema

Two migrations:

```sql
ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS rules_html TEXT NOT NULL DEFAULT '';
ALTER TABLE divisions ADD COLUMN IF NOT EXISTS rules_html TEXT NOT NULL DEFAULT '';
```

### Structs

Add `RulesHTML string` to both `Tournament` (in `mydb/tournaments.go`) and `Division` (in `mydb/divisions.go`).

Only the queries used by pages that display rules need the new column:
- `ReturnTournamentByID` — add `rules_html` to SELECT and Scan (used by tournament page and as division page fallback)
- `ReturnDivisionByID` — add `rules_html` to SELECT and Scan (used by public division page and manage edit pre-fill)
- `ReturnDivisions` — add `rules_html` to SELECT and Scan (used in manage division list; included for struct consistency)

### Methods

Two new write methods added to the DB interface and implemented on `MyDB` and `FakeDB`:

| Method | Signature |
|--------|-----------|
| Set tournament rules | `SetTournamentRules(id int, html string)` |
| Set division rules | `SetDivisionRules(id int, html string)` |

Both follow the same pattern as the existing `SetTournamentExtras`.

---

## Routes and Permissions

### Tournament Rules

| Method | Path | Action |
|--------|------|--------|
| GET | `/tournaments/:tid/manage/rules` | Quill edit form showing current tournament rules |
| POST | `/tournaments/:tid/manage/rules` | Save tournament rules |

Auth guard: existing manage middleware (directors and staff).

### Division Rules

No new routes. Division rules are edited via the existing:

| Method | Path | Action |
|--------|------|--------|
| GET | `/tournaments/:tid/manage/divisions/:did/edit` | Division edit form (extended with Quill) |
| POST | `/tournaments/:tid/manage/divisions/:did/edit` | Saves name + calls `SetDivisionRules` |

---

## Management UI

### Tournament Rules (`manage/rules.html`)

New template with a Quill editor pre-filled from `Tournament.RulesHTML`. Form POSTs to `/tournaments/:tid/manage/rules`. Same Quill setup as extras and news editors.

### Division Edit Form (`manage/edit_division.html`)

Quill editor added below the name field. On POST, `ManageDivisionEdit` reads `rules_html` from the form and calls `me.DB.SetDivisionRules(div.ID, rulesHTML)` after saving the name.

### Navigation

- Manage dashboard (`manage/dashboard.html`): add "Rules" link to `/tournaments/:tid/manage/rules`

---

## Public Display

### Tournament Page (`tournament.html`)

Add a "Rules" section below the division list, shown only when `Tournament.RulesHTML` is non-empty:

```
{{if .Tournament.RulesHTML}}
<hr>
<h2>Rules</h2>
{{.Tournament.RulesHTML | htmlSafe}}
{{end}}
```

### Division Page (`divisions.html`)

Add a "Rules" section below the games table. Shows division-specific rules if set; falls back to tournament rules otherwise. `divisionData` already carries `baseData` which includes `Tournament`:

```
{{if .Division.RulesHTML}}
<hr>
<h2>Rules</h2>
{{.Division.RulesHTML | htmlSafe}}
{{else if .Tournament.RulesHTML}}
<hr>
<h2>Rules</h2>
<p><em>(Tournament rules — no division-specific rules set)</em></p>
{{.Tournament.RulesHTML | htmlSafe}}
{{end}}
```

---

## Handler

New handler `ManageRules` in `webhandler/manage_divisions.go` (alongside the division handlers):

- GET: renders `manageRules` template with `newBaseWithTournament(r, t)`
- POST: reads `rules_html` form field, calls `me.DB.SetTournamentRules(t.ID, html)`, redirects back to GET

---

## Template Data Structs

No new structs needed:
- Tournament rules edit page uses `baseData` (Tournament already embedded)
- Division edit page uses existing `manageDivisionEditData` — `Division.RulesHTML` is available after the struct update

---

## Testing

- `FakeDB.SetTournamentRules` and `FakeDB.SetDivisionRules` implemented
- Test: set tournament rules, verify `ReturnTournamentByID` returns them
- Test: set division rules, verify `ReturnDivisionByID` returns them
- Test: division with no rules returns empty `RulesHTML`; tournament rules remain separate

---

## Error Handling

- Tournament not found → existing `tournamentFromRoute` returns 404
- Division not found → existing handler returns 404

---

## Out of Scope

- Admin-side rules editing (admins use the manage route)
- Per-team rules
- Rules versioning or history
