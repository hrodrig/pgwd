# syntax=docker/dockerfile:1
# Local / CI image: compile inside Docker (make docker-build, security workflow Grype scan).
# Release images: GoReleaser builds static binaries, then Dockerfile.release packages them (distroless).
FROM golang:1.26.6-alpine AS build
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

# distroless/static: CA certs for HTTPS notifiers; no shell, apk, wget, or nc (smaller attack surface than Alpine).
FROM gcr.io/distroless/static-debian13:nonroot
LABEL org.opencontainers.image.title="pgwd"
LABEL org.opencontainers.image.description="Postgres Watch Dog - monitor PostgreSQL connections and notify via Slack/Loki"
LABEL org.opencontainers.image.source="https://github.com/hrodrig/pgwd"
LABEL org.opencontainers.image.authors="Hermes Rodríguez <https://github.com/hrodrig/pgwd>"
COPY --from=build /pgwd /home/pgwd/pgwd
USER nonroot:nonroot
ENTRYPOINT ["/home/pgwd/pgwd"]
