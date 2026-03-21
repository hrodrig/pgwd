# DragonFly BSD rc.d script for pgwd

[DragonFly BSD](https://www.dragonflybsd.org) uses rc.d, not systemd. This script runs pgwd as a daemon with `daemon(8)` for logging and pidfile management. DragonFly's rc.d is derived from FreeBSD; syntax is the same.

**One config = one Postgres.** For multiple instances (different clusters, thresholds), cron is often simpler: one cron entry per config file. See main README "Running from cron" and "Example: multiple services".

## Install (from tarball)

The release tarball (`pgwd_v*_dragonfly_amd64.tar.gz`) includes the rc.d script at `share/dragonfly/rc.d/pgwd`.

```bash
# Download (curl or fetch)
curl -L -o /tmp/pgwd.tar.gz "https://github.com/hrodrig/pgwd/releases/download/v0.5.7/pgwd_v0.5.7_dragonfly_amd64.tar.gz"
cd /tmp && tar xzf pgwd.tar.gz

# Install binary and rc.d script
install -m755 pgwd /usr/local/bin/
install -m555 share/dragonfly/rc.d/pgwd /etc/rc.d/pgwd

# Config (required)
mkdir -p /etc/pgwd
cp etc/pgwd/pgwd.conf.example /etc/pgwd/pgwd.conf
vi /etc/pgwd/pgwd.conf   # client, db.url, notifications, etc.
```

**From repo:** `cp contrib/dragonflybsd/rc.d/pgwd /etc/rc.d/pgwd` (run as root).

## Config (required)

pgwd reads `/etc/pgwd/pgwd.conf` by default. Edit the config:

```bash
vi /etc/pgwd/pgwd.conf
```

**Required fields:** `client`, `db.url`, `interval` (use 60 for daemon mode).

## Daemon (rc.d)

### Setup

1. Install pgwd and create the config (see [Config](#config-required) above).
2. Enable pgwd in rc.conf and start it:

```bash
echo 'pgwd_enable="YES"' >> /etc/rc.conf
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

To enable or disable on boot, edit `/etc/rc.conf` or use `sysrc`:

```bash
sysrc pgwd_enable="YES"   # enable
sysrc pgwd_enable="NO"    # disable
```

### rc.conf variables

| Variable       | Default                          | Description                                      |
|----------------|-----------------------------------|--------------------------------------------------|
| `pgwd_enable`  | `NO`                              | Set to `YES` to enable pgwd on boot              |
| `pgwd_flags`   | `-config /etc/pgwd/pgwd.conf`     | CLI flags passed to pgwd                         |
| `pgwd_config`  | `/etc/pgwd/pgwd.conf`             | Config file path                                 |
| `pgwd_logfile` | `/var/log/pgwd.log`               | Log file for daemon output                       |
| `pgwd_env`     | (none)                            | Environment variables (e.g. `KUBECONFIG=...`)     |

### Logging

The daemon writes to `pgwd_logfile` (default `/var/log/pgwd.log`). View logs with `tail -f /var/log/pgwd.log`.

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

Set `interval: 0` in the config when using cron.

## Kubernetes (kube-postgres, kube-loki)

When using `-kube-postgres` or `-kube-loki`, pgwd needs `KUBECONFIG`. Set in rc.conf:

```
pgwd_env="KUBECONFIG=/root/.kube/config"
```

Install kubectl: `pkg install kubectl`. See `contrib/freebsd/README.md` for a full config example (structure is the same).
