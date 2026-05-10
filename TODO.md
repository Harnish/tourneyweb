# TourneyWeb — Improvement Backlog

Priority: **P0** critical · **P1** high · **P2** medium · **P3** low  
Effort: **S** small (hours) · **M** medium (a day or two) · **L** large (several days+)

---

## Security

- **[P1/S]** No rate limiting on `/login` — brute-force the single admin password with no friction; add a simple attempt counter or exponential backoff
- **[P1/S]** Admin password stored in plaintext in config — interim fix is bcrypt hashing and `$TANADMINPASS` env var support; superseded by full auth system below once that ships

---

## Bugs

- **[P1/S]** `LoadFavico()` is defined in `main.go` but never called — the favicon never loads
---

## Code Quality

- **[P1/S]** Replace deprecated `ioutil.ReadFile` with `os.ReadFile` in `config.go` and `main.go` (deprecated since Go 1.16)
- **[P2/M]** Add unit tests for the team ranking algorithms in `webhandler/sortteams.go` — the two sort strategies (`WinsRunsAgainstRunsEarnedHead2Head`, `WinsHead2HeadRunsAgainstRunsEarned`) have complex tie-breaking logic with no test coverage
- **[P2/M]** Mock PostgreSQL for testing — introduce a DB interface so unit tests can inject a fake without a real database; use `pgxmock` or a hand-rolled fake
- **[P2/S]** Fix CSS served inline from `main.go` — move to a real static file or embed with `//go:embed`

---

## Missing Features

- **[P1/M]** Edit support — no entity can be edited after creation; need edit forms for Tournaments, Divisions, Teams, and Games
- **[P1/S]** Show division in the all-games table (`/tournaments/:tid/games`) — the games table stores `division_id` but the public games view omits it
- **[P2/M]** Fields/Locations UI — the `locations` table is defined in the schema but there are no routes, forms, or display pages for it
- **[P2/M]** News UI — the `event_news` table is defined in the schema but has no routes or display
- **[P2/S]** Make teams clickable links from the division standings view — team names appear as plain text but `ShowTeam` route already exists at `/tournaments/:tid/teams/:teamid`
- **[P2/S]** Make the HR Derby info page content configurable — it is entirely hardcoded in `webhandler/webhandler.go` with event-specific text, Venmo links, and signup URLs; should come from the DB or config
- **[P2/M]** Use a proper datetime type for game start times — `start_time` is stored as TEXT; a real timestamp enables sorting, calendar view, and validation
- **[P2/M]** Add delete confirmation — currently clicking delete on a game or team immediately executes with no "are you sure?" step
- **[P3/M]** Client-side table sorting — allow clicking column headers to sort standings and game lists
- **[P3/L]** Calendar view — display games on a calendar grouped by date
- **[P3/L]** Bracket/playoff support — see Major Features below

---

## Major Features

These are multi-sprint architectural additions. Each depends on the ones above it.

### Authentication & Multi-user

- **[P1/L]** Full user authentication system — replace the single shared admin password with email-based accounts: registration with email verification link, bcrypt password hashing, password reset flow, and session management. Username is the email address. This is a prerequisite for everything else in this section.
- **[P1/M]** Tournament Director role — a registered user can be designated as Tournament Director for a tournament; only the TD can edit tournament settings, divisions, teams, and ranking rules. Separate from score-entry staff.
- **[P2/M]** Score-entry staff — Tournament Director can invite additional registered users (by email) to enter scores for a specific tournament; they get a scoped permission to POST scores only, not edit structure.
- **[P1/S]** Public read access — unauthenticated users must continue to be able to view all tournaments, schedules, division standings, and scores without logging in.

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

### QR Codes

- **[P3/S]** Team QR code — generate a QR code per team linking to their schedule/score page; display on the team detail page and on a printable team sheet. Use `github.com/skip2/go-qrcode` server-side.

---

## Ops / Deployment

- **[P2/S]** Add `/healthz` endpoint — needed for container orchestration liveness/readiness probes (the `HEALTHCHECK` in the Dockerfile currently hits `/` which renders a full page)
- **[P2/S]** Add graceful shutdown — `log.Fatal(http.ListenAndServe(...))` has no cleanup path; use `http.Server` with context cancellation on `SIGTERM`/`SIGINT`
- **[P3/S]** Structured logging — replace scattered `log.Println` with `log/slog` to make log output parseable in production
