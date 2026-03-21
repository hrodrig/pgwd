# Changelog

All notable changes to this project are documented in this file.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). History below is derived from the project plan (release scope).

## [Unreleased]

### Added

- **Config file** (YAML): Load settings from `/etc/pgwd/pgwd.conf` (or `-config` / `PGWD_CONFIG`). Keys match `-flag` and `PGWD_*` env vars. Example: `contrib/pgwd.conf.example`.
- **.deb and .rpm packages:** Install `/etc/pgwd/pgwd.conf` from the example (type `config|noreplace` — not overwritten on upgrade if user modified). Edit before use. Also install systemd units to `/lib/systemd/system/` (pgwd.service, pgwd-once.service, pgwd.timer) — enable with `systemctl enable --now pgwd`. Debian/Ubuntu: prerm stops and disables services before removal; postrm removes `/etc/pgwd` on `apt purge`.

### Changed

- **Config file as single source:** When a config file is loaded, env vars (PGWD_*) are ignored; config file is the only source. When no config file exists, env vars apply. CLI flags always override. Removed `EnvironmentFile` from systemd units; use config file only.
- **Config file layout (breaking):** Reorganized YAML structure: `db` (url, threshold, stale_age, default_threshold_percent), `kube`, `notifications` (loki, slack). Top-level: client, cluster, interval, dry_run, etc.
- **client required (breaking):** `client` is now mandatory; no fallback to hostname or kube resource. Set in config or `-client`.
- **cluster from kubeconfig:** Cluster name is computed from kubeconfig when `-kube-postgres` is set. Removed `cluster` from config file and CLI; not configurable.
- **Notification CLI flags and env vars renamed (breaking):** `-loki-url` → `-notifications-loki-url`, `-slack-webhook` → `-notifications-slack-webhook`, etc. Env: `PGWD_LOKI_URL` → `PGWD_NOTIFICATIONS_LOKI_URL`, `PGWD_SLACK_WEBHOOK` → `PGWD_NOTIFICATIONS_SLACK_WEBHOOK`, etc. Aligns CLI and env with YAML structure (`notifications.loki.*`, `notifications.slack.*`).
- **DB threshold env vars renamed (breaking):** `PGWD_THRESHOLD_TOTAL` → `PGWD_DB_THRESHOLD_TOTAL`, `PGWD_THRESHOLD_ACTIVE` → `PGWD_DB_THRESHOLD_ACTIVE`, `PGWD_THRESHOLD_IDLE` → `PGWD_DB_THRESHOLD_IDLE`, `PGWD_THRESHOLD_STALE` → `PGWD_DB_THRESHOLD_STALE`, `PGWD_THRESHOLD_LEVELS` → `PGWD_DB_THRESHOLD_LEVELS`, `PGWD_STALE_AGE` → `PGWD_DB_STALE_AGE`, `PGWD_DEFAULT_THRESHOLD_PERCENT` → `PGWD_DB_DEFAULT_THRESHOLD_PERCENT`. Aligns env with YAML structure (`db.threshold.*`, `db.stale_age`, `db.default_threshold_percent`).
- **DB threshold CLI flags renamed (breaking):** `-threshold-total` → `-db-threshold-total`, `-threshold-active` → `-db-threshold-active`, `-threshold-idle` → `-db-threshold-idle`, `-threshold-stale` → `-db-threshold-stale`, `-threshold-levels` → `-db-threshold-levels`, `-stale-age` → `-db-stale-age`, `-default-threshold-percent` → `-db-default-threshold-percent`. Aligns CLI with YAML and env (`db.*`).
- **Man page** (`man pgwd`): `contrib/man/man1/pgwd.1` with all options, examples, and env vars. `make install-man` (MANDIR defaults to /usr/local/share/man). `.deb` and `.rpm` packages include the man page in `/usr/share/man/man1/`. Homebrew cask installs the man page automatically.
- **Release tarball:** LICENSE included as `share/doc/pgwd/LICENSE` for Alpine and other packagers (MIT compliance).
- **scripts/install.sh:** One-liner installer for Linux, macOS, and BSD (downloads latest release, extracts to BINDIR).
- **OpenBSD pledge:** On OpenBSD, pgwd calls `pledge()` to restrict syscalls (stdio, rpath, inet, dns, proc). Stub on other platforms.
- **Makefile:** `check-docker` runs before snapshot, release, test-integration, test-e2e-kube, docker-build, docker-scan — fails early with clear message if Docker is not running.
- **Cursor rule:** `.cursor/rules/man-page-sync.mdc` — keep man page in sync when adding flags or changing version.

---

## [0.5.6] - 2026-03-21

### Added

- **FreeBSD port:** `contrib/freebsd/` with Makefile, pkg-plist, pkg-descr, and rc.d script. Install from local port or (when accepted) official ports. See `contrib/freebsd/README.md`.
- **FreeBSD rc.d:** Daemon with `daemon(8)` for logging to `/var/log/pgwd.log`. Custom stop/status using pidfile (supervisor pid). rc.conf variables: `pgwd_enable`, `pgwd_flags`, `pgwd_config`, `pgwd_env`, `pgwd_logfile`. Supports kube-postgres and kube-loki (external VPS with kubeconfig).
- **README:** FreeBSD section (port, tarball, config, daemon, cron). Main README badge and FreeBSD tarball URLs updated.

### Changed

- **contrib/freebsd:** Config example installed to `${PREFIX}/etc/pgwd/` (was `/etc/pgwd/`). Reinstall: `make deinstall`, `make clean`, `make install` to pick up port file changes.

---

## [0.5.4] - 2026-03-19

### Added

- **Alpine Linux (OpenRC):** `contrib/openrc/pgwd.initd` init script. See `contrib/openrc/README.md`. Main README: Alpine section with install and daemon setup.
- **OpenBSD rc.d:** `contrib/openbsd/pgwd` script for rc.d. Tarball `pgwd_v*_openbsd_amd64.tar.gz` includes rc.d script and config example. See `contrib/openbsd/README.md`. Main README: OpenBSD section. Logging: set `pgwd_logger="daemon.info"` in rc.conf.local to send output to syslog (`tail -f /var/log/daemon`).
- **OpenBSD + Kubernetes:** When using `-kube-postgres` or `-kube-loki`, pgwd skips pledge on OpenBSD so kubectl can run (pledge would block exec). Documented in `contrib/openbsd/README.md` with anonymous config example (external VPS, kubeconfig, port-forward to Postgres and Loki).
- **Loki client label:** `client` stream label and log line prefix when set. Enables Grafana filtering by instance (e.g. `{app="pgwd", client="pgwd-vps-01"}`).

### Changed

- **Docker:** Runtime base image `alpine:3.23` → `alpine:3.21` to avoid CVE-2026-2673 (OpenSSL 3.5/3.6; 3.21 uses 3.3.6, not affected).

---

## [0.5.0] - 2026-03-13

### Added

- **Loki labels:** `database` and `cluster` stream labels when Event has them (from connection URL and `-cluster` or kubeconfig). Enables LogQL filtering by database and cluster.
- **Loki log line:** Database and cluster at the start of the message when present: `pgwd [cluster=X] [database=Y]: message | total=...`.
- **docs/loki-grafana-alerts.md:** Labels, log line format, LogQL examples, Grafana alert rule setup, JSON payload reference.
- **docs/testing-alert-levels.md:** Procedure to trigger attention/alert/danger with `-test-max-connections` against production without changing Postgres config.
- **testing/compose:** Resource limits (`mem_limit`, `cpus`) and non-root `user` for client; resource limits for postgres. Addresses Snyk findings.
- **testing/compose-loki:** Resource limits and non-root `user` (10001) for Loki.
- **testing/k8s/postgres.yaml, loki.yaml:** securityContext (allowPrivilegeEscalation, runAsNonRoot/runAsUser for Loki), resources limits, livenessProbe, imagePullPolicy. Addresses Snyk K8s findings.
- **Dockerfile, Dockerfile.release:** `apk update && apk upgrade` before ca-certificates to pick up zlib 1.3.2-r0 (fixes CVE-2026-22184, CVE-2026-27171).

---

## [0.4.0] - 2026-03-11

### Added

- **README:** Log rotation (logrotate) for cron logs — examples for `/var/log/pgwd.log` and `~/log/pgwd-cron.log` with `su username groupname` for logs in user home.
- **README:** Usage examples updated to use `-threshold-levels` (3-tier) as primary; `-threshold-total` and `-threshold-active` deprecated. Table and examples now show levels, idle, stale.
- **README:** TOC, logo/banner, "Back to top" links, and FAQ section (expandable) for better navigation and discoverability.
- **-kube-loki** (`PGWD_KUBE_LOKI`): Connect to Loki via kubectl port-forward when Loki is inside the cluster and pgwd runs outside (e.g. VM with cron). Same format as `-kube-postgres`: `namespace/svc/name` (e.g. `monitoring/svc/loki`). Mutually exclusive with `-notifications-loki-url`. Use `-kube-loki-local-port` and `-kube-loki-remote-port` (default 3100) when Loki uses a different port.
- **E2E kube test:** Now deploys Loki and runs `pgwd -kube-loki -force-notification` to validate the full path. `testing/k8s/loki.yaml` added.
- **docs:** Sequence diagrams audit ([docs/sequence/AUDIT.md](docs/sequence/AUDIT.md)) mapping each diagram step to code; README and docs/README link to it.
- **Cursor rule:** `.cursor/rules/diagrams-mermaid.mdc` — validate Mermaid rendering when adding/editing diagrams; avoid backticks, semicolons, and colons inside message text; keep diagrams in sync with code (see AUDIT.md).
- **tools/:** Scripts and docs for scanning before merge/release: `tools/scan.sh` (govulncheck + optional Grype on dir), [tools/README.md](tools/README.md) (install Grype, scan image with Grype, realistic results, do not upgrade zlib/base packages in Alpine). CI: `.github/workflows/security.yml` (govulncheck + Grype on built image, `--fail-on high,critical`). Release rule and AGENTS updated to run scan before release.

### Changed

- **docs:** Mermaid diagram fixes so all sequence diagrams render correctly (semicolon/colon in message text; 01, 02, 05, 07).
- **docs:** Diagram 04 (dry-run) — log line now mentions `max_connections` when available.
- **docs:** Diagram 07 (connect-failure) — log step shows fixed message (no error detail); run context in parentheses.

---

## [0.3.6] - 2026-03-03

### Fixed

- **kube port-forward:** setupKube had `defer cleanup()` inside it; in Go defer runs when the enclosing function returns, so the port-forward was killed as soon as setupKube returned. Now setupKube returns the cleanup function and main defers it. Regression introduced in v0.2.4 refactor; v0.2.2 worked correctly.

### Added

- **-validate-k8s-access** (`PGWD_VALIDATE_K8S_ACCESS`): validate kubectl connectivity and list pods, then exit. Use `-kube-context` to select context. Useful before running pgwd with `-kube-postgres`.
- **E2E kube test:** `make test-e2e-kube` — creates kind cluster, deploys Postgres, runs `pgwd -validate-k8s-access` and `pgwd -kube-postgres -dry-run`, then destroys cluster. Requires kind, kubectl, Docker. `testing/k8s/postgres.yaml`, `testing/scripts/test-e2e-kube.sh`.
- **CI:** `test-e2e-kube` job in GitHub Actions.
- **release-check:** `test-e2e-kube` added to pre-release checklist.

### Deprecated

- **`-threshold-total` and `-threshold-active`:** use `-threshold-levels` instead (e.g. `-threshold-levels 75,85,95`). Will be removed in v1.0.0. A warning is printed to stderr when these flags are used.

---

## [0.3.1] - 2026-03-03

### Fixed

- **Dockerfile:** Replace `COPY . .` with explicit `COPY cmd/` and `COPY internal/` to avoid CopyIgnoredFile warning when using whitelist `.dockerignore`.
- **Security workflow:** Use `--fail-on high` (single value) instead of `high,critical`; Grype 0.109+ accepts one severity only.

### Changed

- **tools/README:** Update Grype examples to match workflow.

---

## [0.3.0] - 2026-03-03

### Added

- **3-tier alert levels:** **`-threshold-levels`** (`PGWD_THRESHOLD_LEVELS`): when both `threshold-total` and `threshold-active` are 0, use comma-separated percentages (default 75,85,95). Levels: **attention** (1st), **alert** (2nd), **danger** (3rd). Only the highest breached level fires. MySQL-style thresholds.
- **Slack:** Level-specific colors (yellow/orange/red) and emojis (large_yellow_circle, large_orange_circle, red_circle) for attention/alert/danger.
- **Loki:** `level` label derived from percentage when available (attention/alert/danger).
- **Config:** `ParseThresholdLevels`, `UsesLevelMode`; `DefaultThresholdLevels` constant.

### Changed

- **Default behaviour:** When both total and active thresholds are 0, pgwd now uses 3-tier level mode (75,85,95%) instead of a single default-threshold-percent. Use `-threshold-total` or `-threshold-active` to keep the previous single-threshold behaviour.
- **Explicit thresholds:** When using `threshold-total` or `threshold-active`, Level is now computed from the actual percentage for correct Slack/Loki colors (attention/alert/danger).
- **default-threshold-percent:** Now only applies when one of total/active is set (ignored in level mode).

---

## [0.2.4] - 2026-03-02

### Added

- **Kubernetes context:** **-kube-context** (`PGWD_KUBE_CONTEXT`) to select which kubeconfig context to use when you have multiple (e.g. dev, staging, prod). All kubectl operations (port-forward, pod resolution, password discovery, cluster name) use that context. README: parameters table and "Multiple contexts" in Kubernetes section.
- **Makefile:** **docker-scan** target — build image as `pgwd:scan`, run Grype with `--fail-on high`. Requires Docker and Grype on PATH.
- **Release tests:** `.cursor/rules/release-tests.mdc` — **make docker-scan** added to pre-release checklist.

---

## [0.2.3] - 2026-02-28

### Added

- **Connect failure / too many clients:** Notify on any connection failure when notifiers are configured (no `-notify-on-connect-failure` required). Send even in dry-run. New event **too_many_clients** (Slack/Loki URGENT) when Postgres returns 53300. When `applyThresholdDefaults` fails (e.g. first query "too many clients"), notify then exit. Log "Sending notification…" before sending.
- **testing:** Non-superuser **pgwd_app** for client containers (init script); reserved slots stay free for DBA (`psql -U pgwd`). README: recreate steps, production note and link to [PostgreSQL runtime-config-connection](https://www.postgresql.org/docs/current/runtime-config-connection.html) (`superuser_reserved_connections`). Whitelist `testing/` in .gitignore.
- **make lint / lint-fix** (gofmt -s, gocyclo); CI runs lint; cursor rules updated.

### Changed

- **Docs/diagram 07:** Connect failure always notifies when a notifier is configured; `-notify-on-connect-failure` documented as legacy.
- **README/AGENTS:** Connect failure behaviour; troubleshooting for "no thresholds set" and notify-on-connect-failure.

---

## [0.2.2] - 2026-02-26

### Added

- **-test-max-connections** (`PGWD_TEST_MAX_CONNECTIONS`): override server `max_connections` for threshold defaults and display (testing only). Notifications show "(test override)" when used.
- **README:** Flag and usage (parameters table, "Test alerts without low max_connections"); "Running from cron" (PATH for kubectl, redirecting logs).

### Changed

- **demo.gif** regenerated for 0.2.2.

---

## [0.2.1]

### Added

- **CodeQL:** No clear-text logging of sensitive data (DB URL, kube password, connect errors).
- Override of `max_connections` for testing (later renamed to **-test-max-connections** in 0.2.2).

### Changed

- **Slack/Loki:** Test notification wording ("delivery check"), connection line format; **max_connections** in messages.

---

## [0.2.0]

### Added

- **Slack:** Run context (cluster, client, namespace, database), attachment **colors** (green/red/yellow by event type).
- **Kubernetes:** **-kube-postgres** (namespace/svc or pod), kubectl port-forward, optional **DISCOVER_MY_PASSWORD** from pod env.

### Changed

- **Docs:** Sequence diagrams updated, release-tests rule.

---

## [0.1.8] (pre-0.2.0, not tagged)

### Added

- **AGENTS.md**, **SECURITY.md**, **.agents/**.
- Tests before release (release-tests rule).

---

## [0.1.7]

### Added

- **Docker:** Multi-arch image to **ghcr.io/hrodrig/pgwd** (Dockerfile.release), dockers_v2 in goreleaser.

### Changed

- **VERSION** 0.1.7.
- **README:** Version badge, embed **demo.gif**, Docker section with ghcr.io.
- **.cursor/rules:** Version badge sync when bumping.
- **.gitignore:** Allow Dockerfile.release.

---

## [0.1.6]

### Added

- **VERSION** 0.1.6, **go.mod** Go 1.26.
- **Makefile:** build/install/test, cross-compile, release/snapshot, docker-build (VERSION, COMMIT, BUILDDATE).
- **Docker:** Multi-stage Go 1.26 / Alpine 3.23, non-root user `pgwd`, ca-certificates only, OCI labels, whitelist `.dockerignore`.
- **README:** Badges (Release, Go, License), Releases link, Docker section.
- **Docs:** `docs/` sequence diagrams (Mermaid), demo.tape (VHS), docs/README.
- **Cursor:** `.cursorrules` → `.cursor/rules/` (git-flow, gitignore-whitelist, language-english), rule `readme-badges-version.mdc`.
- **Goreleaser:** .goreleaser.yaml (builds, nfpms, homebrew, changelog).
- **Install section:** go install @latest, releases link.

---

## Initial / 0.1.x baseline

- **pgwd CLI:** thresholds (total, active, idle, stale), Slack and Loki notifiers, defaults from server `max_connections`, systemd units, Docker, tests.

---

[Unreleased]: https://github.com/hrodrig/pgwd/compare/v0.5.6...HEAD
[0.5.6]: https://github.com/hrodrig/pgwd/compare/v0.5.4...v0.5.6
[0.5.4]: https://github.com/hrodrig/pgwd/compare/v0.5.0...v0.5.4
[0.5.0]: https://github.com/hrodrig/pgwd/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/hrodrig/pgwd/compare/v0.3.6...v0.4.0
[0.3.6]: https://github.com/hrodrig/pgwd/compare/v0.3.1...v0.3.6
[0.3.1]: https://github.com/hrodrig/pgwd/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/hrodrig/pgwd/compare/v0.2.4...v0.3.0
[0.2.4]: https://github.com/hrodrig/pgwd/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/hrodrig/pgwd/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/hrodrig/pgwd/compare/v0.2.0...v0.2.2
[0.2.1]: https://github.com/hrodrig/pgwd/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/hrodrig/pgwd/compare/v0.1.7...v0.2.0
[0.1.8]: https://github.com/hrodrig/pgwd/compare/v0.1.7...v0.1.8
[0.1.7]: https://github.com/hrodrig/pgwd/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/hrodrig/pgwd/releases/tag/v0.1.6
