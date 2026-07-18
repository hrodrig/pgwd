# pgwd — operator use cases

Copy-paste starting points for common deployments. Profiles live under **[contrib/profiles/](../contrib/profiles/)**. K8s credential patterns (including DISCOVER migration): **[kubernetes-passwords.md](./kubernetes-passwords.md)**.

---

## Choose your scenario

| ID | You want… | Where pgwd runs | DB count | Credentials | Start here |
|----|-----------|-----------------|----------|-------------|------------|
| **UC-1** | Cron / one-shot alert | Any host | 1 | Direct URL or env | [minimal-slack.yml](../contrib/profiles/minimal-slack.yml) |
| **UC-2** | Daemon + Loki + `/metrics` | Any host | 1 | Direct URL | [daemon-loki.yml](../contrib/profiles/daemon-loki.yml) |
| **UC-3** | Daemon, Postgres **in** K8s | **Inside** cluster | 1 | `secretKeyRef` → env or config | [contrib/k8s/README.md](../contrib/k8s/README.md) |
| **UC-4** | Daemon, Postgres **in** K8s | **Outside** cluster (VM) | 1 | `kube.password_from_secret` or wrapper | [kube-prod.yml](../contrib/profiles/kube-prod.yml), [kubernetes-passwords.md](./kubernetes-passwords.md) |
| **UC-5** | Daemon, **N Postgres**, different creds | **Inside** cluster | N | N URLs in `databases:` (inject at deploy) | [UC-5 below](#uc-5--multi-database-in-cluster-n-different-credentials) |
| **UC-6** | Daemon, **N Postgres**, different creds | **Outside** cluster | N | N port-forwards + `databases:` | [UC-6 below](#uc-6--multi-database-outside-cluster-n-port-forwards) |
| **UC-7** | Cron, **N Postgres**, diverse setups | Mixed | N | One config **or** one cron line per instance | [UC-7 below](#uc-7--multi-database-cron--one-config-per-instance) |

---

## Not supported in 0.9.x

| Request | Result | Workaround |
|---------|--------|------------|
| `kube.postgres` + `databases:` in **one** process | Validation **error** | UC-6 (port-forwards + `databases:`) or UC-7 (N processes) |
| `kube.password_from_secret` for **multiple** Secrets | **One** Secret per config | Full DSN per `databases[].url`, or UC-7 |
| Per-database `kube.postgres` inside `databases:` | Not implemented | Roadmap post-1.0 — [ROADMAP.md](../ROADMAP.md) |
| `DISCOVER_MY_PASSWORD` | Removed | [kubernetes-passwords.md](./kubernetes-passwords.md) |

**SQLite / hysteresis:** rows keyed by `(client, cluster, database)` — **not** URL host. Use a **unique `client` per `databases:` entry** when the same DB name appears on different hosts.

---

## UC-1 — One-shot / cron, single database

**Profile:** [minimal-slack.yml](../contrib/profiles/minimal-slack.yml)

```yaml
client: "my-monitor"
interval: 0          # one-shot; cron re-invokes
databases:
  - url: postgres://USER:PASS@db.example.com:5432/mydb?sslmode=disable
notifications:
  slack:
    webhook: "https://hooks.slack.com/services/..."
```

```bash
*/5 * * * * pgwd -config /etc/pgwd/minimal-slack.yml >> /var/log/pgwd.log 2>&1
```

---

## UC-2 — Daemon, single DB, observability stack

**Profile:** [daemon-loki.yml](../contrib/profiles/daemon-loki.yml)

One Postgres over **direct TCP** (VM, RDS, in-cluster DNS). Daemon gets SQLite history, resolution alerts, Loki, `/metrics`.

---

## UC-3 — Single DB, pgwd inside Kubernetes

**Doc:** [contrib/k8s/README.md](../contrib/k8s/README.md) (in-cluster Deployment)

- Connect with **service DNS** in the URL — **no** `-kube-postgres`.
- Password from **`secretKeyRef`** (never in git).

```yaml
env:
  - name: PGWD_DB_URL
    valueFrom:
      secretKeyRef:
        name: pgwd-db
        key: url
  - name: PGWD_INTERVAL
    value: "60"
```

Helm: [pgwd-selfhosted](https://github.com/hrodrig/pgwd-selfhosted).

---

## UC-4 — Single DB, pgwd outside Kubernetes (port-forward)

**Profile:** [kube-prod.yml](../contrib/profiles/kube-prod.yml)  
**Guide:** [kubernetes-passwords.md](./kubernetes-passwords.md) Options 1–4

One Postgres service in the cluster; pgwd on a VM uses **client-go port-forward** + **one** credential source:

| Style | Best for |
|-------|----------|
| `kube.password_from_secret` | Long-lived systemd daemon; pgwd calls `secrets/get` |
| `contrib/k8s/pgwd-kube-run.sh` | Cron; operator `kubectl` reads Secret |
| Manual `kubectl get secret` | Ad-hoc / scripts |

---

## UC-5 — Multi-database, in-cluster, N different credentials

**Profile base:** [multi-db.yml](../contrib/profiles/multi-db.yml)  
**Pattern:** one pgwd Deployment, **`databases:`** with **full DSN per entry** (each user/password/host distinct).

pgwd does **not** read multiple Secrets by itself. **Inject** URLs at deploy time (Helm, Kustomize, External Secrets, CI) — same as any app config with secrets.

### Example config (rendered into ConfigMap / Secret — do not commit passwords)

```yaml
client: "pgwd-prod"
interval: 60

databases:
  - url: postgres://app_prod:{{ from secret pgwd-dsn-prod }}@postgres-prod.default.svc.cluster.local:5432/prod?sslmode=disable
    client: pgwd-prod-primary
    threshold:
      levels: "75,85,95"
  - url: postgres://app_analytics:{{ from secret pgwd-dsn-analytics }}@postgres-analytics.monitoring.svc:5432/analytics?sslmode=disable
    client: pgwd-prod-analytics
    threshold:
      levels: "80,90,98"

sqlite:
  path: /var/lib/pgwd/pgwd.db

notifications:
  slack:
    webhook: "https://hooks.slack.com/services/..."
```

### Helm-style deploy sketch

1. Store **one Secret per database** (or one Secret with keys `url_prod`, `url_analytics`).
2. Template `pgwd.conf` in the chart: `databases[].url` from `.Values` populated from those Secrets.
3. Mount config: `PGWD_CONFIG=/etc/pgwd/pgwd.conf`.
4. **No** `kube.postgres` — URLs use **in-cluster DNS**.

See [contrib/k8s/README.md — Multiple databases](../contrib/k8s/README.md#multiple-databases-in-cluster).

---

## UC-6 — Multi-database, outside cluster, N port-forwards

**When:** several Postgres **Services** inside K8s; pgwd on a **VM**; you want **one daemon** checking all.

**Cannot use:** `-kube-postgres` + `databases:` together.  
**Use:** **N port-forwards** (distinct local ports) + **`databases:`** with `localhost` URLs.

### 1. Port-forwards (systemd, supervisor, or script)

```bash
kubectl port-forward -n app svc/postgres-a 15432:5432 &
kubectl port-forward -n app svc/postgres-b 15433:5432 &
kubectl port-forward -n analytics svc/postgres-c 15434:5432 &
```

### 2. Build config with **different credentials per URL**

Read each Secret before starting pgwd (deploy script, envsubst, or configuration management):

```yaml
client: "pgwd-outside"
interval: 60

databases:
  - url: postgres://user_a:SECRET_A@127.0.0.1:15432/db_a?sslmode=disable
    client: pgwd-outside-db-a
  - url: postgres://user_b:SECRET_B@127.0.0.1:15433/db_b?sslmode=disable
    client: pgwd-outside-db-b
  - url: postgres://user_c:SECRET_C@127.0.0.1:15434/db_c?sslmode=disable
    client: pgwd-outside-db-c

notifications:
  slack:
    webhook: "https://hooks.slack.com/services/..."
```

### 3. Example: render config from three Secrets

```bash
PASS_A=$(kubectl get secret creds-a -n app -o jsonpath='{.data.password}' | base64 -d)
PASS_B=$(kubectl get secret creds-b -n app -o jsonpath='{.data.password}' | base64 -d)
PASS_C=$(kubectl get secret creds-c -n analytics -o jsonpath='{.data.password}' | base64 -d)

envsubst < /etc/pgwd/pgwd.conf.template > /run/pgwd/pgwd.conf
pgwd -config /run/pgwd/pgwd.conf
```

E2E reference: `testing/scripts/test-e2e-kube.sh` (three port-forwards + `databases:` dry-run).

### Alternative: N pgwd processes (simpler ops)

One **[kube-prod.yml](../contrib/profiles/kube-prod.yml)** per Postgres service — each with its own `kube.postgres`, `password_from_secret`, and **`kube.local_port`**. No shared `databases:`.

---

## UC-7 — Multi-database cron / one config per instance

**When:** instances differ by **cluster**, **kube context**, or you prefer **isolation** over one fat config.

```bash
*/5 * * * * pgwd -config /etc/pgwd/prod-db1.conf >> /var/log/pgwd-prod-db1.log 2>&1
*/5 * * * * pgwd -config /etc/pgwd/analytics.conf >> /var/log/pgwd-analytics.log 2>&1
```

Each file can be UC-1 (direct URL), UC-4 (`kube-prod`), etc. See [README — Example: multiple services](../README.md#example-multiple-services-and-heartbeat-via-bash--cron).

---

## Quick reference: config shape

| Mode | Config keys | Password |
|------|-------------|----------|
| Single DB, direct | `databases:` (one entry) | In each `url` or env `PGWD_DB_URL` |
| Single DB, outside K8s | `kube.postgres` + `db.url` or `databases:` (one entry) | `password_from_secret`, wrapper, or URL |
| Multi DB | `databases:` only (no `kube.postgres`) | **Per-entry `url`** (inject at deploy) |

---

## Related docs

| Doc | Contents |
|-----|----------|
| [kubernetes-passwords.md](./kubernetes-passwords.md) | DISCOVER migration, single-DB outside-cluster credentials, RBAC |
| [contrib/k8s/README.md](../contrib/k8s/README.md) | In-cluster Deployment, outside-cluster quick starts, multi-DB |
| [contrib/profiles/README.md](../contrib/profiles/README.md) | Profile index |
| [SPECIFICATIONS.md](../SPECIFICATIONS.md) | Behavior contract, `databases:` fields |
| [README.md](../README.md) | Full CLI, multi-database limitations |
