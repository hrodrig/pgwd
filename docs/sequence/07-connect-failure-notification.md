# Sequence: Connection failure — send alert and exit

When Postgres connection fails and `-notify-on-connect-failure` or `-force-notification` is set (with at least one notifier), pgwd sends a connect-failure event to Slack and/or Loki before exiting. Use this to get an infrastructure alert when the database is unreachable.

```mermaid
sequenceDiagram
    participant User
    participant pgwd
    participant Postgres
    participant Slack as Slack/Loki

    User->>pgwd: pgwd -notify-on-connect-failure (or -force-notification) -db-url ... -slack-webhook ...
    Note over pgwd: validations, build senders (before Pool)
    pgwd->>Postgres: Pool(ctx, dbURL)
    Postgres-->>pgwd: error (e.g. connection refused, timeout)
    pgwd->>pgwd: build connect_failure event (message + run context: cluster, client, namespace, database when available)
    loop for each sender (Slack, Loki)
        pgwd->>Slack: Send(ctx, connect_failure event)
        Slack-->>pgwd: (ok or error log)
    end
    pgwd->>User: log.Fatalf("postgres connect: %v"), exit 1
```

**Slack:** connect_failure is sent with attachment color `danger` (red bar). Threshold-exceeded events use `warning` (yellow bar).

**See also:** [01-startup-validation](./01-startup-validation.md) (startup flow including this failure path).
