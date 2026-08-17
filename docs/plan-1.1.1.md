# pgwd plan 1.1.1 — contract repair (patch)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Roadmap index:** [ROADMAP.md](../ROADMAP.md) · **Band:** 1.1.1 (patch; restores promises of [plan-1.1.x.md](./plan-1.1.x.md) + Go toolchain)

**Goal:** Make the v1.1.0 “quiet by default” latch work across daemon ticks **without** a metrics store; align SPEC with dry-run and connection counting; fix broken post-1.0 YAML examples; ship **Go 1.26.6** (already on `develop`).

**Architecture:** (1) Daemon keeps per-target check runner state (`MakeRunFunc` closure / `memPrev`) across ticker intervals instead of creating and discarding a new closure every tick. (2) `postgres.Stats` excludes the monitor backend (`pid <> pg_backend_pid()`), matching `LongQueryCount`. (3) SPEC documents that `-dry-run` sends **no** outbound HTTP (including connect failure). (4) Operator-facing YAML examples use `databases:` and current key names only.

**Tech Stack:** Go 1.26.6, existing `internal/cli`, `internal/run`, `internal/postgres`, `internal/store` (optional).

**Baseline:** v1.1.0 on `main` / `develop` (+ commit bumping `go.mod` to 1.26.6) · **Previous band:** [plan-1.1.x.md](./plan-1.1.x.md) · **Target tag:** `v1.1.1` after `make release-check` on `main`

---

## Global Constraints

- English only in code, comments, commits, docs (repo rule).
- Work on `develop`; never commit features to `main`.
- Show proposed commit message; wait for explicit user approval before `git commit`.
- Cyclomatic complexity ≤ 14 (`gocyclo -over 14`); `gofmt -s`; `go vet ./...`.
- Library coverage gate remains ≥ 80% (`make cover-check`).
- No new direct dependencies.
- No Dependabot / `KnownFields` / packaging one-liner sweep beyond broken YAML examples (those belong in [plan-1.2.x.md](./plan-1.2.x.md)).
- Do not require metrics store for latch; store remains preferred when configured.

---

## Behavior contract (to land in SPECIFICATIONS.md)

### Latch across daemon ticks (no store)

| Mode | Previous firing state source |
|------|------------------------------|
| Metrics store configured | `LastStates(..., 1)` (unchanged; survives process restart) |
| No store, **daemon** | In-memory state held for the lifetime of the daemon loop **per target** (survives ticker intervals) |
| No store, one-shot | Single check; latch N/A across process runs |

**Bug today:** `runOneTarget` invokes `MakeRunFunc(...)()` once and drops the closure; `runTickerLoop` calls `runOneTarget` every interval → `memPrev` always empty → `ApplyFiringRepeatFilter` never suppresses.

**Fix:** Reuse the check runner (or an equivalent per-target `memPrev` + stable `MakeRunFunc`) across ticks. Pool lifecycle may be created once per target in daemon mode (preferred) or equivalent; one-shot behavior unchanged.

### Dry-run and connect failure

| Path | `-dry-run` |
|------|------------|
| Threshold / resolution / long_query / force-notification | No outbound HTTP (log `[dry-run] would send: …` where applicable) |
| `connect_failure` / `too_many_clients` | **Also** no outbound HTTP |

Remove SPEC/CHANGELOG claims that infrastructure failures “bypass dry-run”. Code in `NotifyConnectFailure` already returns early on `cfg.DryRun`.

### Connection Stats

- **Total / active / idle:** exclude the monitor’s own backend (`pid <> pg_backend_pid()`), as already documented for Total and as already implemented for long-query counts.

### Docs / examples

- No `db:` key in current operator examples (removed in v1.0; hard load error).
- Slack YAML key is `notifications.slack.webhook` (not `webhook_url`).
- HTTP health path key is `http.healthz_path` (not `health_path`).

---

## Out of scope (this band)

- Dependabot / Renovate / pinning CI `@latest` tools → [plan-1.2.x.md](./plan-1.2.x.md)
- `yaml.KnownFields(true)`, `envInt` fail-loud → 1.2.x
- `ReadHeaderTimeout`, OCI/nfpm “five channels” one-liners → 1.2.x
- `nokube` build tag, `slog`, new notifiers, per-DB kube

---

## File map

| File | Role |
|------|------|
| `internal/cli/cli.go` | Daemon tick loop: reuse runner / pool per target |
| `internal/run/run.go` | Unchanged filter logic if wiring fixed; adjust only if API needed |
| `internal/cli/*_test.go` or dedicated daemon-path test | Two ticks, **no store**, same severity → no second threshold send |
| `internal/postgres/stats.go` | Exclude `pg_backend_pid()` |
| `internal/postgres/stats_*_test.go` | Cover self-exclusion (unit/mock or integration) |
| `SPECIFICATIONS.md` | Latch in-memory wording; dry-run table; Stats |
| `CHANGELOG.md` | `[1.1.1]` Security (Go) + Fixed |
| `docs/kubernetes-passwords.md`, `contrib/k8s/README.md`, `contrib/netbsd/README.md`, `contrib/solaris/README.md` | Replace `db:` / wrong keys in “current” examples |
| `VERSION`, README badge/install, `contrib/man/man1/pgwd.1`, BSD ports | 1.1.1 sync |
| `ROADMAP.md` | Band status |
| `docs/plan-1.1.x.md` | Pointer: in-memory cross-tick latch completed in 1.1.1 |

---

## Design notes (locked)

1. **Prefer fix wiring over “require sqlite”.** SPEC already promises in-memory daemon state; requiring a store would be a product regression for minimal installs.
2. **Existing `TestMakeRunFunc_*` that call `fn(); fn()` on one closure stay valid** but do **not** prove production wiring. Add a test that exercises the **daemon path** (or extracted helper used by `runTickerLoop`) without store.
3. **Dry-run = zero HTTP** matches README “no HTTP calls” and the name dry-run; SPEC was wrong.
4. **Go 1.26.6** may already be on `develop`; still document under `[1.1.1]` Security when tagging.

---

### Task 1: Failing test — latch without store across two daemon ticks

**Files:**
- Modify or create: test under `internal/cli` or `internal/run` that mirrors production wiring (new helper OK if it keeps `gocyclo` ≤ 14)

**Want:**

```text
st == nil
repeat_while_firing == false
tick 1: threshold alert → Send called
tick 2: same severity → Send NOT called for connection-threshold events
```

Must **fail** on current `runOneTarget` + `runTickerLoop` behavior (or fail until helper used by production is fixed).

- [ ] **Step 1: Write failing test**
- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./internal/cli/ ./internal/run/ -count=1 -run Latch|Daemon|TwoTick
```

---

### Task 2: Fix daemon wiring

**Files:**
- Modify: `internal/cli/cli.go` (`runTickerLoop` / `runOneTarget`)

**Approach (pick one; prefer A):**

- **A:** Per target, create pool once, `fn := run.MakeRunFunc(...)`, call `fn()` each tick; `defer` close pools on daemon exit.
- **B:** Keep reconnect-per-tick but pass/update a shared `map[targetKey]*string` for `memPrev` into an extended `MakeRunFunc` / check entrypoint.

- [ ] **Step 1: Implement**
- [ ] **Step 2: Test PASS**
- [ ] **Step 3: Commit** (after approval)

---

### Task 3: Stats exclude monitor PID

**Files:**
- Modify: `internal/postgres/stats.go`
- Modify: unit/integration tests

- [ ] **Step 1: Add `AND pid <> pg_backend_pid()` to Stats query**
- [ ] **Step 2: Test + `go test ./internal/postgres/ -count=1`**
- [ ] **Step 3: Commit** (after approval)

---

### Task 4: SPEC + CHANGELOG + YAML example sweep

**Files:**
- `SPECIFICATIONS.md` (dry-run, latch in-memory, Stats)
- `CHANGELOG.md` → section `[1.1.1]`
- Operator examples still showing `db:` / wrong Slack/health keys (SPEC §15 samples, kubernetes/kubernetes-passwords.md “after” examples, contrib READMEs listed in file map)

- [ ] **Step 1: Align SPEC dry-run + latch wording**
- [ ] **Step 2: Fix YAML examples to v1.0+ keys**
- [ ] **Step 3: CHANGELOG `[1.1.1]` Fixed + Security (Go 1.26.6)**
- [ ] **Step 4: Commit** (after approval)

---

### Task 5: Release prep 1.1.1

- [ ] **Step 1:** `VERSION` → `1.1.1`; man `.TH`; README badge/install examples; FreeBSD/OpenBSD port `PORTVERSION` / equivalent
- [ ] **Step 2:** Note in [plan-1.1.x.md](./plan-1.1.x.md); ROADMAP row **1.1.1**
- [ ] **Step 3:** `make release-check` on `main` path per git-flow
- [ ] **Step 4:** PR `develop` → `main`; annotated tag `v1.1.1`

---

## Success criteria

- [ ] Daemon without metrics store does not re-notify same connection-threshold severity every interval
- [ ] New test covers production wiring (not only reused closure)
- [ ] `Stats` excludes monitor PID; SPEC matches
- [ ] Dry-run documented as zero HTTP including connect failure
- [ ] No `db:` in current operator copy-paste examples listed above
- [ ] Tag `v1.1.1` with Go 1.26.6
