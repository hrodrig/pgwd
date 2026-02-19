# pgwd — Postgres Watch Dog

**Repo:** [github.com/hrodrig/pgwd](https://github.com/hrodrig/pgwd)

Go CLI that checks PostgreSQL connection counts (active/idle) and notifies via **Slack** and/or **Loki** when configured thresholds are exceeded. It can also alert on **stale connections** (connections that stay open and never close).

---

## Quick start

```bash
# See all options
pgwd -h

# Minimal: check once, alert to Slack (total/active default to 80% of server max_connections; change with -default-threshold-percent)
pgwd -db-url "postgres://user:pass@localhost:5432/mydb" \
     -slack-webhook "https://hooks.slack.com/services/..."

# Same but alert at 70% of max_connections
pgwd -db-url "postgres://..." -slack-webhook "https://..." -default-threshold-percent 70

# Or set an explicit threshold
pgwd -db-url "postgres://user:pass@localhost:5432/mydb" \
     -threshold-total 80 \
     -slack-webhook "https://hooks.slack.com/services/..."
```

---

## Configuration: CLI vs environment

Every option can be set by **CLI flag** or **environment variable** (prefix `PGWD_`). **CLI overrides env.** That lets you use env for secrets and defaults, and override with flags when needed.

### Using only environment variables

```bash
export PGWD_DB_URL="postgres://user:pass@localhost:5432/mydb"
export PGWD_THRESHOLD_TOTAL=80
export PGWD_THRESHOLD_IDLE=50
export PGWD_SLACK_WEBHOOK="https://hooks.slack.com/services/..."
export PGWD_INTERVAL=60

pgwd
# Runs as daemon every 60s; no need to pass any flag.
```

### Env for defaults, CLI to override

```bash
export PGWD_DB_URL="postgres://localhost:5432/mydb"
export PGWD_THRESHOLD_TOTAL=80
export PGWD_SLACK_WEBHOOK="https://hooks.slack.com/..."

# Override DB and run once (e.g. for a different host)
pgwd -db-url "postgres://prod-host:5432/mydb" -interval 0

# Override threshold for a quick test
pgwd -threshold-total 5 -dry-run
```

---

## Usage examples

### By threshold type

| Threshold | Use when you care about… | Example |
|-----------|---------------------------|--------|
| **total** | Overall connection usage (e.g. near `max_connections`) | `-threshold-total 80` |
| **active** | Queries running right now (load / long queries) | `-threshold-active 50` |
| **idle** | Pool size / connections sitting idle | `-threshold-idle 40` |
| **stale** | Connections open too long (leaks, never closed) | `-stale-age 600 -threshold-stale 1` |

```bash
# Total connections ≥ 80 (one-shot, Slack)
pgwd -db-url "postgres://user:pass@localhost:5432/mydb" \
     -threshold-total 80 \
     -slack-webhook "https://hooks.slack.com/services/..."

# Active connections ≥ 50 (one-shot, Loki)
pgwd -db-url "postgres://..." -threshold-active 50 -loki-url "http://localhost:3100/loki/api/v1/push"

# Idle connections ≥ 40 (daemon every 60s, Slack)
pgwd -db-url "postgres://..." -threshold-idle 40 -interval 60 -slack-webhook "https://..."

# Stale: ≥ 1 connection open longer than 10 minutes
pgwd -db-url "postgres://..." -stale-age 600 -threshold-stale 1 -slack-webhook "https://..."
```

### Multiple thresholds in one run

You can combine several thresholds; each one that is exceeded generates an alert (same run can send multiple events).

```bash
# Alert on total OR idle OR stale in a single run
pgwd -db-url "postgres://..." \
     -threshold-total 90 \
     -threshold-idle 60 \
     -stale-age 600 -threshold-stale 1 \
     -interval 120 \
     -slack-webhook "https://..." \
     -loki-url "http://localhost:3100/loki/api/v1/push"
```

### By notifier

```bash
# Slack only
pgwd -db-url "postgres://..." -threshold-total 80 -slack-webhook "https://hooks.slack.com/..."

# Loki only (optional labels)
pgwd -db-url "postgres://..." -threshold-total 80 \
     -loki-url "http://localhost:3100/loki/api/v1/push" \
     -loki-labels "job=pgwd,env=prod,db=myapp"

# Slack and Loki (same event sent to both)
pgwd -db-url "postgres://..." -threshold-total 80 \
     -slack-webhook "https://hooks.slack.com/..." \
     -loki-url "http://localhost:3100/loki/api/v1/push"
```

### Run mode and dry-run

```bash
# One-shot: run once, then exit (ideal for cron)
pgwd -db-url "postgres://..." -threshold-total 80 -slack-webhook "https://..."
# or: PGWD_INTERVAL=0 pgwd

# Daemon: run every N seconds until Ctrl+C or SIGTERM
pgwd -db-url "postgres://..." -threshold-total 80 -interval 60 -slack-webhook "https://..."

# Dry run: only print stats (total/active/idle), no notifications; no webhook/loki needed
pgwd -db-url "postgres://..." -threshold-total 100 -dry-run
# Output example: total=42 active=3 idle=39

# Force notification: send a test message to all configured notifiers (no threshold required)
# Use to validate delivery and format before relying on real alerts
pgwd -db-url "postgres://..." -slack-webhook "https://..." -force-notification
pgwd -db-url "postgres://..." -loki-url "http://localhost:3100/loki/api/v1/push" -force-notification
```

---

## Typical scenarios

| Scenario | Suggestion |
|----------|------------|
| **Cron check every 5 min** | One-shot (`interval` 0 or unset), one or more thresholds, Slack or Loki. Run from cron every 5 minutes. |
| **Long-running watcher** | Daemon with `-interval 60` (or 120). Run under systemd/supervisor; stop with SIGTERM. |
| **Detect connection leaks** | Use `stale-age` + `threshold-stale` (e.g. 600 and 1). Alert when any connection stays open longer than 10 min. |
| **Pre-production test** | `-dry-run` and low thresholds to see current counts without sending alerts. |
| **Validate notifications** | `-force-notification` with Slack/Loki: sends one test message regardless of thresholds. Use one-shot to confirm delivery and format. |
| **Zero config (use defaults)** | Only set `-db-url` and a notifier; total and active thresholds default to `default-threshold-percent` (default 80%) of server `max_connections`. Use `-default-threshold-percent` to change (e.g. 70 or 90). |
| **Multiple environments** | Set `PGWD_*` in env per environment; override `-db-url` or `-loki-labels` per deploy. |

---

## Parameters

All parameters can be set via **CLI** or **environment variables** with prefix `PGWD_`. CLI overrides env.

| CLI | Env | Description |
|-----|-----|-------------|
| `-db-url` | `PGWD_DB_URL` | PostgreSQL connection URL (required) |
| `-threshold-total` | `PGWD_THRESHOLD_TOTAL` | Alert when total connections ≥ N (default: default-threshold-percent of max_connections if 0) |
| `-threshold-active` | `PGWD_THRESHOLD_ACTIVE` | Alert when active connections ≥ N (default: default-threshold-percent of max_connections if 0) |
| `-threshold-idle` | `PGWD_THRESHOLD_IDLE` | Alert when idle connections ≥ N |
| `-stale-age` | `PGWD_STALE_AGE` | Consider connection stale if open longer than N seconds (requires `-threshold-stale`) |
| `-threshold-stale` | `PGWD_THRESHOLD_STALE` | Alert when stale connections (open > stale-age) ≥ N |
| `-slack-webhook` | `PGWD_SLACK_WEBHOOK` | Slack Incoming Webhook URL |
| `-loki-url` | `PGWD_LOKI_URL` | Loki push API URL (e.g. `http://localhost:3100/loki/api/v1/push`) |
| `-loki-labels` | `PGWD_LOKI_LABELS` | Loki labels, e.g. `job=pgwd,env=prod` |
| `-interval` | `PGWD_INTERVAL` | Run every N seconds; 0 = run once |
| `-dry-run` | `PGWD_DRY_RUN` | Only print stats, do not send notifications |
| `-force-notification` | `PGWD_FORCE_NOTIFICATION` | Always send one test notification to all notifiers (to validate delivery/format); requires at least one notifier |
| `-default-threshold-percent` | `PGWD_DEFAULT_THRESHOLD_PERCENT` | When total/active threshold are 0, set them to this % of max_connections (1–100). Default: 80 |

**Stale connections:** A connection is "stale" if it has been open longer than `stale-age` seconds (based on `backend_start` in `pg_stat_activity`). Use this to detect leaks or connections that are never closed. When using `threshold-stale`, `stale-age` must be set and > 0.

**Default thresholds:** If you do not set `threshold-total` or `threshold-active` (leave them 0), pgwd sets them to a **percentage of the server's `max_connections`** after connecting. The percentage is controlled by **`-default-threshold-percent`** / **`PGWD_DEFAULT_THRESHOLD_PERCENT`** (default **80**, range 1–100). Example: with `max_connections=100` and default percent 80, total and active thresholds become 80; with `-default-threshold-percent 70` they become 70. So you can run with only `-db-url` and a notifier and get alerts at your chosen percentage of the server limit. Idle and stale have no default (0 = disabled). Defaults are applied once at startup; the DB user must be able to read `max_connections` (any normal role can).

## Build

```bash
go build -o pgwd ./cmd/pgwd
```

## Testing

Unit tests for config (env, defaults, overrides) and notify (Loki label parsing):

```bash
go test ./internal/config/... ./internal/notify/... -v
```

Run all tests (including any in other packages):

```bash
go test ./...
```

## Requirements

- At least one of: a threshold (`threshold-total`, `threshold-active`, `threshold-idle`, or `threshold-stale` with `stale-age`), `-dry-run`, or `-force-notification`. If you set only `-db-url` and a notifier, pgwd defaults total and active to `default-threshold-percent` (default 80) of `max_connections`.
- If not using `-dry-run`: at least one notifier (`slack-webhook` or `loki-url`). For `-force-notification`, a notifier is required.
- For `threshold-stale`, `stale-age` must be set and greater than 0.

## Behavior and exit

- **One-shot** (`interval` 0 or unset): runs one check, sends alerts if thresholds are exceeded, then exits. Exit code 0 on success; non-zero on fatal errors (e.g. DB connection failure).
- **Daemon** (`interval` greater than 0): runs every `interval` seconds until interrupted (Ctrl+C or SIGTERM). Exits with 0 after a clean shutdown.
- **Dry run**: same as above but no HTTP calls to Slack/Loki; only logs stats to stdout.

## Help

```bash
pgwd -h
```

Shows all flags and their env equivalents.

## Slack

Create an [Incoming Webhook](https://api.slack.com/messaging/webhooks) in your Slack workspace and set `PGWD_SLACK_WEBHOOK` or `-slack-webhook`.

**Notification format:** One message per alert. Body (plain text in the webhook payload):

```
:warning: *pgwd* – Threshold exceeded
*<Message>*
Connections: total=<Total>, active=<Active>, idle=<Idle> (limit <Threshold>=<ThresholdValue>)
```

- `<Message>` is the event message (e.g. `Total connections 85 >= 80` or `Test notification — delivery check (force-notification). Current: total=…`).
- `<Total>`, `<Active>`, `<Idle>` are the current connection counts from `pg_stat_activity` for the current database.
- `<Threshold>` is one of `total`, `active`, `idle`, `stale`, or `test` (for force-notification).
- `<ThresholdValue>` is the configured limit that was exceeded (0 for `test`).

## Loki

Set the Loki push endpoint URL (e.g. `http://loki:3100/loki/api/v1/push`). Optionally set `PGWD_LOKI_LABELS` for stream labels (e.g. `job=pgwd,env=prod`); default includes `job=pgwd`.

**Notification format:** Each alert is one log line in a stream. The stream has labels from `PGWD_LOKI_LABELS` plus `job=pgwd` (if not set) and `threshold=<total|active|idle|stale|test>`. The log line is:

```
pgwd threshold exceeded: <Message> | total=<Total> active=<Active> idle=<Idle> (limit <Threshold>=<ThresholdValue>)
```

Same placeholders as Slack. Timestamp is the time of the push. You can query in Grafana or LogCLI by label (e.g. `{job="pgwd", threshold="total"}`).

---

## Troubleshooting

| Symptom | What to check |
|--------|----------------|
| **"missing database URL"** | Set `PGWD_DB_URL` or `-db-url`. The URL must be a valid [PostgreSQL connection string](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING). |
| **"at least one of: threshold..."** | You need a threshold, or `-dry-run`, or `-force-notification`. If you set only `-db-url` and a notifier, pgwd tries to default total/active to 80% of `max_connections` after connecting; this error means connect failed before defaults could be applied, or the server did not return `max_connections`. |
| **"no notifier configured"** | Set `PGWD_SLACK_WEBHOOK` or `PGWD_LOKI_URL` (or use `-dry-run` to skip notifications). |
| **"force-notification requires at least one notifier"** | Use `-force-notification` together with `-slack-webhook` and/or `-loki-url`. |
| **"when using threshold-stale, stale-age must be > 0"** | Set `-stale-age N` (e.g. 600) when using `-threshold-stale`. |
| **Slack/Loki not receiving alerts** | Run once with `-force-notification` to send a test message. Check webhook URL, network/firewall, and that the app can reach Slack/Loki. |
| **"postgres connect: ..."** | DB unreachable: check host, port, TLS, credentials, and that the pgwd host can reach the Postgres server. |
| **Stats or stale count errors in logs** | Permissions: the DB user must be able to read `pg_stat_activity` (usually any role can). Check `log.Printf` output for the exact error. |

---

## Docker

Example run with Docker (one-shot, env-based config). Build an image that includes the `pgwd` binary, or use a multi-stage build.

```bash
# Build (from repo root)
docker build -t pgwd -f Dockerfile .

# Run one-shot: pass env and ensure network to Postgres (and Slack/Loki if used)
docker run --rm \
  -e PGWD_DB_URL="postgres://user:pass@host.docker.internal:5432/mydb" \
  -e PGWD_THRESHOLD_TOTAL=80 \
  -e PGWD_SLACK_WEBHOOK="https://hooks.slack.com/..." \
  pgwd

# Run daemon (interval 60s)
docker run --rm -d --name pgwd \
  -e PGWD_DB_URL="postgres://user:pass@host.docker.internal:5432/mydb" \
  -e PGWD_THRESHOLD_TOTAL=80 \
  -e PGWD_SLACK_WEBHOOK="https://hooks.slack.com/..." \
  -e PGWD_INTERVAL=60 \
  pgwd
```

Minimal **Dockerfile** (from project root, with `go.mod` and source):

```dockerfile
FROM golang:1.21-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /pgwd ./cmd/pgwd

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=build /pgwd /pgwd
ENTRYPOINT ["/pgwd"]
```

Notes: use `host.docker.internal` (or your host IP) to reach Postgres on the host from the container; for secrets, prefer env files or a secrets manager instead of hardcoding in the image.

---

## systemd

pgwd is configured **only via environment variables** (no config file yet). On systemd you use an env file that the unit loads.

**Convention**

| What | Path |
|------|------|
| Binary | `/usr/local/bin/pgwd` |
| Env file (option A) | `/etc/pgwd.env` |
| Env file (option B) | `/etc/pgwd/pgwd.env` (useful if you later add e.g. `/etc/pgwd/pgwd.toml`) |

The unit files in the repo try both env paths (`EnvironmentFile=-/etc/pgwd/pgwd.env` then `-/etc/pgwd.env`). Create one of them and restrict permissions: `sudo chmod 600 /etc/pgwd.env`.

**Two ways to run**

1. **Daemon** — pgwd runs continuously and checks every `PGWD_INTERVAL` seconds. Use `contrib/systemd/pgwd.service`.
2. **One-shot on a schedule** — pgwd runs once per tick (e.g. every 5 minutes). Use `contrib/systemd/pgwd.timer` + `contrib/systemd/pgwd-once.service`.

**Daemon (long-running)**

```bash
# Install binary
sudo cp pgwd /usr/local/bin/pgwd

# Copy unit and create env file
sudo cp contrib/systemd/pgwd.service /etc/systemd/system/
sudo tee /etc/pgwd.env > /dev/null << 'EOF'
PGWD_DB_URL=postgres://user:pass@localhost:5432/mydb
PGWD_THRESHOLD_TOTAL=80
PGWD_THRESHOLD_IDLE=50
PGWD_SLACK_WEBHOOK=https://hooks.slack.com/services/...
PGWD_LOKI_URL=http://localhost:3100/loki/api/v1/push
PGWD_INTERVAL=60
EOF
sudo chmod 600 /etc/pgwd.env

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable --now pgwd
sudo systemctl status pgwd

# Logs
journalctl -u pgwd -f
```

**One-shot from a timer (cron-like)**

Runs pgwd once every 5 minutes (no `PGWD_INTERVAL` needed; the timer is the schedule).

```bash
sudo cp pgwd /usr/local/bin/pgwd
sudo cp contrib/systemd/pgwd-once.service contrib/systemd/pgwd.timer /etc/systemd/system/
# Create /etc/pgwd.env as above (omit PGWD_INTERVAL or set 0)

sudo systemctl daemon-reload
sudo systemctl enable --now pgwd.timer
systemctl list-timers --all | grep pgwd
```

To change the interval, edit the timer: `OnUnitActiveSec=5min` → e.g. `OnUnitActiveSec=10min`, then `sudo systemctl daemon-reload`.

**Env file example** (`/etc/pgwd.env` or `/etc/pgwd/pgwd.env`)

```bash
PGWD_DB_URL=postgres://user:pass@localhost:5432/mydb
PGWD_THRESHOLD_TOTAL=80
PGWD_THRESHOLD_IDLE=50
PGWD_SLACK_WEBHOOK=https://hooks.slack.com/services/...
PGWD_LOKI_URL=http://localhost:3100/loki/api/v1/push
PGWD_INTERVAL=60
# For timer (one-shot) omit PGWD_INTERVAL or set 0
```

**Optional:** Run the service as a dedicated user: create `useradd -r -s /bin/false pgwd`, then in the unit add `User=pgwd` and `Group=pgwd`. Ensure that user can read the env file (e.g. same group or move secrets to a credential store).
