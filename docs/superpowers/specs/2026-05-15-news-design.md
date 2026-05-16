# News Feature Design

**Date:** 2026-05-15  
**Status:** Approved

## Overview

Two-tier news system: site-level news managed by admins (shown on the main landing page) and tournament-level news managed by directors and staff (shown on the public tournament page). Both use a Quill rich-text editor and display a published timestamp with the author's name.

---

## Data Layer

### Schema

Single table `event_news`, extended via migrations:

```sql
-- Make tournament_id nullable (NULL = site news)
ALTER TABLE event_news ALTER COLUMN tournament_id DROP NOT NULL;

-- Add timestamp and author
ALTER TABLE event_news ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE event_news ADD COLUMN IF NOT EXISTS author_id INTEGER REFERENCES users(id) ON DELETE SET NULL;
```

`tournament_id IS NULL` → site news  
`tournament_id IS NOT NULL` → tournament news

### Struct

```go
type NewsItem struct {
    ID           int
    TournamentID int       // 0 = site news
    Title        string
    Body         string    // HTML from Quill, rendered via htmlSafe
    CreatedAt    time.Time
    AuthorID     int
    AuthorName   string    // joined from users
}
```

### Methods

| Method | Signature |
|--------|-----------|
| Add | `AddNews(tournamentID int, title, body string, authorID int) int` — pass 0 for site news |
| Site list | `GetSiteNews() []NewsItem` — newest first |
| Tournament list | `GetTournamentNews(tournamentID int) []NewsItem` — newest first |
| By ID | `GetNewsByID(id int) (NewsItem, bool)` |
| Update | `UpdateNews(id int, title, body string)` |
| Delete | `DeleteNews(id int)` |

---

## Routes and Permissions

### Site News (admin only)

| Method | Path | Action |
|--------|------|--------|
| GET | `/admin/news` | List all site news + Quill add form |
| POST | `/admin/news` | Create site news item |
| GET | `/admin/news/:nid/edit` | Quill edit form |
| POST | `/admin/news/:nid/edit` | Update site news item |
| POST | `/admin/news/:nid/delete` | Delete site news item |

Auth guard: `IsAdmin` middleware (same as all `/admin/*` routes).

### Tournament News (create: director or staff; edit/delete: director only)

| Method | Path | Action |
|--------|------|--------|
| GET | `/tournaments/:tid/manage/news` | List tournament news + Quill add form |
| POST | `/tournaments/:tid/manage/news` | Create tournament news item |
| GET | `/tournaments/:tid/manage/news/:nid/edit` | Quill edit form |
| POST | `/tournaments/:tid/manage/news/:nid/edit` | Update (director guard) |
| POST | `/tournaments/:tid/manage/news/:nid/delete` | Delete (director guard) |

Auth guards: list/create use the existing `manage` middleware (director or staff). Edit/delete check `IsDirectorFor(tid)` explicitly and return 403 if staff-only.

Cross-tenant guard: verify the news item's `tournament_id` matches `:tid` before edit/delete.

---

## Public Display

### Landing page (`/`)

A "News" section is added below the tournament list. Shows the 10 most recent site news items, newest first. Each item:

```
[Title]                           Jan 2, 2006 3:04 PM — Author Name
[Quill HTML body]
<hr>
```

If no site news exists, the section is omitted.

### Tournament page (`/tournaments/:tid`)

A "News" section is added below the division list. Shows all news for that tournament, newest first. Same item format as above. If no tournament news, the section is omitted.

---

## Templates

| Template | Purpose |
|----------|---------|
| `admin/news.html` | Site news list (newest first, with edit/delete buttons) + Quill add form |
| `admin/news_edit.html` | Quill edit form for a single site news item |
| `manage/news.html` | Tournament news list + Quill add form (director/staff); edit/delete buttons shown only when `User.IsDirectorFor(tid)` — not controlled by `DisableDelete` |
| `manage/news_edit.html` | Quill edit form for a single tournament news item (director only) |

Quill setup identical to `edit_extras.html`: load Quill CSS/JS from CDN, initialize editor with toolbar (headers, bold, italic, underline, link, lists), pre-fill from existing content, copy to hidden input on submit.

Public rendering uses the existing `htmlSafe` FuncMap function.

---

## Navigation Links

- Admin sidebar (`layout.html` or `admin/news.html` header): add link to `/admin/news`
- Manage dashboard (`manage/dashboard.html`): add "News" link to `/tournaments/:tid/manage/news`
- Tournament admin view (`admin/tournament_view.html`): add "Manage News" link

---

## Data Structs (templates.go)

```go
type newsListData struct {
    baseData
    Tournament    mydb.Tournament  // zero value for site news
    News          []mydb.NewsItem
    DisableDelete bool
}

type newsEditData struct {
    baseData
    Tournament mydb.Tournament  // zero value for site news
    Item       mydb.NewsItem
}
```

---

## Error Handling

- News item not found → 404 via `renderError`
- Tournament mismatch (item belongs to different tournament) → 404
- Staff attempting director-only edit/delete → 403 via `renderError`

---

## Out of Scope

- Pagination (show all for tournament news, cap at 10 for site news)
- News categories or tags
- Draft/unpublished state
- Comment threads
