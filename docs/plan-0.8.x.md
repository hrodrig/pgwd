# pgwd plan 0.8.x — supply chain security

**Roadmap index:** [ROADMAP.md](../ROADMAP.md) · **Band:** 0.8.x · **Status:** ✅ shipped (v0.8.0)

**Theme:** Syft SBOM + Cosign keyless signing for GHCR images and release artifacts. Same pattern as [groot](https://github.com/hrodrig/groot) and [kzero](https://github.com/hrodrig/kzero).

**Baseline:** [SPECIFICATIONS.md](../SPECIFICATIONS.md) updated for 0.7.x notifiers  
**Previous band:** [plan-0.7.x.md](./plan-0.7.x.md)  
**Next band:** [plan-0.9.x.md](./plan-0.9.x.md) (pre-1.0 polish)  
**Target window:** Jun 25–30, 2026

---

## Scope

### 1. Syft SBOM

Generate SPDX JSON SBOM for:

- Docker images (attached as OCI referrers)
- Release artifacts (`.tar.gz`, `.deb`, `.rpm` attached to GitHub Release)

### 2. Cosign keyless signing

Sign with OIDC (GitHub OIDC provider):

- Docker images: `cosign sign ghcr.io/hrodrig/pgwd:<tag>`
- Release artifacts: `cosign sign-blob` on checksums file
- Uses `github-actions` OIDC issuer from GHA workflow

### 3. GoReleaser integration

- Re-enable `sbom` and `docker_sign` once `buildx` supports attestations, **or**
- Separate GHA step after goreleaser for cosign (same as groot/kzero — **preferred for now**)

### 4. Operator documentation

- `cosign verify` commands in README
- SBOM inspection examples
- What operators should check before deploying

### 5. `make docker-scan` and CI

- Extend release/CI workflow to verify SBOM presence where applicable
- Consider `cosign verify` in CI (non-blocking initially if needed)
- `make docker-scan` (Grype) must still pass with `--fail-on high`

---

## Files to modify

| File | Action |
|------|--------|
| `.github/workflows/release.yml` | Add cosign + syft steps after goreleaser |
| `.github/workflows/security.yml` | Optional cosign verify on published image |
| `Makefile` | Add `make sbom`, `make sign` (or document GHA-only flow) |
| `README.md` | Supply chain verification section |
| `SPECIFICATIONS.md` | §11 Build and release (SBOM, signing) |
| `CHANGELOG.md` | 0.8.x release notes |

Status on `develop`:

| File | Status |
|------|--------|
| `.goreleaser.yaml` | ✅ `signs`, `sboms`, `docker_signs` |
| `.github/workflows/release.yml` | ✅ cosign + syft + post-release verify |
| `README.md` | ✅ Supply chain verification section |
| `SPECIFICATIONS.md` | ✅ §11 supply chain |
| `CHANGELOG.md` | ✅ [Unreleased] entry |
| `.github/workflows/security.yml` | ⏸️ cosign verify deferred (release workflow verifies) |
| `Makefile` | ⏸️ GHA-only via GoReleaser (documented in README) |

---

## Testing

- `make docker-scan` passes on built image
- Release workflow dry-run (GHA dispatch) validates cosign steps
- Manual: `cosign verify ghcr.io/hrodrig/pgwd:<tag>` on a test tag
- `make release-check` still green (no regression)

---

## Open questions

| # | Question | Decision |
|---|----------|----------|
| 1 | GoReleaser native SBOM vs post-release syft step? | Post-release GHA step matches groot/kzero; revisit when GoReleaser + buildx attestations are stable. |

---

## Release checklist

- [ ] Tag `v0.8.0` from `main`
- [ ] Published image and release artifacts signed; SBOM attached or documented
- [x] README verification section complete
- [x] SPEC §11 updated
- [x] GoReleaser + release workflow (cosign, syft, verify)
