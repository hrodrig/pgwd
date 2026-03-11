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

echo "Waiting for Postgres pod to be ready..."
kubectl wait --for=condition=ready pod -l app=postgres -n pgwd-e2e --timeout=120s

echo "Waiting for Loki pod to be ready..."
kubectl wait --for=condition=ready pod -l app=loki -n pgwd-e2e --timeout=120s

echo "Building pgwd..."
cd "$REPO_ROOT"
make build

echo "Running pgwd -validate-k8s-access..."
./pgwd -validate-k8s-access

echo "Running pgwd -kube-postgres with -dry-run..."
./pgwd -kube-postgres pgwd-e2e/svc/postgres \
  -kube-local-port 15432 \
  -db-url 'postgres://pgwd:DISCOVER_MY_PASSWORD@localhost:15432/pgwd?sslmode=disable' \
  -dry-run

echo "Running pgwd -kube-postgres -kube-loki with -force-notification (daemon mode to keep port-forward up)..."
./pgwd -kube-postgres pgwd-e2e/svc/postgres \
  -kube-local-port 15432 \
  -kube-loki pgwd-e2e/svc/loki \
  -kube-loki-local-port 13100 \
  -db-url 'postgres://pgwd:DISCOVER_MY_PASSWORD@localhost:15432/pgwd?sslmode=disable' \
  -force-notification \
  -interval 60 &
PGWD_PID=$!

echo "Waiting for pgwd to send notification..."
sleep 5

echo "Verifying log reached Loki..."
LOKI_RESULT=$(curl -sf "http://127.0.0.1:13100/loki/api/v1/query_range?query=%7Bapp%3D%22pgwd%22%7D&limit=1" 2>/dev/null || echo "")
echo "--- Loki query response (raw) ---"
echo "$LOKI_RESULT"
echo "--- end ---"
if [ -z "$LOKI_RESULT" ]; then
  kill $PGWD_PID 2>/dev/null || true
  echo "ERROR: Could not query Loki or no results. Push may have failed."
  exit 1
fi
if ! echo "$LOKI_RESULT" | grep -q 'pgwd'; then
  kill $PGWD_PID 2>/dev/null || true
  echo "ERROR: Loki query returned no pgwd logs. Push may have failed."
  echo "Response: $LOKI_RESULT"
  exit 1
fi

kill $PGWD_PID 2>/dev/null || true
wait $PGWD_PID 2>/dev/null || true

echo "E2E kube test passed (notification verified in Loki)."
