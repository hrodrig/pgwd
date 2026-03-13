# Testing alert levels without changing production

Use this procedure to trigger **attention**, **alert**, and **danger** notifications against a production Postgres instance **without** modifying its `max_connections` or waiting for real load to reach thresholds.

## Why

- Production has e.g. 2048 `max_connections` and 1638 in use (~80%).
- You want to validate that Grafana/Slack alerts fire correctly and show the right messages.
- You do **not** want to change Postgres config or rely on real traffic to hit thresholds.

## How it works

Use `-test-max-connections N` so pgwd treats `N` as the effective `max_connections` for threshold calculation. Stats (total, active, idle) stay real from Postgres; only the denominator changes. Notifications show `(test override)` so you know it is a test.

With default 3-tier levels (75%, 85%, 95%):

- **attention** fires when `total >= 0.75 × test_max`
- **alert** fires when `total >= 0.85 × test_max`
- **danger** fires when `total >= 0.95 × test_max`

## Procedure

### 1. Get current connection count

```bash
pgwd -db-url "postgres://..." -dry-run
```

Or use `-kube-postgres` if applicable. Note the `total` value (e.g. 1638).

### 2. Compute test-max-connections for each level

For a given `total`, set `test_max = total / (level_percent / 100)`:

| Level     | Percent | Formula           | Example (total=1638) |
|-----------|---------|-------------------|----------------------|
| attention | 75%     | total / 0.75      | 2184                 |
| alert     | 85%     | total / 0.85      | 1927                 |
| danger    | 95%     | total / 0.95      | 1724                 |

### 3. Run pgwd three times (one per level)

Use your real connection URL and notifiers (Loki, Slack). Replace `TOTAL` with your actual total from step 1.

**Attention (75%):**

```bash
pgwd -db-url "postgres://..." -loki-url "http://..." -loki-org-id "mytenant" \
  -test-max-connections $((TOTAL * 100 / 75))
```

**Alert (85%):**

```bash
pgwd -db-url "postgres://..." -loki-url "http://..." -loki-org-id "mytenant" \
  -test-max-connections $((TOTAL * 100 / 85))
```

**Danger (95%):**

```bash
pgwd -db-url "postgres://..." -loki-url "http://..." -loki-org-id "mytenant" \
  -test-max-connections $((TOTAL * 100 / 95))
```

### 4. Example with concrete values

If `total=1638`:

```bash
# Attention
pgwd -kube-postgres myns/svc/postgres -kube-loki myns/svc/loki \
  -db-url "postgres://..." -loki-org-id "mytenant" \
  -test-max-connections 2184

# Alert
pgwd -kube-postgres myns/svc/postgres -kube-loki myns/svc/loki \
  -db-url "postgres://..." -loki-org-id "mytenant" \
  -test-max-connections 1927

# Danger
pgwd -kube-postgres myns/svc/postgres -kube-loki myns/svc/loki \
  -db-url "postgres://..." -loki-org-id "mytenant" \
  -test-max-connections 1724
```

Each run sends one notification to Loki (and Slack if configured) with the corresponding `level` label.

### 5. Validate in Grafana

1. Open Loki Explore and query `{app="pgwd"}`.
2. Confirm three log entries with `level=attention`, `level=alert`, `level=danger`.
3. Use these as reference when defining Grafana alert rules (see [loki-grafana-alerts.md](./loki-grafana-alerts.md)).

## Custom threshold levels

If you use `-threshold-levels 70,85,90` instead of the default 75,85,95, adjust the formula:

- attention at 70%: `test_max = total / 0.70`
- alert at 85%: `test_max = total / 0.85`
- danger at 90%: `test_max = total / 0.90`

## Notes

- Notifications include `(test override)` so they are distinguishable from real alerts.
- Postgres `max_connections` is unchanged; only pgwd’s threshold calculation uses the override.
- Run during a maintenance window or low-traffic period so `total` is stable across the three runs.
