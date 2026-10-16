# In-app notifications and umpire availability — design

Implements GitHub issue #24 ("Allow umpires to see upcoming tournaments") and the notification system it needs.

## Intent

Last time a director built a tournament from a pool of known umpires, an umpire the
director didn't know wanted to work it and had no way to say so, and missed out.
Umpires need to see upcoming tournaments and raise their hand; directors need to be
told, in the app and by email. The app has no notification system today (only
fire-and-forget emails), so this builds one and moves the existing umpire emails onto it.

## Goals

- **Part 1 — Notifications.** A per-user inbox: short message, optional link, timestamp,
  read/unread. Nav bar shows an unread count. One helper sends the in-app notification
  and the email together.
- **Part 1 — Retrofit.** The existing umpire notices use it: assigned to a game, game
  changed (re-confirm), game cancelled, removed from a game, umpire declined (to directors).
- **Part 2 — Availability.** A registered umpire sees upcoming published tournaments and
  can mark "I'm available" (with an optional short note) and withdraw it.
- **Part 2 — Director view.** Volunteers appear on the tournament's manage-umpires page,
  marked "volunteered" with their note; directors get a notification (in-app + email)
  when someone volunteers. Volunteers are picked in the game forms exactly like any
  registered umpire.

## Non-goals

- Per-user notification settings / muting, digests, push or SMS.
- Notifications for non-umpire features (scores, news, invitations) — the helper makes them easy later.
- Free-form umpires (no account) get nothing.
- Travel-distance filtering of tournaments (separate TODO).
- Auto-assigning volunteers.

## Anti-goals

- Volunteering must not widen who can see credentials: certificates and the umpire
  directory stay visible only to the owner, admins, and directors of non-draft
  tournaments (existing rule, `canViewCertificate`).
- Draft tournaments never appear in the upcoming list.
- A user must never read or mark another user's notifications.

## Decisions (from the conversation)

- Notification = in-app **and** email, always both (no opt-out in v1).
- Existing umpire notices move onto the new system (confirmed).
- Directors are notified of volunteers; they also see them on the manage-umpires page.

## Builder's calls (mine, not confirmed — please check)

- **Upcoming** = tournaments with status not `draft` and `start_date` today or later, ordered by date. Listed on the umpire's `/umpire` page above their games. Only users flagged as umpire see the section.
- **Availability** is one row per (user, tournament) with an optional note (max 200 chars). Withdrawing deletes the row. No dates/field preferences.
- **Who is notified of a volunteer:** users with the `director` role on that tournament (same rule as the decline notice). Each (re)volunteer by the same umpire notifies once; editing only the note does not re-notify.
- **Inbox:** `/notifications` lists newest first (cap 100), with "mark all read"; opening one via its link marks it read (`GET /notifications/:id` marks read then redirects to the stored link, owner only).
- **Unread count** in the nav is loaded per request for logged-in users (one COUNT query).
- **Email keeps its current content/links**; the helper takes the same subject/message plus a link path (e.g. `/umpire`).
- **Retention:** keep everything; no pruning in v1.
- **Existing `notifyUmpire(email, …)`** is replaced by `notifyUser(userID, …)`; callers that only have an email-bearing slot use the slot's `UserID`.

## More builder's calls (from the review)

- **Stored text:** `notifications` has both `subject` and `message`; the inbox shows the subject in bold with the message under it. The helper is `notifyUser(userID int, subject, message, link string)`; the email uses the same subject/body. Existing tests that assert email content are updated.
- **Links:** assigned / changed / removed / cancelled → `/umpire`. Umpire declined (to directors) → `/tournaments/:tid/manage/games/:gid/edit`. Volunteer (to directors) → `/tournaments/:tid/manage/umpires`.
- **Recipients of director notices:** users with the `director` role on the tournament; **if none exist, fall back to site admins** so a volunteer is never silent.
- **Upcoming:** `status != 'draft'` (any non-draft status counts) and `start_date >= today - 7 days`, so a tournament that already started can still be volunteered for. There is no end date to be more precise.
- **Withdrawing** is always allowed, sends no notification, and does not touch existing slot assignments.
- **Who can volunteer:** any user flagged as umpire, including ones with an expired certification (directors see the flag).
- **Manage-umpires page:** volunteers are sorted to the top of the list with a "volunteered" badge and their note; everyone else stays listed below as today.
- **Notify once:** the upsert reports whether the row was new (`INSERT ... ON CONFLICT DO UPDATE ... RETURNING (xmax = 0)` in Postgres); only a new row notifies. Best-effort against double-clicks is acceptable.
- **Failure behavior:** the in-app row is written synchronously; the email goes out in the background and failures are only logged. A user without an email, or whose email fails, still gets the in-app row.

## Data model (suggested)

- `notifications(id, user_id → users, message TEXT, link TEXT, created_at TIMESTAMPTZ, read_at TIMESTAMPTZ NULL)`, index on `(user_id, read_at)`.
- `umpire_availability(user_id → users, tournament_id → tournaments, note TEXT, created_at, PRIMARY KEY(user_id, tournament_id))`.
- New methods go in `mydb.DB`, `*MyDB` and `*FakeDB` (project convention); schema in `pgtables`; idempotent.

## Surfaces

- Nav: "Notifications (N)" link for logged-in users.
- `/notifications`, `/notifications/:id`, `POST /notifications/read-all` (all CSRF-protected, owner-only).
- `/umpire`: new "Upcoming tournaments" section with Available / Withdraw and a note field.
- `/tournaments/:tid/manage/umpires`: volunteered badge + note.

## Left to the builder

Route paths beyond those above, template layout, message wording, test structure
(follow existing FakeDB-based tests; extend the opt-in Postgres test).
