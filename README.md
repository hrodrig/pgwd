# pgwd — Postgres Watch Dog

[![Version](https://img.shields.io/badge/version-0.2.2-blue)](https://github.com/hrodrig/pgwd/releases)
[![Release](https://img.shields.io/github/v/release/hrodrig/pgwd)](https://github.com/hrodrig/pgwd/releases)
[![Go 1.26](https://img.shields.io/badge/go-1.26-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/hrodrig/pgwd)](https://pkg.go.dev/github.com/hrodrig/pgwd)
[![Go Report Card](https://goreportcard.com/badge/github.com/hrodrig/pgwd)](https://goreportcard.com/report/github.com/hrodrig/pgwd)

**Repo:** [github.com/hrodrig/pgwd](https://github.com/hrodrig/pgwd) · **Releases:** [Releases](https://github.com/hrodrig/pgwd/releases)

Go CLI that checks PostgreSQL connection counts (active/idle) and notifies via **Slack** and/or **Loki** when configured thresholds are exceeded. It can also alert on **stale connections** (connections that stay open and never close).

**Documentation:** [Sequence diagrams](docs/README.md#sequence-diagrams) (Mermaid) for each use case, [audited against the code](docs/sequence/AUDIT.md), and terminal demo (recorded with [VHS](https://github.com/charmbracelet/vhs)) — see [docs/](docs/README.md). **Scanning** before release (govulncheck, Grype): [tools/README.md](tools/README.md).

![Terminal demo](docs/demo.gif)

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
| **Validate notifications** | `-force-notification` with Slack/Loki: sends one test message regardless of thresholds. Use one-shot to confirm delivery, format, and how messages look. (If the connection to Postgres fails, pgwd always sends a connect-failure alert when a notifier is configured.) |
| **Test alerts without low max_connections** | Use `-test-max-connections N` (e.g. 20) with `-force-notification` or low thresholds: thresholds and messages use N as “max_connections”, while stats stay real. Notifications show “(test override)” so total can exceed N. |
| **Zero config (use defaults)** | Only set `-db-url` and a notifier; total and active thresholds default to `default-threshold-percent` (default 80%) of server `max_connections`. Use `-default-threshold-percent` to change (e.g. 70 or 90). |
| **Multiple environments** | Set `PGWD_*` in env per environment; override `-db-url` or `-loki-labels` per deploy. |
| **Postgres in Kubernetes** | Use `-kube-postgres namespace/svc/name` (or `namespace/pod/name`). pgwd runs `kubectl port-forward` and connects to localhost. Optionally put `DISCOVER_MY_PASSWORD` in the URL to read the password from the pod's env (e.g. `POSTGRES_PASSWORD`). Requires `kubectl` in PATH. |
| **Alert when Postgres is unreachable** | If you configure a notifier (Slack/Loki), pgwd **always** sends an alert when the connection fails (e.g. refused, timeout, or "too many clients"). No extra flag needed. |

### Running from cron

Cron runs with a **minimal environment** (e.g. `PATH=/usr/bin:/bin`). Two things to keep in mind:

1. **`-kube-postgres` and PATH:** If you use `-kube-postgres`, cron must see `kubectl` in PATH. Set `PATH` in the cron line or in a wrapper script so it includes the directory where `kubectl` lives (e.g. `/usr/local/bin`):

   ```bash
   # In crontab: set PATH before the command
   PATH=/usr/local/bin:/usr/bin:/bin
   */5 * * * * /usr/local/bin/pgwd -kube-postgres default/svc/postgres -db-url "postgres://..." -slack-webhook "https://..."
   ```

   Or use a wrapper script that exports PATH and runs pgwd:

   ```bash
   #!/bin/sh
   export PATH="/usr/local/bin:$PATH"
   exec /usr/local/bin/pgwd "$@"
   ```

2. **Seeing errors:** If `kubectl` is not found, pgwd exits immediately with a clear message to stderr. Cron often mails stderr to the user; otherwise redirect stdout and stderr to a log file so you can see why the job failed:

   ```bash
   */5 * * * * /usr/local/bin/pgwd -db-url "postgres://..." -slack-webhook "https://..." >> /var/log/pgwd.log 2>&1
   ```

   Here `>>` appends stdout to the file and `2>&1` sends stderr to the same place.

### Example: multiple services and heartbeat via bash + cron

You can run pgwd for several Postgres instances (e.g. one per Kubernetes service) from a single cron schedule: use a bash script that sets `KUBECONFIG`, `PGWD_SLACK_WEBHOOK`, and `PATH`, then invokes pgwd once per service with distinct **`-kube-local-port`** values so port-forwards do not clash. Add a second script that runs **`-force-notification`** on a schedule (e.g. every 2 hours) as a “still alive” heartbeat.

**Check script** (e.g. `~/bin/pgwd-cron.sh`): runs every 5 minutes, checks all services, alerts only when thresholds are exceeded.

```bash
#!/bin/bash
mkdir -p ~/log
export KUBECONFIG=/path/to/your/kubeconfig
export PGWD_SLACK_WEBHOOK="https://hooks.slack.com/services/..."
export PATH="/usr/local/bin:$PATH"
PGWD=${PGWD:-/usr/local/bin/pgwd}

echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) checking postgres-a"
$PGWD -kube-postgres mynamespace/svc/postgres-a \
  -kube-local-port 15432 \
  -db-url 'postgres://postgres:DISCOVER_MY_PASSWORD@postgres-a:15432/db_a'

echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) checking postgres-b"
$PGWD -kube-postgres mynamespace/svc/postgres-b \
  -kube-local-port 15433 \
  -db-url 'postgres://postgres:DISCOVER_MY_PASSWORD@postgres-b:15433/db_b'

echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) checking postgres-c"
$PGWD -kube-postgres mynamespace/svc/postgres-c \
  -kube-local-port 15434 \
  -db-url 'postgres://postgres:DISCOVER_MY_PASSWORD@postgres-c:15434/db_c'

exit 0
```

**Heartbeat script** (e.g. `~/bin/pgwd-heartbeat.sh`): runs every 2 hours, sends a test notification per service so you know the pipeline is up. Use different local ports (e.g. 25432…) so they do not conflict with the check script if both run close together.

```bash
#!/bin/bash
mkdir -p ~/log
export KUBECONFIG=/path/to/your/kubeconfig
export PGWD_SLACK_WEBHOOK="https://hooks.slack.com/services/..."
export PATH="/usr/local/bin:$PATH"
PGWD=${PGWD:-/usr/local/bin/pgwd}

$PGWD -kube-postgres mynamespace/svc/postgres-a \
  -kube-local-port 25432 \
  -db-url 'postgres://postgres:DISCOVER_MY_PASSWORD@postgres-a:25432/db_a' \
  -force-notification

$PGWD -kube-postgres mynamespace/svc/postgres-b \
  -kube-local-port 25433 \
  -db-url 'postgres://postgres:DISCOVER_MY_PASSWORD@postgres-b:25433/db_b' \
  -force-notification

exit 0
```

**Crontab** (`crontab -e`): run checks every 5 minutes and heartbeat every 2 hours; append output to one log file.

```
PATH=/usr/bin:/bin
*/5 * * * * /bin/bash -l -c '~/bin/pgwd-cron.sh >> ~/log/pgwd.log 2>&1'
0 */2 * * * /bin/bash -l -c '~/bin/pgwd-heartbeat.sh >> ~/log/pgwd.log 2>&1'
```

Adjust `KUBECONFIG`, webhook URL, namespace, service names, database names, and `PGWD` path to your environment. If a pod uses a different env var for the password, add **`-kube-password-var VARNAME`** (and **`-kube-password-container`** if the var is in another container). The `echo` lines in the check script make it easy to see which service produced an error in the log.

---

## Kubernetes

When Postgres runs inside a Kubernetes cluster, use **`-kube-postgres`** so pgwd connects via `kubectl port-forward` (no separate script or manual port-forward).

**Format:** `-kube-postgres <namespace>/<type>/<name>` with `type` = `svc` or `pod`, e.g. `default/svc/postgres` or `default/pod/postgres-0`.

- Set **`PGWD_DB_URL`** with host **`localhost`** and the same port as **`-kube-local-port`** (default 5432). Example: `postgres://user:pass@localhost:5432/mydb`.
- **Password from the pod:** If the URL password is the literal **`DISCOVER_MY_PASSWORD`**, pgwd reads the password from the Postgres pod's environment (`POSTGRES_PASSWORD` by default, or `PGPASSWORD`). Use **`-kube-password-var`** to choose the env var and **`-kube-password-container`** if the Postgres container is not the default.
- **Requires:** `kubectl` in PATH and a valid kubeconfig. pgwd checks for `kubectl` before any kube step and exits with a clear error if it is missing. pgwd starts the port-forward, connects, and stops it on exit. **When running from cron**, set PATH so `kubectl` is findable (see [Running from cron](#running-from-cron) above).

```bash
# With password in URL
PGWD_DB_URL="postgres://postgres:secret@localhost:5432/mydb" \
  pgwd -kube-postgres default/svc/postgres -slack-webhook "https://..." -dry-run

# Password from pod env (POSTGRES_PASSWORD)
PGWD_DB_URL="postgres://postgres:DISCOVER_MY_PASSWORD@localhost:5432/mydb" \
  pgwd -kube-postgres default/svc/postgres -dry-run
```

---

## Parameters

All parameters can be set via **CLI** or **environment variables** with prefix `PGWD_`. CLI overrides env.

| CLI | Env | Description |
|-----|-----|-------------|
| `-db-url` | `PGWD_DB_URL` | PostgreSQL connection URL (required). With `-kube-postgres`, use host localhost and port matching `-kube-local-port`. |
| `-kube-postgres` | `PGWD_KUBE_POSTGRES` | Connect via kubectl port-forward: `namespace/type/name` (e.g. `default/svc/postgres`). Requires kubectl in PATH. |
| `-kube-local-port` | `PGWD_KUBE_LOCAL_PORT` | Local port for port-forward (default 5432). Use different ports to run multiple pgwd against different Postgres in the cluster. |
| `-kube-password-var` | `PGWD_KUBE_PASSWORD_VAR` | Pod env var name when URL password is `DISCOVER_MY_PASSWORD` (default `POSTGRES_PASSWORD`). |
| `-kube-password-container` | `PGWD_KUBE_PASSWORD_CONTAINER` | Container name in pod for password discovery (default: primary container). |
| `-cluster` | `PGWD_CLUSTER` | Cluster name shown in Slack/Loki (health-check style). When using `-kube-postgres`, detected from kubeconfig if unset. |
| `-client` | `PGWD_CLIENT` | Client/service/pod name shown in Slack (e.g. VM or service name). When using `-kube-postgres`, derived from resource (e.g. `svc/name`) if unset; otherwise hostname. |
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
| `-force-notification` | `PGWD_FORCE_NOTIFICATION` | Always send at least one notification: test event when connected (to validate delivery, format, and channel). Requires at least one notifier. (Connection failure is always notified when a notifier is configured, with or without this flag.) |
| `-notify-on-connect-failure` | `PGWD_NOTIFY_ON_CONNECT_FAILURE` | Legacy: connection failure is **always** notified when a notifier is configured; this flag is no longer required. Kept for backward compatibility; if set, still requires at least one notifier at startup. |
| `-default-threshold-percent` | `PGWD_DEFAULT_THRESHOLD_PERCENT` | When total/active threshold are 0, set them to this % of max_connections (1–100). Default: 80 |
| `-test-max-connections` | `PGWD_TEST_MAX_CONNECTIONS` | Override server `max_connections` for threshold defaults and display (testing only). When set, defaults and notifications use this value instead of the server’s; stats (total/active/idle) remain real. Notifications show “(test override)” so you can simulate e.g. a low limit and trigger alerts without a real low max_connections. |

**Stale connections:** A connection is "stale" if it has been open longer than `stale-age` seconds (based on `backend_start` in `pg_stat_activity`). Use this to detect leaks or connections that are never closed. When using `threshold-stale`, `stale-age` must be set and > 0.

**Default thresholds:** If you do not set `threshold-total` or `threshold-active` (leave them 0), pgwd sets them to a **percentage of the server's `max_connections`** after connecting. The percentage is controlled by **`-default-threshold-percent`** / **`PGWD_DEFAULT_THRESHOLD_PERCENT`** (default **80**, range 1–100). Example: with `max_connections=100` and default percent 80, total and active thresholds become 80; with `-default-threshold-percent 70` they become 70. So you can run with only `-db-url` and a notifier and get alerts at your chosen percentage of the server limit. Idle and stale have no default (0 = disabled). Defaults are applied once at startup; the DB user must be able to read `max_connections` (any normal role can).

## Install

**From source (recommended):**

```bash
go install github.com/hrodrig/pgwd@latest
```

This installs the binary to `$GOBIN` (default `$HOME/go/bin`). Ensure `$GOBIN` is on your `PATH`.

**Pre-built binaries:** [Releases](https://github.com/hrodrig/pgwd/releases) provide binaries (tar.gz, zip), `.deb`, and `.rpm` packages for Linux, macOS, and Windows (amd64 and arm64).

**Homebrew (macOS):**

```bash
brew install hrodrig/pgwd/pgwd
```

## Build

```bash
go build -o pgwd ./cmd/pgwd
# or use the Makefile:
make build
make install
# Custom install path: GOBIN=~/bin make install  (default is $HOME/go/bin)
```

**Release (GitHub):** From branch `main`, after tagging (e.g. `git tag v0.2.2`), run `make release`. Requires [goreleaser](https://goreleaser.com) (`brew install goreleaser`). For a local snapshot build without publishing: `make snapshot` (outputs to `dist/`).

## Testing

Unit tests for config (env, defaults, overrides) and notify (Loki label parsing):

```bash
go test ./internal/config/... ./internal/notify/... -v
```

Run all tests (including any in other packages):

```bash
go test ./...
```

## Development — validating locally

Run a PostgreSQL container to test pgwd without a real server. Use port **5433** on the host so connections from your machine go to the container and not to a local Postgres on 5432 (common on macOS):

```bash
docker stop pgwd-pg 2>/dev/null; docker rm pgwd-pg 2>/dev/null
docker run -d --name pgwd-pg \
  -e POSTGRES_PASSWORD=secret \
  -e POSTGRES_DB=postgres \
  -p 5433:5432 \
  postgres:16-alpine
```

**Connection URL** (host port 5433, `sslmode=disable`):

`postgres://postgres:secret@127.0.0.1:5433/postgres?sslmode=disable`

**Dry-run:**

```bash
pgwd -db-url "postgres://postgres:secret@127.0.0.1:5433/postgres?sslmode=disable" -dry-run
```

**With a notifier** (e.g. Slack): add `-slack-webhook "https://..."` and optionally `-threshold-total 5`.

**Stop and remove:**

```bash
docker stop pgwd-pg && docker rm pgwd-pg
```

Using **127.0.0.1** and host port **5433** avoids hitting a local Postgres on 5432 and avoids IPv6 resolution quirks.

**Who is on 5432?** Run `lsof -i :5432`. If you see both `postgres` (local) and `com.docke` (Docker), connections to **localhost:5432** go to the local postgres (it binds to localhost); the container is on `*:5432`. Use host port **5433** for the container so your client clearly reaches the container.

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

- `<Message>` is the event message (e.g. `Total connections 85 >= 80` or `Test notification — delivery check (force-notification).`).
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
| **"no thresholds set and could not default from server..."** | pgwd could not read `max_connections` from the server (error or 0). Set `-threshold-total` and/or `-threshold-active` explicitly, or use `-dry-run` or `-force-notification`. With a normal Postgres, only `-db-url` and a notifier should be enough (defaults to 80% of `max_connections`). |
| **"no notifier configured"** | Set `PGWD_SLACK_WEBHOOK` or `PGWD_LOKI_URL` (or use `-dry-run` to skip notifications). |
| **"force-notification requires at least one notifier"** | Use `-force-notification` together with `-slack-webhook` and/or `-loki-url`. |
| **"notify-on-connect-failure requires at least one notifier"** | You set `-notify-on-connect-failure` but have no notifier. Add `-slack-webhook` and/or `-loki-url`. (Connect failure is always notified when a notifier is configured; the flag is optional.) |
| **"kubectl not found in PATH"** | When using `-kube-postgres`, ensure `kubectl` is installed and on your `PATH` (e.g. `which kubectl`). pgwd exits with this message before attempting port-forward or password discovery. |
| **"when using threshold-stale, stale-age must be > 0"** | Set `-stale-age N` (e.g. 600) when using `-threshold-stale`. |
| **Slack/Loki not receiving alerts** | Run once with `-force-notification` to send a test message. Check webhook URL, network/firewall, and that the app can reach Slack/Loki. |
| **"postgres connect: ..."** | DB unreachable: check host, port, TLS, credentials, and that the pgwd host can reach the Postgres server. |
| **Stats or stale count errors in logs** | Permissions: the DB user must be able to read `pg_stat_activity` (usually any role can). Check `log.Printf` output for the exact error. |

---

## Docker

**Published image (each release):** Multi-arch images (linux/amd64, linux/arm64) are published to [GitHub Container Registry](https://github.com/hrodrig/pgwd/pkgs/container/pgwd) as `ghcr.io/hrodrig/pgwd`. Use a version tag or `latest`:

```bash
docker pull ghcr.io/hrodrig/pgwd:v0.2.2
# or
docker pull ghcr.io/hrodrig/pgwd:latest
```

**Build from source:** The repo includes a multi-stage **Dockerfile** (Go 1.26, Alpine 3.23): build stage compiles the binary with version/commit/build date injected via build args; runtime stage is minimal and runs as non-root. Use `make docker-build` to build locally with version info.

**Image details**

- **Runtime base:** Alpine 3.23. Only `ca-certificates` for HTTPS (Slack/Loki). No `wget`, `nc`, or `curl` (base image’s `wget`/`nc` are BusyBox applets and are removed; they are not separate packages, so we remove the symlinks).
- **User:** Runs as non-root user `pgwd` (binary in `/home/pgwd/pgwd`).
- **Labels:** OCI image labels (title, description, source, authors).
- **Build context:** `.dockerignore` uses a whitelist: only `go.mod`, `go.sum`, `cmd/`, and `internal/` are sent; `docs/`, `contrib/`, README, etc. are excluded.

**Build (from repo root)**

Use **`make docker-build`** so the image gets version, commit, and build date from the `VERSION` file and git (same as `make build`):

```bash
make docker-build
```

This runs `docker build` with `--build-arg VERSION=...`, `--build-arg COMMIT=...`, `--build-arg BUILDDATE=...`. If you build with plain `docker build -t pgwd .`, the binary will report `dev` / `unknown` for version and commit.

**Validate the image**

Use the published image `ghcr.io/hrodrig/pgwd:latest` (or `:v0.2.2`), or `pgwd` if you built locally with `make docker-build`:

```bash
# Help (no DB needed)
docker run --rm ghcr.io/hrodrig/pgwd:latest -h

# Version (should show e.g. pgwd v0.2.2 (commit ..., built ...))
docker run --rm ghcr.io/hrodrig/pgwd:latest --version

# Expect "missing database URL" (validates startup path)
docker run --rm ghcr.io/hrodrig/pgwd:latest
```

**Run (one-shot or daemon)**

```bash
# One-shot: pass env and ensure network to Postgres (and Slack/Loki if used)
docker run --rm \
  -e PGWD_DB_URL="postgres://user:pass@host.docker.internal:5432/mydb" \
  -e PGWD_THRESHOLD_TOTAL=80 \
  -e PGWD_SLACK_WEBHOOK="https://hooks.slack.com/..." \
  ghcr.io/hrodrig/pgwd:latest

# Daemon (interval 60s)
docker run --rm -d --name pgwd \
  -e PGWD_DB_URL="postgres://user:pass@host.docker.internal:5432/mydb" \
  -e PGWD_THRESHOLD_TOTAL=80 \
  -e PGWD_SLACK_WEBHOOK="https://hooks.slack.com/..." \
  -e PGWD_INTERVAL=60 \
  ghcr.io/hrodrig/pgwd:latest
```

Use `host.docker.internal` (or your host IP) to reach Postgres on the host from the container. For secrets, prefer env files or a secrets manager instead of hardcoding in the image.

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
