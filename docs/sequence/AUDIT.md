# Sequence diagrams — audit vs code

Last audit: checked each diagram against `cmd/pgwd/main.go` and `internal/notify`, `internal/postgres`.

## 01 — Startup and config validation

| Diagram step | Code |
|--------------|------|
| User runs pgwd (CLI args) | `main()`, `flag.Parse()` |
| read PGWD_* vars | `config.FromEnv()` before flags |
| config.FromEnv + flag.Parse (CLI overrides) | Lines 46, 50–71 |
| validate DB URL present | 78–80 `log.Fatal("missing database URL...")` |
| validate stale-age if threshold-stale | 82–84 |
| validate at least one notifier (or dry-run) | 85–87 |
| validate force-notification / notify-on-connect-failure require notifier | 88–93 |
| signal.NotifyContext(SIGINT, SIGTERM) | 99 |
| opt -kube-postgres: resolve pod, get password, port-forward, replace DB URL | 103–137 |
| compute run context (cluster, client, namespace, database) | 139–166 |
| build senders (Slack, Loki) | 169–178 |
| Pool(ctx, dbURL) | 180 |
| connect error → opt Send(connect_failure) then log.Fatalf | 181–200 |
| MaxConnections, apply default thresholds when total/active 0 | 211–229 |
| validate at least one threshold or dry-run or force-notification | 230–232 |

**Verdict:** Matches.

---

## 02 — One-shot, threshold exceeded

| Diagram step | Code |
|--------------|------|
| run() called (interval 0) | `run()` invoked at 306; interval <= 0 → return 307–309 |
| Stats(ctx, pool) | 234 `postgres.Stats(ctx, pool)` |
| opt StaleCount if threshold-stale set | 249–266 |
| compare stats to thresholds, build events | 268–290 (total, active, idle, stale, then force test event if set) |
| loop for each event → Send to Slack/Loki | 293–300 |

**Verdict:** Matches.

---

## 03 — One-shot, no alert

| Diagram step | Code |
|--------------|------|
| run(), Stats, opt StaleCount | Same as 02 |
| all below thresholds → events = [] | No threshold condition true → events unchanged (empty) |
| no Send(), return | Loop over events is empty; then return when interval <= 0 |

**Verdict:** Matches.

---

## 04 — Dry-run

| Diagram step | Code |
|--------------|------|
| Stats, log total/active/idle | 246–251 `log.Printf("total=%d active=%d idle=%d"...)` (and max_connections when > 0) |
| build events if any threshold exceeded | 248–290 (same logic; dry-run only affects sending) |
| for each event: log [dry-run] would send, no Send() | 294–296 `if cfg.DryRun { log.Printf("[dry-run] would send: %s", ev.Message); continue }` |

**Verdict:** Matches. (Diagram could mention that max_connections is logged when available.)

---

## 05 — Force notification

| Diagram step | Code |
|--------------|------|
| Stats | 234 |
| append test event (threshold=test, run context) | 278–290 `if cfg.ForceNotification { events = append(events, notify.Event{ Threshold: "test", ... }) }` |
| loop Send to Slack/Loki | 293–300 |

**Verdict:** Matches.

---

## 06 — Daemon loop

| Diagram step | Code |
|--------------|------|
| startup (connect, defaults, senders) | All before `run` definition |
| run() once | 306 |
| ticker := NewTicker(interval) | 311 |
| loop: ticker → run() (Stats, StaleCount, build events, Send) or ctx.Done → return | 313–319 `select { case <-ctx.Done(): return; case <-ticker.C: run() }` |

**Verdict:** Matches.

---

## 07 — Connect failure notification

| Diagram step | Code |
|--------------|------|
| validations, build senders (before Pool) | 78–178 |
| Pool(ctx, dbURL) → error | 180–181 |
| build connect_failure event (message + run context) | 183–192 `notify.Event{ Threshold: "connect_failure", Cluster, Client, Namespace, Database }` |
| loop Send to Slack/Loki | 193–197 |
| log.Fatalf, exit 1 | 200 `log.Fatal("postgres connect failed...")` (message omits error detail intentionally) |

**Verdict:** Matches.

---

## When to re-audit

- After changing startup order, validations, kube flow, or connect-failure handling (01, 07).
- After changing `run()` (stats, thresholds, events, dry-run, force-notification) (02, 03, 04, 05, 06).
- Before a release if any of the above code paths were modified.
