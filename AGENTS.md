# AGENTS.md

Context and instructions for AI coding agents working on **pgwd** (Postgres Watch Dog). See [agents.md](https://agents.md/) for the format.

## Project overview

- **What it is:** Go CLI that monitors PostgreSQL connection counts (total, active, idle, stale), optional **long-running query** alerts (`db.long_query_*`, cooldown via metrics store), and notifies via Slack and/or Loki when configured thresholds are exceeded.
- **Entrypoint:** `cmd/pgwd/main.go`. Packages: `internal/config`, `internal/postgres`, `internal/notify` (Slack, Loki).
- **Config:** Config file (YAML) at `/etc/pgwd/pgwd.conf` or `-config` / `PGWD_CONFIG`. Use `databases:` for one or more Postgres (canonical). Legacy `db:` deprecated (emits warning, removed in v1.0). **`kube.postgres` / `-kube-postgres` is not supported with `databases:`** (multi-DB requires direct URLs; single-DB + kube until per-db kube exists). **SQLite / hysteresis** rows are keyed by **`(client, cluster, database)`** — not by URL host; use a **unique `client` per `databases:` entry** when the same DB name is used on different hosts. When file loads, env vars ignored; otherwise `ApplyDefaults` + `ApplyEnv`. CLI flags override. See `internal/config`, `contrib/pgwd.conf.example`, README “Multi-database limitations”.
- **Kubernetes:** Optional `-kube-postgres namespace/svc/name` (or `pod/name`) runs `kubectl port-forward` and connects to localhost; URL password `DISCOVER_MY_PASSWORD` reads password from pod env. Optional `-kube-loki namespace/svc/loki` runs port-forward to Loki when Loki is inside the cluster and pgwd runs outside. Requires `kubectl` in PATH (pgwd checks at startup and exits with a clear error if missing). See `internal/kube`. **Helm / in-cluster deployment manifests** are in **[pgwd-selfhosted](https://github.com/hrodrig/pgwd-selfhosted)**; see `contrib/HELM.md` and `contrib/k8s/README.md`.
- **Metrics persistence:** Daemon writes check history to **`sqlite.path`** (SQLite) or **`metrics_store.driver`** + **`metrics_store.dsn`** (PostgreSQL / MySQL via **`internal/store/sqlstore`**) for hysteresis, resolution alerts, and `/metrics`. **`internal/metricsstore`** selects the backend for export; **`store.MetricsStorer`** is the interface used by **`cmd/pgwd`** and **`internal/httpsrv`**. **CSV export:** `-export-metrics-format csv` + `-export-metrics-destination` → **`internal/metricsexport`** + **`metricsstore.ExportRows`** — see README.
- **Connect failure:** When Postgres connection fails, pgwd always sends a `connect_failure` (or `too_many_clients` if the error is "too many clients already") event to all notifiers if any are configured and not `-dry-run`. No extra flag is required. Senders are built before connecting so the alert can be sent on failure.

## Setup and build

- Install deps: `go mod download` (or `go build` will pull them).
- Build binary: **`make build`** (reads **`VERSION`**, injects Version/Commit/BuildDate/Branch via ldflags). The repo root **`GNUmakefile`** holds the real rules (**GNU Make**). On **FreeBSD**, the root **`Makefile`** is a small **BSD Make** stub that forwards to **`gmake`** — install **`devel/gmake`** (`pkg install gmake`) and **`lang/go`** (`pkg install go`) so **`go`** is on **`PATH`** (typically **`/usr/local/bin`**).
- Install to `$GOBIN`: `make install`. Custom path: `GOBIN=/usr/local/bin make install`.
- Cross-compile: `make build-linux`, `make build-darwin`, `make build-windows`, or `make build-all` (output in `dist/`).

## Test commands

- Run all tests: `make test` or `go test ./...`. Optional: **`make cover`** (unit tests only → `coverage.out`), **`make cover-integration`** (Docker Postgres + Loki → `coverage-integration.out`; same stack as **`make test-integration`**), **`make tools`** (install `govulncheck` and `gocyclo` to `$GOBIN`).
- **Integration tests:** `make test-integration` (requires Docker). Starts Postgres and Loki via `testing/compose.yaml` and `testing/compose-loki.yaml`, runs integration tests, then stops. Must pass before release (see `.cursor/rules/release-tests.mdc`).
- Tests exist in `internal/config`, `internal/notify` (unit + Loki integration), `internal/checker`, `internal/validator`, `internal/store`, `internal/httpsrv`, `internal/kube`, and `internal/postgres` (integration, requires `PGWD_TEST_DB_URL`). `cmd/pgwd` has black-box tests (version, help, validation exits).
- **Platform tests:** `make test-platforms` (requires Ansible + VMs). Ansible playbooks under `testing/platforms/` automate install, daemon, notification (Loki+Slack mock), timer, and uninstall validation across Linux and BSD. See `testing/platforms/README.md`. Target one platform: `make test-platforms PLATFORM=pgwd-ubuntu`. Quick connectivity check: `make test-platforms-ping` runs `playbooks/ping.yml` using Ansible's **`ping`** module (not ICMP); a healthy host responds with **`pong`**.
- **Lint:** `make lint` runs **gofmt -s**, **`go vet ./...`**, and **gocyclo** (complexity ≤ 14); the CI **lint** job does the same.
- Before committing or proposing changes, ensure `go test ./...` passes. Before release, also run `make test-integration`.
- **Before a release:** run `make test-platforms` (or at minimum the platforms affected by the change) to validate install, daemon, notifications, and uninstall on real OS targets. This is not automated in CI (requires VMs) but is a manual pre-release gate.

## Code style and conventions

- **Language:** English only. Code, comments, commit messages, docs, and variable/function names must be in English (see `.cursor/rules/language-english.mdc`).
- **Go:** Standard Go style. Use `gofmt`/`goimports` if available. Module path: `github.com/hrodrig/pgwd`.
- **Version:** Canonical version lives in the `VERSION` file (e.g. `0.2.0`). Makefile and Docker build use it; keep README badges and `go.mod` in sync when versions change (see `.cursor/rules/readme-badges-version.mdc`).

## Git flow

- **Branches:** Work on `develop`. `main` is production and is only updated from `develop` at release time (see `.cursor/rules/git-flow.mdc`).
- **Commits:** Always show the proposed commit message and wait for user approval before running `git commit`. See `.cursor/rules/commit-message-review.mdc`.
- **Releases:** Before releasing: run **`make release-check`** (validates **`VERSION`** semver, then lint, test, test-integration, test-e2e-kube, **`make docker-scan`**). All must pass — they are MANDATORY. **`docker-scan`** uses Grype on PATH if present, otherwise the **anchore/grype** Docker image (`GRYPE_FAIL_ON` defaults to `high`). Then merge `develop` → `main`, and on `main`: create annotated tag (e.g. `git tag -a v0.2.0 -m "Release 0.2.0"`), push tag, run `make release` (requires goreleaser). `make release` runs `release-check` first. Do not commit features directly to `main`. See `.cursor/rules/release-tests.mdc`.
- **Versioning:** Semantic versioning (MAJOR.MINOR.PATCH) for tags.

## Docker

- Build image with version info: `make docker-build` (passes VERSION, COMMIT, BUILDDATE; without it the binary reports `dev`/`unknown`). For **linux/amd64** only (e.g. push to a private registry from another arch): `make docker-buildx-amd64` (`pgwd:amd64` locally) or `make docker-buildx-amd64-push DOCKER_IMAGE=registry/repo:tag` after `docker login`.
- Build context is whitelisted via `.dockerignore`: only `go.mod`, `go.sum`, `cmd/`, and `internal/` are sent.
- Dockerfile: multi-stage (Go 1.26.3 build; **Alpine 3.22** runtime — not 3.23, OpenSSL/CVE-2026-2673), non-root user `pgwd`, minimal runtime (ca-certificates only; wget/nc removed).

## Repository structure

- `cmd/pgwd/` — main package. Black-box tests in `main_test.go` (version, help, validation).
- `internal/checker/` — pure logic for thresholds, levels, event collection, state derivation (extracted from main for testability).
- `internal/validator/` — config validation returning errors (extracted from main for testability).
- `internal/config/` — config from file (YAML), env (`PGWD_*`), and CLI. `file.go`: FromFile, ApplyDefaults.
- `internal/postgres/` — pool, stats, stale count, max_connections.
- `internal/notify/` — Slack and Loki senders, event type.
- `internal/kube/` — Kubernetes port-forward, pod resolution, password discovery; `RequireKubectl()` at startup when `-kube-postgres` is set.
- `docs/` — sequence diagrams (Mermaid), VHS demo tape (`docs/demo.tape` → `docs/demo.gif`). Regenerate after **`VERSION` changes:** `make install && bash -c "vhs docs/demo.tape"` from repo root (use `bash -c` so zsh/Oh My Zsh does not break recording — see `docs/README.md`).
- `contrib/systemd/` — systemd units (daemon, timer, one-shot).
- `contrib/HELM.md` — pointer to the Helm chart in **pgwd-selfhosted** (this repo does not ship the chart).
- `contrib/k8s/README.md` — Kubernetes deployment notes (raw manifests); Helm lives in pgwd-selfhosted.
- `testing/platforms/` — Ansible roles and playbooks for multi-platform install/test/uninstall validation (Linux, BSD). See `testing/platforms/README.md`.
- `tools/` — **`make security`** (govulncheck + `docker-scan` / Grype on image; mirrors CI Security). Also `tools/scan.sh` (govulncheck only). See `tools/README.md`.

## Skills

- **golang-pro** (`.agents/skills/golang-pro/SKILL.md`): Use when implementing concurrent Go patterns (goroutines, channels), designing interfaces, writing table-driven tests, or optimizing performance. Covers generics, context propagation, error wrapping, and idiomatic Go. References: `references/concurrency.md`, `references/interfaces.md`, `references/generics.md`, `references/testing.md`, `references/project-structure.md`.

## Other instructions

- **README:** Must keep badges (Release, Go version, License) and explicit link to Releases; see `.cursor/rules/readme-badges-version.mdc`.
- **CHANGELOG:** Update `CHANGELOG.md` when adding notable user-facing changes (under `[Unreleased]`) and when preparing a release (move items into the new version section; align with the plan release scope). See `.cursor/rules/changelog.mdc`.
- When adding dependencies, run `go mod tidy` and ensure tests still pass.
