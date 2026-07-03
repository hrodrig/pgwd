# Platform Testing for pgwd

Automated multi-platform validation of pgwd installation, daemon management, notifications (Loki + Slack), and uninstallation using Ansible.

## Scope

These tests validate that pgwd installs, runs as a daemon, sends notifications, and uninstalls cleanly on each **operating system**. They cover the native deployment path: package install, init system, config file.

Kubernetes and container deployments are **not** in scope here — they are covered by:

- `make test-e2e-kube` — Kind cluster with `pgwd -kube-postgres`, Loki verification (`DISCOVER_MY_PASSWORD` still used in e2e until 0.9.x migration).
- `make docker-scan` — Container image security scan (Grype).

| Test suite | What it validates |
|---|---|
| `make test-platforms` | Binary on bare-metal OS (deb/rpm/tarball, systemd/OpenRC/rc.d) |
| `make test-e2e-kube` | Binary with K8s (kube-postgres, port-forward, Loki check) |
| `make docker-scan` | Container image (vulnerabilities) |

## Supported platforms

| Platform | Install method | Init system |
|---|---|---|
| Debian 13 | .deb | systemd |
| Ubuntu 24.x | .deb | systemd |
| AlmaLinux 9 | .rpm (dnf) | systemd |
| OpenSUSE | .rpm (zypper) | systemd |
| Arch Linux | tarball | systemd |
| Alpine 3.x | tarball | OpenRC |
| FreeBSD 15 | tarball | rc.d (sysrc/service) |
| OpenBSD 7.x | tarball (+ optional `share/openbsd/rc.d/pgwd` in release) | rc.d (`rcctl`; repo `contrib/openbsd/pgwd` in Ansible) |
| NetBSD 10.x | tarball | rc.d (/etc/rc.d) |
| DragonFly BSD 6.x | tarball | rc.d (sysrc/service) |

## Prerequisites

### On your workstation

- [Ansible](https://docs.ansible.com/ansible/latest/installation_guide/) (2.14+)
- SSH key-based root access to the target VMs

### Infrastructure (your responsibility)

1. **VMs accessible via SSH as root.** Any topology works: VMs behind a firewall with port-forwarded SSH (same public IP, different ports), direct IPs, cloud instances, or local VMs.
2. **At least 3 PostgreSQL instances** reachable from the VMs (same subnet or routed). Used for multi-database testing. These can be 3 separate hosts, 3 containers, or 3 databases on one server.
3. **Python 3** on each target VM (required by Ansible and the notification mock). Debian, Ubuntu, AlmaLinux, and OpenSUSE include it. Minimal distros need manual install: Alpine `apk add python3`, Arch `pacman -S python`, FreeBSD `pkg install python3`, OpenBSD `pkg_add python%3`, NetBSD `pkg_add python312` (then `ln -s /usr/pkg/bin/python3.12 /usr/pkg/bin/python3`), DragonFly `pkg install python3`.
4. **BSD hosts** need `ansible_python_interpreter` in inventory: FreeBSD/OpenBSD/DragonFly use `/usr/local/bin/python3`, NetBSD uses `/usr/pkg/bin/python3`. See `hosts.yml.example`.

## Quick start

```bash
# 1. Copy the example inventory and edit with your IPs, ports, Postgres URLs
cp inventory/hosts.yml.example inventory/hosts.yml
vim inventory/hosts.yml

# 2. Verify SSH and Ansible can reach each host using Ansible's ping module (not ICMP).
#    ansible.builtin.ping checks the connection and remote Python; a successful host shows "pong".
cd testing/platforms
ansible-playbook playbooks/ping.yml
# Optional: one host or group
ansible-playbook playbooks/ping.yml --limit pgwd-ubuntu

# 3. Run the full release validation cycle (all platforms)
ansible-playbook playbooks/full-cycle.yml

# 4. Or target a single platform
ansible-playbook playbooks/full-cycle.yml --limit pgwd-ubuntu

# 5. Or target a group
ansible-playbook playbooks/full-cycle.yml --limit linux_systemd
```

From the repo root you can also use:

```bash
# Connectivity only — Ansible ping module (success → "pong"), same as:
#   cd testing/platforms && ansible-playbook playbooks/ping.yml
make test-platforms-ping

# Single host
make test-platforms-ping PLATFORM=pgwd-ubuntu

# All platforms (full cycle)
make test-platforms

# Single platform
make test-platforms PLATFORM=pgwd-ubuntu
```

## Playbooks

| Playbook | Description |
|---|---|
| `ping.yml` | Runs `ansible.builtin.ping` on each host (not ICMP); success output includes **`pong`**. Validates SSH, inventory, and remote Python before `full-cycle.yml` |
| `setup.yml` | Install pgwd, deploy config, start daemon |
| `test.yml` | Dry-run, notification (Loki+Slack) tests, timer tests |
| `teardown.yml` | Uninstall pgwd, verify full cleanup |
| `full-cycle.yml` | Setup, test, teardown (full release validation) |

## Test flow

```
setup.yml                    test.yml                         teardown.yml
+-----------+               +--------------+                  +-------------+
| install   |               | dry-run      |                  | uninstall   |
| configure | ------------> | notifications| ---------------> | verify      |
| daemon    |               | timer        |                  | cleanup     |
+-----------+               +--------------+                  +-------------+
```

1. **Install**: Downloads the release package (or uses a local build) and installs it.
2. **Configure**: Deploys `/etc/pgwd/pgwd.conf` from template with multi-database config and mock notification endpoints.
3. **Daemon**: Starts via the platform's init system (systemd/OpenRC/rc.d), verifies it's running, then stops.
4. **Dry-run**: Runs `pgwd -dry-run` against all configured Postgres instances, checks output.
5. **Notifications**: Starts a Python mock HTTP server, runs `pgwd -force-notification`, verifies both Loki and Slack captures.
6. **Timer**: Tests the scheduled execution mechanism (systemd timer or cron).
7. **Uninstall**: Removes package/binary, config, init scripts. Verifies nothing is left.

## Inventory

Edit `inventory/hosts.yml` (not committed). Each host only needs `ansible_host`, `ansible_port`, and `ansible_user`. Any topology works: all VMs behind one firewall IP with NAT ports, each VM with its own public IP, or a mix of both.

**Example: VMs behind a single firewall (NAT port-forwarding)**

```yaml
all:
  vars:
    pgwd_version: "0.6.4"
    postgres_instances:
      - url: "postgres://pgwd:secret@10.0.0.50:5432/pgwd?sslmode=disable"
        client: "pgwd-main"
      - url: "postgres://pgwd:secret@10.0.0.51:5432/analytics?sslmode=disable"
        client: "pgwd-analytics"
      - url: "postgres://pgwd:secret@10.0.0.52:5432/replica?sslmode=disable"
        client: "pgwd-replica"
  children:
    linux_systemd:
      hosts:
        pgwd-ubuntu:
          ansible_host: 203.0.113.10
          ansible_port: 2298
          ansible_user: root
          platform_vars: ubuntu
        pgwd-almalinux:
          ansible_host: 203.0.113.10
          ansible_port: 2299
          ansible_user: root
          platform_vars: almalinux
```

**Example: each VM with its own public IP**

```yaml
    linux_systemd:
      hosts:
        pgwd-ubuntu:
          ansible_host: 45.77.100.10
          ansible_port: 22
          ansible_user: root
          platform_vars: ubuntu
        pgwd-almalinux:
          ansible_host: 167.233.50.20
          ansible_port: 2222
          ansible_user: root
          platform_vars: almalinux
```

### Published release vs local packages

If **`pgwd_local_package` is not set** for a host, the role downloads from `pgwd_release_url` using `pgwd_version`. That only works when **`v{{ pgwd_version }}` exists on GitHub**.

You can explicitly choose package source behavior with **`pgwd_package_source`**:

- `auto` (default): use `pgwd_local_package` when set; otherwise download from `pgwd_release_url`.
- `release`: always download published assets.
- `local`: always use `pgwd_local_package` (fails fast if missing for a host).

Examples:

```bash
# Force published artifacts for this run
cd testing/platforms
ansible-playbook playbooks/full-cycle.yml \
  --limit pgwd-freebsd \
  -e pgwd_package_source=release \
  -e pgwd_version=0.6.4

# Force local artifacts for this run (requires pgwd_local_package on targeted hosts)
ansible-playbook playbooks/full-cycle.yml \
  --limit pgwd-freebsd \
  -e pgwd_package_source=local \
  -e pgwd_version=0.6.4-next
```

### Local snapshot (before you create a release tag)

Use this to validate VMs against **`make snapshot`** artifacts (no GitHub release required).

1. From the repo root, run **`make snapshot`** ([goreleaser](https://goreleaser.com) on `PATH`; no Docker required; artifacts under **`dist/`**).
2. Open **`dist/metadata.json`** and read the **`version`** field (for example `0.6.4-next`). Snapshot naming comes from the repo **`VERSION`** file (`make snapshot` sets `PGWD_SNAPSHOT_VERSION=<VERSION>-next` for GoReleaser).
3. Set **`pgwd_version`** in `inventory/hosts.yml` (under `all.vars` or per group) to **that exact `version` string**. The install role runs **`pgwd -version`** and asserts this value appears in the output.
4. List **`dist/`** and set **`pgwd_local_package`** on each host to the matching artifact using a **full absolute path on the machine where you run `ansible-playbook`** (the control node). `ansible.builtin.copy` reads those files locally before pushing to the VM.
5. Run **`make test-platforms-ping`**, then **`make test-platforms`** (or `--limit` one host).

Example after a snapshot (adjust paths and the `0.6.4-next` placeholder to match **your** `metadata.json` and `ls dist/`):

```yaml
all:
  vars:
    pgwd_version: "0.6.4-next"   # from dist/metadata.json → "version"
    pgwd_release_url: "https://github.com/hrodrig/pgwd/releases/download/v{{ pgwd_version }}"
  children:
    linux_systemd:
      hosts:
        pgwd-ubuntu:
          ansible_host: 203.0.113.10
          ansible_port: 2298
          ansible_user: root
          platform_vars: ubuntu
          pgwd_local_package: "/full/path/to/pgwd/dist/pgwd_v0.6.4-next_linux_amd64.deb"
        pgwd-almalinux:
          ansible_host: 203.0.113.10
          ansible_port: 2299
          ansible_user: root
          platform_vars: almalinux
          pgwd_local_package: "/full/path/to/pgwd/dist/pgwd_v0.6.4-next_linux_amd64.rpm"
    linux_openrc:
      hosts:
        pgwd-alpine:
          ansible_host: 203.0.113.10
          ansible_port: 2297
          ansible_user: root
          platform_vars: alpine
          pgwd_local_package: "/full/path/to/pgwd/dist/pgwd_v0.6.4-next_linux_amd64.tar.gz"
```

For **BSD** hosts, use the matching `*_freebsd_*`, `*_openbsd_*`, `*_netbsd_*`, or `*_dragonfly_*` tarball from `dist/`.

### OpenBSD (Ansible vs manual install)

| Path | What gets installed |
| --- | --- |
| **Ansible `full-cycle`** | Binary (+ man) from release tarball or `pgwd_local_package`; **`contrib/openbsd/pgwd`** → `/etc/rc.d/pgwd` (overwrites tarball rc.d when both run); `rcctl` enable/start; dry-run + notification mock + cron |
| **Release tarball only** | `pgwd`, man, **`share/openbsd/rc.d/pgwd`** — see **`contrib/openbsd/README.md`** |
| **OpenBSD ports** | **`contrib/openbsd/port/`** — not used by playbooks yet |

**OpenBSD rc.d:** `daemon="/usr/local/bin/pgwd"`, **`rc_bg=YES`**. No separate serve wrapper (unlike gghstats). VM checklist: **`contrib/openbsd/DEBUG-VM.md`**.

**Stuck daemon / port conflicts:** `rcctl stop pgwd; pkill -x pgwd`. Setup stops stray processes before each start.

**Without `curl`:** platform tests do not require HTTP on the host; dry-run and notification tests use the CLI. For optional **`/healthz`**, use `ftp` or install `curl` manually.

```bash
make test-platforms-ping PLATFORM=pgwd-openbsd
make test-platforms PLATFORM=pgwd-openbsd
```

### FreeBSD manual smoke test (CLI + rc.d)

When validating a FreeBSD local package manually (outside Ansible), run this quick sequence after install/reinstall:

```bash
# 1) CLI sanity
pgwd -version
pgwd -help | head -40
pgwd -config /etc/pgwd/pgwd.conf -dry-run -interval 0

# 2) rc.d in safe mode first
sysrc pgwd_enable=YES
sysrc pgwd_flags="-config /etc/pgwd/pgwd.conf -dry-run"
service pgwd restart
service pgwd status
tail -n 80 /var/log/pgwd.log

# 3) rc.d real mode
sysrc pgwd_flags="-config /etc/pgwd/pgwd.conf"
service pgwd restart
service pgwd status
tail -n 120 /var/log/pgwd.log
```

If the port install reports an older package already installed, use `make reinstall` in the port directory to replace it cleanly.

### Testing a local build (summary)

Set **`pgwd_local_package`** per host (`.deb`, `.rpm`, or `.tar.gz` as required by `platform_vars`). Keep **`pgwd_version`** aligned with whatever **`pgwd -version`** prints for that artifact (release tag or snapshot `version` from `metadata.json`).

## Notification mock

The test suite includes a lightweight Python 3 HTTP server that captures both Loki and Slack notifications:

- **Loki**: `POST /loki/api/v1/push` -> captured to `/tmp/pgwd-mock-loki-api-v1-push.json`
- **Slack**: `POST /slack` -> captured to `/tmp/pgwd-mock-slack.json`

Both endpoints return `200 {}`. The mock requires no dependencies beyond Python 3.

## Pre-release checklist

Before merging to `main` and tagging a release, validate on real VMs using a **local snapshot** (see **Local snapshot** above): `pgwd_version` from `dist/metadata.json`, `pgwd_local_package` as absolute paths into `dist/`.

```bash
# 1. Snapshot packages (no tag required)
make snapshot

# 2. Align inventory: pgwd_version = "version" from dist/metadata.json;
#    per-host pgwd_local_package = absolute path to matching dist/pgwd_v*_...

# 3. Connectivity, then full cycle
make test-platforms-ping
make test-platforms

# 4. If all pass, run repo release checks, merge, tag, then release
make release-check
```

This is a manual gate (not in CI) because it requires external VMs. At minimum, test the platforms affected by the release changes.

## Adding a new platform

1. Create `platform_vars/<name>.yml` with `pgwd_install_method`, `pgwd_init_system`, `pgwd_binary_path`, and `pgwd_tarball_os`.
2. Add the host to your `inventory/hosts.yml` with `platform_vars: <name>`.
3. If the init system is new (not systemd/OpenRC/rc.d), add handlers to `roles/pgwd_daemon/tasks/main.yml`.
