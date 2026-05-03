# OpenBSD port for pgwd

Port files for submitting pgwd to the official OpenBSD ports tree.

## Version bump

From the pgwd repo root, after updating `VERSION`, run `make port-openbsd-sync` to refresh `DISTNAME`, `PKGNAME`, `MASTER_SITES`, and `DISTFILES` in this Makefile.

## distinfo (not shipped in the pgwd repo)

**`distinfo`** holds `SHA256` / `SIZE` lines for each **DISTFILES** artifact. Those checksums **require the exact tarball bytes** (usually from a **published GitHub release**). Committing **`distinfo` here was inconsistent**: it lagged **Makefile** versions or implied a release that did not exist yet.

- This skeleton **does not include `distinfo`**. After **`make fetch`** (or a local tarball; see below), run **`make makesum`** in your **OpenBSD ports** checkout to generate **`distinfo`** there, then include it in the diff you send to **ports@openbsd.org**.
- **`contrib/openbsd/port/distinfo`** is listed in **`.gitignore`** so a local **`make makesum`** inside a clone does not get committed by mistake.

If you keep **internal** release notes, tarball mirrors, or port digests next to Helm / self-hosted material, **[pgwd-selfhosted](https://github.com/hrodrig/pgwd-selfhosted)** is a reasonable place for **operator-only** copies of **`distinfo`** (or checksum lists)—not in the main **pgwd** tree.

## Layout

Copy this directory to `/usr/ports/sysutils/pgwd/`:

```bash
# On OpenBSD
cd /usr/ports
mkdir -p sysutils/pgwd
cp -r /path/to/pgwd/contrib/openbsd/port/* sysutils/pgwd/
cd sysutils/pgwd
```

## Build and test

```bash
make fetch      # needs distfile (GitHub release or local; see below)
make makesum    # writes distinfo from fetched files
make install
```

## Test with a local tarball (before a GitHub release)

The port normally downloads **DISTFILES** from **MASTER_SITES** (GitHub releases). To try **install** / **plist** / **rc.d** against a tarball you built locally (no `vX.Y.Z` tag on GitHub yet):

1. **Match the filename** in **DISTFILES** for your architecture (e.g. `pgwd_v0.6.4_openbsd_amd64.tar.gz` after `make port-openbsd-sync` from the pgwd repo).

   To generate that exact tarball layout/name locally (without running the full snapshot matrix), run from repo root:

   ```bash
   make dist-openbsd
   # Optional arch override:
   # make dist-openbsd OPENBSD_ARCH=arm64
   ```

   Output goes to `dist/pgwd_v<version>_openbsd_<arch>.tar.gz` using `VERSION`.

2. **Option A — copy into `DISTDIR`:** From the port directory, run `make show=DISTDIR`, then copy your tarball there with the **exact** name **DISTFILES** expects. Run `make checksum` (or `make makesum` if you are refreshing **distinfo**), then `make install`.

3. **Option B — `file://` override:** Point **MASTER_SITES** at a local directory that contains the same filename, for example:

   ```bash
   cd /usr/ports/sysutils/pgwd
   cp /path/to/pgwd/dist/pgwd_v0.6.4_openbsd_amd64.tar.gz /tmp/pgwd-dist/
   make fetch MASTER_SITES=file:///tmp/pgwd-dist/
   make install
   ```

   Use an **absolute** path after `file://` (three slashes for `file:///tmp/...`). Adjust the path and version to match **DISTFILES** / **PKGNAME** in this **Makefile**.

Do not commit **MASTER_SITES=file://...** to the official ports tree; it is only for local validation. For the real port, **MASTER_SITES** stays the GitHub release URL and **distinfo** matches the published asset.

## Submit to OpenBSD

OpenBSD ports are maintained via diff/patch to the mailing list:

1. Make your changes in a checkout of the ports tree
2. Generate a diff: `cvs diff -u > pgwd-port.diff` (or `hg diff` / `git diff` for the ports repo)
3. Send to **ports@openbsd.org** with a descriptive subject

See [OpenBSD Porting Guide](https://www.openbsd.org/faq/ports/guide.html) for details.
