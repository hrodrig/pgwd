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

## pgwd outside the cluster

When pgwd runs on a VM or cron host, use `-kube-postgres` / `-kube-loki` with client-go port-forward (kubeconfig required; no kubectl binary). **Do not use `DISCOVER_MY_PASSWORD`** in the DSN — it requires `pods/exec` RBAC and will be **removed in 0.9.x**. Rationale and migration: **[docs/kubernetes-passwords.md](../docs/kubernetes-passwords.md)**.

## Multiple databases

Use a config file (ConfigMap) with `databases:` — env vars do not support multiple URLs. Mount the config and set `PGWD_CONFIG` or `-config`. Template the config with Helm/Kustomize to inject the DB URL(s) from Secrets.

## Persistence

For resolution notifications and metrics history across restarts, use a PersistentVolumeClaim for `/var/lib/pgwd` instead of `emptyDir`.
