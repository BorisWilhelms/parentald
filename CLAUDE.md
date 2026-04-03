# parentald

Linux parental control system. Two Go binaries, one repo.

## Architecture

- **`cmd/server/`** — Config server with HTMX web UI, serves daemon binaries and install script
- **`cmd/daemon/`** — Client daemon, enforces screen time rules, tracks app usage, self-updates

Communication: daemon polls server (`GET /api/config`, `POST /api/activity`, `GET /api/version`).

## Key packages

- `internal/config/` — Shared types (Config, User, Schedule), schedule evaluation (`IsAllowed`), JSON store with atomic writes
- `internal/activity/` — Process scanning (`/proc`), `.desktop` file parsing, process tree grouping, activity storage (per-day JSON)
- `internal/server/` — HTTP handlers, HMAC cookie auth, Pico CSS templates, embedded static assets
- `internal/denylist/` — Atomic read/write of PAM deny-users file
- `internal/update/` — Daemon self-update (download binary from server, replace, exit for systemd restart)

## Enforcement mechanism

- `pam_listfile` blocks login via `/etc/parentald/deny-users`
- `loginctl terminate-user` ends active sessions
- Priority: Lock > Bonus > No-schedules-means-allowed > Schedule

## Build

```
make build-all    # server + cross-compiled daemon binaries
make test         # go test ./internal/...
```

Version is git commit hash, injected via `-ldflags`.

Docker: `docker compose up` (needs `ADMIN_PASS` env var).

## Development

- Go 1.23+, stdlib only, no external dependencies
- Templates use `embed.FS` — compiled into binary
- Go 1.22+ `ServeMux` pattern routing (method + path)
- Templates: each page is self-contained HTML (no shared `{{define "content"}}` — that causes conflicts). Shared parts via `{{template "head"}}` and `{{template "nav"}}`.
- Pico CSS via CDN, HTMX via CDN
- All user-facing text in German with proper umlauts
