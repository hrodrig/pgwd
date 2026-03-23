# Sequence: Daemon mode — ticker loop

With `-interval N` (N > 0): run once immediately, then every N seconds until SIGINT/SIGTERM. Targets come from config `databases:` or single `-db-url` (env). Each tick: loop over targets, connect → check → close.

```mermaid
sequenceDiagram
    participant User
    participant pgwd
    participant Postgres
    participant Notifiers

    User->>pgwd: pgwd -interval 60 (config or -db-url, notifiers)
    Note over pgwd: startup: targets from databases or DBURL
    loop for each target (first run)
        pgwd->>Postgres: connect → Stats / StaleCount
        Postgres-->>pgwd: stats
        pgwd->>pgwd: build events, Send if any (and not dry-run)
        pgwd->>pgwd: close pool
    end
    pgwd->>pgwd: ticker := NewTicker(60s)
    loop every 60s
        alt ticker fires
            loop for each target
                pgwd->>Postgres: connect → Stats / StaleCount
                Postgres-->>pgwd: stats
                pgwd->>Notifiers: Send(events) if any
                pgwd->>pgwd: close pool
            end
        else ctx.Done() (SIGINT/SIGTERM)
            pgwd->>pgwd: return, exit 0
        end
    end
```
