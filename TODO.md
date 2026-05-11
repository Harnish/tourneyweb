# TourneyWeb — Improvement Backlog

Priority: **P0** critical · **P1** high · **P2** medium · **P3** low  
Effort: **S** small (hours) · **M** medium (a day or two) · **L** large (several days+)

---

---

## Code Quality


---

## Missing Features

- **[P2/M]** News UI — the `event_news` table is defined in the schema but has no routes or display
- **[P2/S]** Make the HR Derby info page content configurable — it is entirely hardcoded in `webhandler/webhandler.go` with event-specific text, Venmo links, and signup URLs; should come from the DB or config
- **[P2/M]** Use a proper datetime type for game start times — `start_time` is stored as TEXT; a real timestamp enables sorting, calendar view, and validation
- **[P3/M]** Client-side table sorting — allow clicking column headers to sort standings and game lists
- **[P3/L]** Calendar view — display games on a calendar grouped by date
- **[P3/L]** Bracket/playoff support — see Major Features below

---

## Major Features

These are multi-sprint architectural additions. Each depends on the ones above it.

### Rankings

- **[P2/M]** Configurable ranking algorithm per division — Tournament Director selects the tiebreaker order from a preset list. Supported criteria (in addition to the two already coded):
  - Win percentage (wins / games played)
  - Run differential (runs for minus runs against, capped per game)
  - Fewest runs allowed per game
  - Head-to-head record (already partially implemented)
  - Fewest runs allowed in head-to-head games
  - Coin flip / random seed as final fallback
  - Forfeits treated as losses with fixed score (e.g., 7-0)
  - Store selected algorithm order in DB per division; default to current `WinsRunsAgainstRunsEarnedHead2Head` behavior.

### Brackets

- **[P2/L]** Visual bracket display — render single-elimination and double-elimination brackets as SVG or structured HTML; seeds drawn from division standings. Tournament Director chooses format per division. Bracket advances automatically when scores are entered.
- **[P2/M]** Bracket seeding and generation — UI to review auto-seeded bracket, manually adjust seed order via drag-and-drop, then lock and publish.

### Division Rules

- **[P2/M]** Per-division rules editor — Tournament Director can author rules for each division using a rich-text editor; stored as HTML in a TEXT column. Public division page renders rules below the standings.

### Roster & QR Codes

- **[P2/M]** Player roster management — coaches can manage their team's roster (player name, number, date of birth) via a coach-role UI. Public views show only first initial + last name and omit DOB. Requires auth+roles to ship first.
- **[P2/S]** Team QR code — coaches can upload a custom QR code image or generate one server-side (via `github.com/skip2/go-qrcode`) linking to the team's public page. Displayed on the team detail page and a printable team sheet. Requires roster feature.

---

## Ops / Deployment

- **[P3/S]** Structured logging — replace scattered `log.Println` with `log/slog` to make log output parseable in production

---

## Recently Completed

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
