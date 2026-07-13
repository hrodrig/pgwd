# pgwd — Documentation

## Roadmap

**Canonical index:** **[ROADMAP.md](../ROADMAP.md)** at repo root.

**Brand:** [`logo.svg`](./logo.svg) / [`logo.png`](./logo.png) — hybrid **`pg`** + live status dot (landing favicon uses the same mark).

Release bands from **v0.7.0** to **v1.0.0**. Behavior contract: [SPECIFICATIONS.md](../SPECIFICATIONS.md). Release notes: [CHANGELOG.md](../CHANGELOG.md).

| Band | Plan | Theme |
|------|------|--------|
| **0.7.x** | [plan-0.7.x.md](./plan-0.7.x.md) | ✅ v0.7.0 — PagerDuty, Teams, generic webhook, HTTP retry |
| **0.8.x** | [plan-0.8.x.md](./plan-0.8.x.md) | ✅ v0.8.0 — Syft SBOM, Cosign signing |
| **0.9.x** | [plan-0.9.x.md](./plan-0.9.x.md) | ✅ v0.9.0 — DISCOVER removal, profiles, collector, operator docs |
| **1.0.x** | [plan-1.0.x.md](./plan-1.0.x.md) | Breaking API, deprecations removed, **compare / positioning** |
| **1.0.x** | [plan-1.0.x.md](./plan-1.0.x.md) | Breaking stable API, deprecations removed |

## Upgrading

**[Upgrading from 0.5.x to 0.6.x](./UPGRADE-0.5-to-0.6.md)** — Checklist and links for moving from **0.5.10** (or earlier **0.5.x**) to **0.6.x**: config/CLI/env breaks introduced in **0.5.10**, optional **0.6.x** features (metrics store, HTTP, CSV export), and **Helm** moving to [pgwd-selfhosted](https://github.com/hrodrig/pgwd-selfhosted). For the raw rename table, see the [README breaking-changes section](../README.md#breaking-changes-upgrade-from-05x).

**[Operator use cases](./use-cases.md)** — Scenario matrix: single/multi database, in/out of Kubernetes, cron vs daemon, different credentials per target. Profiles under `contrib/profiles/`.

**[Kubernetes passwords — migration from DISCOVER_MY_PASSWORD](./kubernetes-passwords.md)** — Single-DB outside-cluster credentials (`password_from_secret`, wrapper, RBAC). Multi-DB: [use-cases.md](./use-cases.md).

## Loki and Grafana alerts

[Loki payload structure for Grafana alerts](./loki-grafana-alerts.md) — Labels, log line format, example LogQL queries, and how to build Grafana alert rules that react to pgwd notifications (attention, alert, danger).

[Testing alert levels without changing production](./testing-alert-levels.md) — Procedure to trigger attention, alert, and danger notifications using `-test-max-connections` against a real Postgres instance (e.g. production) without modifying its config or waiting for real load.

## Sequence diagrams

Sequence diagrams for main use cases (Mermaid format). View in any Markdown viewer that supports Mermaid (e.g. GitHub, VS Code with Mermaid extension, or [Mermaid Live](https://mermaid.live)).

| Diagram | Description |
|---------|-------------|
| [01-startup-validation](./sequence/01-startup-validation.md) | Startup: config load, validation, Postgres connect, default thresholds, sender setup |
| [02-one-shot-threshold-exceeded](./sequence/02-one-shot-threshold-exceeded.md) | One-shot run: stats fetched, threshold exceeded, notifications sent, exit |
| [03-one-shot-no-alert](./sequence/03-one-shot-no-alert.md) | One-shot run: stats below thresholds, no events, exit |
| [04-dry-run](./sequence/04-dry-run.md) | Dry-run: stats logged only, no HTTP calls to Slack/Loki, exit |
| [05-force-notification](./sequence/05-force-notification.md) | Force notification: one test event sent to all notifiers, exit |
| [06-daemon-loop](./sequence/06-daemon-loop.md) | Daemon mode: ticker loop, run on each tick and on SIGTERM/SIGINT exit |
| [07-connect-failure-notification](./sequence/07-connect-failure-notification.md) | Connection failed: connect_failure (or too_many_clients) event sent to notifiers when at least one notifier is configured, then exit |

Diagrams are audited against the code; see [AUDIT.md](./sequence/AUDIT.md) for the mapping and when to re-audit.

## Terminal demo (VHS)

A [VHS](https://github.com/charmbracelet/vhs) tape records a short terminal demo of pgwd (help, version, optional dry-run).

### Prerequisites

- [VHS](https://github.com/charmbracelet/vhs): `brew install vhs` (or see project install docs)
- pgwd binary on `PATH`: run `make install` from repo root (installs to `$GOBIN`, default `$HOME/go/bin`)

### Render the demo

From the **repository root**, run **both** lines below. **Do not** run `vhs docs/demo.tape` alone from an interactive **zsh** session (with Oh My Zsh or custom prompts): VHS inherits that shell and the recording can break (`git_prompt_info: command not found`, garbled prompt, wrong `PATH`).

```bash
make install
bash -c "vhs docs/demo.tape"
```

Wrapping VHS in **`bash -c "..."`** starts a clean bash subshell for the recorder, matching `Set Shell "bash"` inside the tape. This is the same command required when **`VERSION` changes** before release (see `.cursor/rules/release-tests.mdc`).

Output is written to `docs/demo.gif` (or the path set by `Output` in the tape). To produce MP4 instead, change the `Output` line in `demo.tape` to e.g. `Output docs/demo.mp4` and run the same two commands again.

### Tape location

- Tape file: **`docs/demo.tape`**
- Rendered GIF (default): **`docs/demo.gif`**

### Prompt / Oh My Zsh issues

Symptoms: `git_prompt_info: command not found`, broken prompt in the GIF, or wrong pgwd binary. Fix: use the exact **two-command** flow in [Render the demo](#render-the-demo) (`make install` then `bash -c "vhs docs/demo.tape"`); do not invoke `vhs` directly from an interactive zsh.
