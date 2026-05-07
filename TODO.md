# TourneyWeb — Improvement Backlog

Priority: **P0** critical · **P1** high · **P2** medium · **P3** low  
Effort: **S** small (hours) · **M** medium (a day or two) · **L** large (several days+)

---

## Security

- **[P0/S]** `tourneyweb.conf` contains real credentials and is committed to git — rotate the DB password and admin password immediately, add `tourneyweb.conf` to `.gitignore`, and document using a separate untracked config or environment variables instead
- **[P0/M]** No CSRF protection on any POST form — all admin forms (add/delete division, team, game, score) are vulnerable to cross-site request forgery; add CSRF token middleware
- **[P0/M]** XSS — user-supplied strings (team names, coach names, division names, location, umpire) are injected directly into HTML without escaping; every one of these is an XSS vector (`webhandler/webhandler.go`, `teams.go`, `divisions.go`)
- **[P1/S]** No rate limiting on `/login` — brute-force the single admin password with no friction; add a simple attempt counter or exponential backoff
- **[P1/S]** Admin password stored in plaintext in config — superseded by the full auth system below once that ships; interim fix is bcrypt hashing and `$TANADMINPASS` env var support

---

## Bugs

- **[P1/S]** `LoadFavico()` is defined in `main.go:97` but never called — the favicon never loads
- **[P1/S]** `method=port` typo in delete-team form (`webhandler/teams.go:59`) — the delete button silently does nothing because the form never POSTs
- **[P1/S]** `localdb` fallback mode is broken in two ways (`mydb/mydb.go:43`): the connection string is missing the `@` separator before `tcp(...)`, and `createTestDatabase()` creates a database named `test`, not `tourneyweb`
- **[P1/S]** `DelGame` (`webhandler/webhandler.go:342`) has no redirect after deletion — user is left on a blank page
- **[P1/S]** `ScoreGame` retry on failure (`mydb/games.go:32`, `teams.go:29`) re-executes the identical statement without any change — the retry will always fail the same way
- **[P1/S]** `ReturnTeamsByDivisionIDTable` (`webhandler/webhandler.go:293`) is never called from public routes — `ReturnTeamsByDivisionIDRankedTable` replaced it but the old function wasn't removed
- **[P1/S]** `TeamOptions` HTML is missing quotes around attribute values (`webhandler/teams.go:74`): `id=N value=N` should be `id="N" value="N"`

---

## Code Quality

- **[P1/M]** Replace string-concatenated HTML with `html/template` — the current approach makes XSS impossible to fix systematically, is error-prone to maintain, and makes the UI impossible to redesign without touching Go code
- **[P1/S]** Replace deprecated `ioutil.ReadFile` with `os.ReadFile` in `config.go:47`, `main.go:89`, and `main.go:99` (deprecated since Go 1.16)
- **[P1/S]** Remove or wire up dead code:
  - `GetEnvironmentConfig()` in `config.go:59` — defined but never called
  - `exists()` in `localdb/localdb.go:70` — unused
  - Unreachable `return nil` in `mydb/mydb.go:63` — after the real `return database`
- **[P1/S]** Standardize all SELECT/DELETE queries to use `?` placeholders — the `mydb` package mixes parameterized inserts/updates with string-concatenated SELECTs/DELETEs; IDs are safe via `strconv.Itoa` today but the inconsistency is fragile
- **[P1/M]** Update `go.mod` from `go 1.14` to a current version (1.21+) and audit/update dependencies — several packages are years old
- **[P1/S]** Add `.gitignore` — the repo has none; binary, config with credentials, and IDE files are all unprotected
- **[P2/M]** Add unit tests for the team ranking algorithms in `webhandler/sortteams.go` — the two sort strategies (`WinsRunsAgainstRunsEarnedHead2Head`, `WinsHead2HeadRunsAgainstRunsEarned`) have complex tie-breaking logic with no test coverage
- **[P2/S]** Fix CSS served inline from `main.go:67` — move to a real static file or embed it properly; the current approach buries CSS in Go source

---

## Missing Features

- **[P1/M]** Edit support — no entity can be edited after creation; need edit forms for Divisions, Teams, and Games
- **[P1/S]** Show division in the all-games table (`/games` route) — the `GAMES` table stores `divisionid` but the public games view omits it
- **[P1/S]** Remember the selected division on the Add Team form — after adding a team it resets to the first division
- **[P2/M]** Fields/Locations UI — the `LOCATION` table is defined in the schema but there are no routes, forms, or display pages for it
- **[P2/M]** News UI — the `EVENTNEWS` table is defined in the schema but has no routes or display
- **[P2/S]** Make teams clickable links from the division standings view — team names appear as plain text but `ShowTeam` route already exists at `/teams/:id`
- **[P2/S]** Make the HR Derby info page content configurable — it is entirely hardcoded in `webhandler/webhandler.go:452` with event-specific text, Venmo links, and signup URLs; should come from the DB or config
- **[P2/M]** Use a proper datetime type for game start times — `starttime` is `Varchar(255)` in the schema; storing it as a real datetime enables sorting, calendar view, and validation
- **[P2/M]** Add delete confirmation — currently clicking delete on a game or team immediately executes with no "are you sure?" step
- **[P3/M]** Client-side table sorting — allow clicking column headers to sort standings and game lists
- **[P3/L]** Calendar view — display games on a calendar grouped by date
- **[P3/L]** Bracket/playoff support — superseded by the full bracket feature in Major Features below

---

## Major Features

These are multi-sprint architectural additions. Each depends on the ones above it.

### Authentication & Multi-user

- **[P1/L]** Full user authentication system — replace the single shared admin password with email-based accounts: registration with email verification link, bcrypt password hashing, password reset flow, and session management. Username is the email address. This is a prerequisite for everything else in this section.
- **[P1/M]** Tournament Director role — a registered user can be designated as Tournament Director for a tournament; only the TD can edit tournament settings, divisions, teams, and ranking rules. Separate from score-entry staff.
- **[P2/M]** Score-entry staff — Tournament Director can invite additional registered users (by email) to enter scores for a specific tournament; they get a scoped permission to POST scores only, not edit structure.
- **[P1/S]** Public read access — unauthenticated users must continue to be able to view all tournaments, schedules, division standings, and scores without logging in (preserve current behavior, just don't break it during auth refactor).

### Multiple Tournaments

- **[P1/L]** Multi-tournament support — all current data (divisions, teams, games, standings) is implicitly one tournament; add a `TOURNAMENTS` table and scope every query to a tournament ID. Tournament Directors create new tournaments. Public landing page lists all active/past tournaments. This is a significant schema and routing change that touches every package.

### Database

- **[P1/L]** Migrate from MySQL to PostgreSQL — replace `github.com/go-sql-driver/mysql` with `lib/pq` or `jackc/pgx`; update all schema DDL (MySQL `AUTO_INCREMENT` → `SERIAL`, `Varchar` → `TEXT`, etc.); update the `localdb` in-memory fallback. PostgreSQL is the project standard per global config.
- **[P2/M]** Mock PostgreSQL for testing — introduce a DB interface (e.g., `mydb.Store` interface) so unit tests can inject a fake implementation without a real database; use `pgxmock` or a hand-rolled fake. Required before meaningful test coverage is achievable.

### Rankings

- **[P2/M]** Configurable ranking algorithm per division — Tournament Director selects the tiebreaker order from a preset list and can reorder via drag-and-drop. Supported criteria to implement (in addition to the two already coded):
  - Win percentage (wins / games played)
  - Run differential (runs for minus runs against, capped per game to limit blowout effect)
  - Fewest runs allowed per game
  - Head-to-head record (already partially implemented)
  - Fewest runs allowed in head-to-head games
  - Coin flip / random seed as final fallback
  - Forfeits treated as losses with fixed score (e.g., 7-0)
  - Store selected algorithm order in DB per division; default to current `WinsRunsAgainstRunsEarnedHead2Head` behavior.

### Brackets

- **[P2/L]** Visual bracket display — render single-elimination and double-elimination brackets as SVG or structured HTML; bracket seeds are drawn from division standings at time of bracket generation. Tournament Director chooses elimination format (single or double) per division. Bracket advances automatically when scores are entered. Public users can view the live bracket without login.
- **[P2/M]** Bracket seeding and generation — UI for Tournament Director to review auto-seeded bracket (from standings), manually adjust seed order via drag-and-drop, then lock and publish the bracket.

### Division Rules

- **[P2/M]** Per-division rules editor — Tournament Director can author rules for each division using a WYSIWYG rich-text editor (e.g., Quill or TipTap embedded via CDN); stored as HTML in a `TEXT` column. Public division page renders the rules below the standings. Also support pasting a large plain-text dump as a fallback input mode.

### QR Codes

- **[P3/S]** Team QR code — generate a QR code per team that links to their schedule/score page (`/teams/:id`); display on the team detail page and on a printable team sheet. Intended for teams to link to their GameChanger app or share with parents. Use a server-side QR library (e.g., `github.com/skip2/go-qrcode`) so no external service is called.

---

## Ops / Deployment

- **[P1/S]** Wire up `GetEnvironmentConfig()` so the app can be configured via environment variables without a config file on disk — the function exists in `config.go:59` but `main.go` never calls it
- **[P2/M]** Add Dockerfile — multi-stage build, non-root user, `HEALTHCHECK` instruction
- **[P2/M]** Add CI/CD — GitHub Actions workflow: build + vet on PR, build and push Docker image on merge to main
- **[P2/S]** Add `/health` or `/healthz` endpoint — needed for container orchestration liveness/readiness probes
- **[P2/S]** Add graceful shutdown — `log.Fatal(http.ListenAndServe(...))` has no cleanup path; use `http.Server` with context cancellation on `SIGTERM`/`SIGINT`
- **[P3/S]** Structured logging — replace scattered `log.Println` with a structured logger (e.g., `log/slog` from Go 1.21) to make log output parseable in production
