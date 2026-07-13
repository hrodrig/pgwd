# pgwd in Kubernetes (in-cluster deployment)

When pgwd runs **inside** Kubernetes as a Deployment, use **direct service URLs**. No kubectl or port-forward needed.

## Helm

The maintained chart is in **[pgwd-selfhosted](https://github.com/hrodrig/pgwd-selfhosted)** (`run/kubernetes/helm/pgwd/`). See **[contrib/HELM.md](../HELM.md)** for links and migration notes (OCI chart publishing moved there).

## Raw manifests

1. Create a Secret with the DB URL (or individual credentials):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: pgwd-db
  namespace: default
type: Opaque
stringData:
  url: "postgres://postgres:YOUR_PASSWORD@postgres.default.svc.cluster.local:5432/mydb?sslmode=disable"
```

2. Create a Secret for Slack webhook (or Loki URL):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: pgwd-secrets
  namespace: default
type: Opaque
stringData:
  slack-webhook: "https://hooks.slack.com/services/..."
```

3. Deploy pgwd:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pgwd
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: pgwd
  template:
    metadata:
      labels:
        app: pgwd
    spec:
      containers:
        - name: pgwd
          image: ghcr.io/hrodrig/pgwd:latest
          env:
            - name: PGWD_DB_URL
              valueFrom:
                secretKeyRef:
                  name: pgwd-db
                  key: url
            - name: PGWD_CLIENT
              value: "pgwd-prod"
            - name: PGWD_INTERVAL
              value: "60"
            - name: PGWD_NOTIFICATIONS_SLACK_WEBHOOK
              valueFrom:
                secretKeyRef:
                  name: pgwd-secrets
                  key: slack-webhook
            - name: PGWD_NOTIFICATIONS_LOKI_URL
              value: "http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/push"
            - name: PGWD_HTTP_LISTEN
              value: ":8080"
            - name: PGWD_SQLITE_PATH
              value: "/var/lib/pgwd/pgwd.db"
          ports:
            - containerPort: 8080
              name: http
          volumeMounts:
            - name: data
              mountPath: /var/lib/pgwd
          livenessProbe:
            httpGet:
              path: /api/pgwd/v1/healthz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            requests:
              memory: "32Mi"
              cpu: "10m"
      volumes:
        - name: data
          emptyDir: {}
```

## Environment variables (when no config file)

| Env | Purpose |
|-----|---------|
| `PGWD_DB_URL` | Postgres URL with in-cluster host, e.g. `postgres://...@postgres.namespace.svc.cluster.local:5432/mydb` |
| `PGWD_CLIENT` | Monitor identity (required) |
| `PGWD_INTERVAL` | Seconds between checks (e.g. 60 for daemon) |
| `PGWD_NOTIFICATIONS_SLACK_WEBHOOK` | Slack webhook URL |
| `PGWD_NOTIFICATIONS_LOKI_URL` | Loki push URL, e.g. `http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/push` |
| `PGWD_HTTP_LISTEN` | HTTP server bind, e.g. `:8080` for /healthz and /metrics |
| `PGWD_SQLITE_PATH` | Path for metrics store (resolution notifications, /metrics export) |

When `PGWD_HTTP_LISTEN` is set, default paths are `/api/pgwd/v1/healthz` and `/api/pgwd/v1/metrics`.

## HTTP `/metrics` privacy (opt-in)

**Default: no credentials.** Leave `http.metrics_token` and `http.metrics_basic_*` empty (or unset). Prometheus, Grafana Alloy, and kube probes behave as before — anonymous `GET /metrics`, open `GET /healthz`.

| Setting | Effect |
|---------|--------|
| *(empty)* | In-cluster scrape without auth — typical `ServiceMonitor` / `PodMonitor` setup |
| `http.metrics_token` | `/metrics` requires `Authorization: Bearer <token>` or `?token=`; `/healthz` unchanged |
| `http.metrics_basic_user` + `password` | `/metrics` requires HTTP Basic auth; `/healthz` unchanged |

**When to enable:** pgwd listens on a host network, VM, or `port-forward` where firewall / NetworkPolicy alone is not enough. **In-cluster:** prefer ClusterIP + NetworkPolicy; auth is optional extra.

**Prometheus scrape with token** (only if you set `metrics_token`):

```yaml
# prometheus.yml excerpt
authorization:
  credentials: "<same value as http.metrics_token>"
```

Env equivalents: `PGWD_HTTP_METRICS_TOKEN`, `PGWD_HTTP_METRICS_BASIC_USER`, `PGWD_HTTP_METRICS_BASIC_PASSWORD`.

## pgwd outside the cluster

When pgwd runs on a **VM, bare metal, or cron** and Postgres/Loki are **inside** Kubernetes, use **`-kube-postgres`** / **`-kube-loki`** with client-go port-forward (kubeconfig required; **no kubectl binary inside pgwd**).

**`DISCOVER_MY_PASSWORD` was removed in 0.9.x.** Do not put it in the DSN. Use one of the supported paths below.

**Full migration guide (copy-paste recipes, RBAC, troubleshooting):** **[docs/kubernetes-passwords.md](../docs/kubernetes-passwords.md)**

### Quick start — `kube.password_from_secret` (recommended daemon)

1. Copy profile **[contrib/profiles/kube-prod.yml](../profiles/kube-prod.yml)** and edit Secret name, `kube.postgres`, `db.url`.
2. Apply RBAC: **`kubectl apply -f contrib/k8s/rbac-outside-cluster.yaml`** (edit `resourceNames` first).
3. Run: `pgwd -config /etc/pgwd/pgwd.conf -dry-run`

```yaml
# excerpt — see kube-prod.yml for full example
kube:
  postgres: default/svc/postgres
  local_port: 5432
  password_from_secret:
    namespace: default
    name: postgres-credentials
    key: password
db:
  url: postgres://postgres@127.0.0.1:5432/mydb?sslmode=disable
```

### Quick start — wrapper script (cron / no pgwd Secrets RBAC)

```bash
SECRET_NS=default SECRET_NAME=postgres-credentials SECRET_KEY=password \
  DB_USER=postgres DB_NAME=mydb \
  ./contrib/k8s/pgwd-kube-run.sh \
    -kube-postgres default/svc/postgres -client prod -interval 60 \
    -notifications-slack-webhook "$WEBHOOK"
```

Requires **kubectl** on the host for the script only.

### Quick start — manual Secret read (one-shot)

```bash
PGPASSWORD="$(kubectl get secret postgres-credentials -n default -o jsonpath='{.data.password}' | base64 -d)"
PGWD_DB_URL="postgres://postgres:${PGPASSWORD}@localhost:5432/mydb?sslmode=disable" \
  pgwd -kube-postgres default/svc/postgres -client prod -dry-run
```

### RBAC summary

| Path | pgwd ServiceAccount needs |
|------|---------------------------|
| `password_from_secret` | `secrets/get` on **one** named Secret — [rbac-outside-cluster.yaml](rbac-outside-cluster.yaml) |
| Wrapper / manual kubectl | Nothing on pgwd; operator kubeconfig reads Secret |
| In-cluster Deployment | `secretKeyRef` in Pod spec — no `-kube-postgres` |

**Do not grant `pods/exec`** for password retrieval — that was the removed DISCOVER path.

Validate before cutover: `pgwd -validate-k8s-access` and `pgwd -config … -dry-run`.

## Multiple databases (in-cluster)

**Use case:** one pgwd Deployment monitors **N Postgres** instances with **different credentials**.

- Config: **`databases:`** — one **full DSN per entry** (`user`, `password`, in-cluster host).
- **Do not** set `kube.postgres` (validation rejects `kube.postgres` + `databases:`).
- **Do not** rely on `kube.password_from_secret` for N Secrets — inject each URL at deploy time.

### Deploy pattern

1. One Secret per database (or one Secret with multiple keys: `url_prod`, `url_analytics`, …).
2. Helm / Kustomize / External Secrets renders `pgwd.conf` into a ConfigMap or Secret volume.
3. Mount config; set `PGWD_CONFIG=/etc/pgwd/pgwd.conf`, `PGWD_INTERVAL=60`.
4. Set **unique `client` per entry** when the same DB name exists on different hosts.

```yaml
# Rendered pgwd.conf (passwords from Secrets at deploy — not in git)
client: pgwd-prod
interval: 60
databases:
  - url: postgres://user1:PASS1@postgres-a.default.svc.cluster.local:5432/prod?sslmode=disable
    client: pgwd-prod-a
  - url: postgres://user2:PASS2@postgres-b.default.svc.cluster.local:5432/analytics?sslmode=disable
    client: pgwd-prod-b
sqlite:
  path: /var/lib/pgwd/pgwd.db
```

**Full walkthrough:** [docs/use-cases.md UC-5](../docs/use-cases.md#uc-5--multi-database-in-cluster-n-different-credentials).  
**Profile:** [contrib/profiles/multi-db.yml](../profiles/multi-db.yml).

## Multiple databases (outside cluster)

N Services in K8s, pgwd on a VM: **N port-forwards** + `databases:` with `127.0.0.1` and distinct local ports — or **N pgwd processes** (one `kube-prod.yml` each).

**Full walkthrough:** [docs/use-cases.md UC-6](../docs/use-cases.md#uc-6--multi-database-outside-cluster-n-port-forwards). E2E: `testing/scripts/test-e2e-kube.sh`.

## Operator use-case index

All scenarios (cron, daemon, single/multi DB, in/out of cluster): **[docs/use-cases.md](../docs/use-cases.md)**.

## Persistence

For resolution notifications and metrics history across restarts, use a PersistentVolumeClaim for `/var/lib/pgwd` instead of `emptyDir`.
