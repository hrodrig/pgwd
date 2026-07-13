#!/usr/bin/env bash
# Run pgwd outside Kubernetes with -kube-postgres port-forward.
# Reads the Postgres password from a Secret via kubectl (operator RBAC), never from git.
#
# Required env (or defaults):
#   SECRET_NS, SECRET_NAME, SECRET_KEY (default: password)
#   DB_USER, DB_NAME, DB_PORT (default 5432), KUBE_LOCAL_PORT (default 5432)
#
# Example:
#   SECRET_NS=default SECRET_NAME=postgres-credentials SECRET_KEY=password \
#     DB_USER=postgres DB_NAME=mydb \
#     ./contrib/k8s/pgwd-kube-run.sh \
#       -kube-postgres default/svc/postgres \
#       -client prod -interval 60 \
#       -notifications-slack-webhook "$WEBHOOK"
set -euo pipefail

SECRET_NS="${SECRET_NS:-default}"
SECRET_NAME="${SECRET_NAME:?SECRET_NAME is required}"
SECRET_KEY="${SECRET_KEY:-password}"
DB_USER="${DB_USER:?DB_USER is required}"
DB_NAME="${DB_NAME:?DB_NAME is required}"
DB_PORT="${DB_PORT:-5432}"
KUBE_LOCAL_PORT="${KUBE_LOCAL_PORT:-5432}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "pgwd-kube-run: kubectl not found on PATH" >&2
  exit 1
fi

password="$(kubectl -n "$SECRET_NS" get secret "$SECRET_NAME" -o "jsonpath={.data.${SECRET_KEY}}" | base64 -d)"
if [ -z "$password" ]; then
  echo "pgwd-kube-run: secret $SECRET_NS/$SECRET_NAME key $SECRET_KEY is empty or missing" >&2
  exit 1
fi

# URL-encode password minimally for postgres DSN (avoid breaking on @ : /)
encoded_pw="$password"
if [[ "$password" == *[@:/]* ]]; then
  encoded_pw="$(python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=''))" "$password")"
fi

export PGWD_DB_URL="postgres://${DB_USER}:${encoded_pw}@localhost:${KUBE_LOCAL_PORT}/${DB_NAME}?sslmode=disable"
export PGWD_KUBE_LOCAL_PORT="$KUBE_LOCAL_PORT"

exec pgwd "$@"
