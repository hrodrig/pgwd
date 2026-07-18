# pgwd — build and install (macOS, Linux, Windows, FreeBSD with gmake).
# GNU Make uses this file (GNUmakefile) before Makefile. On FreeBSD, plain
# `make` reads Makefile, which forwards targets to `gmake` (pkg install gmake).

BINARY   := pgwd
# Check Docker is running before targets that use it. Fails early with clear message.
check-docker = @docker info >/dev/null 2>&1 || { echo "Error: Docker is not running. Start Docker and try again."; exit 1; }
DIST     := dist
# Grype image scan (docker-scan): anchore/grype default gate; override e.g. GRYPE_FAIL_ON=critical
GRYPE_FAIL_ON ?= high
# Minimum total statement coverage for `make cover-check` (library packages; see COVER_TEST_PKGS).
COVERAGE_MIN ?= 80
# Packages measured by cover-check (excludes internal/cli — exercised via cmd/pgwd black-box tests).
COVER_TEST_PKGS := $(shell go list ./internal/... ./contrib/... | grep -v '/internal/cli$$')
# Version: read from VERSION file (e.g. 0.1.0); if missing, use v0.1.0. Override: make build VERSION=v0.2.0
VERSION  ?= $(shell v=$$(cat VERSION 2>/dev/null | tr -d '\n\r'); [ -n "$$v" ] && echo "v$$v" || echo "v0.1.0")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BRANCH   := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
BUILDDATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -ldflags "-s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILDDATE) -X main.Branch=$(BRANCH)"
# OpenBSD dist helper target default arch. Override: make dist-openbsd OPENBSD_ARCH=arm64
OPENBSD_ARCH ?= amd64

GREEN  := \033[0;32m
YELLOW := \033[0;33m
CYAN   := \033[0;36m
RESET  := \033[0m

# Default target: show help
.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "$(GREEN)pgwd$(RESET) — Postgres Watch Dog"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "$(YELLOW)Build:$(RESET)"
	@echo "  $(GREEN)build$(RESET)              Build binary for current platform"
	@echo "  $(GREEN)build-all$(RESET)          Cross-compile for Linux, macOS, Windows (output in dist/)"
	@echo "  $(GREEN)build-linux$(RESET)        Cross-compile for Linux (amd64, arm64, riscv64)"
	@echo "  $(GREEN)build-darwin$(RESET)       Cross-compile for macOS (amd64, arm64)"
	@echo "  $(GREEN)build-windows$(RESET)      Cross-compile for Windows (amd64, arm64)"
	@echo "  $(GREEN)build-solaris$(RESET)      Cross-compile for Solaris (amd64)"
	@echo ""
	@echo "$(YELLOW)Install & run:$(RESET)"
	@echo "  $(GREEN)install$(RESET)            Install to \$$GOBIN (go install)"
	@echo "  $(GREEN)install-man$(RESET)        Install man page to \$$MANDIR/man1 (default /usr/local/share/man)"
	@echo "  $(GREEN)clean$(RESET)              Remove binary and dist/"
	@echo ""
	@echo "$(YELLOW)Test:$(RESET)"
	@echo "  $(GREEN)test$(RESET)               Unit tests"
	@echo "  $(GREEN)bench$(RESET)              Run internal package benchmarks (non-blocking in CI)"
	@echo "  $(GREEN)test-integration$(RESET)   Integration tests (requires Docker)"
	@echo "  $(GREEN)test-e2e-kube$(RESET)      E2E test with kind cluster (requires kind, kubectl, Docker)"
	@echo "  $(GREEN)test-platforms$(RESET)     Multi-platform tests via Ansible (requires VMs; see testing/platforms/)"
	@echo "                         Target one platform: make test-platforms PLATFORM=pgwd-ubuntu"
	@echo "  $(GREEN)test-platforms-ping$(RESET)  Ansible ping (pong) + Postgres TCP preflight; same inventory"
	@echo ""
	@echo "$(YELLOW)Quality:$(RESET)"
	@echo "  $(GREEN)lint$(RESET)               Check gofmt, go vet, and gocyclo"
	@echo "  $(GREEN)lint-fix$(RESET)           Fix formatting (gofmt -s -w)"
	@echo "  $(GREEN)cover$(RESET)              Unit tests with coverage (coverage.out + summary line)"
	@echo "  $(GREEN)cover-check$(RESET)        Fail if library coverage < $(COVERAGE_MIN)% (Docker + Postgres; see COVER_TEST_PKGS)"
	@echo "  $(GREEN)cover-integration$(RESET)  Same stack as test-integration; go test ./... with coverage (coverage-integration.out)"
	@echo "  $(GREEN)tools$(RESET)              Install govulncheck and gocyclo to \$$GOBIN"
	@echo "  $(GREEN)security$(RESET)           govulncheck + docker-scan (same as CI Security workflow)"
	@echo ""
	@echo "$(YELLOW)Docker:$(RESET)"
	@echo "  $(GREEN)docker-build$(RESET)              Build image (native platform) as pgwd"
	@echo "  $(GREEN)docker-buildx-amd64$(RESET)       Build linux/amd64 only, load as pgwd:amd64"
	@echo "  $(GREEN)docker-buildx-amd64-push$(RESET)  Push linux/amd64 (needs DOCKER_IMAGE=registry/img:tag)"
	@echo "  $(GREEN)docker-scan$(RESET)               Build image and run Grype (uses Grype on PATH, else anchore/grype container)"
	@echo ""
	@echo "$(YELLOW)Release:$(RESET)"
	@echo "  $(GREEN)release-check$(RESET)      Run all checks (lint, test, cover-check, test-integration, test-e2e-kube, docker-scan)"
	@echo "  $(GREEN)release$(RESET)            Full release (from main only; runs release-check first)"
	@echo "  $(GREEN)snapshot$(RESET)           Goreleaser snapshot build (outputs to dist/)"
	@echo "  $(GREEN)dist-freebsd$(RESET)       Build FreeBSD tar.gz distfile for ports local testing"
	@echo "  $(GREEN)dist-openbsd$(RESET)       Build OpenBSD tar.gz distfile for ports local testing"
	@echo "  $(GREEN)port-freebsd-sync$(RESET)  Sync VERSION to contrib/freebsd/Makefile (run before port update)"
	@echo "  $(GREEN)port-openbsd-sync$(RESET)  Sync VERSION to contrib/openbsd/port/Makefile (run before port update)"
	@echo ""
	@echo "$(CYAN)Current version:$(RESET) $$(cat VERSION 2>/dev/null | tr -d '\n\r' || echo '?') (ldflags VERSION=$(VERSION), branch $(BRANCH))"
	@echo ""
	@echo "$(CYAN)Examples:$(RESET)"
	@echo "  make build"
	@echo "  make build VERSION=v0.6.4"
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

# Benchmarks (non-blocking in CI; no minimum performance gate)
.PHONY: bench
bench:
	go test -bench=. -benchmem -run=^$$ ./internal/...

# Coverage report (writes coverage.out in repo root; not part of release-check)
.PHONY: cover cover-check cover-integration integration-compose-up integration-compose-down
cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	@go tool cover -func=coverage.out | tail -1

# cover-check: library packages >= COVERAGE_MIN% (default 80). Requires Docker (Postgres for store/postgres tests).
cover-check:
	$(check-docker)
	@echo "Running cover-check (minimum $(COVERAGE_MIN)% on library packages)..."
	@$(MAKE) integration-compose-up
	@PGWD_TEST_DB_URL="postgres://pgwd:pgwd@localhost:5432/pgwd?sslmode=disable" \
	 go test $(COVER_TEST_PKGS) ./cmd/pgwd -count=1 -coverprofile=coverage.out -covermode=atomic || \
	 ($(MAKE) integration-compose-down; exit 1)
	@go tool cover -func=coverage.out | tail -1
	@pct=$$(go tool cover -func=coverage.out | grep '^total:' | awk '{print $$NF}' | tr -d '%'); \
	echo "Total statement coverage: $$pct% (minimum $(COVERAGE_MIN)%)"; \
	awk -v p="$$pct" -v m="$(COVERAGE_MIN)" 'BEGIN { if (p+0 < m+0) { print "Error: coverage is below " m "% — add tests or set COVERAGE_MIN="; exit 1 } }'
	@$(MAKE) integration-compose-down

# Start Postgres + Loki for integration tests (shared by test-integration and cover-integration).
integration-compose-up:
	$(check-docker)
	@echo "Starting Postgres..."
	@docker compose -f testing/compose.yaml up -d --scale client=0
	@echo "Starting Loki..."
	@docker compose -f testing/compose-loki.yaml up -d
	@echo "Waiting for Postgres (healthcheck)..."
	@until docker compose -f testing/compose.yaml exec -T postgres pg_isready -U pgwd -d pgwd 2>/dev/null; do sleep 2; done
	@until docker compose -f testing/compose.yaml exec -T postgres2 pg_isready -U pgwd -d analytics 2>/dev/null; do sleep 2; done
	@until docker compose -f testing/compose.yaml exec -T postgres3 pg_isready -U pgwd -d replica 2>/dev/null; do sleep 2; done
	@echo "Waiting for Loki (/ready)..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
	  curl -sf http://localhost:3100/ready | grep -q ready && break; \
	  sleep 2; \
	  if [ $$i -eq 15 ]; then echo "Loki not ready after 30s"; exit 1; fi; \
	done

integration-compose-down:
	@docker compose -f testing/compose.yaml down
	@docker compose -f testing/compose-loki.yaml down

# Full tree coverage with live Postgres + Loki (same env as integration tests). Writes coverage-integration.out.
cover-integration:
	@$(MAKE) integration-compose-up
	@PGWD_TEST_DB_URL="postgres://pgwd:pgwd@localhost:5432/pgwd?sslmode=disable" \
	 PGWD_TEST_LOKI_URL="http://localhost:3100/loki/api/v1/push" \
	 go test ./... -count=1 -coverprofile=coverage-integration.out -covermode=atomic || \
	 ($(MAKE) integration-compose-down; exit 1)
	@go tool cover -func=coverage-integration.out | tail -1
	@$(MAKE) integration-compose-down
	@echo "Wrote coverage-integration.out (open with: go tool cover -html=coverage-integration.out)"

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
	@$(MAKE) integration-compose-up
	@echo "Running integration tests..."
	@PGWD_TEST_DB_URL="postgres://pgwd:pgwd@localhost:5432/pgwd?sslmode=disable" \
	 PGWD_TEST_LOKI_URL="http://localhost:3100/loki/api/v1/push" \
	 go test ./internal/postgres/... ./internal/notify/... -v -count=1 -run 'TestPool_Integration|TestStats_Integration|TestMaxConnections_Integration|TestStaleCount_Integration|TestLoki_Integration$$' || ($(MAKE) integration-compose-down; exit 1)
	@echo "Running pgwd multi-database (databases: 3 Postgres)..."
	@$(MAKE) build
	@./pgwd -config testing/multidb-e2e.conf -dry-run -interval 0 || ($(MAKE) integration-compose-down; exit 1)
	@$(MAKE) integration-compose-down
	@echo "Integration tests passed."

# Multi-platform tests via Ansible. Requires VMs configured in testing/platforms/inventory/hosts.yml.
# Target one platform: make test-platforms PLATFORM=pgwd-ubuntu
PLATFORM ?=
.PHONY: test-platforms test-platforms-ping
test-platforms:
	@command -v ansible-playbook >/dev/null 2>&1 || { echo "ansible-playbook not found; install with: pip install ansible"; exit 1; }
	@test -f testing/platforms/inventory/hosts.yml || { echo "Error: testing/platforms/inventory/hosts.yml not found. Copy hosts.yml.example and edit."; exit 1; }
	cd testing/platforms && ansible-playbook playbooks/full-cycle.yml $(if $(PLATFORM),--limit $(PLATFORM),)

test-platforms-ping:
	@command -v ansible-playbook >/dev/null 2>&1 || { echo "ansible-playbook not found; install with: pip install ansible"; exit 1; }
	@test -f testing/platforms/inventory/hosts.yml || { echo "Error: testing/platforms/inventory/hosts.yml not found. Copy hosts.yml.example and edit."; exit 1; }
	cd testing/platforms && ansible-playbook playbooks/ping.yml $(if $(PLATFORM),--limit $(PLATFORM),)

# Install optional dev tools (govulncheck, gocyclo)
.PHONY: tools
tools:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/fzipp/gocyclo/cmd/gocyclo@latest

# Lint: gofmt + go vet + gocyclo (run during development; CI runs this too)
lint:
	@echo "Checking gofmt -s..."
	@unformatted=$$(gofmt -s -l .); [ -z "$$unformatted" ] || { echo "Files not formatted (run make lint-fix):"; echo "$$unformatted"; exit 1; }
	@echo "Running go vet..."
	@go vet ./...
	@echo "Checking gocyclo (complexity <= 14)..."
	@command -v gocyclo >/dev/null 2>&1 || go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	@gocyclo -over 14 .

# Fix formatting only (gofmt -s -w); re-run make lint to verify go vet and gocyclo
lint-fix:
	gofmt -s -w .

# Docker image with version/commit/builddate from VERSION and git (run from repo root)
docker-build:
	$(check-docker)
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILDDATE=$(BUILDDATE) --build-arg BRANCH=$(BRANCH) -t pgwd .

# linux/amd64 only — useful from Apple Silicon or when the target VPS is amd64. Loads into local Docker as pgwd:amd64.
.PHONY: docker-buildx-amd64
docker-buildx-amd64:
	$(check-docker)
	@docker buildx version >/dev/null 2>&1 || { echo "Error: docker buildx not available"; exit 1; }
	docker buildx build --platform linux/amd64 \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILDDATE=$(BUILDDATE) --build-arg BRANCH=$(BRANCH) \
		-t pgwd:amd64 --load .

# Push linux/amd64 to your registry (GHCR, Docker Hub, etc.). Login first: docker login <registry>
# Example: make docker-buildx-amd64-push DOCKER_IMAGE=ghcr.io/myorg/pgwd:develop-amd64
DOCKER_IMAGE ?=
.PHONY: docker-buildx-amd64-push
docker-buildx-amd64-push:
	$(check-docker)
	@docker buildx version >/dev/null 2>&1 || { echo "Error: docker buildx not available"; exit 1; }
	@[ -n "$(DOCKER_IMAGE)" ] || { echo "Error: set DOCKER_IMAGE (e.g. ghcr.io/org/pgwd:develop-amd64)"; exit 1; }
	docker buildx build --platform linux/amd64 --provenance=false \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILDDATE=$(BUILDDATE) --build-arg BRANCH=$(BRANCH) \
		-t $(DOCKER_IMAGE) --push .

# security: govulncheck (Go deps) + Grype on built image — mirrors .github/workflows/security.yml (not in release-check).
.PHONY: security
security:
	@echo "Running security checks (govulncheck, docker-scan)..."
	@command -v govulncheck >/dev/null 2>&1 || $(MAKE) tools
	@govulncheck ./...
	@$(MAKE) docker-scan
	@echo "All security checks passed."

# Build image as pgwd:scan and run Grype. Uses local grype if on PATH; otherwise anchore/grype via Docker (--pull=always).
docker-scan:
	$(check-docker)
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILDDATE=$(BUILDDATE) --build-arg BRANCH=$(BRANCH) -t pgwd:scan .
	@if command -v grype >/dev/null 2>&1; then \
		grype pgwd:scan --fail-on $(GRYPE_FAIL_ON); \
	else \
		echo "grype not on PATH; using anchore/grype container..."; \
		docker run --rm --pull=always -v /var/run/docker.sock:/var/run/docker.sock anchore/grype:latest \
			pgwd:scan --fail-on $(GRYPE_FAIL_ON); \
	fi

# --- Release (requires goreleaser: brew install goreleaser) ---
# release-check: MANDATORY before release. Requires Docker (all tests use it). Runs lint, test, cover-check, test-integration, test-e2e-kube, docker-scan. All must pass.
.PHONY: release-check
release-check:
	$(check-docker)
	@set -e; \
	test -f VERSION || { echo "Error: VERSION file is required"; exit 1; }; \
	ver_raw=$$(cat VERSION | tr -d '\n\r'); ver=$${ver_raw#v}; \
	echo "$$ver" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "Error: VERSION must be semantic MAJOR.MINOR.PATCH (got: $$ver_raw)"; exit 1; }; \
	echo "Release version: $$ver (tag v$$ver)"; \
	echo "Running release checks (lint, test, cover-check, test-integration, test-e2e-kube, docker-scan)...";
	@$(MAKE) lint
	@$(MAKE) test
	@$(MAKE) cover-check
	@$(MAKE) test-integration
	@$(MAKE) test-e2e-kube
	@$(MAKE) docker-scan
	@echo "All release checks passed."

# Release: only from main. Requires release-check to pass. Merge develop → main, update VERSION, then: git tag v0.1.0 && make release
.PHONY: help release snapshot dist-freebsd dist-openbsd docker-build docker-buildx-amd64 docker-buildx-amd64-push docker-scan security lint lint-fix test-integration port-openbsd-sync cover cover-check cover-integration integration-compose-up integration-compose-down tools
release: release-check
	$(check-docker)
	@branch=$$(git branch --show-current 2>/dev/null); \
	if [ "$$branch" != "main" ]; then \
	  echo "Error: release only from main (current: $$branch). Merge and checkout main first."; \
	  exit 1; \
	fi; \
	goreleaser release --clean

# Sync VERSION file to FreeBSD port Makefile. Run before updating the port for a new release.
PORT_VERSION := $(shell cat VERSION 2>/dev/null | tr -d '\n\r' | sed 's/^v//')
.PHONY: port-freebsd-sync
port-freebsd-sync:
	@[ -n "$(PORT_VERSION)" ] || { echo "Error: VERSION file empty or missing"; exit 1; }
	@sed -i.bak "s/^PORTVERSION=.*/PORTVERSION=\t$(PORT_VERSION)/" contrib/freebsd/Makefile
	@rm -f contrib/freebsd/Makefile.bak
	@echo "Updated contrib/freebsd/Makefile PORTVERSION to $(PORT_VERSION)"

# Sync VERSION file to OpenBSD port Makefile (DISTNAME, PKGNAME, MASTER_SITES, DISTFILES).
.PHONY: port-openbsd-sync
port-openbsd-sync:
	@[ -n "$(PORT_VERSION)" ] || { echo "Error: VERSION file empty or missing"; exit 1; }
	@test -f contrib/openbsd/port/Makefile || { echo "Error: contrib/openbsd/port/Makefile not found"; exit 1; }
	@sed -i.bak \
	  -e 's#^DISTNAME =.*#DISTNAME =	pgwd_v$(PORT_VERSION)_openbsd_$${MACHINE_ARCH:S/aarch64/arm64/}#' \
	  -e 's#^PKGNAME =.*#PKGNAME =	pgwd-$(PORT_VERSION)#' \
	  -e 's#^MASTER_SITES =.*#MASTER_SITES =	https://github.com/hrodrig/pgwd/releases/download/v$(PORT_VERSION)/#' \
	  -e 's#^DISTFILES =.*#DISTFILES =	pgwd_v$(PORT_VERSION)_openbsd_$${MACHINE_ARCH:S/aarch64/arm64/}.tar.gz#' \
	  contrib/openbsd/port/Makefile
	@rm -f contrib/openbsd/port/Makefile.bak
	@cp contrib/openbsd/pgwd contrib/openbsd/port/pkg/pgwd.rc
	@cp contrib/openbsd/pgwd contrib/openbsd/port/files/pgwd
	@echo "Updated contrib/openbsd/port/Makefile to $(PORT_VERSION)"
	@echo "Synced contrib/openbsd/port/pkg/pgwd.rc and port/files/pgwd from contrib/openbsd/pgwd"

# Snapshot build (no tag required), outputs to dist/. No Docker required (dockers_v2 disabled for snapshots in .goreleaser.yaml).
# Snapshot version comes from VERSION (e.g. VERSION=0.6.4 => snapshot 0.6.4-next), independent from reachable git tags.
# --skip=sign: keyless cosign needs GHA OIDC; local snapshot is for platform/package testing only.
snapshot:
	@ver_raw=$$(cat VERSION 2>/dev/null | tr -d '\n\r'); \
	[ -n "$$ver_raw" ] || { echo "Error: VERSION file is required for snapshot"; exit 1; }; \
	ver=$${ver_raw#v}; \
	echo "$$ver" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "Error: VERSION must be semantic MAJOR.MINOR.PATCH (got: $$ver_raw)"; exit 1; }; \
	PGWD_SNAPSHOT_VERSION="$$ver-next" goreleaser release --snapshot --clean --skip=sign

# Build only the FreeBSD distfile tarball expected by contrib/freebsd/Makefile DISTFILES.
# Uses VERSION file (v prefix optional) and current machine arch (aarch64 -> arm64).
dist-freebsd:
	@ver_raw=$$(cat VERSION 2>/dev/null | tr -d '\n\r'); \
	[ -n "$$ver_raw" ] || { echo "Error: VERSION file is required"; exit 1; }; \
	ver=$${ver_raw#v}; \
	echo "$$ver" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "Error: VERSION must be semantic MAJOR.MINOR.PATCH (got: $$ver_raw)"; exit 1; }; \
	arch=$$(uname -m | sed 's/^aarch64$$/arm64/'); \
	out="$(DIST)/pgwd_v$${ver}_freebsd_$$arch.tar.gz"; \
	stage="/tmp/pgwd-dist-root-$$PPID"; \
	echo "Building pgwd for FreeBSD $$arch with VERSION=v$$ver..."; \
	$(MAKE) build VERSION=v$$ver; \
	rm -rf "$$stage"; \
	mkdir -p "$$stage/share/man/man1" "$$stage/share/doc/pgwd" "$$stage/etc/pgwd" "$(DIST)"; \
	cp "$(BINARY)" "$$stage/pgwd"; \
	cp "contrib/man/man1/pgwd.1" "$$stage/share/man/man1/pgwd.1"; \
	cp "LICENSE" "$$stage/share/doc/pgwd/LICENSE"; \
	cp "contrib/pgwd.conf.example" "$$stage/etc/pgwd/pgwd.conf.example"; \
	tar -C "$$stage" -czf "$$out" .; \
	rm -rf "$$stage"; \
	echo "Wrote $$out"

# Build only the OpenBSD distfile tarball expected by contrib/openbsd/port/Makefile DISTFILES.
# Uses VERSION file (v prefix optional). Default arch is amd64; override with OPENBSD_ARCH=arm64.
dist-openbsd:
	@ver_raw=$$(cat VERSION 2>/dev/null | tr -d '\n\r'); \
	[ -n "$$ver_raw" ] || { echo "Error: VERSION file is required"; exit 1; }; \
	ver=$${ver_raw#v}; \
	echo "$$ver" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "Error: VERSION must be semantic MAJOR.MINOR.PATCH (got: $$ver_raw)"; exit 1; }; \
	echo "$(OPENBSD_ARCH)" | grep -qE '^(amd64|arm64|riscv64)$$' || { echo "Error: OPENBSD_ARCH must be one of: amd64, arm64, riscv64"; exit 1; }; \
	arch="$(OPENBSD_ARCH)"; \
	out="$(DIST)/pgwd_v$${ver}_openbsd_$$arch.tar.gz"; \
	stage="/tmp/pgwd-openbsd-dist-root-$$PPID"; \
	echo "Building pgwd for OpenBSD $$arch with VERSION=v$$ver..."; \
	GOOS=openbsd GOARCH="$$arch" go build $(LDFLAGS) -o "$(BINARY)" ./cmd/pgwd; \
	rm -rf "$$stage"; \
	mkdir -p "$$stage/share/man/man1" "$$stage/share/doc/pgwd" "$$stage/etc/pgwd" "$$stage/share/openbsd/rc.d" "$(DIST)"; \
	cp "$(BINARY)" "$$stage/pgwd"; \
	cp "contrib/man/man1/pgwd.1" "$$stage/share/man/man1/pgwd.1"; \
	cp "LICENSE" "$$stage/share/doc/pgwd/LICENSE"; \
	cp "contrib/pgwd.conf.example" "$$stage/etc/pgwd/pgwd.conf.example"; \
	cp "contrib/openbsd/pgwd" "$$stage/share/openbsd/rc.d/pgwd"; \
	tar -C "$$stage" -czf "$$out" .; \
	rm -rf "$$stage"; \
	echo "Wrote $$out"

# Remove built binary and dist/
clean:
	rm -f $(BINARY)
	rm -rf $(DIST)
