# Scanning and security tools

Scripts and guidance for running security and quality scans **before merging to main** (e.g. on `develop` or in a PR). CI also runs some of these; see `.github/workflows/security.yml`.

**Recommended before merge/release:** Run `./tools/scan.sh` (govulncheck + optional Grype on dir), then **build the image and run Grype on it** (see below) for a complete, release-grade scan of what you actually ship.

## What we use

| Tool | Purpose | When |
|------|---------|------|
| **govulncheck** | Go vulnerability DB — finds known vulns in Go module dependencies | Local (`tools/scan.sh`), CI on PR/push to develop |
| **Grype** | Vulnerability scanner (SBOM or container image) | Optional: local or CI; run against built image or Syft-generated SBOM |
| **SonarQube / SonarCloud** | Code quality and security (optional) | If you have a project configured; run in CI or locally with `sonar-scanner` |

## Install Grype locally (macOS)

**Homebrew (recommended):**
```bash
brew install grype
```

**Alternative (installer script):**
```bash
curl -sSfL https://get.anchore.io/grype | sh -s -- -b /usr/local/bin
```

**Other:** See [Grype installation](https://github.com/anchore/grype#installation) (Linux, Docker, etc.).

## Run locally before pushing to main

From the repo root:

```bash
./tools/scan.sh
```

- Runs **govulncheck** (Go deps). Install if needed: `go install golang.org/x/vuln/cmd/govulncheck@latest`.
- If **Grype** is on `PATH`, runs it against the repo (directory scan). Install with the commands above.

Exit code is non-zero if govulncheck finds vulnerabilities, so you can gate merges on it.

**Grype on a directory:** When `scan.sh` runs Grype against the current directory, you may see warnings: *"no explicit name and version provided for directory source"* (Grype derives an artifact ID) and *"Unable to determine the OS distribution of some packages"* (the repo is mixed content, not an OS image, so OS-specific vulnerabilities may be missed). For **high-quality, release-grade** results, run Grype against the **built Docker image** (see next section); that scan sees the exact runtime (Alpine, OS packages, binary) and avoids these limitations.

## Scan the Docker image locally with Grype (recommended before release)

Build the image (same Dockerfile as CI), then run Grype on it. This is the **recommended** way to run Grype before merging to main or cutting a release: it scans the exact artifact you ship (OS + packages + binary), with no directory-scan warnings.

```bash
# From repo root
docker build -t pgwd:scan .
grype pgwd:scan
```

To fail the command on high or critical vulnerabilities (e.g. for scripting):

```bash
grype pgwd:scan --fail-on high,critical
```

CI does the same: builds the image and runs `grype pgwd:scan --fail-on high,critical` (see `.github/workflows/security.yml`).

**What you’ll see in practice:** A scan of the image often reports vulnerabilities in the **base image and OS packages** (Alpine: e.g. busybox, zlib, libc). Grype prints a table: package name, installed version, CVE ID, severity (Critical/High/Medium/Low), and sometimes EPSS. It’s normal to have some **Medium** (or Low) findings in base layers. CI uses `--fail-on high,critical` so the job only fails on High/Critical; Medium findings are still visible in the log so you can track them. To reduce findings over time: pin a **newer Alpine base** in the Dockerfile (e.g. `FROM golang:1.26-alpine` or a newer `alpine:3.x` when the Go image allows it), rebuild and re-scan. Let the base image maintainers ship fixed versions; **do not** try to remove or upgrade individual base packages (e.g. zlib) in Alpine — that often breaks the image. Accept or document any remaining medium/low risk if they are not exploitable in our use case.

## CI

- **Security** workflow: runs on push/PR to `main` and `develop`. Jobs: **govulncheck** (Go deps), **Grype** (builds image, scans it with `--fail-on high,critical`).

## Adding SonarQube

If you use SonarCloud or a SonarQube server:

1. Add a job in `.github/workflows/security.yml` (or a new workflow) that runs `sonar-scanner` with the project key and token (secret).
2. Optionally add a script under `tools/` for local runs (e.g. `tools/sonar.sh`) that invokes `sonar-scanner` with the same config.

No SonarQube project or token is required for govulncheck or Grype.
