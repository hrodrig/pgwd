# FreeBSD port for pgwd

This directory contains the [FreeBSD](https://www.freebsd.org) port files for pgwd. Use them to build and install pgwd from the official ports tree (when accepted) or from a local port.

**One config = one Postgres.** For multiple instances (different clusters, thresholds), cron is often simpler: one cron entry per config file. See main README "Running from cron" and "Example: multiple services".

## Install from port

When the port is in the official FreeBSD ports tree:

```bash
cd /usr/ports/sysutils/pgwd
make install
```

When using a local port (before it is accepted):

```bash
# Copy Makefile, pkg-plist, pkg-descr, rc.d/pgwd to ports/sysutils/pgwd/
cd ~/ports/sysutils/pgwd
make install
```

**Reinstalling after updating port files:** Run `make deinstall`, then `make clean`, then `make install`. Without `make clean`, the port may reuse a cached stage and not pick up changes (e.g. to `rc.d/pgwd`).

The port installs:

- Binary: `/usr/local/bin/pgwd`
- Man page: `/usr/local/share/man/man1/pgwd.1.gz`
- License: `/usr/local/share/doc/pgwd/LICENSE`
- Config example: `/usr/local/etc/pgwd/pgwd.conf.example`
- rc.d script: `/usr/local/etc/rc.d/pgwd`

## Config (required)

pgwd reads `/etc/pgwd/pgwd.conf` by default. Copy the example and edit:

```bash
mkdir -p /etc/pgwd
cp /usr/local/etc/pgwd/pgwd.conf.example /etc/pgwd/pgwd.conf
vi /etc/pgwd/pgwd.conf   # client, db.url, notifications, etc.
```

## Daemon (rc.d)

### Setup

1. Install the port and create the config (see [Config](#config-required) above).
2. Enable pgwd in rc.conf and start it:

```bash
echo 'pgwd_enable="YES"' >> /etc/rc.conf
service pgwd start
```

### Commands

| Command                | Action                          |
|------------------------|---------------------------------|
| `service pgwd start`   | Start the daemon                |
| `service pgwd stop`    | Stop the daemon                 |
| `service pgwd restart` | Restart the daemon              |
| `service pgwd status`  | Check if running                |

To enable or disable on boot, edit `/etc/rc.conf` or `/etc/rc.conf.local`:

| Setting                | Effect                          |
|------------------------|---------------------------------|
| `pgwd_enable="YES"`    | Start pgwd on boot              |
| `pgwd_enable="NO"`     | Do not start pgwd on boot       |

### rc.conf variables

Add these to `/etc/rc.conf` or `/etc/rc.conf.local` to customize the daemon:

| Variable       | Default                          | Description                                      |
|----------------|-----------------------------------|--------------------------------------------------|
| `pgwd_enable`  | `NO`                              | Set to `YES` to enable pgwd on boot              |
| `pgwd_flags`   | `-config /etc/pgwd/pgwd.conf`     | CLI flags passed to pgwd (e.g. config path)      |
| `pgwd_config`  | `/etc/pgwd/pgwd.conf`             | Config file path (used for `required_files`)     |
| `pgwd_env`     | (none)                            | Environment variables (e.g. `KUBECONFIG=...`)     |
| `pgwd_logfile` | `/var/log/pgwd.log`               | Log file for daemon output (stdout/stderr)       |

**Example** — default config path:

```bash
pgwd_enable="YES"
pgwd_flags="-config /etc/pgwd/pgwd.conf"
```

**Example** — custom config and kube-postgres:

```bash
pgwd_enable="YES"
pgwd_flags="-config /etc/pgwd/prod.conf"
pgwd_env="KUBECONFIG=/root/.kube/config"
```

When using `-kube-postgres` or `-kube-loki`, pgwd needs `KUBECONFIG`. Copy your kubeconfig to a path root can read (e.g. `/root/.kube/config`) and set `pgwd_env`.

### Logging

The daemon writes to `pgwd_logfile` (default `/var/log/pgwd.log`). View logs with:

```bash
tail -f /var/log/pgwd.log
```

**Log rotation:** Add to `/etc/newsyslog.conf`:

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

**Log rotation:** Add to `/etc/newsyslog.conf`:

```txt
/var/log/pgwd.log   644  5  100  *  B
```

## Kubernetes (kube-postgres, kube-loki)

**Use case:** pgwd runs on an external FreeBSD host (e.g. VPS) with a kubeconfig that has access to the cluster. Postgres and Loki run inside the cluster. pgwd uses `kubectl port-forward` to reach both and sends alerts to Loki and Slack.

Install kubectl: `pkg install kubectl`.

Example config:

```yaml
client: "pgwd-freebsd-01"
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

**Grafana:** Loki logs include a `client` label. Filter by `{app="pgwd", client="pgwd-freebsd-01"}`.

## Port files

| File           | Purpose                                      |
|----------------|----------------------------------------------|
| `Makefile`     | Port definition (fetch, extract, install)     |
| `pkg-plist`    | List of files installed by the package       |
| `pkg-descr`    | Package description (one line)               |
| `rc.d/pgwd`    | rc.d script for daemon management            |

## Submitting to official ports

When the port is ready, submit via [Bugzilla](https://bugs.freebsd.org) (preferred) or see [Porter's Handbook](https://docs.freebsd.org/en/books/porters-handbook/). The maintainer email in the Makefile must be valid and responsive.
