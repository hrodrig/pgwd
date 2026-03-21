# pgwd — build and install (macOS, Linux, Windows)

BINARY   := pgwd
# Check Docker is running before targets that use it. Fails early with clear message.
check-docker = @docker info >/dev/null 2>&1 || { echo "Error: Docker is not running. Start Docker and try again."; exit 1; }
DIST     := dist
# Version: read from VERSION file (e.g. 0.1.0); if missing, use v0.1.0. Override: make build VERSION=v0.2.0
VERSION  ?= $(shell v=$$(cat VERSION 2>/dev/null | tr -d '\n\r'); [ -n "$$v" ] && echo "v$$v" || echo "v0.1.0")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDDATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -ldflags "-s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILDDATE)"

# Default target: show help
.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "pgwd — Postgres Watch Dog"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build:"
	@echo "  build              Build binary for current platform"
	@echo "  build-all          Cross-compile for Linux, macOS, Windows (output in dist/)"
	@echo "  build-linux        Cross-compile for Linux (amd64, arm64, riscv64)"
	@echo "  build-darwin       Cross-compile for macOS (amd64, arm64)"
	@echo "  build-windows      Cross-compile for Windows (amd64, arm64)"
	@echo "  build-solaris      Cross-compile for Solaris (amd64)"
	@echo ""
	@echo "Install & run:"
	@echo "  install            Install to \$$GOBIN (go install)"
	@echo "  install-man        Install man page to \$$MANDIR/man1 (default /usr/local/share/man)"
	@echo "  clean              Remove binary and dist/"
	@echo ""
	@echo "Test:"
	@echo "  test               Unit tests"
	@echo "  test-integration   Integration tests (requires Docker)"
	@echo "  test-e2e-kube      E2E test with kind cluster (requires kind, kubectl, Docker)"
	@echo ""
	@echo "Quality:"
	@echo "  lint               Check gofmt and gocyclo"
	@echo "  lint-fix           Fix formatting (gofmt -s -w)"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build       Build image with version info"
	@echo "  docker-scan        Build image and run Grype (security scan)"
	@echo ""
	@echo "Release:"
	@echo "  release-check      Run all checks (lint, test, test-integration, test-e2e-kube, docker-scan)"
	@echo "  release            Full release (from main only; runs release-check first)"
	@echo "  snapshot           Goreleaser snapshot build (outputs to dist/)"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make build VERSION=v0.5.0"
	@echo "  GOBIN=/usr/local/bin make install"
	@echo "  make release-check"

# Build for current platform. Override version: make build VERSION=v0.1.0
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/pgwd

# --- Cross-compile: all platforms (output in dist/) ---

.PHONY: build-all build-linux build-darwin build-windows build-solaris
build-all: build-linux build-darwin build-windows build-solaris

build-linux:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-linux-amd64 ./cmd/pgwd
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-linux-arm64 ./cmd/pgwd
	GOOS=linux GOARCH=riscv64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-linux-riscv64 ./cmd/pgwd

build-darwin:
	@mkdir -p $(DIST)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-darwin-amd64 ./cmd/pgwd
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-darwin-arm64 ./cmd/pgwd

build-windows:
	@mkdir -p $(DIST)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-windows-amd64.exe ./cmd/pgwd
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-windows-arm64.exe ./cmd/pgwd

build-solaris:
	@mkdir -p $(DIST)
	GOOS=solaris GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-solaris-amd64 ./cmd/pgwd

# Install: go install → $GOBIN (default $HOME/go/bin). Custom path: GOBIN=/usr/local/bin make install
install:
	go install $(LDFLAGS) ./cmd/pgwd

# Install man page. MANDIR defaults to /usr/local/share/man. Use: MANDIR=/usr/share/man make install-man
MANDIR ?= /usr/local/share/man
.PHONY: install-man
install-man:
	@mkdir -p $(MANDIR)/man1
	@cp contrib/man/man1/pgwd.1 $(MANDIR)/man1/
	@echo "Installed man page to $(MANDIR)/man1/pgwd.1"

# Run tests (unit tests; integration tests are skipped without PGWD_TEST_* env vars)
test:
	go test ./...

# E2E kube test: create kind cluster, deploy Postgres, run pgwd -kube-postgres -dry-run, destroy cluster.
# Requires: kind, kubectl, docker. Use before release to validate -kube-postgres path.
test-e2e-kube:
	$(check-docker)
	@command -v kind >/dev/null 2>&1 || { echo "kind not found; install with: brew install kind or https://kind.sigs.k8s.io/docs/user/quick-start/#installation"; exit 1; }
	@command -v kubectl >/dev/null 2>&1 || { echo "kubectl not found; install with: brew install kubectl"; exit 1; }
	@chmod +x testing/scripts/test-e2e-kube.sh
	@testing/scripts/test-e2e-kube.sh

# Integration tests: require Docker. Start Postgres and Loki, run tests, then stop.
# Use before release to validate Postgres and Loki integration.
test-integration:
	$(check-docker)
	@echo "Starting Postgres..."
	@docker compose -f testing/compose.yaml up -d --scale client=0
	@echo "Starting Loki..."
	@docker compose -f testing/compose-loki.yaml up -d
	@echo "Waiting for Postgres (healthcheck)..."
	@until docker compose -f testing/compose.yaml exec -T postgres pg_isready -U pgwd -d pgwd 2>/dev/null; do sleep 2; done
	@echo "Waiting for Loki (/ready)..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
	  curl -sf http://localhost:3100/ready | grep -q ready && break; \
	  sleep 2; \
	  if [ $$i -eq 15 ]; then echo "Loki not ready after 30s"; exit 1; fi; \
	done
	@echo "Running integration tests..."
	@PGWD_TEST_DB_URL="postgres://pgwd:pgwd@localhost:5432/pgwd?sslmode=disable" \
	 PGWD_TEST_LOKI_URL="http://localhost:3100/loki/api/v1/push" \
	 go test ./internal/postgres/... ./internal/notify/... -v -count=1 -run 'TestPool_Integration|TestStats_Integration|TestMaxConnections_Integration|TestStaleCount_Integration|TestLoki_Integration$$' || (docker compose -f testing/compose.yaml down; docker compose -f testing/compose-loki.yaml down; exit 1)
	@docker compose -f testing/compose.yaml down
	@docker compose -f testing/compose-loki.yaml down
	@echo "Integration tests passed."

# Lint: gofmt + gocyclo (run during development; CI runs this too)
lint:
	@echo "Checking gofmt -s..."
	@unformatted=$$(gofmt -s -l .); [ -z "$$unformatted" ] || { echo "Files not formatted (run make lint-fix):"; echo "$$unformatted"; exit 1; }
	@echo "Checking gocyclo (complexity <= 14)..."
	@command -v gocyclo >/dev/null 2>&1 || go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	@gocyclo -over 14 .

# Fix formatting only (gofmt -s -w); re-run make lint to verify gocyclo
lint-fix:
	gofmt -s -w .

# Docker image with version/commit/builddate from VERSION and git (run from repo root)
docker-build:
	$(check-docker)
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILDDATE=$(BUILDDATE) -t pgwd .

# Build image as pgwd:scan and run Grype (--fail-on high). Requires: docker, grype on PATH.
docker-scan:
	$(check-docker)
	@command -v grype >/dev/null 2>&1 || { echo "grype not found; install with: brew install grype or https://github.com/anchore/grype#installation"; exit 1; }
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILDDATE=$(BUILDDATE) -t pgwd:scan .
	grype pgwd:scan --fail-on high

# --- Release (requires goreleaser: brew install goreleaser) ---
# release-check: MANDATORY before release. Runs lint, test, test-integration, test-e2e-kube, docker-scan. All must pass.
.PHONY: release-check
release-check:
	@echo "Running release checks (lint, test, test-integration, test-e2e-kube, docker-scan)..."
	@$(MAKE) lint
	@$(MAKE) test
	@$(MAKE) test-integration
	@$(MAKE) test-e2e-kube
	@$(MAKE) docker-scan
	@echo "All release checks passed."

# Release: only from main. Requires release-check to pass. Merge develop → main, update VERSION, then: git tag v0.1.0 && make release
.PHONY: help release snapshot docker-build docker-scan lint lint-fix test-integration
release: release-check
	$(check-docker)
	@branch=$$(git branch --show-current 2>/dev/null); \
	if [ "$$branch" != "main" ]; then \
	  echo "Error: release only from main (current: $$branch). Merge and checkout main first."; \
	  exit 1; \
	fi; \
	goreleaser release --clean

# Snapshot build (no tag required), outputs to dist/
snapshot:
	$(check-docker)
	goreleaser release --snapshot --clean

# Remove built binary and dist/
clean:
	rm -f $(BINARY)
	rm -rf $(DIST)
