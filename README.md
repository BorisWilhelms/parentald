# parentald

A self-hosted parental control system for Linux. Manage screen time schedules, track app usage, and instantly lock/unlock user sessions — all from a web UI or Home Assistant.

> **Transparency note:** This project was vibe-coded in a single session with [Claude Code](https://claude.ai/code). The entire codebase — architecture, implementation, and this README — was developed collaboratively between a human and an AI.

## Features

- **Screen time schedules** — Define allowed time windows per user, per day of the week
- **Instant lock/unlock** — Immediately terminate a user's session and block login until the next schedule
- **Bonus time** — Grant extra minutes on the fly, stackable
- **App activity tracking** — See which applications each user runs, with icons and categories from freedesktop `.desktop` files
- **Process tree grouping** — Child processes are collapsed into their parent app (e.g., Steam's sub-processes show as one entry)
- **Flatpak support** — Flatpak apps detected via cgroup, with proper names and icons
- **Self-updating daemon** — Clients auto-update from the server with Ed25519 signature verification
- **Ed25519 signed communication** — Config responses and activity reports are cryptographically signed
- **Config caching** — Daemon enforces rules even when the server is unreachable (laptop away from home)
- **PWA** — Installable as a mobile app
- **Dark mode** — Automatic or manual toggle
- **Multi-language** — English and German
- **Home Assistant integration** — REST API with API key auth for sensors and automations

## Architecture

```
┌─────────────────┐         ┌──────────────────┐
│  parentald-server│◄────────│  Web Browser/App │
│  (Docker/host)  │         │  (Admin UI)      │
│                 │         └──────────────────┘
│  - Config mgmt  │
│  - Activity DB  │         ┌──────────────────┐
│  - HTMX Web UI  │◄────────│  Home Assistant   │
│  - Binary dist  │         │  (REST sensors)  │
└────────┬────────┘         └──────────────────┘
         │ HTTPS (signed)
    ┌────┴────┐
    │         │
┌───▼──┐  ┌──▼───┐
│Client│  │Client│    parentald daemon (systemd)
│  1   │  │  2   │    - Polls config every 60s
│      │  │      │    - Enforces schedules via PAM + loginctl
│      │  │      │    - Tracks app usage via /proc
└──────┘  └──────┘    - Reports activity to server
```

## Quick Start

### Server (Docker)

```bash
# Create .env file
echo "ADMIN_PASS=your-secure-password" > .env
echo "API_KEY=$(openssl rand -hex 32)" >> .env

# Start server
docker compose up -d
```

The server is now running on port 8080. Open `http://your-server:8080` to access the web UI.

### Client Installation

On each Linux machine you want to manage:

```bash
curl -sSf https://your-server:8080/install | sudo bash
```

This will:
1. Download the daemon binary for your platform
2. Save the server's public key for signature verification
3. Install a systemd service
4. Configure PAM to block denied users
5. Start the daemon

The daemon generates its own Ed25519 keypair on first start and auto-registers with the server.

## Configuration

### Server flags / environment variables

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--admin-pass` | `ADMIN_PASS` | (required) | Admin password for web UI |
| `--admin-user` | `ADMIN_USER` | `admin` | Admin username |
| `--listen` | `LISTEN` | `:8080` | Listen address |
| `--config` | `CONFIG_PATH` | `config.json` | Config file path |
| `--data-dir` | `DATA_DIR` | `data` | Data directory (activity, keys) |
| `--bin-dir` | `BIN_DIR` | `dist` | Daemon binaries directory |
| `--api-key` | `API_KEY` | (optional) | API key for Home Assistant |

### Daemon flags

| Flag | Default | Description |
|------|---------|-------------|
| `--server` | `http://localhost:8080` | Server URL |
| `--interval` | `60s` | Poll interval |
| `--deny-file` | `/etc/parentald/deny-users` | PAM deny file path |
| `--key-dir` | `/etc/parentald` | Directory for keys |

## How It Works

### Enforcement

1. The daemon polls the server for the config every 60 seconds
2. For each configured user, it checks if the current time falls within an allowed schedule
3. Users outside their schedule are added to `/etc/parentald/deny-users`
4. Active sessions are terminated via `loginctl terminate-user`
5. PAM's `pam_listfile` module blocks new logins for denied users

**Priority:** Instant Lock > Bonus Time > No schedules (unrestricted) > Schedule check

### Activity Tracking

1. The daemon builds a process tree per user via `/proc`
2. Only top-level apps are reported (children collapsed into parent)
3. Apps are identified via `.desktop` files (name, category, icon)
4. Flatpak apps detected via `/proc/<pid>/cgroup`
5. Idle/locked sessions (via `loginctl`) are not counted
6. Activity is stored server-side as one JSON file per day

### Security

- Server and daemon communicate via Ed25519 signed messages
- Config responses are signed by the server — daemon verifies before applying
- Activity reports are signed by the daemon — server verifies the registered client
- Binary updates are signed — daemon verifies before replacing itself
- If signature verification fails, the daemon uses its local config cache and skips activity reporting

## Home Assistant Integration

Set `API_KEY` on the server, then configure HA:

```yaml
# configuration.yaml
rest:
  - resource: "http://your-server:8080/api/status"
    headers:
      "X-API-Key": "your-api-key"
    scan_interval: 30
    sensor:
      - name: "Child Status"
        value_template: "{{ value_json.username.status | default('offline') }}"

rest_command:
  parentald_lock:
    url: "http://your-server:8080/users/{{ user }}/lock"
    method: POST
    headers:
      "X-API-Key": "your-api-key"
```

See [examples/homeassistant.yaml](examples/homeassistant.yaml) for a complete configuration.

## Development

```bash
# Build
make build-all    # Server + cross-compiled daemon binaries
make test         # Run tests

# Run locally
go run ./cmd/server/ --admin-pass dev
```

**Tech stack:** Go (stdlib only, no external dependencies), Pico CSS, HTMX, Ed25519, embedded templates.

## License

MIT
