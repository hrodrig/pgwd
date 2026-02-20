# Sequence: Force notification — test delivery

With `-force-notification`: fetch stats, add a single "test" event, send to all configured notifiers (Slack and/or Loki), then exit. Used to validate webhook/URL and message format.

```mermaid
sequenceDiagram
    participant User
    participant pgwd
    participant Postgres
    participant Slack
    participant Loki

    User->>pgwd: pgwd -force-notification -db-url ... -slack-webhook ... (-loki-url ...)
    pgwd->>Postgres: Stats(ctx, pool)
    Postgres-->>pgwd: total, active, idle
    pgwd->>pgwd: append test event (threshold=test, message with current stats)
    loop for test event
        alt Slack configured
            pgwd->>Slack: Send(ctx, test event)
            Slack-->>pgwd: (ok or error log)
        end
        alt Loki configured
            pgwd->>Loki: Send(ctx, test event)
            Loki-->>pgwd: (ok or error log)
        end
    end
    pgwd->>pgwd: return (interval <= 0 → exit)
```
