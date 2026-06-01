# BSD ports — step-by-step guide (pgwd)

Ordered workflow for **FreeBSD** and **OpenBSD** port maintainers. Assumes you are new to ports; detail lives in the linked docs.

| If you want… | Read |
|--------------|------|
| **This guide** (first time / recap) | You are here |
| FreeBSD operator install | [freebsd/README.md](freebsd/README.md) |
| FreeBSD port lab + Bugzilla | [freebsd/PORT-RELEASE.md](freebsd/PORT-RELEASE.md) |
| OpenBSD tarball install | [openbsd/README.md](openbsd/README.md) |
| OpenBSD port lab + ports@ diff | [openbsd/PORT-RELEASE.md](openbsd/PORT-RELEASE.md) |
| OpenBSD port directory reference | [openbsd/port/README.md](openbsd/port/README.md) |
| Ansible smoke (tarball, not port) | [testing/platforms/README.md](../testing/platforms/README.md) |

---

## 1. Three layers (do not mix them up)

| Layer | What it is | Where it lives |
|-------|------------|----------------|
| **Application** | Go binary, man, config example, rc.d in tarball | **pgwd** repo; **GoReleaser** on tag **`v*`** |
| **Port recipe** | “Fetch tarball X, install files Y, register service Z” | **`contrib/freebsd/`** · **`contrib/openbsd/port/`** |
| **Ports tree** | OS framework that builds a **native package** | **`/usr/ports`** on a BSD VM |

The port **does not compile Go**. It **fetches a release tarball** and runs **do-install** (+ **PLIST** / **@rcscript** on OpenBSD).

```text
  VERSION (repo root)
       │
       ├─ gmake port-freebsd-sync  ──► contrib/freebsd/Makefile (PORTVERSION)
       ├─ gmake port-openbsd-sync  ──► contrib/openbsd/port/Makefile + pkg/pgwd.rc + files/pgwd
       │
       ├─ gmake dist-freebsd       ──► dist/pgwd_v<ver>_freebsd_<arch>.tar.gz
       ├─ gmake dist-openbsd       ──► dist/pgwd_v<ver>_openbsd_<arch>.tar.gz
       │     (or GoReleaser on tag v<ver> — same filenames)
       │
       └─ copy contrib/*/port ──► /usr/ports/sysutils/pgwd/  (OpenBSD)
                │
                ▼
          make makesum → make package → make install → rcctl / sysrc
```

---

## 2. Where to run commands

| Machine | Make | Typical targets |
|---------|------|-----------------|
| **pgwd repo** (Mac / Linux) | **GNU make** (`gmake` on BSD if `make` is BSD make) | `port-freebsd-sync`, `port-openbsd-sync`, `dist-freebsd`, `dist-openbsd`, `make snapshot` |
| **FreeBSD / OpenBSD VM** | **BSD make** in **`/usr/ports/...`** | `make fetch`, `make makesum`, `make package`, `make install` |

After every **`VERSION`** bump, run **`gmake port-freebsd-sync`** and/or **`gmake port-openbsd-sync`** before copying port files to a VM.

---

## 3. Scenario A — Local lab (no GitHub tag yet)

Use when you want to test the port **before** GoReleaser publishes assets.

1. Bump **`VERSION`** on **`develop`** (or use current **`VERSION`** for a throwaway test).
2. Sync ports:

   ```bash
   cd /path/to/pgwd
   gmake port-openbsd-sync
   gmake port-freebsd-sync    # if testing FreeBSD too
   ```

3. Build local distfiles:

   ```bash
   gmake dist-openbsd
   gmake dist-freebsd
   ```

4. Copy **only** the port skeleton to the VM (OpenBSD example):

   ```bash
   cp -r contrib/openbsd/port /tmp/pgwd-port
   scp -r /tmp/pgwd-port/* root@openbsd:/tmp/pgwd-port/
   scp dist/pgwd_v*_openbsd_*.tar.gz root@openbsd:/tmp/
   ```

5. On the VM, follow **[openbsd/PORT-RELEASE.md](openbsd/PORT-RELEASE.md)** Part **B** (ports tree, **makesum**, **package**, **install**).

**OpenBSD shortcut:** [openbsd/port/test-install-from-dist.sh](openbsd/port/test-install-from-dist.sh) — mimics install without full **/usr/ports**; not a substitute for Part B before **ports@** submission.

---

## 4. Scenario B — Release maintainer (tag on GitHub)

1. Complete application release: **`make release-check`**, merge **`develop` → `main`**, tag **`v<VERSION>`**, CI/GoReleaser publishes assets.
2. Confirm on [GitHub Releases](https://github.com/hrodrig/pgwd/releases), for example:
   - `pgwd_v0.6.7_openbsd_amd64.tar.gz`
   - `pgwd_v0.6.7_freebsd_amd64.tar.gz`
   - `.deb`, `.rpm`, Linux archives, container image, etc.
3. **`gmake port-openbsd-sync`** (and **`port-freebsd-sync`** if needed) on **`main`** or **`develop`** at the released **`VERSION`**.
4. On each BSD VM: refresh **`/usr/ports/sysutils/pgwd`**, **`make fetch`** (or copy distfile to **`DISTDIR`**), **`make makesum`**, **`make package FETCH_PACKAGES=No`**, **`make install`**.

You do **not** need **`gmake dist-*`** on the laptop if GoReleaser already published matching filenames.

---

## 5. Submit to official trees (optional)

| OS | Submit to | Guide |
|----|-----------|--------|
| FreeBSD | [Bugzilla](https://bugs.freebsd.org/) | [freebsd/PORT-RELEASE.md](freebsd/PORT-RELEASE.md) |
| OpenBSD | **ports@openbsd.org** | [openbsd/PORT-RELEASE.md](openbsd/PORT-RELEASE.md) Part **C** |

Until accepted, **`pkg_add pgwd`** / **`pkg install pgwd`** from **official mirrors will not work**. Use tarball or lab **`make install`**.

**OpenBSD:** **`distinfo`** is generated on the VM with **`make makesum`** and included in the **ports-tree diff** — never commit it in the **pgwd** repo.

---

## 6. `gmake` targets (pgwd repo root)

| Target | Effect |
|--------|--------|
| **`port-freebsd-sync`** | **`PORTVERSION=`** in **`contrib/freebsd/Makefile`** |
| **`port-openbsd-sync`** | **`DISTNAME`**, **`PKGNAME`**, **`MASTER_SITES`**, **`DISTFILES`**; sync **`pkg/pgwd.rc`** and **`files/pgwd`** |
| **`dist-freebsd`** | **`dist/pgwd_v<ver>_freebsd_<arch>.tar.gz`** |
| **`dist-openbsd`** | **`dist/pgwd_v<ver>_openbsd_<arch>.tar.gz`** (**`OPENBSD_ARCH`**, default **amd64**) |

---

## 7. FreeBSD vs OpenBSD cheat sheet

| Topic | FreeBSD | OpenBSD |
|-------|---------|---------|
| Port in repo | **`contrib/freebsd/`** | **`contrib/openbsd/port/`** |
| Service | **`contrib/freebsd/rc.d/pgwd`** → **`/usr/local/etc/rc.d/pgwd`** | **`pkg/pgwd.rc`** + **`@rcscript`** → **`/etc/rc.d/pgwd`** |
| Daemon command | **`service pgwd`** / **sysrc** | **`rcctl`** |
| Tarball rc.d | optional in tarball | **`share/openbsd/rc.d/pgwd`** in tarball |
| Enable / start | **`sysrc pgwd_enable=YES`**, **`service pgwd start`** | **`rcctl enable pgwd`**, **`rcctl start pgwd`** |
| Build package | **`make install`** often enough in lab | **`make package FETCH_PACKAGES=No`** then **`make install`** |
| **`distinfo` in pgwd repo?** | No | No (**gitignored**) |

---

## 8. Ansible vs port validation

| Path | Command | What it proves |
|------|---------|----------------|
| **Ansible** | **`make test-platforms`** | Tarball install; **rc.d from repo clone**; dry-run; notifications; uninstall |
| **Port lab** | **`make install`** in **`/usr/ports/sysutils/pgwd`** | **Makefile**, **PLIST**, **`@rcscript`**, **`pkg_add`** lifecycle |

Use **both** before emailing **ports@** or FreeBSD Bugzilla: Ansible for fast regression; port lab for what operators get from the package system.

---

## 9. Common mistakes

| Mistake | Fix |
|---------|-----|
| **`make port-openbsd-sync`** on FreeBSD | Use **`gmake`** on the app repo |
| **`gmake makesum`** inside **`contrib/openbsd/port/`** | Copy port to **`/usr/ports/sysutils/pgwd`**, then BSD **`make makesum`** |
| **`VERSION`** out of sync with port **Makefile** | Always **`gmake port-*-sync`** after bumping **`VERSION`** |
| Copied all of **`contrib/openbsd/`** into ports | Copy **only** **`contrib/openbsd/port/*`** |
| OpenBSD **`make package`** only *Link to …* | **`make clean=package clean`**, **`make package FETCH_PACKAGES=No`** |
| Expect **`pkg_add pgwd`** from mirrors today | Port not in official tree until accepted |
| Wrong rc.d path after port install | OpenBSD needs **`@rcscript`** — see **[openbsd/PORT-RELEASE.md](openbsd/PORT-RELEASE.md)** |
