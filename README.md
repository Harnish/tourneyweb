# TourneyWeb

TourneyWeb is an open-source Go web application for managing sports tournaments. Initially built for baseball, it works for any sport with scheduled games between two teams. It tracks divisions, teams, games, and standings with configurable tiebreaker rankings.

Check out [TODO.md](TODO.md) if you want to help out.

---

## Features

- **Multi-tournament** — manage multiple tournaments simultaneously, each with their own divisions, teams, and schedule
- **Division standings** — automatic win/loss/runs standings with two configurable tiebreaker algorithms
- **Game scheduling** — create and edit games with location, date/time, and umpire fields
- **Score entry** — score games with 0–40 per-team run counts; standings update immediately
- **Full edit support** — edit tournaments, divisions, teams, and games after creation
- **User authentication** — email-based accounts with bcrypt passwords, email verification, and password reset flow
- **Role-based access control** — three roles per tournament: `director` (full tournament management), `staff` (score entry only), `coach` (reserved); site-wide `admin` flag for system administration
- **Invitations** — directors can invite unregistered users by email; role is applied automatically on registration
- **Score entry for staff** — staff users see a "Score" link on the public games page; directors get a full admin score view
- **CSRF protection** — all state-changing forms are protected with `gorilla/csrf`
- **Delete confirmation** — browser confirm dialogs guard all delete operations
- **Delete lock** — optional `disabledelete` flag prevents accidental deletions during live tournaments
- **Container-ready** — Docker image with multi-stage build, non-root user, and health check

---

## Building

```bash
go build
```

Requires Go 1.21+. All dependencies are managed with Go modules.

---

## Database Setup

TourneyWeb uses **PostgreSQL**. Tables are created automatically on first startup — you only need to provision the database and user.

Use [homelab_db_provisioner](https://github.com/Harnish/homelab_db_provisioner) to provision the database via a JSON config:

```json
{
  "servers": [
    {
      "name": "local",
      "connection": "postgres://postgres:rootpassword@localhost:5432/postgres?sslmode=disable",
      "databases": [
        {
          "database": "tourneyweb",
          "username": "tourneyweb1",
          "password": "CHANGE_ME"
        }
      ]
    }
  ]
}
```

Run the provisioner (Docker example):

```bash
docker run --rm \
  -v $(pwd)/config.json:/config/config.json \
  ghcr.io/harnish/homelab_db_provisioner
```

Or manually:

```sql
CREATE DATABASE tourneyweb;
CREATE USER tourneyweb1 WITH PASSWORD 'CHANGE_ME';
GRANT ALL PRIVILEGES ON DATABASE tourneyweb TO tourneyweb1;
-- PostgreSQL 15+: also grant schema ownership
ALTER DATABASE tourneyweb OWNER TO tourneyweb1;
```

---

## Configuration

Copy the example config and fill in your values:

```bash
cp tourneyweb.conf.example tourneyweb.conf
```

```yaml
port: 8989
debug: false
database: postgres://tourneyweb1:CHANGE_ME@localhost:5432/tourneyweb?sslmode=disable
csrfkey: CHANGE_ME_generate_with_openssl_rand_hex_32
disabledelete: false
bannerimagepath: dawgpoundlogo.jpg
```

Generate a CSRF key:

```bash
openssl rand -hex 32
```

Config is loaded from `tourneyweb.conf`, then `config.yaml`, then `/etc/go-periodical-rack/config.yaml` — first file found wins.

### Environment Variable Overrides

All config values can be overridden with environment variables (useful for containers):

| Variable | Config key |
|---|---|
| `PORT` | `port` |
| `DEBUG` | `debug` |
| `DATABASE_URL` | `database` |
| `CSRF_KEY` | `csrfkey` |
| `DISABLE_DELETE` | `disabledelete` |
| `BANNER_IMAGE_PATH` | `bannerimagepath` |
| `SMTP_HOST` | `smtphost` |
| `SMTP_PORT` | `smtpport` |
| `SMTP_USERNAME` | `smtpusername` |
| `SMTP_PASSWORD` | `smtppassword` |
| `FROM_EMAIL` | `fromemail` |
| `BASE_URL` | `baseurl` |
| `DISABLE_EMAIL_VERIFICATION` | `disableemailverification` |

---

## Running

```bash
./tourneyweb
```

The app listens on the configured port (default `8989`). All tables are created automatically if they don't exist.

---

## Docker

### Build

```bash
docker build -t tourneyweb .
```

### Run

```bash
docker run -d \
  -p 8989:8989 \
  -e DATABASE_URL="postgres://tourneyweb1:CHANGE_ME@db:5432/tourneyweb?sslmode=disable" \
  -e CSRF_KEY="$(openssl rand -hex 32)" \
  -v /path/to/banner.jpg:/app/banner.jpg \
  -e BANNER_IMAGE_PATH=/app/banner.jpg \
  tourneyweb
```

Pre-built images are published to GitHub Container Registry on every push to `master`:

```bash
docker pull ghcr.io/harnish/tourneyweb:latest
```

---

## URL Structure

### Public

| Path | Description |
|---|---|
| `/` | Tournament list |
| `/tournaments/:tid` | Tournament home / division standings |
| `/tournaments/:tid/divisions/:did` | Division standings detail |
| `/tournaments/:tid/teams/:teamid` | Team schedule and results |
| `/tournaments/:tid/games` | Full game schedule |

### Auth

| Path | Description |
|---|---|
| `/login` | Sign in |
| `/register` | Create account |
| `/verify` | Email verification (link from email) |
| `/resend-verification` | Resend verification email |
| `/password-reset` | Request password reset |
| `/password-reset/confirm` | Set new password (link from email) |

### Score entry (directors + staff)

| Path | Description |
|---|---|
| `/tournaments/:tid/score/games/:gid` | Enter score for a game |

### Director management (directors + admins)

| Path | Description |
|---|---|
| `/tournaments/:tid/manage/roles` | View/assign/remove roles, invite users |

### Admin (requires `is_admin`)

| Path | Description |
|---|---|
| `/admin/tournaments` | List / create tournaments |
| `/admin/tournaments/:tid` | Tournament overview |
| `/admin/tournaments/:tid/edit` | Edit tournament |
| `/admin/tournaments/:tid/divisions` | Add division |
| `/admin/tournaments/:tid/divisions/:did` | Division admin view |
| `/admin/tournaments/:tid/divisions/:did/edit` | Edit division |
| `/admin/tournaments/:tid/teams` | Add / list teams |
| `/admin/tournaments/:tid/teams/:teamid/edit` | Edit team |
| `/admin/tournaments/:tid/games` | Game list with score / edit / delete |
| `/admin/tournaments/:tid/divisions/:did/games/new` | Create game |
| `/admin/tournaments/:tid/games/:gid/score` | Enter score |
| `/admin/tournaments/:tid/games/:gid/edit` | Edit game |

---

## Standings / Rankings

Two tiebreaker algorithms are available. Both sort by wins first, then apply different secondary criteria:

- `WinsRunsAgainstRunsEarnedHead2Head` — fewest runs allowed, then most runs scored, then head-to-head
- `WinsHead2HeadRunsAgainstRunsEarned` — head-to-head first, then fewest runs allowed, then most runs scored

The algorithm is selected per-call in the handler. A configurable per-division algorithm is on the roadmap (see TODO.md).

---

## Development Notes

- No ORM — direct SQL via `jackc/pgx/v5/stdlib` using `database/sql`
- HTML rendering uses `html/template` with layouts in `webhandler/templates/`
- All admin routes share the `RequestLogger` middleware which enforces session auth
- The `DisableDelete` flag is checked in handlers before any delete operation
- `ScoreGame` writes to both `games` (final score columns) and `games_by_team` (one row per team per game, used for standings); `UpdateGame` re-syncs `games_by_team` if the game is already scored
