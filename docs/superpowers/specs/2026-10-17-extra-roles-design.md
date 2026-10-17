# Extra game roles — design

Implements GitHub issue #20 ("Allow tournament management to add additional roles for each game").

## Intent

A director may want official scorekeepers, scoreboard runners, safety staff and so on, per
level of play. Today only umpires can be staffed on a game. Management defines the roles for
a tournament, says how many of each a level needs, and fills the slots on each game with a
registered user or a typed name. Registered people see their duties on one dashboard and
accept or decline, exactly like umpires.

## Goals

- **Extra Roles section** per tournament (director/staff): create, rename and delete free-form role names.
- **Per-level counts:** the division create/edit form shows a number box for every tournament role (0 = level doesn't use it).
- **Per-game assignment:** each game shows slots for its level's roles. A slot is filled by searching registered users or typing a free-form name; it can be cleared.
- **Notify + accept/decline:** a registered assignee is notified (in-app + email, `notifyUser`) and can accept or decline. A decline reopens the slot and notifies management. Game changed / cancelled / assignee removed notify like umpire notices.
- **Overlap blocking:** one person cannot hold overlapping assignments across umpire slots and extra-role slots together (same fixed 2-hour window, `UmpireWindowsOverlap`).
- **One dashboard:** the existing `/umpire` page becomes "My assignments": umpire games and extra-role games together, sorted by time, each with role, where/when, and accept/decline. The umpire-only sections (upcoming tournaments, certification) stay for umpires.

## Non-goals

- No opt-in: any registered user can be assigned (confirmed). Accept/decline is the protection.
- No certification, expiry, verification or volunteering for extra roles.
- No pay report for extra roles (pay report stays umpire-only).
- Free-form names get no notifications or dashboard.
- Follow-based ordering of the search results — depends on the separate "follow a tournament/level/team" ticket, not built here.
- Auto-assignment, role templates across tournaments.

## Anti-goals

- A user must never see or alter another user's assignments; only the assignee responds to their own slot.
- Directors/staff of one tournament must never manage roles, counts or slots of another tournament (cross-tournament IDs in URLs are rejected, as in the other manage routes).
- The user search must not expose data beyond what management needs (name, id), must require director/staff of the tournament, and must be size-limited.
- Deleting a role must not leave orphan slots or silently notify nobody: assignees of the removed slots are told (same as an umpire removed).

## Decisions (from the conversation)

- Roles are free-form names, created in an "Extra Roles" section of the tournament.
- Counts are set per level when creating/editing a division, one number per role.
- Slots are filled per game from a search of registered users or a typed name.
- Notify and accept/decline, same as umpires.
- Overlaps blocked, same rule, across umpire and extra roles.
- Any registered user can be assigned (no opt-in).
- One combined dashboard.

## Builder's calls (mine, not confirmed — please check)

- **Tables** (separate from umpires, which keep their own): `tournament_roles(id, tournament_id → tournaments ON DELETE CASCADE, name TEXT)` unique `(tournament_id, lower(name))`; `division_role_needs(division_id, role_id, needed INTEGER)` PK `(division_id, role_id)`; `game_role_slots(game_id, role_id, slot, user_id NULL, free_name, status, absent?)` PK `(game_id, role_id, slot)` mirroring `game_umpires` (`status`: `pending`, `accepted`, `declined`). I leave out `absent`; no pay report needs it.
- **Count cap:** 0–5 per role per level (umpires use a clamp too).
- **Role names:** trimmed, 1–40 chars, unique per tournament case-insensitively. Rename keeps slots. Delete removes its slots, needs and notifies assignees.
- **Search:** `GET /tournaments/:tid/manage/users/search?q=` returns up to 10 matches by name or email substring as JSON `{id, name}`; the slot form uses a plain text input with a datalist-style JS dropdown (progressive: if JS is off, a typed name is stored as free-form). Result order: alphabetical (follow-based ordering is the later ticket).
- **Conflict check** extends the existing umpire conflict check to also query `game_role_slots` for the user (declined slots don't count); a new combined helper keeps one source of truth.
- **Reopen on edit:** when a game's start time or location changes, role slots reset to `pending` and assignees are notified, as umpire slots do today (`ResetUmpireSlotsPending` analogue).
- **Notification links:** assigned/changed/removed/cancelled → `/umpire` (the dashboard); declined (to directors via `directorRecipients`) → the game edit page.
- **Division counts changed down:** slots beyond the new count are cleared and their assignees told, same as umpire count reductions today.
- **Routes** all under `/tournaments/:tid/manage/` (director/staff middleware): `roles` (GET, POST create, rename, delete), division forms get the number inputs, game edit form gets the slot block. Assignee response: `POST /umpire/role-slots/respond` (login required, own slot only, CSRF).
- **Dashboard** lives at `/umpire`, which needs no umpire flag for the assignments section; anyone with an assignment sees it. Nav shows the link for users who have any assignment or are flagged umpire.

## Data flow

Roles → division needs → slots appear on each game of that division → management fills them → assignee notified → assignee accepts/declines on `/umpire` → decline reopens slot and notifies management.

## Surfaces

- Manage: "Extra Roles" page; division form number inputs; game edit slot block.
- `/umpire` renamed "My assignments" in the nav and heading; mixed list.
- Notifications inbox (existing).

## Left to the builder

Template layout, wording, JS for the search dropdown, test structure (FakeDB tests plus extend the opt-in Postgres test), file names.
