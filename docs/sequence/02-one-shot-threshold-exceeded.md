# Sequence: One-shot — threshold exceeded, notify and exit

Single run (interval 0): fetch stats, one or more thresholds exceeded, build events, send to Slack/Loki, then exit.

```mermaid
sequenceDiagram
    participant pgwd
    participant Postgres
    participant Slack
    participant Loki

    Note over pgwd: run() called (interval 0)
    pgwd->>Postgres: Stats(ctx, pool)
    Postgres-->>pgwd: total, active, idle
    opt threshold-stale set
        pgwd->>Postgres: StaleCount(ctx, pool, staleAge)
        Postgres-->>pgwd: staleCount
    end
    pgwd->>pgwd: compare stats to thresholds
    Note over pgwd: e.g. total >= thresholdTotal → event
    pgwd->>pgwd: build events[] (total, active, idle, stale as needed; each event includes run context: time, client, database, cluster, namespace, connections)
    loop for each event
        alt Slack configured
            pgwd->>Slack: Send(ctx, event)
            Slack-->>pgwd: (ok or error log)
        end
        alt Loki configured
            pgwd->>Loki: Send(ctx, event)
            Loki-->>pgwd: (ok or error log)
        end
    end
    pgwd->>pgwd: return (interval <= 0 → exit)
```
