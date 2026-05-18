# Max Games Per Team Design

## Goal

Allow tournament directors to cap the number of pool-play games each team plays when auto-generating a schedule. The generator distributes games randomly so no team only plays teams clustered near them in the list. When the cap and team count make a perfectly equal distribution impossible, the one short team receives an extra game against a random at-cap opponent, but that game does not affect the over-cap team's standings.

## Tech Stack

Go, PostgreSQL, html/template, no new dependencies.

---

## Algorithm

When `max_games_per_team > 0`:

1. Shuffle the teams slice randomly (`math/rand`).
2. Generate all unique pairs from the shuffled teams (same nested-loop structure as today).
3. Shuffle the pairs slice randomly.
4. Walk the pairs: accept a pair if both teams have fewer than `max_games_per_team` games recorded so far; otherwise skip.
5. Schedule accepted games sequentially starting at `start_datetime`, incrementing by `minutes_between` per game.

When `max_games_per_team = 0` or blank, existing behavior is unchanged (full round-robin). The `round_robin_type` (single/double) field is ignored when `max_games_per_team > 0`.

### Odd-team handling

When `len(teams) × max_games_per_team` is odd, one team will end up with `cap − 1` games after the greedy pass. In that case:

1. Identify the one short team.
2. Pick a random at-cap opponent.
3. Schedule one extra game between them.
4. Call `SetGameScrimmage(gameID, overCapTeamID)` to mark the game as non-counting for the over-cap team.

---

## Data Model

### `games` table

```sql
ALTER TABLE games ADD COLUMN IF NOT EXISTS scrimmage_team_id INT NULL;
```

`scrimmage_team_id` is the ID of the team for whom this game does not count toward standings. NULL means the game counts normally for both teams.

### `Game` struct

```go
type Game struct {
    // existing fields ...
    ScrimmageTeamID int // 0 = counts for both teams
}
```

### `ScoreGame` change

`ScoreGame` already writes two rows to `GAMESBYTEAM` (one per team). With this change:

- If `game.ScrimmageTeamID == game.HomeTeam.ID`: skip the home team's `GAMESBYTEAM` write.
- If `game.ScrimmageTeamID == game.AwayTeam.ID`: skip the away team's `GAMESBYTEAM` write.

The scrimmage team's wins, losses, runs for, and runs against are unaffected by the game result.

---

## DB Interface Methods

New method on `mydb.DB`:

```go
SetGameScrimmage(gameID, teamID int)
```

Sets `scrimmage_team_id = teamID` for the given game.

---

## Schedule Display

Anywhere a game row is rendered (division games list, team schedule page, admin games list), if `game.ScrimmageTeamID` matches one of the teams in the row, show a small **"Non-counting"** label next to that team's name. No new routes or pages needed.

---

## UI Changes

Both `templates/admin/create_game.html` and `templates/manage/create_game.html` gain a new optional field in the "Auto-Generate Schedule" section:

- **Label:** Max Games Per Team
- **Input:** `<input type="number" name="max_games_per_team" min="1" placeholder="leave blank for full round-robin">`
- **Placement:** after the Round Robin Type field
- **Behavior:** when this field has a value > 0, the Round Robin Type selection is ignored server-side

---

## Files

| File | Change |
|------|--------|
| `mydb/mydb.go` | Add `scrimmage_team_id` migration |
| `mydb/games.go` | `ScrimmageTeamID int` on `Game`; update SELECT/scan; `ScoreGame` skips GAMESBYTEAM for scrimmage team; `SetGameScrimmage` |
| `mydb/db.go` | Add `SetGameScrimmage` to interface |
| `mydb/fakedb.go` | Implement `SetGameScrimmage`; update `ScoreGame` |
| `webhandler/games.go` | Read `max_games_per_team`; capped randomized algorithm; call `SetGameScrimmage` for odd-team game |
| `webhandler/manage_games.go` | Same changes as `games.go` |
| `webhandler/templates/admin/create_game.html` | Add `max_games_per_team` field |
| `webhandler/templates/manage/create_game.html` | Add `max_games_per_team` field |
| `webhandler/templates/games.html` | "Non-counting" label when `ScrimmageTeamID` matches |
| `webhandler/templates/team.html` | "Non-counting" label when `ScrimmageTeamID` matches |
| `webhandler/templates/admin/games.html` | "Non-counting" label when `ScrimmageTeamID` matches |
