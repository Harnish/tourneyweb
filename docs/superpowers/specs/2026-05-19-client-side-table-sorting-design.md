# Client-Side Table Sorting — Design Spec

**Date:** 2026-05-19  
**Status:** Approved

---

## Goal

Allow users to click any column header to sort the table rows by that column. Applies to all data tables (public and admin/director views). No server round-trips, no new dependencies.

---

## Architecture

A single vanilla-JS file (`sort.js`) is embedded in the binary and served at `GET /sort.js`. The header template includes it globally. Tables opt in with `class="tw-sortable"`. Numeric columns carry `data-sort="number"` on their `<th>`; all other columns sort as text. Clicking a header sorts ascending; clicking again toggles descending. An arrow indicator (↑/↓) shows the active sort column and direction.

---

## Serving Infrastructure

**File:** `webhandler/sort.js`  
Embedded alongside `style.css` via `//go:embed sort.js`. New handler `PrintJS` at `GET /sort.js` writes the file with `Content-Type: application/javascript`. Route registered in `main.go` the same way `PrintCSS` is.

**Header template:** `webhandler/templates/header.html`  
Add `<script src="/sort.js" defer></script>` once, globally.

---

## JavaScript Design

```
makeSortable(table)
  - Walk all <th> in the <thead> row, add click listener + cursor:pointer style
  - On click:
    - Determine column index from th.cellIndex
    - Read data-sort attribute ("number" or absent → text)
    - Toggle direction if same column was clicked, else reset to ascending
    - Collect all <tr> rows from <tbody>
    - Sort rows by the text content of td at column index:
        number: parseFloat(content) || 0
        text: localeCompare, case-insensitive
    - Re-append sorted rows to <tbody>
    - Clear ↑/↓ from all headers, set on clicked header

DOMContentLoaded:
  - querySelectorAll('table.tw-sortable') → call makeSortable on each
```

No external libraries. ~50 lines total.

---

## Tables and Columns

### Public

| Template | Table | Numeric columns |
|---|---|---|
| `divisions.html` | Standings | Rank, Wins, Losses, RA, RF, GP |
| `divisions.html` | Games list | Home score, Away score |
| `games.html` | All games | Home score, Away score |
| `team.html` | Games | Home score, Away score |
| `team.html` | Roster | # (jersey number) |

### Admin / Director

| Template | Table | Numeric columns |
|---|---|---|
| `admin/games.html` | Games | Home score, Away score |
| `admin/division_view.html` | Teams | Wins, Losses |
| `admin/teams.html` | Teams | none (text sort only) |
| `manage/teams.html` | Teams | none (text sort only) |
| `manage/roster.html` | Roster | # (jersey number) |
| `admin/locations.html` | Locations | none (text sort only) |
| `admin/queue.html` | Queue | none (text sort only) |

Form tables (create-game, edit-game, location-edit, etc.) are layout tables, not data tables — they are skipped.

---

## Changes Required

1. **New file** `webhandler/sort.js` — vanilla JS sort utility (~50 lines)
2. **Modify** `webhandler/webhandler.go` — add `//go:embed sort.js` and `PrintJS` handler
3. **Modify** `main.go` — register `GET /sort.js` route
4. **Modify** `webhandler/templates/header.html` — add `<script src="/sort.js" defer></script>`
5. **Modify** public templates (5 tables): add `tw-sortable` and `data-sort="number"` attributes
6. **Modify** admin/director templates (7 tables): add `tw-sortable` and `data-sort="number"` attributes

---

## Non-Goals

- Server-side sorting (no query changes)
- Pagination (out of scope)
- Persisting sort preference across page loads
- Sorting tables in form/edit pages
