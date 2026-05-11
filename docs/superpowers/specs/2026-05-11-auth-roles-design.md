# Auth & Roles Implementation Design

## Goal

Replace the single shared admin password with email-based user accounts, email verification, password reset, and a per-tournament role system (director, staff, coach). Public read access remains unchanged for unauthenticated users.

## Architecture

User identity is stored in PostgreSQL. Sessions continue to use `github.com/rivo/sessions`, storing only the `user_id` integer. A middleware layer loads the full user + roles from DB on every authenticated request and injects them into the request context. Email is sent via `net/smtp` (plain auth, matching the advent-calendar pattern).

## Tech Stack

- Auth: bcrypt (`golang.org/x/crypto/bcrypt`), `github.com/rivo/sessions`
- Email: `net/smtp` (stdlib), HTML email bodies
- Tokens: 32-byte `crypto/rand` hex strings (verification, reset, invitation)

---

## Database Schema

Three new tables added to the auto-create list in `mydb/mydb.go`:

```sql
CREATE TABLE IF NOT EXISTS users (
    id                  SERIAL PRIMARY KEY,
    email               TEXT UNIQUE NOT NULL,
    name                TEXT NOT NULL,
    password_hash       TEXT NOT NULL,
    email_verified      BOOLEAN NOT NULL DEFAULT false,
    verification_token  TEXT,
    reset_token         TEXT,
    reset_expires       TIMESTAMPTZ,
    is_admin            BOOLEAN NOT NULL DEFAULT false,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tournament_roles (
    id             SERIAL PRIMARY KEY,
    user_id        INTEGER NOT NULL REFERENCES users(id),
    tournament_id  INTEGER NOT NULL REFERENCES tournaments(id),
    role           TEXT NOT NULL CHECK (role IN ('director','staff','coach')),
    team_id        INTEGER REFERENCES teams(id),
    UNIQUE(user_id, tournament_id)
);

CREATE TABLE IF NOT EXISTS invitations (
    id             SERIAL PRIMARY KEY,
    email          TEXT NOT NULL,
    tournament_id  INTEGER NOT NULL REFERENCES tournaments(id),
    role           TEXT NOT NULL CHECK (role IN ('director','staff','coach')),
    team_id        INTEGER REFERENCES teams(id),
    token          TEXT NOT NULL UNIQUE,
    expires_at     TIMESTAMPTZ NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Bootstrap:** The first user to register gets `is_admin = true` and skips email verification. All subsequent users must verify before logging in.

**Tournament creation:** `CreateTournament` auto-inserts a `director` row in `tournament_roles` for the creating user.

**Invitations:** When a user registers with an email that matches a pending, unexpired invitation, the role is automatically assigned and the invitation marked used (deleted).

---

## Auth Flow

### Registration (`GET/POST /register`)
- Form fields: email, name, password, confirm password
- On submit: validate inputs, bcrypt hash password, create user (unverified)
- Send verification email with 32-byte hex token
- Redirect to login page with "check your email" message
- If a pending invitation exists for that email: assign role immediately, delete invitation
- First user registered: `is_admin = true`, `email_verified = true` (no email sent)

### Email Verification (`GET /verify?token=xxx`)
- Look up user by `verification_token`
- Set `email_verified = true`, clear `verification_token`
- Redirect to login with success message
- Unverified users who attempt login see an error and a "resend verification email" link

### Login (`GET/POST /login`)
- Look up user by email, `bcrypt.CompareHashAndPassword`
- Reject if `email_verified = false` (show resend link)
- On success: store `user_id` in session, redirect to `/`
- Existing rate limiting (exponential backoff per IP) applies unchanged

### Logout (`POST /logout`)
- Destroy session, redirect to `/`

### Password Reset
- `GET/POST /password-reset` — user enters email; app generates 1-hour token, stores in `reset_token`/`reset_expires`, sends reset email
- `GET/POST /password-reset/confirm?token=xxx` — user enters new password; app verifies token not expired, bcrypt hashes new password, clears token

---

## Role Model

### Permissions

| Action | Public | No role | Coach | Staff | Director | Admin |
|---|---|---|---|---|---|---|
| View tournaments, standings, games | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Create tournament | | | | | | ✓ |
| Edit tournament settings | | | | | ✓ | ✓ |
| Add/edit/delete divisions & teams | | | | | ✓ | ✓ |
| Enter scores | | | | ✓ | ✓ | ✓ |
| Assign roles / invite users | | | | | ✓ | ✓ |

Coach-specific actions (roster, QR codes) are Sub-project 2 and not implemented here. The `coach` role and `team_id` column are created in this schema so Sub-project 2 can build on them without a migration.

### User Context

```go
type TWUser struct {
    ID      int
    Email   string
    Name    string
    IsAdmin bool
    Roles   []TournamentRole
}

type TournamentRole struct {
    TournamentID int
    Role         string // "director", "staff", "coach"
    TeamID       int    // non-zero for coach only
}
```

Helper methods on `TWUser`:
- `IsDirectorFor(tid int) bool`
- `IsStaffFor(tid int) bool`
- `IsCoachFor(tid int) bool`
- `CanManage(tid int) bool` — true if `IsAdmin` or `IsDirectorFor(tid)`
- `CanScore(tid int) bool` — true if `IsAdmin`, `IsDirectorFor(tid)`, or `IsStaffFor(tid)`
- `LoggedIn() bool` — true if `ID > 0`

### Middleware

One middleware function replaces the current `RequestLogger` auth guard:
1. Log the request (same as today)
2. Read `user_id` from session
3. If present: load user row + all `tournament_roles` rows in one JOIN query; inject `TWUser` into context
4. If absent: inject zero-value `TWUser` (unauthenticated)
5. Check route prefix guards:
   - `/admin/*` → require `IsAdmin`
   - `/tournaments/:tid/manage/*` → require `CanManage(tid)`

Individual handlers check `CanScore(tid)` or other helpers as needed.

---

## New Routes

### Public

| Method | Path | Handler |
|---|---|---|
| GET | `/register` | `RegisterForm` |
| POST | `/register` | `Register` |
| GET | `/verify` | `VerifyEmail` |
| POST | `/resend-verification` | `ResendVerification` — accepts email in form body, sends new token |
| GET | `/password-reset` | `PasswordResetForm` |
| POST | `/password-reset` | `PasswordReset` |
| GET | `/password-reset/confirm` | `PasswordResetConfirmForm` |
| POST | `/password-reset/confirm` | `PasswordResetConfirm` |
| POST | `/logout` | `Logout` |

### Director (requires `CanManage(tid)`)

| Method | Path | Handler |
|---|---|---|
| GET | `/tournaments/:tid/manage/roles` | `ManageRoles` |
| POST | `/tournaments/:tid/manage/roles` | `AssignRole` |
| POST | `/tournaments/:tid/manage/invite` | `InviteUser` |
| POST | `/tournaments/:tid/manage/roles/:uid/remove` | `RemoveRole` |

---

## Config Changes

**Removed:** `adminpassword` / `ADMIN_PASSWORD`

**Added:**

| Env var | Config key | Description |
|---|---|---|
| `SMTP_HOST` | `smtphost` | SMTP server hostname |
| `SMTP_PORT` | `smtpport` | SMTP port (default: 587) |
| `SMTP_USERNAME` | `smtpusername` | SMTP auth username |
| `SMTP_PASSWORD` | `smtppassword` | SMTP auth password |
| `FROM_EMAIL` | `fromemail` | Sender address for outgoing mail |
| `BASE_URL` | `baseurl` | Public URL for email links (e.g. `https://tourneyweb.example.com`) |
| `DISABLE_EMAIL_VERIFICATION` | `disableemailverification` | `true` skips verification for all users (dev only) |

---

## File Structure

### New files

| File | Responsibility |
|---|---|
| `webhandler/auth.go` | Register, VerifyEmail, Login, Logout, PasswordReset* handlers |
| `webhandler/roles.go` | ManageRoles, AssignRole, InviteUser, RemoveRole handlers |
| `webhandler/email.go` | EmailService: sendEmail, SendVerificationEmail, SendPasswordResetEmail, SendInvitationEmail |
| `mydb/users.go` | CreateUser, GetUserByEmail, GetUserByID, GetUserByVerificationToken, GetUserByResetToken, MarkEmailVerified, SetResetToken, UpdatePassword, SetAdmin, CountUsers |
| `mydb/roles.go` | GetRolesForUser, AssignRole, RemoveRole, CreateInvitation, GetInvitationByToken, DeleteInvitation, GetInvitationByEmail |
| `webhandler/templates/auth/register.html` | Registration form |
| `webhandler/templates/auth/verify.html` | "Check your email" / verification result page |
| `webhandler/templates/auth/password_reset.html` | Request reset form |
| `webhandler/templates/auth/password_reset_confirm.html` | New password form |
| `webhandler/templates/admin/manage_roles.html` | Role management page for directors |

### Modified files

| File | Change |
|---|---|
| `webhandler/webhandler.go` | New TWUser/TournamentRole structs, updated middleware, helper methods, remove AdminPW |
| `webhandler/tournaments.go` | Auto-assign director role in CreateTournament |
| `mydb/mydb.go` | Add users, tournament_roles, invitations to pgtables |
| `config.go` | Add SMTP/BaseURL/DisableEmailVerification fields, remove AdminPassword |
| `main.go` | Wire EmailService into Env, register new routes, remove AdminPW wiring |
| `webhandler/templates.go` | Add data structs for new templates |
| `webhandler/templates/layout.html` | Nav: show login/register or username+logout based on session |

---

## Email Templates

All HTML, matching advent-calendar style (`font-family: Arial`, `max-width: 600px`, button links).

- **Verification:** subject "Verify your TourneyWeb email", button → `{BASE_URL}/verify?token=xxx`
- **Password reset:** subject "Reset your TourneyWeb password", button → `{BASE_URL}/password-reset/confirm?token=xxx`, expires note
- **Invitation:** subject "You've been invited to manage {Tournament Name}", explains role, button → `{BASE_URL}/register`, notes account will be pre-configured on signup
