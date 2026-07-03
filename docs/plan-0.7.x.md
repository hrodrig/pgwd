# pgwd plan 0.7.x — notification channels expansion

**Roadmap index:** [ROADMAP.md](../ROADMAP.md) · **Band:** 0.7.x · **Status:** ✅ shipped (v0.7.0)

**Theme:** Add PagerDuty Events v2, Microsoft Teams, and Generic Webhook + JWT/HMAC notifiers, following the same pattern as [groot](https://github.com/hrodrig/groot). Refactor outbound HTTP with shared retry/backoff.

**Baseline:** [SPECIFICATIONS.md](../SPECIFICATIONS.md) (v0.7.0)  
**Previous band:** v0.6.x (shipped)  
**Next band:** [plan-0.8.x.md](./plan-0.8.x.md) (supply chain)  
**Target window:** Jun 18–25, 2026 (after v0.6.10)

---

## Scope

### 1. PagerDuty Events API v2 (`internal/notify/pagerduty.go`)

- New `PagerDuty` sender struct with `routing_key`, `severity`, `source`
- POST to `https://events.pagerduty.com/v2/enqueue`
- Envelope: `{routing_key, event_action: "trigger", payload: {summary, source, severity, timestamp, custom_details}}`
- `custom_details` includes: total/active/idle connections, max_connections, threshold, database, client, cluster
- Map pgwd event thresholds → PagerDuty severity:
  - `danger` / `too_many_clients` / `connect_failure` → `critical`
  - `alert` → `warning`
  - `attention` → `info`
  - `resolution` → `info`
  - `test` → `info`
- Default severity: `warning` (configurable via `severity` in config/CLI)
- Default source: `pgwd`

### 2. Microsoft Teams (`internal/notify/teams.go`)

- New `Teams` sender struct with `webhook_url`
- POST JSON `{"text": "<formatted message>"}` to Teams incoming webhook
- Message format matches Slack summary (connections, cluster, database, client, threshold)
- Simpler payload than Slack (no attachments/color — Teams just gets formatted text)

### 3. Generic Webhook + JWT/HMAC (`internal/notify/generic.go`)

- New `GenericWebhook` sender with fields matching groot's `GenericWebhookCfg`:
  - `webhook_url` — target URL
  - `json_key` — field name for the main text (default `"text"`)
  - `headers` — custom HTTP headers (for JWT: `Authorization: Bearer <token>`)
  - `extra_fields` — additional key-value pairs in the JSON payload
  - `body_template` — Go template string for fully custom payload (must be valid JSON after substitution). Template variables: `{{.Message}}`, `{{.Threshold}}`, `{{.Level}}`, `{{.Total}}`, `{{.Active}}`, `{{.Idle}}`, `{{.MaxConn}}`, `{{.Cluster}}`, `{{.Client}}`, `{{.Database}}`, `{{.Namespace}}`, `{{.EventType}}`
  - `hmac_secret` — HMAC-SHA256 signing key
  - `hmac_header` — header name for signature (default `"X-Pgwd-Signature"`)
- Default payload (no template): `{"<json_key>": "<formatted message>", ...extra_fields}`
- HMAC: `sha256=<hex>` over raw POST body
- Use case: JWT bearer auth → `headers: {Authorization: "Bearer eyJ..."}`, or any custom API

### 4. Shared HTTP retry (`internal/notify/http.go`)

- Extract `postJSONWithRetry()` from groot's pattern
- Global HTTP client with 30s timeout
- Retry config: `max_attempts` (default 3), `initial_backoff` (default 1s), `max_backoff` (default 10s)
- Retry only on 5xx and network errors; 4xx fails immediately
- Exponential backoff with context cancellation support
- Migrate Slack and Loki senders to shared retry (optional in same release, preferred)

### 5. Config and integration

#### Config struct changes (`internal/config/config.go`)

New fields on `Config`:

```go
// Notifications
PagerDutyEnabled    bool
PagerDutyRoutingKey string
PagerDutySeverity   string   // default "warning"
PagerDutySource     string   // default "pgwd"

TeamsWebhook string

GenericEnabled      bool
GenericWebhookURL   string
GenericJSONKey      string   // default "text"
GenericHeaders      map[string]string
GenericExtraFields  map[string]string
GenericBodyTemplate string
GenericHMACSecret   string
GenericHMACHeader   string   // default "X-Pgwd-Signature"

RetryMaxAttempts    int
RetryInitialBackoff time.Duration
RetryMaxBackoff     time.Duration
```

#### `HasAnyNotifier()` — include new channels

```go
func (c *Config) HasAnyNotifier() bool {
    return c.SlackWebhook != "" || c.LokiURL != "" || c.KubeLoki != "" ||
        c.PagerDutyEnabled || c.TeamsWebhook != "" || c.GenericEnabled
}
```

(Use actual field names from implementation; `TeamsWebhook` non-empty or explicit `enabled` flag — decide at implement time.)

#### Environment variables

| Env | Maps to |
|-----|---------|
| `PGWD_NOTIFICATIONS_PAGERDUTY_ROUTING_KEY` | `notifications.pagerduty.routing_key` |
| `PGWD_NOTIFICATIONS_PAGERDUTY_SEVERITY` | `notifications.pagerduty.severity` |
| `PGWD_NOTIFICATIONS_PAGERDUTY_SOURCE` | `notifications.pagerduty.source` |
| `PGWD_NOTIFICATIONS_TEAMS_WEBHOOK` | `notifications.teams.webhook_url` |
| `PGWD_NOTIFICATIONS_GENERIC_WEBHOOK_URL` | `notifications.generic.webhook_url` |
| `PGWD_NOTIFICATIONS_GENERIC_JSON_KEY` | `notifications.generic.json_key` |
| `PGWD_NOTIFICATIONS_GENERIC_HEADERS` | `notifications.generic.headers` (JSON string) |
| `PGWD_NOTIFICATIONS_GENERIC_EXTRA_FIELDS` | `notifications.generic.extra_fields` (JSON string) |
| `PGWD_NOTIFICATIONS_GENERIC_BODY_TEMPLATE` | `notifications.generic.body_template` |
| `PGWD_NOTIFICATIONS_GENERIC_HMAC_SECRET` | `notifications.generic.hmac_secret` |
| `PGWD_NOTIFICATIONS_GENERIC_HMAC_HEADER` | `notifications.generic.hmac_header` |
| `PGWD_NOTIFICATIONS_RETRY_MAX_ATTEMPTS` | `notifications.retry.max_attempts` |
| `PGWD_NOTIFICATIONS_RETRY_INITIAL_BACKOFF` | `notifications.retry.initial_backoff` |
| `PGWD_NOTIFICATIONS_RETRY_MAX_BACKOFF` | `notifications.retry.max_backoff` |

#### CLI flags

```
-notifications-pagerduty-routing-key
-notifications-pagerduty-severity (default "warning")
-notifications-pagerduty-source (default "pgwd")
-notifications-teams-webhook
-notifications-generic-webhook-url
-notifications-generic-json-key (default "text")
-notifications-generic-headers (JSON string)
-notifications-generic-extra-fields (JSON string)
-notifications-generic-body-template
-notifications-generic-hmac-secret
-notifications-generic-hmac-header (default "X-Pgwd-Signature")
-notifications-retry-max-attempts (default 3)
-notifications-retry-initial-backoff (default "1s")
-notifications-retry-max-backoff (default "10s")
```

#### Config file section (YAML)

Existing Slack key stays **`webhook`** (v0.6.x); new sections below:

```yaml
notifications:
  slack:
    webhook: "https://hooks.slack.com/services/..."
  loki:
    url: "http://localhost:3100/loki/api/v1/push"
  pagerduty:
    enabled: true
    routing_key: "..."
    severity: critical
    source: pgwd-prod
  teams:
    enabled: true
    webhook_url: "https://..."
  generic:
    enabled: true
    webhook_url: "https://..."
    json_key: "message"
    headers:
      Authorization: "Bearer eyJ..."
    extra_fields:
      source: "pgwd"
      environment: "production"
    body_template: '{"text":"{{.Message}}","source":"pgwd","event":"{{.EventType}}"}'
    hmac_secret: "..."
    hmac_header: "X-Pgwd-Signature"
  retry:
    max_attempts: 5
    initial_backoff: "2s"
    max_backoff: "30s"
```

#### `buildSenders()` in `cmd/pgwd/main.go`

Add PagerDuty, Teams, and Generic alongside Slack and Loki:

```go
func buildSenders(cfg *config.Config) []notify.Sender {
    var senders []notify.Sender
    if cfg.SlackWebhook != "" {
        senders = append(senders, &notify.Slack{WebhookURL: cfg.SlackWebhook})
    }
    if cfg.LokiURL != "" {
        senders = append(senders, &notify.Loki{...})
    }
    if cfg.PagerDutyEnabled {
        senders = append(senders, &notify.PagerDuty{...})
    }
    if cfg.TeamsWebhook != "" {
        senders = append(senders, &notify.Teams{WebhookURL: cfg.TeamsWebhook})
    }
    if cfg.GenericEnabled {
        senders = append(senders, &notify.GenericWebhook{...})
    }
    return senders
}
```

#### Validation (`internal/validator/validator.go`)

- PagerDuty enabled → `routing_key` required
- Teams enabled → `webhook_url` required
- Generic enabled → `webhook_url` required; compile `body_template` at config load; validate JSON output on first send
- Retry: `max_attempts` ≥ 1; backoffs valid durations

---

## Files to create/modify

| File | Action | Notes |
|------|--------|-------|
| `internal/notify/notify.go` | Modify | Shared message helpers if needed |
| `internal/notify/http.go` | **Create** | Shared `postJSONWithRetry()`, HTTP client, retry config |
| `internal/notify/pagerduty.go` | **Create** | PagerDuty sender + tests |
| `internal/notify/teams.go` | **Create** | Teams sender + tests |
| `internal/notify/generic.go` | **Create** | Generic webhook sender + tests |
| `internal/notify/slack.go` | Modify | Use shared retry |
| `internal/notify/loki.go` | Modify | Use shared retry |
| `internal/config/config.go` | Modify | New fields, `HasAnyNotifier()`, env/file mapping |
| `internal/config/file.go` | Modify | YAML unmarshaling for new sections |
| `internal/validator/validator.go` | Modify | `ValidateNotifiers()` for new channels |
| `cmd/pgwd/main.go` | Modify | `buildSenders()`, CLI flags |
| `contrib/pgwd.conf.example` | Modify | New notifier sections |

---

## Testing

- Unit tests per sender (mock HTTP, payload shape, HMAC signature)
- `postJSONWithRetry`: 2xx, 4xx fast-fail, 5xx retry, network retry, context cancel
- Config validation tests for new fields
- `make test`, `make lint`, `make test-integration` must pass before release

---

## Documentation (release gate)

- [x] Update [SPECIFICATIONS.md](../SPECIFICATIONS.md) §4 (config) and §6 (notifications)
- [x] Update [README.md](../README.md) parameter table
- [x] Update [contrib/pgwd.conf.example](../contrib/pgwd.conf.example)
- [x] Update [CHANGELOG.md](../CHANGELOG.md) under `[Unreleased]` (0.7.0 scope)
- [x] Update [contrib/man/man1/pgwd.1](../contrib/man/man1/pgwd.1) `.TH` line on VERSION bump (`2026-07-03`, `pgwd v0.7.0`)

---

## Estimated effort

- ~6 new Go files (notify + shared HTTP)
- ~15 existing files modified
- ~400 lines Go, ~200 lines tests

---

## Open questions

| # | Question | Decision |
|---|----------|----------|
| 1 | Validate `body_template` at config load or only at runtime? | Compile template at load; validate JSON output on first send. |
| 2 | Teams/PagerDuty: `enabled` bool vs non-empty URL only? | Prefer explicit `enabled` + required URL when enabled (matches generic/pagerduty sketch). |

---

## Release checklist

- [x] Tag `v0.7.0` from `main` after merge from `develop` (2026-07-03)
- [x] `make release-check` green (lint, test, cover-check ≥80%, integration, e2e-kube, docker-scan)
- [x] SPEC + CHANGELOG updated in same change set as code
