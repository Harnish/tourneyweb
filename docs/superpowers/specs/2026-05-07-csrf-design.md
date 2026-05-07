# CSRF Protection Design

**Date:** 2026-05-07  
**Status:** Approved

## Problem

All admin POST forms (add/delete division, team, game; score game; login) have no CSRF protection. An attacker can trick an authenticated admin into submitting a malicious form.

## Approach

Wrap the httprouter with `gorilla/csrf` middleware. One integration point protects all routes. Token is injected into each form as a hidden field.

## Changes

### 1. Dependency

Add `github.com/gorilla/csrf` to `go.mod`.

### 2. Config (`config.go`)

Add `CSRFKey string` to the `Config` struct. The value is a 32-character hex string set in `tourneyweb.conf` (and documented in `tourneyweb.conf.example`). At startup, decode the hex string to 32 bytes. If the key is absent or wrong length, panic with a descriptive message.

### 3. Middleware wiring (`main.go`)

Current:
```go
log.Fatal(http.ListenAndServe(":"+cfg.Port, wh.RequestLogger(router)))
```

New:
```go
csrfKey := decodeCSRFKey(cfg.CSRFKey)
csrfMiddleware := csrf.Protect(csrfKey, csrf.Secure(false))
log.Fatal(http.ListenAndServe(":"+cfg.Port, wh.RequestLogger(csrfMiddleware(router))))
```

Order matters: `RequestLogger` is outermost so all requests (including rejected ones) are logged. `csrfMiddleware` is inner, so it validates the token after logging but before the route handler runs. `csrf.Secure(false)` is required because the app runs over HTTP. The middleware returns HTTP 403 for any POST missing a valid token.

### 4. Form updates

Add the following hidden field to every HTML form, using `csrf.Token(r)` which is available in all handler methods that have `*http.Request`:

```html
<input type="hidden" name="gorilla.csrf.Token" value="TOKEN">
```

Forms to update (file → handler):

| File | Handler | Form action |
|------|---------|-------------|
| `webhandler/webhandler.go` | `LoginForm` | `POST /login` |
| `webhandler/webhandler.go` | `CreateGame` | `POST /admin/addgame` |
| `webhandler/webhandler.go` | `CreateGameSubmit` | `POST /admin/addgame` (re-renders same form) |
| `webhandler/webhandler.go` | `ScoreGame` | `POST /admin/scoregamepost` |
| `webhandler/teams.go` | `Teams` | `POST /admin/addteam` and inline delete form |
| `webhandler/divisions.go` | `AddDivisionForm` | `POST /admin/adddivision` and `POST /admin/deldivision` |

### 5. Known gap (out of scope)

`GET /admin/delgame/:gameid` triggers a delete — GET requests with side effects can't carry CSRF tokens. This is tracked separately in TODO as part of "add delete confirmation" (convert delete links to POST forms).

## Out of Scope

- Switching to `html/template` (separate TODO item)
- Converting delete-by-GET links to POST forms (separate TODO item)
- HTTPS enforcement (deployment concern)
