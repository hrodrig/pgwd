# Upgrading from 0.5.x to 0.6.x

This guide helps operators move from **pgwd 0.5.10** (or earlier 0.5.x) to **0.6.4** (or any **0.6.x** patch). It does not replace the full [CHANGELOG](../CHANGELOG.md); it groups what matters for upgrades.

## If you are already on 0.5.10

The large **config and CLI/env renames** shipped in **0.5.10**. Your `pgwd.conf`, systemd units, and scripts should already match the current layout (`db`, `notifications`, mandatory `client`, etc.).

To go to **0.6.x**:

1. Install the new package or binary (**v0.6.4** or latest **0.6.x** from [Releases](https://github.com/hrodrig/pgwd/releases)).
2. Skim **[0.6.0]** and **[0.6.4]** in [CHANGELOG](../CHANGELOG.md) for anything you use:
   - **Metrics CSV export** (optional): `-export-metrics-format csv` and `-export-metrics-destination` (or config/env equivalents).
   - **PostgreSQL / MySQL metrics store** (optional, **0.6.4+**): `metrics_store.driver` and `metrics_store.dsn`; default remains **SQLite** via `sqlite.path`.
   - **Dependency / security:** `pgx` was bumped in **0.6.0**; staying on an old binary is not recommended.
3. **Helm / OCI chart:** If you relied on the chart published from **this** repository, read [contrib/HELM.md](../contrib/HELM.md). Charts and OCI publishing moved to **[pgwd-selfhosted](https://github.com/hrodrig/pgwd-selfhosted)**. Raw Kubernetes notes remain under [contrib/k8s/README.md](../contrib/k8s/README.md).

No additional flag or env rename is required between **0.5.10** and **0.6.4** for the same behavior.

## If you are on 0.5.9 or earlier (including 0.5.8)

You must apply the **0.5.10** breaking changes first (they apply to every **0.6.x** build as well).

### 1. Config file (YAML)

- Use the structure documented in **[0.5.10] → Changed** in [CHANGELOG](../CHANGELOG.md): `db` (url, threshold, stale_age, …), `kube`, `notifications` (loki, slack), top-level `client`, `interval`, etc.
- Compare your file with [contrib/pgwd.conf.example](../contrib/pgwd.conf.example).
- **`client` is required** (config or `-client`). There is no fallback to hostname or kube resource name.
- With **`-kube-postgres`**, cluster context comes from kubeconfig; **`cluster` is not** a config field.

### 2. CLI flags and environment variables

When you use the CLI or env instead of (or on top of) a config file, rename everything in the table:

**→ [README — Breaking changes (upgrade from 0.5.x)](../README.md#breaking-changes-upgrade-from-05x)**

Examples: `-slack-webhook` → `-notifications-slack-webhook`, `PGWD_LOKI_URL` → `PGWD_NOTIFICATIONS_LOKI_URL`, `PGWD_THRESHOLD_LEVELS` → `PGWD_DB_THRESHOLD_LEVELS`, etc.

### 3. Systemd and packages

- **0.5.10** packages install units under `/lib/systemd/system/`; units do **not** use `EnvironmentFile` — use **`/etc/pgwd/pgwd.conf`** (or `-config`) for settings.
- **0.5.10** changed ordering to **`network.target`** (not `network-online.target`). See [CHANGELOG](../CHANGELOG.md) and [contrib/systemd/README.md](../contrib/systemd/README.md) if you had custom overrides.

### 4. Optional 0.6.x features (after the above works)

These are **additive**; enable only if you need them:

| Feature | Where to start |
|---------|----------------|
| Daemon, SQLite metrics, hysteresis, HTTP `/healthz` and Prometheus `/metrics` | [README](../README.md), [contrib/pgwd.conf.example](../contrib/pgwd.conf.example) (`sqlite`, `http`, `confirm_*`) |
| CSV dump of stored metrics | [CHANGELOG — 0.6.0](../CHANGELOG.md), README (CSV export bullet) |
| Store metrics in PostgreSQL or MySQL | [CHANGELOG — 0.6.4](../CHANGELOG.md), `metrics_store` in [contrib/pgwd.conf.example](../contrib/pgwd.conf.example) |
| Multi-database `databases:` | README **Multi-database limitations** (kube vs `databases:`, unique `client` per target when needed) |

## Helm and Kubernetes

- **Helm chart:** Not in this repo anymore. See [contrib/HELM.md](../contrib/HELM.md) and **pgwd-selfhosted**.
- **In-cluster URLs, probes:** [contrib/k8s/README.md](../contrib/k8s/README.md), README Kubernetes sections.

## Verify after upgrade

```bash
pgwd --version
# Expect: pgwd v0.6.x (branch …, commit …, built …)

pgwd -config /etc/pgwd/pgwd.conf -dry-run -interval 0
# Or your usual one-shot / daemon smoke test
```

If validation fails, messages usually point to missing **`client`**, **`db.url`** / **`databases`**, or notifier settings; see `man pgwd` (shipped in `.deb`/`.rpm`) or [contrib/man/man1/pgwd.1](../contrib/man/man1/pgwd.1).

## Reference links

| Topic | Document |
|-------|----------|
| Full release history | [CHANGELOG](../CHANGELOG.md) |
| Flag/env rename table | [README — Breaking changes](../README.md#breaking-changes-upgrade-from-05x) |
| Example config | [contrib/pgwd.conf.example](../contrib/pgwd.conf.example) |
| Helm move | [contrib/HELM.md](../contrib/HELM.md) |
| Agent / dev context | [AGENTS.md](../AGENTS.md) |
