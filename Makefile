# pgwd — build and install (macOS, Linux, Windows)

BINARY   := pgwd
DIST     := dist
# Version: read from VERSION file (e.g. 0.1.0); if missing, use v0.1.0. Override: make build VERSION=v0.2.0
VERSION  ?= $(shell v=$$(cat VERSION 2>/dev/null | tr -d '\n\r'); [ -n "$$v" ] && echo "v$$v" || echo "v0.1.0")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDDATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -ldflags "-s -w -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILDDATE)"

# Build for current platform. Override version: make build VERSION=v0.1.0
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/pgwd

# --- Cross-compile: all platforms (output in dist/) ---

.PHONY: build-all build-linux build-darwin build-windows
build-all: build-linux build-darwin build-windows

build-linux:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-linux-amd64 ./cmd/pgwd
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-linux-arm64 ./cmd/pgwd

build-darwin:
	@mkdir -p $(DIST)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-darwin-amd64 ./cmd/pgwd
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-darwin-arm64 ./cmd/pgwd

build-windows:
	@mkdir -p $(DIST)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-windows-amd64.exe ./cmd/pgwd
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(DIST)/$(BINARY)-windows-arm64.exe ./cmd/pgwd

# Install: go install → $GOBIN (default $HOME/go/bin). Custom path: GOBIN=/usr/local/bin make install
install:
	go install $(LDFLAGS) ./cmd/pgwd

# Run tests
test:
	go test ./...

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
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILDDATE=$(BUILDDATE) -t pgwd .

# --- Release (requires goreleaser: brew install goreleaser) ---
# Release: only from main. Merge develop → main, update VERSION, then: git tag v0.1.0 && make release
.PHONY: release snapshot docker-build lint lint-fix
release:
	@branch=$$(git branch --show-current 2>/dev/null); \
	if [ "$$branch" != "main" ]; then \
	  echo "Error: release only from main (current: $$branch). Merge and checkout main first."; \
	  exit 1; \
	fi; \
	goreleaser release --clean

# Snapshot build (no tag required), outputs to dist/
snapshot:
	goreleaser release --snapshot --clean

# Remove built binary and dist/
clean:
	rm -f $(BINARY)
	rm -rf $(DIST)
