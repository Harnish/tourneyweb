# Leagues — design

Implements GitHub issue #21 ("Add league support": "Add the ability to make a league. The league may or may not work into a tournament.").

## Intent

Some organizers run a season, not a weekend: the same teams play a schedule over weeks, with standings, and
sometimes a playoff tournament at the end. A league is that season. Games are scheduled up front and move
over time with weather and field availability, so everyone affected must hear about changes. A league's
teams are fixed once the season starts. When it ends, the director can spin up a playoff tournament from
the standings.

## Goals

- **A league is a long-running tournament.** It has levels (divisions), teams, games, standings, umpire and extra-role slots, coach/umpire score entry, notifications and following, all with the existing code and screens. Create one from the same form as a tournament by choosing a type (Tournament / League).
- **Fixed rosters:** once the league's start date has passed, adding teams is blocked on every path (manage and legacy admin). Editing a team's name or coach stays allowed.
- **Games scheduled up front, rescheduled later:** existing game create/edit; changing a game's time or field is the "reschedule".
- **Reschedule notices:** when a league game's start time or location changes, both teams' coaches and followers of either team or of that level get an inbox + email notice with old and new time and field. Umpires and extra-role holders keep getting the existing re-confirm notices. Followers' notices honour the follower-alert mute and carry the unfollow link.
- **Create playoff tournament from the league:** a button on the league's manage page creates a new tournament linked to the league. It copies the league's levels and teams, ordered by the league standings of each level, with the league's name in the new tournament's name. A league can have any number of playoff tournaments (for example one per level).
- **Visibility:** leagues appear in their own "Leagues" section on the home page and carry a "League" badge on their page. The home-page name search covers tournaments and leagues. Anyone can follow a league like a tournament.
- **Season dates:** league has a start date and an optional end date, both shown on the page.

## Non-goals

- Multiple seasons or years under one league, registration, fees, waitlists.
- Recurring or auto-generated weekly schedules.
- Promotion/relegation, cross-league standings, player or team statistics across seasons.
- Copying games, scores, umpire/role assignments, coach or staff accounts, or news into the playoff tournament.
- Changing a tournament into a league (or back) after creation.
- Follower-based features for leagues beyond what tournament follows already do.

## Anti-goals

- A team must never be added to a started league by any route (manage form, legacy admin form, playoff copy into the league, bulk import if any), except by the explicit director override described below.
- Directors/staff of one league or tournament must never read, copy or change another's data; the playoff button works only for that league's own directors.
- Reschedule notices must not reach people who aren't coaches of those teams or followers of those teams/level, must not go out when nothing about time or field changed, and must not leak anything the public games page doesn't already show.
- A draft (unapproved) league must not be listed, searched, followed or alert anyone, same as draft tournaments.
- Copying into a playoff must not carry over scores, standings, or any user's role.

## Decisions (from the conversation)

- Leagues schedule and score games like tournaments do.
- Teams cannot join mid-season.
- Games are scheduled up front but can be rescheduled over time (weather, field availability).
- Reschedule notifies both coaches and followers of either team or the level (plus the existing umpire/role notices).
- "Create from league": a button that copies the league's levels and teams, in standings order, into a new linked tournament; a league can have several.
- A league is stored as a tournament marked as a league, so permissions, screens and features carry over.
- Own "Leagues" section on the home page, a "League" badge, searchable, followable.
- Creation uses the tournament form plus a type choice; the type can't be changed later.
- Season = start date plus optional end date.

## Builder's calls (mine, not confirmed — please check)

- **Storage:** add `kind TEXT NOT NULL DEFAULT 'tournament'` (`tournament` | `league`), `end_date DATE NULL` and `league_id INTEGER NULL REFERENCES tournaments(id)` to `tournaments`. A playoff tournament has `kind='tournament'` and `league_id` set. Existing rows migrate as tournaments.
- **"Started" means** `start_date <= today` (server date). A league director or site admin can still add a team to a started league through an explicit "Add late team" confirmation on the manage page (so an honest mistake in setup is fixable); the plain add form refuses with a message. The legacy admin team form has no override and refuses.
- **Lists:** `ReturnTournaments*` and the coming-up/recent/future/past lists continue to hold tournaments only; a new "Leagues" section lists published leagues that are running or upcoming (end date not past, or no end date), newest start first. Name search returns both with a badge.
- **Reschedule notice trigger:** reuse the existing `notifyGameChanged` call sites (manage edit, legacy admin edit); fire only when `Start` or `Location` changed and the tournament is a league. Recipients: coach users of the home and away team (`coach` role with that team), followers of those teams or the level (via `GameFollowers`), de-duplicated, minus the editor. Subject "Game rescheduled"; body `gameLine(old)` → `gameLine(new)`. Follower mute applies to the follower-only recipients; coaches always get it.
- **Playoff copy:** new tournament is created as `draft` (needs the usual admin approval) unless the creator is an admin, names it "<League name> Playoffs" (editable immediately), copies the league's sport, location and ranking criteria, each division (name, ranking criteria), and each team (name, coach text) in standings order within its division; the creator becomes its director. It sets `league_id`. No bracket is created automatically. The league manage page lists its playoff tournaments.
- **Who may create a playoff:** a director of the league or a site admin.
- **Standings order for the copy:** use the same ranking the division page shows (`SortTeams` with the division's criteria).
- **Following a league** uses `kind='tournament'` follows with the league's id; no schema change.
- **Counts of teams/levels in the copy** are not capped beyond existing limits.

## More builder's calls (from the review)

- **Coaches:** a team's coach recipients are users holding a `coach` role for that team. A team that has only free-text coach name and no linked user gets no personal notice (followers still do). The playoff copy carries the coach text only.
- **Lists and search:** playoff tournaments are ordinary tournaments, so they appear in the normal lists and search once published, with a "Playoffs of <League>" line; a draft playoff stays hidden like any draft. Only the new Leagues section filters on `kind='league'`; the existing tournament lists add `kind='tournament'`.
- **"Started":** `start_date <= today` using the server's date; the start day itself counts as started. The app has no tournament timezone today.
- **Late-team override:** a second button "Add late team" with a confirm prompt, available to the league's own directors and site admins only (not staff), logged with `slog`. The legacy admin form has no override.
- **Creation rule:** a league's start date must be today or later at creation, and the end date, if given, must be on or after the start date. An admin editing the start date afterwards is not restricted.
- **"Changed":** exact comparison of `Start` (as an instant) and `Location` (string). Moving a scored game still notifies. Deleting or cancelling a game sends no new coach/follower notice (umpires keep the existing "Game cancelled"); cancellation notices for coaches/followers are a possible follow-up.
- **Overlap of notices:** someone who is a follower and an umpire or role holder gets both the follower/coach notice and the umpire re-confirm notice; they are different asks.
- **Leagues section:** lists published leagues whose end date is today or later, or with no end date and a start within the last 12 months; older leagues are reached through search. A "Past leagues" view is not built.
- **Playoff directors:** the creating user becomes director of the new playoff, and the league's directors are copied in as directors too, so the league's staff keep access when an admin creates it. League staff and coach roles are not copied.
- **Standings order for the copy:** the division's own ranking; teams that tie, or divisions with no scored games, keep alphabetical order by team name.

## Data flow

Create league (type = League, start/end dates) → director builds levels and teams before the start date → schedules all games → season starts (team adds locked) → games are rescheduled over time (notices to coaches/followers/umpires/role holders) and scored (existing paths, follower alerts) → director clicks "Create playoff tournament" → new linked draft tournament with levels/teams in standings order → director schedules or brackets it as usual.

## Surfaces

- Create-tournament form: type choice and end date.
- Home page: "Leagues" section; name search covers both; "League" badge.
- League page: badge, season dates, link(s) to playoff tournaments; playoff tournament page: "Playoffs of <League>" link back.
- League manage page: "Create playoff tournament" button, "Add late team" override, list of playoffs.
- Notifications inbox/email: "Game rescheduled".

## Left to the builder

Template markup and wording, file and function names, bulk-add helpers (none required), test structure (FakeDB tests plus the opt-in Postgres test), where the late-team override lives in the UI.
