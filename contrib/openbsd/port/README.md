# OpenBSD port for pgwd

Port files for submitting **pgwd** to the official OpenBSD ports tree.

**Operator install from tarball:** **[../README.md](../README.md)**.

**Maintainers (full flow):**

| Doc | Content |
|-----|---------|
| **[../BSD-PORTS-STEP-BY-STEP.md](../BSD-PORTS-STEP-BY-STEP.md)** | Ordered overview (FreeBSD + OpenBSD) |
| **[../PORT-RELEASE.md](../PORT-RELEASE.md)** | Generate distfile, VM validation, **ports@** diff |
| **[../DEBUG-VM.md](../DEBUG-VM.md)** | rc.d troubleshooting on a test host |
| **This file** | Ports tree checkout, `file://` fetch, directory layout |

## Version bump

From the **pgwd** repo root, after updating **`VERSION`**:

```bash
gmake port-openbsd-sync
```

Refreshes **`DISTNAME`**, **`PKGNAME`**, **`MASTER_SITES`**, **`DISTFILES`** in this **Makefile**, and copies **`contrib/openbsd/pgwd`** → **`pkg/pgwd.rc`** and **`files/pgwd`**.

## distinfo (not shipped in the pgwd repo)

**`distinfo`** holds **`SHA256`** / **`SIZE`** for **DISTFILES**. Generate it on the VM after **`make fetch`** (or a local tarball):

```sh
cd /usr/ports/sysutils/pgwd
make makesum
```

Include **`distinfo`** in the diff to **ports@openbsd.org**. **`contrib/openbsd/port/distinfo`** is **gitignored** in the pgwd repo.

## Port files vs release tarball

| Source | Installed by port |
|--------|-------------------|
| **DISTFILES** (GitHub / **`dist-openbsd`**) | `bin/pgwd`, man, LICENSE, `pgwd.conf.example` |
| **`pkg/pgwd.rc`** + **`@rcscript ${RCDIR}/pgwd`** in **PLIST** | **`/etc/rc.d/pgwd`** via **generate-readmes** |

Keep **`pkg/pgwd.rc`** in sync with **`contrib/openbsd/pgwd`** (**`gmake port-openbsd-sync`**). Do **not** use **`pkg/pgwd`** (no **`.rc`** suffix). Do **not** list **`etc/rc.d/pgwd`** in **PLIST** or install it in **do-install**.

After **`make install`**: copy **`share/examples/pgwd/pgwd.conf.example`** → **`/etc/pgwd/pgwd.conf`**, edit, then **`rcctl enable pgwd`** and **`rcctl start pgwd`**.

## Install the full ports tree (test VM)

`make` in **`contrib/openbsd/port/`** alone fails with *Could not find bsd.port.mk* until **`/usr/ports/infrastructure/`** exists.

**1. Save your port copy** (if you already created **`/usr/ports/sysutils/pgwd`**):

```sh
cp -r /usr/ports/sysutils/pgwd /tmp/pgwd-port
```

**2. Check OpenBSD version:**

```sh
uname -r
# e.g. 7.6 → use OPENBSD_7_6 for CVS (see below)
```

**3a. Official CVS:**

```sh
pkg_add -u cvs
export CVSROOT=anoncvs@anoncvs.openbsd.org:/cvs
cd /usr
rm -rf ports
cvs -qd checkout -r OPENBSD_7_6 -P ports
```

Use **`-r OPENBSD_7_6`** with a **space** after **`-r`**. For **-current**: **`cvs -qd checkout -A -P ports`**.

**3b. Git mirror (lab VM):**

```sh
pkg_add git
cd /usr
rm -rf ports
git clone --depth 1 https://github.com/openbsd/ports.git ports
```

**4. Restore pgwd port and build:**

```sh
rm -rf /usr/ports/sysutils/pgwd
mkdir -p /usr/ports/sysutils/pgwd
cp -r /tmp/pgwd-port/* /usr/ports/sysutils/pgwd/
export DISTDIR=/usr/ports/distfiles
mkdir -p "$DISTDIR"
cd /usr/ports/sysutils/pgwd
dist=$(make -V DISTFILES)
cp /tmp/$dist "$DISTDIR/"
make makesum
make clean=package clean
make package FETCH_PACKAGES=No
make install
```

Full validated flow and pitfalls: **[../PORT-RELEASE.md](../PORT-RELEASE.md)** Part B.

**5. Configure and run:**

```sh
mkdir -p /etc/pgwd
cp /usr/local/share/examples/pgwd/pgwd.conf.example /etc/pgwd/pgwd.conf
vi /etc/pgwd/pgwd.conf
rcctl enable pgwd
rcctl start pgwd
rcctl check pgwd
pgwd -config /etc/pgwd/pgwd.conf -dry-run -interval 0
```

## Test with a local tarball (before a GitHub release)

1. Match **DISTFILES** (e.g. **`pgwd_v0.6.7_openbsd_amd64.tar.gz`** after **`gmake port-openbsd-sync`**).

2. Build from repo root:

   ```bash
   gmake dist-openbsd
   # gmake dist-openbsd OPENBSD_ARCH=arm64
   ```

3. **Option A — DISTDIR:** `make show=DISTDIR`, copy tarball with exact **DISTFILES** name, **`make makesum`**, **`make install`**.

4. **Option B — `file://`:**

   ```sh
   cp /path/to/pgwd/dist/pgwd_v0.6.7_openbsd_amd64.tar.gz /tmp/pgwd-dist/
   cd /usr/ports/sysutils/pgwd
   make fetch MASTER_SITES=file:///tmp/pgwd-dist/
   make install
   ```

   Use an **absolute** path. Do not commit **`file://`** to the official port.

**Shortcut:** **`test-install-from-dist.sh`** — install without full ports tree (debug only).

## Submit to OpenBSD

See **[../PORT-RELEASE.md](../PORT-RELEASE.md)** Part C: **`make makesum`**, ports-tree **`cvs diff`** / **`git diff`**, email **ports@openbsd.org**.

[OpenBSD Porting Guide](https://www.openbsd.org/faq/ports/guide.html)
