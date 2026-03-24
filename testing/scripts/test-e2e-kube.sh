#!/usr/bin/env bash
# E2E test: pgwd with -kube-postgres and -kube-loki against a kind cluster.
# Creates cluster, deploys Postgres and Loki, runs pgwd -dry-run and -force-notification, destroys cluster.
# Requires: kind, kubectl, docker
set -e

CLUSTER_NAME="${PGWD_E2E_CLUSTER:-pgwd-e2e}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
K8S_DIR="$REPO_ROOT/testing/k8s"

cleanup() {
  echo "Cleaning up: kind delete cluster --name $CLUSTER_NAME"
  kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
}
trap cleanup EXIT

echo "Creating kind cluster: $CLUSTER_NAME"
kind create cluster --name "$CLUSTER_NAME" --wait 60s

echo "Deploying Postgres..."
kubectl apply -f "$K8S_DIR/postgres.yaml"

echo "Deploying Loki..."
kubectl apply -f "$K8S_DIR/loki.yaml"

echo "Waiting for Postgres pods to be ready..."
kubectl wait --for=condition=ready pod -l app=postgres -n pgwd-e2e --timeout=120s
kubectl wait --for=condition=ready pod -l app=postgres2 -n pgwd-e2e --timeout=120s
kubectl wait --for=condition=ready pod -l app=postgres3 -n pgwd-e2e --timeout=120s

echo "Waiting for Loki pod to be ready..."
kubectl wait --for=condition=ready pod -l app=loki -n pgwd-e2e --timeout=120s

echo "Building pgwd..."
cd "$REPO_ROOT"
make build

echo "Running pgwd -validate-k8s-access..."
./pgwd -validate-k8s-access

echo "Running pgwd -kube-postgres with -dry-run (DISCOVER_MY_PASSWORD)..."
# SPDY exec for password discovery can be flaky on freshly-created Kind clusters;
# retry up to 3 times with a short pause between attempts.
DISCOVER_OK=0
for attempt in 1 2 3; do
  if ./pgwd -client pgwd-e2e-test \
    -kube-postgres pgwd-e2e/svc/postgres \
    -kube-local-port 15432 \
    -db-url 'postgres://pgwd:DISCOVER_MY_PASSWORD@localhost:15432/pgwd?sslmode=disable' \
    -dry-run; then
    DISCOVER_OK=1
    break
  fi
  echo "  password discovery attempt $attempt/3 failed, retrying in 3s..."
  sleep 3
done
if [ "$DISCOVER_OK" -ne 1 ]; then
  echo "ERROR: DISCOVER_MY_PASSWORD dry-run failed after 3 attempts"
  exit 1
fi

echo "Running pgwd multi-database (databases: 3 Postgres via port-forward)..."
PF1_PID=""
PF2_PID=""
PF3_PID=""
kubectl port-forward -n pgwd-e2e svc/postgres 15432:5432 &
PF1_PID=$!
kubectl port-forward -n pgwd-e2e svc/postgres2 15433:5432 &
PF2_PID=$!
kubectl port-forward -n pgwd-e2e svc/postgres3 15434:5432 &
PF3_PID=$!
kill_pf() {
  [ -n "$PF1_PID" ] && kill $PF1_PID 2>/dev/null || true
  [ -n "$PF2_PID" ] && kill $PF2_PID 2>/dev/null || true
  [ -n "$PF3_PID" ] && kill $PF3_PID 2>/dev/null || true
}
trap kill_pf EXIT
sleep 3
MULTIDB_CONF=$(mktemp)
cat > "$MULTIDB_CONF" << 'MULTIDBCONF'
client: pgwd-e2e-multidb
interval: 0
dry_run: true
databases:
  - url: postgres://pgwd:pgwd@localhost:15432/pgwd?sslmode=disable
    client: pgwd-e2e-multidb-pgwd
  - url: postgres://pgwd:pgwd@localhost:15433/analytics?sslmode=disable
    client: pgwd-e2e-multidb-analytics
  - url: postgres://pgwd:pgwd@localhost:15434/replica?sslmode=disable
    client: pgwd-e2e-multidb-replica
MULTIDBCONF
./pgwd -config "$MULTIDB_CONF" -dry-run -interval 0 || { rm -f "$MULTIDB_CONF"; exit 1; }
rm -f "$MULTIDB_CONF"
trap - EXIT
kill_pf

echo "Running pgwd -kube-postgres -kube-loki with -force-notification (daemon mode to keep port-forward up)..."
./pgwd -client pgwd-e2e-test \
  -kube-postgres pgwd-e2e/svc/postgres \
  -kube-local-port 15432 \
  -kube-loki pgwd-e2e/svc/loki \
  -kube-loki-local-port 13100 \
  -db-url 'postgres://pgwd:pgwd@localhost:15432/pgwd?sslmode=disable' \
  -force-notification \
  -interval 60 &
PGWD_PID=$!

echo "Waiting for pgwd to send notification and Loki to index..."
# Loki may need extra time to ingest and make the log queryable; poll with backoff.
LOKI_OK=0
for wait_secs in 5 5 10 10; do
  sleep "$wait_secs"
  echo "  querying Loki (after ${wait_secs}s wait)..."
  LOKI_RESULT=$(curl -sf "http://127.0.0.1:13100/loki/api/v1/query_range?query=%7Bapp%3D%22pgwd%22%7D&limit=1" 2>/dev/null || echo "")
  if [ -n "$LOKI_RESULT" ] && echo "$LOKI_RESULT" | grep -q 'pgwd'; then
    LOKI_OK=1
    echo "--- Loki query response (raw) ---"
    echo "$LOKI_RESULT"
    echo "--- end ---"
    break
  fi
done
if [ "$LOKI_OK" -ne 1 ]; then
  kill $PGWD_PID 2>/dev/null || true
  echo "--- Loki query response (raw) ---"
  echo "$LOKI_RESULT"
  echo "--- end ---"
  echo "ERROR: Could not verify pgwd logs in Loki after multiple attempts."
  exit 1
fi

kill $PGWD_PID 2>/dev/null || true
wait $PGWD_PID 2>/dev/null || true

echo "E2E kube test passed (notification verified in Loki)."
