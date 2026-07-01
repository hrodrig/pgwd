# pgwd plan 0.9.x — pre-1.0 polish

**Theme:** Stability, coverage, operator ergonomics, and deprecation runway before the breaking **v1.0.0** release.

**Baseline:** [SPECIFICATIONS.md](../SPECIFICATIONS.md) (0.7.x + 0.8.x shipped)  
**Previous band:** [plan-0.8.x.md](./plan-0.8.x.md)  
**Next band:** [plan-1.0.x.md](./plan-1.0.x.md) (breaking stable API)  
**Target window:** Jul 1–7, 2026

---

## Scope

### 1. Notification retry configuration (if not fully done in 0.7.x)

- Ensure `notifications.retry` YAML section is documented and wired end-to-end
- Env vars `PGWD_NOTIFICATIONS_RETRY_*` match SPEC and README

### 2. `notify-on-connect-failure` deprecation runway

- Flag remains parsed for backward compat but is a **no-op** (behavior since 0.6.x: always-on when notifiers exist)
- Startup warning when flag is set: "ignored, connect failure notifications are always enabled"
- Full removal in [plan-1.0.x.md](./plan-1.0.x.md)

### 3. Config profiles (`contrib/profiles/`)

Ready-to-use YAML snippets:

| Profile | Use case |
|---------|----------|
| `minimal-slack.yml` | One-shot Slack |
| `daemon-loki.yml` | Daemon + Loki + SQLite |
| `kube-prod.yml` | Outside cluster: kube port-forward + Slack/Loki |
| `multi-db.yml` | Multi-DB + Slack |

Document profiles in SPECIFICATIONS.md §15 and README.

### 4. Test coverage

- Target: **70%+** statement coverage (from ~60%)
- Focus: resolution notifications, hysteresis (`confirm_alert` / `confirm_ok`), multi-DB edge cases, dry-run vs connect-failure bypass
- See [testing-main-strategy.md](./testing-main-strategy.md) for integration priorities

### 5. `--strict` mode (optional)

- When `-strict` is set: exit **4** if at least one notifier fails delivery for a threshold event
- Default unchanged: notify errors logged only; exit **0** or **1** (config/connect) as today
- Document reserved exit codes in SPECIFICATIONS.md §3

### 6. Deprecated `db:` config — stronger warning

- Extra startup banner when legacy `db:` is used (in addition to existing stderr warning)
- Migration hint: use `databases:` with one entry
- Removal in 1.0.0

### 7. Benchmarking and CI

- `make bench` target
- CI: `go test -bench=. ./internal/...` on push (non-blocking)

### 8. Anonymous usage collector (daemon mode only)

Same privacy model as [gghstats `internal/collector`](https://github.com/hrodrig/gghstats/tree/main/internal/collector): **one fire-and-forget POST on daemon startup** (`interval > 0`). Never runs in one-shot or export-only exits. Errors logged at debug only; must not block or fail the monitor.

#### Not the same as local metrics

| Feature | Purpose | Default |
|---------|---------|---------|
| **Collector (telemetry)** | Anonymous product usage to improve pgwd | **Opt-in (off)** |
| **Update check** | Warn if a newer release exists on GitHub | **Opt-out (on)** |
| `sqlite.path` / `metrics_store` | User operational history (histéresis, CSV, `/metrics`) | User-configured |
| Slack / Loki | User alert channels | User-configured |

#### Defaults

| Setting | Default | Override |
|---------|---------|----------|
| Collector | **disabled** | `PGWD_ENABLE_COLLECTOR=true` or `enable_collector: true` in config |
| Update check | **enabled** | `PGWD_ENABLE_UPDATE_CHECK=false` or `enable_update_check: false` |

Rationale: pgwd runs beside production Postgres; outbound telemetry must be explicit. Update check only hits the public GitHub releases API (no DB or config values).

#### Payload (allowed fields only)

- `version`, `commit`, `build_date`
- `hash` — one-way fingerprint of boolean feature shape (dedup), not reversible config
- `features` — booleans only, e.g.:
  - `multi_db`, `uses_level_mode`, `long_query_enabled`
  - `has_slack`, `has_loki`, `has_kube_postgres`, `has_kube_loki`
  - `has_sqlite_store`, `has_sql_metrics_store`, `has_http_listen`
  - `confirm_alert_gt_1`, `confirm_ok_gt_1`
  - `dry_run` (should be false in normal daemon; include for honesty)

**Never send:** `client`, DSN/URL, hostnames, database names, cluster/namespace, webhook URLs, file paths, Loki labels, kube context.

#### Startup log (info)

Mirror gghstats messages, e.g.:

- Collector off: *"Anonymous metric collection is disabled. Set PGWD_ENABLE_COLLECTOR=true to help improve pgwd. See …"*
- Collector on: thank-you + link to README privacy section
- Update check off/on variants when collector is also off/on

#### Implementation sketch

| Piece | Notes |
|-------|--------|
| `internal/collector/collector.go` | **Create** — `Collect`, `CheckUpdate`, `CollectWithUpdate`; reuse gghstats patterns |
| `internal/collector/collector_test.go` | Payload shape, no PII fields, semver compare |
| `cmd/pgwd/main.go` | After validation, if `cfg.Interval > 0`: `go startCollector(cfg)` |
| `internal/config/config.go` | `EnableCollector`, `EnableUpdateCheck`; env + YAML + CLI |
| `contrib/pgwd.conf.example` | Commented block with privacy note |
| `README.md` | Table row + short "Anonymous usage" section |
| `SPECIFICATIONS.md` | §3 startup (daemon), new § or subsection on collector |
| `CHANGELOG.md` | 0.9.x entry |

Config keys (YAML):

```yaml
# Optional — both default as above
enable_collector: false
enable_update_check: true
```

Env: `PGWD_ENABLE_COLLECTOR`, `PGWD_ENABLE_UPDATE_CHECK`. CLI flags optional: `-enable-collector`, `-enable-update-check` (only if we want parity with env; env-only is enough for v1 if scope is tight).

#### Endpoint

- Dedicated collector URL (e.g. `collect.pgwd…` or shared Hermes collector with `product: pgwd` in JSON)
- **Open question** — decide before implementation (see below)

#### Air-gapped / proxy

- Respect `HTTP_PROXY` / `HTTPS_PROXY`; fail silently on network error
- No retries in loop; single attempt per process start

### 9. Documentation and SPEC audit

- Finish [SPECIFICATIONS.md](../SPECIFICATIONS.md) audit against **0.9.x** code (K8s client-go, HTTP `/healthz`/`/metrics`, metrics store interface, CSV columns, config load order, **collector privacy**)
- Sequence diagrams in [docs/sequence/](./sequence/) re-audit if behavior changed

---

## Files to modify

| File | Action |
|------|--------|
| `internal/collector/` | **Create** — anonymous daemon startup telemetry + update check |
| `cmd/pgwd/main.go` | `--strict`, deprecation warnings, `startCollector` when `interval > 0` |
| `internal/config/config.go` | `EnableCollector`, `EnableUpdateCheck`; file/env mapping |
| `internal/validator/validator.go` | Strict-mode validation if needed |
| `contrib/profiles/` | **Create** profile YAML snippets |
| `contrib/pgwd.conf.example` | Collector flags (commented) + privacy note |
| `SPECIFICATIONS.md` | Exit codes, profiles, collector, audited behavior |
| `README.md` | Profiles, strict mode, anonymous usage |
| Various `*_test.go` | Coverage improvements |

---

## Testing

- `make test`, `make lint`, `make test-integration`, `make test-e2e-kube`
- Coverage report: `make cover` meets 70% target (or documented exceptions)
- Platform smoke: at least Ubuntu + one BSD if install paths touched

---

## Open questions

| # | Question | Status |
|---|----------|--------|
| 1 | Logo/branding for 1.0? | Wait for user input on design direction. |
| 2 | Implement distinct exit codes 2–3 (connect vs query) or only 4 in strict? | Defer granular 2/3 to 1.0 or post-1.0 unless needed for cron. |
| 3 | Collector endpoint: dedicated `collect.pgwd…` vs shared backend with `product: pgwd`? | Decide before coding `internal/collector`. |

---

## Release checklist

- [ ] Tag `v0.9.0` from `main`
- [ ] SPEC audited for 0.9.x
- [ ] Profiles shipped under `contrib/profiles/`
- [ ] Collector: opt-in default verified; README privacy section published
- [ ] CHANGELOG [Unreleased] → 0.9.x section
