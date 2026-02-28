#!/usr/bin/env bash
# Run security scans before merging to main: govulncheck (Go deps), optional Grype.
# From repo root: ./tools/scan.sh
# Exit non-zero if govulncheck finds vulnerabilities.

set -e

cd "$(dirname "$0")/.."

GOVULNCHECK_FAIL=0

# --- govulncheck (Go vulnerabilities) ---
if command -v govulncheck >/dev/null 2>&1; then
  echo "=== govulncheck ./... ==="
  if ! govulncheck ./...; then
    GOVULNCHECK_FAIL=1
  fi
else
  echo "govulncheck not found; install with: go install golang.org/x/vuln/cmd/govulncheck@latest"
  GOVULNCHECK_FAIL=1
fi

# --- Optional: Grype (if on PATH) ---
if command -v grype >/dev/null 2>&1; then
  echo "=== grype (current dir) ==="
  grype . || true
else
  echo "Grype not found (optional); install from https://github.com/anchore/grype#installation"
fi

exit "$GOVULNCHECK_FAIL"
