# Testing

Local test helpers for pgwd.

## Postgres (compose)

`compose.yaml` runs PostgreSQL 16 (default **max_connections=20**) and an optional **client** service that holds one connection open. Use one or several clients to consume connections and test pgwd thresholds.

**Users:** The **pgwd** user (superuser) is used for the monitor and for `PGWD_TEST_DB_URL`. Client containers use **pgwd_app** (non-superuser), so they only consume the "normal" connection slots; the 3 reserved by `superuser_reserved_connections` stay free. That way you can always open an admin session from inside the Postgres container (`psql -U pgwd -d pgwd`) even when clients have filled the rest. In production, use a non-superuser for application connections so reserved slots remain available for DBA access; see [PostgreSQL runtime config — Connection and Authentication](https://www.postgresql.org/docs/current/runtime-config-connection.html) (`superuser_reserved_connections`).

**If you still get "too many clients" when using `psql -U pgwd` from inside the container:** the client containers were likely started with the old compose (they use **pgwd** and fill all slots). Recreate the stack so clients use **pgwd_app**, and ensure the init script has run (so the role exists). From the repo root:

```bash
# Remove containers and, if the DB was created before init-pgwd-app.sql existed, the volume too:
docker compose -f testing/compose.yaml down -v
# Start again; init runs and creates pgwd_app; clients use pgwd_app (max 17 connections with default max_connections=20):
docker compose -f testing/compose.yaml up -d --scale client=17
```

Then open a shell in the Postgres container and run `psql -U pgwd -d pgwd`; the 3 reserved slots should be free.

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

**Production:** Use a non-superuser role for application connections so `superuser_reserved_connections` (default 3) stays available for DBA/admin access when the instance is saturated. See [PostgreSQL: Connection and Authentication](https://www.postgresql.org/docs/current/runtime-config-connection.html) (`superuser_reserved_connections`).
