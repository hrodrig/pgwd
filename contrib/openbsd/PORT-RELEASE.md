# OpenBSD port — release, validation, and submission

How to bump the **pgwd** port for a new application release, validate it on an OpenBSD VM, and send a diff to **ports@openbsd.org**.

**New to BSD ports?** Read **[BSD-PORTS-STEP-BY-STEP.md](../BSD-PORTS-STEP-BY-STEP.md)** first (ordered overview), then use this document for OpenBSD lab detail.

| Doc | Audience |
|-----|----------|
| **[README.md](README.md)** | Install from release tarball or repo `contrib/openbsd/pgwd` |
| **[port/README.md](port/README.md)** | Port directory layout, ports tree checkout, `file://` fetch |
| **[DEBUG-VM.md](DEBUG-VM.md)** | rc.d troubleshooting without the ports framework |
| **[testing/platforms/README.md](../../testing/platforms/README.md)** | Ansible full-cycle on bare-metal VMs |

---

## Big picture (read this first)

Three layers — easy to mix up:

| Layer | What it is | Where it lives |
|-------|------------|----------------|
| **Application** | `pgwd` binary + man + config example + optional `share/openbsd/rc.d/pgwd` in tarball | **pgwd** repo; GoReleaser publishes **`pgwd_v<ver>_openbsd_<arch>.tar.gz`** |
| **Port skeleton** | Recipe to turn that tarball into an OpenBSD **package** | **`contrib/openbsd/port/`** (copy into **`/usr/ports/sysutils/pgwd/`**) |
| **Ports tree** | Full OpenBSD build system (`bsd.port.mk`, distfiles, packages) | **`/usr/ports`** on the VM |

**Ansible** (`make test-platforms`) uses **tarball + repo `contrib/openbsd/pgwd`**. That path does **not** require `/usr/ports`.

**Port validation** checks that **`make package`** / **`make install`** register **`/etc/rc.d/pgwd`** via **`pkg/pgwd.rc`** and **`@rcscript`** in **PLIST** (same pattern as **gitea** / **gghstats**).

---

## Prerequisites

| Where | What |
|-------|------|
| **Build host** (macOS or Linux) | **GNU make** (`brew install make` → **`gmake`**), **Go**, **pgwd** repo clone |
| **OpenBSD VM** | Disk for **`/usr/ports`** (git shallow clone is enough for one port), **root** SSH |

---

## Part A — In the pgwd repo (before or after GitHub release)

### A1. Bump application version

From the repo root (on **`develop`**, then release per project git flow):

1. Update **`VERSION`**, **`CHANGELOG.md`**, README badges, **`contrib/man/man1/pgwd.1`** (see root **`AGENTS.md`**).
2. Run **`make release-check`**, merge to **`main`**, tag **`v<VERSION>`**, push tag (GitHub **Release** workflow / GoReleaser publishes assets).

### A2. Sync the port skeleton to `VERSION`

```bash
cd /path/to/pgwd
gmake port-openbsd-sync
# or: make port-openbsd-sync   # if GNU make is your default `make`
```

This updates **`contrib/openbsd/port/Makefile`** (`DISTNAME`, `PKGNAME`, `MASTER_SITES`, `DISTFILES`) and copies:

- **`contrib/openbsd/pgwd`** → **`contrib/openbsd/port/pkg/pgwd.rc`**
- **`contrib/openbsd/pgwd`** → **`contrib/openbsd/port/files/pgwd`**

**`COMMENT`** in the port **Makefile** must stay **≤ 60 characters**.

### A3. Build a local distfile (optional, before tag exists on GitHub)

Same layout and filename as GoReleaser:

```bash
gmake dist-openbsd
# gmake dist-openbsd OPENBSD_ARCH=arm64
# gmake dist-openbsd OPENBSD_ARCH=riscv64
```

Output: **`dist/pgwd_v<VERSION>_openbsd_<arch>.tar.gz`** ( **`v`** prefix in the filename — matches **`.goreleaser.yaml`** and **`DISTFILES`**).

### A4. Stage port + distfile for the VM

Copy **only** the port directory (not all of **`contrib/openbsd/`**):

```bash
rm -rf /tmp/pgwd-port
cp -r contrib/openbsd/port /tmp/pgwd-port
scp -r /tmp/pgwd-port/* root@openbsd-test:/tmp/pgwd-port/
scp dist/pgwd_v*_openbsd_*.tar.gz root@openbsd-test:/tmp/
```

---

## Part B — Lab validation on OpenBSD (full ports tree)

Use **BSD `make`** inside **`/usr/ports/sysutils/pgwd`**, not **`gmake`** from the app repo.

### B0. Read versions from the port (do not hardcode in scripts)

```sh
cd /usr/ports/sysutils/pgwd
ver=$(make -V PKGNAME | sed 's/^pgwd-//')    # e.g. 0.6.7
dist=$(make -V DISTFILES)                     # e.g. pgwd_v0.6.7_openbsd_amd64.tar.gz
echo "PKGNAME=$ver DISTFILES=$dist"
```

On the **build host**, the tarball is **`pgwd_v<VERSION>_openbsd_<arch>.tar.gz`**.

**Port layout (must match repo after `port-openbsd-sync`):**

| File | Role |
|------|------|
| **`pkg/pgwd.rc`** | Source for **`generate-readmes`** → **`/etc/rc.d/pgwd`** (only **`*.rc`** under **`pkg/`** are copied) |
| **`pkg/PLIST`** | First line: **`@rcscript ${RCDIR}/pgwd`** — do **not** list **`etc/rc.d/pgwd`** manually |
| **`files/pgwd`** | Synced copy of rc.d (used by **`test-install-from-dist.sh`**; official install uses **`pkg/pgwd.rc`**) |

### B1. Ports tree (once per VM)

```sh
pkg_add git
cd /usr
rm -rf ports
git clone --depth 1 https://github.com/openbsd/ports.git ports
```

CVS checkout for a specific release branch: see **[port/README.md](port/README.md)**.

### B2. Teardown (when redoing the lab test)

```sh
if [ -f /usr/ports/sysutils/pgwd/Makefile ]; then
  ver=$(make -C /usr/ports/sysutils/pgwd -V PKGNAME | sed 's/^pgwd-//')
fi
: "${ver:?set ver= to match repo VERSION}"

pkg_delete pgwd-${ver} 2>/dev/null
rm -f /etc/rc.d/pgwd
rm -rf /usr/ports/sysutils/pgwd
rm -f /usr/ports/plist/amd64/pgwd-${ver}
rm -f /usr/ports/packages/amd64/all/pgwd-*.tgz \
      /usr/ports/packages/amd64/no-arch/pgwd-*.tgz \
      /usr/ports/packages/amd64/ftp/pgwd-*.tgz
```

### B3. Install port directory

On the VM (after **A4**):

```sh
mkdir -p /usr/ports/sysutils/pgwd
cp -r /tmp/pgwd-port/* /usr/ports/sysutils/pgwd/

ls /usr/ports/sysutils/pgwd/
# Makefile  README.md  files/  pkg/  test-install-from-dist.sh

ls /usr/ports/sysutils/pgwd/pkg/
# DESCR  PLIST  pgwd.rc

grep rcscript /usr/ports/sysutils/pgwd/pkg/PLIST
# @rcscript ${RCDIR}/pgwd
```

### B4. Distfile and checksums

```sh
export PORTSDIR=/usr/ports
export DISTDIR=/usr/ports/distfiles
mkdir -p "$DISTDIR"
cd /usr/ports/sysutils/pgwd
dist=$(make -V DISTFILES)
cp /tmp/$dist "$DISTDIR/"

make makesum
```

**`distinfo`** is generated here — include it in the **ports-tree diff** you email. It is **not** committed in the **pgwd** repo (see **`port/README.md`**).

To fetch from GitHub instead of copying locally:

```sh
make fetch
make makesum
```

### B5. Build package and install

```sh
cd /usr/ports/sysutils/pgwd
make clean=package clean
make package FETCH_PACKAGES=No
```

**Must appear in the log:**

```text
Installing /usr/ports/sysutils/pgwd/pkg/pgwd.rc as .../fake-amd64/etc/rc.d/pgwd
===>  Building package for pgwd-<VERSION>
```

If you only see **`Link to ...`** and no **Building package**, the **`.tgz` was not rebuilt** — repeat **`make clean=package clean`**.

If **`Error: change in plist`**, remove the cached plist (lab VM):

```sh
ver=$(make -V PKGNAME | sed 's/^pgwd-//')
rm -f /usr/ports/plist/amd64/pgwd-${ver}
make package FETCH_PACKAGES=No
```

Verify the package contains the rc script:

```sh
PKG=$(ls /usr/ports/packages/amd64/no-arch/pgwd-*.tgz 2>/dev/null \
   || ls /usr/ports/packages/amd64/all/pgwd-*.tgz)
tar -tzf "$PKG" | grep etc/rc.d/pgwd
```

Install:

```sh
make install
```

**Success:**

```text
pgwd-<VERSION>: ok
The following new rcscripts were installed: /etc/rc.d/pgwd
```

```sh
ls -l /etc/rc.d/pgwd
rcctl enable pgwd
```

### B6. Configure and run

```sh
mkdir -p /etc/pgwd
cp /usr/local/share/examples/pgwd/pgwd.conf.example /etc/pgwd/pgwd.conf
vi /etc/pgwd/pgwd.conf    # db.url, client, notifications, etc.

rcctl start pgwd
rcctl check pgwd          # expect pgwd(ok)

pgwd -config /etc/pgwd/pgwd.conf -dry-run -interval 0
tail -n 40 /var/log/daemon
```

Optional boot: add **`pgwd`** to **`pkg_scripts`** in **`/etc/rc.conf.local`**.

**Kube on a VPS:** set **`pgwd_env="KUBECONFIG=..."`** in **`rc.conf.local`** — see **[README.md](README.md)**.

### B7. Ansible regression (optional)

From the **pgwd** repo on your laptop (inventory with **`pgwd-openbsd`** host):

```bash
make test-platforms-ping PLATFORM=pgwd-openbsd
make test-platforms PLATFORM=pgwd-openbsd
```

Set **`pgwd_version`** in **`testing/platforms/inventory/hosts.yml`** to match the release (e.g. **`0.6.7`**) or use **`pgwd_local_package`** pointing at **`dist/pgwd_v...tar.gz`**.

### B8. Port vs tarball (rc.d location)

| Install method | rc.d on disk | `rcctl` |
|----------------|--------------|---------|
| **Port / pkg_add** | **`/etc/rc.d/pgwd`** via **`pkg/pgwd.rc`** + **`@rcscript`** | **`rcctl enable pgwd`** |
| **Tarball / Ansible** | **`/etc/rc.d/pgwd`** (manual **`install`** or from **`share/openbsd/rc.d/pgwd`**) | Same |

### B9. Lab shortcut (no full ports tree)

```sh
doas sh /tmp/pgwd-port/test-install-from-dist.sh /tmp/pgwd_v0.6.7_openbsd_amd64.tar.gz /tmp/pgwd-port
```

Useful for debugging **do-install** layout; **not** a substitute for **B5** before submitting the port.

---

## Part C — Submit the port to ports@openbsd.org

The port is **not** in the official tree until accepted. When ready:

### C1. Prepare the ports-tree diff

1. On a machine with a full **`/usr/ports`** checkout, copy refreshed files:

   ```sh
   rm -rf /usr/ports/sysutils/pgwd
   mkdir -p /usr/ports/sysutils/pgwd
   cp -r /path/to/pgwd/contrib/openbsd/port/* /usr/ports/sysutils/pgwd/
   ```

2. Use the **published** GitHub release tarball (same bytes as **`make makesum`** used):

   ```sh
   cd /usr/ports/sysutils/pgwd
   make fetch
   make makesum
   make clean=package clean
   make package FETCH_PACKAGES=No
   make install
   ```

3. Re-run **B5** on a **clean** VM if possible.

### C2. Generate the diff

From **`/usr/ports`** (tool depends on how you track ports):

```sh
cd /usr/ports
# CVS checkout:
cvs diff -u sysutils/pgwd > /tmp/pgwd-port.diff

# Git ports mirror:
git diff sysutils/pgwd > /tmp/pgwd-port.diff
```

The diff must include **`sysutils/pgwd/`** (Makefile, pkg/, files/, **distinfo**, DESCR if new).

### C3. Email ports@openbsd.org

- **To:** ports@openbsd.org  
- **Subject:** `sysutils/pgwd: new port — Postgres connection watchdog` (or `update pgwd to <version>` for updates)  
- **Body:** Short description, upstream URL, how you tested (**OpenBSD version**, **`make package`**, **`rcctl`**, **`pgwd -dry-run`**).  
- **Attach:** the diff (or inline if small).

Follow the [OpenBSD Porting Guide](https://www.openbsd.org/faq/ports/guide.html) and [porting, testing, and submitting](https://www.openbsd.org/faq/ports/testing.html).

### C4. After first acceptance — version updates

1. **`gmake port-openbsd-sync`** in **pgwd** after each **`VERSION`** bump.  
2. Copy **`contrib/openbsd/port/*`** → **`/usr/ports/sysutils/pgwd/`**.  
3. New distfile in **`$DISTDIR`**, **`make makesum`**, **`make package`**, **`make install`**, send **update** diff.

---

## Distfile naming (after `port-openbsd-sync`)

| Field | Pattern |
|-------|---------|
| **DISTFILES** | `pgwd_v<VERSION>_openbsd_<arch>.tar.gz` |
| **MASTER_SITES** | `https://github.com/hrodrig/pgwd/releases/download/v<VERSION>/` |
| **PKGNAME** | `pgwd-<VERSION>` |

**`v`** is in the tarball filename (GoReleaser **`name_template`** uses the Git tag).

---

## Common pitfalls (lab notes)

| Symptom | Cause | Fix |
|---------|--------|-----|
| `Could not find bsd.port.mk` | No **`/usr/ports`** tree | **B1** |
| `cp: ${PORTSDIR}/distfiles/` | **`DISTDIR`** unset | **`export DISTDIR=/usr/ports/distfiles`** and **`mkdir -p`** |
| `comment is too long` | **COMMENT** > 60 chars | Shorten port **Makefile** |
| `rcctl: service pgwd does not exist` | Stale package or **`.tgz` not rebuilt** | **B2** teardown; **B5** with *Installing pkg/pgwd.rc* in log |
| **`make package`** only *Link to …* | Cached **`.tgz`** | **`make clean=package clean`**, **`make package FETCH_PACKAGES=No`** |
| **`Error: change in plist`** | Old **`/usr/ports/plist/amd64/pgwd-*`** | **`rm`** plist entry; rebuild |
| rc.d under **`/usr/local/etc/rc.d`** | Wrong **PLIST** / missing **`@rcscript`** | Use **`@rcscript ${RCDIR}/pgwd`**; remove manual **`etc/rc.d`** from **do-install** |
| **`make fetch`** 404 | Tag not published or **`MASTER_SITES`** out of sync | **`gmake port-openbsd-sync`** after release; confirm asset on GitHub |

---

## What to validate before sending the diff

- [ ] **`gmake port-openbsd-sync`** run after **`VERSION`** bump  
- [ ] **`gmake dist-openbsd`** tarball matches **DISTFILES** name and installs manually  
- [ ] **`make makesum`** on VM with **release** tarball bytes  
- [ ] **`make package FETCH_PACKAGES=No`** logs *Installing pkg/pgwd.rc*  
- [ ] **`make install`** → **`/etc/rc.d/pgwd`**, **`rcctl enable pgwd`**  
- [ ] **`pgwd -dry-run`** with example config succeeds  
- [ ] Optional: **`make test-platforms PLATFORM=pgwd-openbsd`**  
- [ ] **distinfo** included in ports diff, **not** in pgwd git  

---

## Related

- **FreeBSD port:** **[../freebsd/PORT-RELEASE.md](../freebsd/PORT-RELEASE.md)**  
- **Cross-BSD overview:** **[../BSD-PORTS-STEP-BY-STEP.md](../BSD-PORTS-STEP-BY-STEP.md)**
