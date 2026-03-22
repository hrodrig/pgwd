# systemd units for pgwd

Daemon and timer units for running pgwd under systemd. **No EnvironmentFile** — config comes from `/etc/pgwd/pgwd.conf` (or `-config` for a custom path).

**One config = one Postgres.** For multiple instances (different clusters, thresholds), use one timer per config, or run from cron with one entry per config file. See README "Running from cron" and "Example: multiple services".

## Prerequisites

1. **Config file** — pgwd requires a config file. **.deb/.rpm** install it to `/etc/pgwd/pgwd.conf`. From source:

   ```bash
   sudo mkdir -p /etc/pgwd
   sudo cp contrib/pgwd.conf.example /etc/pgwd/pgwd.conf
   # Edit: set client, db.url, notifications (Slack/Loki), interval, etc.
   sudo chmod 600 /etc/pgwd/pgwd.conf  # if it contains secrets
   ```

2. **Binary path** — Units use `/usr/bin/pgwd` (where .deb/.rpm install). For manual install to `/usr/local/bin`, edit `ExecStart` in the unit(s).

## Files

| Unit | Function | When to use |
|------|----------|-------------|
| `pgwd.service` | Daemon — runs continuously, checks every `interval` seconds from config | Continuous monitoring (e.g. every 60 s) |
| `pgwd-once.service` | One-shot — runs pgwd once and exits. Used by the timer | Do not enable directly |
| `pgwd.timer` | Schedule — triggers pgwd-once every 5 minutes (1 min after boot) | Cron-like: one check every 5 min |

**.deb/.rpm** install these units to `/lib/systemd/system/`. Skip the `cp` steps below; just enable and start.

Units order after **`network.target`** (not `network-online.target`). That avoids `systemctl enable --now pgwd.service` **appearing to hang** while systemd waits for `systemd-networkd-wait-online` (common on minimal or static-IP installs, e.g. Arch). pgwd still starts after basic network setup; if Postgres is remote, ensure routing/DNS work before relying on the first check.

## Quick test (before enabling)

```bash
# One check, print stats, exit — validates config and DB connectivity
pgwd -dry-run -interval 0
```

## Daemon mode

```bash
# .deb/.rpm: units already in /lib/systemd/system/. From source:
# sudo cp contrib/systemd/pgwd.service /etc/systemd/system/
# If manual install: edit ExecStart=/usr/local/bin/pgwd
sudo systemctl daemon-reload
sudo systemctl enable --now pgwd
journalctl -u pgwd -f
```

## Timer (one-shot every 5 min)

```bash
# .deb/.rpm: units already in /lib/systemd/system/. From source:
# sudo cp contrib/systemd/pgwd-once.service contrib/systemd/pgwd.timer /etc/systemd/system/
# If manual install: edit ExecStart in pgwd-once.service
sudo systemctl daemon-reload
sudo systemctl enable --now pgwd.timer
systemctl list-timers --all | grep pgwd
```

Set `interval: 0` in config (or omit) when using the timer — pgwd runs once per tick.

## Optional: dedicated user

```bash
sudo useradd -r -s /bin/false pgwd
```

Then in the unit, uncomment and set:

```conf
User=pgwd
Group=pgwd
```

Ensure the pgwd user can read `/etc/pgwd/pgwd.conf`.

## Troubleshooting

### `systemctl enable --now` seems stuck; service stays `inactive (dead)`; `journalctl -u pgwd.service` empty

Often caused by **`network-online.target`**: an older unit or a copy may still use `Wants=network-online.target`, which waits on **wait-online** services. If you interrupt with Ctrl+C, systemd can show a stale **Job** id.

**Fix (current upstream units):** reinstall or copy `contrib/systemd/pgwd.service` and `pgwd-once.service` from this repo (they use `network.target`). Then:

```bash
sudo systemctl daemon-reload
sudo systemctl reset-failed pgwd.service
sudo systemctl start pgwd.service
sudo systemctl status pgwd.service
```

**Override without replacing the whole unit** (drop-in):

```bash
sudo mkdir -p /etc/systemd/system/pgwd.service.d
sudo tee /etc/systemd/system/pgwd.service.d/override.conf << 'EOF'
[Unit]
After=network.target
Wants=network.target
EOF
sudo systemctl daemon-reload
sudo systemctl reset-failed pgwd.service
sudo systemctl start pgwd.service
```

If `systemctl cancel <jobid>` says the job does not exist, **`reset-failed`** and **`start`** are usually enough.

### No logs from pgwd until the service actually runs

`journalctl -u pgwd.service` stays empty until systemd has **started** the process at least once. Confirm **`ExecStart`** exists (`/usr/bin/pgwd`) if you installed the binary under `/usr/local/bin` (symlink or edit the unit).
