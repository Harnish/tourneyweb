# Self-Service Tournament Creation & Verification Codes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow any logged-in user to self-create tournaments; admin issues per-tournament verification codes to publish them; directors manage their own divisions, teams, games, and locations.

**Architecture:** Add a `status` column to `tournaments` (draft/published), a new `verification_codes` table, and `tournament_id` scoping on `locations`. A parallel set of `/tournaments/:tid/manage/*` routes gives directors the same management power as admin routes, guarded by the existing `CanManage` middleware check.

**Tech Stack:** Go, PostgreSQL (pgx), html/template (embed.FS), httprouter, gorilla/csrf, rivo/sessions

---

## File Map

**Modified:**
- `mydb/mydb.go` — add pgtable + migrations for new schema
- `mydb/tournaments.go` — `Tournament.Status`, updated queries, `ReturnDraftTournaments`
- `mydb/locations.go` — `Location.TournamentID`, `GetLocationsByTournamentID`, updated `AddLocation` sig
- `mydb/db.go` — interface additions
- `mydb/fakedb.go` — implement all new interface methods
- `mydb/fakedb_test.go` — update `AddTournament` call sites, add new tests
- `webhandler/webhandler.go` — `tournamentFromRoute` gets `r *http.Request`, draft visibility check, `locationsForTournament`
- `webhandler/tournaments.go` — `NewTournamentForm`, `NewTournament`, update `CreateTournament` caller
- `webhandler/extras.go`, `webhandler/divisions.go`, `webhandler/teams.go`, `webhandler/roles.go`, `webhandler/games.go` — update `tournamentFromRoute` call sites
- `webhandler/locations.go` — update `AddLocation` call to pass `tournamentID=0`
- `webhandler/templates.go` — new data types, add `"templates/manage/*.html"` to ParseFS
- `webhandler/templates/layout.html` — nav links for Create Tournament + Manage
- `webhandler/templates/admin/tournaments.html` — link to queue page
- `main.go` — register all new routes

**Created:**
- `mydb/verification_codes.go` — `VerificationCode` struct, `IssueVerificationCode`, `RedeemVerificationCode`
- `webhandler/verification.go` — `TournamentQueue`, `IssueCode`, `ManagePublish`
- `webhandler/manage.go` — `ManageDashboard`
- `webhandler/manage_divisions.go` — `ManageDivisions`, `ManageDivisionEdit`, `ManageDivisionDelete`
- `webhandler/manage_teams.go` — `ManageTeams`, `ManageTeamEdit`, `ManageTeamDelete`
- `webhandler/manage_locations.go` — `ManageLocations`, `ManageLocationEdit`, `ManageLocationDelete`
- `webhandler/manage_games.go` — `ManageCreateGame`, `ManageCreateGameSubmit`, `ManageGenerateGames`, `ManageEditGame`, `ManageDeleteGame`
- `webhandler/templates/manage/` — new directory with all manage templates

---

## Task 1: DB — Tournament.Status + schema migrations

**Files:**
- Modify: `mydb/mydb.go`
- Modify: `mydb/tournaments.go`
- Modify: `mydb/db.go`
- Modify: `mydb/fakedb.go`
- Modify: `mydb/fakedb_test.go`
- Modify: `webhandler/tournaments.go` (update `AddTournament` call in `CreateTournament`)

- [ ] **Step 1: Write failing tests in `mydb/fakedb_test.go`**

Add this test function at the end of the file:

```go
func TestFakeDB_TournamentStatus(t *testing.T) {
	db := mydb.NewFakeDB()
	future := time.Date(2027, 8, 1, 0, 0, 0, 0, time.UTC)

	draftID := db.AddTournament("Draft T", "baseball", "City", "", future, "draft")
	pubID := db.AddTournament("Pub T", "baseball", "City", "", future, "published")

	if db.ReturnTournamentByID(draftID).Status != "draft" {
		t.Error("expected draft status")
	}
	if db.ReturnTournamentByID(pubID).Status != "published" {
		t.Error("expected published status")
	}

	drafts := db.ReturnDraftTournaments()
	foundDraft := false
	for _, d := range drafts {
		if d.ID == draftID {
			foundDraft = true
		}
		if d.ID == pubID {
			t.Error("published tournament appeared in ReturnDraftTournaments")
		}
	}
	if !foundDraft {
		t.Error("draft tournament not in ReturnDraftTournaments")
	}

	futureTourneys, _ := db.ReturnTournamentsFuture(1)
	for _, tt := range futureTourneys {
		if tt.ID == draftID {
			t.Error("draft tournament appeared in public future listing")
		}
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./mydb/... 2>&1 | head -20
```

Expected: compile error — `AddTournament` called with wrong number of args (existing tests pass 5 args).

- [ ] **Step 3: Update `Tournament` struct in `mydb/tournaments.go`**

Add `Status string` field and update `scanTournaments`:

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

func scanTournaments(rows *sql.Rows) []Tournament {
	var out []Tournament
	for rows.Next() {
		var t Tournament
		if err := rows.Scan(&t.ID, &t.Name, &t.Sport, &t.Location, &t.StartDate, &t.Notes, &t.ExtrasHTML, &t.Status); err != nil {
			slog.Error("scanTournaments", "err", err)
			continue
		}
		out = append(out, t)
	}
	rows.Close()
	return out
}
```

- [ ] **Step 4: Update `AddTournament` in `mydb/tournaments.go`**

```go
func (me *MyDB) AddTournament(name, sport, location, notes string, date time.Time, status string) int {
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO tournaments (name, sport, location, start_date, notes, status) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		name, sport, location, date, notes, status,
	).Scan(&id)
	if err != nil {
		slog.Error("AddTournament", "err", err)
	}
	return id
}
```

- [ ] **Step 5: Update all `SELECT` queries in `mydb/tournaments.go` to include `status`**

Replace every occurrence of:
```sql
SELECT id, name, sport, location, start_date, notes, extras_html FROM tournaments
```
with:
```sql
SELECT id, name, sport, location, start_date, notes, extras_html, status FROM tournaments
```

That affects: `ReturnTournaments`, `ReturnTournamentByID`, `ReturnTournamentsComingUp`, `ReturnTournamentsRecent`, `ReturnTournamentsFuture`, `ReturnTournamentsPast`.

- [ ] **Step 6: Add `WHERE status='published'` to public listing queries in `mydb/tournaments.go`**

`ReturnTournamentsComingUp` — change WHERE clause to:
```sql
WHERE status='published' AND start_date >= CURRENT_DATE AND start_date <= CURRENT_DATE + INTERVAL '7 days'
```

`ReturnTournamentsRecent` — change WHERE clause to:
```sql
WHERE status='published' AND start_date >= CURRENT_DATE - INTERVAL '7 days' AND start_date < CURRENT_DATE
```

`ReturnTournamentsFuture` — both the COUNT and SELECT queries:
```sql
-- count:
SELECT COUNT(*) FROM tournaments WHERE status='published' AND start_date > CURRENT_DATE + INTERVAL '7 days'
-- rows:
SELECT ... FROM tournaments WHERE status='published' AND start_date > CURRENT_DATE + INTERVAL '7 days' ORDER BY start_date ASC LIMIT 20 OFFSET $1
```

`ReturnTournamentsPast` — both COUNT and SELECT:
```sql
-- count:
SELECT COUNT(*) FROM tournaments WHERE status='published' AND start_date < CURRENT_DATE - INTERVAL '7 days'
-- rows:
SELECT ... FROM tournaments WHERE status='published' AND start_date < CURRENT_DATE - INTERVAL '7 days' ORDER BY start_date DESC LIMIT 20 OFFSET $1
```

`ReturnTournaments` (admin — no filter, shows all):
```sql
SELECT id, name, sport, location, start_date, notes, extras_html, status FROM tournaments ORDER BY start_date DESC
```

- [ ] **Step 7: Add `ReturnDraftTournaments` to `mydb/tournaments.go`**

```go
func (me *MyDB) ReturnDraftTournaments() []Tournament {
	rows, err := me.DB.Query(
		`SELECT id, name, sport, location, start_date, notes, extras_html, status FROM tournaments WHERE status='draft' ORDER BY start_date ASC`,
	)
	if err != nil {
		slog.Error("ReturnDraftTournaments", "err", err)
		return nil
	}
	return scanTournaments(rows)
}
```

- [ ] **Step 8: Add migrations in `mydb/mydb.go`**

Add the `verification_codes` table to `pgtables` (before the closing `}`):
```go
`CREATE TABLE IF NOT EXISTS verification_codes (
    id            SERIAL PRIMARY KEY,
    tournament_id INTEGER NOT NULL REFERENCES tournaments(id),
    code          TEXT NOT NULL UNIQUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    redeemed_at   TIMESTAMPTZ
)`,
```

Add two entries to `pgmigrations`:
```go
`ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'published'`,
`ALTER TABLE locations ADD COLUMN IF NOT EXISTS tournament_id INTEGER REFERENCES tournaments(id)`,
```

- [ ] **Step 9: Update `mydb/db.go` interface**

Change `AddTournament` signature and add `ReturnDraftTournaments`:

```go
AddTournament(name, sport, location, notes string, date time.Time, status string) int
ReturnDraftTournaments() []Tournament
```

- [ ] **Step 10: Update `mydb/fakedb.go`**

Update `AddTournament` to accept and store status:
```go
func (f *FakeDB) AddTournament(name, sport, location, notes string, date time.Time, status string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	f.tournaments[id] = Tournament{ID: id, Name: name, Sport: sport, Location: location, Notes: notes, StartDate: date, Status: status}
	return id
}
```

Update `ReturnTournamentsComingUp`, `ReturnTournamentsRecent`, `ReturnTournamentsFuture`, `ReturnTournamentsPast` to skip drafts. Example for `ReturnTournamentsComingUp`:
```go
func (f *FakeDB) ReturnTournamentsComingUp() []Tournament {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	cutoff := now.AddDate(0, 0, 7)
	var out []Tournament
	for _, t := range f.tournaments {
		if t.Status == "published" && !t.StartDate.Before(now) && !t.StartDate.After(cutoff) {
			out = append(out, t)
		}
	}
	return out
}
```

Apply the same `t.Status == "published"` filter to `ReturnTournamentsRecent`, and both the inner loops in `ReturnTournamentsFuture` and `ReturnTournamentsPast`.

Add `ReturnDraftTournaments`:
```go
func (f *FakeDB) ReturnDraftTournaments() []Tournament {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Tournament
	for _, t := range f.tournaments {
		if t.Status == "draft" {
			out = append(out, t)
		}
	}
	return out
}
```

- [ ] **Step 11: Update all `AddTournament` calls in `mydb/fakedb_test.go`**

Find every call: `grep -n "AddTournament" mydb/fakedb_test.go`

Each call that currently passes 5 args needs a 6th arg `"published"`:
- Line 13: `db.AddTournament("Summer Classic", "baseball", "City Park", "notes", date, "published")`
- Line 33: `db.AddTournament("T", "baseball", "L", "", time.Now(), "published")`
- Line 153: `db.AddTournament("T", "baseball", "L", "", time.Now(), "published")`
- Line 179: `db.AddTournament("T", "baseball", "L", "", time.Now(), "published")`

- [ ] **Step 12: Update `CreateTournament` in `webhandler/tournaments.go`**

Change line `id := me.DB.AddTournament(name, sport, location, notes, date)` to:
```go
id := me.DB.AddTournament(name, sport, location, notes, date, "published")
```

- [ ] **Step 13: Run tests and build**

```bash
go test ./mydb/... -v 2>&1 | tail -20
go build ./...
```

Expected: all tests pass, build succeeds.

- [ ] **Step 14: Commit**

```bash
git add mydb/mydb.go mydb/tournaments.go mydb/db.go mydb/fakedb.go mydb/fakedb_test.go webhandler/tournaments.go
git commit -m "feat: add Tournament.Status, draft/published filtering, verification_codes schema"
```

---

## Task 2: DB — Verification codes

**Files:**
- Create: `mydb/verification_codes.go`
- Modify: `mydb/db.go`
- Modify: `mydb/fakedb.go`
- Modify: `mydb/fakedb_test.go`

- [ ] **Step 1: Write failing test in `mydb/fakedb_test.go`**

```go
func TestFakeDB_VerificationCodes(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "draft")
	otherTID := db.AddTournament("T2", "baseball", "L", "", time.Now(), "draft")

	code, err := db.IssueVerificationCode(tid)
	if err != nil {
		t.Fatalf("IssueVerificationCode: %v", err)
	}
	if len(code) != 8 {
		t.Errorf("expected 8-char code, got %q (len %d)", code, len(code))
	}

	// Wrong tournament → error
	if err := db.RedeemVerificationCode(code, otherTID); err == nil {
		t.Error("expected error for wrong tournament")
	}
	// Tournament not yet published
	if db.ReturnTournamentByID(tid).Status != "draft" {
		t.Error("should still be draft after failed redemption")
	}

	// Valid redemption
	if err := db.RedeemVerificationCode(code, tid); err != nil {
		t.Fatalf("RedeemVerificationCode: %v", err)
	}
	if db.ReturnTournamentByID(tid).Status != "published" {
		t.Error("tournament should be published after redemption")
	}

	// Cannot reuse
	if err := db.RedeemVerificationCode(code, tid); err == nil {
		t.Error("expected error for already-used code")
	}
}
```

- [ ] **Step 2: Run test — confirm compile error**

```bash
go test ./mydb/... 2>&1 | head -10
```

Expected: `db.IssueVerificationCode undefined`.

- [ ] **Step 3: Create `mydb/verification_codes.go`**

```go
package mydb

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"
)

type VerificationCode struct {
	ID           int
	TournamentID int
	Code         string
	CreatedAt    time.Time
	RedeemedAt   time.Time
	Redeemed     bool
}

func generateVerificationCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}

func (me *MyDB) IssueVerificationCode(tournamentID int) (string, error) {
	code, err := generateVerificationCode()
	if err != nil {
		return "", err
	}
	_, err = me.DB.Exec(
		`INSERT INTO verification_codes (tournament_id, code) VALUES ($1, $2)`,
		tournamentID, code,
	)
	if err != nil {
		slog.Error("IssueVerificationCode", "err", err)
		return "", err
	}
	return code, nil
}

func (me *MyDB) RedeemVerificationCode(code string, tournamentID int) error {
	var id, storedTID int
	var redeemed bool
	err := me.DB.QueryRow(
		`SELECT id, tournament_id, redeemed_at IS NOT NULL FROM verification_codes WHERE code=$1`,
		code,
	).Scan(&id, &storedTID, &redeemed)
	if err == sql.ErrNoRows {
		return errors.New("invalid code")
	}
	if err != nil {
		return err
	}
	if storedTID != tournamentID {
		return errors.New("invalid code")
	}
	if redeemed {
		return errors.New("code already used")
	}
	if _, err = me.DB.Exec(`UPDATE verification_codes SET redeemed_at=NOW() WHERE id=$1`, id); err != nil {
		return err
	}
	_, err = me.DB.Exec(`UPDATE tournaments SET status='published' WHERE id=$1`, tournamentID)
	return err
}
```

- [ ] **Step 4: Add interface methods to `mydb/db.go`**

In the `// Tournaments` section of the `DB` interface, add:
```go
IssueVerificationCode(tournamentID int) (string, error)
RedeemVerificationCode(code string, tournamentID int) error
```

- [ ] **Step 5: Add FakeDB fields and implement in `mydb/fakedb.go`**

Add a private struct and a slice field to `FakeDB`:

At the top of `fakedb.go`, after the `gameByTeamRow` struct:
```go
type fakeVerifCode struct {
	id           int
	tournamentID int
	code         string
	redeemed     bool
}
```

Add `verifCodes []fakeVerifCode` to the `FakeDB` struct definition.

Implement the methods (add after the `--- Locations ---` section):
```go
// --- Verification codes ---

func (f *FakeDB) IssueVerificationCode(tournamentID int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	code, err := generateVerificationCode()
	if err != nil {
		return "", err
	}
	f.verifCodes = append(f.verifCodes, fakeVerifCode{id: f.newID(), tournamentID: tournamentID, code: code})
	return code, nil
}

func (f *FakeDB) RedeemVerificationCode(code string, tournamentID int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, vc := range f.verifCodes {
		if vc.code == code {
			if vc.tournamentID != tournamentID {
				return errors.New("invalid code")
			}
			if vc.redeemed {
				return errors.New("code already used")
			}
			f.verifCodes[i].redeemed = true
			if t, ok := f.tournaments[tournamentID]; ok {
				t.Status = "published"
				f.tournaments[tournamentID] = t
			}
			return nil
		}
	}
	return errors.New("invalid code")
}
```

Add `"errors"` to fakedb.go imports.

- [ ] **Step 6: Run tests**

```bash
go test ./mydb/... -v -run TestFakeDB_VerificationCodes
```

Expected: PASS.

- [ ] **Step 7: Build**

```bash
go build ./...
```

- [ ] **Step 8: Commit**

```bash
git add mydb/verification_codes.go mydb/db.go mydb/fakedb.go mydb/fakedb_test.go
git commit -m "feat: add verification codes DB layer"
```

---

## Task 3: DB — Per-tournament locations

**Files:**
- Modify: `mydb/locations.go`
- Modify: `mydb/db.go`
- Modify: `mydb/fakedb.go`
- Modify: `mydb/fakedb_test.go`
- Modify: `webhandler/locations.go` (update `AddLocation` call)

- [ ] **Step 1: Write failing test in `mydb/fakedb_test.go`**

```go
func TestFakeDB_TournamentLocations(t *testing.T) {
	db := mydb.NewFakeDB()
	tid := db.AddTournament("T", "baseball", "L", "", time.Now(), "published")

	db.AddLocation("Global Field", "123 Main", "", 0, 0, 0)
	db.AddLocation("Home Field", "456 Oak", "", 40.0, -75.0, tid)

	all := db.GetLocations()
	if len(all) != 2 {
		t.Fatalf("GetLocations: expected 2, got %d", len(all))
	}

	byTID := db.GetLocationsByTournamentID(tid)
	if len(byTID) != 1 {
		t.Fatalf("GetLocationsByTournamentID: expected 1, got %d", len(byTID))
	}
	if byTID[0].Name != "Home Field" {
		t.Errorf("expected Home Field, got %q", byTID[0].Name)
	}
	if byTID[0].TournamentID != tid {
		t.Errorf("TournamentID: got %d, want %d", byTID[0].TournamentID, tid)
	}
}
```

- [ ] **Step 2: Run — confirm compile error**

```bash
go test ./mydb/... 2>&1 | head -10
```

Expected: `AddLocation` called with 6 args (currently takes 5), `GetLocationsByTournamentID` undefined.

- [ ] **Step 3: Update `Location` struct in `mydb/locations.go`**

```go
type Location struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Address      string  `json:"address"`
	Latitude     float64 `json:"lat"`
	Longitude    float64 `json:"lng"`
	AvailableFor string  `json:"available_for"`
	TournamentID int     `json:"tournament_id"`
}
```

- [ ] **Step 4: Update `AddLocation` in `mydb/locations.go`**

```go
func (me *MyDB) AddLocation(name, address, availableFor string, lat, lng float64, tournamentID int) int {
	var id int
	err := me.DB.QueryRow(
		`INSERT INTO locations (name, address, available_for, latitude, longitude, tournament_id)
		 VALUES ($1,$2,$3,$4,$5, NULLIF($6, 0)) RETURNING id`,
		name, address, availableFor, lat, lng, tournamentID,
	).Scan(&id)
	if err != nil {
		slog.Error("AddLocation", "err", err)
	}
	return id
}
```

- [ ] **Step 5: Update `GetLocations` scan in `mydb/locations.go`**

Change the SELECT and scan to include tournament_id:

```go
func (me *MyDB) GetLocations() []Location {
	rows, err := me.DB.Query(
		`SELECT id, name, address, latitude, longitude, available_for, COALESCE(tournament_id, 0) FROM locations ORDER BY name`)
	if err != nil {
		slog.Error("GetLocations", "err", err)
		return nil
	}
	defer rows.Close()
	var out []Location
	for rows.Next() {
		var l Location
		if err := rows.Scan(&l.ID, &l.Name, &l.Address, &l.Latitude, &l.Longitude, &l.AvailableFor, &l.TournamentID); err != nil {
			slog.Error("GetLocations scan", "err", err)
			continue
		}
		out = append(out, l)
	}
	return out
}
```

- [ ] **Step 6: Update `GetLocationByID` scan in `mydb/locations.go`**

```go
func (me *MyDB) GetLocationByID(id int) Location {
	var l Location
	err := me.DB.QueryRow(
		`SELECT id, name, address, latitude, longitude, available_for, COALESCE(tournament_id, 0) FROM locations WHERE id=$1`, id,
	).Scan(&l.ID, &l.Name, &l.Address, &l.Latitude, &l.Longitude, &l.AvailableFor, &l.TournamentID)
	if err != nil && err != sql.ErrNoRows {
		slog.Error("GetLocationByID", "err", err)
	}
	return l
}
```

- [ ] **Step 7: Add `GetLocationsByTournamentID` in `mydb/locations.go`**

```go
func (me *MyDB) GetLocationsByTournamentID(tournamentID int) []Location {
	rows, err := me.DB.Query(
		`SELECT id, name, address, latitude, longitude, available_for, tournament_id FROM locations WHERE tournament_id=$1 ORDER BY name`,
		tournamentID,
	)
	if err != nil {
		slog.Error("GetLocationsByTournamentID", "err", err)
		return nil
	}
	defer rows.Close()
	var out []Location
	for rows.Next() {
		var l Location
		if err := rows.Scan(&l.ID, &l.Name, &l.Address, &l.Latitude, &l.Longitude, &l.AvailableFor, &l.TournamentID); err != nil {
			slog.Error("GetLocationsByTournamentID scan", "err", err)
			continue
		}
		out = append(out, l)
	}
	return out
}
```

- [ ] **Step 8: Update `mydb/db.go` interface**

Change `AddLocation` signature and add `GetLocationsByTournamentID`:
```go
AddLocation(name, address, availableFor string, lat, lng float64, tournamentID int) int
GetLocationsByTournamentID(tournamentID int) []Location
```

- [ ] **Step 9: Update `mydb/fakedb.go`**

Update `AddLocation`:
```go
func (f *FakeDB) AddLocation(name, address, availableFor string, lat, lng float64, tournamentID int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.newID()
	f.locations[id] = Location{ID: id, Name: name, Address: address, AvailableFor: availableFor, Latitude: lat, Longitude: lng, TournamentID: tournamentID}
	return id
}
```

Add `GetLocationsByTournamentID`:
```go
func (f *FakeDB) GetLocationsByTournamentID(tournamentID int) []Location {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Location
	for _, l := range f.locations {
		if l.TournamentID == tournamentID {
			out = append(out, l)
		}
	}
	return out
}
```

- [ ] **Step 10: Update `AddLocation` call in `webhandler/locations.go`**

In `Locations` handler, change:
```go
me.DB.AddLocation(name, address, availableFor, lat, lng)
```
to:
```go
me.DB.AddLocation(name, address, availableFor, lat, lng, 0)
```

- [ ] **Step 11: Run tests and build**

```bash
go test ./mydb/... -v -run TestFakeDB_TournamentLocations
go build ./...
```

Expected: test passes, build succeeds.

- [ ] **Step 12: Commit**

```bash
git add mydb/locations.go mydb/db.go mydb/fakedb.go mydb/fakedb_test.go webhandler/locations.go
git commit -m "feat: add per-tournament location scoping to DB layer"
```

---

## Task 4: Visibility enforcement + tournamentFromRoute refactor

**Files:**
- Modify: `webhandler/webhandler.go`
- Modify: `webhandler/tournaments.go`
- Modify: `webhandler/extras.go`
- Modify: `webhandler/divisions.go`
- Modify: `webhandler/teams.go`
- Modify: `webhandler/roles.go`
- Modify: `webhandler/games.go`

- [ ] **Step 1: Update `tournamentFromRoute` signature in `webhandler/webhandler.go`**

Change the function signature and add draft visibility check. The full updated function in `webhandler/tournaments.go` (it lives there):

```go
func (me *Env) tournamentFromRoute(w http.ResponseWriter, r *http.Request, ps httprouter.Params) (mydb.Tournament, bool) {
	tid, err := strconv.Atoi(ps.ByName("tid"))
	if err != nil {
		http.Error(w, "Bad tournament ID", http.StatusBadRequest)
		return mydb.Tournament{}, false
	}
	t := me.DB.ReturnTournamentByID(tid)
	if t.ID == 0 {
		http.Error(w, "Tournament not found", http.StatusNotFound)
		return mydb.Tournament{}, false
	}
	if t.Status == "draft" {
		user := userFromContext(r.Context())
		if !user.IsAdmin && !user.IsDirectorFor(t.ID) && !user.IsStaffFor(t.ID) {
			http.NotFound(w, r)
			return mydb.Tournament{}, false
		}
	}
	return t, true
}
```

- [ ] **Step 2: Update all call sites across all handler files**

Run to find every caller:
```bash
grep -rn "tournamentFromRoute" webhandler/
```

In every file, change:
```go
me.tournamentFromRoute(w, ps)
```
to:
```go
me.tournamentFromRoute(w, r, ps)
```

Files with callers: `webhandler/webhandler.go` (PrintDivision, DelGame, ScoreGame, Games, AdminGames, CreateGame, CreateGameSubmit, EditGame), `webhandler/tournaments.go` (TournamentHome, AdminTournamentView, EditTournament), `webhandler/extras.go` (TournamentExtras, ManageExtras), `webhandler/divisions.go` (AddDivisionForm, DeleteDivision, AdminDivisionView, EditDivision), `webhandler/teams.go` (Teams, DeleteTeam, EditTeam, ShowTeam), `webhandler/roles.go` (ManageRoles, AssignRole, InviteUser, RemoveRole), `webhandler/games.go` (GenerateGames).

Use sed to update all at once:
```bash
sed -i 's/me\.tournamentFromRoute(w, ps)/me.tournamentFromRoute(w, r, ps)/g' \
  webhandler/webhandler.go webhandler/tournaments.go webhandler/extras.go \
  webhandler/divisions.go webhandler/teams.go webhandler/roles.go webhandler/games.go
```

- [ ] **Step 3: Build to confirm no remaining mismatches**

```bash
go build ./...
```

Expected: builds clean. If any `too many arguments` or `not enough arguments` errors appear, grep for remaining `tournamentFromRoute` calls and fix them.

- [ ] **Step 4: Commit**

```bash
git add webhandler/webhandler.go webhandler/tournaments.go webhandler/extras.go \
  webhandler/divisions.go webhandler/teams.go webhandler/roles.go webhandler/games.go
git commit -m "feat: draft visibility enforcement in tournamentFromRoute"
```

---

## Task 5: Template infrastructure + Self-service tournament creation

**Files:**
- Modify: `webhandler/templates.go`
- Modify: `webhandler/tournaments.go`
- Create: `webhandler/templates/manage/` (directory)
- Create: `webhandler/templates/manage/new_tournament.html`
- Modify: `main.go`

- [ ] **Step 1: Add `"templates/manage/*.html"` to ParseFS in `webhandler/templates.go`**

Change the `ParseFS` call:
```go
tmpl = template.Must(template.New("").Funcs(template.FuncMap{
    // ... existing funcs unchanged ...
}).ParseFS(templateFS,
    "templates/*.html",
    "templates/admin/*.html",
    "templates/auth/*.html",
    "templates/manage/*.html",
))
```

- [ ] **Step 2: Add new data types to `webhandler/templates.go`**

Add before the `renderError` function:
```go
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
	DisableDelete bool
}

type manageDivisionEditData struct {
	baseData
	Division mydb.Division
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
	Location mydb.Location
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
```

- [ ] **Step 3: Add `NewTournamentForm` and `NewTournament` handlers in `webhandler/tournaments.go`**

```go
func (me *Env) NewTournamentForm(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	me.render(w, "newTournament", newBase(r))
}

func (me *Env) NewTournament(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	user := userFromContext(r.Context())
	if !user.LoggedIn() {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
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
	status := "draft"
	if user.IsAdmin {
		status = "published"
	}
	id := me.DB.AddTournament(name, sport, location, notes, date, status)
	me.DB.AssignRole(user.ID, id, "director", 0)
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage", id), http.StatusSeeOther)
}
```

- [ ] **Step 4: Create `webhandler/templates/manage/new_tournament.html`**

```html
{{define "newTournament"}}
{{template "header" .}}
<h1>Create Tournament</h1>
<form method="post" action="/tournaments/new">
{{.CSRFField}}
<table>
  <tr><td>Name</td><td><input type="text" name="name" required></td></tr>
  <tr><td>Sport</td><td><input type="text" name="sport" required></td></tr>
  <tr><td>Location/Venue</td><td><input type="text" name="location" required></td></tr>
  <tr><td>Start Date</td><td><input type="date" name="start_date" required></td></tr>
  <tr><td>Notes</td><td><input type="text" name="notes"></td></tr>
  <tr><td></td><td><input type="submit" value="Create Tournament"></td></tr>
</table>
</form>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 5: Add routes in `main.go`**

After the existing public routes block, add:
```go
// Self-service tournament creation (requires login, enforced in handler)
router.GET("/tournaments/new", wh.NewTournamentForm)
router.POST("/tournaments/new", wh.NewTournament)
```

- [ ] **Step 6: Build**

```bash
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add webhandler/templates.go webhandler/tournaments.go \
  webhandler/templates/manage/new_tournament.html main.go
git commit -m "feat: self-service tournament creation at /tournaments/new"
```

---

## Task 6: Manage dashboard + nav update

**Files:**
- Create: `webhandler/manage.go`
- Create: `webhandler/templates/manage/dashboard.html`
- Modify: `webhandler/templates/layout.html`
- Modify: `webhandler/templates/admin/tournaments.html`
- Modify: `main.go`

- [ ] **Step 1: Create `webhandler/manage.go`**

```go
package webhandler

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) ManageDashboard(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	me.render(w, "manageDashboard", manageDashboardData{
		baseData: newBaseWithTournament(r, t),
		IsDraft:  t.Status == "draft",
	})
}
```

- [ ] **Step 2: Create `webhandler/templates/manage/dashboard.html`**

```html
{{define "manageDashboard"}}
{{template "header" .}}
<h1>Manage: {{.Tournament.Name}}</h1>
{{if .IsDraft}}
<div style="background:#fff3cd;border:1px solid #ffc107;padding:10px;margin-bottom:15px">
  This tournament is a <strong>draft</strong> and not publicly visible.
  <a href="/tournaments/{{.Tournament.ID}}/manage/publish">Enter verification code to publish &rarr;</a>
</div>
{{end}}
<ul>
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/divisions">Divisions &amp; Games</a></li>
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/teams">Teams</a></li>
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/locations">Locations/Fields</a></li>
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/roles">Roles &amp; Staff</a></li>
  <li><a href="/tournaments/{{.Tournament.ID}}/manage/extras">Event Extras</a></li>
  {{if .IsDraft}}<li><a href="/tournaments/{{.Tournament.ID}}/manage/publish">Publish Tournament</a></li>{{end}}
</ul>
<p><a href="/tournaments/{{.Tournament.ID}}">View Public Page</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Update `webhandler/templates/layout.html` nav**

Replace the `{{else if .User.LoggedIn}}` tournament block:
```html
{{else if .User.LoggedIn}}
  {{if .Tournament.ID}}
    {{if .User.IsDirectorFor .Tournament.ID}}
    | <a href="/tournaments/{{.Tournament.ID}}/manage">Manage</a>
    {{end}}
    {{if .User.CanScore .Tournament.ID}}
    | Score Games
    {{end}}
  {{end}}
```

Also add a "Create Tournament" link for all logged-in users. Change the logout line:
```html
{{if .User.LoggedIn}}
| <a href="/tournaments/new">Create Tournament</a>
| {{.User.Name}} (<form method="post" action="/logout" style="display:inline">{{.CSRFField}}<input type="submit" value="Log out"></form>)
```

- [ ] **Step 4: Link to queue from admin tournament list**

In `webhandler/templates/admin/tournaments.html`, add after the `<h1>Tournaments</h1>` line:
```html
<p><a href="/admin/tournaments/queue">Verification Queue (Draft Tournaments)</a></p>
```

- [ ] **Step 5: Add route in `main.go`**

```go
router.GET("/tournaments/:tid/manage", wh.ManageDashboard)
```

- [ ] **Step 6: Build**

```bash
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add webhandler/manage.go webhandler/templates/manage/dashboard.html \
  webhandler/templates/layout.html webhandler/templates/admin/tournaments.html main.go
git commit -m "feat: manage dashboard and nav links"
```

---

## Task 7: Admin verification queue + issue code

**Files:**
- Create: `webhandler/verification.go`
- Create: `webhandler/templates/admin/queue.html`
- Modify: `main.go`

- [ ] **Step 1: Create `webhandler/verification.go`**

```go
package webhandler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) TournamentQueue(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	me.render(w, "adminQueue", adminQueueData{
		baseData: newBase(r),
		Drafts:   me.DB.ReturnDraftTournaments(),
	})
}

func (me *Env) IssueCode(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tid, err := strconv.Atoi(ps.ByName("tid"))
	if err != nil {
		http.Error(w, "Bad tournament ID", http.StatusBadRequest)
		return
	}
	t := me.DB.ReturnTournamentByID(tid)
	if t.ID == 0 {
		http.Error(w, "Tournament not found", http.StatusNotFound)
		return
	}
	code, err := me.DB.IssueVerificationCode(tid)
	if err != nil {
		http.Error(w, "Could not issue code", http.StatusInternalServerError)
		return
	}
	me.render(w, "adminQueue", adminQueueData{
		baseData:   newBase(r),
		Drafts:     me.DB.ReturnDraftTournaments(),
		IssuedCode: code,
		IssuedFor:  t.Name,
	})
}

func (me *Env) ManagePublish(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if t.Status == "published" {
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage", t.ID), http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		code := r.FormValue("code")
		if err := me.DB.RedeemVerificationCode(code, t.ID); err != nil {
			me.render(w, "managePublish", managePublishData{
				baseData: newBaseWithTournament(r, t),
				Error:    "Invalid or already-used code.",
			})
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "managePublish", managePublishData{
		baseData: newBaseWithTournament(r, t),
	})
}
```

- [ ] **Step 2: Create `webhandler/templates/admin/queue.html`**

```html
{{define "adminQueue"}}
{{template "header" .}}
<h1>Verification Queue</h1>
{{if .IssuedCode}}
<div style="background:#d4edda;border:1px solid #28a745;padding:10px;margin-bottom:15px">
  <strong>Code for "{{.IssuedFor}}":</strong> <code style="font-size:1.3em">{{.IssuedCode}}</code>
  <p>Share this code with the tournament director so they can publish their tournament.</p>
</div>
{{end}}
{{if .Drafts}}
<table border="1" cellpadding="4" cellspacing="0">
<tr><th>Name</th><th>Sport</th><th>Location</th><th>Start Date</th><th>Action</th></tr>
{{range .Drafts}}
<tr>
  <td>{{.Name}}</td>
  <td>{{.Sport}}</td>
  <td>{{.Location}}</td>
  <td>{{.StartDate.Format "Jan 2, 2006"}}</td>
  <td>
    <form method="post" action="/admin/tournaments/{{.ID}}/issue-code">
      {{$.CSRFField}}
      <input type="submit" value="Issue Code">
    </form>
  </td>
</tr>
{{end}}
</table>
{{else}}
<p>No draft tournaments pending.</p>
{{end}}
<p><a href="/admin/tournaments">Back to tournaments</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Create `webhandler/templates/manage/publish.html`**

```html
{{define "managePublish"}}
{{template "header" .}}
<h1>Publish Tournament</h1>
<p>Enter the verification code provided by an admin to make <strong>{{.Tournament.Name}}</strong> publicly visible.</p>
{{if .Error}}
<p style="color:red">{{.Error}}</p>
{{end}}
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/publish">
{{.CSRFField}}
<table>
  <tr><td>Verification Code</td><td><input type="text" name="code" required autofocus style="font-size:1.2em;letter-spacing:0.1em"></td></tr>
  <tr><td></td><td><input type="submit" value="Publish Tournament"></td></tr>
</table>
</form>
<p><a href="/tournaments/{{.Tournament.ID}}/manage">Back to manage</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 4: Add routes in `main.go`**

In the Admin routes section:
```go
router.GET("/admin/tournaments/queue", wh.TournamentQueue)
router.POST("/admin/tournaments/:tid/issue-code", wh.IssueCode)
```

**Important:** `GET /admin/tournaments/queue` must be registered **before** `GET /admin/tournaments/:tid` in `main.go`. httprouter prioritizes static segments, so order doesn't actually matter — but place it before `:tid` routes for clarity.

In the Director manage routes section:
```go
router.GET("/tournaments/:tid/manage/publish", wh.ManagePublish)
router.POST("/tournaments/:tid/manage/publish", wh.ManagePublish)
```

- [ ] **Step 5: Build**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add webhandler/verification.go webhandler/templates/admin/queue.html \
  webhandler/templates/manage/publish.html main.go
git commit -m "feat: admin verification queue, issue code, and director publish flow"
```

---

## Task 8: Manage divisions

**Files:**
- Create: `webhandler/manage_divisions.go`
- Create: `webhandler/templates/manage/divisions.html`
- Create: `webhandler/templates/manage/edit_division.html`
- Modify: `main.go`

- [ ] **Step 1: Create `webhandler/manage_divisions.go`**

```go
package webhandler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) ManageDivisions(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
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
	me.render(w, "manageDivisions", manageDivisionsData{
		baseData:      newBaseWithTournament(r, t),
		Divisions:     me.DB.ReturnDivisions(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) ManageDivisionEdit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		slog.Error("ManageDivisionEdit bad ID", "err", err)
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
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "manageDivisionEdit", manageDivisionEditData{
		baseData: newBaseWithTournament(r, t),
		Division: division,
	})
}

func (me *Env) ManageDivisionDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad Division ID", http.StatusBadRequest)
		return
	}
	if !me.DisableDelete {
		me.DB.DelDivision(did)
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
}
```

- [ ] **Step 2: Create `webhandler/templates/manage/divisions.html`**

```html
{{define "manageDivisions"}}
{{template "header" .}}
<h1>Divisions — {{.Tournament.Name}}</h1>
<h2>Add Division</h2>
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/divisions">
{{.CSRFField}}
<table>
  <tr><td>Name</td><td><input type="text" name="divisionname" required></td></tr>
  <tr><td></td><td><input type="submit" value="Add Division"></td></tr>
</table>
</form>
<h2>Existing Divisions</h2>
{{if .Divisions}}
<table border="1" cellpadding="4" cellspacing="0">
<tr><th>Name</th><th>Games</th><th>Edit</th>{{if not $.DisableDelete}}<th>Delete</th>{{end}}</tr>
{{range .Divisions}}
<tr>
  <td>{{.Name}}</td>
  <td><a href="/tournaments/{{$.Tournament.ID}}/manage/divisions/{{.ID}}/games/new">Manage Games</a></td>
  <td><a href="/tournaments/{{$.Tournament.ID}}/manage/divisions/{{.ID}}/edit">Edit</a></td>
  {{if not $.DisableDelete}}<td>
    <form method="post" action="/tournaments/{{$.Tournament.ID}}/manage/divisions/{{.ID}}/delete" onsubmit="return confirm('Delete this division and all its games?')">
      {{$.CSRFField}}
      <input type="submit" value="Delete">
    </form>
  </td>{{end}}
</tr>
{{end}}
</table>
{{else}}
<p>No divisions yet.</p>
{{end}}
<p><a href="/tournaments/{{.Tournament.ID}}/manage">Back to manage</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Create `webhandler/templates/manage/edit_division.html`**

```html
{{define "manageDivisionEdit"}}
{{template "header" .}}
<h1>Edit Division</h1>
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/divisions/{{.Division.ID}}/edit">
{{.CSRFField}}
<table>
  <tr><td>Name</td><td><input type="text" name="name" value="{{.Division.Name}}" required></td></tr>
  <tr><td></td><td><input type="submit" value="Save"></td></tr>
</table>
</form>
<p><a href="/tournaments/{{.Tournament.ID}}/manage/divisions">Back</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 4: Add routes in `main.go`**

```go
router.GET("/tournaments/:tid/manage/divisions", wh.ManageDivisions)
router.POST("/tournaments/:tid/manage/divisions", wh.ManageDivisions)
router.GET("/tournaments/:tid/manage/divisions/:did/edit", wh.ManageDivisionEdit)
router.POST("/tournaments/:tid/manage/divisions/:did/edit", wh.ManageDivisionEdit)
router.POST("/tournaments/:tid/manage/divisions/:did/delete", wh.ManageDivisionDelete)
```

- [ ] **Step 5: Build**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add webhandler/manage_divisions.go \
  webhandler/templates/manage/divisions.html \
  webhandler/templates/manage/edit_division.html main.go
git commit -m "feat: manage divisions routes for directors"
```

---

## Task 9: Manage teams

**Files:**
- Create: `webhandler/manage_teams.go`
- Create: `webhandler/templates/manage/teams.html`
- Create: `webhandler/templates/manage/edit_team.html`
- Modify: `main.go`

- [ ] **Step 1: Create `webhandler/manage_teams.go`**

```go
package webhandler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"gitlab.joe.beardedgeek.org/harnish/tourneyweb/mydb"
)

func (me *Env) ManageTeams(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
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
		me.DB.AddTeam(t.ID, divisionID, name, coach)
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/teams", t.ID), http.StatusSeeOther)
		return
	}
	divs := me.DB.ReturnDivisions(t.ID)
	byDiv := make(map[int][]mydb.Team)
	for _, div := range divs {
		byDiv[div.ID] = me.DB.ReturnTeamsByDivisionID(div.ID)
	}
	me.render(w, "manageTeams", manageTeamsData{
		baseData:        newBaseWithTournament(r, t),
		Divisions:       divs,
		TeamsByDivision: byDiv,
		DisableDelete:   me.DisableDelete,
	})
}

func (me *Env) ManageTeamEdit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		slog.Error("ManageTeamEdit bad ID", "err", err)
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
		div := me.DB.ReturnDivisionByID(divisionID)
		if div.ID == 0 || div.TournamentID != t.ID {
			http.Error(w, "Division does not belong to this tournament", http.StatusBadRequest)
			return
		}
		me.DB.UpdateTeam(teamID, divisionID, name, coach)
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/teams", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "manageTeamEdit", manageTeamEditData{
		baseData:  newBaseWithTournament(r, t),
		Team:      team,
		Divisions: me.DB.ReturnDivisions(t.ID),
	})
}

func (me *Env) ManageTeamDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	teamID, err := strconv.Atoi(ps.ByName("teamid"))
	if err != nil {
		http.Error(w, "Bad team ID", http.StatusBadRequest)
		return
	}
	if !me.DisableDelete {
		me.DB.DelTeam(teamID)
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/teams", t.ID), http.StatusSeeOther)
}
```

- [ ] **Step 2: Create `webhandler/templates/manage/teams.html`**

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
  <table border="1" cellpadding="4" cellspacing="0">
  <tr><th>Name</th><th>Coach</th><th>Edit</th>{{if not $.DisableDelete}}<th>Delete</th>{{end}}</tr>
  {{range $teams}}
  <tr>
    <td>{{.Name}}</td>
    <td>{{.Coach}}</td>
    <td><a href="/tournaments/{{$.Tournament.ID}}/manage/teams/{{.ID}}/edit">Edit</a></td>
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

- [ ] **Step 3: Create `webhandler/templates/manage/edit_team.html`**

```html
{{define "manageTeamEdit"}}
{{template "header" .}}
<h1>Edit Team</h1>
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/teams/{{.Team.ID}}/edit">
{{.CSRFField}}
<table>
  <tr><td>Division</td><td>
    <select name="division">
      {{range .Divisions}}<option value="{{.ID}}"{{if eq .ID $.Team.Division.ID}} selected{{end}}>{{.Name}}</option>{{end}}
    </select>
  </td></tr>
  <tr><td>Team Name</td><td><input type="text" name="teamname" value="{{.Team.Name}}" required></td></tr>
  <tr><td>Coach</td><td><input type="text" name="teamcoach" value="{{.Team.Coach}}"></td></tr>
  <tr><td></td><td><input type="submit" value="Save"></td></tr>
</table>
</form>
<p><a href="/tournaments/{{.Tournament.ID}}/manage/teams">Back</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 4: Add routes in `main.go`**

```go
router.GET("/tournaments/:tid/manage/teams", wh.ManageTeams)
router.POST("/tournaments/:tid/manage/teams", wh.ManageTeams)
router.GET("/tournaments/:tid/manage/teams/:teamid/edit", wh.ManageTeamEdit)
router.POST("/tournaments/:tid/manage/teams/:teamid/edit", wh.ManageTeamEdit)
router.POST("/tournaments/:tid/manage/teams/:teamid/delete", wh.ManageTeamDelete)
```

- [ ] **Step 5: Build**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add webhandler/manage_teams.go \
  webhandler/templates/manage/teams.html \
  webhandler/templates/manage/edit_team.html main.go
git commit -m "feat: manage teams routes for directors"
```

---

## Task 10: Manage locations (per-tournament)

**Files:**
- Create: `webhandler/manage_locations.go`
- Modify: `webhandler/webhandler.go` (add `locationsForTournament` helper)
- Create: `webhandler/templates/manage/locations.html`
- Create: `webhandler/templates/manage/edit_location.html`
- Modify: `main.go`

- [ ] **Step 1: Add `locationsForTournament` helper in `webhandler/webhandler.go`**

Add after the existing `locationsFor` function:
```go
func (me *Env) locationsForTournament(tid int) []mydb.Location {
	return me.DB.GetLocationsByTournamentID(tid)
}
```

- [ ] **Step 2: Create `webhandler/manage_locations.go`**

```go
package webhandler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) ManageLocations(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		address := r.FormValue("address")
		lat, _ := strconv.ParseFloat(r.FormValue("latitude"), 64)
		lng, _ := strconv.ParseFloat(r.FormValue("longitude"), 64)
		if name != "" {
			me.DB.AddLocation(name, address, "", lat, lng, t.ID)
		}
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/locations", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "manageLocations", manageLocationsData{
		baseData:      newBaseWithTournament(r, t),
		Locations:     me.DB.GetLocationsByTournamentID(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) ManageLocationEdit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	lid, _ := strconv.Atoi(ps.ByName("lid"))
	loc := me.DB.GetLocationByID(lid)
	if loc.ID == 0 || loc.TournamentID != t.ID {
		me.renderError(w, r, http.StatusNotFound, "Not Found", "Location not found.")
		return
	}
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		address := r.FormValue("address")
		lat, _ := strconv.ParseFloat(r.FormValue("latitude"), 64)
		lng, _ := strconv.ParseFloat(r.FormValue("longitude"), 64)
		me.DB.UpdateLocation(lid, name, address, "", lat, lng)
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/locations", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "manageLocationEdit", manageLocationEditData{
		baseData: newBaseWithTournament(r, t),
		Location: loc,
	})
}

func (me *Env) ManageLocationDelete(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	if me.DisableDelete {
		me.renderError(w, r, http.StatusForbidden, "Deletes Disabled", "Delete operations are disabled.")
		return
	}
	lid, _ := strconv.Atoi(ps.ByName("lid"))
	loc := me.DB.GetLocationByID(lid)
	if loc.TournamentID != t.ID {
		me.renderError(w, r, http.StatusForbidden, "Forbidden", "That location does not belong to this tournament.")
		return
	}
	me.DB.DelLocation(lid)
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/locations", t.ID), http.StatusSeeOther)
}
```

- [ ] **Step 3: Create `webhandler/templates/manage/locations.html`**

```html
{{define "manageLocations"}}
{{template "header" .}}
<h1>Locations — {{.Tournament.Name}}</h1>
<h2>Add Location/Field</h2>
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/locations">
{{.CSRFField}}
<table>
  <tr><td>Name</td><td><input type="text" name="name" required></td></tr>
  <tr><td>Address</td><td><input type="text" name="address"></td></tr>
  <tr><td>Latitude</td><td><input type="number" step="any" name="latitude" value="0"></td></tr>
  <tr><td>Longitude</td><td><input type="number" step="any" name="longitude" value="0"></td></tr>
  <tr><td></td><td><input type="submit" value="Add Location"></td></tr>
</table>
</form>
<h2>Existing Locations</h2>
{{if .Locations}}
<table border="1" cellpadding="4" cellspacing="0">
<tr><th>Name</th><th>Address</th><th>Edit</th>{{if not $.DisableDelete}}<th>Delete</th>{{end}}</tr>
{{range .Locations}}
<tr>
  <td>{{.Name}}</td>
  <td>{{.Address}}</td>
  <td><a href="/tournaments/{{$.Tournament.ID}}/manage/locations/{{.ID}}/edit">Edit</a></td>
  {{if not $.DisableDelete}}<td>
    <form method="post" action="/tournaments/{{$.Tournament.ID}}/manage/locations/{{.ID}}/delete" onsubmit="return confirm('Delete this location?')">
      {{$.CSRFField}}
      <input type="submit" value="Delete">
    </form>
  </td>{{end}}
</tr>
{{end}}
</table>
{{else}}
<p>No locations yet.</p>
{{end}}
<p><a href="/tournaments/{{.Tournament.ID}}/manage">Back to manage</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 4: Create `webhandler/templates/manage/edit_location.html`**

```html
{{define "manageLocationEdit"}}
{{template "header" .}}
<h1>Edit Location</h1>
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/locations/{{.Location.ID}}/edit">
{{.CSRFField}}
<table>
  <tr><td>Name</td><td><input type="text" name="name" value="{{.Location.Name}}" required></td></tr>
  <tr><td>Address</td><td><input type="text" name="address" value="{{.Location.Address}}"></td></tr>
  <tr><td>Latitude</td><td><input type="number" step="any" name="latitude" value="{{.Location.Latitude}}"></td></tr>
  <tr><td>Longitude</td><td><input type="number" step="any" name="longitude" value="{{.Location.Longitude}}"></td></tr>
  <tr><td></td><td><input type="submit" value="Save"></td></tr>
</table>
</form>
<p><a href="/tournaments/{{.Tournament.ID}}/manage/locations">Back</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 5: Add routes in `main.go`**

```go
router.GET("/tournaments/:tid/manage/locations", wh.ManageLocations)
router.POST("/tournaments/:tid/manage/locations", wh.ManageLocations)
router.GET("/tournaments/:tid/manage/locations/:lid/edit", wh.ManageLocationEdit)
router.POST("/tournaments/:tid/manage/locations/:lid/edit", wh.ManageLocationEdit)
router.POST("/tournaments/:tid/manage/locations/:lid/delete", wh.ManageLocationDelete)
```

- [ ] **Step 6: Build**

```bash
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add webhandler/manage_locations.go webhandler/webhandler.go \
  webhandler/templates/manage/locations.html \
  webhandler/templates/manage/edit_location.html main.go
git commit -m "feat: per-tournament location management for directors"
```

---

## Task 11: Manage games

**Files:**
- Create: `webhandler/manage_games.go`
- Create: `webhandler/templates/manage/create_game.html`
- Create: `webhandler/templates/manage/edit_game.html`
- Modify: `main.go`

- [ ] **Step 1: Create `webhandler/manage_games.go`**

```go
package webhandler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
)

func (me *Env) ManageCreateGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad division ID", http.StatusBadRequest)
		return
	}
	me.render(w, "manageCreateGame", manageCreateGameData{
		baseData:      newBaseWithTournament(r, t),
		DivisionID:    did,
		Teams:         me.DB.ReturnTeamsByDivisionID(did),
		Games:         me.DB.AllGamesByDivision(did),
		Locations:     me.locationsForTournament(t.ID),
		DisableDelete: me.DisableDelete,
	})
}

func (me *Env) ManageCreateGameSubmit(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(r.FormValue("divisionid"))
	if err != nil {
		http.Error(w, "Bad DivisionID", http.StatusBadRequest)
		return
	}
	hid, err := strconv.Atoi(r.FormValue("hometeam"))
	if err != nil {
		http.Error(w, "Bad Home team ID", http.StatusBadRequest)
		return
	}
	aid, err := strconv.Atoi(r.FormValue("awayteam"))
	if err != nil {
		http.Error(w, "Bad Away team ID", http.StatusBadRequest)
		return
	}
	if aid == hid {
		http.Error(w, "Must select a different team as an opponent.", http.StatusBadRequest)
		return
	}
	startTime, _ := time.ParseInLocation("2006-01-02T15:04", r.FormValue("datetime"), time.Local)
	me.DB.AddGame(t.ID, did, hid, aid, r.FormValue("location"), startTime, r.FormValue("umpire"))
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions/%d/games/new", t.ID, did), http.StatusSeeOther)
}

func (me *Env) ManageGenerateGames(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	did, err := strconv.Atoi(ps.ByName("did"))
	if err != nil {
		http.Error(w, "Bad division ID", http.StatusBadRequest)
		return
	}
	teams := me.DB.ReturnTeamsByDivisionID(did)
	if len(teams) < 2 {
		http.Error(w, "Need at least 2 teams to generate a schedule", http.StatusBadRequest)
		return
	}
	startTime, _ := time.ParseInLocation("2006-01-02T15:04", r.FormValue("start_datetime"), time.Local)
	minutesBetween, _ := strconv.Atoi(r.FormValue("minutes_between"))
	if minutesBetween <= 0 {
		minutesBetween = 120
	}
	location := r.FormValue("location")
	double := r.FormValue("round_type") == "double"
	interval := time.Duration(minutesBetween) * time.Minute
	current := startTime
	for i := 0; i < len(teams); i++ {
		for j := i + 1; j < len(teams); j++ {
			me.DB.AddGame(t.ID, did, teams[i].ID, teams[j].ID, location, current, "")
			current = current.Add(interval)
			if double {
				me.DB.AddGame(t.ID, did, teams[j].ID, teams[i].ID, location, current, "")
				current = current.Add(interval)
			}
		}
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions/%d/games/new", t.ID, did), http.StatusSeeOther)
}

func (me *Env) ManageEditGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
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
		div := me.DB.ReturnDivisionByID(did)
		if div.ID == 0 || div.TournamentID != t.ID {
			http.Error(w, "Division does not belong to this tournament", http.StatusBadRequest)
			return
		}
		startTime, _ := time.ParseInLocation("2006-01-02T15:04", r.FormValue("datetime"), time.Local)
		me.DB.UpdateGame(gid, did, hid, aid, r.FormValue("location"), startTime, r.FormValue("umpire"))
		http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
		return
	}
	me.render(w, "manageEditGame", manageEditGameData{
		baseData:  newBaseWithTournament(r, t),
		Game:      game,
		Teams:     me.DB.ReturnTeamsByTournamentID(t.ID),
		Divisions: me.DB.ReturnDivisions(t.ID),
		Locations: me.locationsForTournament(t.ID),
	})
}

func (me *Env) ManageDeleteGame(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, ok := me.tournamentFromRoute(w, r, ps)
	if !ok {
		return
	}
	gid, err := strconv.Atoi(ps.ByName("gid"))
	if err != nil {
		http.Error(w, "Bad Game ID", http.StatusBadRequest)
		return
	}
	if !me.DisableDelete {
		me.DB.DelGame(gid)
	}
	http.Redirect(w, r, fmt.Sprintf("/tournaments/%d/manage/divisions", t.ID), http.StatusSeeOther)
}
```

- [ ] **Step 2: Create `webhandler/templates/manage/create_game.html`**

```html
{{define "manageCreateGame"}}
{{template "header" .}}
<h2>Add Game</h2>
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/games">
<input type="hidden" name="divisionid" value="{{.DivisionID}}">
{{.CSRFField}}
<table>
  <tr><td>Home Team</td><td><select name="hometeam">
    {{range .Teams}}<option value="{{.ID}}">{{.Name}} - {{.Coach}}</option>{{end}}
  </select></td></tr>
  <tr><td>Away Team</td><td><select name="awayteam">
    {{range .Teams}}<option value="{{.ID}}">{{.Name}} - {{.Coach}}</option>{{end}}
  </select></td></tr>
  <tr><td>Location</td><td>
    {{if .Locations}}
    <select name="location">
      <option value="">-- select --</option>
      {{range .Locations}}<option value="{{.Name}}">{{.Name}}{{if .Address}} — {{.Address}}{{end}}</option>{{end}}
    </select>
    {{else}}
    <input type="text" name="location">
    {{end}}
  </td></tr>
  <tr><td>Date/Time</td><td><input type="datetime-local" name="datetime"></td></tr>
  <tr><td>Umpire</td><td><input type="text" name="umpire"></td></tr>
  <tr><td></td><td><input type="submit" value="Add Game"></td></tr>
</table>
</form>

<hr>
<h2>Auto-Generate Schedule</h2>
<p>Creates a round-robin schedule for all {{len .Teams}} teams in this division.</p>
{{if lt (len .Teams) 2}}
<p><em>Add at least 2 teams to generate a schedule.</em></p>
{{else}}
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/divisions/{{.DivisionID}}/games/generate">
{{.CSRFField}}
<table>
  <tr><td>Start Date/Time</td><td><input type="datetime-local" name="start_datetime" required></td></tr>
  <tr><td>Minutes Between Games</td><td><input type="number" name="minutes_between" value="120" min="1" style="width:80px"></td></tr>
  <tr><td>Location</td><td>
    {{if .Locations}}
    <select name="location">
      <option value="">-- none --</option>
      {{range .Locations}}<option value="{{.Name}}">{{.Name}}{{if .Address}} — {{.Address}}{{end}}</option>{{end}}
    </select>
    {{else}}
    <input type="text" name="location" placeholder="leave blank or enter manually">
    {{end}}
  </td></tr>
  <tr><td>Round Robin Type</td><td>
    <select name="round_type">
      <option value="single">Single (each pair plays once)</option>
      <option value="double">Double (each pair plays home &amp; away)</option>
    </select>
  </td></tr>
  <tr><td></td><td><input type="submit" value="Generate Schedule" onclick="return confirm('Generate {{len .Teams}} team round-robin? Existing games will not be removed.')"></td></tr>
</table>
</form>
{{end}}

<hr>
<h2>Existing Games</h2>
{{if .Games}}
<table border="1" cellpadding="4" cellspacing="0">
<tr><th>Home</th><th>Away</th><th>Location</th><th>Start</th><th>Umpire</th><th>Score</th><th>Edit</th>{{if not $.DisableDelete}}<th>Delete</th>{{end}}</tr>
{{range .Games}}
<tr>
  <td>{{.HomeTeam.Name}}</td>
  <td>{{.AwayTeam.Name}}</td>
  <td>{{.Location}}</td>
  <td>{{formatTime .Start}}</td>
  <td>{{.Umpire}}</td>
  <td>{{if .Scored}}{{.HomeScore}}–{{.AwayScore}}{{end}}</td>
  <td><a href="/tournaments/{{$.Tournament.ID}}/manage/games/{{.ID}}/edit">Edit</a></td>
  {{if not $.DisableDelete}}<td>
    <form method="post" action="/tournaments/{{$.Tournament.ID}}/manage/games/{{.ID}}/delete" onsubmit="return confirm('Delete this game?')">
      {{$.CSRFField}}
      <input type="submit" value="Delete">
    </form>
  </td>{{end}}
</tr>
{{end}}
</table>
{{else}}
<p>No games yet.</p>
{{end}}
<p><a href="/tournaments/{{.Tournament.ID}}/manage/divisions">Back to divisions</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 3: Create `webhandler/templates/manage/edit_game.html`**

```html
{{define "manageEditGame"}}
{{template "header" .}}
<h1>Edit Game</h1>
<form method="post" action="/tournaments/{{.Tournament.ID}}/manage/games/{{.Game.ID}}/edit">
{{.CSRFField}}
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
  <tr><td>Location</td><td>
    {{if .Locations}}
    <select name="location">
      <option value="">-- select --</option>
      {{range .Locations}}<option value="{{.Name}}"{{if eq .Name $.Game.Location}} selected{{end}}>{{.Name}}{{if .Address}} — {{.Address}}{{end}}</option>{{end}}
    </select>
    {{else}}
    <input type="text" name="location" value="{{.Game.Location}}">
    {{end}}
  </td></tr>
  <tr><td>Date/Time</td><td><input type="datetime-local" name="datetime" value="{{formatDateTimeLocal .Game.Start}}"></td></tr>
  <tr><td>Umpire</td><td><input type="text" name="umpire" value="{{.Game.Umpire}}"></td></tr>
  <tr><td></td><td><input type="submit" value="Save"></td></tr>
</table>
</form>
<p><a href="/tournaments/{{.Tournament.ID}}/manage/divisions">Cancel</a></p>
{{template "footer" .}}
{{end}}
```

- [ ] **Step 4: Add routes in `main.go`**

```go
router.GET("/tournaments/:tid/manage/divisions/:did/games/new", wh.ManageCreateGame)
router.POST("/tournaments/:tid/manage/games", wh.ManageCreateGameSubmit)
router.POST("/tournaments/:tid/manage/divisions/:did/games/generate", wh.ManageGenerateGames)
router.GET("/tournaments/:tid/manage/games/:gid/edit", wh.ManageEditGame)
router.POST("/tournaments/:tid/manage/games/:gid/edit", wh.ManageEditGame)
router.POST("/tournaments/:tid/manage/games/:gid/delete", wh.ManageDeleteGame)
```

- [ ] **Step 5: Build and run tests**

```bash
go test ./...
go build ./...
```

Expected: all tests pass, build succeeds.

- [ ] **Step 6: Commit**

```bash
git add webhandler/manage_games.go \
  webhandler/templates/manage/create_game.html \
  webhandler/templates/manage/edit_game.html main.go
git commit -m "feat: manage games and schedule generation for directors"
```

---

## Task 12: Final wiring — run tests, verify build

- [ ] **Step 1: Run full test suite**

```bash
go test ./... -v 2>&1 | tail -30
```

Expected: all tests pass.

- [ ] **Step 2: Verify binary builds cleanly**

```bash
go build ./...
```

- [ ] **Step 3: Verify all new routes are registered by checking main.go**

```bash
grep -c "manage" main.go
```

Expected: at least 20 lines containing "manage".

- [ ] **Step 4: Verify template names match render calls**

```bash
grep -h 'me\.render' webhandler/manage*.go webhandler/verification.go | \
  grep -oP '"[a-zA-Z]+"' | sort | uniq
```

Cross-check each name against the `{{define "..."}}` in the manage templates:
```bash
grep '{{define' webhandler/templates/manage/*.html | grep -oP '"[^"]+"'
```

Both lists must match.

- [ ] **Step 5: Final commit if any fixes needed**

```bash
git add -p
git commit -m "fix: final wiring corrections"
```
