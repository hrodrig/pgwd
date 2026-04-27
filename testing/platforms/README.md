# Platform Testing for pgwd

Automated multi-platform validation of pgwd installation, daemon management, notifications (Loki + Slack), and uninstallation using Ansible.

## Scope

These tests validate that pgwd installs, runs as a daemon, sends notifications, and uninstalls cleanly on each **operating system**. They cover the native deployment path: package install, init system, config file.

Kubernetes and container deployments are **not** in scope here — they are covered by:

- `make test-e2e-kube` — Kind cluster with `pgwd -kube-postgres`, `DISCOVER_MY_PASSWORD`, Loki verification.
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
| OpenBSD 7.x | tarball | rc.d (rcctl) |
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

# 2. Run the full release validation cycle (all platforms)
ansible-playbook playbooks/full-cycle.yml

# 3. Or target a single platform
ansible-playbook playbooks/full-cycle.yml --limit pgwd-ubuntu

# 4. Or target a group
ansible-playbook playbooks/full-cycle.yml --limit linux_systemd
```

From the repo root you can also use:

```bash
# All platforms
make test-platforms

# Single platform
make test-platforms PLATFORM=pgwd-ubuntu
```

## Playbooks

| Playbook | Description |
|---|---|
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
    pgwd_version: "0.5.10"
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

### Testing a local build

To test binaries you built locally instead of a published release, set `pgwd_local_package` **per host** (each platform needs its own format):

```yaml
    linux_systemd:
      hosts:
        pgwd-ubuntu:
          ansible_host: 203.0.113.10
          ansible_port: 2298
          ansible_user: root
          platform_vars: ubuntu
          pgwd_local_package: "/tmp/dist/pgwd_v0.5.10_linux_amd64.deb"
        pgwd-almalinux:
          ansible_host: 203.0.113.10
          ansible_port: 2299
          ansible_user: root
          platform_vars: almalinux
          pgwd_local_package: "/tmp/dist/pgwd_v0.5.10_linux_amd64.rpm"
    linux_openrc:
      hosts:
        pgwd-alpine:
          ansible_host: 203.0.113.10
          ansible_port: 2297
          ansible_user: root
          platform_vars: alpine
          pgwd_local_package: "/tmp/dist/pgwd_v0.5.10_linux_amd64.tar.gz"
```

If `pgwd_local_package` is not set for a host, the role downloads the package from `pgwd_release_url` automatically.

## Notification mock

The test suite includes a lightweight Python 3 HTTP server that captures both Loki and Slack notifications:

- **Loki**: `POST /loki/api/v1/push` -> captured to `/tmp/pgwd-mock-loki-api-v1-push.json`
- **Slack**: `POST /slack` -> captured to `/tmp/pgwd-mock-slack.json`

Both endpoints return `200 {}`. The mock requires no dependencies beyond Python 3.

## Pre-release checklist

Before merging to `main` and tagging a release, run the full platform validation:

```bash
# 1. Build snapshot packages
make snapshot

# 2. Update hosts.yml with pgwd_local_package paths pointing to dist/
# 3. Run all platforms
make test-platforms

# 4. If all pass, proceed with release-check and merge
make release-check
```

This is a manual gate (not in CI) because it requires external VMs. At minimum, test the platforms affected by the release changes.

## Adding a new platform

1. Create `platform_vars/<name>.yml` with `pgwd_install_method`, `pgwd_init_system`, `pgwd_binary_path`, and `pgwd_tarball_os`.
2. Add the host to your `inventory/hosts.yml` with `platform_vars: <name>`.
3. If the init system is new (not systemd/OpenRC/rc.d), add handlers to `roles/pgwd_daemon/tasks/main.yml`.
