# pgwd plan 1.0.x — breaking stable API

**Roadmap index:** [ROADMAP.md](../ROADMAP.md) · **Band:** 1.0.0 (north star)

**Theme:** Remove deprecated surface area; freeze CLI, env, and YAML contract documented in [SPECIFICATIONS.md](../SPECIFICATIONS.md).

**Baseline:** [plan-0.9.x.md](./plan-0.9.x.md) complete (polish, coverage, profiles, **`DISCOVER_MY_PASSWORD` removed in 0.9**)  
**Previous band:** [plan-0.9.x.md](./plan-0.9.x.md)  
**Target:** **v1.0.0** early–mid July 2026

---

## Breaking changes

| Removed | Replacement |
|---------|-------------|
| `-db-threshold-total` / `-db-threshold-active` | `-db-threshold-levels` (3-tier, default `75,85,95`) |
| `-notify-on-connect-failure` flag | Always-on when notifiers configured |
| `PGWD_NOTIFY_ON_CONNECT_FAILURE` env var | Always-on |
| Config key `notify_on_connect_failure` | Removed |
| `db:` config key (single-DB) | `databases:` array (even for one target) |
| Legacy flag names `-threshold-total` / `-threshold-active` | Removed (if any alias remains) |

**Already removed in 0.9.x (not 1.0):** `DISCOVER_MY_PASSWORD`, `-kube-password-var`, `-kube-password-container`, `GetPasswordFromPod` / `pods/exec`. Migration: [kubernetes-passwords.md](./kubernetes-passwords.md).

### Migration

- Single-DB: wrap existing `db:` block as one `databases:` entry (see [UPGRADE-0.5-to-0.6.md](./UPGRADE-0.5-to-0.6.md) patterns; add 1.0 section or new `UPGRADE-0.9-to-1.0.md`)
- Explicit total/active thresholds: switch to `-db-threshold-levels` or `-db-threshold-idle` / `-db-threshold-stale` as needed

---

## Stability contract (from 1.0.0 onward)

- CLI flags, `PGWD_*` env vars, and documented YAML keys are **stable**
- Behavior changes that affect operators require **SPECIFICATIONS.md** update + CHANGELOG
- Breaking changes require **major** semver (2.0.0) with deprecation in at least one prior minor

---

## Milestones (release gate)

- [ ] **100+ tests** total (`go test ./...`)
- [ ] Logo/branding (simple pgwd icon) — optional but targeted for 1.0 announcement
- [ ] [SPECIFICATIONS.md](../SPECIFICATIONS.md) fully audited against 1.0 code
- [ ] All deprecations removed (grep for `Deprecated`, v1.0 warnings)
- [ ] [plan-0.8.x.md](./plan-0.8.x.md) supply chain shipped (SBOM + cosign)
- [ ] Platform tests green on target OSes (`make test-platforms` or documented subset)
- [ ] `make release-check` green on `main`
- [ ] Demo GIF regenerated if VERSION changes (`make install && bash -c "vhs docs/demo.tape"`)
- [ ] Man page [contrib/man/man1/pgwd.1](../contrib/man/man1/pgwd.1) synced

---

## Marketing — transparent comparison (1.0 gate)

Same pattern as **[gfire/docs/compare.md](https://github.com/hrodrig/gfire/blob/main/docs/compare.md)** and **[groot § vs kubectl-gather](https://github.com/hrodrig/groot#groot-vs-kubectl-gather)**: honest matrix, “when to pick”, “when not pgwd”, footnote that competitor claims are best-effort.

### Deliverables

| Artifact | Action |
|----------|--------|
| **`docs/compare.md`** | Full matrix + narratives |
| **`README.md`** | § Compare — short table + link to `docs/compare.md` |
| **`docs/README.md`** | Index link |
| **`docs/use-cases.md`** | Cross-link (“deployment how-to” vs “product choice”) |
| **pgwd-hermesrodriguez-com** | `/compare` page (i18n if site already multi-locale) |
| **CHANGELOG 1.0.0** | Note positioning docs under **Documentation** |

### Alternatives to cover (minimum)

| Alternative | Category | pgwd contrast (honest) |
|-------------|----------|-------------------------|
| **Prometheus `postgres_exporter` + Grafana/Alertmanager** | Metrics stack | pgwd = **connection-focused watchdog**, no Prometheus required; optional `/metrics` export. Exporter = broad SQL metrics, you own alert rules + stack ops. |
| **pgwatch / pgwatch3** | Postgres monitoring suite | pgwd = **single binary**, read-only connection/stale alerts; pgwatch = richer dashboards/metrics, heavier deploy. |
| **Datadog / New Relic / hosted APM** | SaaS observability | pgwd = **self-hosted**, no vendor lock-in, narrow scope, signed supply chain; SaaS = full platform, cost, agent. |
| **AWS CloudWatch / GCP Cloud SQL / Azure** managed DB alarms | Cloud-native | pgwd = **same tool everywhere** (on-prem, K8s, VM, multi-cloud); cloud alarms = per-provider, often connection/limit focused but not pgwd’s notifier/hysteresis story. |
| **cron + `psql` / shell script** | DIY | pgwd = **threshold tiers**, resolution alerts, SQLite history, multi-DB daemon, connect-failure notify; DIY = zero deps but you maintain scripts. |
| **Nagios / check_postgres-style** | Legacy monitoring | pgwd = **Postgres-native** (`pg_stat_activity`), modern notifiers (Slack, Loki, PagerDuty); Nagios = generic plugin ecosystem. |

### Out of scope for compare doc

- Full **query performance** / slow-query platforms (pgwd only has optional **long-query** alerts)
- **Connection poolers** (PgBouncer) as “alternatives” — complementary, not substitutes
- **Incident bundles** (e.g. Groot) — different problem

### Checklist

- [ ] `docs/compare.md` drafted (snapshot version **v1.0.0** in header)
- [ ] README § Compare + TOC entry
- [ ] ROADMAP document map updated (remove “planned” when shipped)
- [ ] Landing `/compare` live or linked from README
- [ ] Competitor feature claims re-verified before tag

---

## Distro packaging (1.x)

**Goal:** get **pgwd** into **official** OS package trees so operators can `apt` / `apk` / `pkg` / `dnf` without downloading GitHub assets. Today: GoReleaser artifacts, Homebrew tap, and **`contrib/`** port seeds — not yet in most official trees.

**Not a hard gate for the first `v1.0.0` tag** (review boards / freeze windows are external). Start submissions when **1.0** is stable; track acceptances through **1.x**.

### Targets

| Distro / OS | Mechanism | Seed / notes |
|-------------|-----------|--------------|
| **Debian** | ITP → unstable → testing | GoReleaser `.deb` is **not** a Debian package; need proper packaging + mentor |
| **Ubuntu** | Sync from Debian or upload | Prefer Debian first |
| **Fedora** | Package review → repos | `.rpm` from GoReleaser ≠ Fedora guidelines |
| **Alpine** | aports MR | OpenRC already in `contrib/openrc` |
| **FreeBSD** | ports Bugzilla | `contrib/freebsd` + [PORT-RELEASE.md](../contrib/freebsd/PORT-RELEASE.md) |
| **OpenBSD** | ports@ diff | `contrib/openbsd` |
| **NetBSD** | pkgsrc | New / adapt from FreeBSD port |
| **DragonFly BSD** | DPorts | `contrib/dragonflybsd` |
| **Arch / openSUSE / …** | As demand | Optional after core set |

### Checklist

- [ ] Inventory: which `contrib/*` ports are submission-ready at v1.0.0
- [ ] FreeBSD: update + submit / refresh Bugzilla if needed
- [ ] OpenBSD: ports@ with makesum + diff
- [ ] Alpine aports draft
- [ ] Debian ITP filed
- [ ] Fedora review request (or COPR interim documented)
- [ ] NetBSD pkgsrc + DragonFly DPorts
- [ ] README “Install” updated when each official path lands
- [ ] Track acceptances in GitHub issues / CHANGELOG under Documentation

---

## Documentation

| Artifact | Action |
|----------|--------|
| `SPECIFICATIONS.md` | Remove "deprecated" and "planned" notes; 1.0 is the contract |
| `README.md` | Badges, version, breaking upgrade link, **§ Compare** (short table → `docs/compare.md`) |
| `docs/compare.md` | **New** — transparent vs alternatives (see Marketing section above) |
| `CHANGELOG.md` | 1.0.0 section with full breaking list + positioning docs |
| `docs/UPGRADE-*` | 0.9 → 1.0 migration guide |
| `contrib/pgwd.conf.example` | Only `databases:` canonical form |

---

## Release process

1. Merge `develop` → `main` when all milestones checked
2. On `main`: annotated tag `v1.0.0`
3. `make release` (runs `release-check`, goreleaser)
4. Push tag; verify CI + Security workflow green

---

## Post-1.0 (out of scope for this plan)

- Per-database `kube.postgres` in `databases:`
- Prometheus native exporter (today: text `/metrics`)
- Notifier plugin system
- See README roadmap / future GitHub issues

---

## Timeline (context)

```
Jun 17 ─ v0.6.10
Jun 18–25 ─ 0.7.x  [plan-0.7.x.md]
Jun 25–30 ─ 0.8.x  [plan-0.8.x.md]
Jul 1–7   ─ 0.9.x  [plan-0.9.x.md]
Jul 8–10  ─ 1.0.0  [this plan]
```

Each band: design → implement → test → review → docs → release.

---

## Changelog mapping

When releasing **1.0.0**:

- Move all `[Unreleased]` items into `## [1.0.0]`
- Reference band plans in release notes where helpful
- Archive completed plan checklists (optional: mark milestones `[x]` in repo)
