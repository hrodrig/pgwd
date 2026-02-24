# Sequence: Startup and config validation

From process start until the first `run()` is invoked: load config, validate, optional Kubernetes port-forward, build senders, connect to Postgres, apply default thresholds.

```mermaid
sequenceDiagram
    participant User
    participant pgwd
    participant Env
    participant Kube
    participant Postgres
    participant Slack as Slack/Loki

    User->>pgwd: run pgwd (CLI args)
    pgwd->>Env: read PGWD_* vars
    pgwd->>pgwd: config.FromEnv() + flag.Parse() (CLI overrides)
    pgwd->>pgwd: validate: DB URL present
    alt missing DB URL
        pgwd->>User: log.Fatal, exit 1
    end
    pgwd->>pgwd: validate: stale-age if threshold-stale
    pgwd->>pgwd: validate: at least one notifier (or dry-run)
    pgwd->>pgwd: validate: force-notification / notify-on-connect-failure require notifier
    pgwd->>pgwd: signal.NotifyContext(SIGINT, SIGTERM)
    opt -kube-postgres set
        pgwd->>Kube: resolve pod, get password (if DISCOVER_MY_PASSWORD)
        pgwd->>Kube: port-forward (background)
        pgwd->>pgwd: replace DB URL (localhost, port)
    end
    pgwd->>pgwd: compute run context (cluster, client, namespace from kube/config; database from DB URL path)
    pgwd->>pgwd: build senders (Slack, Loki from config)
    pgwd->>Postgres: Pool(ctx, dbURL)
    alt connect error
        opt notify-on-connect-failure or force-notification
            pgwd->>Slack: Send(connect_failure event)
            Slack-->>pgwd: (ok or error log)
        end
        pgwd->>User: log.Fatalf, exit 1
    end
    Postgres-->>pgwd: pool
    pgwd->>Postgres: MaxConnections(ctx, pool)
    Postgres-->>pgwd: max_connections
    pgwd->>pgwd: if total/active threshold 0: set to defaultThresholdPercent of max_connections
    pgwd->>pgwd: validate: at least one threshold or dry-run or force-notification
    Note over pgwd: ready, run() can be called
```
