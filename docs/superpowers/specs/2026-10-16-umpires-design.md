# Umpire accounts and game assignments — design

## Intent

Today a game has one free-text `umpire` field. Tournament management wants to
staff each game with the right number of umpires (1 or 2 depending on the
age level), prefer registered umpires who can be notified and can accept or
decline, and let umpires see where and when they are working. Free-form names
must still work for umpires without accounts.

## Goals

- Any user can mark themselves an umpire in their **profile** and attach credentials.
- Directors set how many umpires each division's games need (default 1).
- Directors (and admins) fill each game's umpire slots with a registered umpire or a free-form name.
- A registered umpire is emailed when assigned and can accept or decline.
- A declined slot reopens and the tournament's directors are emailed.
- Umpires get a "my games" view: date/time, field, division, teams, partner, status.
- Directors can mark an umpire **verified for their tournament**.
- The app blocks assigning an umpire to overlapping games.

## Non-goals

- Travel-distance limits on who is offered (tracked in `TODO.md`, "Umpire travel limits").
- Umpire self-signup to games / availability calendars.
- Verifying certificates automatically; "verified" is a human judgment.
- Notifying free-form umpires (no account, no email).
- Per-tournament configurable game length (fixed 2 hours).

## Anti-goals

- Credentials must not leak: certificate URL and image are visible only to
  site admins, directors (of any tournament), and the umpire themself. They
  must never appear on public pages. Every user flagged as umpire appears in
  every director's picker (confirmed: umpires opt in by registering).
- Free-form names must not silently require an account.

## Decisions (from the conversation)

- **Who manages:** tournament directors (and site admins, as everywhere else).
- **Slot count:** per division, set by directors; each game gets that many slots.
- **Slot content:** a registered user flagged as umpire, or free-form text.
- **Accept/decline:** registered umpires only. Decline reopens the slot and emails directors.
- **Umpire flag and credentials** live on the user's **profile page** (a new page, not the summary/landing page): an "I am an umpire" checkbox, a certification URL, and an optional certificate image upload.
- **Verification** is per umpire *per tournament*, set by that tournament's director. Informational only: unverified umpires may still be assigned. Picker shows a verified badge.
- **Conflicts:** each game blocks its umpires for a fixed 2 hours from `start_time`; assigning an umpire whose window overlaps another of their games is rejected with an error (across all tournaments). Games without a start time are not checked.
- **UI shape:** see mockup (umpire "my games" grouped by day with Accept/Decline; game form with one picker per slot that also accepts typed names and shows verified/blocked state).

## Builder's calls (mine, not confirmed — please check)

- Notification = email via existing `EmailService` (`webhandler/email.go`) plus a "Pending" badge on the umpire view. Reuse existing mail setup; no new dependency.
- Existing `games.umpire` text values migrate to a free-form slot 1 on that game; the column stays but is no longer read or written by new code (drop later).
- Slot count is changeable later; if lowered below filled slots, extra assigned slots are kept but flagged, not deleted.
- Certificate image: JPG/PNG/PDF, max 5 MB, stored in the database (bytea) and served only through an auth-checked handler. A URL alone is sufficient; image is optional.
- Declined slots stay visible to management as "declined by X" until refilled; the declining umpire is not re-offered for that game by default but can be reassigned manually.
- Unflagging "I am an umpire" does not delete past assignments; they stay as history and the person is hidden from the picker.
- "My games" lists upcoming games first, past games below, across all tournaments.

## More builder's calls (from the review)

- **Picker UI:** server-rendered. Per slot: a `<select>` of eligible umpires (name, verified badge; umpires blocked by overlap shown disabled) plus a free-text input; typed text is used only if no umpire is selected. Conflicts are enforced on submit with an error message; no live JS check. Mockup showed: umpire "my games" grouped by day (time, game, field, partner, status, Accept/Decline) and a game form with one picker per slot.
- **Permissions:** only directors and admins touch umpire slots/settings. If staff can reach a game form, the umpire fields are hidden/ignored for them.
- **Game edits:** changing `start_time` re-runs the overlap check (reject on conflict). Changing time or location resets that game's accepted registered slots to pending and re-emails them. Deleting a game deletes its slots and emails registered assignees. Swapping an umpire out emails the removed one.
- **Overlap edges:** windows are half-open (`[start, start+2h)`), so back-to-back games at 10:00 and 12:00 are fine. Declined slots do not block. Free-form names are never conflict-checked. Times stay `TIMESTAMPTZ` as elsewhere. Do the check and insert in one transaction.
- **Migration:** idempotent SQL (`INSERT ... WHERE NOT EXISTS`) in the existing migration list. Slot rows are created lazily, on assignment; "needed" count comes from the division.
- **Existing callers:** leave `AddGame`/`UpdateGame` signatures alone; add separate slot methods. Stop writing the old `umpire` text from forms; public pages read slots, falling back to the legacy text when a game has no slots.
- **Email:** decline notice goes to users with the `director` role on that tournament (not all admins). Mail failure or no mail config never blocks assignment; log it.
- **Image:** 5 MB cap on the request body, type chosen by sniffing bytes (JPG/PNG/PDF), served with `Content-Disposition: attachment` and `X-Content-Type-Options: nosniff`.

## Data model (suggested)

- `users`: add `is_umpire BOOL`, `cert_url TEXT`, `cert_image BYTEA NULL`, `cert_image_type TEXT`.
- `divisions`: add `umpires_needed INT NOT NULL DEFAULT 1`.
- `game_umpires`: `id`, `game_id → games`, `slot INT`, `user_id → users NULL`, `free_name TEXT`, `status` (`pending|accepted|declined`, free-form rows are `accepted`), unique `(game_id, slot)`, check exactly one of user_id/free_name.
- `umpire_verifications`: `(user_id, tournament_id)` unique, `verified_by`, `verified_at`.
- Add all new methods to the `mydb.DB` interface and `FakeDB` (project convention); migrations go in the existing auto-migrate list in `mydb/mydb.go`.

## Surfaces

- Profile page (new, logged-in users): umpire checkbox, cert URL, image upload.
- Umpire "my games" page (logged-in umpires): accept/decline actions, CSRF-protected like other forms.
- Division manage page: umpires-needed setting.
- Game create/edit forms (manage and admin): per-slot picker + free-form text; conflict errors.
- Director page listing tournament umpires with verify/unverify and credential access.
- Public games/division/team pages currently print `Umpire`; show the assigned names (no credentials).
- Roles: no new role; "umpire" is a user attribute, not a tournament role.

## Left to the builder

Route paths, template layout, exact email wording, migration SQL, and test
coverage (follow existing FakeDB-based tests, including the overlap rule and
decline-reopens-slot behavior).
