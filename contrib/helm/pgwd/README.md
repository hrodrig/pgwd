# pgwd Helm Chart

Deploy [pgwd](https://github.com/hrodrig/pgwd) (Postgres Watch Dog) in Kubernetes to monitor PostgreSQL connection counts and notify via Slack/Loki.

## Prerequisites

- Helm 3.8+ (OCI support)
- Kubernetes 1.19+
- PostgreSQL accessible from within the cluster (in-cluster DNS)

## Installation

### From OCI registry (ghcr.io)

Charts are published to GitHub Container Registry on each release. Install with:

```bash
helm install pgwd oci://ghcr.io/hrodrig/pgwd/pgwd --version 0.5.10
```

Or add the repo and install:

```bash
helm pull oci://ghcr.io/hrodrig/pgwd/pgwd --version 0.5.10
helm install pgwd ./pgwd-0.5.10.tgz -f my-values.yaml
```

### From source (development)

1. Clone the repo and install from the chart directory:

```bash
helm install pgwd ./contrib/helm/pgwd \
  --create-namespace \
  --namespace monitoring \
  --set secrets.dbUrl="postgres://user:password@postgres.default.svc.cluster.local:5432/mydb?sslmode=disable" \
  --set secrets.slackWebhook="https://hooks.slack.com/services/YOUR/WEBHOOK/URL" \
  --set secrets.lokiUrl="http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/push" \
  --set env.PGWD_CLIENT="pgwd-prod"
```

For production, use `--set-file` or a custom values file to avoid passing secrets on the command line.

## Configuration modes

### Env vars (default)

Use `secrets.dbUrl`, `secrets.slackWebhook`, `secrets.lokiUrl` and `env.*` for single-database monitoring. Secrets are created from values (or reference `secrets.existingSecret`).

```yaml
# values.yaml
secrets:
  create: true
  dbUrl: "postgres://user:pass@postgres.default.svc.cluster.local:5432/mydb"
  slackWebhook: "https://hooks.slack.com/..."
  lokiUrl: "http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/push"

env:
  PGWD_CLIENT: "pgwd-prod"
  PGWD_INTERVAL: "60"
```

### Config file (multiple databases)

Set `config.enabled: true` and provide full YAML in `config.extra`. Use this for multiple Postgres instances. **Note:** When using a config file, env vars are ignored by pgwd.

```yaml
# values.yaml
config:
  enabled: true
  extra: |
    client: pgwd-k8s
    interval: 60
    databases:
      - url: postgres://user:pass@postgres.default.svc.cluster.local:5432/prod
        threshold:
          levels: "75,85,95"
      - url: postgres://user:pass@replica.default.svc.cluster.local:5432/prod
        threshold:
          levels: "75,85,95"
    sqlite:
      path: /var/lib/pgwd/pgwd.db
    http:
      listen: ":8080"
    notifications:
      slack:
        webhook: "https://hooks.slack.com/..."
      loki:
        url: "http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/push"

secrets:
  create: false  # URLs and webhooks are in config
```

For sensitive values in config mode, use [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) or [External Secrets](https://external-secrets.io/) to create a Secret with the config file, then mount it manually or extend the chart.

## Persistence

By default, a 1Gi PVC is created for `/var/lib/pgwd` (SQLite store). This enables resolution notifications and metrics history across restarts.

- Disable: `persistence.enabled: false` (uses emptyDir)
- Use existing PVC: `persistence.existingClaim: "my-pvc-name"`

## Service and metrics

A ClusterIP Service exposes port 8080 for `/api/pgwd/v1/healthz` and `/api/pgwd/v1/metrics`. Use for Prometheus scraping or health checks.

Optional PodMonitor (Prometheus Operator):

```yaml
podMonitor:
  enabled: true
  interval: 30s
  path: /api/pgwd/v1/metrics
```

## Values reference

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `image.repository` | Image repository | `ghcr.io/hrodrig/pgwd` |
| `image.tag` | Image tag (defaults to appVersion) | `""` |
| `secrets.create` | Create Secret from values | `true` |
| `secrets.dbUrl` | Postgres connection URL | `""` |
| `secrets.slackWebhook` | Slack webhook URL | `""` |
| `secrets.lokiUrl` | Loki push URL | `""` |
| `secrets.existingSecret` | Use existing Secret name | `""` |
| `config.enabled` | Use config file instead of env | `false` |
| `config.extra` | Full pgwd YAML config | See values.yaml |
| `persistence.enabled` | Use PVC for SQLite | `true` |
| `persistence.size` | PVC size | `1Gi` |
| `service.enabled` | Create Service | `true` |
| `resources.requests` | CPU/memory requests | `10m/32Mi` |

## Uninstall

```bash
helm uninstall pgwd --namespace monitoring
```

Note: The PVC is retained by default. Delete it manually if needed.
