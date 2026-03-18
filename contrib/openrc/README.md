# OpenRC init script for pgwd (Alpine Linux)

Alpine uses OpenRC, not systemd. This init script runs pgwd as a daemon.

## Install (manual, from tarball)

```bash
# Copy init script
sudo cp contrib/openrc/pgwd.initd /etc/init.d/pgwd
sudo chmod +x /etc/init.d/pgwd

# Config (required)
sudo mkdir -p /etc/pgwd
sudo cp contrib/pgwd.conf.example /etc/pgwd/pgwd.conf
sudo nano /etc/pgwd/pgwd.conf  # edit client, db.url, etc.

# Enable and start
rc-service pgwd start
rc-update add pgwd default
```

## When installed via `apk add pgwd`

The Alpine package (aports) installs this script to `/etc/init.d/pgwd` and config to `/etc/pgwd/pgwd.conf`. Edit config, then:

```bash
rc-service pgwd start
rc-update add pgwd default
```

## Commands

| Command | Action |
|---------|--------|
| `rc-service pgwd start` | Start daemon |
| `rc-service pgwd stop` | Stop daemon |
| `rc-service pgwd status` | Check status |
| `rc-update add pgwd default` | Enable on boot |
| `rc-update del pgwd` | Disable on boot |
