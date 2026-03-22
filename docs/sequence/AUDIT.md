# Sequence diagrams — audit vs code

Last audit: checked each diagram against `cmd/pgwd/main.go` and `internal/notify`, `internal/postgres`.

## 01 — Startup and config validation

| Diagram step | Code |
|--------------|------|
| User runs pgwd (CLI args) | `main()` |
| config file exists and loads | `config.FromFile(path)` 567; if loaded, env vars ignored |
| no config file: ApplyDefaults + ApplyEnv | 571–573 `ApplyDefaults()`, `ApplyEnv()` |
| flag.Parse (CLI overrides) | 577 `parseFlags()` |
| validate client required | `validateClient()` 106–109 |
| validate DB URL present | `validateDBURL()` 112–116 |
| validate stale-age if threshold-stale | `validateStale()` 118–122 |
| validate at least one notifier (or dry-run) | `validateNotifiers()` 124–133 |
| validate force-notification / notify-on-connect-failure require notifier | `validateNotifiers()` |
| signal.NotifyContext(SIGINT, SIGTERM) | 590 |
| opt -kube-postgres: resolve pod, get password, port-forward, replace DB URL | 594 `setupKube()` |
| opt -kube-loki: port-forward to Loki, set Loki URL | 596 `setupKubeLoki()` |
| compute run context (cluster, client, namespace, database) | 601 `runContextStrings()` |
| build senders (Slack, Loki) | 602 `buildSenders()` |
| Pool(ctx, dbURL) | 604 |
| connect error → opt Send(connect_failure) then log.Fatal | 605–607 `notifyConnectFailure()` |
| opt at least one Send ok → log Notification sent | 279–281 in `notifyConnectFailure()` |
| MaxConnections, apply default thresholds when total/active 0 | 611–615 `applyThresholdDefaults()` |
| validate at least one threshold or dry-run or force-notification | `validateThresholdConfig()` |

**Verdict:** Matches.

---

## 02 — One-shot, threshold exceeded

| Diagram step | Code |
|--------------|------|
| run() called (interval 0) | `makeRunFunc()`; interval <= 0 → return 618–620 |
| Stats(ctx, pool) | 512 `postgres.Stats(ctx, pool)` |
| opt StaleCount if threshold-stale set | 443–446 `collectStaleEvent()` |
| compare stats to thresholds, build events | 439–468 `collectEvents()` |
| loop for each event → Send to Slack/Loki | 489–507 `sendEvents()` |
| opt at least one Send ok → log Notification sent | 504–505 |

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
| Stats, log total/active/idle | 521–522 `log.Printf("total=%d active=%d idle=%d"...)` (and max_connections when > 0) |
| build events if any threshold exceeded | 439–468 `collectEvents()` (same logic; dry-run only affects sending) |
| for each event: log [dry-run] would send, no Send() | 491–494 `if cfg.DryRun { log.Printf("[dry-run] would send: %s", ev.Message); continue }` |

**Verdict:** Matches. (Diagram could mention that max_connections is logged when available.)

---

## 05 — Force notification

| Diagram step | Code |
|--------------|------|
| Stats | 512 `postgres.Stats(ctx, pool)` |
| append test event (threshold=test, run context) | 461–467 `collectEvents()` when `cfg.ForceNotification` |
| loop Send to Slack/Loki | 489–507 `sendEvents()` |
| opt at least one Send ok → log Notification sent | 504–505 |

**Verdict:** Matches.

---

## 06 — Daemon loop

| Diagram step | Code |
|--------------|------|
| startup (connect, defaults, senders) | All before `makeRunFunc` |
| run() once | 572 |
| ticker := NewTicker(interval) | 576 |
| loop: ticker → run() (Stats, StaleCount, build events, Send) or ctx.Done → return | 578–585 `select { case <-ctx.Done(): return; case <-ticker.C: run() }` |

**Verdict:** Matches.

---

## 07 — Connect failure notification

| Diagram step | Code |
|--------------|------|
| validations, build senders (before Pool) | 547–560 |
| Pool(ctx, dbURL) → error | 562–563 |
| build connect_failure event (message + run context) | 255–269 `notify.Event{ Threshold: "connect_failure", Cluster, Client, Namespace, Database }` |
| loop Send to Slack/Loki | 271–278 |
| opt at least one Send ok → log Notification sent | 279–281 |
| log.Fatal, exit 1 | 565 `log.Fatal("postgres connect failed...")` (message omits error detail intentionally) |

**Verdict:** Matches.

---

## When to re-audit

- After changing config loading (FromFile, ApplyEnv, config file as single source), startup order, validations, kube flow (kube-postgres, kube-loki), or connect-failure handling (01, 07).
- After changing `run()` (stats, thresholds, events, dry-run, force-notification) (02, 03, 04, 05, 06).
- After changing `sendEvents()` or `notifyConnectFailure()` (Notification sent log).
- Before a release if any of the above code paths were modified.
