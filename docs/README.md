# pgwd — Documentation

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
| [07-connect-failure-notification](./sequence/07-connect-failure-notification.md) | Connection failed: connect_failure event sent to notifiers (notify-on-connect-failure or force-notification), then exit |

Diagrams are audited against the code; see [AUDIT.md](./sequence/AUDIT.md) for the mapping and when to re-audit.

## Terminal demo (VHS)

A [VHS](https://github.com/charmbracelet/vhs) tape records a short terminal demo of pgwd (help, version, optional dry-run).

### Prerequisites

- [VHS](https://github.com/charmbracelet/vhs): `brew install vhs` (or see project install docs)
- pgwd binary on `PATH` (e.g. `make build` then `export PATH="$PWD:$PATH"` from repo root)

### Render the demo

From the **repository root** (so `pgwd` resolves and paths match):

```bash
vhs docs/demo.tape
```

Output is written to `docs/demo.gif` (or the path set by `Output` in the tape). To produce MP4 instead, change the `Output` line in `demo.tape` to e.g. `Output docs/demo.mp4` and run again.

### Tape location

- Tape file: **`docs/demo.tape`**
- Rendered GIF (default): **`docs/demo.gif`**

### Prompt / Oh My Zsh issues

If you see `git_prompt_info: command not found` or a broken prompt in the GIF, run VHS from **bash** so it does not inherit your zsh/Oh My Zsh setup:

```bash
bash
vhs docs/demo.tape
```
