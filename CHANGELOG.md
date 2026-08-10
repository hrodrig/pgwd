# Changelog

All notable changes to this project are documented in this file.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Release bands: [ROADMAP.md](ROADMAP.md). Behavior contract: [SPECIFICATIONS.md](SPECIFICATIONS.md).

## [Unreleased]

## [1.1.0] - 2026-08-10

Incident hygiene for on-call: PagerDuty incident lifecycle and quieter threshold alerts by default.

### Added

- **PagerDuty:** stable `dedup_key` per target/problem class; `event_action: resolve` on resolution (auto-close incidents).
- **Config:** `notifications.repeat_while_firing` / `-notifications-repeat-while-firing` / `PGWD_NOTIFICATIONS_REPEAT_WHILE_FIRING` to restore per-interval threshold spam.

### Changed

- **Alert repeat:** by default, connection-threshold notifiers fire on transition, escalation, and de-escalation only — not every interval while the bad state persists. Set `repeat_while_firing: true` for v1.0 behavior.

### Docs

- **ROADMAP:** band **1.1.x** + [docs/plan-1.1.x.md](docs/plan-1.1.x.md); SPEC/man/example synced for latch and PagerDuty resolve.
- **README / ports:** version badge and install examples → **1.1.0**; FreeBSD/OpenBSD port Makefiles synced from `VERSION`.

## [1.0.1] - 2026-08-01

### Security

- **Dependencies:** bump transitive `golang.org/x/text` to **v0.39.0** ([GO-2026-5970](https://pkg.go.dev/vuln/GO-2026-5970) — infinite loop on invalid input).

### Docs

- **README:** fix deps.dev badge URL encoding; drop retired Go Report Card badge; sync BSD port / install examples to **1.0.1**; expand post-badge nav to family header pattern (Spec · Operator · Changelog · Roadmap · Article).
- **man(1):** `.TH` date/version → **pgwd v1.0.1**.

## [1.0.0] - 2026-07-18

Stable API release: remove remaining pre-1.0 config/flag surface, document exit codes **2**/**3**, and ship operator positioning docs (compare, upgrade, connection-limits).

### Added

- **Exit codes 2 and 3:** single-target connect failure exits **2** (after `Ping`); one-shot stats/query failure exits **3**. Daemon mode does not exit on query errors. Exit **4** (`-strict`) unchanged. See SPECIFICATIONS.md § Exit codes.
- **Brand mark:** [`docs/logo.svg`](docs/logo.svg) (and `docs/logo.png`) — cyan **`pg`** monogram + green live status dot; README header uses the SVG.

### Removed

- **Config key `db:`:** removed. Use **`databases:`** with one entry for a single Postgres target. Loading a file that still has `db:` fails with a migration hint.
- **Connect-failure legacy surface:** removed **`-notify-on-connect-failure`**, **`PGWD_NOTIFY_ON_CONNECT_FAILURE`**, and YAML **`notify_on_connect_failure`**. Connect failure notifications are always sent when notifiers are configured.
- **Legacy total/active thresholds:** removed **`-db-threshold-total`**, **`-db-threshold-active`**, **`PGWD_DB_THRESHOLD_TOTAL`**, **`PGWD_DB_THRESHOLD_ACTIVE`**, and YAML **`threshold.total`** / **`threshold.active`**. Use **`-db-threshold-levels`** instead.

### Changed

- **Cosign v3 signing:** checksum signatures use a single **`checksums.txt.sigstore.json`** bundle (`cosign sign-blob --bundle`); verify with `cosign verify-blob --bundle …`. `make snapshot` skips signing (no local OIDC).

### Documentation

- **[docs/postgresql-connection-limits.md](docs/postgresql-connection-limits.md)** — practical guide to connection saturation and sizing heuristics; README short problem blurb + link.
- **[docs/compare.md](docs/compare.md)** — transparent comparison vs postgres_exporter, pgwatch, SaaS, cloud alarms, DIY, Nagios; README § Compare.
- **[docs/UPGRADE-0.9-to-1.0.md](docs/UPGRADE-0.9-to-1.0.md)** — migration guide for 1.0 breaking removals.

## [0.9.0] - 2026-07-13

Pre-1.0 security and operator polish: removes insecure Kubernetes password discovery (`pods/exec`), hardens `/metrics` and CSV export, adds ready-to-use config profiles and optional daemon telemetry, and documents single- and multi-database deployment patterns.

### Security

- **Kubernetes:** removed **`DISCOVER_MY_PASSWORD`** / `pods/exec` password discovery. Use Secret-backed DSN, **`contrib/k8s/pgwd-kube-run.sh`**, or **`kube.password_from_secret`** — [docs/kubernetes-passwords.md](docs/kubernetes-passwords.md). Sample RBAC: **`contrib/k8s/rbac-outside-cluster.yaml`**.
- **`/metrics` exporter:** full Prometheus label-value escaping — fixes scraper breakage when `client` / `cluster` / `database` contain quotes, newlines, or control characters.
- **CSV export:** prefix sanitization on string columns — mitigates spreadsheet formula injection when opening exports in Excel or Google Sheets.
- **HTTP `/metrics`:** optional **`http.metrics_token`** (Bearer or `?token=`) and **`http.metrics_basic_*`** — **opt-in**; default empty = anonymous scrape (in-cluster Prometheus/Alloy unchanged). Limits exposure when `http.listen` is reachable outside a trusted network; **`/healthz` stays open** for probes. See **`contrib/k8s/README.md`**.

### Added

- **Config profiles:** ready-to-use YAML under **`contrib/profiles/`** (minimal-slack, daemon-loki, kube-prod, multi-db).
- **`-strict`:** optional exit **4** when notifier delivery fails for a threshold event (cron/CI gate); default unchanged.
- **Anonymous collector:** opt-in daemon telemetry (`enable_collector` / `PGWD_ENABLE_COLLECTOR`) → **POST** `https://collect.gghstats.com/a1b2c3d4e5f6a7b8`; opt-out update check (`enable_update_check`) → **GET** GitHub releases API — see README [Anonymous usage](#anonymous-usage).
- **`make bench`:** `go test -bench=. ./internal/...` target; CI **bench** job is non-blocking (not in **`make release-check`**).

### Changed

- **Deprecation runway:** stronger stderr warnings for legacy **`db:`** and ignored **`-notify-on-connect-failure`** (always-on when notifiers exist; removal in v1.0).
- **Notifier TLS warning:** startup stderr when Slack/Loki/Teams/generic webhook URLs use `http://` (non-loopback).
- **Docker runtime:** **`Dockerfile`** and **`Dockerfile.release`** use **`gcr.io/distroless/static-debian13:nonroot`** instead of Alpine 3.24.1 (same pattern as [groot](https://github.com/hrodrig/groot) / [kzero](https://github.com/hrodrig/kzero)). Static binary + bundled CA certs; no BusyBox/apk OS packages. Entrypoint path **`/home/pgwd/pgwd`** unchanged for Compose/Helm compatibility.

### Documentation

- **[docs/use-cases.md](docs/use-cases.md)** — operator scenario matrix (single/multi DB, in/out of Kubernetes, different credentials per target).
- **[docs/kubernetes-passwords.md](docs/kubernetes-passwords.md)** — DISCOVER migration guide; multi-DB credential patterns.
- **[SPECIFICATIONS.md](SPECIFICATIONS.md)** — baseline v0.9.0; collector, TLS warning, CSV sanitization, kube password alternatives.

## [0.8.0] - 2026-07-11

### Added

- **`make cover-check`** — statement coverage gate (default **≥ 80%** on library packages; `internal/cli` excluded, exercised via `cmd/pgwd` black-box tests). Part of **`make release-check`** and CI.
- **Supply chain:** GoReleaser **Syft** SBOMs (SPDX + CycloneDX), **Cosign** keyless signing for `checksums.txt` and `ghcr.io/hrodrig/pgwd` images; release workflow installs cosign/syft and verifies image signature post-publish. See README [Supply chain verification](README.md#supply-chain-verification) and SPEC §11.

### Changed

- **Docs:** [SPECIFICATIONS.md](SPECIFICATIONS.md) — known limitations (v0.7.0), HTTP operator security, corrected `/healthz` and CSV columns, notifier TLS note. [ROADMAP.md](ROADMAP.md) and [docs/plan-0.9.x.md](docs/plan-0.9.x.md) — 0.9.x security/doc checklist from external audit review.

## [0.7.0] - 2026-07-03

### Added

- **PagerDuty Events v2** notifier (`notifications.pagerduty`, `-notifications-pagerduty-*`, `PGWD_NOTIFICATIONS_PAGERDUTY_*`). Severity mapping from pgwd levels; `custom_details` with connection stats and context.
- **Microsoft Teams** incoming webhook (`notifications.teams`, `-notifications-teams-*`).
- **Generic webhook** with custom headers (JWT bearer), optional HMAC-SHA256 signing, and Go `body_template` for custom JSON payloads.
- **Shared HTTP retry/backoff** for all notifiers (`notifications.retry`, `-notifications-retry-*`). Slack and Loki migrated to shared retry. Defaults: 3 attempts, 1s initial backoff, 10s max backoff; retry on 5xx and network errors only.

### Documentation

- **[ROADMAP.md](ROADMAP.md)** — canonical release index (0.7 → 1.0, calendar, document map, key decisions).
- **[SPECIFICATIONS.md](SPECIFICATIONS.md)** — §4/§6 updated for 0.7.x notifiers and retry; config load order; client-go kube; **`DISCOVER_MY_PASSWORD` deprecated** (removed 0.9.x).
- **[docs/kubernetes-passwords.md](docs/kubernetes-passwords.md)** — decision record for deprecating `DISCOVER_MY_PASSWORD`.
- **`contrib/pgwd.conf.example`**, **README**, **man page** — PagerDuty, Teams, generic webhook, and retry settings.

### Deprecated (behavior unchanged until 0.9.x)

- **`DISCOVER_MY_PASSWORD`** in `-db-url` / config — requires `pods/exec` RBAC. Decision record: [docs/kubernetes-passwords.md](docs/kubernetes-passwords.md). Removal: [plan-0.9.x.md](docs/plan-0.9.x.md) §10.

## [0.6.10] - 2026-06-17

### Security

- **Docker runtime `alpine:3.24.1`** in **`Dockerfile`** and **`Dockerfile.release`** (from 3.22; clears Snyk low findings; OpenSSL fixes incl. **CVE-2026-2673**). Grype may still report **Medium** busybox **CVE-2025-60876** until Alpine ships a fixed **apk**; runtime removes **`wget`** (affected applet).

## [0.6.9] - 2026-06-16

### Added

- **`pgwd --print-sample-config`** — writes annotated sample config to stdout (same as `contrib/pgwd.conf.example`); documented in README.

### Fixed

- **Docker image build:** include **`contrib/`** in build context for embedded sample config.

## [0.6.8] - 2026-06-10

### Security

- **Go 1.26.4** — stdlib fixes **GO-2026-5037** (`crypto/x509`, CVE-2026-27145), **GO-2026-5039** (`net/textproto`), and related **1.26.4** patches. Rebuild binaries and images; relevant for HTTPS notifiers (Slack/Loki) and TLS Postgres URLs.

### Changed

- **Go 1.26.4** — `go.mod`, **Dockerfile** build stage (`golang:1.26.4-alpine`), README Go badge.

### Documentation

- **Roadmap:** **0.8.0** supply chain (Syft SBOM + Cosign); drop dedicated **0.7.0** Timescale feature — use **`metrics_store.driver: postgres`** for TimescaleDB. **`contrib/pgwd.conf.example`** note.

## [0.6.7] - 2026-05-31

### Fixed

- **too_many_clients detection:** Map Postgres SQLSTATE **53300** via **`pgconn.PgError`** so saturation alerts work regardless of server **`lc_messages`** locale (not English error text).

### Changed

- **Daemon log:** After a successful notification, **`Notification sent:`** logs a full one-line summary (context prefix, short threshold message, `total`/`active`/`idle`, `max_connections`, and `(limit …, %, level)` for 3-tier alerts). Loki and Slack payloads are unchanged.

### Documentation

- **`cmd/pgwd/main.go`:** Go doc for orchestration and helper functions.

## [0.6.6] - 2026-05-25

### Security

- **`golang.org/x/net` v0.55.0** — fixes **GO-2026-5026** (Punycode / `idna.ToASCII` on HTTP URLs), reached from **`notify.Loki.Send`**. CI **govulncheck** and published **v0.6.5** images/binaries should upgrade to this release.

### Fixed

- **GoReleaser (Homebrew cask):** Removed broken `empty` template and optional `skip_upload`; release **requires** `HOMEBREW_TAP_TOKEN` and always publishes `Casks/pgwd.rb` to **`hrodrig/homebrew-pgwd`**. Workflow fails early if the secret is unset.

### Documentation

- **README:** Links to **[pgwd-selfhosted](https://github.com/hrodrig/pgwd-selfhosted)** (Helm/Compose) and **[gghstats](https://github.com/hrodrig/gghstats)** from the intro.

## [0.6.5] - 2026-05-16

### Security

- **Go 1.26.3** and **`golang.org/x/net` v0.53.0+** — fixes **govulncheck** findings affecting code paths that use **`net`**, **`net/http`**, and HTTP/2 (**GO-2026-4971**, **GO-2026-4918**), including **`notify.Loki.Send`**. Upgrade binaries and container images built before this release.
- **Docker runtime `alpine:3.22`** in **`Dockerfile`** and **`Dockerfile.release`** (not 3.23 — avoids OpenSSL 3.5.x / **CVE-2026-2673**).

### Added

- **`make security`** — **govulncheck** + **`docker-scan`** (Grype on the built image), matching the CI **Security** workflow.
- **README:** gghstats clones badge, **License** and **Star History** sections (with table of contents entries).

### Changed

- **Homebrew release:** GoReleaser publishes the cask to **`hrodrig/homebrew-pgwd`** (requires **`HOMEBREW_TAP_TOKEN`** in CI or local env).

### Fixed

- **Kubernetes (`-kube-postgres` with `svc/`):** Resolve the backend pod via **`discovery.k8s.io/v1` EndpointSlice** instead of deprecated **`core/v1` Endpoints** (Kubernetes 1.33+ warnings). Falls back to Service selector + pod list when slices are missing.
- **GoReleaser (`dockers_v2`):** **`sbom: false`** (GHA buildx attestation failure), removed **`annotations:`** (index annotation export failure), and **QEMU/buildx/GHCR login** in **`.github/workflows/release.yml`** for multi-arch **`linux/amd64,linux/arm64`**.

## [0.6.4] - 2026-05-03

### Added

- **Metrics store (PostgreSQL / MySQL):** Optional **`metrics_store.driver`** + **`metrics_store.dsn`** persist check history like SQLite (FIFO cap **`sqlite.max_metrics`** applies to all backends). Implementation in **`internal/store/sqlstore.go`** (pgx stdlib + **`github.com/go-sql-driver/mysql`**). Daemon, **`/metrics`**, hysteresis, and CSV export use **`internal/metricsstore`** and the **`store.MetricsStorer`** interface.
- **Long-running query alerts:** Optional **`db.long_query_min_seconds`** (active queries with `now() - query_start` exceeding N seconds), **`db.long_query_cooldown_seconds`** (minimum interval between notifications per target; default 3600 when min is set), and **`db.long_query_min_count`**. Requires a **metrics store** (SQLite or SQL) to persist cooldown timestamps. Hysteresis does not gate `long_query` repeats; cooldown suppresses spam while the same condition persists.

### Changed

- **`Makefile` / CI lint:** **`make lint`** and the lint job run **gofmt -s**, **`go vet ./...`**, and **gocyclo** (complexity ≤ 14). **`make help`** prints current **`VERSION`** / ldflags version / branch; **`release-check`** requires **`VERSION`** semver; **`docker-scan`** Grype on PATH or **`anchore/grype`** container (`GRYPE_FAIL_ON`, default `high`); **`make cover`** and **`make tools`**.
- **`make cover-integration`:** Postgres + Loki via compose, then **`go test ./...`** with coverage → **`coverage-integration.out`**. Compose up/down refactored into **`integration-compose-up`** / **`integration-compose-down`** (shared with **`test-integration`**).
- **OpenBSD port:** Removed **`contrib/openbsd/port/distinfo`** from the repo (checksums require published tarballs). Generate **`distinfo`** with **`make makesum`** in the ports tree after **`make fetch`**; **`contrib/openbsd/port/distinfo`** is **`.gitignore`**-d. **`contrib/openbsd/port/README.md`** documents this and optional operator copies in **pgwd-selfhosted**.
- **`make snapshot`:** Does not require Docker. GoReleaser **`--snapshot`** skips **`dockers_v2`** (see **`.goreleaser.yaml`**). **`make release`** / **`release-check`** still use Docker where applicable.
- **`make snapshot` versioning:** Snapshot version now comes from **`VERSION`** (`<VERSION>-next`) instead of reachable git tags, so `develop` snapshots remain predictable without merging `main`.
- **Snapshot artifact names:** GoReleaser archive/package filenames now use snapshot version (`v<version>-next`) during **`--snapshot`** instead of the last reachable git tag, keeping `dist/` names aligned with `metadata.json`.
- **`make dist-freebsd`:** New helper target builds the FreeBSD port distfile tarball only (name/layout expected by `contrib/freebsd/Makefile`) using `VERSION`, avoiding the full multi-platform snapshot build when validating the port locally.
- **`make dist-openbsd`:** New helper target builds the OpenBSD port distfile tarball only (name/layout expected by `contrib/openbsd/port/Makefile`) using `VERSION` and `OPENBSD_ARCH` (default `amd64`).
- **Platform tests package source selection:** `testing/platforms` now supports explicit `pgwd_package_source` mode (`auto`, `release`, `local`) so runs can force published artifacts or local files without editing role logic.
- **Root build on FreeBSD:** Real rules live in **`GNUmakefile`** (GNU Make). Root **`Makefile`** is a **BSD Make** stub that forwards to **`gmake`** (`pkg install gmake`); uses **`all`** + **`.DEFAULT`** with **`gmake $@`** (not **`$(.MAKE.CMDGOALS)`**, often empty when **`.DEFAULT`** runs). **`.gitignore`** whitelists **`GNUmakefile`**.
- **`pgwd -version` / `--version`:** Output now includes the **git branch** (from build-time ldflags), e.g. `pgwd v0.6.4 (branch develop, commit …, built …)`. Same **branch** field appears in the startup log banner. **Makefile**, **Dockerfile**, and **GoReleaser** inject **`main.Branch`**; plain `go build` without flags shows **`branch unknown`**.

### Documentation

- **Multi-database:** Document limitations — **`databases:`** cannot be combined with **`-kube-postgres`**; SQLite hysteresis/history is keyed by **`(client, cluster, database)`** (not URL host), so use a **unique `client` per entry** when the same DB name appears on different hosts (README, `AGENTS.md`, `contrib/pgwd.conf.example`).
- **Upgrade guide:** **`docs/UPGRADE-0.5-to-0.6.md`** — operator checklist from **0.5.10** (or earlier **0.5.x**) to **0.6.x**, with links to README breaking-changes table, CHANGELOG, Helm move, and optional metrics/HTTP features. Linked from **`docs/README.md`** and the README breaking-changes section.
- **FreeBSD (build from source):** **`README.md`** and **`AGENTS.md`** — install **`gmake`** and **`go`** on **`PATH`** (`pkg install gmake`, `pkg install go`; **`go`** usually under **`/usr/local/bin`**).

### Security

- **Dependencies (indirect):** `github.com/moby/spdystream` **v0.5.0 → v0.5.1** (GHSA-pc3f-x583-g7j2); `filippo.io/edwards25519` **v1.1.0 → v1.1.1** (GHSA-fw7p-63qq-7hpr). Unblocks **`make docker-scan`** at default **`GRYPE_FAIL_ON=high`** (Alpine **busybox** CVE-2025-60876 may still report **Medium** until upstream **apk** packages catch up).

### Fixed

- **`testing/scripts/test-e2e-kube.sh`:** **`kind` cluster leak** — `trap` was replaced by **`trap kill_pf EXIT`** and later **`trap - EXIT`**, so **`kind delete cluster`** never ran on success and the next **`make test-e2e-kube` / `release-check`** failed with “node(s) already exist”. Single **`EXIT`** trap now stops port-forwards and always deletes the cluster; **`cleanup_kind`** also runs once before **`kind create cluster`** for idempotent runs.

## [0.6.0] - 2026-04-26

### Added

- **CSV export (metrics store):** **`-export-metrics-format csv`** + **`-export-metrics-destination`** / **`PGWD_EXPORT_METRICS_*`** (env when no config file) reads persisted metrics through **`internal/metricsstore`** (SQLite file via **`sqlite.path`**; **PostgreSQL/MySQL** metrics persistence is **not implemented** — **`metrics_store.driver`** / **`metrics_store.dsn`** reserved, **`ErrSQLMetricsStoreNotImplemented`**). Writes an **RFC4180** CSV (`id`, `ts_ms`, `ts_utc`, labels, counts, state, threshold), then **exits**. SQLite uses a **read-only** open (safe alongside the daemon). **`internal/metricsexport`** formats the sink (CSV now).

### Security

- **Dependencies:** `github.com/jackc/pgx/v5` updated to **v5.9.2** (memory-safety advisory, Dependabot #4 — fixed from **v5.9.0**; GHSA-j88v-2chj-qfwx / CVE-2026-41889 placeholder confusion with dollar-quoted literals, Dependabot #5 — requires **v5.9.2**).

### Removed

- **Helm:** In-repo chart under `contrib/helm/pgwd/` and OCI chart push from this repository’s release workflow / `make release-helm`. The maintained chart is **[pgwd-selfhosted](https://github.com/hrodrig/pgwd-selfhosted)** (`run/kubernetes/helm/pgwd/`). See **`contrib/HELM.md`**.

### Added (highlights since 0.5.10)

- **Makefile:** **`docker-buildx-amd64`** / **`docker-buildx-amd64-push`** — **linux/amd64** image build and push (e.g. from macOS ARM).
- **Daemon mode** with **resolution notifications**, **SQLite** metrics store, **HTTP** `/healthz` and Prometheus **`/metrics`**, **hysteresis** (`confirm_alert` / `confirm_ok`), **`log_level`** (info/debug).
- **Multi-database** config (`databases:`), **`-db-url`** one-shot override, **client-go** kube port-forward (optional **`-kube-postgres`** / **`-kube-loki`** without shelling to kubectl for the data path).
- **Packages / layout:** `internal/checker`, `internal/validator`, `internal/httpsrv`, `internal/store`; expanded tests and **e2e kube** (multi-Postgres compose).
- **Ansible** **`testing/platforms/`** — automated install/daemon/notify/timer/uninstall across Linux and BSD (Phase 1); **NetBSD** and **DragonFly** vars; **OpenBSD** port scaffolding and **FreeBSD** port release docs.

### Changed

- **Config file** layout and **CLI/env** names for DB thresholds and notifications (see README breaking-changes table). **`client`** required.
- **CI / release:** workflow and image metadata updates; Helm chart publishing removed from app repo (see Removed).

## [0.5.10] - 2026-03-24

### Added

- **README:** **AlmaLinux** section — `dnf install` from URL or local `.rpm`, config, `systemctl`, pointer to systemd docs.
- **README:** **Arch Linux** section — tarball install, systemd units from `contrib/systemd/`, AUR note, static IP `/24` and `network.target` pointers.
- **README:** Install table names AlmaLinux, Rocky Linux, and Oracle Linux alongside Fedora/RHEL for the same `.rpm`/`dnf` one-liner; notes `make snapshot` when a GitHub tag is not published yet; documents AlmaLinux + systemd validation (dry-run and `pgwd.service`).
- **FreeBSD port:** `contrib/freebsd/` with Makefile, pkg-plist, pkg-descr, and rc.d script. Install from local port or (when accepted) official ports. See `contrib/freebsd/README.md`.
- **FreeBSD rc.d:** Daemon with `daemon(8)` for logging to `/var/log/pgwd.log`. Custom stop/status using pidfile (supervisor pid). rc.conf variables: `pgwd_enable`, `pgwd_flags`, `pgwd_config`, `pgwd_env`, `pgwd_logfile`. Supports kube-postgres and kube-loki (external VPS with kubeconfig).
- **NetBSD rc.d:** `contrib/netbsd/rc.d/pgwd` script. Tarball `pgwd_v*_netbsd_amd64.tar.gz` includes rc.d script and config example. rc.conf: `pgwd=YES`, `pgwd_flags`, `pgwd_env`. See `contrib/netbsd/README.md`.
- **README:** FreeBSD section (port, tarball, config, daemon, cron). Main README badge and FreeBSD tarball URLs updated.
- **Config file** (YAML): Load settings from `/etc/pgwd/pgwd.conf` (or `-config` / `PGWD_CONFIG`). Keys match `-flag` and `PGWD_*` env vars. Example: `contrib/pgwd.conf.example`.
- **.deb and .rpm packages:** Install `/etc/pgwd/pgwd.conf` from the example (type `config|noreplace` — not overwritten on upgrade if user modified). Edit before use. Also install systemd units to `/lib/systemd/system/` (pgwd.service, pgwd-once.service, pgwd.timer) — enable with `systemctl enable --now pgwd`. Debian/Ubuntu: prerm stops and disables services before removal; postrm removes `/etc/pgwd` on `apt purge`.

### Changed

- **systemd:** `pgwd.service` and `pgwd-once.service` now order after **`network.target`** instead of **`network-online.target`**, so `systemctl enable --now` does not block on `systemd-networkd-wait-online` (notably on static-IP / minimal installs). README troubleshooting and `contrib/systemd/README.md` document the symptom and a drop-in override.
- **contrib/freebsd:** Config example installed to `${PREFIX}/etc/pgwd/` (was `/etc/pgwd/`). Reinstall: `make deinstall`, `make clean`, `make install` to pick up port file changes.
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

## [0.5.8] - 2026-03-22

### Changed

- **illumos / Solaris:** Install commands use `cp` + `chmod` (illumos `install` differs from GNU). FMRI `svc:/application/pgwd:default`; added troubleshooting for wrong FMRI, `svcs -v`, log path via `svcs -L`. Emphasized illumos as primary path. Version 0.5.8: VERSION, README badge, contrib READMEs, FreeBSD port, man page.

---

## [0.5.7] - 2026-02-23

### Added

- **DragonFly BSD rc.d:** `contrib/dragonflybsd/rc.d/pgwd` script. Tarball `pgwd_v*_dragonfly_amd64.tar.gz` includes rc.d script and config example. rc.conf: `pgwd_enable="YES"`, `pgwd_flags`, `pgwd_env`. Uses `daemon(8)` like FreeBSD. See `contrib/dragonflybsd/README.md`.
- **Solaris and Linux riscv64:** GoReleaser builds for `solaris/amd64` and `linux/riscv64`. Makefile: `build-solaris`, `build-linux` (riscv64). Ignore rules for unsupported GOOS/GOARCH combos.
- **Man page FILES section:** Platform-specific setup references (contrib/freebsd, netbsd, dragonflybsd, openbsd READMEs).

### Changed

- **AGENTS.md:** Commit message review rule — show proposed message and wait for approval before `git commit`.
- **Version 0.5.7:** VERSION, README badge, contrib READMEs, FreeBSD port.

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

[Unreleased]: https://github.com/hrodrig/pgwd/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/hrodrig/pgwd/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/hrodrig/pgwd/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/hrodrig/pgwd/compare/v0.9.0...v1.0.0
[0.9.0]: https://github.com/hrodrig/pgwd/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/hrodrig/pgwd/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/hrodrig/pgwd/compare/v0.6.10...v0.7.0
[0.6.10]: https://github.com/hrodrig/pgwd/compare/v0.6.9...v0.6.10
[0.6.9]: https://github.com/hrodrig/pgwd/compare/v0.6.8...v0.6.9
[0.6.8]: https://github.com/hrodrig/pgwd/compare/v0.6.7...v0.6.8
[0.6.7]: https://github.com/hrodrig/pgwd/compare/v0.6.6...v0.6.7
[0.6.6]: https://github.com/hrodrig/pgwd/compare/v0.6.5...v0.6.6
[0.6.5]: https://github.com/hrodrig/pgwd/compare/v0.6.4...v0.6.5
[0.6.4]: https://github.com/hrodrig/pgwd/compare/v0.6.0...v0.6.4
[0.6.0]: https://github.com/hrodrig/pgwd/compare/v0.5.10...v0.6.0
[0.5.10]: https://github.com/hrodrig/pgwd/compare/v0.5.8...v0.5.10
[0.5.8]: https://github.com/hrodrig/pgwd/compare/v0.5.7...v0.5.8
[0.5.7]: https://github.com/hrodrig/pgwd/compare/v0.5.4...v0.5.7
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
