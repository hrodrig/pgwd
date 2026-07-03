# pgwd specifications (behavior contract)

## 1. Purpose

`pgwd` (Postgres Watch Dog) is a Go CLI that **monitors PostgreSQL connection counts** (total, active, idle, stale) and optionally alerts on **long-running queries**. When configured thresholds are exceeded, it notifies via configured channels (Slack, Loki, PagerDuty, Microsoft Teams, and/or generic webhook). It can run as a **one-shot check** (for cron or ad-hoc) or as a **daemon** (recurring interval).

This document is the source of truth for **observable behavior** and test expectations (**baseline: v0.7.0** shipped code; deprecated and planned items are labeled). **Roadmap:** [ROADMAP.md](ROADMAP.md). Band plans: [docs/plan-0.7.x.md](docs/plan-0.7.x.md) → [docs/plan-1.0.x.md](docs/plan-1.0.x.md). Shipped releases: **[CHANGELOG.md](CHANGELOG.md)**.

## 2. Scope

### In scope

- One binary: **`pgwd`** with CLI flags, environment variables (`PGWD_*`), and YAML config (`pgwd.conf`).
- Database targets: single-DB (`-db-url`) or multi-DB (`databases:` in config).
- Kubernetes: optional **port-forward** to Postgres via `-kube-postgres` (client-go; kubeconfig required, no kubectl binary). Also optional **port-forward to Loki** via `-kube-loki`.
- Thresholds:
  - **Explicit** — `-db-threshold-total` / `-db-threshold-active` / `-db-threshold-idle` / `-db-threshold-stale`
  - **3-tier level mode** — `-db-threshold-levels` (default `75,85,95`): attention, alert, danger.
  - **Long-running query alerts** — `-db-long-query-min-seconds` (requires a metrics store for cooldown).
- Notifications: **Slack** Incoming Webhook, **Loki** push API (with org ID, bearer token, kube port-forward), **PagerDuty** Events v2, **Microsoft Teams** incoming webhook, **generic webhook** (custom headers, HMAC, JSON template). Shared HTTP retry/backoff for all outbound notifier calls.
- Metrics persistence: **SQLite** (default) or **PostgreSQL/MySQL** (`metrics_store.driver` + `metrics_store.dsn`). Used for hysteresis (confirm-alert/confirm-ok), resolution notifications, long-query cooldown, and HTTP `/metrics` endpoint.
- HTTP server: optional `/metrics` and `/healthz` endpoints for Kubernetes probes.
- CSV export: one-shot dump of persisted metrics via `-export-metrics-format csv`.
- System packages: `.deb`, `.rpm`, Homebrew cask, FreeBSD/OpenBSD/NetBSD ports, Solaris SMF.
- Rootless Docker image (Alpine, non-root user `pgwd`).

### Out of scope (v1)

- Mutating Postgres (vacuum, terminate, alter). Read-only monitoring only.
- Full SQL monitoring (slow query log parsing, index analysis, table bloat).
- Built-in dashboard/GUI. Metrics via HTTP `/metrics` (Prometheus text exposition), Loki/Grafana, or CSV export.
- Multi-cluster monitoring from one process (one pgwd per Postgres target).
- Arbitrary notifier plugin system (channels are compiled-in).
- PostgreSQL replication lag monitoring.

### Design principles

- **Simple config-first:** YAML at `/etc/pgwd/pgwd.conf`; **CLI** overrides file when set; **`PGWD_*` env** applies only when no config file is loaded. Multi-DB via `databases:` in config.
- **Fail fast on config errors** — validate at startup; exit with a clear message.
- **Connection failure is always notified** when any notifier is configured (no extra flag needed).
- **3-tier level mode** by default (75/85/95%) — no mandatory threshold arithmetic.
- **Dry-run** mode prints what would happen without sending notifications.
- **Honest config:** every field is documented; behavior matches code and this spec.

## 3. CLI contract

### Commands

| Command / Flag | Behavior |
|----------------|----------|
| `pgwd` (no args, no env) | Loads config from default path, runs once or daemon. Prints help on config errors. |
| `pgwd -version` / `--version` | `pgwd vX.Y.Z (branch …, commit …, built …)` to stdout, exit 0. |
| `pgwd version` | Same output as `--version`. |
| `pgwd -config <path>` | Explicit YAML config path. |
| `pgwd --print-sample-config` | Writes annotated sample YAML to **stdout** and exits 0. |
| `pgwd -dry-run` | Runs normally but skips all outbound HTTP notifications (Slack, Loki, PagerDuty, Teams, generic webhook, etc.). Logs every event as `[dry-run] would send: …`. |
| `pgwd -force-notification` | Sends one test event to all configured notifiers after a successful Postgres connect, regardless of thresholds. |
| `pgwd -validate-k8s-access` | Validates cluster connectivity (lists pods), then exits. Uses `-kube-context` when set. Does not require `-kube-postgres`. |
| `pgwd -export-metrics-format csv -export-metrics-destination <path>` | Dumps persisted metrics from the configured store to a CSV file, then exits. |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (no thresholds exceeded, or dry-run, or force-notification delivered, or export complete) |
| 1 | Config validation error (missing required flags, invalid combo, no notifier + no dry-run) |
| 2 | Postgres connection failure (or too many clients) after config is valid |
| 3 | Postgres query error during stats collection |
| 4 | **Not currently used.** Reserved for notify delivery failure (when at least one sender fails). |

### Persistent flags

| Flag | Env | Effect |
|------|-----|--------|
| `-config <path>` | `PGWD_CONFIG` | Config file path. Default `/etc/pgwd/pgwd.conf`. |
| `-db-url <url>` | `PGWD_DB_URL` | PostgreSQL connection URL. Required unless `-kube-postgres` or `databases:` in config. |
| `-client <name>` | `PGWD_CLIENT` | **Required.** Monitor identity label. |
| `-interval <sec>` | `PGWD_INTERVAL` | Daemon interval; 0 = run once. Default 0. |
| `-dry-run` | `PGWD_DRY_RUN` | No outbound notifications; log events locally. |
| `-force-notification` | `PGWD_FORCE_NOTIFICATION` | Send a test event regardless of thresholds. |
| `-notify-on-connect-failure` | `PGWD_NOTIFY_ON_CONNECT_FAILURE` | Legacy; currently always-on when notifiers configured. |
| `-log-level <level>` | `PGWD_LOG_LEVEL` | `info` (default) or `debug`. |
| `-test-max-connections <n>` | `PGWD_TEST_MAX_CONNECTIONS` | Override server `max_connections` for testing alerts. |
| `-validate-k8s-access` | `PGWD_VALIDATE_K8S_ACCESS` | Connectivity probe, then exit. |

## 4. Configuration contract

### Load order

1. **Config file** (`/etc/pgwd/pgwd.conf` by default, or `-config` / `PGWD_CONFIG`). If the file exists and loads, YAML is the base config. **`PGWD_*` env vars are not applied** (`ApplyEnv` is skipped; debug log: `ignored (config file is source)`).
2. **No config file** — `ApplyDefaults`, then **`ApplyEnv`** (`PGWD_*` overrides defaults).
3. **CLI flags** — parsed last; highest precedence.

**Precedence:** when a file is loaded, **CLI > file > defaults** (env ignored). When no file is loaded, **CLI > env > defaults**.

### `databases:` (multi-DB mode)

When `databases:` is non-empty in the config file, each entry is one Postgres target with its own thresholds, client name, and long-query settings. Entry fields:

| Key | Type | Default | Notes |
|-----|------|---------|-------|
| `url` | string | — | Required. Postgres DSN. |
| `client` | string | `base_client + "-" + dbname` | Monitor identity. Must be unique across entries when same DB name appears on different hosts (SQLite keys on `(client, cluster, database)`). |
| `stale_age` | int | `config.StaleAge` | Seconds. 0 = no stale detection. |
| `default_threshold_percent` | int | `config.DefaultThresholdPercent` | 1-100. |
| `threshold_total`, `threshold_active`, `threshold_idle`, `threshold_stale` | int | Config-level values | Per-target override. |
| `threshold_levels` | string | Config-level value | e.g. `75,85,95`. |
| `long_query_min_seconds` | int | Config-level value | 0 = disabled. |
| `long_query_cooldown_seconds` | int | Config-level value | Default 3600 when min is set. |
| `long_query_min_count` | int | Config-level value | Default 1. |

`kube.postgres` is **not supported** with `databases:` (multi-DB requires direct URLs; single-DB + kube until per-db kube exists).

### Top-level config keys (config file)

| Key | Type | Default | Notes |
|-----|------|---------|-------|
| `db` | object | — | **Deprecated** single-DB config (removed in v1.0). Warning printed if used. |
| `databases` | array | — | Multi-DB targets. When set, `db` is ignored. |
| `kube.postgres` | string | — | `namespace/type/name` for port-forward. |
| `kube.context` | string | — | Kubectl context override. |
| `kube.local_port` | int | `5432` | Local port. |
| `kube.password_var` | string | `POSTGRES_PASSWORD` | **Deprecated** (removed in 0.9.x). Pod env for legacy `DISCOVER_MY_PASSWORD`. |
| `kube.password_container` | string | `""` | **Deprecated** (removed in 0.9.x). Container for legacy password discovery. |
| `kube.loki` | string | — | `namespace/type/name` for Loki port-forward. |
| `kube.loki_local_port` | int | `3100` | |
| `kube.loki_remote_port` | int | `3100` | |
| `client` | string | — | **Required.** Monitor identity label. |
| `db.threshold_total` | int | 0 | **Deprecated** (v1.0). |
| `db.threshold_active` | int | 0 | **Deprecated** (v1.0). |
| `db.threshold_idle` | int | 0 | |
| `db.stale_age` | int | 0 | Seconds. |
| `db.threshold_stale` | int | 0 | |
| `db.default_threshold_percent` | int | 80 | |
| `db.threshold_levels` | string | `75,85,95` | 3-tier percentages. |
| `db.long_query_min_seconds` | int | 0 | 0 = off. |
| `db.long_query_cooldown_seconds` | int | 3600 | |
| `db.long_query_min_count` | int | 1 | |
| `notifications.slack.webhook` | string | — | Slack Incoming Webhook URL. |
| `notifications.loki.url` | string | — | Loki push API. |
| `notifications.loki.labels` | string | — | `k1=v1,k2=v2`. |
| `notifications.loki.org_id` | string | — | X-Scope-OrgID. |
| `notifications.loki.bearer_token` | string | — | Bearer token. |
| `notifications.pagerduty.enabled` | bool | false | Enable PagerDuty Events v2. |
| `notifications.pagerduty.routing_key` | string | — | PagerDuty integration routing key. Required when enabled. |
| `notifications.pagerduty.severity` | string | `warning` | Default severity when event mapping does not apply. |
| `notifications.pagerduty.source` | string | `pgwd` | PagerDuty event source. |
| `notifications.teams.enabled` | bool | false | Enable Microsoft Teams webhook. |
| `notifications.teams.webhook_url` | string | — | Teams incoming webhook URL. Required when enabled. |
| `notifications.generic.enabled` | bool | false | Enable generic webhook. |
| `notifications.generic.webhook_url` | string | — | Target URL. Required when enabled. |
| `notifications.generic.json_key` | string | `text` | JSON field for summary text (default payload). |
| `notifications.generic.headers` | map | — | Custom HTTP headers (e.g. JWT bearer). |
| `notifications.generic.extra_fields` | map | — | Extra key-value pairs in default JSON payload. |
| `notifications.generic.body_template` | string | — | Go template for custom JSON body; validated at load and on first send. |
| `notifications.generic.hmac_secret` | string | — | HMAC-SHA256 signing key for request body. |
| `notifications.generic.hmac_header` | string | `X-Pgwd-Signature` | Header for HMAC signature (`sha256=<hex>`). |
| `notifications.retry.max_attempts` | int | 3 | HTTP retry attempts (5xx and network errors). |
| `notifications.retry.initial_backoff` | duration | `1s` | Initial retry backoff. |
| `notifications.retry.max_backoff` | duration | `10s` | Maximum retry backoff. |
| `interval` | int | 0 | 0 = one-shot. |
| `dry_run` | bool | false | |
| `force_notification` | bool | false | |
| `notify_on_connect_failure` | bool | false | Legacy. |
| `test_max_connections` | int | 0 | |
| `validate_k8s_access` | bool | false | |
| `log_level` | string | `info` | |
| `sqlite.path` | string | — | Metrics store path. |
| `sqlite.max_metrics` | int | 10000 | FIFO cap. |
| `sqlite.stale_age` | int | 0 | |
| `confirm_alert` | int | 1 | Consecutive bad checks before alert. |
| `confirm_ok` | int | 1 | Consecutive ok checks before resolution. |
| `metrics_store.driver` | string | — | `postgres` or `mysql` (overrides sqlite). |
| `metrics_store.dsn` | string | — | DSN for metrics store. |
| `http.listen` | string | — | e.g. `:8080`. |
| `http.base_path` | string | `/api/pgwd/v1` | |
| `http.health_path` | string | `/healthz` | |
| `http.metrics_path` | string | `/metrics` | |

### Environment variables (PGWD_*)

All config keys map to `PGWD_<UPPER_SNAKE>` equivalents. Notifier env vars:

| Env | Maps to |
|-----|---------|
| `PGWD_NOTIFICATIONS_SLACK_WEBHOOK` | `notifications.slack.webhook` |
| `PGWD_NOTIFICATIONS_LOKI_URL` | `notifications.loki.url` |
| `PGWD_NOTIFICATIONS_LOKI_LABELS` | `notifications.loki.labels` |
| `PGWD_NOTIFICATIONS_LOKI_ORG_ID` | `notifications.loki.org_id` |
| `PGWD_NOTIFICATIONS_LOKI_BEARER_TOKEN` | `notifications.loki.bearer_token` |
| `PGWD_NOTIFICATIONS_PAGERDUTY_ENABLED` | `notifications.pagerduty.enabled` |
| `PGWD_NOTIFICATIONS_PAGERDUTY_ROUTING_KEY` | `notifications.pagerduty.routing_key` |
| `PGWD_NOTIFICATIONS_PAGERDUTY_SEVERITY` | `notifications.pagerduty.severity` |
| `PGWD_NOTIFICATIONS_PAGERDUTY_SOURCE` | `notifications.pagerduty.source` |
| `PGWD_NOTIFICATIONS_TEAMS_ENABLED` | `notifications.teams.enabled` |
| `PGWD_NOTIFICATIONS_TEAMS_WEBHOOK` | `notifications.teams.webhook_url` |
| `PGWD_NOTIFICATIONS_GENERIC_ENABLED` | `notifications.generic.enabled` |
| `PGWD_NOTIFICATIONS_GENERIC_WEBHOOK_URL` | `notifications.generic.webhook_url` |
| `PGWD_NOTIFICATIONS_GENERIC_JSON_KEY` | `notifications.generic.json_key` |
| `PGWD_NOTIFICATIONS_GENERIC_HEADERS` | `notifications.generic.headers` (JSON object string) |
| `PGWD_NOTIFICATIONS_GENERIC_EXTRA_FIELDS` | `notifications.generic.extra_fields` (JSON object string) |
| `PGWD_NOTIFICATIONS_GENERIC_BODY_TEMPLATE` | `notifications.generic.body_template` |
| `PGWD_NOTIFICATIONS_GENERIC_HMAC_SECRET` | `notifications.generic.hmac_secret` |
| `PGWD_NOTIFICATIONS_GENERIC_HMAC_HEADER` | `notifications.generic.hmac_header` |
| `PGWD_NOTIFICATIONS_RETRY_MAX_ATTEMPTS` | `notifications.retry.max_attempts` |
| `PGWD_NOTIFICATIONS_RETRY_INITIAL_BACKOFF` | `notifications.retry.initial_backoff` |
| `PGWD_NOTIFICATIONS_RETRY_MAX_BACKOFF` | `notifications.retry.max_backoff` |

### Multi-database limitations

- **SQLite / hysteresis** rows keyed by `(client, cluster, database)` — not by URL host. Use a **unique `client` per `databases:` entry** when the same DB name is used on different hosts.
- **kube.postgres** not supported with `databases:` (multi-DB requires direct URLs). Single-DB + kube until per-db kube exists.

### Validation rules

- At least one notifier (Slack, Loki, kube-loki, PagerDuty, Teams, or generic webhook) OR `-dry-run` must be configured (or config validation fails).
- PagerDuty enabled (or routing key set) → `routing_key` required.
- Teams enabled (or webhook URL set) → `webhook_url` required.
- Generic enabled (or webhook URL set) → `webhook_url` required; `body_template` must compile; rendered output must be valid JSON on send.
- Notification retry: `max_attempts` ≥ 0; backoffs ≥ 0 (defaults applied when zero).
- `-force-notification` requires at least one notifier (not compatible with `-dry-run`).
- `-notify-on-connect-failure` requires at least one notifier.
- When `-kube-postgres` or `-kube-loki` is set, a valid **kubeconfig** must be loadable (client-go; no kubectl binary required).
- Threshold levels: 3 comma-separated percentages, 1-100, ascending.
- `-test-max-connections` > 0 overrides server value for defaults and display.

## 5. Monitoring workflow

### Startup sequence

1. Load config (file, or defaults + env if no file; then CLI flags)
2. Validate config
3. If `-kube-postgres`, start port-forward; if URL password is `DISCOVER_MY_PASSWORD` (**deprecated**, removed in 0.9.x), read pod env via `pods/exec` — see [docs/kubernetes-passwords.md](docs/kubernetes-passwords.md)
4. If `-kube-loki`, start port-forward, set `LokiURL` to `localhost:port`
5. Connect to Postgres
6. Query `max_connections`
7. Apply threshold defaults from `-db-default-threshold-percent` (when not in level mode)
8. If `-force-notification`, send test event to all notifiers
9. Enter daemon loop or run once

### Connection stats

Queried from `pg_stat_activity`:
- **Total:** count of all connections (including idle, active, etc.), excluding the monitor's own connection
- **Active:** `state = 'active'`
- **Idle:** `state = 'idle'`
- **Stale:** `backend_start` older than `stale_age` seconds
- **Long-running queries:** `state = 'active'` and `query_start` older than `long_query_min_seconds`

### Threshold evaluation order (each check cycle)

1. **Stale connections** — if `threshold_stale > 0` and `stale_age > 0`
2. **Long-running queries** — if `long_query_min_seconds > 0` (requires metrics store; cooldown per target)
3. **Level mode** — if both `threshold_total` and `threshold_active` are 0 and `threshold_levels` is valid: evaluate percentages against `max_connections`
4. **Explicit thresholds** — total, active, idle individual comparisons
5. **Force notification** — always emits a `test` event when `-force-notification` is set (and connected)

### Hysteresis (confirm-alert / confirm-ok)

- When `confirm_alert > 1`: alert fires only after N consecutive check cycles where a threshold is breached.
- When `confirm_ok > 1`: resolution fires only after N consecutive check cycles where no threshold is breached.
- Persisted in the metrics store (SQLite or SQL).

### Resolution notifications

When a previously-breached threshold returns to normal for `confirm_ok` consecutive checks, a `resolution` event is sent to all notifiers.

### Long-query cooldown

After a `long_query` notification fires for a target, no further `long_query` notifications for the same target until `long_query_cooldown_seconds` have elapsed (cooldown timestamp stored in metrics store).

### Connect failure

When Postgres connection fails:
- Always sends a `connect_failure` (or `too_many_clients` for SQLSTATE 53300) event to all configured notifiers
- No extra flag required; senders are built before connecting

### Dry-run mode

- Threshold evaluation proceeds as normal
- Threshold events are logged as `[dry-run] would send: …` — **no outbound HTTP** for threshold/resolution/long-query alerts
- **Connect failure and `too_many_clients` are still sent** to notifiers when configured (infrastructure failures bypass dry-run)

## 6. Notifications

### Event types

| Event | Trigger |
|-------|---------|
| `test` | `-force-notification` |
| `connect_failure` | Postgres connect error (non-53300) |
| `too_many_clients` | Postgres SQLSTATE 53300 |
| `resolution` | Threshold clear after confirm_ok |
| `total` | Total connections ≥ threshold-total |
| `active` | Active connections ≥ threshold-active |
| `idle` | Idle connections ≥ threshold-idle |
| `stale` | Stale connections ≥ threshold-stale |
| `long_query` | Long-running queries ≥ min count |

### Levels (3-tier)

| Level | Color | Meaning |
|-------|-------|---------|
| `attention` | Yellow / `#FFD700` | First threshold breached (default 75%) |
| `alert` | Orange / `#FF8C00` | Second threshold breached (default 85%) |
| `danger` | Red / `#CC0000` | Third threshold breached (default 95%) |

Only the highest breached level fires per check cycle.

### Shared HTTP retry

All notifier senders (Slack, Loki, PagerDuty, Teams, generic webhook) use shared outbound HTTP with:

- 30s client timeout
- Retry on **5xx** and network errors only (4xx fails immediately)
- Defaults: `max_attempts=3`, `initial_backoff=1s`, `max_backoff=10s` (configurable via `notifications.retry` or CLI/env)
- Non-2xx final response logs an error but does not fail the check

### Slack

- Incoming Webhook: POST JSON with `attachments[].{color, text, fallback}`
- Header varies by event type (test, connect_failure, too_many_clients, resolution, level-based)
- Body includes: connections summary, cluster, database, client, namespace, timestamp
- Non-2xx response logs an error but does not fail the check

### Loki

- Push API POST to `/loki/api/v1/push`
- Labels: `app=pgwd`, `threshold`, `level`, plus optional `namespace`, `database`, `cluster`
- Log line includes: prefix (`pgwd:` or `pgwd [cluster=X database=Y client=Z]:`), message, total/active/idle, max_connections
- Support for `X-Scope-OrgID` header (multi-tenancy) and `Authorization: Bearer ***`
- Non-2xx response logs an error but does not fail the check

### PagerDuty

- Events API v2: POST `https://events.pagerduty.com/v2/enqueue`
- Envelope: `routing_key`, `event_action: trigger`, `payload.{summary, source, severity, timestamp, custom_details}`
- `custom_details`: total, active, idle, max_connections, threshold, threshold_value, level, database, client, cluster, namespace
- Severity mapping: `danger` / `too_many_clients` / `connect_failure` → `critical`; `alert` → `warning`; `attention` / `resolution` / `test` → `info`; otherwise config default (`warning`)
- Default source: `pgwd`

### Microsoft Teams

- Incoming webhook: POST JSON `{"text": "<plain-text summary>"}`
- Summary matches Slack content (connections, cluster, database, client, namespace, timestamp) without attachments or color

### Generic webhook

- POST JSON to configured URL
- Default payload: `{<json_key>: "<summary>", ...extra_fields}` (`json_key` default `text`)
- Optional `headers` (e.g. `Authorization: Bearer <JWT>`)
- Optional `body_template` (Go template → valid JSON). Variables: `Message`, `Threshold`, `Level`, `Total`, `Active`, `Idle`, `MaxConn`, `Cluster`, `Client`, `Database`, `Namespace`, `EventType`
- Optional HMAC-SHA256 over raw body: header `X-Pgwd-Signature` (configurable) with value `sha256=<hex>`

### Connect failure notification

- Always-on when any notifier is configured (no extra flag; `-notify-on-connect-failure` is legacy/no-op)
- **Not suppressed by `-dry-run`** — real HTTP notifications are sent
- Maps SQLSTATE 53300 → event type `too_many_clients`
- Maps any other connection error → `connect_failure`
- Sent before the monitor exits (single-target) or before skipping the target (multi-DB)

## 7. Metrics store

### Backends

| Driver | Implementation | Dependencies |
|--------|---------------|--------------|
| SQLite (empty driver) | `internal/store/sqlite.go` | Embedded (CGo-free) |
| `postgres` | `internal/store/sqlstore.go` | `github.com/jackc/pgx/v5` (stdlib) |
| `mysql` | `internal/store/sqlstore.go` | `github.com/go-sql-driver/mysql` |

### Shared interface (`store.MetricsStorer`)

- `StoreRow(… MetricsRow) error`
- `StoreResolved(…) error`
- `StaleCount(…) (int, error)`
- `RecentEvents(…) ([]MetricsRow, error)`
- `HasRecentEvent(…) (bool, error)`
- `GetCooldown(…, eventType string) (time.Time, error)`
- `SetCooldown(…, eventType string, until time.Time) error`
- `ExportRows(…) ([]MetricsRow, error)`
- `Close() error`

### Purpose

- Hysteresis counters (confirm-alert, confirm-ok)
- Resolution notification tracking
- Long-query cooldown timestamps
- HTTP `/metrics` endpoint data
- CSV export

### Row schema (common across backends)

Keyed by `(client, cluster, database)`. Fields: timestamp, total/active/idle connections, max_connections, threshold, level, state (ok/alert).

### FIFO eviction

When `sqlite.max_metrics` (default 10000) is exceeded, oldest rows are pruned.

## 8. HTTP server

Optional (`http.listen`). Endpoints:

| Path | Method | Response |
|------|--------|----------|
| `{base_path}{health_path}` (default `/api/pgwd/v1/healthz`) | GET | `{"status":"ok"}` — always returns 200 when the server is running |
| `{base_path}{metrics_path}` (default `/api/pgwd/v1/metrics`) | GET | **Prometheus text exposition** (`text/plain; version=0.0.4`) — gauges `pgwd_connections_*`, `pgwd_state`, etc. Empty store returns `# No metrics store configured` or `# No metrics yet` |

Used for Kubernetes liveness/readiness probes and Prometheus scraping.

## 9. Kubernetes integration

### Postgres port-forward (`-kube-postgres`)

- Valid formats: `namespace/svc/service-name` or `namespace/pod/pod-name`
- Uses **client-go** port-forward (kubeconfig required; **no kubectl binary**)
- **Deprecated (0.6.x, removed in 0.9.x):** when the URL password is the literal `DISCOVER_MY_PASSWORD`, pgwd runs `printenv` in the Postgres pod via `pods/exec` (extra RBAC). **Decision record:** [docs/kubernetes-passwords.md](docs/kubernetes-passwords.md). Prefer a Secret-backed DSN or planned `kube.password_from_secret` (0.9.x).
- `-kube-password-var` / `-kube-password-container` — **deprecated** with `DISCOVER_MY_PASSWORD`

### Loki port-forward (`-kube-loki`)

- Same format: `namespace/svc/loki-service-name`
- Mutually exclusive with an explicit `-notifications-loki-url` (port-forward takes precedence when `-kube-loki` is set and LokiURL is empty)
- Uses **client-go** port-forward (same kubeconfig requirements as Postgres)

### Cluster identity

- Cluster name resolved from kubeconfig context (used in notification context fields)
- When `-kube-postgres` is not set, cluster is empty

## 10. CSV export

One-shot mode (`-export-metrics-format csv -export-metrics-destination <path>`):

1. Opens the configured metrics store (SQLite or SQL)
2. Calls `ExportRows()` to retrieve all rows
3. Writes CSV with header: `timestamp,client,cluster,database,total,active,idle,max_connections,threshold,level,state`
4. Logs row count and exits 0

Requires an active metrics store (sqlite.path or metrics_store.driver+dsn).

## 11. Build and release

### Build

- Go module: `github.com/hrodrig/pgwd`
- Minimum Go: 1.26.4 (as of 0.7.0)
- `make build`: reads `VERSION`, injects `Version`/`Commit`/`BuildDate`/`Branch` via ldflags
- `make install`: installs to `$GOBIN`
- Cross-compile: `make build-linux`, `make build-darwin`, `make build-windows`, `make build-all` (output in `dist/`)

### Docker

- Multi-stage build: `golang:1.26.4-alpine` → `alpine:3.24.1`
- Non-root user `pgwd` (UID/GID 1000)
- Minimal runtime: `ca-certificates` only
- Image scanning via `make docker-scan` (Grype)

### Release

- `make release-check`: validates VERSION semver, lint, unit test, integration test, e2e kube test, docker security scan
- `make release`: runs `release-check` then goreleaser (build → package → publish to GitHub Releases, Homebrew, Docker)
- Platform tests: Ansible playbooks against Linux and BSD VMs (manual pre-release gate)
- Semantic versioning (MAJOR.MINOR.PATCH)

### Supply chain (planned, 0.8.0)

- Syft SBOM for GHCR images and release artifacts
- Cosign keyless signing (OIDC)
- `cosign verify` documentation for operators

## 12. Testing baseline

| Layer | Expectation |
|-------|-------------|
| Unit | `make test` or `go test ./...` passes |
| Coverage | Unit coverage: `make cover` → `coverage.out`. Integration coverage: `make cover-integration` (Docker Postgres + Loki) → `coverage-integration.out` |
| Integration | `make test-integration` (Docker compose with Postgres + Loki). Must pass before release. |
| E2E kube | `make test-e2e-kube` (kind cluster + kubeconfig). Must pass before release. |
| Lint | `make lint`: gofmt -s, go vet, gocyclo (≤ 14). CI lint job matches. |
| Security | `make security`: govulncheck + docker-scan (Grype). CI Security workflow. |
| Platform | `make test-platforms` (Ansible, Linux + BSD VMs). Manual pre-release gate. |

## 13. Package and distribution

| Platform | Format | Notes |
|----------|--------|-------|
| Linux (Debian/Ubuntu) | `.deb` | Includes man page, `/etc/pgwd/pgwd.conf`, systemd unit |
| Linux (Fedora/RHEL/Alma/Rocky/Oracle) | `.rpm` | Same as .deb |
| Linux (Alpine) | `.tar.gz` | OpenRC script at `/etc/init.d/pgwd` |
| macOS | Homebrew cask | `brew install hrodrig/pgwd/pgwd` |
| Windows | `.zip` | Standalone binary |
| FreeBSD | Port (`/usr/ports/sysutils/pgwd`) | BSD makefile in `contrib/freebsd/` |
| OpenBSD | Port (`/usr/ports/sysutils/pgwd`) | rc.d script in `contrib/openbsd/port/` |
| NetBSD | `.tar.gz` + rc.d | |
| DragonFly BSD | `.tar.gz` + rc.d | |
| illumos/Solaris | `.tar.gz` + SMF manifest | |
| Docker | `ghcr.io/hrodrig/pgwd` | Alpine, non-root, multi-arch (amd64 + arm64) |

Helm chart and in-cluster deployment manifests maintained in **[pgwd-selfhosted](https://github.com/hrodrig/pgwd-selfhosted)**; see `contrib/HELM.md` and `contrib/k8s/README.md`.

## 14. Repository structure

```
cmd/pgwd/               — Main package, CLI orchestration, black-box tests
internal/
  checker/              — Threshold logic, event collection, state derivation
  config/               — Config load (file, env, CLI)
  httpsrv/              — HTTP server (metrics, health)
  kube/                 — Kubernetes port-forward (client-go); legacy password discovery via pods/exec (deprecated, 0.9.x)
  metricsexport/        — CSV export
  metricsstore/         — Backend selection for metrics store
  notify/               — Slack, Loki, PagerDuty, Teams, generic webhook senders (Sender interface)
  postgres/             — Pool, stats, stale count, max_connections
  store/                — MetricsStorer interface, SQLite + SQL backends
  validator/            — Config validation
docs/                   — Sequence diagrams, upgrade guide, Grafana alerts
contrib/
  systemd/              — systemd unit files
  freebsd/              — FreeBSD port files
  openbsd/              — OpenBSD port files
testing/
  compose.yaml          — Integration test stack (Postgres + Loki)
  platforms/            — Ansible roles/playbooks for platform tests
```

## 15. Configuration examples

### Minimal one-shot with Slack

```yaml
db:
  url: "postgres://user:pass@localhost:5432/mydb"
client: "prod-db"
notifications:
  slack:
    webhook_url: "https://hooks.slack.com/services/…"
```

### Daemon with level mode and Loki

```yaml
client: "prod-db"
db:
  threshold_levels: "70,85,95"
  long_query_min_seconds: 300
  long_query_cooldown_seconds: 1800
interval: 30
sqlite:
  path: "/var/lib/pgwd/pgwd.db"
notifications:
  slack:
    webhook_url: "https://hooks.slack.com/services/…"
  loki:
    url: "http://localhost:3100/loki/api/v1/push"
    labels: "app=pgwd,env=prod"
    org_id: "1"
```

### Multi-DB with per-target thresholds

```yaml
client: "monitor"
databases:
  - url: "postgres://user:pass@host1:5432/db1"
    client: "monitor-db1"
    stale_age: 3600
    threshold_stale: 10
  - url: "postgres://user:pass@host2:5432/db2"
    client: "monitor-db2"
    threshold_levels: "80,90,98"
interval: 60
sqlite:
  path: "/var/lib/pgwd/pgwd.db"
notifications:
  slack:
    webhook_url: "https://hooks.slack.com/services/…"
```

### Kubernetes outside cluster (port-forward; Secret-backed password)

Preferred: inject the password from a Kubernetes Secret (no `DISCOVER_MY_PASSWORD`). See [docs/kubernetes-passwords.md](docs/kubernetes-passwords.md).

```bash
export PGWD_DB_URL="postgres://postgres:${PGPASSWORD}@localhost:5432/mydb?sslmode=disable"
# PGPASSWORD from: kubectl get secret postgres-credentials -o jsonpath='{.data.password}' | base64 -d
pgwd -kube-postgres default/svc/postgres -client kube-db -config /etc/pgwd/pgwd.conf
```

```yaml
client: "kube-db"
kube:
  postgres: "default/svc/postgres"
  loki: "monitoring/svc/loki"
db:
  url: "postgres://postgres:YOUR_PASSWORD@localhost:5432/mydb?sslmode=disable"
notifications:
  loki:
    labels: "app=pgwd,env=production"
    org_id: "my-tenant"
```

> **Note:** `DISCOVER_MY_PASSWORD` in the URL is **deprecated** and will be **removed in 0.9.x** (requires `pods/exec` RBAC). Rationale: [docs/kubernetes-passwords.md](docs/kubernetes-passwords.md).

### Dry-run for testing

```yaml
client: "test-db"
db:
  url: "postgres://user:pass@localhost:5432/testdb"
  threshold_levels: "10,20,30"
dry_run: true
```

When behavior in this document changes, update **ROADMAP**, **SPEC**, band **plan**, and **CHANGELOG** in the same change set or release.
