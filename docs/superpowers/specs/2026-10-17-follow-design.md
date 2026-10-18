# Follow tournaments, levels and teams — design

Implements GitHub issue #29 ("Let users follow tournaments, levels and teams").

## Intent

Parents and fans want to know how their kid's level is doing without refreshing the
games page. A registered user follows a level (division) or a team and gets an alert,
with the score, when a game there is complete. Following also tells directors who is
invested: when filling umpire or extra-role slots, people who follow that tournament or
level are listed first.

## Goals

- **Follow targets:** a tournament, a level (division), or a team.
- **Score alert:** a follower of a level, or of a team in a game, is notified when that game's score becomes official, with the final score. Delivered through the existing `notifyUser` (in-app inbox + email).
- **One alert per game per person:** following both a level and a team in it still sends one alert.
- **Tournament follow means interest only:** no score alerts, so it never floods. It only feeds the ranking below.
- **Discovery:** the tournament list (`/`) gets a name search box and a Follow button per row; tournament home, division and team pages get a Follow / Following toggle. Logged-out visitors are sent to log in first.
- **"Following" page** (linked from the profile) lists everything the user follows with Unfollow buttons. Alert emails include an unfollow link.
- **Ranking:** in the director/staff user search used to fill umpire and extra-role slots, users following that game's tournament or its level come first; everyone else follows, alphabetical within each group.

## Non-goals

- Browser/phone push notifications (inbox + email is enough for now; later ticket).
- Alerts for schedule changes, news, or anything other than completed-game scores.
- Alerts for a tournament follow.
- Follower counts or follower names shown publicly or to directors beyond the ordering.
- Ranking followers in any list other than the slot user search.
- Following leagues (separate ticket #21).

## Anti-goals

- A user must never see or alter another user's follows.
- An alert must never leak anything beyond what the public games page already shows (teams, level, final score).
- Following something must never grant access to a draft tournament or its games; draft tournaments cannot be followed or searched publicly, and no alert goes out for one.
- A wrong or unconfirmed score must not be pushed to followers: a coach's unconfirmed report and a disputed game send nothing.
- Unfollow must be possible for anyone who received an alert, even logged out of the site (see builder's calls).

## Decisions (from the conversation)

- Alert when a game in a followed level is complete, with what the score was.
- Inbox + email is enough; push later.
- Level follow: alerts for every completed game in the level. Team follow: alerts only for that team's games. Tournament follow: ranking only. Both level and team → one alert.
- Ranking applies to the slot user search only.
- Follow buttons on the tournament list, tournament home, division and team pages; a Following page from the profile; unfollow link in alert emails; no public follower counts.

## Builder's calls (mine, not confirmed — please check)

- **"Complete" means official:** the alert fires when a game's `score_status` becomes `confirmed` or `final` (see #18), or for games scored before that field mattered, when staff/director/umpire enters a score. It does not fire on `reported` (one coach) or `disputed`.
- **Corrections:** if an official score is later changed to a different score, followers get an alert titled "Game score corrected". An edit that leaves the score unchanged sends nothing.
- **Subject/body:** subject "Final score" (or "Game score corrected"); body `gameLine(g)` + `Home N, Away M`; link to the public games page of the tournament.
- **Tables:** `follows(user_id → users ON DELETE CASCADE, kind TEXT CHECK in ('tournament','division','team'), target_id INT, created_at)` primary key `(user_id, kind, target_id)`. Targets removed with their tournament/division/team (the deletes clean up, as roles do); no FK because the target table varies by kind.
- **Who gets alerts:** followers of the game's division or of either team, de-duplicated, minus the person who entered the score, minus users who muted follower alerts. This is a new, separate setting (`mute_follow_alerts`, second checkbox on the profile) so a director who mutes director notices still gets alerts for what they follow (confirmed by the user). Registered users with an unverified/unusable email still get the inbox row.
- **Draft tournaments:** cannot be followed; hidden from the search; a tournament moving out of `published` keeps its follows but sends no alerts.
- **Search:** `GET /?q=` filters the tournament list by case-insensitive name substring; only tournaments already visible on the list.
- **Routes:** `POST /follow` (form: `kind`, `target`, `next`) and `POST /unfollow` (login required, CSRF, own follows only, `next` limited to same-site paths); `GET /following`; `GET /unfollow?t=<token>` for email links.
- **Email unfollow link:** a signed, unguessable token (HMAC over user id + kind + target with a server secret) so the link works without logging in; GET shows a confirm button and POST performs it (mail scanners prefetch GETs).
- **Ranking:** `SearchUsers` stays name-only and ≥2 chars; the handler asks for followers of the game's tournament and division and sorts them first. For the create-game form (no game yet) the division is known from the URL. The JSON shape `{id,name}` is unchanged.
- **Cap:** a user may follow at most 50 targets, so the Following page and fan-out stay bounded.

## More builder's calls (from the review)

- **"Becomes official" rule:** define the official result of a game as `(score_status in {confirmed, final}, home, away)`. Every score path reads the game before the write and calls `notifyFollowers` only when the official result changes: unofficial → official ("Final score"), or official → a different official score ("Game score corrected"). Staff entry on a `reported` or `disputed` game goes to `final` and alerts. A staff re-save with the same score sends nothing, which also stops double-firing. `UpdateGame` clearing reports on a team change makes the game unofficial (no alert); scoring it again alerts again.
- **Who is excluded as "the scorer":** the user whose action made the result official: the second coach on a coach confirmation, or the staff/director/umpire who entered it. The first coach, if they follow, still gets the alert.
- **Ranking with the search limit:** add a DB method that returns the users matching the name query who follow the game's tournament or division (same 2-char minimum, wildcard escaping and limit). The handler lists those first, then fills the rest from `SearchUsers`, de-duplicated, up to 10.
- **Unfollow-token secret:** new optional config key `secret`; if unset, derive from `adminpassword`. Tokens stay valid until the secret changes. Acceptable ceiling: rotating the secret kills old email links.
- **Cap and orphans:** the 50-follow cap counts rows as stored; delete paths for tournaments, divisions and teams remove their follows.
- **Duplicate follow** is a silent no-op; following a draft tournament, or a division/team in one, or a nonexistent target, returns 404.
- **Tournament-list search** applies on top of the existing visibility and ordering; plain case-insensitive name substring, no pagination, "No tournaments match" when empty.

## Data flow

User follows level/team → game becomes official (coach agreement, umpire, or staff score) → `notifyFollowers(game)` finds followers, drops the scorer and muted users, sends one alert each → inbox row + email with unfollow link.

## Surfaces

- Tournament list: search box, Follow buttons.
- Tournament home / division / team pages: Follow toggle.
- Profile → "Following" page.
- Alert in the notifications inbox and by email.
- Slot user search ordering (no UI change).

## Left to the builder

Template markup and wording, token format, test structure (FakeDB tests plus the opt-in Postgres test), file names, how `notifyFollowers` hooks the existing score paths (`RecordScore`, coach confirm, umpire entry).
