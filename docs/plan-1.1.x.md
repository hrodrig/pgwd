# pgwd plan 1.1.x — incident hygiene (on-call ready)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Roadmap index:** [ROADMAP.md](../ROADMAP.md) · **Band:** 1.1.x (minor, non-breaking API; behavior change for alert repeat defaults)

**Goal:** Make sustained outages produce one coherent on-call incident (PagerDuty dedup + resolve) and stop threshold notifiers from spamming every interval while the bad state persists.

**Architecture:** (1) PagerDuty Events API v2 gets a stable `dedup_key` per target/problem and uses `event_action: resolve` for `resolution` events. (2) A new filter in `internal/run` (after hysteresis, before send) drops repeated threshold alerts unless severity escalates or the operator opts back into v1.0 “repeat while firing”. (3) SPEC + CHANGELOG document the new contract; tests lock envelopes and filter behavior.

**Tech Stack:** Go 1.26.x, existing `internal/notify`, `internal/run`, `internal/store`, `httptest` for PD unit tests, SQLite metrics store for latch persistence across process restarts.

**Follow-up:** In-memory latch across daemon ticks without a metrics store is completed in **[plan-1.1.1.md](./plan-1.1.1.md)** (v1.1.0 filter existed; production wiring discarded the closure each tick).

**Baseline:** v1.0.1 on `develop` · **Previous band:** [plan-1.0.x.md](./plan-1.0.x.md) · **Target tag:** v1.1.0 after `make release-check` on `main`

---

## Global Constraints

- English only in code, comments, commits, docs (repo rule).
- Work on `develop`; never commit features to `main`.
- Show proposed commit message; wait for explicit user approval before `git commit`.
- Cyclomatic complexity ≤ 14 (`gocyclo -over 14`); `gofmt -s`; `go vet ./...`.
- Library coverage gate remains ≥ 80% (`make cover-check`).
- No new direct dependencies for this band.
- Stable CLI/env/YAML keys from 1.0 stay; new keys are additive. Default **behavior** for alert repeat changes (document under CHANGELOG **Changed**).
- Do not expand scope into APM, dashboards, or new notifier channels in this band (C5 doc-drift fix is in scope as XS).

---

## Behavior contract (to land in SPECIFICATIONS.md)

### A1 — PagerDuty dedup + resolve

| Case | `event_action` | `dedup_key` |
|------|----------------|------------|
| Threshold (`total`/`active`/`idle`/`stale`) | `trigger` | `pgwd:{client}:{cluster}:{database}:connections` |
| Level escalation (same target) | `trigger` (same key — PD updates open incident) | same as above |
| `resolution` | `resolve` | same `…:connections` key as the open incident |
| `long_query` | `trigger` | `pgwd:{client}:{cluster}:{database}:long_query` |
| `connect_failure` / `too_many_clients` | `trigger` | `pgwd:{client}:{cluster}:{database}:connect` |
| `test` / force-notification | `trigger` | `pgwd:{client}:{cluster}:{database}:test` |

Empty `client` / `cluster` / `database` segments normalize to `_`. Max key length: keep under PD’s 255-char limit (truncate with stable hash suffix only if needed — unlikely for these fields).

**Resolve payload:** PD Events v2 allows resolve with `routing_key`, `dedup_key`, `event_action: resolve`. Include a short `payload.summary` (existing resolution message is fine). Severity on resolve is irrelevant; do not send resolve as `trigger`+`info`.

### A2 — Transition latch (anti-spam)

After `ApplyHysteresisFilter` and `ApplyLongQueryCooldownFilter`, apply **`ApplyFiringRepeatFilter`**:

| Event class | Latch behavior |
|-------------|----------------|
| `total` / `active` / `idle` / `stale` | Send on **enter** bad state or **escalation** (`attention`→`alert`→`danger`). Suppress while same or lower severity persists. |
| `long_query` | Unchanged (existing cooldown). |
| `connect_failure` / `too_many_clients` | Always pass (always-on connect path). |
| `test` / force-notification | Always pass. |
| `resolution` | Unchanged (separate `TrySendResolutionNotification`). |

**Severity rank:** `ok`=0, `attention`=1, `alert`=2, `danger`=3, `connect_failure`=4 (connect path not latched).

**Previous state source (priority):**

1. Metrics store `LastStates(..., 1)` when `st != nil` (survives process restart).
2. Else in-memory last state in the `MakeRunFunc` closure (daemon / one-shot without store).

**Escape hatch (v1.0 spam restored):**

- YAML: `notifications.repeat_while_firing: true`
- Env: `PGWD_NOTIFICATIONS_REPEAT_WHILE_FIRING=true` (when no config file / via ApplyEnv rules)
- CLI: `-notifications-repeat-while-firing`

Default: **`false`** (transition/escalation only).

**Optional (same band if cheap):** `notifications.repeat_cooldown_seconds` (int, 0=off). When >0 and state unchanged, allow one re-alert after N seconds (reuse `AlertCooldownRecorder` with kind `threshold_repeat`). Skip if this grows scope past one PR — park for 1.1.1.

### Out of scope (this band)

- Per-target notifier routing
- Opsgenie / Discord / email
- Mandatory TLS for webhooks
- `slog` migration (post-1.1 idea)
- Dependabot / CI `@latest` pins (Band B from audit triage)
- Multi-DB + kube

---

## File map

| File | Role |
|------|------|
| `internal/notify/pagerduty.go` | `dedup_key`, `event_action` trigger/resolve |
| `internal/notify/pagerduty_test.go` | **Create** — envelope tests via `httptest` |
| `internal/notify/notify.go` | Optional helper docs only; Event unchanged unless a tiny `EventType` helper helps |
| `internal/run/run.go` | `ApplyFiringRepeatFilter` + wire into `MakeRunFunc` |
| `internal/run/run_test.go` | Latch / escalation / escape-hatch tests |
| `internal/config/config.go`, `file.go`, `cli` flags | `RepeatWhileFiring` field + wire |
| `internal/validator/validator.go` | No new hard fails; document defaults |
| `contrib/pgwd.conf.example`, `contrib/profiles/*` | Example key |
| `SPECIFICATIONS.md` | PagerDuty § + hysteresis/repeat § + known-limitations row |
| `CHANGELOG.md` | `[Unreleased]` Added/Changed |
| `README.md`, OCI/nfpm one-liners | C5: full notifier list (Slack/Loki/PagerDuty/Teams/webhook) |
| `ROADMAP.md` | Band status → Ready when tagged |

---

## Design notes (locked)

1. **SPEC bug today:** Known Limitations row claims “hysteresis covers threshold repeats”. False — hysteresis only delays the *first* fire. Replace that row when A2 ships.
2. **One incident per DB for connection pressure** (`…:connections`) is intentional: escalation updates the same PD incident; resolve closes it. Per-threshold keys would leave orphaned incidents when resolve uses `threshold=resolution`.
3. **Default change is product-correct** for on-call; escape hatch preserves v1.0 operators.

---

### Task 1: PagerDuty envelope — failing tests first

**Files:**
- Create: `internal/notify/pagerduty_test.go`
- Modify: `internal/notify/pagerduty.go` (Task 2)

**Interfaces:**
- Consumes: `PagerDuty.Send(ctx, Event)`, existing `httptest` patterns from other notify `*_test.go`
- Produces: tests that require `dedup_key` + resolve action

- [x] **Step 1: Write failing tests** (landed with implementation in `pagerduty_test.go`)

```go
func TestPagerDuty_Send_triggerIncludesDedupKey(t *testing.T) {
	// httptest captures JSON body
	// Event{Threshold:"total", Client:"c", Cluster:"cl", Database:"d", Level:"alert", Message:"m"}
	// assert event_action == "trigger"
	// assert dedup_key == "pgwd:c:cl:d:connections"
}

func TestPagerDuty_Send_resolutionIsResolveAction(t *testing.T) {
	// Event{Threshold:"resolution", Client:"c", Cluster:"cl", Database:"d", Message:"back"}
	// assert event_action == "resolve"
	// assert dedup_key == "pgwd:c:cl:d:connections"
}

func TestPagerDuty_dedupKey_longQueryAndConnect(t *testing.T) {
	// long_query → …:long_query
	// connect_failure → …:connect
	// empty client → _
}

func TestPagerDuty_Send_testUsesTestSuffix(t *testing.T) {
	// Threshold "test" → …:test, event_action trigger
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
go test ./internal/notify/ -run TestPagerDuty_ -count=1
```

Expected: FAIL (missing `dedup_key` / always `trigger`).

- [ ] **Step 3: Stop** — implement in Task 2 (TDD).

---

### Task 2: Implement PagerDuty dedup + resolve

**Files:**
- Modify: `internal/notify/pagerduty.go`
- Test: `internal/notify/pagerduty_test.go`

**Interfaces:**
- Produces: `func pagerDutyDedupKey(ev Event) string`, envelope field `DedupKey string \`json:"dedup_key"\``

- [ ] **Step 1: Extend envelope**

```go
type pagerDutyEnvelope struct {
	RoutingKey  string           `json:"routing_key"`
	EventAction string           `json:"event_action"`
	DedupKey    string           `json:"dedup_key,omitempty"`
	Payload     pagerDutyPayload `json:"payload"`
}
```

- [ ] **Step 2: Implement `pagerDutyDedupKey` + `pagerDutyEventAction`**

```go
func pagerDutyEventAction(ev Event) string {
	if ev.Threshold == "resolution" {
		return "resolve"
	}
	return "trigger"
}

func pagerDutyDedupKey(ev Event) string {
	seg := func(s string) string {
		if s == "" {
			return "_"
		}
		return s
	}
	suffix := "connections"
	switch ev.Threshold {
	case "long_query":
		suffix = "long_query"
	case "connect_failure", "too_many_clients":
		suffix = "connect"
	case "test":
		suffix = "test"
	case "resolution":
		suffix = "connections"
	}
	return fmt.Sprintf("pgwd:%s:%s:%s:%s", seg(ev.Client), seg(ev.Cluster), seg(ev.Database), suffix)
}
```

- [ ] **Step 3: Wire into `Send`** — set `EventAction` and `DedupKey` from helpers.

- [ ] **Step 4: Run tests — expect PASS**

```bash
go test ./internal/notify/ -run TestPagerDuty_ -count=1
```

- [ ] **Step 5: Commit** (after user approves message)

```
fix(notify): PagerDuty dedup_key and resolve action

Close incidents on resolution; one key per target problem class.
```

---

### Task 3: Firing repeat filter — failing tests

**Files:**
- Modify: `internal/run/run_test.go`
- Modify: `internal/run/run.go` (Task 4)
- Modify: `internal/config/config.go` (field stub OK in Task 4)

**Interfaces:**
- Produces: `ApplyFiringRepeatFilter(ctx, st, cfg, client, cluster, db, prevState, events) []notify.Event`

- [ ] **Step 1: Write failing tests**

```go
func TestApplyFiringRepeatFilter_suppressesSameState(t *testing.T) {
	// prevState "alert", events [{Threshold:total, Level:alert}]
	// cfg.RepeatWhileFiring == false
	// want: filtered empty (or only non-threshold)
}

func TestApplyFiringRepeatFilter_allowsEscalation(t *testing.T) {
	// prevState "attention", event Level "alert" → pass
}

func TestApplyFiringRepeatFilter_allowsFirstFire(t *testing.T) {
	// prevState "ok" or "" → pass
}

func TestApplyFiringRepeatFilter_repeatWhileFiringBypass(t *testing.T) {
	// cfg.RepeatWhileFiring true → pass even if same state
}

func TestApplyFiringRepeatFilter_keepsConnectAndLongQuery(t *testing.T) {
	// connect_failure / long_query always retained
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./internal/run/ -run TestApplyFiringRepeatFilter_ -count=1
```

---

### Task 4: Implement filter + wire + config

**Files:**
- Modify: `internal/run/run.go` (`MakeRunFunc` pipeline)
- Modify: `internal/config/config.go`, `file.go`, `ApplyEnv` / CLI in `internal/cli/cli.go`
- Modify: `internal/config/config_test.go`, `file` tests as needed
- Modify: `contrib/pgwd.conf.example`

**Interfaces:**
- Config field: `RepeatWhileFiring bool` // yaml `notifications.repeat_while_firing`
- Pipeline order: hysteresis → long_query cooldown → **firing repeat** → `SendEvents`

- [ ] **Step 1: Add config field + YAML/env/CLI** (default false).

- [ ] **Step 2: Implement `severityRank` + `ApplyFiringRepeatFilter`**

Logic sketch:

```go
func ApplyFiringRepeatFilter(cfg *config.Config, prevState string, events []notify.Event) []notify.Event {
	if cfg.RepeatWhileFiring || prevState == "" || prevState == "ok" {
		return events
	}
	cur, _ := checker.StateAndThresholdFromEvents(events)
	// If only long_query/connect remain after splitting, handle carefully:
	// compute current threshold-event severity from threshold events only.
	if severityRank(cur) > severityRank(prevState) {
		return events // escalation
	}
	if severityRank(cur) < severityRank(prevState) {
		return events // de-escalation notify (optional product choice: YES — operator sees improvement before full OK)
	}
	// same rank: drop total/active/idle/stale only
	var out []notify.Event
	for _, e := range events {
		switch e.Threshold {
		case "total", "active", "idle", "stale":
			continue
		default:
			out = append(out, e)
		}
	}
	return out
}
```

**Product lock:** same-rank suppress; **escalation and de-escalation both notify**; resolution path unchanged.

- [ ] **Step 3: Wire `prevState`** in `MakeRunFunc`:

```go
prev := ""
if st != nil {
	if last, err := st.LastStates(ctx, client, cluster, db, 1); err == nil && len(last) > 0 {
		prev = last[0]
	}
}
// fallback: closure var lastState string
res.Events = ApplyFiringRepeatFilter(cfg, prev, res.Events)
// after successful path, lastState = state from post-filter events / inserted record
```

Use store when available; keep `var lastState string` in closure for no-store daemon.

- [ ] **Step 4: Tests PASS**

```bash
go test ./internal/run/ ./internal/config/ ./internal/notify/ -count=1
```

- [ ] **Step 5: Commit** (after approval)

```
feat(run): suppress repeated threshold alerts while firing

Default transition/escalation only; opt in with repeat_while_firing.
```

---

### Task 5: SPEC + CHANGELOG + doc drift (C5)

**Files:**
- Modify: `SPECIFICATIONS.md` (§ PagerDuty, hysteresis/notifications, Known Limitations row)
- Modify: `CHANGELOG.md` `[Unreleased]`
- Modify: `README.md` one-liner / badges area if it still says “Slack/Loki” only
- Grep: Dockerfile `LABEL`, `.goreleaser*` nfpm descriptions — update channel list
- Modify: `ROADMAP.md` band status when ready to tag (keep ⬜ until implementation done)

- [ ] **Step 1: SPEC edits**

  - PagerDuty: document `dedup_key` table + `resolve` on resolution.
  - Notifications: document default transition latch + `repeat_while_firing`.
  - Known Limitations: replace “Per-threshold cooldown not planned (hysteresis covers…)” with accurate text (latch shipped; optional cooldown future).

- [ ] **Step 2: CHANGELOG `[Unreleased]`**

```markdown
### Added
- **PagerDuty:** stable `dedup_key` per target/problem; `event_action: resolve` on resolution.
- **Config:** `notifications.repeat_while_firing` (CLI/env) to restore interval repeats.

### Changed
- **Alert repeat:** by default, threshold notifiers fire on transition/escalation/de-escalation only (not every interval while bad). Use `repeat_while_firing: true` for v1.0 behavior.
```

- [ ] **Step 3: C5 doc drift** — full notifier list in README/OCI/nfpm blurbs.

- [ ] **Step 4: `go test ./...` + `make lint`**

- [ ] **Step 5: Commit** (after approval)

```
docs: SPEC/CHANGELOG for 1.1 incident hygiene; fix notifier one-liners
```

---

### Task 6: Band closure gate

- [ ] `make lint`
- [ ] `go test ./...`
- [ ] `make cover-check` (Docker)
- [ ] Update `ROADMAP.md` 1.1.x status → ✅ Ready when tagging
- [ ] Bump `VERSION` to `1.1.0` only at release time (separate release PR): man page, demo GIF per release rules
- [ ] `make release-check` on `main` before tag

---

## Acceptance criteria

1. Sustained `alert` state for 1h at 60s interval → **one** PD incident (triggers may update same `dedup_key`), **one** Slack/Loki message at enter (plus escalations), then silence until de-escalation/resolve.
2. Resolution → PD incident auto-closes (`resolve` + same `dedup_key`).
3. `repeat_while_firing: true` restores per-interval threshold notifications.
4. `force-notification` and connect-failure paths still always notify.
5. All gates green; SPEC matches code.

---

## Suggested PR split

| PR | Tasks | Notes |
|----|-------|-------|
| PR1 | 1–2 | PD only — safe alone |
| PR2 | 3–4 | Latch + config |
| PR3 | 5–6 | Docs + polish |

Or one PR if small enough for review.

---

*Plan authored 2026-08-09 from audit triage. No application code changed in the planning commit unless bundled with ROADMAP band open.*
