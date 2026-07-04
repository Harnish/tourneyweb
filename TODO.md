# TourneyWeb — Improvement Backlog

Priority: **P0** critical · **P1** high · **P2** medium · **P3** low  
Effort: **S** small (hours) · **M** medium (a day or two) · **L** large (several days+)

---

---

## Code Quality


---

## Missing Features

- **[P3/M]** Client-side table sorting — allow clicking column headers to sort standings and game lists
- **[P3/L]** Calendar view — display games on a calendar grouped by date
- **[P3/L]** Bracket/playoff support — see Major Features below

---

## Major Features

These are multi-sprint architectural additions. Each depends on the ones above it.

### Roster & QR Codes

- **[P2/S]** Team QR code — coaches can upload a custom QR code image or generate one server-side (via `github.com/skip2/go-qrcode`) linking to the team's public page. Displayed on the team detail page and a printable team sheet. Roster feature has shipped, so this is unblocked.

---

## Bugs
---

## Ops / Deployment

- **[P2/M]** Access log metrics pipeline — parse structured JSON access logs (method, path, status, latency, IP) into aggregated metrics (request counts, p50/p95 latency, top paths, error rates) instead of writing every request row to SQLite. Ship a lightweight batch job or log-tail processor that emits to a time-series store or a static summary report on a schedule.

---

## Recently Completed

- **Double-elimination brackets** — Directors choose single- or double-elimination when starting a bracket (double-elim gated to team counts that are an exact power of 2); losers bracket, grand final, and bracket-reset game (if the losers-bracket entrant wins the grand final) all auto-generate and cascade on scoring; public bracket page renders Winners/Losers/Grand Final sections
- **Visual bracket display + drag-and-drop seeding** — single-elimination bracket tree with connectors, bye/TBD/winner states, and "Seeded by: …" label; seed review page supports native drag-and-drop reorder in addition to ↑/↓ buttons
- **Default ranking criteria at creation** — Directors/admins set a default division ranking order when creating a tournament; applied automatically to new divisions; locked on publish with admin override; manage dashboard editable until publish
- **Bracket transition in director views** — Directors can start bracket phase from manage/divisions; Start Bracket button, phase column, and bracket status links added to director manage view
- **Roster management** — Coaches manage player rosters (name, number, position, handedness); public team pages show condensed roster (first initial + last name); 6 routes under `/tournaments/:tid/manage/teams/:teamid/roster`
- **Configurable rankings** — Directors configure tiebreaker order per division via ordered checklist (9 criteria: wins, win pct, head-to-head, run diff, runs against, RA/game, runs for, H2H RA, coin flip); stored as JSON in DB; public division page shows "Ranked by: …" label
- **Map pin fix** — Leaflet marker icon fixed via `L.Icon.Default.mergeOptions()` with explicit CDN URLs; interactive map pickers added to director manage/locations and manage/edit_location forms (previously had only bare lat/lng inputs)
- **Per-division rules editor** — Tournament Director authors rules per division with Quill WYSIWYG; public division page shows division rules with fallback to tournament-level rules; `rules_html` column on both `divisions` and `tournaments` tables
- **News UI** — per-tournament news CRUD for directors at `/tournaments/:tid/manage/news`; admin global news at `/admin/news`; public display on tournament home page
- **Dark navy + gold theme** — full CSS rewrite with custom nav, card/table/form overrides, Bootstrap integration; replaces default Bootstrap look
- **Public fields page** — `/tournaments/:tid/fields` lists all tournament field locations with Google Maps links; surfaced as a main category on the tournament home page
- **Tournament Event Extras** — per-tournament freeform page (`/tournaments/:tid/extras`) editable by directors via Quill WYSIWYG at `/tournaments/:tid/manage/extras`; replaces hardcoded `/hrderbyinfo` route; `extras_html` column added to `tournaments` table via migration
- **Structured logging** — all `log.Println`/`log.Fatalf`/`log.Printf` replaced with `log/slog`; JSON handler set as default in `main()`; request logs include method/path/proto/ip as fields; `github.com/davecgh/go-spew` dependency removed
- **Proper game start time** — `start_time` migrated from TEXT to TIMESTAMPTZ; forms use `datetime-local` input; displays formatted as "Jan 2, 2006 3:04 PM"; zero value renders as empty string
- **Ranking algorithm tests** — 14 tests covering both sort strategies and all three helper functions; uses FakeDB for head-to-head scenarios
- **Fields/Locations UI** — CRUD admin at `/admin/locations`, public map at `/map`; Leaflet + OpenStreetMap, click-to-place pin on add/edit forms; `latitude`/`longitude` added to `locations` table via migration
- **DB interface + FakeDB** — `mydb.DB` interface lets handlers accept either `*MyDB` or `*FakeDB`; in-memory fake covers all 50 methods with 6 passing tests
- **Static asset embeds** — `Banner.png`, `favicon.ico`, and `style.css` all embedded with `//go:embed`; no disk files required at runtime
- **Login rate limiting** — DB-backed per-IP exponential lockout (2s–256s cap), persists across restarts, prunes stale records on startup
- **Authentication system** — email-based accounts with bcrypt passwords, email verification link, password reset flow; first registered user is auto-admin
- **Role-based access control** — `admin` (site-wide), `director` (per-tournament full management), `staff` (score entry only), `coach` (reserved); middleware enforces roles on `/admin/*`, `/tournaments/:tid/manage/*`, and `/tournaments/:tid/score/*`
- **Invitations** — directors invite unregistered users by email; pending invitation auto-applied on registration
- **Tournament director auto-assign** — creating a tournament grants the creator a `director` role automatically
- **Score entry for staff** — staff users see a "Score" link on the public games page
- **Delete confirmations** — browser `confirm()` dialogs on all delete buttons (games, teams, divisions)
- **Proper auth error handling** — unauthenticated users are redirected to `/login`; authenticated users lacking a role get a rendered 403 page
