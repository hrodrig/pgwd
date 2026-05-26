# FreeBSD Port: Release and Update Procedure

Steps to create or update the pgwd port for a new release.

## Prerequisites

- FreeBSD machine or VM with ports tree
- `pkg install sharutils` (provides `gshar`)

## Supported architectures

**`ONLY_FOR_ARCHS`** in the port **`Makefile`** must match assets on the GitHub release for that **`PORTVERSION`**:

| FreeBSD `ARCH` | Release tarball suffix | In release today (v0.6.6) |
|----------------|------------------------|---------------------------|
| amd64 | `_freebsd_amd64` | yes |
| aarch64 | `_freebsd_arm64` | yes |
| riscv64 | `_freebsd_riscv64` | no — omit from **`ONLY_FOR_ARCHS`** until published and tested |

QA for Bugzilla: document the FreeBSD version and arch you tested (e.g. FreeBSD 15 amd64). arm64 is supported when the matching tarball exists; riscv64 is out of scope until the table row is **yes**.

## Part A: In the pgwd repo (before release)

1. **Bump version** in `contrib/freebsd/Makefile`:
   ```bash
   # Edit PORTVERSION=	0.5.10  →  new version
   ```

2. **Commit and release** pgwd (tag, `make release`). The GitHub release must include **`pgwd_vX.Y.Z_freebsd_amd64.tar.gz`** and **`pgwd_vX.Y.Z_freebsd_arm64.tar.gz`** (see **Supported architectures**). Goreleaser does not publish **freebsd/riscv64** today.

3. **Copy port files** to your FreeBSD ports tree:
   ```bash
   # On FreeBSD
   cp -r /path/to/pgwd/contrib/freebsd/* ~/ports/sysutils/pgwd/
   ```

## Part B: On FreeBSD (ports tree)

### First-time submission (port not yet in official tree)

1. **Create/checkout branch:**
   ```bash
   cd ~/ports
   git checkout -b add-pgwd-port   # or update-pgwd-0.5.11 for updates
   ```

2. **Update files** (if not already copied):
   ```bash
   cp -r /path/to/pgwd/contrib/freebsd/* sysutils/pgwd/
   ```

3. **Generate distinfo:**
   ```bash
   cd sysutils/pgwd
   make makesum
   ```

4. **Test install:**
   ```bash
   make install
   pkg info pgwd
   pgwd -version
   ```

5. **Clean work directory** (required — shar must not include work/stage):
   ```bash
   make clean
   ```

6. **Create shar file** (output outside the ports tree):
   ```bash
   cd ~/ports
   gshar $(find sysutils/pgwd -type f | sort) > ~/pgwd.shar
   ```
   Note: FreeBSD uses `gshar` (not `shar`) from the sharutils package. Output to `~/` or `/tmp/` so the shar does not sit in the ports repo root.

7. **Commit:**
   ```bash
   git add sysutils/pgwd/
   git commit -m "New port: sysutils/pgwd - Postgres Watch Dog"   # or "Update sysutils/pgwd to 0.5.11"
   ```

8. **Submit to Bugzilla:**
   - URL: https://bugs.freebsd.org/submit/
   - Product: Ports & Packages
   - Component: Individual Port(s)
   - Summary: `New port: sysutils/pgwd - Postgres Watch Dog, monitor connection counts`
   - Description: Short blurb + "Tested on FreeBSD 15 amd64"
   - Attachment: `~/pgwd.shar` (or path where you saved it)

### Update (port already in official tree)

1. Update port files from contrib/freebsd/
2. `cd sysutils/pgwd && make makesum`
3. `make install` and verify
4. Generate diff: `cd ~/ports && git diff main..add-pgwd-port > pgwd-update.diff`
5. Submit diff to Bugzilla or Phabricator as an update to the existing port

## Quick reference: version bump in Makefile

| Variable | Example |
|----------|---------|
| PORTVERSION | 0.6.6 |
| ONLY_FOR_ARCHS | amd64 aarch64 |
| DISTFILES | pgwd_v${PORTVERSION}_freebsd_${ARCH:S/aarch64/arm64/}.tar.gz |
| MASTER_SITES | https://github.com/hrodrig/pgwd/releases/download/v${PORTVERSION}/ |

## Git config (one-time on FreeBSD)

If `git commit` fails with "Author identity unknown":

```bash
git config --global user.email "hrodrig@usb.ve"
git config --global user.name "Hermes Jesús Rodríguez Azuaje"
```
