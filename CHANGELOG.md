# Changelog

All notable changes to this project are documented in this file.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). History below is derived from the project plan (release scope).

## [Unreleased]

### Added

- **docs:** Sequence diagrams audit ([docs/sequence/AUDIT.md](docs/sequence/AUDIT.md)) mapping each diagram step to code; README and docs/README link to it.
- **Cursor rule:** `.cursor/rules/diagrams-mermaid.mdc` — validate Mermaid rendering when adding/editing diagrams; avoid backticks, semicolons, and colons inside message text; keep diagrams in sync with code (see AUDIT.md).
- **tools/:** Scripts and docs for scanning before merge/release: `tools/scan.sh` (govulncheck + optional Grype on dir), [tools/README.md](tools/README.md) (install Grype, scan image with Grype, realistic results, do not upgrade zlib/base packages in Alpine). CI: `.github/workflows/security.yml` (govulncheck + Grype on built image, `--fail-on high,critical`). Release rule and AGENTS updated to run scan before release.

### Changed

- **docs:** Mermaid diagram fixes so all sequence diagrams render correctly (semicolon/colon in message text; 01, 02, 05, 07).
- **docs:** Diagram 04 (dry-run) — log line now mentions `max_connections` when available.
- **docs:** Diagram 07 (connect-failure) — log step shows fixed message (no error detail); run context in parentheses.

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

[Unreleased]: https://github.com/hrodrig/pgwd/compare/v0.2.3...HEAD
[0.2.3]: https://github.com/hrodrig/pgwd/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/hrodrig/pgwd/compare/v0.2.0...v0.2.2
[0.2.1]: https://github.com/hrodrig/pgwd/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/hrodrig/pgwd/compare/v0.1.7...v0.2.0
[0.1.8]: https://github.com/hrodrig/pgwd/compare/v0.1.7...v0.1.8
[0.1.7]: https://github.com/hrodrig/pgwd/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/hrodrig/pgwd/releases/tag/v0.1.6
