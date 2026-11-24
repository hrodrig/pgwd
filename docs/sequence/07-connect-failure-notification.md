# Sequence: Connection failure — notify (and exit only in one-shot)

When Postgres connection fails and at least one notifier is configured, pgwd **always** sends a connect-failure event (or `too_many_clients` if the error is "too many clients already") to all notifiers. No extra flag is required.

- **One-shot** (`interval == 0`), single target: log and **exit 2**.
- **Daemon** (`interval > 0`): log, persist `connect_failure` when a metrics store is configured, and **continue** to the next tick (no process exit). Multi-DB always logs and skips the failed target.

```mermaid
sequenceDiagram
    participant User
    participant pgwd
    participant Postgres
    participant Store
    participant Slack
    participant Loki

    User->>pgwd: pgwd with notifiers configured
    Note over pgwd: validations, build senders before Pool
    pgwd->>Postgres: Pool and Ping with check timeout
    Postgres-->>pgwd: error (connection refused, timeout)
    pgwd->>pgwd: build connect_failure or too_many_clients event
    alt Slack configured
        pgwd->>Slack: Send connect_failure event
        Slack-->>pgwd: ok or error log
    end
    alt Loki configured
        pgwd->>Loki: Send connect_failure event
        Loki-->>pgwd: ok or error log
    end
    opt at least one Send ok
        pgwd->>pgwd: log Notification sent
    end
    opt metrics store configured
        pgwd->>Store: Insert state connect_failure
    end
    alt one-shot single target
        pgwd->>User: log postgres connect failed, exit 2
    else daemon or multi-DB
        pgwd->>User: log postgres connect failed, continue
    end
```

**Slack:** connect_failure is sent with attachment color `danger` (red bar). Threshold-exceeded events use `warning` (yellow bar).

**See also:** [01-startup-validation](./01-startup-validation.md) (startup flow including this failure path).
