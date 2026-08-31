# pgwd plan 1.2.x — audit hygiene (minor)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Roadmap index:** [ROADMAP.md](../ROADMAP.md) · **Band:** 1.2.x (minor; process + config honesty + docs/packaging sync)

**Goal:** Automate dependency and Actions updates; make config parsing fail honestly; harden optional HTTP server timeouts; sync public packaging/docs one-liners and man/README drift left after 1.1.x.

**Architecture:** (1) GitHub Dependabot for `gomod` and `github-actions` with grouped PRs. (2) Strict YAML (`KnownFields`) and non-silent env int/duration parse. (3) Log store errors that today are discarded with `_` on hysteresis/resolution paths. (4) `http.Server` timeouts. (5) Docs/OCI/nfpm/Homebrew describe all five notifiers; README image details match distroless.

**Tech Stack:** Existing Go module; GitHub Dependabot; no new runtime dependencies required.

**Baseline:** v1.1.1 (contract repair) · **Previous band:** [plan-1.1.1.md](./plan-1.1.1.md) · **Target tag:** `v1.2.0` after `make release-check` on `main`

---

## Global Constraints

- English only in code, comments, commits, docs (repo rule).
- Work on `develop`; never commit features to `main`.
- Show proposed commit message; wait for explicit user approval before `git commit`.
- Cyclomatic complexity ≤ 14; `gofmt -s`; `go vet ./...`.
- Library coverage gate ≥ 80% (`make cover-check`).
- `KnownFields(true)` is **operator-visible**: configs with unknown YAML keys start failing load — document under CHANGELOG **Changed** and a short UPGRADE note or README tip.
- Do not flip `enable_update_check` default in this band (separate product decision).
- Do not add `nokube` build tag, `slog` migration, or new notifier channels.

---

## Behavior contract (to land in SPECIFICATIONS.md / docs)

### Dependabot

- Repo ships `.github/dependabot.yml` for:
  - `package-ecosystem: gomod` (directory `/`)
  - `package-ecosystem: github-actions` (directory `/`)
- Weekly schedule; group Go module updates where Dependabot allows to reduce PR noise.

### Config honesty

| Input | Behavior after 1.2 |
|-------|-------------------|
| Unknown YAML keys | Load error (`KnownFields(true)`) |
| `PGWD_*` int/duration set but unparseable | Do **not** silently use `0`; reject or keep previous/default with explicit log/error (prefer fail validation / skip apply with warning — pick one and lock in Task 2) |
| `LastStates` / MaxConn / Stale query errors | Log at error or warn; do not fail open without signal |

### HTTP server

- When `http.listen` is set: `ReadHeaderTimeout` (and sensible `ReadTimeout` / `IdleTimeout`) on `http.Server`.

### Public surface sync

- OCI / nfpm / Homebrew descriptions list Slack, Loki, PagerDuty, Teams, and generic webhook (not “Slack/Loki” only).
- README “Image details” describes distroless runtime (not Alpine BusyBox runtime).
- Man page: CSV export not “SQLite only”; document `-notifications-repeat-while-firing` and other missing flags as needed.
- README: remove obsolete `-kube-password-*` rows; add 1.1 latch flag to parameter table if still missing.
- `docs/compare.md` snapshot version line → 1.2.0 era.

---

## Out of scope (this band)

- Latch / Stats / dry-run contract fixes → [plan-1.1.1.md](./plan-1.1.1.md)
- `nokube` binary split, Patroni, per-DB kube, Discord/email
- Blocking `golangci-lint` (optional non-blocking job may be parked)
- Query `statement_timeout` / `db.query_timeout` (nice-to-have; park unless XS leftover)
- Marketing / co-maintainer process

---

## File map

| File | Role |
|------|------|
| `.github/dependabot.yml` | **Create** |
| `.github/workflows/*.yml`, `GNUmakefile` | Pin tool versions where `@latest` today |
| `internal/config/file.go` | `yaml.NewDecoder` + `KnownFields(true)` |
| `internal/config/config.go` | `envInt` / `envDuration` honesty |
| `internal/config/*_test.go` | Unknown key fails; bad env behavior |
| `internal/run/run.go` | Log discarded store/query errors |
| `internal/httpsrv/server.go` | Server timeouts |
| `Dockerfile`, `Dockerfile.release`, `.goreleaser.yaml` | Description strings |
| `README.md`, `contrib/man/man1/pgwd.1`, `docs/compare.md` | Drift fixes |
| `contrib/systemd/pgwd.service` | Optional XS hardening |
| `SPECIFICATIONS.md`, `CHANGELOG.md`, `ROADMAP.md`, `VERSION` | 1.2.0 |

---

## Design notes (locked)

1. **Dependabot over Renovate** for this band (native GitHub, one YAML file). Revisit Renovate only if grouping/noise becomes painful.
2. **KnownFields** may break configs that carried typo keys or forward-compat junk — that is intended; document migration (“remove unknown keys”).
3. **envInt:** returning `0` on parse failure is worse than default (turns interval into one-shot). Prefer: if env set and parse fails → log + keep `def`, **or** fail validation at startup when that key was required. Lock in Task 2 implementation notes.
4. Pinning Actions by SHA is desirable; if too heavy for one PR, pin tool **install** versions in Makefile/CI first and leave Action SHA pinning as follow-up checkbox.

---

### Task 1: Dependabot + tool pins

- [ ] **Step 1: Add `.github/dependabot.yml`** (`gomod` + `github-actions`, weekly, groups for gomod)
- [ ] **Step 2: Replace `@latest` govulncheck/gocyclo (and document goreleaser-action version policy)**
- [ ] **Step 3: Commit** (after approval)

---

### Task 2: KnownFields + env parse + store error logs

- [ ] **Step 1: Failing tests** — unknown YAML key; `PGWD_INTERVAL=abc` does not become `0`
- [ ] **Step 2: Implement decoder KnownFields + envInt/envDuration fix**
- [ ] **Step 3: Log errors instead of `_` on LastStates / related paths in `internal/run`**
- [ ] **Step 4: Tests PASS; commit** (after approval)

---

### Task 3: HTTP timeouts

- [ ] **Step 1: Set `ReadHeaderTimeout` (e.g. 10s) on `http.Server` in `httpsrv`**
- [ ] **Step 2: Test or smoke that server still starts; commit** (after approval)

---

### Task 4: Docs / packaging sync

- [ ] **Step 1: OCI / goreleaser / Homebrew descriptions → five notifiers**
- [ ] **Step 2: README image details → distroless; parameter table + drop dead kube-password flags**
- [ ] **Step 3: Man page CSV + missing flags; compare.md snapshot**
- [ ] **Step 4: Optional systemd hardening XS**
- [ ] **Step 5: Commit** (after approval)

---

### Task 5: Release 1.2.0

- [ ] **Step 1:** `VERSION` 1.2.0; CHANGELOG; SPEC notes; ROADMAP; man/badge/ports
- [ ] **Step 2:** `make release-check`
- [ ] **Step 3:** PR → `main`; tag `v1.2.0`

---

## Success criteria

- [ ] Dependabot config merged; first PR cadence possible
- [ ] Unknown YAML keys fail load; bad env ints do not silently become `0`
- [ ] HTTP metrics server has header timeout
- [ ] Public one-liners and README/man match distroless + five notifiers
- [ ] Tag `v1.2.0`
