# pgwd — Postgres Watch Dog

<a id="top"></a>

<p align="center">
  <strong>🐕</strong> <em>Watch your PostgreSQL connections</em>
</p>

[![Version](https://img.shields.io/badge/version-0.5.7-blue)](https://github.com/hrodrig/pgwd/releases)
[![Release](https://img.shields.io/github/v/release/hrodrig/pgwd)](https://github.com/hrodrig/pgwd/releases)
[![Go 1.26](https://img.shields.io/badge/go-1.26-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![pkg.go.dev](https://pkg.go.dev/badge/github.com/hrodrig/pgwd)](https://pkg.go.dev/github.com/hrodrig/pgwd)
[![Go Report Card](https://goreportcard.com/badge/github.com/hrodrig/pgwd)](https://goreportcard.com/report/github.com/hrodrig/pgwd)
[![deps.dev](https://img.shields.io/badge/deps.dev-go%20module-blue)](https://deps.dev/go/github.com/hrodrig/pgwd)
[![DEV.to](https://img.shields.io/badge/DEV.to-Article-0A0A0A?logo=dev.to)](https://dev.to/hrodrig/pgwd-a-watchdog-for-your-postgresql-connections-1pjg)

**Repo:** [github.com/hrodrig/pgwd](https://github.com/hrodrig/pgwd) · **Releases:** [Releases](https://github.com/hrodrig/pgwd/releases)

Go CLI that checks PostgreSQL connection counts (active/idle) and notifies via **Slack** and/or **Loki** when configured thresholds are exceeded. It can also alert on **stale connections** (connections that stay open and never close).

**Documentation:** [Sequence diagrams](docs/README.md#sequence-diagrams) (Mermaid) for each use case, [audited against the code](docs/sequence/AUDIT.md), terminal demo (recorded with [VHS](https://github.com/charmbracelet/vhs)), and `man pgwd` (included in .deb/.rpm packages) — see [docs/](docs/README.md). **Scanning** before release (govulncheck, Grype): [tools/README.md](tools/README.md).

![Terminal demo](docs/demo.gif)

## Table of contents

- [Quick start](#quick-start)
- [Configuration: CLI vs environment](#configuration-cli-vs-environment)
- [Usage examples](#usage-examples)
- [Typical scenarios](#typical-scenarios)
- [Kubernetes](#kubernetes)
- [Parameters](#parameters)
- [Install](#install)
- [Build](#build)
- [Testing](#testing)
- [Requirements](#requirements)
- [Slack](#slack)
- [Loki](#loki)
- [Troubleshooting](#troubleshooting)
- [FAQ](#faq)
- [Docker](#docker)
- [systemd](#systemd)
- [Alpine Linux (OpenRC)](#alpine-linux-openrc)
- [OpenBSD](#openbsd)
- [FreeBSD](#freebsd)
- [NetBSD](#netbsd)
- [DragonFly BSD](#dragonfly-bsd)
- [Roadmap](#roadmap)
- [Get involved](#get-involved)

---

## Quick start

```bash
# See all options
pgwd -h

# Minimal: check once, alert to Slack (3-tier levels 75/85/95% by default; or use -db-default-threshold-percent)
pgwd -db-url "postgres://user:pass@localhost:5432/mydb" \
     -notifications-slack-webhook "https://hooks.slack.com/services/..."

# Custom 3-tier levels (default 75,85,95)
pgwd -db-url "postgres://..." -notifications-slack-webhook "https://..." -db-threshold-levels 70,85,90
```

### Breaking changes (upgrade from 0.5.x)

If you use CLI flags or env vars for notifications or DB thresholds, update your scripts:

| Old | New |
|-----|-----|
| `-threshold-total` | `-db-threshold-total` |
| `-threshold-active` | `-db-threshold-active` |
| `-threshold-idle` | `-db-threshold-idle` |
| `-threshold-stale` | `-db-threshold-stale` |
| `-threshold-levels` | `-db-threshold-levels` |
| `-stale-age` | `-db-stale-age` |
| `-default-threshold-percent` | `-db-default-threshold-percent` |
| `-slack-webhook` | `-notifications-slack-webhook` |
| `-loki-url` | `-notifications-loki-url` |
| `-loki-labels` | `-notifications-loki-labels` |
| `-loki-org-id` | `-notifications-loki-org-id` |
| `-loki-bearer-token` | `-notifications-loki-bearer-token` |
| `PGWD_SLACK_WEBHOOK` | `PGWD_NOTIFICATIONS_SLACK_WEBHOOK` |
| `PGWD_LOKI_URL` | `PGWD_NOTIFICATIONS_LOKI_URL` |
| `PGWD_LOKI_LABELS` | `PGWD_NOTIFICATIONS_LOKI_LABELS` |
| `PGWD_LOKI_ORG_ID` | `PGWD_NOTIFICATIONS_LOKI_ORG_ID` |
| `PGWD_LOKI_BEARER_TOKEN` | `PGWD_NOTIFICATIONS_LOKI_BEARER_TOKEN` |
| `PGWD_THRESHOLD_TOTAL` | `PGWD_DB_THRESHOLD_TOTAL` |
| `PGWD_THRESHOLD_ACTIVE` | `PGWD_DB_THRESHOLD_ACTIVE` |
| `PGWD_THRESHOLD_IDLE` | `PGWD_DB_THRESHOLD_IDLE` |
| `PGWD_THRESHOLD_STALE` | `PGWD_DB_THRESHOLD_STALE` |
| `PGWD_THRESHOLD_LEVELS` | `PGWD_DB_THRESHOLD_LEVELS` |
| `PGWD_STALE_AGE` | `PGWD_DB_STALE_AGE` |
| `PGWD_DEFAULT_THRESHOLD_PERCENT` | `PGWD_DB_DEFAULT_THRESHOLD_PERCENT` |

Config file keys unchanged.

---

## Configuration: config file, env, CLI

pgwd loads settings from (in order): **config file** → **environment variables** → **CLI flags**. Each layer overrides the previous.

| Source | Path / prefix |
|--------|---------------|
| Config file | `/etc/pgwd/pgwd.conf` (or `-config` / `PGWD_CONFIG`) |
| Environment | `PGWD_*` |
| CLI | `-flag` |

**Config file** (YAML) — keys match `-flag` and `PGWD_*` env vars. See `contrib/pgwd.conf.example`. One config = one Postgres; for multiple instances, use one config per instance (e.g. cron with `-config /etc/pgwd/prod-db1.conf`).

```bash
# Use default path /etc/pgwd/pgwd.conf
pgwd

# Or specify path
pgwd -config /etc/pgwd/pgwd.conf
PGWD_CONFIG=/path/to/pgwd.conf pgwd
```

**CLI overrides env, env overrides config file.** Use env for secrets and overrides; use config file for base settings.

### Using only environment variables

```bash
export PGWD_DB_URL="postgres://user:pass@localhost:5432/mydb"
export PGWD_DB_THRESHOLD_LEVELS="75,85,95"
export PGWD_DB_THRESHOLD_IDLE=50
export PGWD_NOTIFICATIONS_SLACK_WEBHOOK="https://hooks.slack.com/services/..."
export PGWD_INTERVAL=60

pgwd
# Runs as daemon every 60s; no need to pass any flag.
```

### Env for defaults, CLI to override

```bash
export PGWD_DB_URL="postgres://localhost:5432/mydb"
export PGWD_DB_THRESHOLD_LEVELS="70,85,90"
export PGWD_NOTIFICATIONS_SLACK_WEBHOOK="https://hooks.slack.com/..."

# Override DB and run once (e.g. for a different host)
pgwd -db-url "postgres://prod-host:5432/mydb" -interval 0

# Override threshold for a quick test
pgwd -db-threshold-levels 5,10,15 -dry-run
```

---

## Usage examples

### By threshold type

| Threshold | Use when you care about… | Example |
|-----------|---------------------------|--------|
| **levels** (3-tier) | % of `max_connections` — attention / alert / danger (default for total/active) | `-db-threshold-levels 75,85,95` (default) or `-db-threshold-levels 70,85,90` |
| **idle** | Pool size / connections sitting idle | `-db-threshold-idle 40` |
| **stale** | Connections open too long (leaks, never closed) | `-db-stale-age 600 -db-threshold-stale 1` |

```bash
# 3-tier levels (default 75,85,95% of max_connections) — one-shot, Slack
pgwd -db-url "postgres://user:pass@localhost:5432/mydb" \
     -notifications-slack-webhook "https://hooks.slack.com/services/..."

# Custom levels (e.g. 70,85,90%) — one-shot, Loki
pgwd -db-url "postgres://..." -db-threshold-levels 70,85,90 -notifications-loki-url "http://localhost:3100/loki/api/v1/push"

# Idle connections ≥ 40 (daemon every 60s, Slack)
pgwd -db-url "postgres://..." -db-threshold-idle 40 -interval 60 -notifications-slack-webhook "https://..."

# Stale: ≥ 1 connection open longer than 10 minutes
pgwd -db-url "postgres://..." -db-stale-age 600 -db-threshold-stale 1 -notifications-slack-webhook "https://..."
```

### Multiple thresholds in one run

You can combine several thresholds; each one that is exceeded generates an alert (same run can send multiple events).

```bash
# Alert on levels (3-tier) OR idle OR stale in a single run
pgwd -db-url "postgres://..." \
     -db-threshold-levels 75,85,95 \
     -db-threshold-idle 60 \
     -db-stale-age 600 -db-threshold-stale 1 \
     -interval 120 \
     -notifications-slack-webhook "https://..." \
     -notifications-loki-url "http://localhost:3100/loki/api/v1/push"
```

### By notifier

```bash
# Slack only (default 3-tier levels)
pgwd -db-url "postgres://..." -notifications-slack-webhook "https://hooks.slack.com/..."

# Loki only (optional labels)
pgwd -db-url "postgres://..." \
     -notifications-loki-url "http://localhost:3100/loki/api/v1/push" \
     -notifications-loki-labels "app=pgwd,env=prod,db=myapp"

# Slack and Loki (same event sent to both)
pgwd -db-url "postgres://..." \
     -notifications-slack-webhook "https://hooks.slack.com/..." \
     -notifications-loki-url "http://localhost:3100/loki/api/v1/push"
```

### Run mode and dry-run

| `interval` | Behavior |
|-------------|----------|
| **0** | One-shot — check once, then exit |
| **> 0** (e.g. 60) | Daemon — check every N seconds until Ctrl+C or SIGTERM |

```bash
# One-shot: run once, then exit (ideal for cron)
pgwd -db-url "postgres://..." -notifications-slack-webhook "https://..."
# or: PGWD_INTERVAL=0 pgwd

# Daemon: run every N seconds until Ctrl+C or SIGTERM
pgwd -db-url "postgres://..." -interval 60 -notifications-slack-webhook "https://..."

# Dry run: only print stats (total/active/idle), no notifications; no webhook/loki needed
pgwd -db-url "postgres://..." -dry-run
# With interval > 0 (default 60): runs as daemon, prints every interval — Ctrl+C to stop
# With interval 0: runs once and exits — quick connectivity test
pgwd -db-url "postgres://..." -dry-run -interval 0

# Force notification: send a test message to all configured notifiers (no threshold required)
# Use to validate delivery and format before relying on real alerts
pgwd -db-url "postgres://..." -notifications-slack-webhook "https://..." -force-notification
pgwd -db-url "postgres://..." -notifications-loki-url "http://localhost:3100/loki/api/v1/push" -force-notification
```

**Quick test** (after install): `pgwd -dry-run -interval 0` — one check, prints stats, exits. Use config file or `-db-url` + `-config`.

[↑ Back to top](#top)

---

## Typical scenarios

| Scenario | Suggestion |
|----------|------------|
| **Many Postgres instances** | One config per instance; one cron entry per instance. Each instance can have different clusters, thresholds, and environments. No coordination needed; add a new instance = add a cron line. Often more efficient than a daemon when instances are diverse. |
| **Cron check every 5 min** | One-shot (`interval` 0 or unset), one or more thresholds, Slack or Loki. Run from cron every 5 minutes. |
| **Long-running watcher** | Daemon with `-interval 60` (or 120). Run under systemd/supervisor; stop with SIGTERM. |
| **Detect connection leaks** | Use `stale-age` + `threshold-stale` (e.g. 600 and 1). Alert when any connection stays open longer than 10 min. |
| **Pre-production test** | `-dry-run` and low thresholds to see current counts without sending alerts. |
| **Validate notifications** | `-force-notification` with Slack/Loki: sends one test message regardless of thresholds. Use one-shot to confirm delivery, format, and how messages look. (If the connection to Postgres fails, pgwd always sends a connect-failure alert when a notifier is configured.) |
| **Test alerts without low max_connections** | Use `-test-max-connections N` (e.g. 20) with `-force-notification` or low thresholds: thresholds and messages use N as “max_connections”, while stats stay real. Notifications show “(test override)” so total can exceed N. See [docs/testing-alert-levels.md](docs/testing-alert-levels.md) for a procedure to trigger attention/alert/danger against production without changing Postgres config. |
| **Zero config (use defaults)** | Only set `-db-url` and a notifier; pgwd uses 3-tier levels (75,85,95%) by default. Use `-db-threshold-levels` to customize or `-db-default-threshold-percent` when using explicit thresholds. |
| **Multiple environments** | Set `PGWD_*` in env per environment; override `-db-url` or `-notifications-loki-labels` per deploy. |
| **Postgres in Kubernetes** | Use `-kube-postgres namespace/svc/name` (or `namespace/pod/name`). pgwd runs `kubectl port-forward` and connects to localhost. Optionally put `DISCOVER_MY_PASSWORD` in the URL to read the password from the pod's env (e.g. `POSTGRES_PASSWORD`). Requires `kubectl` in PATH. |
| **Alert when Postgres is unreachable** | If you configure a notifier (Slack/Loki), pgwd **always** sends an alert when the connection fails (e.g. refused, timeout, or "too many clients"). No extra flag needed. |

### Running from cron

**One config = one Postgres.** When you have many diverse instances (different clusters, thresholds, kube contexts), cron is often the most efficient approach: one cron entry per instance, each with its own config file. No daemon to manage; add or remove instances by editing crontab.

Cron runs with a **minimal environment** (e.g. `PATH=/usr/bin:/bin`). Two things to keep in mind:

1. **`-kube-postgres` and PATH:** If you use `-kube-postgres`, cron must see `kubectl` in PATH. Set `PATH` in the cron line or in a wrapper script so it includes the directory where `kubectl` lives (e.g. `/usr/local/bin`):

   ```bash
   # In crontab: set PATH before the command
   PATH=/usr/local/bin:/usr/bin:/bin
   */5 * * * * /usr/local/bin/pgwd -kube-postgres default/svc/postgres -db-url "postgres://..." -notifications-slack-webhook "https://..."
   ```

   Or use a wrapper script that exports PATH and runs pgwd:

   ```bash
   #!/bin/sh
   export PATH="/usr/local/bin:$PATH"
   exec /usr/local/bin/pgwd "$@"
   ```

2. **Seeing errors:** If `kubectl` is not found, pgwd exits immediately with a clear message to stderr. Cron often mails stderr to the user; otherwise redirect stdout and stderr to a log file so you can see why the job failed:

   ```bash
   */5 * * * * /usr/local/bin/pgwd -db-url "postgres://..." -notifications-slack-webhook "https://..." >> /var/log/pgwd.log 2>&1
   ```

   Here `>>` appends stdout to the file and `2>&1` sends stderr to the same place.

3. **Log rotation:** When redirecting to a file, it grows indefinitely. Use logrotate to avoid filling disk. Example `/etc/logrotate.d/pgwd`:

   For `/var/log/pgwd.log`:

   ```
   /var/log/pgwd.log {
       daily
       rotate 7
       compress
       missingok
       notifempty
   }
   ```

   For `/home/username/log/pgwd-cron.log` (logs in user home): add `su username groupname` so logrotate runs as the file owner (avoids "insecure permissions" error). Use the same user and group that runs pgwd (e.g. the cron user).

   ```
   /home/username/log/pgwd-cron.log {
       daily
       rotate 7
       compress
       missingok
       notifempty
       su username groupname
   }
   ```

### Example: multiple services and heartbeat via bash + cron

**Simpler: one cron line per instance.** If each instance has its own config file (e.g. `/etc/pgwd/prod-db1.conf`, `/etc/pgwd/analytics.conf`), add one cron entry per config:

```bash
PATH=/usr/local/bin:/usr/bin:/bin
*/5 * * * * pgwd -config /etc/pgwd/prod-db1.conf >> /var/log/pgwd-prod-db1.log 2>&1
*/5 * * * * pgwd -config /etc/pgwd/analytics.conf >> /var/log/pgwd-analytics.log 2>&1
```

Each run is independent; no port clashes when using different configs (each has its own `kube.local_port` if using kube).

**Alternative: single script for many services.** You can run pgwd for several Postgres instances (e.g. one per Kubernetes service) from a single cron schedule: use a bash script that sets `KUBECONFIG`, `PGWD_NOTIFICATIONS_SLACK_WEBHOOK`, and `PATH`, then invokes pgwd once per service with distinct **`-kube-local-port`** values so port-forwards do not clash. Add a second script that runs **`-force-notification`** on a schedule (e.g. every 2 hours) as a “still alive” heartbeat.

**Check script** (e.g. `~/bin/pgwd-cron.sh`): runs every 5 minutes, checks all services, alerts only when thresholds are exceeded.

```bash
#!/bin/bash
mkdir -p ~/log
export KUBECONFIG=/path/to/your/kubeconfig
export PGWD_NOTIFICATIONS_SLACK_WEBHOOK="https://hooks.slack.com/services/..."
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
export PGWD_NOTIFICATIONS_SLACK_WEBHOOK="https://hooks.slack.com/services/..."
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

[↑ Back to top](#top)

---

## Kubernetes

When Postgres runs inside a Kubernetes cluster, use **`-kube-postgres`** so pgwd connects via `kubectl port-forward` (no separate script or manual port-forward).

**Format:** `-kube-postgres <namespace>/<type>/<name>` with `type` = `svc` or `pod`, e.g. `default/svc/postgres` or `default/pod/postgres-0`.

- Set **`PGWD_DB_URL`** with host **`localhost`** and the same port as **`-kube-local-port`** (default 5432). Example: `postgres://user:pass@localhost:5432/mydb`.
- **Password from the pod:** If the URL password is the literal **`DISCOVER_MY_PASSWORD`**, pgwd reads the password from the Postgres pod's environment (`POSTGRES_PASSWORD` by default, or `PGPASSWORD`). Use **`-kube-password-var`** to choose the env var and **`-kube-password-container`** if the Postgres container is not the default.
- **Requires:** `kubectl` in PATH and a valid kubeconfig. pgwd checks for `kubectl` before any kube step and exits with a clear error if it is missing. pgwd starts the port-forward, connects, and stops it on exit. **When running from cron**, set PATH so `kubectl` is findable (see [Running from cron](#running-from-cron) above).
- **Multiple contexts:** If your kubeconfig has several contexts (e.g. dev, staging, prod), use **`-kube-context`** (or `PGWD_KUBE_CONTEXT`) to select which cluster to use. All kubectl operations (port-forward, pod resolution, password discovery, cluster name) use that context.
- **Validate connectivity:** Use **`-validate-k8s-access`** to check kubectl connectivity and list pods before running with `-kube-postgres`. No DB or notifier required. Useful to confirm context and access before a real run.
- **Loki inside the cluster:** When pgwd runs on a host outside the cluster (e.g. VM with cron) and Loki is inside the cluster, use **`-kube-loki namespace/svc/loki`** instead of `-notifications-loki-url`. pgwd runs `kubectl port-forward` to Loki and sends notifications to localhost. Use `-kube-loki-local-port` (default 3100) and `-kube-loki-remote-port` (default 3100) if Loki uses a different port. Mutually exclusive with `-notifications-loki-url` (use one or the other).

```bash
# Validate kubectl connectivity (no DB or notifier needed)
pgwd -validate-k8s-access
# With specific context:
pgwd -kube-context prod -validate-k8s-access

# With password in URL
PGWD_DB_URL="postgres://postgres:secret@localhost:5432/mydb" \
  pgwd -kube-postgres default/svc/postgres -notifications-slack-webhook "https://..." -dry-run

# Password from pod env (POSTGRES_PASSWORD)
PGWD_DB_URL="postgres://postgres:DISCOVER_MY_PASSWORD@localhost:5432/mydb" \
  pgwd -kube-postgres default/svc/postgres -dry-run

# Loki inside cluster (pgwd runs outside): port-forward to Loki, then notify
PGWD_DB_URL="postgres://postgres:DISCOVER_MY_PASSWORD@localhost:5432/mydb" \
  pgwd -kube-postgres default/svc/postgres -kube-loki monitoring/svc/loki -notifications-slack-webhook "https://..." -force-notification

# Port 3100 already in use: use -kube-loki-local-port (like -kube-local-port for Postgres)
pgwd -kube-postgres default/svc/postgres -kube-loki monitoring/svc/loki -kube-loki-local-port 13100 \
  -db-url "postgres://..." -notifications-slack-webhook "https://..." -force-notification

# Grafana/Loki stack (kube-prometheus-stack etc.): match Grafana's X-Scope-OrgId or logs won't appear
# Check your Grafana Loki data source (or Helm values: secureJsonData.httpHeaderValue1 for Loki)
pgwd -kube-postgres mynamespace/svc/postgres -kube-local-port 15432 \
  -kube-loki mynamespace/svc/loki -kube-loki-local-port 13100 \
  -notifications-loki-org-id 1 \
  -db-url 'postgres://postgres:DISCOVER_MY_PASSWORD@localhost:15432/mydb?sslmode=disable' \
  -force-notification
```

[↑ Back to top](#top)

---

## Parameters

All parameters can be set via **config file**, **CLI**, or **environment variables** (`PGWD_*`). Precedence: CLI > env > config file.

| CLI | Env | Description |
|-----|-----|-------------|
| `-config` | `PGWD_CONFIG` | Config file path (YAML). Default `/etc/pgwd/pgwd.conf`. See `contrib/pgwd.conf.example`. |
| `-db-url` | `PGWD_DB_URL` | PostgreSQL connection URL (required). With `-kube-postgres`, use host localhost and port matching `-kube-local-port`. |
| `-kube-postgres` | `PGWD_KUBE_POSTGRES` | Connect via kubectl port-forward: `namespace/type/name` (e.g. `default/svc/postgres`). Requires kubectl in PATH. |
| `-kube-loki` | `PGWD_KUBE_LOKI` | Connect to Loki via kubectl port-forward when Loki is inside the cluster: `namespace/type/name` (e.g. `monitoring/svc/loki`). Mutually exclusive with `-notifications-loki-url`. |
| `-kube-loki-local-port` | `PGWD_KUBE_LOKI_LOCAL_PORT` | Local port for Loki port-forward (default 3100). |
| `-kube-loki-remote-port` | `PGWD_KUBE_LOKI_REMOTE_PORT` | Remote port on the Loki service (default 3100). Use when Loki listens on a different port. |
| `-kube-context` | `PGWD_KUBE_CONTEXT` | Kubectl context to use (empty = current context). Use when you have multiple contexts in kubeconfig and want to target a specific cluster. |
| `-kube-local-port` | `PGWD_KUBE_LOCAL_PORT` | Local port for port-forward (default 5432). Use different ports to run multiple pgwd against different Postgres in the cluster. |
| `-kube-password-var` | `PGWD_KUBE_PASSWORD_VAR` | Pod env var name when URL password is `DISCOVER_MY_PASSWORD` (default `POSTGRES_PASSWORD`). |
| `-kube-password-container` | `PGWD_KUBE_PASSWORD_CONTAINER` | Container name in pod for password discovery (default: primary container). |
| `-validate-k8s-access` | `PGWD_VALIDATE_K8S_ACCESS` | Validate kubectl connectivity and list pods, then exit. Use `-kube-context` to select context. No DB or notifier required. |
| `-client` | `PGWD_CLIENT` | **Required.** Custom name for this monitor instance (e.g. prod-db-primary). Identifies which monitor sent the alert when multiple instances run. Cluster name is computed from kubeconfig when using `-kube-postgres`; not configurable. |
| `-db-threshold-total` | `PGWD_DB_THRESHOLD_TOTAL` | Alert when total connections ≥ N. **Deprecated:** use `-db-threshold-levels`; will be removed in v1.0.0. |
| `-db-threshold-active` | `PGWD_DB_THRESHOLD_ACTIVE` | Alert when active connections ≥ N. **Deprecated:** use `-db-threshold-levels`; will be removed in v1.0.0. |
| `-db-threshold-idle` | `PGWD_DB_THRESHOLD_IDLE` | Alert when idle connections ≥ N |
| `-db-stale-age` | `PGWD_DB_STALE_AGE` | Consider connection stale if open longer than N seconds (requires `-db-threshold-stale`) |
| `-db-threshold-stale` | `PGWD_DB_THRESHOLD_STALE` | Alert when stale connections (open > stale-age) ≥ N |
| `-notifications-slack-webhook` | `PGWD_NOTIFICATIONS_SLACK_WEBHOOK` | Slack Incoming Webhook URL |
| `-notifications-loki-url` | `PGWD_NOTIFICATIONS_LOKI_URL` | Loki push API URL (e.g. `http://localhost:3100/loki/api/v1/push`) |
| `-notifications-loki-labels` | `PGWD_NOTIFICATIONS_LOKI_LABELS` | Loki labels, e.g. `app=pgwd,env=prod` |
| `-notifications-loki-org-id` | `PGWD_NOTIFICATIONS_LOKI_ORG_ID` | Loki `X-Scope-OrgID` header (multi-tenancy). Required for 401; **must match Grafana's Loki data source** or logs won't appear (e.g. `1`, `my-tenant`). |
| `-notifications-loki-bearer-token` | `PGWD_NOTIFICATIONS_LOKI_BEARER_TOKEN` | Loki `Authorization: Bearer` token |
| `-interval` | `PGWD_INTERVAL` | Run every N seconds; 0 = run once |
| `-dry-run` | `PGWD_DRY_RUN` | Only print stats, do not send notifications |
| `-force-notification` | `PGWD_FORCE_NOTIFICATION` | Always send at least one notification: test event when connected (to validate delivery, format, and channel). Requires at least one notifier. (Connection failure is always notified when a notifier is configured, with or without this flag.) |
| `-notify-on-connect-failure` | `PGWD_NOTIFY_ON_CONNECT_FAILURE` | Legacy: connection failure is **always** notified when a notifier is configured; this flag is no longer required. Kept for backward compatibility; if set, still requires at least one notifier at startup. |
| `-db-default-threshold-percent` | `PGWD_DB_DEFAULT_THRESHOLD_PERCENT` | When one of total/active is 0, set it to this % of max_connections (1–100). Default: 80. Ignored when using db-threshold-levels mode. |
| `-db-threshold-levels` | `PGWD_DB_THRESHOLD_LEVELS` | When both total and active are 0: comma-separated percentages for 3-tier alerts (e.g. 75,85,95). Levels: attention (1st), alert (2nd), danger (3rd). Only highest breached level fires. Default: 75,85,95. |
| `-test-max-connections` | `PGWD_TEST_MAX_CONNECTIONS` | Override server `max_connections` for threshold defaults and display (testing only). When set, defaults and notifications use this value instead of the server’s; stats (total/active/idle) remain real. Notifications show “(test override)” so you can simulate e.g. a low limit and trigger alerts without a real low max_connections. |

**Stale connections:** A connection is "stale" if it has been open longer than `stale-age` seconds (based on `backend_start` in `pg_stat_activity`). Use this to detect leaks or connections that are never closed. When using `threshold-stale`, `stale-age` must be set and > 0.

**Default thresholds:** If you do not set `-db-threshold-total` or `-db-threshold-active` (leave both 0), pgwd uses **3-tier level mode** with **`-db-threshold-levels`** (default **75,85,95**). At 75% of max_connections → attention (yellow); at 85% → alert (orange); at 95% → danger (red). Only the highest breached level fires. Use `-db-threshold-levels 70,80,90` to customize. If you set one of total/active explicitly, the other defaults from **`-db-default-threshold-percent`** (default 80). Idle and stale have no default (0 = disabled). The DB user must be able to read `max_connections` (any normal role can).

[↑ Back to top](#top)

---

## Install

**From source (recommended):**

```bash
go install github.com/hrodrig/pgwd@latest
```

This installs the binary to `$GOBIN` (default `$HOME/go/bin`). Ensure `$GOBIN` is on your `PATH`.

**One-liner (Linux, macOS, BSD) — installs latest release:**

```bash
curl -sSL https://raw.githubusercontent.com/hrodrig/pgwd/main/scripts/install.sh | bash
```

**Package managers:**

| Platform | Command |
|----------|---------|
| **Homebrew (macOS)** | `brew install hrodrig/pgwd/pgwd` |
| **Debian/Ubuntu** | `wget -q -O /tmp/pgwd.deb https://github.com/hrodrig/pgwd/releases/download/v0.5.0/pgwd_v0.5.0_linux_amd64.deb && sudo dpkg -i /tmp/pgwd.deb` |
| **Fedora/RHEL** | `sudo dnf install https://github.com/hrodrig/pgwd/releases/download/v0.5.0/pgwd_v0.5.0_linux_amd64.rpm` |
| **Alpine** | `wget -qO- https://github.com/hrodrig/pgwd/releases/download/v0.5.0/pgwd_v0.5.0_linux_amd64.tar.gz \| tar -xzf - -C /usr/local/bin` — see [Alpine (OpenRC)](#alpine-linux-openrc) |
| **OpenBSD** | tarball with rc.d: see [OpenBSD](#openbsd) |
| **FreeBSD** | port or tarball: see [FreeBSD](#freebsd) |
| **NetBSD** | tarball with rc.d: see [NetBSD](#netbsd) |
| **DragonFly BSD** | tarball with rc.d: see [DragonFly BSD](#dragonfly-bsd) |

Replace `v0.5.0` and `amd64` with your desired version and arch (e.g. `arm64`). See [Releases](https://github.com/hrodrig/pgwd/releases) for all assets.

**Pre-built binaries:** [Releases](https://github.com/hrodrig/pgwd/releases) provide binaries (tar.gz, zip), `.deb`, and `.rpm` packages for Linux, macOS, and Windows (amd64 and arm64). The `.deb` and `.rpm` packages include the man page (`man pgwd`) and install `/etc/pgwd/pgwd.conf` (edit before use).

## Build

```bash
go build -o pgwd ./cmd/pgwd
# or use the Makefile:
make build
make install
# Custom install path: GOBIN=~/bin make install  (default is $HOME/go/bin)
# Install man page: make install-man  (MANDIR=/usr/share/man for system-wide)
```

**Release (GitHub):** See [Release steps](#release-steps) below for the full workflow. Quick: from `main`, `git tag v0.5.0`, `make release`. Requires [goreleaser](https://goreleaser.com) (`brew install goreleaser`). For a local snapshot build without publishing: `make snapshot` (outputs to `dist/`).

### Release steps

Example: releasing **v1.0.0**. Copy, adjust the version and token, then run.

**1. Prerequisites** (install once):

```bash
brew install goreleaser grype
# Docker: required for test-integration, docker-scan, and E2E tests (kind, test-e2e-kube)
```

**2. On `develop`** — ensure everything is committed and checks pass:

```bash
git checkout develop
git pull origin develop

# Mandatory checks (all must pass)
make release-check
# Runs: lint, test, test-integration, docker-scan
```

**3. Update version** — edit `VERSION` and `CHANGELOG.md`:

```bash
echo "1.0.0" > VERSION
# Edit CHANGELOG.md: move [Unreleased] items into [1.0.0], update compare links
git add VERSION CHANGELOG.md README.md  # README badge if needed
git commit -m "Release 1.0.0"
git push origin develop
```

**4. Merge to `main`** and tag:

```bash
git checkout main
git pull origin main
git merge develop
git push origin main

git tag -a v1.0.0 -m "Release 1.0.0"
git push origin v1.0.0
```

**5. Publish release** — requires tokens:

```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export HOMEBREW_TAP_TOKEN="ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
make release
```

- **GITHUB_TOKEN:** `repo` scope — GitHub release, Docker push to ghcr.io.
- **HOMEBREW_TAP_TOKEN:** `repo` scope — pushes the Homebrew cask to the tap (`hrodrig/homebrew-pgwd`). Can be the same token as `GITHUB_TOKEN`.

Use a [Personal Access Token](https://github.com/settings/tokens) with `repo` scope. Before releasing, verify each token's **expiration date** and **scopes** at [github.com/settings/tokens](https://github.com/settings/tokens).

**Snapshot (no publish):** `make snapshot` — outputs to `dist/` without pushing.

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

**With a notifier** (e.g. Slack): add `-notifications-slack-webhook "https://..."` (default 3-tier levels 75,85,95%).

**Stop and remove:**

```bash
docker stop pgwd-pg && docker rm pgwd-pg
```

Using **127.0.0.1** and host port **5433** avoids hitting a local Postgres on 5432 and avoids IPv6 resolution quirks.

**Who is on 5432?** Run `lsof -i :5432`. If you see both `postgres` (local) and `com.docker` (Docker), connections to **localhost:5432** go to the local postgres (it binds to localhost); the container is on `*:5432`. Use host port **5433** for the container so your client clearly reaches the container.

## Requirements

- At least one of: a threshold (`-db-threshold-levels` for 3-tier, `-db-threshold-idle`, or `-db-threshold-stale` with `-db-stale-age`), `-dry-run`, or `-force-notification`. If you set only `-db-url` and a notifier, pgwd uses 3-tier levels (75,85,95%) of `max_connections`.
- If not using `-dry-run`: at least one notifier (`-notifications-slack-webhook`, `-notifications-loki-url`, or `-kube-loki`). For `-force-notification`, a notifier is required.
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

**Man page:** When installed via `.deb` or `.rpm`, run `man pgwd` for the full manual. From source: `make install-man` (or `MANDIR=/usr/share/man make install-man` for system-wide).

## Slack

Create an [Incoming Webhook](https://api.slack.com/messaging/webhooks) in your Slack workspace and set `PGWD_NOTIFICATIONS_SLACK_WEBHOOK` or `-notifications-slack-webhook`.

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

**3-tier levels:** When using `-db-threshold-levels` (or when level is derived from percentage), Slack shows distinct colors and emojis: **attention** (yellow bar, yellow circle), **alert** (orange bar, orange circle), **danger** (red bar, red circle).

## Loki

Set the Loki push endpoint URL (e.g. `http://loki:3100/loki/api/v1/push`). Optionally set `PGWD_NOTIFICATIONS_LOKI_LABELS` for stream labels (e.g. `app=pgwd,env=prod`); default includes `app=pgwd`.

**Auth:** If Loki returns `401 Unauthorized`, set `-notifications-loki-org-id` (e.g. `1`) for multi-tenancy, or `-notifications-loki-bearer-token` if your Loki requires auth (env: `PGWD_NOTIFICATIONS_LOKI_ORG_ID`, `PGWD_NOTIFICATIONS_LOKI_BEARER_TOKEN`).

**Grafana / Loki stacks (kube-prometheus-stack, etc.):** Grafana's Loki data source is often provisioned with a specific `X-Scope-OrgId` (e.g. `1`, `my-tenant`). **pgwd must use the same org ID** or logs will not appear in Grafana. Check your Grafana Loki data source config (or Helm values: `grafana.additionalDataSources` → Loki → `secureJsonData.httpHeaderValue1`). Use `-notifications-loki-org-id <value>` to match.

**Notification format:** Each alert is one log line in a stream. The stream has labels from `PGWD_NOTIFICATIONS_LOKI_LABELS` plus `app=pgwd` (if not set), `threshold`, `level` (attention/alert/danger), `namespace` (when using `-kube-postgres`), `database`, `cluster`, and `client` (when set). The log line includes database, cluster, and client at the start when available:

Filter by instance: `{app="pgwd", client="my-monitor"}` in Grafana.

```
pgwd [cluster=<Cluster>] [database=<Database>] [client=<Client>]: <Message> | total=<Total> active=<Active> idle=<Idle> (limit <Threshold>=<ThresholdValue>)
```

Example: `pgwd [cluster=prod] [database=myapp] [client=pgwd-vps-01]: Test notification — delivery check (force-notification). | total=33 active=1 idle=32 max_connections=2048 (delivery check)`

Same placeholders as Slack. Timestamp is the time of the push. You can query in Grafana or LogCLI by label (e.g. `{app="pgwd", threshold="total"}` or `{app="pgwd", level="danger"}`). For Grafana alert rules, see [docs/loki-grafana-alerts.md](docs/loki-grafana-alerts.md) (labels, LogQL examples, payload structure).

---

## Troubleshooting

| Symptom | What to check |
|--------|----------------|
| **"missing database URL"** | Set `PGWD_DB_URL` or `-db-url`. The URL must be a valid [PostgreSQL connection string](https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING). |
| **"no thresholds set and could not default from server..."** | pgwd could not read `max_connections` from the server (error or 0). Use `-test-max-connections N` to override, or `-dry-run`, or `-force-notification`. With a normal Postgres, only `-db-url` and a notifier should be enough (defaults to 3-tier levels 75,85,95%). |
| **"no notifier configured"** | Set `PGWD_NOTIFICATIONS_SLACK_WEBHOOK`, `PGWD_NOTIFICATIONS_LOKI_URL`, or `PGWD_KUBE_LOKI` (or use `-dry-run` to skip notifications). |
| **"force-notification requires at least one notifier"** | Use `-force-notification` together with `-notifications-slack-webhook` and/or `-notifications-loki-url` or `-kube-loki`. |
| **"notify-on-connect-failure requires at least one notifier"** | You set `-notify-on-connect-failure` but have no notifier. Add `-notifications-slack-webhook` and/or `-notifications-loki-url` or `-kube-loki`. (Connect failure is always notified when a notifier is configured; the flag is optional.) |
| **"kubectl not found in PATH"** | When using `-kube-postgres` or `-kube-loki`, ensure `kubectl` is installed and on your `PATH` (e.g. `which kubectl`). pgwd exits with this message before attempting port-forward or password discovery. |
| **"when using -db-threshold-stale, -db-stale-age must be > 0"** | Set `-db-stale-age N` (e.g. 600) when using `-db-threshold-stale`. |
| **Slack/Loki not receiving alerts** | Run once with `-force-notification` to send a test message. Check webhook URL, network/firewall, and that the app can reach Slack/Loki. |
| **Loki: 401 Unauthorized** | Loki requires auth. Set `-notifications-loki-org-id 1` (multi-tenancy) or `-notifications-loki-bearer-token <token>` (or env `PGWD_NOTIFICATIONS_LOKI_ORG_ID` / `PGWD_NOTIFICATIONS_LOKI_BEARER_TOKEN`). |
| **Logs sent to Loki but not visible in Grafana** | Grafana queries a specific tenant. pgwd must use the **same** `-notifications-loki-org-id` as Grafana's Loki data source (e.g. `1`, `my-tenant`). Check Grafana data source config or Helm values (`secureJsonData.httpHeaderValue1` for Loki). |
| **"postgres connect: ..."** | DB unreachable: check host, port, TLS, credentials, and that the pgwd host can reach the Postgres server. |
| **Stats or stale count errors in logs** | Permissions: the DB user must be able to read `pg_stat_activity` (usually any role can). Check `log.Printf` output for the exact error. |

[↑ Back to top](#top)

---

## FAQ

<details>
<summary><strong>What is max_connections and why does pgwd need it?</strong></summary>

pgwd uses `max_connections` (from Postgres) to compute percentage-based thresholds. With `-db-threshold-levels 75,85,95`, at 75% of max_connections you get an "attention" alert, at 85% an "alert", at 95% "danger". If pgwd cannot read it (e.g. restricted role), use `-test-max-connections N` to override for testing.
</details>

<details>
<summary><strong>Can I run pgwd from cron?</strong></summary>

Yes. Use one-shot mode (`PGWD_INTERVAL=0` or omit it). Run pgwd every 5 minutes (or your preferred interval). Ensure `PATH` includes `kubectl` if you use `-kube-postgres`. See [Running from cron](#running-from-cron) for details and log rotation.
</details>

<details>
<summary><strong>Can I monitor multiple Postgres instances?</strong></summary>

Yes. One config file = one Postgres. For many diverse instances (different clusters, thresholds, kube contexts), cron is often the most efficient: one cron entry per instance, each with its own config (`-config /etc/pgwd/instance-name.conf`). No coordination needed; add a new instance = add a cron line. See [Example: multiple services](#example-multiple-services-and-heartbeat-via-bash--cron).
</details>

<details>
<summary><strong>How do I validate Slack/Loki before going live?</strong></summary>

Use `-force-notification`: pgwd sends one test message to all configured notifiers regardless of thresholds. Run once to confirm delivery, format, and that messages look correct in your channel.
</details>

<details>
<summary><strong>Postgres is in Kubernetes — how do I connect?</strong></summary>

Use `-kube-postgres namespace/svc/name` (e.g. `default/svc/postgres`). pgwd runs `kubectl port-forward` and connects to localhost. Validate first with `-validate-k8s-access`. See [Kubernetes](#kubernetes).
</details>

<details>
<summary><strong>Loki is inside the cluster — what if pgwd runs outside?</strong></summary>

Use `-kube-loki namespace/svc/loki` (e.g. `monitoring/svc/loki`). pgwd runs `kubectl port-forward` to Loki (port 3100) and sends notifications to localhost. Mutually exclusive with `-notifications-loki-url`; use one or the other. See [Kubernetes](#kubernetes).
</details>

<details>
<summary><strong>Logs sent to Loki but not visible in Grafana — why?</strong></summary>

Loki uses multi-tenancy: each `X-Scope-OrgId` is a separate tenant. Grafana's Loki data source is provisioned with a specific org ID (e.g. `1`, `my-tenant`). pgwd must use the **same** value via `-notifications-loki-org-id` or `PGWD_NOTIFICATIONS_LOKI_ORG_ID`. Check your Grafana Loki data source config (or Helm values: `grafana.additionalDataSources` → Loki → `secureJsonData.httpHeaderValue1`).
</details>

<details>
<summary><strong>What are the 3-tier levels (attention / alert / danger)?</strong></summary>

When you use `-db-threshold-levels 75,85,95` (default), pgwd fires one alert per run at the highest breached level: 75% → attention (yellow), 85% → alert (orange), 95% → danger (red). Slack and Loki show distinct colors/emojis per level.
</details>

[↑ Back to top](#top)

---

## Docker

**Published image (each release):** Multi-arch images (linux/amd64, linux/arm64) are published to [GitHub Container Registry](https://github.com/hrodrig/pgwd/pkgs/container/pgwd) as `ghcr.io/hrodrig/pgwd`. Use a version tag or `latest`:

```bash
docker pull ghcr.io/hrodrig/pgwd:v0.5.0
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

Use the published image `ghcr.io/hrodrig/pgwd:latest` (or `:v0.5.0`), or `pgwd` if you built locally with `make docker-build`:

```bash
# Help (no DB needed)
docker run --rm ghcr.io/hrodrig/pgwd:latest -h

# Version (should show e.g. pgwd v0.5.0 (commit ..., built ...))
docker run --rm ghcr.io/hrodrig/pgwd:latest --version

# Expect "missing database URL" (validates startup path)
docker run --rm ghcr.io/hrodrig/pgwd:latest
```

**Run (one-shot or daemon)**

```bash
# One-shot: pass env and ensure network to Postgres (and Slack/Loki if used)
docker run --rm \
  -e PGWD_DB_URL="postgres://user:pass@host.docker.internal:5432/mydb" \
  -e PGWD_NOTIFICATIONS_SLACK_WEBHOOK="https://hooks.slack.com/..." \
  ghcr.io/hrodrig/pgwd:latest

# Daemon (interval 60s)
docker run --rm -d --name pgwd \
  -e PGWD_DB_URL="postgres://user:pass@host.docker.internal:5432/mydb" \
  -e PGWD_NOTIFICATIONS_SLACK_WEBHOOK="https://hooks.slack.com/..." \
  -e PGWD_INTERVAL=60 \
  ghcr.io/hrodrig/pgwd:latest
```

Use `host.docker.internal` (or your host IP) to reach Postgres on the host from the container. For secrets, prefer env files or a secrets manager instead of hardcoding in the image.

[↑ Back to top](#top)

---

## systemd

pgwd uses a **config file** as the single source. When the config file is loaded, env vars (PGWD_*) are ignored. Use `-config` to specify a custom path.

**Convention**

| What | Path |
|------|------|
| Binary | `/usr/bin/pgwd` (.deb/.rpm) or `/usr/local/bin/pgwd` (manual) |
| Config file | `/etc/pgwd/pgwd.conf` — installed by .deb/.rpm; from source, copy `contrib/pgwd.conf.example` |

Restrict permissions if the config contains secrets: `sudo chmod 600 /etc/pgwd/pgwd.conf`.

**Units** (.deb/.rpm install to `/lib/systemd/system/`)

| Unit | Function | When to use |
|------|----------|-------------|
| `pgwd.service` | Daemon — runs continuously, checks every `interval` seconds from config | Continuous monitoring (e.g. every 60 s) |
| `pgwd-once.service` | One-shot — runs pgwd once and exits. Used by the timer | Do not enable directly |
| `pgwd.timer` | Schedule — triggers pgwd-once every 5 minutes (1 min after boot) | Cron-like: one check every 5 min |

**Two ways to run:** daemon (`pgwd.service`) or timer (`pgwd.timer`). See [contrib/systemd/README.md](contrib/systemd/README.md) for setup details.

**Daemon (long-running)**

```bash
# .deb/.rpm: dpkg -i pgwd_*_linux_amd64.deb — installs binary, config, and systemd units
# Edit config: sudo nano /etc/pgwd/pgwd.conf (client, db.url, notifications, etc.)
sudo systemctl daemon-reload
sudo systemctl enable --now pgwd
journalctl -u pgwd -f
```

From source:

```bash
sudo cp pgwd /usr/local/bin/pgwd
sudo mkdir -p /etc/pgwd
sudo cp contrib/pgwd.conf.example /etc/pgwd/pgwd.conf
# Edit /etc/pgwd/pgwd.conf
sudo cp contrib/systemd/pgwd.service /etc/systemd/system/
# If /usr/local/bin: edit unit, set ExecStart=/usr/local/bin/pgwd
sudo systemctl daemon-reload
sudo systemctl enable --now pgwd
```

**One-shot from a timer (cron-like)**

Runs pgwd once every 5 minutes (the timer is the schedule; set `interval: 0` in config or omit).

```bash
# .deb/.rpm: units already installed. Just enable the timer:
sudo systemctl daemon-reload
sudo systemctl enable --now pgwd.timer
systemctl list-timers --all | grep pgwd
```

From source: copy `contrib/systemd/pgwd-once.service` and `contrib/systemd/pgwd.timer` to `/etc/systemd/system/`, then enable the timer.

To change the interval, edit the timer: `OnUnitActiveSec=5min` → e.g. `OnUnitActiveSec=10min`, then `sudo systemctl daemon-reload`.

**Optional:** Run the service as a dedicated user: create `useradd -r -s /bin/false pgwd`, then in the unit add `User=pgwd` and `Group=pgwd`. Ensure that user can read the config file.

[↑ Back to top](#top)

---

## Alpine Linux (OpenRC)

Alpine uses **OpenRC** (rc.d), not systemd. Config: `/etc/pgwd/pgwd.conf`.

**Install** — tar.gz (binario estático, musl-compatible):

```bash
wget -qO- https://github.com/hrodrig/pgwd/releases/download/v0.5.0/pgwd_v0.5.0_linux_amd64.tar.gz | tar -xzf - -C /usr/local/bin
# arm64: replace amd64 with arm64
```

**When available in aports:** `apk add pgwd` (installs binary, config, and OpenRC init script).

**Daemon (OpenRC)**

```bash
# Config (required)
sudo mkdir -p /etc/pgwd
sudo cp contrib/pgwd.conf.example /etc/pgwd/pgwd.conf
sudo nano /etc/pgwd/pgwd.conf  # client, db.url, etc.

# Init script (from tarball; apk installs it automatically)
sudo cp contrib/openrc/pgwd.initd /etc/init.d/pgwd
sudo chmod +x /etc/init.d/pgwd

# Start and enable on boot
rc-service pgwd start
rc-update add pgwd default
```

See [contrib/openrc/README.md](contrib/openrc/README.md) for details.

[↑ Back to top](#top)

---

## OpenBSD

OpenBSD uses **rc.d**, not systemd. Config: `/etc/pgwd/pgwd.conf`. Supports `-kube-postgres` and `-kube-loki` (external VPS with kubeconfig; see [contrib/openbsd/README.md](contrib/openbsd/README.md)).

**Install** — tarball includes binary, rc.d script, and config example:

```bash
tar xzf pgwd_v0.5.4_openbsd_amd64.tar.gz
doas install -m755 pgwd /usr/local/bin/
doas install -m555 share/openbsd/rc.d/pgwd /etc/rc.d/pgwd
doas mkdir -p /etc/pgwd
doas cp etc/pgwd/pgwd.conf.example /etc/pgwd/pgwd.conf
doas vi /etc/pgwd/pgwd.conf  # client, db.url, etc.
doas rcctl enable pgwd
doas rcctl start pgwd
```

See [contrib/openbsd/README.md](contrib/openbsd/README.md) for details.

[↑ Back to top](#top)

---

## FreeBSD

FreeBSD uses **ports** or a pre-built tarball. Config: `/etc/pgwd/pgwd.conf`. Supports `-kube-postgres` and `-kube-loki` (external VPS with kubeconfig; see [contrib/freebsd/README.md](contrib/freebsd/README.md)).

**Install from port** (when available in official ports):

```bash
cd /usr/ports/sysutils/pgwd
make install
```

**Install from local port** (before it is in official ports):

```bash
# Clone ports, copy pgwd port, then:
cd ~/ports/sysutils/pgwd
make install
```

**Install from tarball** (or use the [one-liner](#install) which works on FreeBSD and installs only the binary):

```bash
fetch -o /tmp/pgwd.tgz https://github.com/hrodrig/pgwd/releases/download/v0.5.7/pgwd_v0.5.7_freebsd_amd64.tar.gz
tar -xzf /tmp/pgwd.tgz -C /tmp
sudo install -m755 /tmp/pgwd /usr/local/bin/
sudo mkdir -p /usr/local/etc/pgwd
sudo install -m444 /tmp/etc/pgwd/pgwd.conf.example /usr/local/etc/pgwd/
# arm64: replace amd64 with arm64 in the URL
```

**Config** (required for port or tarball):

```bash
mkdir -p /etc/pgwd
cp /usr/local/etc/pgwd/pgwd.conf.example /etc/pgwd/pgwd.conf
vi /etc/pgwd/pgwd.conf  # client, db.url, etc.
```

The port installs the binary, man page, LICENSE, config example, and rc.d script to `/usr/local/`. Run as daemon: `echo 'pgwd_enable="YES"' >> /etc/rc.conf && service pgwd start`. See [contrib/freebsd/README.md](contrib/freebsd/README.md) for details.

[↑ Back to top](#top)

---

## NetBSD

NetBSD uses **rc.d**, not systemd. Config: `/etc/pgwd/pgwd.conf`. Supports `-kube-postgres` and `-kube-loki` (external host with kubeconfig; see [contrib/netbsd/README.md](contrib/netbsd/README.md)).

**Install** — tarball includes binary, rc.d script, and config example:

```bash
tar xzf pgwd_v0.5.7_netbsd_amd64.tar.gz
install -m755 pgwd /usr/local/bin/
install -m555 share/netbsd/rc.d/pgwd /etc/rc.d/pgwd
mkdir -p /etc/pgwd
cp etc/pgwd/pgwd.conf.example /etc/pgwd/pgwd.conf
vi /etc/pgwd/pgwd.conf  # client, db.url, etc.
echo 'pgwd=YES' >> /etc/rc.conf
service pgwd start
```

See [contrib/netbsd/README.md](contrib/netbsd/README.md) for details.

[↑ Back to top](#top)

---

## DragonFly BSD

[DragonFly BSD](https://www.dragonflybsd.org) uses **rc.d**, not systemd. Config: `/etc/pgwd/pgwd.conf`. Supports `-kube-postgres` and `-kube-loki` (external host with kubeconfig; see [contrib/dragonflybsd/README.md](contrib/dragonflybsd/README.md)).

**Install** — tarball includes binary, rc.d script, and config example:

```bash
tar xzf pgwd_v0.5.7_dragonfly_amd64.tar.gz
install -m755 pgwd /usr/local/bin/
install -m555 share/dragonfly/rc.d/pgwd /etc/rc.d/pgwd
mkdir -p /etc/pgwd
cp etc/pgwd/pgwd.conf.example /etc/pgwd/pgwd.conf
vi /etc/pgwd/pgwd.conf  # client, db.url, etc.
echo 'pgwd_enable="YES"' >> /etc/rc.conf
service pgwd start
```

See [contrib/dragonflybsd/README.md](contrib/dragonflybsd/README.md) for details.

[↑ Back to top](#top)

---

## Roadmap

Target **v1.0.0** by early July.

| Version | Target | Scope |
|---------|--------|-------|
| **0.4.0** | Mar 2026 ✅ | Loki auth (-notifications-loki-org-id, -notifications-loki-bearer-token), kube-loki, Grafana org ID docs, notification sent log. |
| **0.5.0** | Mar 2026 ✅ | Loki database/cluster labels and log line, Grafana alert docs, security hardening (zlib, compose, k8s). |
| **0.6.0** | May | **CSV metrics** — save time series to file. |
| **0.7.0** | May–Jun | **DB metrics** — save to database (PostgreSQL/TimescaleDB). Last 0.x before 1.0. |
| **1.0.0** | Early Jul | **Breaking:** remove threshold-total and threshold-active. Stable API. Criteria: 100+ tests, logo, deprecations removed. |

[↑ Back to top](#top)

---

## Get involved

Found pgwd useful? We’d love your help to make it better. You can:

- **Report bugs** or **suggest features** — [open an issue](https://github.com/hrodrig/pgwd/issues)
- **Contribute code** — see [CONTRIBUTING.md](CONTRIBUTING.md) for how to submit a pull request
- **Star the repo** — it helps others discover pgwd

Thanks for using pgwd. Happy watching.
