# NetBSD rc.d script for pgwd

[NetBSD](https://www.netbsd.org) uses rc.d, not systemd. This script runs pgwd as a daemon with background execution, pidfile, and log file.

**One config = one Postgres.** For multiple instances (different clusters, thresholds), cron is often simpler: one cron entry per config file. See main README "Running from cron" and "Example: multiple services".

## Prerequisites

- **DNS:** Ensure `/etc/resolv.conf` has nameservers (e.g. `nameserver 8.8.8.8`) so you can reach GitHub and package mirrors.
- **curl:** NetBSD base has no `fetch`. Install curl: `pkg_add curl` (set `PKG_PATH` to the pkgsrc CDN if needed).

## Install

### Option A: From tarball (when NetBSD asset exists in release)

Check [Releases](https://github.com/hrodrig/pgwd/releases) for `pgwd_v*_netbsd_amd64.tar.gz`. If present:

```bash
# Download (use curl; fetch is not in NetBSD base)
curl -L -o /tmp/pgwd.tar.gz "https://github.com/hrodrig/pgwd/releases/download/v0.5.0/pgwd_v0.5.0_netbsd_amd64.tar.gz"
cd /tmp && tar xzf pgwd.tar.gz

# Install binary
install -m755 pgwd /usr/local/bin/

# Copy rc.d script (from tarball)
install -m555 share/netbsd/rc.d/pgwd /etc/rc.d/pgwd

# Config (required; tarball includes etc/pgwd/pgwd.conf.example)
mkdir -p /etc/pgwd
cp etc/pgwd/pgwd.conf.example /etc/pgwd/pgwd.conf
```

### Option B: Build from source and copy (when NetBSD tarball not in release)

On your Mac (or any machine with Go):

```bash
cd /path/to/pgwd
GOOS=netbsd GOARCH=amd64 go build -o pgwd-netbsd ./cmd/pgwd
```

On the NetBSD server, create directories first:

```bash
mkdir -p /usr/local/bin /etc/pgwd
```

From your Mac, copy files:

```bash
scp -P 2294 pgwd-netbsd root@YOUR_NETBSD_IP:/usr/local/bin/pgwd
scp -P 2294 contrib/netbsd/rc.d/pgwd root@YOUR_NETBSD_IP:/etc/rc.d/pgwd
scp -P 2294 contrib/pgwd.conf.example root@YOUR_NETBSD_IP:/etc/pgwd/pgwd.conf
```

On NetBSD, set permissions:

```bash
chmod 755 /usr/local/bin/pgwd
chmod 555 /etc/rc.d/pgwd
```

**From repo (no tarball):** `cp contrib/netbsd/rc.d/pgwd /etc/rc.d/pgwd` (run as root).

## Config (required)

pgwd reads `/etc/pgwd/pgwd.conf` by default. Edit the config:

```bash
vi /etc/pgwd/pgwd.conf
```

**Required fields:**

| Field      | Description |
|------------|-------------|
| `client`   | Unique name for this monitor (e.g. `prod-db-primary`). Required. |
| `db.url`   | PostgreSQL connection URL, e.g. `postgres://user:pass@host:5432/dbname` |
| `interval` | Seconds between checks. Use `60` for daemon mode; `0` for one-shot (cron). |

**Example minimal config:**

```yaml
client: "netbsd-monitor-01"
interval: 60
dry_run: true   # Set false when notifiers (Slack/Loki) are configured

db:
  url: postgres://pgwd:secret@192.168.110.99:5432/pgwd
  threshold:
    levels: "75,85,95"
```

**Quick test** (one-shot, no daemon):

```bash
pgwd -config /etc/pgwd/pgwd.conf -dry-run -interval 0
```

## Daemon (rc.d)

### Setup

1. Install pgwd and create the config (see [Config](#config-required) above).
2. Enable pgwd in rc.conf and start it:

```bash
echo 'pgwd=YES' >> /etc/rc.conf
service pgwd start
```

3. Verify:

```bash
service pgwd status
tail -f /var/log/pgwd.log
```

### Commands

| Command                 | Action                |
|-------------------------|------------------------|
| `service pgwd start`    | Start the daemon       |
| `service pgwd stop`     | Stop the daemon        |
| `service pgwd restart`  | Restart the daemon     |
| `service pgwd status`   | Check if running       |

To enable or disable on boot, edit `/etc/rc.conf` or `/etc/rc.conf.d/pgwd`:

| Setting      | Effect                    |
|--------------|---------------------------|
| `pgwd=YES`   | Start pgwd on boot        |
| `pgwd=NO`    | Do not start pgwd on boot |

### rc.conf variables

Add these to `/etc/rc.conf` or `/etc/rc.conf.d/pgwd` to customize the daemon:

| Variable       | Default                          | Description                                      |
|----------------|-----------------------------------|--------------------------------------------------|
| `pgwd`         | `NO`                              | Set to `YES` to enable pgwd on boot              |
| `pgwd_flags`   | `-config /etc/pgwd/pgwd.conf`     | CLI flags passed to pgwd (e.g. config path)      |
| `pgwd_config`  | `/etc/pgwd/pgwd.conf`             | Config file path (used for `required_files`)     |
| `pgwd_logfile` | `/var/log/pgwd.log`               | Log file for daemon output (stdout/stderr)       |
| `pgwd_env`     | (none)                            | Environment variables (e.g. `KUBECONFIG=...`)     |

**Example** — default config path:

```
pgwd=YES
pgwd_flags="-config /etc/pgwd/pgwd.conf"
```

**Example** — custom config and kube-postgres:

```
pgwd=YES
pgwd_flags="-config /etc/pgwd/prod.conf"
pgwd_env="KUBECONFIG=/root/.kube/config"
```

When using `-kube-postgres` or `-kube-loki`, pgwd needs `KUBECONFIG`. Copy your kubeconfig to a path root can read (e.g. `/root/.kube/config`) and set `pgwd_env`.

### Logging

The daemon writes to `pgwd_logfile` (default `/var/log/pgwd.log`). View logs with:

```bash
tail -f /var/log/pgwd.log
```

**Log rotation:** Add to `/etc/newsyslog.conf` (or equivalent on NetBSD):

```
/var/log/pgwd.log   644  5  100  *  B
```

## Cron (one-shot)

For periodic checks instead of a daemon:

```bash
# crontab -e
PATH=/usr/local/bin:/usr/bin:/bin
*/5 * * * * pgwd -config /etc/pgwd/pgwd.conf >> /var/log/pgwd.log 2>&1
```

Set `interval: 0` in the config when using cron (or omit; pgwd runs once per cron tick).

## Kubernetes (kube-postgres, kube-loki)

**Use case:** pgwd runs on an external NetBSD host (e.g. VPS) with a kubeconfig that has access to the cluster. Postgres and Loki run inside the cluster. pgwd uses `kubectl port-forward` to reach both and sends alerts to Loki and Slack.

Install kubectl: `pkg_add kubectl` (or from pkgsrc).

Example config:

```yaml
client: "pgwd-netbsd-01"
interval: 60
dry_run: false

db:
  url: "postgres://postgres:DISCOVER_MY_PASSWORD@localhost:25432/mydb"
  threshold:
    levels: "75,85,95"

kube:
  context: "my-context"
  local_port: 25432
  loki: "default/svc/loki"
  loki_local_port: 3100
  postgres: "default/svc/postgres"

notifications:
  loki:
    org_id: "my-tenant"
  slack:
    webhook: "https://hooks.slack.com/..."
```

**Required:** Set `KUBECONFIG` in the environment. For the rc.d daemon: `pgwd_env="KUBECONFIG=/root/.kube/config"` in rc.conf. For cron: `KUBECONFIG=/root/.kube/config` in the cron line or a wrapper script.

**Grafana:** Loki logs include a `client` label. Filter by `{app="pgwd", client="pgwd-netbsd-01"}`.

## Troubleshooting

| Issue | Solution |
|-------|----------|
| `pkg_add: no pkg found` | Set `PKG_PATH` to pkgsrc CDN, e.g. `export PKG_PATH="https://cdn.NetBSD.org/pub/pkgsrc/packages/NetBSD/amd64/10.0/All/"` |
| `Transient resolver failure` | Configure DNS in `/etc/resolv.conf`: `nameserver 8.8.8.8` |
| `fetch: not found` | Use `curl` or `ftp`; install curl with `pkg_add curl` |
| `pgwd is not running` after start | Ensure `interval` in config is > 0 (e.g. 60) for daemon mode. If 0, pgwd runs once and exits. |
| `no pg_hba.conf entry` | Add the NetBSD host IP to PostgreSQL `pg_hba.conf` on the DB server |
| `su -c '...'` fails | NetBSD `su` does not support `-c` like Linux. Run commands as root directly (prompt `#`). |
