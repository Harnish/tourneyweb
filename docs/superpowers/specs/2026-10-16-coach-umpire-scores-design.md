# Coach and umpire score entry — design

Implements GitHub issue #18 ("Allow the umpire to enter the score for the game").

## Intent

Today only directors and staff can enter scores, so someone from management has to be
at the field or chase the result. In practice the winning coach usually reports the
score. Coaches should be able to enter it themselves; when the two coaches disagree
the umpire, who actually saw the game, settles it; and management can still fix typos.

## Goals

- A coach can enter the score for a game their team played.
- If both coaches enter the same score, it is confirmed. If they differ, the game is
  flagged **disputed** and the umpire(s) assigned to the game are notified to enter the final score.
- An umpire assigned to the game can enter the score at any time; it is final.
- Director/staff (existing score form) can overwrite any score at any time; it is final and clears a dispute.
- Standings use the first coach's score until a final score exists.

## Non-goals

- Fouls, innings or box scores; only the final run totals (0–40 each, as today).
- Locking games after they are final (staff can always fix a typo).
- Coaches entering scores for games their team did not play.
- Anything about free-form umpires: they have no account, so they cannot enter scores.

## Anti-goals

- A coach must never be able to enter or change a score for another team's game, or once a final score exists.
- An umpire must never score a game they are not assigned to (declined slots do not count).
- A user must never score a game belonging to a different tournament than the URL's (the existing score routes were fixed for this in #25; the new routes must hold the same line).

## Decisions (from the conversation)

- Either coach can go first; normally the winning coach does.
- Same score from both coaches → confirmed. Different → disputed → umpire notified → umpire enters the final score.
- Director or staff can fix the score in case of a typo.
- **While disputed, standings show the first coach's score** (confirmed by the user).

## Builder's calls (mine, not confirmed — please check)

- **Status** is stored per game in `games.score_status`: `''` (unscored), `reported` (one coach entered), `confirmed` (both coaches agree), `disputed` (coaches differ), `final` (umpire, staff or director entered). Migration: existing scored games become `final`.
- **Reports table** `game_score_reports(game_id, reporter TEXT, user_id, home_score, away_score, reported_at)`, `reporter` ∈ `coach_home`, `coach_away`; unique `(game_id, reporter)`. Umpire/staff scores are not reports; they write the score directly with status `final`.
- **A coach may re-submit** their own report until a final score exists; the dispute state is recomputed (same → confirmed, different → disputed). Notifying the umpire happens only on the transition into `disputed`.
- **Standings** follow the **first reporter's current value** (the report with the earliest `reported_at`; if they edit it, their new value) via the existing `ScoreGame`, until `final`. A final score overwrites it.
- **Eligibility:** coach = a `coach` role whose `TeamID` equals the game's home or away team in that tournament; a team with several coach users: any of them can report for the team (the team slot is shared, last report from the team wins).
- **Other team's coach** gets a notification when the first report comes in ("score reported — confirm or correct") with a one-click Confirm that submits the same score.
- **Umpire resolution:** the disputed notice goes to the registered umpires on the game's non-declined slots; if there are none, to `directorRecipients` (directors, else admins). Link: the umpire score form.
- **Routes:** coach `GET/POST /tournaments/:tid/coach/games/:gid/score`; umpire `GET/POST /tournaments/:tid/umpire/games/:gid/score`; staff/director keep `/tournaments/:tid/score/games/:gid`. Coach/umpire routes are outside `/manage` and `/score`, so authorization is checked in the handlers.
- **Notices to directors** ("Game scored") keep working: sent whenever standings change (first report, edits, final). They already skip the person who entered it and honour the mute.
- **Bracket games:** reuse whatever `RecordScore` does today (`AdvanceBracket` on a non-tie); the builder should check how re-scoring an already-advanced bracket game behaves and not make it worse. A tie reported by a coach in a bracket game is rejected.
- **UI:** the public games page shows a small "disputed" badge next to a disputed score, and a "Score" link to coaches (their team's games) and umpires (their games) where staff already see it. Score forms use the existing 0–40 dropdowns and are validated server-side (0–40).
- **Notifications** use the existing `notifyUser` (inbox + email); free-form umpires get nothing.

## More builder's calls (from the review)

- **Bracket games advance only on `confirmed` or `final`.** A coach's unconfirmed report updates standings but does not move a team forward, because bracket advancement is not known to be reversible. Pool-play standings still follow the first coach's score.
- **`confirmed` is terminal for coaches.** Once both coaches agree they can no longer change the score; only an umpire, staff or a director can (staff and directors at any time).
- **Umpire entry** is allowed on games that are unscored, `reported` or `disputed`. It is refused on `confirmed` and `final` games (a staff/director fix is the way to change those), so an umpire can't silently replace an agreed result or a staff correction.
- **Slot ↔ user link:** an umpire slot has a nullable `user_id` (existing `game_umpires.user_id`); "assigned to the game" means a slot with that `user_id` and status not `declined`.
- **Timestamps:** `reported_at` is set once and never changes (it defines the first reporter); a separate `updated_at` records edits. `user_id` on a report is the latest coach user who submitted for that team; the "skip the entrant" rule and the Confirm notice refer to that user.
- **Notification matrix:** score reported → the other team's coaches; disputed → registered umpires on the game, else directors, else admins; standings change (first report, an edit that changes the first reporter's value, final) → the existing game-scored notice to directors/admins. A plain confirmation, or a second coach's differing report that doesn't change standings, sends no director notice.

## Data model (suggested)

- `games`: add `score_status TEXT NOT NULL DEFAULT ''`; migration sets `final` for already-scored games.
- `game_score_reports`: as above, `PRIMARY KEY(game_id, reporter)`, game FK `ON DELETE CASCADE`.
- New methods in `mydb.DB`, `*MyDB`, `*FakeDB` (project convention); `Game` gains `ScoreStatus string`.

## Surfaces

- Coach and umpire score forms (above); "Score" links on the public games page.
- "disputed" badge on the games page; nothing else changes for staff.
- Notifications: score reported (to the other team's coaches), score disputed (to umpires), plus the existing game-scored notice.

## Left to the builder

Route handler names, template layout, message wording, test structure (FakeDB tests plus
the opt-in Postgres test).
