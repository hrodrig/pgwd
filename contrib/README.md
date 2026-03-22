# contrib — platform-specific files for pgwd

This directory contains platform-specific installation and configuration files. Each subdirectory has its own README with setup details.

## Contents

| Directory / File | Purpose |
|------------------|---------|
| [deb/](deb/) | Debian/Ubuntu packaging scripts (prerm.sh, postrm.sh) for .deb packages |
| [dragonflybsd/](dragonflybsd/README.md) | [DragonFly BSD](https://www.dragonflybsd.org) rc.d script and install docs |
| [freebsd/](freebsd/README.md) | [FreeBSD](https://www.freebsd.org) port (Makefile, pkg-plist) and rc.d script |
| [solaris/](solaris/README.md) | [illumos](https://illumos.org) / [Oracle Solaris](https://www.oracle.com/solaris) SMF manifest and method script |
| [man/man1/](man/man1/) | Man page (`man pgwd`) — included in .deb, .rpm, and tarballs |
| [netbsd/](netbsd/README.md) | [NetBSD](https://www.netbsd.org) rc.d script and install docs |
| [openbsd/](openbsd/README.md) | [OpenBSD](https://www.openbsd.org) rc.d script and install docs |
| [openrc/](openrc/README.md) | [Alpine Linux](https://alpinelinux.org) OpenRC init script |
| [systemd/](systemd/README.md) | systemd units (daemon, timer, one-shot) for Linux |
| [pgwd.conf.example](pgwd.conf.example) | Example YAML config — copy to `/etc/pgwd/pgwd.conf` and edit |

## Quick reference

- **Linux (systemd):** [contrib/systemd/README.md](systemd/README.md)
- **Alpine (OpenRC):** [contrib/openrc/README.md](openrc/README.md)
- **FreeBSD:** [contrib/freebsd/README.md](freebsd/README.md)
- **NetBSD:** [contrib/netbsd/README.md](netbsd/README.md)
- **OpenBSD:** [contrib/openbsd/README.md](openbsd/README.md)
- **DragonFly BSD:** [contrib/dragonflybsd/README.md](dragonflybsd/README.md)
- **illumos / Solaris:** [contrib/solaris/README.md](solaris/README.md)

See the [main README](../README.md) for install commands and platform-specific sections.
