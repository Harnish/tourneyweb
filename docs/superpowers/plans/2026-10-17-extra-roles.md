# Extra Game Roles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Directors define free-form roles per tournament, set per-level counts, fill per-game slots (registered user via search, or free-form name); assignees are notified, accept/decline on one combined "My assignments" page; overlaps are blocked across umpire and role slots. Also closes the stale-score-report security finding on #18.

**Architecture:** Three new tables (`extra_roles`, `division_role_needs`, `game_role_slots`) mirroring the umpire slot model, added to `mydb.DB`, `*MyDB`, `*FakeDB`. The existing `UmpireConflict` becomes the single overlap check covering both slot kinds. Handlers mirror `umpire_slots.go` / `umpire_view.go`; notifications use `notifyUser`.

**Tech Stack:** Go, httprouter, html/template, pgx via database/sql, FakeDB + opt-in Postgres test (`TEST_DATABASE_URL`).

**Spec:** `docs/superpowers/specs/2026-10-17-extra-roles-design.md`

## Global Constraints

- Every new DB method goes in `mydb/db.go` interface, `*MyDB` and `*FakeDB`. Schema is idempotent (`pgtables`/`pgmigrations` in `mydb/mydb.go`).
- Role name: trimmed, 1–40 chars, unique per tournament case-insensitively. Count per role per level: 0–5.
- Overlap window is `UmpireWindowsOverlap` (2h half-open). Declined slots never conflict.
- Registered users may only be assigned on non-draft tournaments (same rule as umpires); free-form names always allowed.
- All new manage routes sit under `/tournaments/:tid/manage/` (director/staff middleware); cross-tournament IDs → 404. Assignee response route is login-checked in the handler, own slot only, CSRF via global middleware + `{{.CSRFField}}`.
- Notification links: assigned/changed/removed/cancelled → `/umpire`; declined → `/tournaments/:tid/manage/games/:gid/edit` via `directorRecipients`.
- Conventional commits, never push without the user's confirmation. Mandatory TDD (test first, watch it fail).

## Review Focus

- Deleting a role / lowering a count / deleting a game or division with assigned users: slots vanish and assignees are told, no orphan rows, no panic.
- A user assigned as umpire on game A cannot be given a role slot on an overlapping game B (and vice versa); same-game double slot is refused.
- Role names differing only by case/whitespace are refused; empty or 41-char names refused.
- The user search endpoint refuses non-staff and other tournaments' staff, limits to 10 results, and escapes names in JSON/HTML.
- A role slot assigned to user X cannot be accepted/declined by user Y (403).
- Draft tournament: a registered user in a role slot is refused.

---

### Task 1: Clear stale score reports when a game's teams change

(Security finding on #18: `UpdateGame` leaves old reports/status attached.)

**Files:**
- Modify: `mydb/score_reports.go`, `mydb/fakedb_score_reports.go`, `mydb/db.go`
- Modify: `webhandler/manage_games.go` (`ManageEditGame`)
- Test: `mydb/score_reports_test.go`, `webhandler/score_forms_test.go`

**Interfaces:**
- Produces: `ClearScoreReports(gameID int)` on `mydb.DB`: deletes the game's reports and, only when the game's `score_status` is `reported`, `confirmed` or `disputed`, resets it to `''`... EXCEPT keep `final` games untouched. It does not touch scores (a reset game keeps the standings value until restaffed; see ruling below).

Ruling: for a `reported/confirmed/disputed` game, also un-score it is out of scope. Reset status to `''` and reports only; the game's old score stays until a director fixes it (staff are told nothing). Record this in the ledger as a ruling.

- [ ] **Step 1: failing tests.** `TestClearScoreReports` (mydb, FakeDB): game with two reports and status `disputed` → after `ClearScoreReports` `ScoreReportsForGame` is empty and `ScoreStatus==""`; a `final` game keeps status `final`. `TestEditGameTeamsClearsReports` (webhandler): fixture from `score_forms_test.go` with a coach report on the game; POST `ManageEditGame` changing `awayteam` to another team in the tournament → reports empty. A POST that changes only the location leaves reports intact.
- [ ] **Step 2: run, expect FAIL** (`ClearScoreReports` undefined): `go test ./mydb ./webhandler -run 'ClearScoreReports|EditGameTeams'`.
- [ ] **Step 3: implement** `ClearScoreReports` in both DBs (Postgres: delete from `game_score_reports`, then `UPDATE games SET score_status='' WHERE id=$1 AND score_status IN ('reported','confirmed','disputed')`). In `ManageEditGame` call it after `UpdateGame` when `hid != game.HomeTeam.ID || aid != game.AwayTeam.ID`.
- [ ] **Step 4: run** `go test ./...` → PASS.
- [ ] **Step 5: commit** `fix: clear score reports when a game's teams change`.

---

### Task 2: Roles, per-level counts, user search (DB layer)

**Files:**
- Create: `mydb/extra_roles.go`, `mydb/fakedb_extra_roles.go`, `mydb/extra_roles_test.go`
- Modify: `mydb/db.go`, `mydb/mydb.go` (tables), `mydb/fakedb.go` (fields, `DelDivision`/`DelTournament` cleanup), `mydb/users.go` + FakeDB users (search)

**Interfaces — Produces:**
```go
type ExtraRole struct{ ID, TournamentID int; Name string }
var ErrRoleNameInvalid, ErrRoleNameTaken error
AddExtraRole(tournamentID int, name string) (int, error)
RenameExtraRole(tournamentID, roleID int, name string) error   // ErrRoleNameTaken/Invalid
DelExtraRole(tournamentID, roleID int)                          // also deletes needs and slots
TournamentExtraRoles(tournamentID int) []ExtraRole              // ordered by name
ExtraRoleByID(roleID int) ExtraRole                             // ID 0 if missing
DivisionRoleNeeds(divisionID int) map[int]int                   // roleID → n (only n>0)
SetDivisionRoleNeeded(divisionID, roleID, n int)                // clamps 0..5; 0 deletes the row
SearchUsers(q string, limit int) []User                         // case-insensitive substring of name or email, q trimmed, len(q)>=2 else nil, ordered by name
```
Schema: `extra_roles(id SERIAL PK, tournament_id → tournaments ON DELETE CASCADE, name TEXT)` + unique index on `(tournament_id, lower(name))`; `division_role_needs(division_id → divisions ON DELETE CASCADE, role_id → extra_roles ON DELETE CASCADE, needed INTEGER, PK(division_id, role_id))`.

- [ ] **Step 1: failing tests** (FakeDB): `TestExtraRoleNames` ("Scorekeeper" ok; " scorekeeper " → ErrRoleNameTaken; "" and 41 chars → ErrRoleNameInvalid; same name in another tournament ok; rename to own name with different case ok), `TestDivisionRoleNeedsClamp` (7→5, 0 removes, `DelExtraRole` removes needs), `TestSearchUsers` (matches name/email substring, "a" returns nil, limit respected).
- [ ] **Step 2: run, expect FAIL** (undefined).
- [ ] **Step 3: implement** in both DBs. FakeDB: `DelDivision` drops that division's needs; `DelTournament` drops its roles/needs. Postgres LIKE with escaped `%`/`_` (`strings.NewReplacer`).
- [ ] **Step 4: add `TestExtraRolesPostgres`** to `mydb/umpires_pg_test.go` style file (opt-in): name uniqueness via the index, needs cascade on role delete.
- [ ] **Step 5: run** `go test ./mydb` (and with `TEST_DATABASE_URL=...55432` on `twpg` if up) → PASS; `go vet ./...`.
- [ ] **Step 6: commit** `feat: tournament extra roles and per-level counts`.

---

### Task 3: Role slots and the shared overlap check (DB layer)

**Files:**
- Create: `mydb/role_slots.go`, `mydb/fakedb_role_slots.go`, `mydb/role_slots_test.go`
- Modify: `mydb/db.go`, `mydb/mydb.go`, `mydb/umpires.go` (`umpireConflict`, `AssignUmpire` duplicate check), `mydb/fakedb_umpires.go` (`conflictLocked`, `AssignUmpire`), `mydb/fakedb.go` (`DelGame` removes the game's role slots; field `roleSlots []RoleSlot`)

**Interfaces — Consumes:** Task 2 `ExtraRole`.
**Produces:**
```go
type RoleSlot struct {
    ID, GameID, RoleID, Slot, UserID int
    RoleName, FreeName, Status, UserName, UserEmail string
}
GameRoleSlots(gameID int) []RoleSlot                 // ordered by role name, slot; includes RoleName
RoleSlotsForUser(userID int) []RoleSlot              // includes declined
AssignRoleSlot(gameID, roleID, slot, userID int, freeName string) error
ClearRoleSlot(gameID, roleID, slot int)
RespondRoleSlot(gameID, roleID, slot, userID int, accept bool) error   // ErrUmpireSlotDeclined if already declined
ResetRoleSlotsPending(gameID int) []RoleSlot         // accepted registered → pending, returns the affected
```
Table `game_role_slots(id SERIAL PK, game_id → games ON DELETE CASCADE, role_id → extra_roles ON DELETE CASCADE, slot INT, user_id → users NULL, free_name TEXT, status CHECK pending/accepted/declined, UNIQUE(game_id, role_id, slot), CHECK((user_id IS NOT NULL) <> (free_name <> '')))`.

Semantics identical to `AssignUmpire`/`RespondUmpireSlot`: registered → status `pending`; free-form → `accepted`. Registered assignment returns `ErrUmpireDuplicate` if the user holds any other non-declined slot (umpire or role, other than this exact slot) on the same game, `ErrUmpireConflict` if `UmpireConflict` (now covering both tables, excluding the same game) is true. `AssignUmpire` gets the same cross-table duplicate check. Role slots do not touch `games.umpire`.

- [ ] **Step 1: failing tests** (FakeDB): `TestRoleSlotLifecycle` (assign registered → pending; respond accept → accepted; decline → declined and second respond → `ErrUmpireSlotDeclined`; other user responding errors), `TestOverlapAcrossUmpireAndRole` (user umpire on game A at 10:00 → `AssignRoleSlot` on game B at 11:00 returns `ErrUmpireConflict`; back-to-back at 12:00 ok; reverse direction too; declined slot doesn't block), `TestSameGameDoubleSlotRefused` (umpire slot + role slot, same game, same user → `ErrUmpireDuplicate` both directions), `TestResetRoleSlotsPending`, `TestDelGameRemovesRoleSlots`.
- [ ] **Step 2: run, expect FAIL.**
- [ ] **Step 3: implement** both DBs; Postgres assign uses a transaction with `SELECT 1 FROM users WHERE id=$1 FOR UPDATE` like `AssignUmpire`. Extend `umpireConflict` SQL with `OR EXISTS` over `game_role_slots` joined to `games`.
- [ ] **Step 4: add `TestRoleSlotsPostgres`** (opt-in): assign, conflict across tables, role delete cascades slots.
- [ ] **Step 5: run** `go test ./... && go vet ./...` → PASS (existing umpire tests must still pass).
- [ ] **Step 6: commit** `feat: role slots with cross-table overlap blocking`.

---

### Task 4: Extra Roles manage page and division counts

**Files:**
- Create: `webhandler/manage_roles.go`, `webhandler/templates/manage/roles.html`, `webhandler/manage_roles_test.go`
- Modify: `main.go` (routes), `webhandler/manage_divisions.go` (create + edit), `webhandler/templates/manage/edit_division.html` (+ the create-division form template), `webhandler/templates/manage/dashboard.html` (link), template registration wherever `manage/umpires.html` is registered.

**Interfaces — Consumes:** Task 2 DB methods.
**Produces:** routes `GET /tournaments/:tid/manage/roles`, `POST .../roles` (create, field `name`), `POST .../roles/:rid/rename` (`name`), `POST .../roles/:rid/delete`. Handlers `ManageRoles`, `ManageRoleCreate`, `ManageRoleRename`, `ManageRoleDelete`; helper `func (me *Env) roleTarget(w, r, ps) (mydb.Tournament, mydb.ExtraRole, bool)` that 404s when the role belongs to another tournament. Delete: collects `GameRoleSlots` of affected games first (Task 3) and `notifyUser(uid, "Role removed", "...", "/umpire")` for each non-declined registered assignee before `DelExtraRole`; respects `DisableDelete`. Division forms read inputs `role_need_<roleID>` (ignored if absent) and call `SetDivisionRoleNeeded`; the edit form pre-fills from `DivisionRoleNeeds`, only roles of this tournament.

- [ ] **Step 1: failing tests** (httptest, pattern from `manage_umpires_test.go`): `TestCreateRole` (POST creates; duplicate case-insensitive → 400/409 with message; page lists it), `TestRoleOtherTournament404` (rename/delete with a role of tournament 2 under tournament 1's URL → 404), `TestDeleteRoleNotifiesAssignees` (accepted user gets a notification row; slots gone), `TestDivisionEditSetsRoleNeeds` (POST `role_need_<id>=3` stored; role of another tournament ignored; 9 clamps to 5).
- [ ] **Step 2: run, expect FAIL.**
- [ ] **Step 3: implement** handlers, templates, routes, dashboard link "Extra Roles".
- [ ] **Step 4: run** `go test ./... && go vet ./...` → PASS.
- [ ] **Step 5: commit** `feat: Extra Roles management page and per-level counts`.

---

### Task 5: Role slots on game forms (assign, search, notify)

**Files:**
- Create: `webhandler/role_slots.go`, `webhandler/user_search.go`, `webhandler/templates/role_slots.html`, `webhandler/role_slots_test.go`
- Modify: `webhandler/manage_games.go` (create/edit/delete + data structs), the game form templates (`manage/create_game.html`, `manage/edit_game.html`; the legacy `admin/*` ones too if they still render `umpireSlots`), `webhandler/umpire_slots.go` (`notifyGameChanged`, `notifyGameDeleted` also cover role slots), `main.go`.

**Interfaces — Consumes:** Tasks 2–3. **Produces:**
- `type roleSlotBlock struct{ Roles []roleSlotGroup }`, `roleSlotGroup{ RoleID int; Name string; Slots []slotView }`; `func (me *Env) roleSlotBlock(t mydb.Tournament, divisionID int, game *mydb.Game) roleSlotBlock` (slot count per role = `DivisionRoleNeeds`, raised to the highest existing slot like `slotCount`).
- Form fields `role_user_<roleID>_<slot>` (hidden registered id, wins) and `role_name_<roleID>_<slot>`; `parseRoleChoices(r, divisionID) []roleChoice{RoleID, Slot, UserID int; Name string}`.
- `func (me *Env) validateRoleChoices(t, choices, start, gameID) string` (draft → registered users refused; user must exist; same user twice in the submission refused; `UmpireConflict` message like `validateSlots`) and `func (me *Env) applyRoleChoices(g mydb.Game, choices []roleChoice) string` modelled on `applySlots`: clear changed slots (notify "Removed from a game"), assign new (notify "You have been scheduled for a role", message names the role, link `/umpire`).
- `ManageEditGame` also resets role slots on time/location change (`ResetRoleSlotsPending` → "Game changed — please re-confirm"); `ManageDeleteGame` notifies role assignees "Game cancelled".
- Route `GET /tournaments/:tid/manage/users/search?q=` → `UserSearch`, JSON `[{"id":1,"name":"Ann"}]`, ≤10; director/staff via the manage middleware; any registered user (name only, no email in the response, though email is searched).
- Template: per slot a text input + hidden id; a small inline script fetches the search endpoint into a `<datalist>`; choosing an option sets the hidden id, typing anything else clears it (free-form). Works without JS as free-form only.

- [ ] **Step 1: failing tests**: `TestAssignRoleOnGameForm` (POST edit-game with `role_user_<rid>_1=<uid>` → pending slot, notification "scheduled", email recorded via `sendNotice` seam, pattern from `newSlotFx`; restore with `notifyWG.Wait()`); `TestRoleFreeFormNoNotice`; `TestRoleConflictRejected` (409, no slot written, message names the user); `TestDraftRefusesRegisteredRole`; `TestGameChangeResetsRoleSlots`; `TestDeleteGameNotifiesRoleAssignee`; `TestUserSearch` (JSON, limit 10, no email key, `<script>` in a name stays JSON-escaped, 2-char minimum, a non-staff request is rejected by the router middleware — assert via the middleware test helper used in existing manage tests, or hit the handler with a non-staff context and expect 403).
- [ ] **Step 2: run, expect FAIL.**
- [ ] **Step 3: implement** per Interfaces.
- [ ] **Step 4: run** `go test -race ./... && go vet ./...` → PASS.
- [ ] **Step 5: commit** `feat: assign extra roles on games with user search`.

---

### Task 6: "My assignments" dashboard, accept/decline, docs

**Files:**
- Modify: `webhandler/umpire_view.go` (`UmpireGames`, `umpireRow` gets `RoleID int`, `RoleName string`; `UmpireRespond` sibling), `webhandler/templates/umpire.html`, nav template/`baseData` (link visibility), `main.go`, README, `webhandler/templates/help.html`, `TODO.md`
- Create: `webhandler/role_view.go` (`RoleRespond`), `webhandler/role_view_test.go`

**Interfaces — Consumes:** Task 3. **Produces:** route `POST /umpire/games/:gid/roles/:rid/slots/:slot` → `RoleRespond` (login check; `RespondRoleSlot` with `user.ID`; `ErrUmpireSlotDeclined` → 409; other errors → 403; decline notifies `directorRecipients` "Role declined" linking to the game edit page; redirect `/umpire`). `UmpireGames` merges umpire rows (RoleName "Umpire") and role rows (declined hidden) into the same upcoming/past day groups; each row shows the role, and accept/decline posts to the right route. Page heading "My assignments"; the upcoming-tournaments / certification parts remain umpire-only. The page is viewable by anyone logged in; the nav link shows for flagged umpires or anyone with a non-declined role slot.

- [ ] **Step 1: failing tests**: `TestDashboardMixesUmpireAndRoles` (user with one umpire game and one role game sees both, ordered by time, role names shown, declined hidden, a non-umpire with only a role slot gets 200 and no "Upcoming tournaments" section); `TestRoleRespondAccept`; `TestRoleRespondOtherUser403`; `TestRoleDeclineNotifiesDirectors` (link ends `/manage/games/<gid>/edit`); `TestRoleRespondAlreadyDeclined409`.
- [ ] **Step 2: run, expect FAIL.**
- [ ] **Step 3: implement** handlers, template, nav, route.
- [ ] **Step 4: docs**: README feature list + help page section "Extra roles"; TODO.md: add "Follow tournaments/levels/teams; rank followers first in role search" and "Pay report for extra roles (not built)"; mark #20 items done.
- [ ] **Step 5: run** `go test -race -count=3 ./... && go vet ./...` and the Postgres tests → PASS.
- [ ] **Step 6: commit** `feat: combined My assignments page with role accept/decline`.
