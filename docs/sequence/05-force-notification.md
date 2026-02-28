# Sequence: Force notification — test delivery

With `-force-notification`: when connected, fetch stats, add a single "test" event, send to all configured notifiers (Slack and/or Loki), then exit. When the connection to Postgres fails, a connect-failure event is sent instead (see [01-startup-validation](./01-startup-validation.md)). Used to validate webhook/URL and message format in both cases.

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
    pgwd->>pgwd: append test event (threshold=test, message, event includes run context (time, client, database, cluster, namespace, connections))
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

**Slack:** Message is sent as an attachment with color `good` (green bar) for test events; the body includes Time, Client, Database, Cluster, Namespace, and Connections as a bullet list.
