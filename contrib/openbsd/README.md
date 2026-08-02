# OpenBSD support for pgwd

[OpenBSD](https://www.openbsd.org) uses **rc.d**, not systemd. **`daemon="/usr/local/bin/pgwd"`** with **`rc_bg=YES`** — **`rc.subr`** backgrounds the daemon (no custom `rc_start` with `&`).

| Piece | Role |
|--------|------|
| **`/usr/local/bin/pgwd`** | Application binary |
| **`/etc/rc.d/pgwd`** | rc.d script (`contrib/openbsd/pgwd` or port **`pkg/pgwd.rc`** + **`@rcscript`**) |
| **`/etc/pgwd/pgwd.conf`** | Config (YAML); copy from **`pgwd.conf.example`** |

**One config = one Postgres.** For multiple instances, cron is often simpler — see main README "Running from cron".

**Maintainers (port + release):**

| Doc | Content |
|-----|---------|
| **[../BSD-PORTS-STEP-BY-STEP.md](../BSD-PORTS-STEP-BY-STEP.md)** | Full BSD port workflow (start here) |
| **[PORT-RELEASE.md](PORT-RELEASE.md)** | Distfile, VM **`make package`**, **ports@** diff |
| **[port/README.md](port/README.md)** | Port directory / ports tree checkout |
| **[DEBUG-VM.md](DEBUG-VM.md)** | Manual rc.d checklist on a test VM |
| **[testing/platforms/README.md](../../testing/platforms/README.md)** | Ansible **`make test-platforms`** |

## Install (from tarball)

The release tarball (`pgwd_v*_openbsd_amd64.tar.gz`) includes the rc.d script at `share/openbsd/rc.d/pgwd`.

```bash
# Extract and install binary
tar xzf pgwd_v1.0.1_openbsd_amd64.tar.gz
doas install -m755 pgwd /usr/local/bin/

# Copy rc.d script (from tarball)
doas install -m555 share/openbsd/rc.d/pgwd /etc/rc.d/pgwd

# Config (required; tarball includes etc/pgwd/pgwd.conf.example)
doas mkdir -p /etc/pgwd
doas cp etc/pgwd/pgwd.conf.example /etc/pgwd/pgwd.conf
doas vi /etc/pgwd/pgwd.conf  # edit client, databases[].url, etc.

# Enable and start
doas rcctl enable pgwd
doas rcctl start pgwd
```

**From repo (no tarball):** `doas cp contrib/openbsd/pgwd /etc/rc.d/pgwd`

## Optional: override flags and env in rc.conf.local

```bash
# /etc/rc.conf.local
pgwd_flags="-config /etc/pgwd/pgwd.conf"

# When using -kube-postgres or -kube-loki, pgwd needs KUBECONFIG (kubectl uses it).
# Copy your kubeconfig to a path root can read, e.g. /root/.kube/config:
pgwd_env="KUBECONFIG=/root/.kube/config"
```

## Logging

pgwd writes to stdout. By default, rc.d does not capture it. To send output to syslog:

```bash
# /etc/rc.conf.local
pgwd_logger="daemon.info"
```

Then restart: `rcctl restart pgwd`. View logs with:

```bash
tail -f /var/log/daemon
```

## Kubernetes (kube-postgres, kube-loki)

**Use case:** pgwd runs on an external VPS (e.g. OpenBSD) with a kubeconfig that has access to the cluster. Postgres and Loki run inside the cluster. pgwd uses `kubectl port-forward` to reach both and sends alerts to Loki and Slack. In Grafana, filter by the `client` label to see logs from this instance.

Example config (anonymous):

```yaml
client: "pgwd-vps-01"        # Identifies this instance in Slack/Loki
interval: 60
dry_run: false

# Deprecated: DISCOVER_MY_PASSWORD removed in 0.9.x — use Secret-backed password in URL
databases:
  - url: "postgres://postgres:YOUR_PASSWORD@localhost:25432/mydb"
    threshold:
      levels: "75,85,95"

kube:
  context: "my-context"       # Use context name from kubeconfig, not cluster name
  local_port: 25432
  loki: "default/svc/loki"
  loki_local_port: 3100
  postgres: "default/svc/postgres"

notifications:
  loki:
    org_id: "my-tenant"       # Must match Grafana Loki data source
  slack:
    webhook: "https://hooks.slack.com/..."
```

**Required:** Set `pgwd_env="KUBECONFIG=/root/.kube/config"` in rc.conf.local and copy kubeconfig to `/root/.kube/config`.

**Grafana:** Loki logs include a `client` label. Filter by `{app="pgwd", client="pgwd-vps-01"}` to see logs from this instance.

**Troubleshooting:** If you see "context X does not exist", use the **context** name from `kubectl config get-contexts`, not the cluster name. The database URL host must be `localhost` and the port must match `kube.local_port`.

## Commands

| Command | Action |
|---------|--------|
| `rcctl start pgwd` | Start daemon |
| `rcctl stop pgwd` | Stop daemon |
| `rcctl check pgwd` | Check if running |
| `rcctl enable pgwd` | Enable on boot |
| `rcctl disable pgwd` | Disable on boot |
