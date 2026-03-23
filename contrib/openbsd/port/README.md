# OpenBSD port for pgwd

Port files for submitting pgwd to the official OpenBSD ports tree.

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
make makesum    # Verify/update distinfo checksums
make fetch
make install
```

## Submit to OpenBSD

OpenBSD ports are maintained via diff/patch to the mailing list:

1. Make your changes in a checkout of the ports tree
2. Generate a diff: `cvs diff -u > pgwd-port.diff` (or `hg diff` / `git diff` for the ports repo)
3. Send to **ports@openbsd.org** with a descriptive subject

See [OpenBSD Porting Guide](https://www.openbsd.org/faq/ports/guide.html) for details.
