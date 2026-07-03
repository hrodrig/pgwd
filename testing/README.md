# Testing

Local test helpers for pgwd.

**Security:** Compose files set resource limits (`mem_limit`, `cpus`) and run client/Loki as non-root (`user`) to satisfy Snyk and similar scanners. Postgres entrypoint requires root for init.

## Postgres (compose)

`compose.yaml` runs PostgreSQL 16 (default **max_connections=20**), an optional **client** service that holds one connection open, and **postgres2** / **postgres3** for multi-database E2E (databases `pgwd`, `analytics`, `replica` on ports 5432, 5433, 5434).

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

---

## Coverage

- **`make cover-check`** — library packages ≥ **80%** (default; requires Docker + Postgres). Excludes `internal/cli` (black-box via `cmd/pgwd`). Gate in **`make release-check`** and CI.
- **`make cover`** (repo root) — runs **`go test ./...`** with **`-coverprofile=coverage.out`**. Unit tests only: **`internal/postgres`** integration tests are skipped without **`PGWD_TEST_DB_URL`**, and **`cmd/pgwd`** stays near **0%** in-package (black-box tests build a separate binary via **`exec`**).

- **`make cover-integration`** — starts the same **Postgres + Loki** stack as **`make test-integration`**, exports **`PGWD_TEST_DB_URL`** and **`PGWD_TEST_LOKI_URL`**, then **`go test ./... -count=1 -coverprofile=coverage-integration.out`**. Use this profile to see coverage **with** DB and Loki paths. HTML report: **`go tool cover -html=coverage-integration.out`**.

- **`make integration-compose-up`** / **`make integration-compose-down`** — shared compose lifecycle used by **`test-integration`** and **`cover-integration`**. If a command fails or you interrupt a run, run **`make integration-compose-down`** to stop containers.

---

## Local daemon run (local-test.conf)

`local-test.conf` runs pgwd in **daemon mode** with SQLite store and HTTP server — useful to verify the full flow: multi-DB checks, SQLite persistence, `/healthz` and `/metrics` endpoints.

**Prerequisites:** Postgres stack running (all 3 databases: pgwd, analytics, replica).

```bash
docker compose -f testing/compose.yaml up -d --scale client=0
```

**Run pgwd** (from repo root):

```bash
./pgwd -config testing/local-test.conf
```

**What it does:**
- Monitors 3 databases (ports 5432, 5433, 5434)
- Stores metrics in `/tmp/pgwd-test.db` (SQLite, ncruces driver)
- Exposes HTTP on `:8080`; health and metrics at `/api/pgwd/v1/healthz` and `/api/pgwd/v1/metrics`
- `log_level: info` (default) — minimal logs; set `log_level: debug` to print `[client/database] total=X active=Y ...` every 5 seconds

**Verify endpoints:**

```bash
curl http://localhost:8080/api/pgwd/v1/healthz
# → ok

curl http://localhost:8080/api/pgwd/v1/metrics
# → Prometheus metrics (pgwd_connections_total, etc.)
```

**Port conflict:** If `:8080` is in use, edit `http.listen` in `local-test.conf` (e.g. `:8081`).

**Verbose stats:** Set `log_level: debug` in the config to print connection stats every interval.

---

## Loki (compose-loki.yaml)

Separate compose for the Loki stack. Used to validate that pgwd notifications are correctly formatted when pushed to Loki (for Grafana alerting).

**Start Loki** (from repo root):

```bash
docker compose -f testing/compose-loki.yaml up -d
```

Loki may take ~20 seconds to become ready. Check: `curl http://localhost:3100/ready` (should return `ready`).

**Run Loki integration test** (validates push + query + log format):

```bash
export PGWD_TEST_LOKI_URL="http://localhost:3100/loki/api/v1/push"
go test ./internal/notify/... -v -run TestLoki_Integration$
```

**Show payload sent and response received** (for debugging / Grafana alert rules):

```bash
export PGWD_TEST_LOKI_URL="http://localhost:3100/loki/api/v1/push"
export PGWD_TEST_LOKI_VERBOSE=1
go test ./internal/notify/... -v -run TestLoki_Integration_ShowPayload
```

Without `PGWD_TEST_LOKI_URL` the tests are skipped.

**Stop Loki:**

```bash
docker compose -f testing/compose-loki.yaml down
```

### Loki payload reference (for Grafana alerts and jq)

**Request** (POST `http://localhost:3100/loki/api/v1/push`):

```json
{
  "streams": [
    {
      "stream": {
        "app": "pgwd",
        "threshold": "total",
        "level": "attention",
        "env": "prod",
        "namespace": "mydb"
      },
      "values": [
        ["1730500000000000000", "pgwd: Total connections 16 >= 16 | total=16 active=8 idle=8 max_connections=20 (limit total=16)"]
      ]
    }
  ]
}
```

Labels: `app` (default pgwd), `threshold`, `level` (attention/alert/danger), `namespace` (when in K8s). Level mapping: `connect_failure`/`too_many_clients` → danger; `total`/`active`/`idle`/`stale`/`test` → attention.

**Response** (GET `http://localhost:3100/loki/api/v1/query_range?query={app="pgwd"}`):

```json
{
  "status": "success",
  "data": {
    "resultType": "streams",
    "result": [
      {
        "stream": {
          "app": "pgwd",
          "threshold": "total",
          "level": "attention",
          "env": "prod",
          "namespace": "mydb"
        },
        "values": [
          ["1730500000000000000", "pgwd: Total connections 16 >= 16 | total=16 active=8 idle=8 max_connections=20 (limit total=16)"]
        ]
      }
    ]
  }
}
```

**jq examples** (for alert rules or scripts):

```bash
# Extract log lines only
curl -s "http://localhost:3100/loki/api/v1/query_range?query={app=\"pgwd\"}" | jq -r '.data.result[].values[][1]'

# Extract threshold and level labels
curl -s "http://localhost:3100/loki/api/v1/query_range?query={app=\"pgwd\"}" | jq -r '.data.result[].stream | "\(.threshold) \(.level)"'

# Extract lines matching level=danger (connect_failure, too_many_clients)
curl -s "http://localhost:3100/loki/api/v1/query_range?query={app=\"pgwd\",level=\"danger\"}" | jq -r '.data.result[].values[][1]'

# Filter by namespace (K8s)
curl -s "http://localhost:3100/loki/api/v1/query_range?query={app=\"pgwd\",namespace=\"mydb\"}" | jq -r '.data.result[].values[][1]'
```

---

## E2E Kubernetes (kind)

Validates pgwd with `-kube-postgres` and `-kube-loki` against a real cluster. Creates a kind cluster, deploys 3 Postgres (pgwd, analytics, replica) and Loki, runs `pgwd -dry-run` (single DB via kube), `pgwd -config` with `databases:` (multi-DB via port-forward), and `pgwd -kube-loki -force-notification`, then destroys the cluster.

**Requires:** kind, kubectl, docker.

```bash
make test-e2e-kube
```

Or run the script directly (from repo root):

```bash
testing/scripts/test-e2e-kube.sh
```

Override cluster name: `PGWD_E2E_CLUSTER=my-cluster make test-e2e-kube`

---

**Production:** Use a non-superuser role for application connections so `superuser_reserved_connections` (default 3) stays available for DBA/admin access when the instance is saturated. See [PostgreSQL: Connection and Authentication](https://www.postgresql.org/docs/current/runtime-config-connection.html) (`superuser_reserved_connections`).
