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
    alt config file exists and loads
        pgwd->>pgwd: FromFile(path) → loaded (env vars ignored)
    else no config file
        pgwd->>Env: ApplyDefaults + ApplyEnv (PGWD_* vars)
    end
    pgwd->>pgwd: flag.Parse() (CLI overrides)
    opt -db-url + -interval 0 and config has databases:
        pgwd->>pgwd: override: use single target from -db-url (ignore databases)
    end
    pgwd->>pgwd: targets := Targets() (single from DBURL or list from Databases)
    pgwd->>pgwd: validate: client required
    alt missing client
        pgwd->>User: log.Fatal, exit 1
    end
    pgwd->>pgwd: validate: DB URL present (when single target; skip when databases)
    alt missing DB URL
        pgwd->>User: log.Fatal, exit 1
    end
    pgwd->>pgwd: validate: stale-age if threshold-stale
    pgwd->>pgwd: validate: at least one notifier (or dry-run)
    pgwd->>pgwd: validate: force-notification / notify-on-connect-failure require notifier
    pgwd->>pgwd: stderr warnings: deprecated flags, http:// notifiers
    opt interval > 0 (daemon)
        pgwd->>pgwd: optional collector POST / GitHub update check (see SPEC §3)
    end
    pgwd->>pgwd: signal.NotifyContext(SIGINT, SIGTERM)
    opt -kube-postgres set
        pgwd->>Kube: optional password_from_secret (Secret GET) or operator DSN
        pgwd->>Kube: port-forward (background)
        pgwd->>pgwd: replace DB URL (localhost, port)
        Note over pgwd: DISCOVER_MY_PASSWORD → config error (removed 0.9.x)
    end
    opt -kube-loki set
        pgwd->>Kube: port-forward to Loki (background)
        pgwd->>pgwd: set Loki URL (localhost, kube-loki-local-port)
    end
    pgwd->>pgwd: compute run context (cluster, client, namespace from kube/config, database from DB URL path)
    pgwd->>pgwd: build senders (Slack, Loki from config)
    pgwd->>Postgres: Pool(ctx, dbURL)
    alt connect error
        opt senders configured
            pgwd->>Slack: Send(connect_failure event)
            Slack-->>pgwd: (ok or error log)
            opt at least one ok
                pgwd->>pgwd: log Notification sent
            end
        end
        pgwd->>User: log connect failed, exit 2
    end
    Postgres-->>pgwd: pool
    pgwd->>Postgres: MaxConnections(ctx, pool)
    Postgres-->>pgwd: max_connections
    pgwd->>pgwd: if total/active threshold 0: set to defaultThresholdPercent of max_connections
    pgwd->>pgwd: validate: at least one threshold or dry-run or force-notification
    Note over pgwd: ready, run() can be called
```
