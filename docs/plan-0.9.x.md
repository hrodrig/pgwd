# pgwd plan 0.9.x — pre-1.0 polish

**Roadmap index:** [ROADMAP.md](../ROADMAP.md) · **Band:** 0.9.x

**Theme:** Stability, coverage, operator ergonomics, and deprecation runway before the breaking **v1.0.0** release.

**Baseline:** [SPECIFICATIONS.md](../SPECIFICATIONS.md) (v0.6.10 shipped; 0.7.x and 0.8.x planned ahead of this band)  
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
| `kube-prod.yml` | Outside cluster: kube port-forward + Secret/env (no `DISCOVER_MY_PASSWORD`) |
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

### 10. Remove `DISCOVER_MY_PASSWORD` (security) + secure alternatives

**Problem:** Today pgwd runs `printenv` inside the Postgres pod via `pods/exec` (SPDY) when the DSN password is the literal `DISCOVER_MY_PASSWORD`. That needs **`pods/exec` RBAC** — not documented in SPEC/contrib/k8s/Helm — and contradicts read-only monitoring / least privilege. Password crosses into the pgwd process and DSN in memory.

**Goal users had:** run pgwd **outside** the cluster with `-kube-postgres` without storing the Postgres password in config files or git.

**0.9 approach:** ship **documented replacements first**, then **remove** the placeholder in the same band (deprecation warning in early 0.9.x patch if needed; hard fail + code removal before 0.9.0 tag or in final 0.9.x — pick one release cut).

#### What to remove

| Area | Remove |
|------|--------|
| `internal/kube/kube.go` | `discoverPasswordPlaceholder`, `GetPasswordFromPod`, `DiscoverPasswordPlaceholder`, `URLContainsDiscoverPassword`; password branch in `ReplaceDBURLForKube` |
| `internal/config/` | `KubePasswordVar`, `KubePasswordContainer`, YAML `kube.password_var` / `kube.password_container`, env `PGWD_KUBE_PASSWORD_*` |
| `cmd/pgwd/main.go` | Discovery branch in `setupKube`; flags `-kube-password-var`, `-kube-password-container` |
| Tests/docs | `kube_test` discovery cases, README/AGENTS/SPEC/man/BSD contrib examples, `test-e2e-kube.sh` DISCOVER path |

**Validation after removal:** if DSN still contains `DISCOVER_MY_PASSWORD`, exit 1 with migration link:

```text
DISCOVER_MY_PASSWORD was removed in pgwd 0.9.x (security).
Use a Secret-backed URL — see docs/kubernetes-passwords.md
```

#### Secure alternatives (same outcomes, no `pods/exec`)

Operators choose by deployment model. Document all in **`docs/kubernetes-passwords.md`** (new) and link from README § Kubernetes.

##### A. In-cluster (preferred — already supported)

pgwd Deployment/DaemonSet; DSN from Secret → env. No `-kube-postgres`, no port-forward.

```yaml
env:
  - name: PGWD_DB_URL
    valueFrom:
      secretKeyRef:
        name: pgwd-db
        key: url
```

Source: [contrib/k8s/README.md](../contrib/k8s/README.md), pgwd-selfhosted Helm. **RBAC:** none beyond default SA (no access to Postgres pod).

##### B. Outside cluster — wrapper script (no pgwd code change)

Ship **`contrib/k8s/pgwd-kube-run.sh`**: reads Secret with `kubectl`, builds `PGWD_DB_URL`, execs `pgwd` with `-kube-postgres`. Password never in repo; **RBAC on the operator identity** (human CI), not on a long-lived pgwd ServiceAccount with `pods/exec`.

```bash
# Example pattern (script wraps error handling + port defaults)
SECRET_NS=default SECRET_NAME=postgres-credentials SECRET_KEY=password \
  DB_USER=postgres DB_NAME=mydb DB_PORT=5432 \
  ./contrib/k8s/pgwd-kube-run.sh \
    -kube-postgres default/svc/postgres \
    -client prod -interval 60 \
    -notifications-slack-webhook "$WEBHOOK"
```

Cron/systemd: same script; kubeconfig + `kubectl` on PATH.

##### C. Outside cluster — `kube.password_from_secret` (optional pgwd feature, 0.9)

**Replacement inside pgwd** for users who want config-file ergonomics without exec:

```yaml
kube:
  postgres: default/svc/postgres
  password_from_secret:
    namespace: default
    name: postgres-credentials
    key: password          # or key: url → full DSN, skip URL assembly
db:
  url: "postgres://postgres@localhost:5432/mydb?sslmode=disable"  # no password in URL
```

Implementation: client-go **`secrets.Get`** (read only), assemble DSN locally. **RBAC for pgwd SA:**

```yaml
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    resourceNames: ["postgres-credentials"]
    verbs: ["get"]
  - apiGroups: [""]
    resources: ["pods", "services"]
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["pods/portforward"]
    verbs: ["create"]
```

No `pods/exec`. Namespace-scoped Role, not ClusterRole. Document in `contrib/k8s/rbac-outside-cluster.yaml`.

**Decision:** ship in **0.9.0** (with wrapper script as secondary path).

##### D. Outside cluster — pre-export env (manual / CI)

One-liner before `pgwd` (documented, no new code):

```bash
export PGWD_DB_URL="postgres://postgres:$(kubectl get secret postgres-credentials -n default \
  -o jsonpath='{.data.password}' | base64 -d)@localhost:5432/mydb?sslmode=disable"
pgwd -kube-postgres default/svc/postgres -client prod ...
```

##### E. Enterprise sync

External Secrets Operator, Vault Agent, Sealed Secrets → file or env consumed by pgwd or wrapper. Point to patterns; no pgwd-specific code required.

##### F. Direct TCP (no kube)

If Postgres is reachable without port-forward: Secret → `PGWD_DB_URL` or `~/.pgpass` (mode `0600`). Out of kube scope.

#### Comparison

| Method | pgwd outside cluster | Password in git/config | pgwd needs `pods/exec` | pgwd needs `secrets get` |
|--------|---------------------|------------------------|-------------------------|---------------------------|
| ~~DISCOVER_MY_PASSWORD~~ | yes | placeholder only | **yes** | no |
| Wrapper script | yes | no | no | no (kubectl in shell) |
| `password_from_secret` | yes | no | no | **yes** (scoped) |
| In-cluster Secret env | no (inside) | no | no | no (kube mounts env) |
| Manual `kubectl` export | yes | no | no | no |

#### Deliverables (0.9.x)

| Item | Action |
|------|--------|
| `docs/kubernetes-passwords.md` | **Created** — decision record + migration + alternatives |
| `contrib/k8s/pgwd-kube-run.sh` | **Create** — wrapper for outside-cluster |
| `contrib/k8s/rbac-outside-cluster.yaml` | **Create** — SA + Role (port-forward + optional secrets get) |
| `contrib/profiles/kube-prod.yml` | Secret/wrapper pattern, no placeholder |
| `internal/kube/` + config | Remove exec discovery; add `password_from_secret` (**0.9.0**) |
| `testing/scripts/test-e2e-kube.sh` | Use kubectl secret read or `password_from_secret`; drop DISCOVER retries |
| `contrib/k8s/README.md` | Link passwords doc; outside vs in-cluster split |
| pgwd-selfhosted (follow-up) | Chart values + RBAC without exec |
| `CHANGELOG.md` | Security removal + migration |
| `SPECIFICATIONS.md` §9 | Remove DISCOVER; document `password_from_secret` if shipped |
| [plan-1.0.x.md](./plan-1.0.x.md) | Note: DISCOVER removed in 0.9 (not deferred to 1.0) |

#### Phasing (suggested)

1. **0.9.0 (develop):** `password_from_secret`, `docs/kubernetes-passwords.md`, wrapper, RBAC sample, profile; e2e migrated; remove DISCOVER; fail fast on old placeholder.

### 11. Documentation and SPEC audit

Finish [SPECIFICATIONS.md](../SPECIFICATIONS.md) audit against **0.9.x** code. Checklist:

- [ ] §8 HTTP: `healthz` body is plain `ok`; operator security notes current
- [ ] §8 HTTP: Prometheus label escaping spec matches implementation (or document fix shipped in 0.9.x)
- [ ] §10 CSV: column list matches `internal/metricsexport/csv.go`; formula-injection note current
- [ ] §6 Notifications: TLS operator responsibility documented
- [ ] K8s client-go, metrics store interface, CSV columns, config load order
- [ ] Collector privacy (§3 / dedicated subsection when collector ships)
- [ ] Sequence diagrams in [docs/sequence/](./sequence/) re-audit if behavior changed

---

## Files to modify

| File | Action |
|------|--------|
| `internal/collector/` | **Create** — anonymous daemon startup telemetry + update check |
| `cmd/pgwd/main.go` | `--strict`, deprecation warnings, `startCollector` when `interval > 0` |
| `internal/config/config.go` | `EnableCollector`, `EnableUpdateCheck`; file/env mapping |
| `internal/validator/validator.go` | Strict-mode validation if needed |
| `internal/kube/` | Remove `DISCOVER_MY_PASSWORD` / exec; add `password_from_secret` (0.9.0) |
| `contrib/k8s/pgwd-kube-run.sh` | **Create** — outside-cluster wrapper |
| `contrib/k8s/rbac-outside-cluster.yaml` | **Create** — least-privilege SA |
| `docs/kubernetes-passwords.md` | **Created** — decision record + migration + alternatives |
| `testing/scripts/test-e2e-kube.sh` | No DISCOVER; secret-based URL |
| `contrib/profiles/` | **Create** profile YAML snippets (kube-prod without DISCOVER) |
| `contrib/pgwd.conf.example` | Remove `kube.password_*`; optional `password_from_secret` block |
| `SPECIFICATIONS.md` | Exit codes, profiles, collector, audited behavior |
| `README.md` | Profiles, strict mode, anonymous usage |
| Various `*_test.go` | Coverage improvements |

---

## Testing

- `make test`, `make lint`, `make test-integration`, `make test-e2e-kube` (no `pods/exec` in e2e path)
- Coverage report: `make cover` meets 70% target (or documented exceptions)
- Platform smoke: at least Ubuntu + one BSD if install paths touched

---

## Open questions

| # | Question | Status |
|---|----------|--------|
| 1 | Logo/branding for 1.0? | Wait for user input on design direction. |
| 2 | Implement distinct exit codes 2–3 (connect vs query) or only 4 in strict? | Defer granular 2/3 to 1.0 or post-1.0 unless needed for cron. |
| 3 | Collector endpoint: dedicated `collect.pgwd…` vs shared backend with `product: pgwd`? | **Shared** — `https://collect.gghstats.com/a1b2c3d4e5f6a7b8` → `project: pgwd` (see collect-gghstats-com.Infrastructure). |
| 4 | Ship `kube.password_from_secret` in 0.9.0 or 0.9.1? | **0.9.0** — with wrapper script as alternate path. |

---

## Release checklist

- [ ] Tag `v0.9.0` from `main`
- [ ] SPEC audited for 0.9.x
- [ ] Profiles shipped under `contrib/profiles/`
- [ ] `DISCOVER_MY_PASSWORD` removed; `docs/kubernetes-passwords.md` + wrapper/RBAC shipped; e2e kube updated
- [ ] CHANGELOG [Unreleased] → 0.9.x section
