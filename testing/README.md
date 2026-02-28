# Testing

Local test helpers for pgwd.

## Postgres (compose)

`compose.yaml` runs PostgreSQL 16 (default **max_connections=20**) and an optional **client** service that holds one connection open. Use one or several clients to consume connections and test pgwd thresholds.

From the repo root:

```bash
# For integration tests: use 0 clients so the test process can connect (default max_connections=20).
docker compose -f testing/compose.yaml up -d --scale client=0
export PGWD_TEST_DB_URL="postgres://pgwd:pgwd@localhost:5432/pgwd?sslmode=disable"
go test ./internal/postgres/... -v -count=1
```

Use `-count=1` so the tests always run (no cache). Without `PGWD_TEST_DB_URL` the tests are skipped. If you see "too many clients already", scale clients down: `docker compose -f testing/compose.yaml up -d --scale client=0` then re-run the tests.

**Increase server connections** (e.g. 50):

```bash
MAX_CONNECTIONS=50 docker compose -f testing/compose.yaml up -d
```

**Several clients** (each holds one connection; to test pgwd thresholds). Run this *after* integration tests, or scale back to 0 before running tests:

```bash
docker compose -f testing/compose.yaml up -d --scale client=10
```

Stop:

```bash
docker compose -f testing/compose.yaml down
```
