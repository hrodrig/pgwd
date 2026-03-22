# Solaris SMF service for pgwd

[illumos](https://illumos.org) (OmniOS, OpenIndiana, SmartOS) and [Oracle Solaris](https://www.oracle.com/solaris) use **SMF (Service Management Facility)**, not systemd or rc.d. This manifest and method script run pgwd as a managed service.

## Supported versions

**illumos or Solaris 11.4+ only.** Pre-built pgwd binaries are supported on:

- **illumos** (OpenIndiana, [OmniOS](https://omnios.org/download.html), SmartOS, etc.) — actively developed
- **Oracle Solaris 11.4+** (x86 or SPARC) — sustaining support

**Solaris 11.3 and earlier are not supported.** Reasons:

1. **`pipe2()` syscall:** Go's runtime expects `pipe2(2)`, which was added in Solaris 11.4. Binaries compiled with standard Go crash (SIGSEGV) on earlier versions.
2. **Known Go issues:** [golang/go#61950](https://go.dev/issue/61950), [golang/go#56499](https://github.com/golang/go/issues/56499) document segfaults and runtime failures on Solaris 11.3.

On Solaris 11.3 you would need to build Go from source with compatibility patches; pre-built pgwd releases will not run.

**One config = one Postgres.** For multiple instances (different clusters, thresholds), cron is often simpler: one cron entry per config file. See main README "Running from cron" and "Example: multiple services".

## Prerequisites

- **curl or wget:** For downloading releases. Solaris 11.4+ includes curl.
- **Go** (optional): To build from source if no pre-built binary is available.

## Install

### Option A: From tarball (when Solaris asset exists in release)

Check [Releases](https://github.com/hrodrig/pgwd/releases) for `pgwd_v*_solaris_amd64.tar.gz`. If present:

```bash
# Download (Solaris 11.4+ has curl)
curl -L -o /tmp/pgwd.tar.gz "https://github.com/hrodrig/pgwd/releases/download/v0.5.8/pgwd_v0.5.8_solaris_amd64.tar.gz"
cd /tmp && tar xzf pgwd.tar.gz

# Install binary (on illumos use cp+chmod if install fails)
pfexec mkdir -p /usr/local/bin
pfexec cp pgwd /usr/local/bin/pgwd && pfexec chmod 755 /usr/local/bin/pgwd

# Install SMF method script
pfexec cp share/solaris/smf/pgwd /lib/svc/method/pgwd && pfexec chmod 555 /lib/svc/method/pgwd

# Install SMF manifest (site scope for admin-installed services)
pfexec mkdir -p /lib/svc/manifest/site
pfexec cp share/solaris/smf/pgwd.xml /lib/svc/manifest/site/pgwd.xml && pfexec chmod 444 /lib/svc/manifest/site/pgwd.xml

# Config (required; tarball includes etc/pgwd/pgwd.conf.example)
pfexec mkdir -p /etc/pgwd
pfexec cp etc/pgwd/pgwd.conf.example /etc/pgwd/pgwd.conf
```

### Option B: Build from source and copy (when Solaris tarball not in release)

On your Mac or Linux (with Go):

```bash
cd /path/to/pgwd
GOOS=solaris GOARCH=amd64 go build -o pgwd-solaris ./cmd/pgwd
```

On the Solaris server, create directories first:

```bash
pfexec mkdir -p /usr/local/bin /etc/pgwd /lib/svc/manifest/site
```

From your build machine, copy files:

```bash
scp pgwd-solaris root@YOUR_SOLARIS_IP:/usr/local/bin/pgwd
scp contrib/solaris/smf/pgwd root@YOUR_SOLARIS_IP:/lib/svc/method/pgwd
scp contrib/solaris/smf/pgwd.xml root@YOUR_SOLARIS_IP:/lib/svc/manifest/site/pgwd.xml
scp contrib/pgwd.conf.example root@YOUR_SOLARIS_IP:/etc/pgwd/pgwd.conf
```

On Solaris, set permissions:

```bash
pfexec chmod 755 /usr/local/bin/pgwd
pfexec chmod 555 /lib/svc/method/pgwd
pfexec chmod 444 /lib/svc/manifest/site/pgwd.xml
```

**From repo (no tarball):** `pfexec cp contrib/solaris/smf/pgwd /lib/svc/method/pgwd` and `pfexec cp contrib/solaris/smf/pgwd.xml /lib/svc/manifest/site/pgwd.xml` (run as root or with pfexec).

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
client: "solaris-monitor-01"
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

## SMF service

### Setup

1. Install pgwd, the SMF method script, manifest, and create the config (see [Config](#config-required) above).
2. Import the manifest and enable the service:

```bash
pfexec svccfg import /lib/svc/manifest/site/pgwd.xml
# On illumos/OmniOS the service is svc:/application/pgwd:default (use 'svcs -a | grep pgwd' to confirm)
pfexec svcadm enable svc:/application/pgwd:default
```

3. Verify:

```bash
svcs -v svc:/application/pgwd:default
tail -f /var/svc/log/application-pgwd:default.log
```

### Commands

| Command | Action |
|---------|--------|
| `svcadm enable svc:/application/pgwd:default` | Start and enable on boot |
| `svcadm disable svc:/application/pgwd:default` | Stop and disable on boot |
| `svcadm restart svc:/application/pgwd:default` | Restart the service |
| `svcs svc:/application/pgwd:default` | Check status |

### Custom config path

The method script uses `-config /etc/pgwd/pgwd.conf` by default. To use a different path, edit `/lib/svc/method/pgwd` and change the `exec` line:

```sh
exec /usr/local/bin/pgwd -config /etc/pgwd/prod.conf
```

### Logging

SMF captures stdout/stderr. Get the log path with `svcs -L svc:/application/pgwd:default`; typically `/var/svc/log/application-pgwd:default.log`. View logs:

```bash
tail -f $(svcs -L svc:/application/pgwd:default)
```

**Log rotation:** Solaris logadm can rotate SMF logs. Check `/etc/logadm.conf` for existing rules.

## Cron (one-shot)

For periodic checks instead of a daemon:

```bash
# crontab -e
PATH=/usr/local/bin:/usr/bin:/bin
*/5 * * * * pgwd -config /etc/pgwd/pgwd.conf >> /var/log/pgwd.log 2>&1
```

Set `interval: 0` in the config when using cron (or omit; pgwd runs once per cron tick).

## Kubernetes (kube-postgres, kube-loki)

**Use case:** pgwd runs on an external Solaris host (e.g. VM) with a kubeconfig that has access to the cluster. Postgres and Loki run inside the cluster. pgwd uses `kubectl port-forward` to reach both and sends alerts to Loki and Slack.

**kubectl on Solaris:** The official Kubernetes releases do not provide a pre-built kubectl binary for Solaris or illumos. To use `-kube-postgres` or `-kube-loki` on Solaris, you must either:

1. **Build kubectl from source:** Go supports Solaris (`GOOS=solaris GOARCH=amd64`), so you can build kubectl from the [kubernetes/kubectl](https://github.com/kubernetes/kubectl) repo or the main Kubernetes release tag.
2. **Use a community port:** Projects like [Uwubernetes](https://kubernaut.eu/posts/uwubernetes-kubernetes-v1-30-for-illumos-openbsd-freebsd/) provide kubectl builds for illumos/Solaris; check compatibility with your Oracle Solaris version.

Ensure `kubectl` is in `PATH` (e.g. `/usr/local/bin`) before running pgwd with kube options.

Example config:

```yaml
client: "pgwd-solaris-01"
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

**Required:** Set `KUBECONFIG` in the environment. For SMF, add an `environment` property group to the manifest or use a wrapper script. For cron: `KUBECONFIG=/path/to/kubeconfig` in the cron line.

**Grafana:** Loki logs include a `client` label. Filter by `{app="pgwd", client="pgwd-solaris-01"}`.

## SSH: root login with public key (Solaris 11.3)

Solaris 11.3 uses SunSSH, which integrates with PAM. By default, PAM requires a password even when the SSH server accepts a public key—so key-based auth appears to "fail" and falls back to keyboard-interactive (password prompt). To allow root login via SSH keys without a password:

### 1. sshd_config

```bash
# In /etc/ssh/sshd_config:
PermitRootLogin without-password
PubkeyAuthentication yes
PAMServiceName sshd
```

### 2. Root's authorized_keys

```bash
mkdir -p /root/.ssh
chmod 700 /root/.ssh
# Add your public key(s) to /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
```

### 3. PAM config (critical)

The default PAM stack (`/etc/pam.d/other`) uses `pam_unix_auth.so.1` (required), which demands a password. SunSSH runs PAM after accepting the key; PAM then fails and auth falls back to password. To fix, add an sshd-specific PAM config in `/etc/pam.conf` that permits auth without password:

```bash
# Append to /etc/pam.conf:
sshd  auth     required    pam_allow.so.1
sshd  account  required    pam_unix_account.so.1
sshd  session  required    pam_unix_session.so.1
sshd  password required    pam_allow.so.1
```

Then restart: `svcadm restart ssh`.

### 4. Client options (modern SSH vs. SunSSH)

Modern OpenSSH clients disable `ssh-rsa` by default. SunSSH 2.2 only offers `ssh-rsa` and `ssh-dss`. From the client:

```bash
ssh -o HostKeyAlgorithms=ssh-rsa -o PubkeyAcceptedAlgorithms=ssh-rsa -p 22 root@YOUR_SOLARIS_IP
```

Or add to `~/.ssh/config`:

```
Host pgwd-solaris
    HostName YOUR_SOLARIS_IP
    Port 22
    User root
    HostKeyAlgorithms ssh-rsa
    PubkeyAcceptedAlgorithms ssh-rsa
```

### Note on pam_allow

`pam_allow.so.1` permits auth without challenge. Because `PermitRootLogin without-password` already restricts root to key-only, using `pam_allow` for the sshd auth stack is acceptable: only users who passed SSH key verification reach PAM.

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| `svccfg import` fails | Ensure manifest path is correct and XML is valid. Check `svccfg validate`. |
| `svcadm enable svc:/site/pgwd:default` not found | On illumos the FMRI is `svc:/application/pgwd:default`. Run `svcs -a | grep pgwd` to find it. |
| Service stays in maintenance | Check `/var/svc/log/application-pgwd:default.log` for errors. Ensure config exists and pgwd binary is at `/usr/local/bin/pgwd`. |
| `pgwd is not running` after enable | Ensure `interval` in config is > 0 (e.g. 60) for daemon mode. If 0, pgwd runs once and exits. |
| `no pg_hba.conf entry` | Add the Solaris host IP to PostgreSQL `pg_hba.conf` on the DB server |
| Binary not found | The method script expects `/usr/local/bin/pgwd`. Create `pfexec mkdir -p /usr/local/bin` first; on illumos use `cp`+`chmod` instead of `install`. |
| `kubectl not found in PATH` | When using `-kube-postgres` or `-kube-loki`, pgwd needs kubectl. Solaris has no official kubectl package; build from source or use a community port (see [Kubernetes](#kubernetes-kube-postgres-kube-loki) section). |
| Root SSH key accepted but still asks for password | PAM requires password by default. Add sshd entries to `/etc/pam.conf` with `pam_allow.so.1` for auth (see [SSH: root login with public key](#ssh-root-login-with-public-key-solaris-113)). |
| `pgwd` Segmentation Fault (core dumped) | Pre-built binaries require Solaris 11.4+ or illumos. Solaris 11.3 lacks `pipe2()` and has known Go runtime issues. Upgrade to 11.4 or use illumos; see [golang/go#61950](https://go.dev/issue/61950). |
