# Division Rules Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a two-tier rules system — tournament-level rules editable by directors/staff (shown on the public tournament page) and division-level rules editable in the existing division edit form (shown on the public division page, with tournament rules as fallback when no division-specific rules exist).

**Architecture:** Two new `rules_html TEXT NOT NULL DEFAULT ''` columns (one on `tournaments`, one on `divisions`) added via migrations. Two new DB interface methods (`SetTournamentRules`, `SetDivisionRules`). Tournament rules managed via a new `/tournaments/:tid/manage/rules` route. Division rules managed by extending the existing `ManageDivisionEdit` handler. Public display uses `htmlSafe` with a template-level fallback on the division page.

**Tech Stack:** Go `database/sql`, Quill 1.3.7 CDN, `html/template`, `httprouter`

---

## File Map

| Action | Path | Purpose |
|--------|------|---------|
| Modify | `mydb/mydb.go` | Add 2 migrations |
| Modify | `mydb/db.go` | Add 2 methods to DB interface |
| Modify | `mydb/tournaments.go` | Add `RulesHTML` to struct; update queries + scan; add `SetTournamentRules` |
| Modify | `mydb/divisions.go` | Add `RulesHTML` to struct; update queries + scan; add `SetDivisionRules` |
| Modify | `mydb/fakedb.go` | Add `SetTournamentRules`, `SetDivisionRules` |
| Modify | `mydb/fakedb_test.go` | Add `TestFakeDB_RulesCRUD` |
| Modify | `webhandler/manage_divisions.go` | Add `ManageRules` handler; extend `ManageDivisionEdit` POST |
| Create | `webhandler/templates/manage/rules.html` | Quill editor for tournament rules |
| Modify | `webhandler/templates/manage/edit_division.html` | Add Quill editor for division rules |
| Modify | `webhandler/templates/manage/dashboard.html` | Add Rules nav link |
| Modify | `webhandler/templates/tournament.html` | Show tournament rules if set |
| Modify | `webhandler/templates/divisions.html` | Show division rules with tournament fallback |
| Modify | `main.go` | Register 2 new routes |

---

### Task 1: DB layer — structs, queries, migrations, methods

**Files:**
- Modify: `mydb/mydb.go`
- Modify: `mydb/db.go`
- Modify: `mydb/tournaments.go`
- Modify: `mydb/divisions.go`

- [ ] **Step 1: Add 2 migrations to `mydb/mydb.go`**

Find the `pgmigrations` slice and append before the closing `}`:

```go
`ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS rules_html TEXT NOT NULL DEFAULT ''`,
`ALTER TABLE divisions  ADD COLUMN IF NOT EXISTS rules_html TEXT NOT NULL DEFAULT ''`,
```

- [ ] **Step 2: Add 2 methods to the DB interface in `mydb/db.go`**

After `SetTournamentExtras(id int, html string)`, add:

```go
SetTournamentRules(id int, html string)
```

After `// Locations` block (or anywhere in the interface — keep it near `SetTournamentExtras`). Also add before the `// Login rate limiting` block:

```go
SetDivisionRules(id int, html string)
```

Full diff in context — find the `SetTournamentExtras` line in `db.go` and add immediately after it:

```go
SetTournamentRules(id int, html string)
```

Find `UpdateDivision(id int, name string)` and add after it:

```go
SetDivisionRules(id int, html string)
```

- [ ] **Step 3: Add `RulesHTML` to `Tournament` struct in `mydb/tournaments.go`**

Change:

```go
type Tournament struct {
	ID         int
	Name       string
	Sport      string
	Location   string
	StartDate  time.Time
	Notes      string
	ExtrasHTML string
	Status     string
}
```

To:

```go
type Tournament struct {
	ID         int
	Name       string
	Sport      string
	Location   string
	StartDate  time.Time
	Notes      string
	ExtrasHTML string
	RulesHTML  string
	Status     string
}
```

- [ ] **Step 4: Update `scanTournaments` to scan `rules_html` in `mydb/tournaments.go`**

Change:

```go
if err := rows.Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes, &t.ExtrasHTML, &t.Status); err != nil {
```

To:

```go
if err := rows.Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes, &t.ExtrasHTML, &t.RulesHTML, &t.Status); err != nil {
```

- [ ] **Step 5: Update all SELECT strings in `mydb/tournaments.go` to include `rules_html`**

Every query that feeds `scanTournaments` uses `extras_html, status` — change each occurrence of `extras_html, status` to `extras_html, rules_html, status`. There are 6 bulk queries plus `ReturnTournamentByID`. Use search-and-replace across the file: change every `extras_html, status` to `extras_html, rules_html, status`.

Also update the `ReturnTournamentByID` inline Scan:

Change:
```go
).Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes, &t.ExtrasHTML, &t.Status)
```
To:
```go
).Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes, &t.ExtrasHTML, &t.RulesHTML, &t.Status)
```

- [ ] **Step 6: Add `SetTournamentRules` to `mydb/tournaments.go`**

After the existing `SetTournamentExtras` method, add:

```go
func (me *MyDB) SetTournamentRules(id int, html string) {
	_, err := me.DB.Exec(`UPDATE tournaments SET rules_html=$1 WHERE id=$2`, html, id)
	if err != nil {
		slog.Error("SetTournamentRules", "err", err)
	}
}
```

- [ ] **Step 7: Add `RulesHTML` to `Division` struct in `mydb/divisions.go`**

Change:

```go
type Division struct {
	ID           int
	TournamentID int
	Name         string
}
```

To:

```go
type Division struct {
	ID           int
	TournamentID int
	Name         string
	RulesHTML    string
}
```

- [ ] **Step 8: Update `ReturnDivisions` in `mydb/divisions.go`**

Change the query and scan:

```go
func (me *MyDB) ReturnDivisions(tournamentID int) []Division {
	rows, err := me.DB.Query(
		`SELECT id, tournament_id, name, rules_html FROM divisions WHERE tournament_id=$1 ORDER BY name`,
		tournamentID,
	)
	if err != nil {
		slog.Error("ReturnDivisions", "err", err)
		return nil
	}
	var out []Division
	for rows.Next() {
		var d Division
		if err := rows.Scan(&d.ID, &d.TournamentID, &d.Name, &d.RulesHTML); err != nil {
			slog.Error("ReturnDivisions scan", "err", err)
			continue
		}
		out = append(out, d)
	}
	rows.Close()
	return out
}
```

- [ ] **Step 9: Update `ReturnDivisionByID` in `mydb/divisions.go`**

```go
func (me *MyDB) ReturnDivisionByID(id int) Division {
	var d Division
	err := me.DB.QueryRow(
		`SELECT id, tournament_id, name, rules_html FROM divisions WHERE id=$1`, id,
	).Scan(&d.ID, &d.TournamentID, &d.Name, &d.RulesHTML)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("ReturnDivisionByID", "err", err)
	}
	return d
}
```

- [ ] **Step 10: Add `SetDivisionRules` to `mydb/divisions.go`**

After `UpdateDivision`, add:

```go
func (me *MyDB) SetDivisionRules(id int, html string) {
	_, err := me.DB.Exec(`UPDATE divisions SET rules_html=$1 WHERE id=$2`, html, id)
	if err != nil {
		slog.Error("SetDivisionRules", "err", err)
	}
}
```

- [ ] **Step 11: Verify build**

```bash
cd /home/jharnish/Work/tourneyweb && go build ./mydb/... 2>&1
```

Expected: fails with "missing method SetTournamentRules" and "missing method SetDivisionRules" on FakeDB (compile-time interface check). That's correct — FakeDB hasn't been updated yet. If there are OTHER errors, fix them first.

- [ ] **Step 12: Commit**

```bash
git add mydb/mydb.go mydb/db.go mydb/tournaments.go mydb/divisions.go
git commit -m "feat: add rules_html to tournaments and divisions — struct, queries, DB interface"
```

---

### Task 2: FakeDB implementation and tests

**Files:**
- Modify: `mydb/fakedb.go`
- Modify: `mydb/fakedb_test.go`

- [ ] **Step 1: Write failing test in `mydb/fakedb_test.go`**

Add at the end of the file:

```go
func TestFakeDB_RulesCRUD(t *testing.T) {
	db := mydb.NewFakeDB()
	date := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	tid := db.AddTournament("Test", "baseball", "Park", "", date, "published")

	// Initially empty tournament rules
	tournament := db.ReturnTournamentByID(tid)
	if tournament.RulesHTML != "" {
		t.Errorf("expected empty rules, got %q", tournament.RulesHTML)
	}

	// Set tournament rules
	db.SetTournamentRules(tid, "<p>Tournament rules</p>")
	tournament = db.ReturnTournamentByID(tid)
	if tournament.RulesHTML != "<p>Tournament rules</p>" {
		t.Errorf("SetTournamentRules: got %q", tournament.RulesHTML)
	}

	// Add division
	db.AddDivision(tid, "12U")
	divs := db.ReturnDivisions(tid)
	if len(divs) != 1 {
		t.Fatalf("expected 1 division, got %d", len(divs))
	}
	did := divs[0].ID

	// Initially empty division rules
	div := db.ReturnDivisionByID(did)
	if div.RulesHTML != "" {
		t.Errorf("expected empty division rules, got %q", div.RulesHTML)
	}

	// Set division rules
	db.SetDivisionRules(did, "<p>Division rules</p>")
	div = db.ReturnDivisionByID(did)
	if div.RulesHTML != "<p>Division rules</p>" {
		t.Errorf("SetDivisionRules: got %q", div.RulesHTML)
	}

	// ReturnDivisions also returns RulesHTML
	divs = db.ReturnDivisions(tid)
	if divs[0].RulesHTML != "<p>Division rules</p>" {
		t.Errorf("ReturnDivisions RulesHTML: got %q", divs[0].RulesHTML)
	}

	// Tournament rules unchanged after setting division rules
	tournament = db.ReturnTournamentByID(tid)
	if tournament.RulesHTML != "<p>Tournament rules</p>" {
		t.Errorf("tournament rules unchanged: got %q", tournament.RulesHTML)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/jharnish/Work/tourneyweb && go test ./mydb/... -run TestFakeDB_RulesCRUD -v 2>&1
```

Expected: FAIL — build error `db.SetTournamentRules undefined`.

- [ ] **Step 3: Add `SetTournamentRules` to `mydb/fakedb.go`**

After the existing `SetTournamentExtras` method:

```go
func (f *FakeDB) SetTournamentRules(id int, html string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.tournaments[id]; ok {
		t.RulesHTML = html
		f.tournaments[id] = t
	}
}
```

- [ ] **Step 4: Add `SetDivisionRules` to `mydb/fakedb.go`**

After the existing `UpdateDivision` method (in the `// --- Divisions ---` section):

```go
func (f *FakeDB) SetDivisionRules(id int, html string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d, ok := f.divisions[id]; ok {
		d.RulesHTML = html
		f.divisions[id] = d
	}
}
```

- [ ] **Step 5: Run all tests to verify they pass**

```bash
cd /home/jharnish/Work/tourneyweb && go test ./mydb/... -v 2>&1
```

Expected: all tests pass including `TestFakeDB_RulesCRUD`.

- [ ] **Step 6: Commit**

```bash
git add mydb/fakedb.go mydb/fakedb_test.go
git commit -m "feat: implement FakeDB rules methods and add CRUD tests"
```

---

### Task 3: Tournament rules manage route

**Files:**
- Modify: `webhandler/manage_divisions.go`
- Create: `webhandler/templates/manage/rules.html`
- Modify: `webhandler/templates/manage/dashboard.html`
- Modify: `main.go`

- [ ] **Step 1: Add `ManageRules` handler to `webhandler/manage_divisions.go`**

Add at the end of the file:

```go
func (me *Env) ManageRules(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		me.DB.SetTournamentRules(t.ID, r.FormValue("rules_html"))
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/rules", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "manageRules", newBaseWithTournament(r, t))
}
```

- [ ] **Step 2: Create `webhandler/templates/manage/rules.html`**

```html
{{define "manageRules"}}
{{template "header" .}}
<link rel="stylesheet" href="https://cdn.quilljs.com/1.3.7/quill.snow.css">
<h2>Tournament Rules &mdash; {{.Tournament.Name}}</h2>
<p>These rules appear on the public tournament page. If a division has no division-specific rules, these rules are shown on the division page as well.</p>
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/rules" id="rules-form">
{{.CSRFField}}
<div id="editor" style="height:400px;margin-bottom:1em;"></div>
<textarea id="existing_content" style="display:none">{{.Tournament.RulesHTML}}</textarea>
<input type="hidden" name="rules_html" id="rules_html_input">
<input type="submit" value="Save">
&nbsp;<a href="/tournaments/{{.Tournament.ID}}/manage">Back to manage</a>
</form>
<script src="https://cdn.quilljs.com/1.3.7/quill.js"></script>
<script>
var quill = new Quill('#editor', {
  theme: 'snow',
  modules: { toolbar: [
    [{ 'header': [1, 2, 3, false] }],
    ['bold', 'italic', 'underline'],
    ['link'],
    [{ 'list': 'ordered' }, { 'list': 'bullet' }],
    ['clean']
  ]}
});
var existing = document.getElementById('existing_content').value;
if (existing.trim()) {
  quill.clipboard.dangerouslyPasteHTML(existing);
}
document.getElementById('rules-form').addEventListener('submit', function() {
  document.getElementById('rules_html_input').value = quill.root.innerHTML;
});
</script>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Add Rules link to `webhandler/templates/manage/dashboard.html`**

After the `<li>` for Event Extras:

```html
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/rules">Rules</a></li>
```

- [ ] **Step 4: Register 2 routes in `main.go`**

After the manage news routes block, add:

```go
router.GET("/tournaments/:tid/manage/rules", wh.ManageRules)
router.POST("/tournaments/:tid/manage/rules", wh.ManageRules)
```

- [ ] **Step 5: Verify build and tests**

```bash
cd /home/jharnish/Work/tourneyweb && go build ./... 2>&1 && go test ./... 2>&1
```

Expected: clean build, all tests pass.

- [ ] **Step 6: Commit**

```bash
git add webhandler/manage_divisions.go webhandler/templates/manage/rules.html webhandler/templates/manage/dashboard.html main.go
git commit -m "feat: add tournament rules manage route and template"
```

---

### Task 4: Division rules in the edit form

**Files:**
- Modify: `webhandler/manage_divisions.go`
- Modify: `webhandler/templates/manage/edit_division.html`

- [ ] **Step 1: Update `ManageDivisionEdit` POST to save division rules**

In `webhandler/manage_divisions.go`, find the `ManageDivisionEdit` handler's POST block:

```go
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		if name == "" {
			http.Error(w, "Division name required", http.StatusBadRequest)
			return
		}
		me.DB.UpdateDivision(did, name)
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
		return
	}
```

Change to:

```go
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		if name == "" {
			http.Error(w, "Division name required", http.StatusBadRequest)
			return
		}
		me.DB.UpdateDivision(did, name)
		me.DB.SetDivisionRules(did, r.FormValue("rules_html"))
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
		return
	}
```

- [ ] **Step 2: Replace `webhandler/templates/manage/edit_division.html` with Quill editor**

Replace the entire file content:

```html
{{define "manageDivisionEdit"}}
{{template "header" .}}
<link rel="stylesheet" href="https://cdn.quilljs.com/1.3.7/quill.snow.css">
<h1>Edit Division</h1>
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/divisions/{{.Division.ID}}/edit" id="division-form">
{{.CSRFField}}
<table>
  <tr><td>Name</td><td><input type="text" name="name" value="{{.Division.Name}}" required></td></tr>
</table>
<h3>Division Rules</h3>
<p>Optional. If left empty, the tournament rules (if any) will be shown on this division's public page.</p>
<div id="editor" style="height:300px;margin-bottom:1em;"></div>
<textarea id="existing_content" style="display:none">{{.Division.RulesHTML}}</textarea>
<input type="hidden" name="rules_html" id="rules_html_input">
<input type="submit" value="Save">
&nbsp;<a href="/tournaments/{{.Tournament.ID}}/manage/divisions">Back</a>
</form>
<script src="https://cdn.quilljs.com/1.3.7/quill.js"></script>
<script>
var quill = new Quill('#editor', {
  theme: 'snow',
  modules: { toolbar: [
    [{ 'header': [1, 2, 3, false] }],
    ['bold', 'italic', 'underline'],
    ['link'],
    [{ 'list': 'ordered' }, { 'list': 'bullet' }],
    ['clean']
  ]}
});
var existing = document.getElementById('existing_content').value;
if (existing.trim()) {
  quill.clipboard.dangerouslyPasteHTML(existing);
}
document.getElementById('division-form').addEventListener('submit', function() {
  document.getElementById('rules_html_input').value = quill.root.innerHTML;
});
</script>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Verify build and tests**

```bash
cd /home/jharnish/Work/tourneyweb && go build ./... 2>&1 && go test ./... 2>&1
```

Expected: clean build, all tests pass.

- [ ] **Step 4: Commit**

```bash
git add webhandler/manage_divisions.go webhandler/templates/manage/edit_division.html
git commit -m "feat: add division rules Quill editor to division edit form"
```

---

### Task 5: Public display

**Files:**
- Modify: `webhandler/templates/tournament.html`
- Modify: `webhandler/templates/divisions.html`

- [ ] **Step 1: Add tournament rules to `webhandler/templates/tournament.html`**

Add before `{{template "footer" .}}`:

```html
{{if .Tournament.RulesHTML}}
<hr>
<h2>Rules</h2>
{{.Tournament.RulesHTML | htmlSafe}}
{{end}}
```

- [ ] **Step 2: Add division rules (with tournament fallback) to `webhandler/templates/divisions.html`**

Add before `{{template "footer" .}}`:

```html
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
```

- [ ] **Step 3: Final build and tests**

```bash
cd /home/jharnish/Work/tourneyweb && go build ./... 2>&1 && go test ./... 2>&1 && go vet ./... 2>&1
```

Expected: all clean.

- [ ] **Step 4: Commit**

```bash
git add webhandler/templates/tournament.html webhandler/templates/divisions.html
git commit -m "feat: show tournament and division rules on public pages with fallback"
```

---

## Self-Review

### Spec Coverage

| Spec Requirement | Task |
|-----------------|------|
| `rules_html` migration on `tournaments` | Task 1 |
| `rules_html` migration on `divisions` | Task 1 |
| `RulesHTML` on `Tournament` struct | Task 1 |
| `RulesHTML` on `Division` struct | Task 1 |
| `ReturnTournamentByID` includes `rules_html` | Task 1 |
| `ReturnDivisionByID` includes `rules_html` | Task 1 |
| `ReturnDivisions` includes `rules_html` | Task 1 |
| `SetTournamentRules` on DB interface + MyDB | Task 1 |
| `SetDivisionRules` on DB interface + MyDB | Task 1 |
| FakeDB `SetTournamentRules` | Task 2 |
| FakeDB `SetDivisionRules` | Task 2 |
| FakeDB tests | Task 2 |
| `ManageRules` handler (GET/POST) | Task 3 |
| `manage/rules.html` Quill template | Task 3 |
| Rules nav link in manage dashboard | Task 3 |
| 2 routes registered | Task 3 |
| `ManageDivisionEdit` POST calls `SetDivisionRules` | Task 4 |
| Division edit template has Quill editor | Task 4 |
| Tournament rules on public tournament page | Task 5 |
| Division rules on public division page | Task 5 |
| Fallback to tournament rules when division has none | Task 5 |
| Directors and staff can access manage routes | Covered by existing manage middleware (no new auth code needed) |
