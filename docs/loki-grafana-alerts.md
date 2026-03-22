# Loki payload structure for Grafana alerts

This document describes the structure of pgwd notifications sent to Loki, for building Grafana alert rules that react to pgwd events (attention, alert, danger).

## Loki push payload structure

pgwd sends logs to Loki's `/loki/api/v1/push` API. Each notification is one log entry with:

### Stream labels (for LogQL filtering)

| Label       | Always present | Description                                      |
|-------------|----------------|--------------------------------------------------|
| `app`       | yes            | Always `pgwd`                                    |
| `threshold` | yes            | `test`, `total`, `active`, `idle`, `stale`, `connect_failure`, `too_many_clients` |
| `level`     | yes            | Severity: `attention`, `alert`, or `danger`        |
| `namespace` | when K8s       | Kubernetes namespace (e.g. `mynamespace`)       |
| `database`  | when set       | Database name from connection URL                |
| `cluster`   | when set       | Cluster name (from kubeconfig when using -kube-postgres)     |

Additional labels from `-notifications-loki-labels` (e.g. `env=prod`) are also included.

### Log line format

```bash
pgwd [cluster=<Cluster>] [database=<Database>]: <Message> | total=<Total> active=<Active> idle=<Idle> max_connections=<Max> [suffix]
```

Example:

```bash
pgwd [cluster=prod] [database=myapp]: Test notification — delivery check (force-notification). | total=33 active=1 idle=32 max_connections=2048 (delivery check)
```

**Suffixes:**

- `(delivery check)` — test notification
- `(connection failed)` — connect_failure
- `(too many clients — DB saturated)` — too_many_clients
- `(limit <threshold>=<value>)` — threshold exceeded

## Level values

| Level       | When used                                           |
|-------------|-----------------------------------------------------|
| `attention` | 3-tier 75%, 80%, etc.; `test`; `idle`, `stale`      |
| `alert`     | 3-tier 85% (configurable)                           |
| `danger`    | 3-tier 95%; `connect_failure`; `too_many_clients`    |

## Example LogQL queries for Grafana alerts

### All pgwd notifications

```logql
{app="pgwd"}
```

### Only danger (critical)

```logql
{app="pgwd", level="danger"}
```

### Alert or danger (skip attention)

```logql
{app="pgwd", level=~"alert|danger"}
```

### Specific database

```logql
{app="pgwd", database="myapp"}
```

### Specific cluster

```logql
{app="pgwd", cluster="prod"}
```

### Danger + specific database

```logql
{app="pgwd", level="danger", database="myapp"}
```

### Connect failures only

```logql
{app="pgwd", threshold="connect_failure"}
```

### Too many clients (DB saturated)

```logql
{app="pgwd", threshold="too_many_clients"}
```

## Grafana alert rule setup

1. **Alert type:** Use a **Log** alert (not metric).
2. **Query:** Use one of the LogQL queries above (e.g. `{app="pgwd", level="danger"}`).
3. **Condition:** "No data" or "Alert when result matches" — e.g. when any log line is returned.
4. **Evaluation:** Set the interval (e.g. every 1m) and for how long the condition must be true.

For "fire when any pgwd danger log appears":

- Query: `{app="pgwd", level="danger"}`
- Condition: `count_over_time(...[5m]) > 0` or use Grafana's "Alert when result matches" with a count.

## Raw JSON payload example

For reference, the payload sent to Loki looks like:

```json
{
  "streams": [
    {
      "stream": {
        "app": "pgwd",
        "threshold": "total",
        "level": "danger",
        "namespace": "mynamespace",
        "database": "myapp",
        "cluster": "prod"
      },
      "values": [
        ["1731400000000000000", "pgwd [cluster=prod] [database=myapp]: Total connections 95 >= 95 | total=95 active=10 idle=85 max_connections=100 (limit total=95)"]
      ]
    }
  ]
}
```

Labels are indexed by Loki and appear in Grafana's Fields panel when you expand a log entry.

## Related

[Testing alert levels without changing production](./testing-alert-levels.md) — Procedure to trigger attention, alert, and danger notifications using `-test-max-connections` against production Postgres so you can validate alert messages and query patterns before deploying pgwd.
