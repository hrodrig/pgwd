# Testing strategy for cmd/pgwd (main.go)

**Objective:** Increase coverage of `main.go` (currently ~0%) toward the v1.0.0 criterion of 100+ tests.

---

## Current state

| Package        | Tests | Notes                             |
|----------------|-------|-----------------------------------|
| internal/config | ✅   | FromEnv, file parsing, helpers    |
| internal/notify | ✅   | Slack, Loki, event formatting     |
| internal/postgres | ✅  | Stats queries                     |
| internal/store   | ✅   | SQLite CRUD, LastStates           |
| internal/kube    | ✅   | ParseKubePostgres, etc.           |
| **cmd/pgwd**     | ✅   | Black-box tests (version, help, validation) + logic in checker |

`main.go` (~876 lines, 45+ functions) contains:

- **Pure logic**: levelFromPercent, levelToLabel, title, allStringsEqual, stateAndThresholdFromEvents
- **Config logic**: applySingleThresholdDefaults, validateThresholdConfig, collectLevelModeEvent, collectExplicitThresholdEvents, baseEvent
- **Validation**: validateConfig and 9 helpers — all use `log.Fatal` (hard to unit-test)
- **Integration**: setupKube, doRunCheck, applyHysteresisFilter, etc. — need pool, store, kube

---

## Strategy: extract + black-box

### Phase 1: Extract pure logic to `internal/checker` ✅ (done)

Create `internal/checker` with functions that have **zero external deps** (no DB, no kube, no store). These are easy to test with table-driven tests.

| Function in main.go              | Move to checker                 | Test cases |
|----------------------------------|--------------------------------|------------|
| `levelFromPercent(percent, levels)` | `LevelFromPercent`             | 0%, 74%, 75%, 84%, 85%, 94%, 95%, 100% vs [75,85,95] |
| `levelToLabel(level)`            | `LevelToLabel`                 | 0,1→attention, 2→alert, 3,4,5→danger |
| `title(s)`                       | `Title`                        | "", "hello", "HELLO" |
| `allStringsEqual(sl, v)`         | `AllStringsEqual`              | [], [""], [a,a,a], [a,b] |
| `stateAndThresholdFromEvents`    | `StateAndThresholdFromEvents`  | empty events, connect_failure, danger, alert, attention |
| `applySingleThresholdDefaults`   | `ApplySingleThresholdDefaults`| percent 0/50/100, maxConn 100, thresholds 0 |
| `validateThresholdConfig`       | `ValidateThresholdConfig`     | level mode + maxConn 0, no thresholds, dry-run |
| `collectLevelModeEvent`          | `CollectLevelModeEvent`       | levels [75,85,95], total/active % combinations |
| `collectExplicitThresholdEvents` | `CollectExplicitThresholdEvents` | total/active thresholds exceeded |
| `baseEvent`                      | `BaseEvent`                    | stats, maxConn, override, labels |

**Effort:** Low. Copy functions, fix imports, add `*_test.go`. `main.go` imports checker and delegates.

---

### Phase 2: Extract validation to `internal/validator` (return error, not log.Fatal)

Create `internal/validator` with functions that **return `error`** instead of calling `log.Fatal`. `main.go` calls them and does `if err != nil { log.Fatal(err) }`.

| Function in main.go     | Move to validator       | Test cases |
|-------------------------|-------------------------|------------|
| `validateConfig`        | `Validate(cfg)`         | orchestrates sub-validators |
| `validateDatabases`     | `ValidateDatabases`     | empty OK, missing URL → error, kube+databases → error |
| `validateClient`        | `ValidateClient`       | empty → error |
| `validateDBURL`         | `ValidateDBURL`        | empty (single-DB) → error |
| `validateStale`         | `ValidateStale`        | threshold-stale > 0 and stale-age ≤ 0 → error |
| `validateNotifiers`     | `ValidateNotifiers`    | no notifier + !dry-run → error |
| `validateKubePostgres`  | `ValidateKubePostgres` | invalid format → error |
| `validateKubeLoki`      | `ValidateKubeLoki`     | invalid format → error |

**Effort:** Medium. Refactor each validate function to return `error`; add table-driven tests. `main` keeps a thin wrapper that logs and exits on error.

---

### Phase 3: Black-box tests for the binary ✅ (done)

Add `cmd/pgwd/main_test.go` or `cmd/pgwd/integration_test.go` (or `test/integration/`):

| Test                       | How                                | Assertion |
|----------------------------|------------------------------------|-----------|
| `TestMain_Version`         | `exec.Command(binary, "-version")` | stdout contains "pgwd" |
| `TestMain_VersionLong`     | `exec.Command(binary, "--version")`| same |
| `TestMain_Help`            | `exec.Command(binary, "-h")`       | stdout contains "pgwd" |
| `TestMain_MissingClient`   | `exec.Command(binary, "-config", "/dev/null", "-db-url", "postgres://...")` | exit code ≠ 0 |
| `TestMain_MissingDBURL`    | `exec.Command(binary, "-config", "/dev/null")` | exit code ≠ 0 |

**Effort:** Low. Requires `go build` before test, or use `TestMain` to build once. See `testing/README.md` for local setup.

---

### Phase 4 (optional): Integration tests with mocks

For `applyHysteresisFilter`, `trySendResolutionNotification`, `doRunCheck`:

1. **Store mock**: Define interface `LastStatesProvider` with `LastStates(ctx, client, cluster, db, n) ([]string, error)`. Real `*store.Store` implements it. Tests use a fake that returns predefined slices.
2. **Pool mock**: Harder — `doRunCheck` uses `postgres.Stats`, `postgres.MaxConnections`, `postgres.StaleCount`. Options:
   - Testcontainers + real Postgres (heavy, use `-short` skip)
   - Inject a `StatsFunc` / `MaxConnFunc` if we introduce interfaces (larger refactor)

**Effort:** Medium–high. Start with store mock for hysteresis/resolution; defer pool integration tests unless critical.

---

## Recommended order

1. **Phase 1** — Quick wins, high coverage, no behavior change.
2. **Phase 3** — Simple black-box tests for version/help and fatal validation paths.
3. **Phase 2** — Validation extraction; enables more precise tests.
4. **Phase 4** — Only if time permits or for specific regression coverage.

---

## Files to create/modify

| Action  | Path |
|---------|------|
| Create | `internal/checker/checker.go` |
| Create | `internal/checker/checker_test.go` |
| Create | `internal/validator/validator.go` |
| Create | `internal/validator/validator_test.go` |
| Create | `cmd/pgwd/main_test.go` (black-box) |
| Modify | `cmd/pgwd/main.go` (import checker, validator; delegate) |

---

## Acceptance

- `go test ./...` passes.
- Coverage of `cmd/pgwd` increases from ~0% to at least 30–40% (Phase 1+3), or 50%+ with Phase 2.
- No change to user-visible behavior.
- AGENTS.md updated to note that cmd/pgwd has unit tests (via checker/validator) and black-box tests.
