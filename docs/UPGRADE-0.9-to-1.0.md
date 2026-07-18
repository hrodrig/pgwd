# Upgrading from 0.9.x to 1.0.0

This guide helps operators move from **pgwd 0.9.x** to **1.0.0**. It does not replace the full [CHANGELOG](../CHANGELOG.md); it lists **breaking** config/CLI changes and a short verify checklist.

**Baseline:** install **v1.0.0** (or later **1.0.x**) from [Releases](https://github.com/hrodrig/pgwd/releases). Behavior contract: [SPECIFICATIONS.md](../SPECIFICATIONS.md).

## Breaking changes (summary)

| Removed | Replacement |
|---------|-------------|
| Top-level YAML **`db:`** | **`databases:`** with one entry (even for a single target) |
| **`-db-threshold-total`** / **`-db-threshold-active`** | **`-db-threshold-levels`** (default `75,85,95`) |
| **`PGWD_DB_THRESHOLD_TOTAL`** / **`PGWD_DB_THRESHOLD_ACTIVE`** | **`PGWD_DB_THRESHOLD_LEVELS`** |
| YAML **`threshold.total`** / **`threshold.active`** | **`threshold.levels`** |
| **`-notify-on-connect-failure`**, **`PGWD_NOTIFY_ON_CONNECT_FAILURE`**, YAML **`notify_on_connect_failure`** | Always-on when any notifier is configured (no flag) |

**Already removed in 0.9.x** (not new in 1.0): `DISCOVER_MY_PASSWORD` / `pods/exec` password discovery. See [kubernetes-passwords.md](./kubernetes-passwords.md).

**Exit codes (aligned with SPEC):** single-target connect failure → **2**; one-shot stats/query failure → **3**; `-strict` notifier failure → **4**. See README [Behavior and exit](../README.md#behavior-and-exit).

## 1. Migrate `db:` → `databases:`

If your config still has top-level `db:`, **1.0.0 refuses to load** it. Wrap as one `databases:` entry:

**Before (0.9.x, deprecated):**

```yaml
client: "prod-db-primary"
interval: 60
db:
  url: postgres://user:pass@localhost:5432/mydb
  threshold:
    levels: "75,85,95"
```

**After (1.0.0):**

```yaml
client: "prod-db-primary"
interval: 60
databases:
  - url: postgres://user:pass@localhost:5432/mydb
    threshold:
      levels: "75,85,95"
```

Canonical example: [contrib/pgwd.conf.example](../contrib/pgwd.conf.example). Profiles: [contrib/profiles/](../contrib/profiles/).

## 2. Replace total/active thresholds with levels

Remove CLI flags `-db-threshold-total` / `-db-threshold-active`, env `PGWD_DB_THRESHOLD_*` total/active, and YAML `threshold.total` / `threshold.active`. Use levels:

```bash
pgwd -db-url "postgres://..." -client my-monitor \
  -notifications-slack-webhook "https://..." \
  -db-threshold-levels 75,85,95
```

Idle and stale thresholds (`-db-threshold-idle`, `-db-threshold-stale` + `-db-stale-age`) are unchanged.

## 3. Drop `notify-on-connect-failure`

Delete the flag, env var, and YAML key. When Slack/Loki/PagerDuty/Teams/generic (or kube-loki) is configured, connect failures are notified automatically. No replacement flag.

## 4. Verify

```bash
# Config loads (no db: / no removed keys)
pgwd -config /etc/pgwd/pgwd.conf -dry-run

# Optional: full local gate before production rollout
make release-check   # from a clone; needs Docker, kind for e2e kube, etc.
```

Confirm systemd/cron units do not pass removed flags. After upgrade, skim **[Unreleased]** / **[1.0.0]** in [CHANGELOG.md](../CHANGELOG.md).

## Related

- [Compare pgwd vs alternatives](./compare.md)
- [Operator use cases](./use-cases.md)
- [Upgrading 0.5 → 0.6](./UPGRADE-0.5-to-0.6.md) (older path)
- [plan-1.0.x.md](./plan-1.0.x.md)
