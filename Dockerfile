FROM golang:1.26.4-alpine AS build
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILDDATE=unknown
ARG BRANCH=unknown
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY contrib/ ./contrib/
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILDDATE} -X main.Branch=${BRANCH}" -o /pgwd ./cmd/pgwd

# Minimal runtime: only ca-certificates for HTTPS (Slack/Loki). wget and nc are BusyBox applets
# (symlinks), not separate apk packages, so we cannot apk del them; we remove the symlinks with rm.
# curl is not in the base image.
# Runtime Alpine 3.24.1: OpenSSL security fixes (incl. CVE-2026-2673); pin patch release. Keep in sync with Dockerfile.release.
FROM alpine:3.24.1
LABEL org.opencontainers.image.title="pgwd"
LABEL org.opencontainers.image.description="Postgres Watch Dog - monitor PostgreSQL connections and notify via Slack/Loki"
LABEL org.opencontainers.image.source="https://github.com/hrodrig/pgwd"
LABEL org.opencontainers.image.authors="Hermes Rodríguez <https://github.com/hrodrig/pgwd>"
RUN apk update && apk upgrade && apk --no-cache add ca-certificates \
	&& rm -f /usr/bin/wget /usr/bin/nc
RUN adduser -D -g "" pgwd
COPY --from=build /pgwd /home/pgwd/pgwd
RUN chown pgwd:pgwd /home/pgwd/pgwd
USER pgwd
WORKDIR /home/pgwd
ENTRYPOINT ["/home/pgwd/pgwd"]
