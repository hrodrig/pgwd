# pgwd plan 1.0.x — breaking stable API

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

## Documentation

| Artifact | Action |
|----------|--------|
| `SPECIFICATIONS.md` | Remove "deprecated" and "planned" notes; 1.0 is the contract |
| `README.md` | Badges, version, breaking upgrade link |
| `CHANGELOG.md` | 1.0.0 section with full breaking list |
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
