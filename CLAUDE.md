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

## Activity tracking

**Process tree grouping**: The daemon builds a process tree per user via `/proc`. Only direct children of session roots (e.g., children of `systemd --user`) are reported as top-level apps. All descendants are collapsed into their ancestor. This reduces ~80 raw processes to ~10-15 actual apps.

**App identification**: `.desktop` files are parsed at daemon startup for `Name=`, `Exec=`, `Categories=`, `Icon=`. Lookup is by exe basename. Processes without a `.desktop` match are reported as "Sonstiges" (uncategorized).

**Flatpak**: Detected via `/proc/<pid>/cgroup` (`app-flatpak-<appid>` pattern). Desktop files searched in `/var/lib/flatpak/exports/share/` and `~/.local/share/flatpak/exports/share/`. `Exec=flatpak run ...` is parsed to extract the app ID.

**Categories**: Uses freedesktop.org main categories (Game, Network, Office, etc.). Toolkit markers (GNOME, GTK, KDE, Qt) are skipped — first main category wins.

**Icons**: Resolved from `/usr/share/icons/hicolor/` (48x48 preferred), flatpak exports, and `/usr/share/pixmaps/`. Encoded as base64 data URIs, max 32KB per icon. Sent with activity reports and stored in per-day JSON.

**Activity storage**: One JSON file per day in `<data-dir>/activity/`. Keyed by hostname → user → app name. Server aggregates across hosts for UI display.

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
