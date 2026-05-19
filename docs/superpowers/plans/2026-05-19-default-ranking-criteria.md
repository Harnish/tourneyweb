# Tournament Default Ranking Criteria Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow tournament directors/admins to set a default division ranking order at tournament creation; the order is applied automatically when new divisions are created and locks on publish.

**Architecture:** Add a `default_ranking_criteria` TEXT column to `tournaments`, store as comma-separated keys (consistent with how forms already submit ranking values). Extend `Tournament` struct, update all SELECT queries and `scanTournaments`, add `SetTournamentDefaultRanking` to the DB layer, change `AddDivision` to return the new division ID so handlers can immediately apply the default. ManageDashboard gains a POST branch for saving the default; create-tournament forms gain a ranking checklist.

**Tech Stack:** Go, PostgreSQL, html/template, no new dependencies.

---

## File Map

| File | Change |
|------|--------|
| `mydb/mydb.go` | Add migration: `ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS default_ranking_criteria TEXT` |
| `mydb/tournaments.go` | Add `DefaultRankingCriteria []string` to `Tournament`; add `parseTournamentRanking`; update every SELECT to include the column; update `scanTournaments` and `ReturnTournamentByID` to scan 10 columns; add `SetTournamentDefaultRanking` |
| `mydb/db.go` | Change `AddDivision` signature to return `int`; add `SetTournamentDefaultRanking(id int, criteria []string)` |
| `mydb/fakedb.go` | Change `FakeDB.AddDivision` to return `int`; add `FakeDB.SetTournamentDefaultRanking` |
| `mydb/mydb_test.go` | New: tests using FakeDB for SetTournamentDefaultRanking and AddDivision return value |
| `webhandler/divisions.go` | `AddDivisionForm` POST: capture returned division ID; apply `t.DefaultRankingCriteria` if non-empty |
| `webhandler/manage_divisions.go` | `ManageDivisions` POST: same as above |
| `webhandler/manage.go` | `ManageDashboard`: add POST branch; load divisions count; populate AllCriteria |
| `webhandler/templates.go` | Extend `manageDashboardData`; add `newTournamentData`; extend `adminTournamentsData` |
| `webhandler/tournaments.go` | `NewTournamentForm`: use `newTournamentData`; `NewTournament` POST: parse and save default ranking; `AdminTournaments` GET: populate AllCriteria; `CreateTournament` POST: parse and save default ranking |
| `main.go` | Add `router.POST("/tournaments/:tid/manage", wh.ManageDashboard)` |
| `webhandler/templates/manage/new_tournament.html` | Add ranking checklist + JS |
| `webhandler/templates/manage/dashboard.html` | Add "Default Ranking Order" section + JS |
| `webhandler/templates/admin/tournaments.html` | Add ranking checklist to create form + JS |

---

## Task 1: DB layer — Tournament struct, migration, SetTournamentDefaultRanking

**Files:**
- Modify: `mydb/mydb.go`
- Modify: `mydb/tournaments.go`
- Modify: `mydb/db.go`
- Modify: `mydb/fakedb.go`
- Create: `mydb/mydb_test.go`

- [ ] **Step 1: Write failing tests**

Create `mydb/mydb_test.go`:

```go
package mydb

import (
	"testing"
	"time"
)

func TestSetTournamentDefaultRanking(t *testing.T) {
	db := NewFakeDB()
	id := db.AddTournament("Test", "Baseball", "Here", "", time.Time{}, "draft")
	criteria := []string{"wins", "runs_against"}
	db.SetTournamentDefaultRanking(id, criteria)
	got := db.ReturnTournamentByID(id)
	if len(got.DefaultRankingCriteria) != 2 {
		t.Fatalf("expected 2 criteria, got %d: %v", len(got.DefaultRankingCriteria), got.DefaultRankingCriteria)
	}
	if got.DefaultRankingCriteria[0] != "wins" || got.DefaultRankingCriteria[1] != "runs_against" {
		t.Errorf("expected [wins runs_against], got %v", got.DefaultRankingCriteria)
	}
}

func TestSetTournamentDefaultRankingEmpty(t *testing.T) {
	db := NewFakeDB()
	id := db.AddTournament("Test", "Baseball", "Here", "", time.Time{}, "draft")
	got := db.ReturnTournamentByID(id)
	if len(got.DefaultRankingCriteria) != 0 {
		t.Errorf("expected nil/empty DefaultRankingCriteria on new tournament, got %v", got.DefaultRankingCriteria)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /home/jharnish/Work/tourneyweb
go test ./mydb/... -run TestSetTournamentDefault -v
```

Expected: compilation error — `SetTournamentDefaultRanking` not defined and `DefaultRankingCriteria` not on struct.

- [ ] **Step 3: Add migration in `mydb/mydb.go`**

In the `pgmigrations` slice (after the last entry ending with the players table), append:

```go
`ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS default_ranking_criteria TEXT`,
```

- [ ] **Step 4: Update `Tournament` struct and add helpers in `mydb/tournaments.go`**

Add `DefaultRankingCriteria []string` to the struct and a `parseTournamentRanking` helper. Add `"strings"` to the import block.

Replace the `Tournament` struct:

```go
type Tournament struct {
	ID                     int
	Name                   string
	Sport                  string
	Location               string
	StartDate              time.Time
	Notes                  string
	ExtrasHTML             string
	RulesHTML              string
	Status                 string
	DefaultRankingCriteria []string
}
```

Add after the struct (before `scanTournaments`):

```go
func parseTournamentRanking(s sql.NullString) []string {
	if !s.Valid || s.String == "" {
		return nil
	}
	var out []string
	for _, k := range strings.Split(s.String, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			out = append(out, k)
		}
	}
	return out
}
```

- [ ] **Step 5: Update `scanTournaments` to scan 10 columns**

Replace `scanTournaments`:

```go
func scanTournaments(rows *sql.Rows) []Tournament {
	var out []Tournament
	for rows.Next() {
		var t Tournament
		var defRanking sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes, &t.ExtrasHTML, &t.RulesHTML, &t.Status, &defRanking); err != nil {
			slog.Error("scanTournaments", "err", err)
			continue
		}
		t.DefaultRankingCriteria = parseTournamentRanking(defRanking)
		out = append(out, t)
	}
	rows.Close()
	return out
}
```

- [ ] **Step 6: Update all SELECT queries in `mydb/tournaments.go` to include the new column**

Every query that selects the 9 tournament columns must add `, default_ranking_criteria` at the end. Update these functions (all use the same column list pattern):

- `ReturnTournaments`: change SELECT to `SELECT id, name, sport, location, start_date, notes, extras_html, rules_html, status, default_ranking_criteria FROM tournaments ORDER BY start_date DESC`
- `ReturnTournamentsComingUp`: add `, default_ranking_criteria` to SELECT
- `ReturnTournamentsRecent`: add `, default_ranking_criteria` to SELECT
- `ReturnTournamentsFuture`: add `, default_ranking_criteria` to SELECT
- `ReturnTournamentsPast`: add `, default_ranking_criteria` to SELECT
- `ReturnDraftTournaments`: add `, default_ranking_criteria` to SELECT

Also update `ReturnTournamentByID` (uses its own QueryRow/Scan, not scanTournaments):

```go
func (me *MyDB) ReturnTournamentByID(id int) Tournament {
	var t Tournament
	var defRanking sql.NullString
	err := me.DB.QueryRow(
		`SELECT id, name, sport, location, start_date, notes, extras_html, rules_html, status, default_ranking_criteria FROM tournaments WHERE id=$1`, id,
	).Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes, &t.ExtrasHTML, &t.RulesHTML, &t.Status, &defRanking)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("ReturnTournamentByID", "err", err)
	}
	t.DefaultRankingCriteria = parseTournamentRanking(defRanking)
	return t
}
```

- [ ] **Step 7: Add `SetTournamentDefaultRanking` to `mydb/tournaments.go`**

Append after `SetTournamentRules`:

```go
func (me *MyDB) SetTournamentDefaultRanking(id int, criteria []string) {
	var val interface{}
	if len(criteria) > 0 {
		val = strings.Join(criteria, ",")
	}
	_, err := me.DB.Exec(`UPDATE tournaments SET default_ranking_criteria=$1 WHERE id=$2`, val, id)
	if err != nil {
		slog.Error("SetTournamentDefaultRanking", "err", err)
	}
}
```

- [ ] **Step 8: Update `mydb/db.go` interface**

Add `SetTournamentDefaultRanking(id int, criteria []string)` to the Tournaments section of the interface (after `SetTournamentRules`):

```go
SetTournamentDefaultRanking(id int, criteria []string)
```

- [ ] **Step 9: Add `FakeDB.SetTournamentDefaultRanking` in `mydb/fakedb.go`**

Add after `FakeDB.SetTournamentRules` (around line 222):

```go
func (f *FakeDB) SetTournamentDefaultRanking(id int, criteria []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.tournaments[id]
	t.DefaultRankingCriteria = criteria
	f.tournaments[id] = t
}
```

- [ ] **Step 10: Run tests to verify they pass**

```bash
go test ./mydb/... -run TestSetTournamentDefault -v
```

Expected: PASS (both tests pass).

- [ ] **Step 11: Verify the project still builds**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 12: Commit**

```bash
git add mydb/mydb.go mydb/tournaments.go mydb/db.go mydb/fakedb.go mydb/mydb_test.go
git commit -m "feat: add default_ranking_criteria to Tournament struct and DB layer"
```

---

## Task 2: Change AddDivision to return int

**Files:**
- Modify: `mydb/divisions.go` (line 33)
- Modify: `mydb/db.go` (line 22)
- Modify: `mydb/fakedb.go` (line 224)
- Modify: `mydb/mydb_test.go`

- [ ] **Step 1: Write failing test**

Add to `mydb/mydb_test.go`:

```go
func TestAddDivisionReturnsID(t *testing.T) {
	db := NewFakeDB()
	tid := db.AddTournament("Test", "Baseball", "Here", "", time.Time{}, "draft")
	id1 := db.AddDivision(tid, "Division A")
	if id1 <= 0 {
		t.Errorf("expected positive ID from AddDivision, got %d", id1)
	}
	id2 := db.AddDivision(tid, "Division B")
	if id2 <= 0 || id2 == id1 {
		t.Errorf("expected unique positive ID, got %d (first was %d)", id2, id1)
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./mydb/... -run TestAddDivisionReturnsID -v
```

Expected: compilation error — `AddDivision` returns void, cannot assign to `id1`.

- [ ] **Step 3: Update `mydb/divisions.go` `AddDivision` to return int**

Replace lines 33–41 (the `AddDivision` function):

```go
func (me *MyDB) AddDivision(tournamentID int, name string) int {
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO divisions (tournament_id, name) VALUES ($1,$2) RETURNING id`,
		tournamentID, name,
	).Scan(&id)
	if err != nil {
		slog.Error("AddDivision", "err", err)
	}
	return id
}
```

- [ ] **Step 4: Update `mydb/db.go` interface**

Change the `AddDivision` line in the Divisions section from:

```go
AddDivision(tournamentID int, name string)
```

to:

```go
AddDivision(tournamentID int, name string) int
```

- [ ] **Step 5: Update `mydb/fakedb.go` `FakeDB.AddDivision` to return int**

Replace lines 224–235 (the `FakeDB.AddDivision` function):

```go
func (f *FakeDB) AddDivision(tournamentID int, name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	f.divisions[id] = Division{
		ID:              id,
		TournamentID:    tournamentID,
		Name:            name,
		RankingCriteria: DefaultRankingCriteria,
		Phase:           "pool",
	}
	return id
}
```

- [ ] **Step 6: Run test to verify it passes**

```bash
go test ./mydb/... -run TestAddDivisionReturnsID -v
```

Expected: PASS.

- [ ] **Step 7: Verify the full build still compiles**

```bash
go build ./...
```

Expected: no errors. (The webhandler callers currently discard the return value, which is valid Go — the void callers become discarding callers.)

- [ ] **Step 8: Commit**

```bash
git add mydb/divisions.go mydb/db.go mydb/fakedb.go mydb/mydb_test.go
git commit -m "feat: AddDivision returns new division ID"
```

---

## Task 3: Apply tournament default ranking when creating a division

**Files:**
- Modify: `webhandler/divisions.go`
- Modify: `webhandler/manage_divisions.go`

- [ ] **Step 1: Update `webhandler/divisions.go` `AddDivisionForm` POST branch**

The POST branch currently (around line 19–26) is:

```go
if r.Method == http.MethodPost {
    name := r.FormValue("divisionname")
    if name == "" {
        http.Error(w, "Division name required", http.StatusBadRequest)
        return
    }
    me.DB.AddDivision(t.ID, name)
    http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/divisions", t.ID), http.StatusSeeOther)
    return
}
```

Replace with:

```go
if r.Method == http.MethodPost {
    name := r.FormValue("divisionname")
    if name == "" {
        http.Error(w, "Division name required", http.StatusBadRequest)
        return
    }
    divID := me.DB.AddDivision(t.ID, name)
    if len(t.DefaultRankingCriteria) > 0 {
        me.DB.UpdateDivision(divID, name, t.DefaultRankingCriteria)
    }
    http.Redirect(w, r, fmt.Sprintf("/admin/tournaments/%d/divisions", t.ID), http.StatusSeeOther)
    return
}
```

- [ ] **Step 2: Update `webhandler/manage_divisions.go` `ManageDivisions` POST branch**

The POST branch currently (around line 19–27) is:

```go
if r.Method == http.MethodPost {
    name := r.FormValue("divisionname")
    if name == "" {
        http.Error(w, "Division name required", http.StatusBadRequest)
        return
    }
    me.DB.AddDivision(t.ID, name)
    http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
    return
}
```

Replace with:

```go
if r.Method == http.MethodPost {
    name := r.FormValue("divisionname")
    if name == "" {
        http.Error(w, "Division name required", http.StatusBadRequest)
        return
    }
    divID := me.DB.AddDivision(t.ID, name)
    if len(t.DefaultRankingCriteria) > 0 {
        me.DB.UpdateDivision(divID, name, t.DefaultRankingCriteria)
    }
    http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
    return
}
```

- [ ] **Step 3: Build to confirm no errors**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add webhandler/divisions.go webhandler/manage_divisions.go
git commit -m "feat: apply tournament default ranking criteria when creating a division"
```

---

## Task 4: ManageDashboard — default ranking section

**Files:**
- Modify: `webhandler/templates.go`
- Modify: `webhandler/manage.go`
- Modify: `main.go`
- Modify: `webhandler/templates/manage/dashboard.html`

- [ ] **Step 1: Extend `manageDashboardData` in `webhandler/templates.go`**

Replace:

```go
type manageDashboardData struct {
	baseData
	IsDraft bool
}
```

with:

```go
type manageDashboardData struct {
	baseData
	IsDraft      bool
	AllCriteria  []CriterionUIRow
	HasDivisions bool
}
```

- [ ] **Step 2: Update `ManageDashboard` in `webhandler/manage.go`**

The file currently has only a GET handler. Replace the entire file content with:

```go
package webhandler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func (me *Env) ManageDashboard(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !user.CanManage(t.ID) {
		me.renderError(w, r, http.StatusForbidden, "Not Authorized", "You must be a tournament director to access this page.")
		return
	}

	if r.Method == http.MethodPost {
		if t.Status == "published" && !user.IsAdmin {
			http.Error(w, "Ranking order is locked after publishing", http.StatusBadRequest)
			return
		}
		criteriaStr := r.FormValue("default_ranking_criteria")
		var criteria []string
		for _, k := range strings.Split(criteriaStr, ",") {
			k = strings.TrimSpace(k)
			if _, ok := criteriaRegistry[k]; ok {
				criteria = append(criteria, k)
			}
		}
		if len(criteria) == 0 {
			criteria = mydb.DefaultRankingCriteria
		}
		me.DB.SetTournamentDefaultRanking(t.ID, criteria)
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage", t.ID), http.StatusSeeOther)
		return
	}

	divisions := me.DB.ReturnDivisions(t.ID)
	active := t.DefaultRankingCriteria
	if len(active) == 0 {
		active = mydb.DefaultRankingCriteria
	}
	me.render(w, "manageDashboard", manageDashboardData{
		baseData:     newBaseWithTournament(r, t),
		IsDraft:      t.Status == "draft",
		AllCriteria:  AllCriteriaForUI(active),
		HasDivisions: len(divisions) > 0,
	})
}
```

- [ ] **Step 3: Add POST route in `main.go`**

Find the line:

```go
router.GET("/tournaments/:tid/manage", wh.ManageDashboard)
```

Add immediately after it:

```go
router.POST("/tournaments/:tid/manage", wh.ManageDashboard)
```

- [ ] **Step 4: Update `webhandler/templates/manage/dashboard.html`**

Replace the entire file:

```html
{{define "manageDashboard"}}
{{template "header" .}}
<h1>Manage: {{.Tournament.Name}}</h1>
{{if .IsDraft}}
<div style="background:rgba(245,166,35,0.1);border:1px solid rgba(245,166,35,0.4);padding:12px 16px;margin-bottom:18px;border-radius:6px;font-size:0.9rem">
  This tournament is a <strong>draft</strong> and not publicly visible.
  <a href="/tournaments/{{.Tournament.ID}}/manage/publish">Enter verification code to publish &rarr;</a>
</div>
{{end}}
<ul class="tw-manage-nav">
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/divisions">Divisions &amp; Games</a></li>
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/teams">Teams</a></li>
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/locations">Locations / Fields</a></li>
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/roles">Roles &amp; Staff</a></li>
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/rules">Rules</a></li>
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/extras">Event Extras</a></li>
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/news">News</a></li>
  {{if .IsDraft}}<li><a href="/tournaments/{{.Tournament.ID}}/manage/publish">Publish Tournament</a></li>{{end}}
</ul>

<h2>Default Division Ranking Order</h2>
{{if eq .Tournament.Status "published"}}
<p style="color:#888;font-size:0.9rem">Ranking order is locked after publishing.</p>
{{else}}
  {{if .HasDivisions}}
  <p style="font-size:0.9rem;color:#666">This updates the default for new divisions only — existing divisions must be edited manually.</p>
  {{end}}
{{end}}
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage" id="dashboard-form">
{{.CSRFField}}
<p>Check criteria to include them. Use ↑/↓ to set the tiebreaker order (top = first tiebreaker).</p>
<ul id="ranking-list" style="list-style:none;padding:0;max-width:500px;">
{{range .AllCriteria}}
<li style="display:flex;align-items:center;gap:0.5em;margin-bottom:0.3em;padding:0.5em 0.75em;background:var(--tw-bg-card,#f9f9f9);border:1px solid var(--tw-border,#ddd);border-radius:4px;">
  <input type="checkbox" value="{{.Key}}"{{if .Checked}} checked{{end}}{{if eq $.Tournament.Status "published"}} disabled{{end}}>
  <span style="flex:1">{{.Label}}</span>
  <button type="button" onclick="moveUp(this)" style="padding:0 6px;background:none;border:1px solid #ccc;border-radius:3px;cursor:pointer;"{{if eq $.Tournament.Status "published"}} disabled{{end}}>↑</button>
  <button type="button" onclick="moveDown(this)" style="padding:0 6px;background:none;border:1px solid #ccc;border-radius:3px;cursor:pointer;"{{if eq $.Tournament.Status "published"}} disabled{{end}}>↓</button>
</li>
{{end}}
</ul>
<input type="hidden" name="default_ranking_criteria" id="dashboard_ranking_input">
{{if ne .Tournament.Status "published"}}
<input type="submit" value="Save Default Ranking">
{{end}}
</form>

<p style="margin-top:1.25rem"><a href="/tournaments/{{.Tournament.ID}}" class="tw-back">&larr; View Public Page</a></p>
<script>
document.getElementById('dashboard-form').addEventListener('submit', function() {
  var checks = document.querySelectorAll('#ranking-list input[type=checkbox]');
  var active = [];
  checks.forEach(function(cb) { if (cb.checked) active.push(cb.value); });
  document.getElementById('dashboard_ranking_input').value = active.join(',');
});
function moveUp(btn) {
  var li = btn.closest('li');
  var prev = li.previousElementSibling;
  if (prev) li.parentNode.insertBefore(li, prev);
}
function moveDown(btn) {
  var li = btn.closest('li');
  var next = li.nextElementSibling;
  if (next) li.parentNode.insertBefore(next, li);
}
</script>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 5: Build to confirm no errors**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add webhandler/templates.go webhandler/manage.go main.go webhandler/templates/manage/dashboard.html
git commit -m "feat: manage dashboard default ranking section with publish lock"
```

---

## Task 5: Create-tournament forms with ranking checklist

**Files:**
- Modify: `webhandler/templates.go`
- Modify: `webhandler/tournaments.go`
- Modify: `webhandler/templates/manage/new_tournament.html`
- Modify: `webhandler/templates/admin/tournaments.html`

- [ ] **Step 1: Add `newTournamentData` and extend `adminTournamentsData` in `webhandler/templates.go`**

Add `newTournamentData` after `editTournamentData`:

```go
type newTournamentData struct {
	baseData
	AllCriteria []CriterionUIRow
}
```

Extend `adminTournamentsData` (currently has `baseData` + `Tournaments`):

```go
type adminTournamentsData struct {
	baseData
	Tournaments []mydb.Tournament
	AllCriteria []CriterionUIRow
}
```

- [ ] **Step 2: Update `NewTournamentForm` in `webhandler/tournaments.go`**

Find `NewTournamentForm` (around line 134):

```go
func (me *Env) NewTournamentForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	me.render(w, "newTournament", newBase(r))
}
```

Replace with:

```go
func (me *Env) NewTournamentForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	me.render(w, "newTournament", newTournamentData{
		baseData:    newBase(r),
		AllCriteria: AllCriteriaForUI(mydb.DefaultRankingCriteria),
	})
}
```

- [ ] **Step 3: Update `NewTournament` POST handler to save default ranking**

Find `NewTournament` (around line 143). After the `me.DB.AssignRole(user.ID, id, "director", 0)` line and before the redirect, add:

```go
criteriaStr := r.FormValue("default_ranking_criteria")
var criteria []string
for _, k := range strings.Split(criteriaStr, ",") {
    k = strings.TrimSpace(k)
    if _, ok := criteriaRegistry[k]; ok {
        criteria = append(criteria, k)
    }
}
if len(criteria) > 0 {
    me.DB.SetTournamentDefaultRanking(id, criteria)
}
```

Confirm `"strings"` is already in the import block of `webhandler/tournaments.go` (it is used by `time.Parse`-adjacent code already — add it if missing).

- [ ] **Step 4: Update `AdminTournaments` GET handler**

Find `AdminTournaments` (around line 93):

```go
func (me *Env) AdminTournaments(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "adminTournaments", adminTournamentsData{
		baseData:    newBase(r),
		Tournaments: me.DB.ReturnTournaments(),
	})
}
```

Replace with:

```go
func (me *Env) AdminTournaments(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "adminTournaments", adminTournamentsData{
		baseData:    newBase(r),
		Tournaments: me.DB.ReturnTournaments(),
		AllCriteria: AllCriteriaForUI(mydb.DefaultRankingCriteria),
	})
}
```

- [ ] **Step 5: Update `CreateTournament` POST handler to save default ranking**

Find `CreateTournament` (around line 100). After the `me.DB.AssignRole` call (inside the `if user.LoggedIn()` block) and before the redirect, add:

```go
criteriaStr := r.FormValue("default_ranking_criteria")
var criteria []string
for _, k := range strings.Split(criteriaStr, ",") {
    k = strings.TrimSpace(k)
    if _, ok := criteriaRegistry[k]; ok {
        criteria = append(criteria, k)
    }
}
if len(criteria) > 0 {
    me.DB.SetTournamentDefaultRanking(id, criteria)
}
```

The `strings` package is already imported in `webhandler/tournaments.go` (`time.Parse` uses it, and the file already has `"strings"` in the import — if not, add it).

- [ ] **Step 6: Update `webhandler/templates/manage/new_tournament.html`**

Replace the entire file:

```html
{{define "newTournament"}}
{{template "header" .}}
<h1>Create Tournament</h1>
<form method="post" action="/create-tournament" id="new-tournament-form">
{{.CSRFField}}
<table>
  <tr><td>Name</td><td><input type="text" name="name" required></td></tr>
  <tr><td>Sport</td><td><input type="text" name="sport" required></td></tr>
  <tr><td>Location/Venue</td><td><input type="text" name="location" required></td></tr>
  <tr><td>Start Date</td><td><input type="date" name="start_date" required></td></tr>
  <tr><td>Notes</td><td><input type="text" name="notes"></td></tr>
</table>
<h3>Default Division Ranking Order</h3>
<p>Applied automatically when new divisions are created.</p>
<ul id="new-ranking-list" style="list-style:none;padding:0;max-width:500px;">
{{range .AllCriteria}}
<li style="display:flex;align-items:center;gap:0.5em;margin-bottom:0.3em;padding:0.5em 0.75em;background:var(--tw-bg-card,#f9f9f9);border:1px solid var(--tw-border,#ddd);border-radius:4px;">
  <input type="checkbox" value="{{.Key}}"{{if .Checked}} checked{{end}}>
  <span style="flex:1">{{.Label}}</span>
  <button type="button" onclick="newMoveUp(this)" style="padding:0 6px;background:none;border:1px solid #ccc;border-radius:3px;cursor:pointer;">↑</button>
  <button type="button" onclick="newMoveDown(this)" style="padding:0 6px;background:none;border:1px solid #ccc;border-radius:3px;cursor:pointer;">↓</button>
</li>
{{end}}
</ul>
<input type="hidden" name="default_ranking_criteria" id="new_ranking_input">
<br>
<input type="submit" value="Create Tournament">
</form>
<script>
document.getElementById('new-tournament-form').addEventListener('submit', function() {
  var checks = document.querySelectorAll('#new-ranking-list input[type=checkbox]');
  var active = [];
  checks.forEach(function(cb) { if (cb.checked) active.push(cb.value); });
  document.getElementById('new_ranking_input').value = active.join(',');
});
function newMoveUp(btn) {
  var li = btn.closest('li');
  var prev = li.previousElementSibling;
  if (prev) li.parentNode.insertBefore(li, prev);
}
function newMoveDown(btn) {
  var li = btn.closest('li');
  var next = li.nextElementSibling;
  if (next) li.parentNode.insertBefore(next, li);
}
</script>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 7: Update `webhandler/templates/admin/tournaments.html`**

Replace the entire file:

```html
{{define "adminTournaments"}}
{{template "header" .}}
<h1>Tournaments</h1>
<p><a href="/admin/queue">Verification Queue (Draft Tournaments)</a></p>
<h2>Create Tournament</h2>
<form method="post" action="/admin/tournaments" id="admin-create-form">
<table>
<tr><td>Name</td><td><input type="text" name="name" required></td></tr>
<tr><td>Sport</td><td><input type="text" name="sport" required></td></tr>
<tr><td>Location</td><td><input type="text" name="location" required></td></tr>
<tr><td>Start Date</td><td><input type="date" name="start_date" required></td></tr>
<tr><td>Notes</td><td><input type="text" name="notes"></td></tr>
</table>
<h3>Default Division Ranking Order</h3>
<p>Applied automatically when new divisions are created.</p>
<ul id="admin-ranking-list" style="list-style:none;padding:0;max-width:500px;">
{{range .AllCriteria}}
<li style="display:flex;align-items:center;gap:0.5em;margin-bottom:0.3em;padding:0.5em 0.75em;background:var(--tw-bg-card,#f9f9f9);border:1px solid var(--tw-border,#ddd);border-radius:4px;">
  <input type="checkbox" value="{{.Key}}"{{if .Checked}} checked{{end}}>
  <span style="flex:1">{{.Label}}</span>
  <button type="button" onclick="adminMoveUp(this)" style="padding:0 6px;background:none;border:1px solid #ccc;border-radius:3px;cursor:pointer;">↑</button>
  <button type="button" onclick="adminMoveDown(this)" style="padding:0 6px;background:none;border:1px solid #ccc;border-radius:3px;cursor:pointer;">↓</button>
</li>
{{end}}
</ul>
<input type="hidden" name="default_ranking_criteria" id="admin_ranking_input">
{{.CSRFField}}
<br>
<input type="submit" value="Create Tournament">
</form>
<h2>All Tournaments</h2>
<table border="1" cellpadding="1" cellspacing="0">
<tr><th>Name</th><th>Sport</th><th>Location</th><th>Start Date</th></tr>
{{range .Tournaments}}
<tr>
  <td><a href="/admin/tournaments/{{.ID}}">{{.Name}}</a></td>
  <td>{{.Sport}}</td>
  <td>{{.Location}}</td>
  <td>{{.StartDate.Format "Jan 2, 2006"}}</td>
</tr>
{{end}}
</table>
<script>
document.getElementById('admin-create-form').addEventListener('submit', function() {
  var checks = document.querySelectorAll('#admin-ranking-list input[type=checkbox]');
  var active = [];
  checks.forEach(function(cb) { if (cb.checked) active.push(cb.value); });
  document.getElementById('admin_ranking_input').value = active.join(',');
});
function adminMoveUp(btn) {
  var li = btn.closest('li');
  var prev = li.previousElementSibling;
  if (prev) li.parentNode.insertBefore(li, prev);
}
function adminMoveDown(btn) {
  var li = btn.closest('li');
  var next = li.nextElementSibling;
  if (next) li.parentNode.insertBefore(next, li);
}
</script>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 8: Build to confirm no errors**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 9: Run full test suite**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 10: Commit**

```bash
git add webhandler/templates.go webhandler/tournaments.go \
        webhandler/templates/manage/new_tournament.html \
        webhandler/templates/admin/tournaments.html
git commit -m "feat: add default ranking criteria to create-tournament forms"
```
