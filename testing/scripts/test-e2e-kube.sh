#!/usr/bin/env bash
# E2E test: pgwd with -kube-postgres against a kind cluster.
# Creates cluster, deploys Postgres, runs pgwd -dry-run, destroys cluster.
# Requires: kind, kubectl, docker
set -e

CLUSTER_NAME="${PGWD_E2E_CLUSTER:-pgwd-e2e}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
K8S_MANIFEST="$REPO_ROOT/testing/k8s/postgres.yaml"

cleanup() {
  echo "Cleaning up: kind delete cluster --name $CLUSTER_NAME"
  kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
}
trap cleanup EXIT

echo "Creating kind cluster: $CLUSTER_NAME"
kind create cluster --name "$CLUSTER_NAME" --wait 60s

echo "Deploying Postgres..."
kubectl apply -f "$K8S_MANIFEST"

echo "Waiting for Postgres pod to be ready..."
kubectl wait --for=condition=ready pod -l app=postgres -n pgwd-e2e --timeout=120s

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

echo "E2E kube test passed."
