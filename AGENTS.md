# AGENTS.md

Context and instructions for AI coding agents working on **pgwd** (Postgres Watch Dog). See [agents.md](https://agents.md/) for the format.

## Project overview

- **What it is:** Go CLI that monitors PostgreSQL connection counts (total, active, idle, stale) and notifies via Slack and/or Loki when configured thresholds are exceeded.
- **Entrypoint:** `cmd/pgwd/main.go`. Packages: `internal/config`, `internal/postgres`, `internal/notify` (Slack, Loki).
- **Config:** CLI flags and env vars (`PGWD_*`). No config file yet. CLI overrides env.
- **Kubernetes:** Optional `-kube-postgres namespace/svc/name` (or `pod/name`) runs `kubectl port-forward` and connects to localhost; URL password `DISCOVER_MY_PASSWORD` reads password from pod env. Requires `kubectl` in PATH (pgwd checks at startup and exits with a clear error if missing). See `internal/kube`.
- **Connect failure:** `-notify-on-connect-failure` or `-force-notification` send an event to notifiers when Postgres connection fails (infrastructure alert or validation). Senders are built before connecting so the alert can be sent on failure.

## Setup and build

- Install deps: `go mod download` (or `go build` will pull them).
- Build binary: `make build` (reads `VERSION` file, injects Version/Commit/BuildDate via ldflags).
- Install to `$GOBIN`: `make install`. Custom path: `GOBIN=/usr/local/bin make install`.
- Cross-compile: `make build-linux`, `make build-darwin`, `make build-windows`, or `make build-all` (output in `dist/`).

## Test commands

- Run all tests: `make test` or `go test ./...`
- Tests exist in `internal/config` and `internal/notify`. No tests in `cmd/pgwd` or `internal/postgres` (optional future: testcontainers or mocks).
- Before committing or proposing changes, ensure `go test ./...` passes.

## Code style and conventions

- **Language:** English only. Code, comments, commit messages, docs, and variable/function names must be in English (see `.cursor/rules/language-english.mdc`).
- **Go:** Standard Go style. Use `gofmt`/`goimports` if available. Module path: `github.com/hrodrig/pgwd`.
- **Version:** Canonical version lives in the `VERSION` file (e.g. `0.2.0`). Makefile and Docker build use it; keep README badges and `go.mod` in sync when versions change (see `.cursor/rules/readme-badges-version.mdc`).

## Git flow

- **Branches:** Work on `develop`. `main` is production and is only updated from `develop` at release time (see `.cursor/rules/git-flow.mdc`).
- **Releases:** Before releasing: ensure **all tests pass** (`make test` or `go test ./...`). Then merge `develop` → `main`, and on `main`: create annotated tag (e.g. `git tag -a v0.2.0 -m "Release 0.2.0"`), push tag, run `make release` (requires goreleaser). Do not commit features directly to `main`. See `.cursor/rules/release-tests.mdc`.
- **Versioning:** Semantic versioning (MAJOR.MINOR.PATCH) for tags.

## Docker

- Build image with version info: `make docker-build` (passes VERSION, COMMIT, BUILDDATE; without it the binary reports `dev`/`unknown`).
- Build context is whitelisted via `.dockerignore`: only `go.mod`, `go.sum`, `cmd/`, and `internal/` are sent.
- Dockerfile: multi-stage (Go 1.26, Alpine 3.23), non-root user `pgwd`, minimal runtime (ca-certificates only; wget/nc removed).

## Repository structure

- `cmd/pgwd/` — main package.
- `internal/config/` — config from env and CLI.
- `internal/postgres/` — pool, stats, stale count, max_connections.
- `internal/notify/` — Slack and Loki senders, event type.
- `internal/kube/` — Kubernetes port-forward, pod resolution, password discovery; `RequireKubectl()` at startup when `-kube-postgres` is set.
- `docs/` — sequence diagrams (Mermaid), VHS demo tape.
- `contrib/systemd/` — systemd units (daemon, timer, one-shot).
- `tools/` — scripts for scanning before merging to main: `tools/scan.sh` (govulncheck, optional Grype). See `tools/README.md`. CI runs govulncheck in the Security workflow.

## Other instructions

- **README:** Must keep badges (Release, Go version, License) and explicit link to Releases; see `.cursor/rules/readme-badges-version.mdc`.
- **CHANGELOG:** Update `CHANGELOG.md` when adding notable user-facing changes (under `[Unreleased]`) and when preparing a release (move items into the new version section; align with PLAN release scope). See `.cursor/rules/changelog.mdc`.
- When adding dependencies, run `go mod tidy` and ensure tests still pass.
