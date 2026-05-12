# Self-Service Tournament Creation & Verification Codes — Design Spec

**Subsystem:** A of 2 (verification codes + self-service creation + director management UI)
**Deferred to subsystem B:** team metadata (hometown, season record, GameChanger link), coach roster management

---

## Goal

Allow any logged-in user to create and self-manage a tournament. Tournaments start as drafts (invisible publicly). An admin issues a per-tournament verification code; the director enters it to publish. Admins skip the code step and publish immediately.

---

## Data Model

### `tournaments` table — add column

```sql
ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'published';
```

Existing tournaments default to `'published'` so they remain visible after deploy. New admin-created tournaments are explicitly inserted with `status='published'`. New self-service tournaments are explicitly inserted with `status='draft'`.

### New `verification_codes` table

```sql
CREATE TABLE IF NOT EXISTS verification_codes (
    id            SERIAL PRIMARY KEY,
    tournament_id INTEGER NOT NULL REFERENCES tournaments(id),
    code          TEXT NOT NULL UNIQUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    redeemed_at   TIMESTAMPTZ
);
```

One row per issuance. `redeemed_at IS NULL` means unused. Each code is single-use — once redeemed, it cannot be reused.

### `locations` table — add column

```sql
ALTER TABLE locations ADD COLUMN IF NOT EXISTS tournament_id INTEGER REFERENCES tournaments(id);
```

- `tournament_id IS NULL` — existing global/admin-managed locations (backward compatible)
- `tournament_id = N` — director-owned, scoped to tournament N

When scheduling games for tournament N, only locations with `tournament_id = N` are shown. Global locations (NULL) are no longer shown in the game scheduler — directors manage their own fields.

---

## Flows

### Self-Service Tournament Creation

1. Any logged-in user visits `GET /tournaments/new`.
2. Form fields: name, sport, location (text, city/venue), start date, notes.
3. On `POST /tournaments/new`:
   - If user is admin: insert with `status='published'`.
   - Otherwise: insert with `status='draft'`.
   - Insert a `tournament_roles` row: `(user_id, tournament_id, role='director')`.
4. Redirect to `GET /tournaments/:tid/manage`.

### Admin Queue

- `GET /admin/tournaments/queue` lists all tournaments with `status='draft'`, showing name, creator email, and created date.
- Each row has an "Issue Code" button (`POST /admin/tournaments/:tid/issue-code`).
- On submit: generate a random 8-character alphanumeric code (uppercase), insert into `verification_codes`, display the code on-screen for the admin to copy and share out-of-band. No email sent — delivery is manual.
- A tournament can have multiple codes issued (if admin re-issues), but only the first redemption publishes it.

### Director Publishes

- `GET /tournaments/:tid/manage/publish` shows a form to enter the verification code (only visible if tournament is still `'draft'`).
- On `POST /tournaments/:tid/manage/publish`:
  - Validate: code exists, `tournament_id` matches, `redeemed_at IS NULL`.
  - If valid: set `redeemed_at = NOW()`, set tournament `status='published'`. Redirect to manage dashboard with success message.
  - If invalid: re-render form with error "Invalid or already-used code."
- Once published, the publish page redirects away (nothing to do).

### Visibility Enforcement

- **Public listing** (`TournamentList`): query filters `WHERE status='published'`.
- **Direct URL access**: `tournamentFromRoute` checks `tournament.status`. If `'draft'` and the requesting user is not an admin, director, or staff for that tournament → return 404 (not redirect, to avoid leaking existence).
- Directors and staff can always access their own draft tournament via direct URL after login.
- The `RequestLogger` `/manage/*` guard already enforces login + `CanManage` for management pages — no change needed there.

---

## Director Management Routes

All routes require login + `CanManage(tid)` (enforced by existing `RequestLogger` guard on `/tournaments/:tid/manage/*`).

### Dashboard
```
GET /tournaments/:tid/manage
```
Landing page with links to divisions, teams, games, locations, roles, extras, publish.

### Divisions
```
GET  /tournaments/:tid/manage/divisions
POST /tournaments/:tid/manage/divisions
GET  /tournaments/:tid/manage/divisions/:did/edit
POST /tournaments/:tid/manage/divisions/:did/edit
POST /tournaments/:tid/manage/divisions/:did/delete
```

### Teams
```
GET  /tournaments/:tid/manage/teams
POST /tournaments/:tid/manage/teams
GET  /tournaments/:tid/manage/teams/:teamid/edit
POST /tournaments/:tid/manage/teams/:teamid/edit
POST /tournaments/:tid/manage/teams/:teamid/delete
```

### Games & Scheduling
```
GET  /tournaments/:tid/manage/divisions/:did/games/new
POST /tournaments/:tid/manage/games
POST /tournaments/:tid/manage/divisions/:did/games/generate
GET  /tournaments/:tid/manage/games/:gid/edit
POST /tournaments/:tid/manage/games/:gid/edit
POST /tournaments/:tid/manage/games/:gid/delete
```

### Locations (per-tournament)
```
GET  /tournaments/:tid/manage/locations
POST /tournaments/:tid/manage/locations
GET  /tournaments/:tid/manage/locations/:lid/edit
POST /tournaments/:tid/manage/locations/:lid/edit
POST /tournaments/:tid/manage/locations/:lid/delete
```

### Publish
```
GET  /tournaments/:tid/manage/publish
POST /tournaments/:tid/manage/publish
```

### Already-existing manage routes (unchanged)
```
GET/POST /tournaments/:tid/manage/extras
GET/POST /tournaments/:tid/manage/roles
POST     /tournaments/:tid/manage/invite
POST     /tournaments/:tid/manage/roles/:uid/remove
```

---

## Handler Implementation Notes

- Division, team, and game handlers under `/manage/` reuse the same DB calls as their `/admin/` counterparts. The only differences are: route prefix, template names (new templates or shared), and that `locationsFor` is replaced by a tournament-scoped location lookup.
- The existing `/admin/tournaments/:tid/*` routes are **unchanged** — admins continue using them.
- `locationsFor(sport string)` currently filters global locations by sport. For the manage routes, replace with `locationsForTournament(tid int)` which queries `WHERE tournament_id = $1`.
- Score routes (`/tournaments/:tid/score/*`) are unchanged — staff use them already.

---

## Admin Routes Added

```
GET  /admin/tournaments/queue
POST /admin/tournaments/:tid/issue-code
```

The queue page is linked from the existing admin tournament list.

---

## Out of Scope (Subsystem B)

- Team hometown, season record, GameChanger link
- Coach roster management
- Coach-specific UI
