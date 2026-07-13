# Kubernetes passwords — operator guide (0.9.x)

**Status:** **`DISCOVER_MY_PASSWORD` removed in pgwd 0.9.x.** pgwd no longer runs `printenv` inside Postgres pods (`pods/exec`).

**All deployment scenarios (single DB, multi-DB, in/out of cluster):** **[use-cases.md](./use-cases.md)** — start there if unsure which pattern fits.

If your URL or config still contains the literal `DISCOVER_MY_PASSWORD`, pgwd **exits at startup** with:

```text
DISCOVER_MY_PASSWORD was removed in pgwd 0.9.x. Migration: docs/kubernetes-passwords.md — use kube.password_from_secret (contrib/profiles/kube-prod.yml), contrib/k8s/pgwd-kube-run.sh, or PGWD_DB_URL from kubectl get secret
```

This page covers **credentials for Kubernetes-related setups** (especially **single-DB outside-cluster port-forward**). Multi-database patterns are in **[use-cases.md](./use-cases.md)** (UC-5, UC-6).

---

## Scenario map (credentials only)

| Scenario | Doc | Credential pattern |
|----------|-----|-------------------|
| 1 DB, outside K8s, daemon | Options 1–2 below | `password_from_secret` or wrapper |
| 1 DB, outside K8s, cron | Options 2, 4 | `kubectl get secret` → `PGWD_DB_URL` |
| 1 DB, inside K8s | Option 3 | `secretKeyRef` in Pod |
| **N DBs, different passwords** | **[use-cases.md UC-5 / UC-6](./use-cases.md)** | Full DSN per `databases[].url` — **not** `kube.password_from_secret` × N |
| N DBs, outside K8s | **[UC-6](./use-cases.md#uc-6--multi-database-outside-cluster-n-port-forwards)** | N port-forwards + `databases:` **or** N pgwd processes (UC-4 each) |

**Hard rule (0.9.x):** `kube.postgres` and `databases:` (2+ targets) **cannot** coexist in one config. Multi-DB always uses **direct URLs** (in-cluster DNS, TCP, or `localhost` after port-forward).

---

## Choose your path (single database, outside cluster)

| Where pgwd runs | Recommended approach | pgwd needs `pods/exec` | pgwd needs `secrets get` |
|-----------------|----------------------|------------------------|---------------------------|
| **Inside the cluster** (Deployment) | Secret → env (`PGWD_DB_URL`) + in-cluster DNS — [Option 3](#option-3--in-cluster-deployment-no-port-forward) | No | No |
| **Outside** — long-lived daemon | **`kube.password_from_secret`** in config — [Option 1](#option-1--kubepassword_from_secret-outside-cluster-daemon) | No | **Yes** (one named Secret) |
| **Outside** — cron / one-shot | **`contrib/k8s/pgwd-kube-run.sh`** or shell + `kubectl get secret` — [Options 2, 4](#option-2--wrapper-script-outside-cluster-cron) | No | No (operator identity reads Secret) |

Copy-paste recipes below. Ready-made profile: **[contrib/profiles/kube-prod.yml](../contrib/profiles/kube-prod.yml)**.

---

## Multiple databases with different credentials

**Supported:** one daemon, `databases:` with **a full `postgres://user:pass@host:port/db` per entry**. Each entry may use a **different user and password**.

**Not supported:** one `kube.password_from_secret` block for multiple Secrets, or `kube.postgres` alongside `databases:`.

| Where pgwd runs | Pattern |
|-----------------|---------|
| **Inside K8s** | Template `databases:` at deploy (Helm/Kustomize) from N Secrets — [UC-5](./use-cases.md#uc-5--multi-database-in-cluster-n-different-credentials) |
| **Outside K8s** | N `kubectl port-forward` (ports 15432, 15433, …) + `databases:` with `127.0.0.1` and per-URL passwords — [UC-6](./use-cases.md#uc-6--multi-database-outside-cluster-n-port-forwards) |
| **Simpler outside K8s** | N pgwd processes, each [kube-prod.yml](../contrib/profiles/kube-prod.yml) — [UC-7](./use-cases.md#uc-7--multi-database-cron--one-config-per-instance) |

Profile: **[contrib/profiles/multi-db.yml](../contrib/profiles/multi-db.yml)** (direct TCP example; swap hosts to `127.0.0.1` + ports when using port-forward).

```yaml
# Excerpt — each url may use a different user:password
databases:
  - url: postgres://app1:SECRET_A@postgres-a.svc.cluster.local:5432/prod?sslmode=disable
    client: monitor-prod
  - url: postgres://app2:SECRET_B@postgres-b.svc.cluster.local:5432/analytics?sslmode=disable
    client: monitor-analytics
```

Never commit secrets. Inject `SECRET_*` at deploy (CI, Helm, `envsubst`, Ansible).

---

## Option 1 — `kube.password_from_secret` (outside cluster, daemon)

Best when pgwd runs as a **systemd service or long-lived process** on a VM and you want the password in config **without** putting it in git.

### 1. Find the Secret your Postgres chart uses

```bash
# Common patterns — adjust namespace and labels
kubectl get secrets -n default | grep -i postgres
kubectl get secret postgres-credentials -n default -o jsonpath='{.data}' | jq 'keys'
```

Note **namespace**, **Secret name**, and **key** (`password`, `postgres-password`, or `url` for a full DSN).

### 2. Replace your old config

**Before (0.8.x — fails on 0.9.x):**

```yaml
kube:
  postgres: default/svc/postgres
db:
  url: "postgres://postgres:DISCOVER_MY_PASSWORD@localhost:5432/mydb?sslmode=disable"
```

**After (0.9.x):**

```yaml
kube:
  postgres: default/svc/postgres
  local_port: 5432
  password_from_secret:
    namespace: default
    name: postgres-credentials
    key: password          # or "url" if the Secret holds a full postgres:// DSN
db:
  url: "postgres://postgres@127.0.0.1:5432/mydb?sslmode=disable"
```

- Leave the **password empty** in `db.url` when `key: password` — pgwd injects it after reading the Secret.
- Host must be **`127.0.0.1` or `localhost`**; port must match **`kube.local_port`** (port-forward target).

### 3. Apply RBAC (scoped `secrets/get` only)

Edit **`contrib/k8s/rbac-outside-cluster.yaml`** — set `resourceNames` to your Secret — then:

```bash
kubectl apply -f contrib/k8s/rbac-outside-cluster.yaml
```

pgwd’s kubeconfig must use the ServiceAccount token (or a user bound to that Role). **No `pods/exec`** is required.

### 4. Verify

```bash
pgwd -kube-context YOUR_CTX -validate-k8s-access
pgwd -config /etc/pgwd/pgwd.conf -dry-run
```

---

## Option 2 — Wrapper script (outside cluster, cron)

Best when you prefer **operator / CI RBAC** (`kubectl get secret`) and pgwd itself should **not** call the Secrets API.

```bash
export KUBECONFIG=/path/to/kubeconfig
export SECRET_NS=default
export SECRET_NAME=postgres-credentials
export SECRET_KEY=password
export DB_USER=postgres
export DB_NAME=mydb

./contrib/k8s/pgwd-kube-run.sh \
  -kube-postgres default/svc/postgres \
  -client prod \
  -interval 60 \
  -notifications-slack-webhook "$WEBHOOK"
```

The script reads the Secret with **kubectl**, builds `PGWD_DB_URL`, and execs pgwd. Requires **kubectl on PATH** for the wrapper only — pgwd still uses client-go for port-forward.

---

## Option 3 — In-cluster Deployment (no port-forward)

Best for **daemon mode** inside Kubernetes. Connect with **service DNS**; no `-kube-postgres`.

```yaml
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
```

Full manifest examples: **[contrib/k8s/README.md](../contrib/k8s/README.md)** and **[pgwd-selfhosted](https://github.com/hrodrig/pgwd-selfhosted)** Helm chart.

---

## Option 4 — Manual env injection (cron / ad hoc)

Same security model as the wrapper — **you** read the Secret once per run:

```bash
export KUBECONFIG=/path/to/kubeconfig
PGPASSWORD="$(kubectl get secret postgres-credentials -n default \
  -o jsonpath='{.data.password}' | base64 -d)"
export PGWD_DB_URL="postgres://postgres:${PGPASSWORD}@localhost:5432/mydb?sslmode=disable"

pgwd -kube-postgres default/svc/postgres \
  -client prod -interval 0 -dry-run
```

Never commit the password. Use cron with a restricted kubeconfig or wrapper script.

---

## Migration checklist (from DISCOVER)

1. **Find** the Postgres credentials Secret (name, namespace, key).
2. **Pick** Option 1 (daemon + `password_from_secret`), 2 (wrapper), 3 (in-cluster), or 4 (manual env).
3. **Remove** from config and scripts:
   - `DISCOVER_MY_PASSWORD` in any URL
   - `-kube-password-var`, `-kube-password-container`, YAML `kube.password_var` / `kube.password_container` (removed in 0.9.x)
4. **Tighten RBAC** — if you added `pods/exec` only for DISCOVER, remove it from pgwd’s Role.
5. **Test** `-validate-k8s-access` and `-dry-run` before production.
6. **Upgrade** pgwd to 0.9.x when ready.

---

## Troubleshooting

| Symptom | What to do |
|---------|------------|
| Startup error mentions `DISCOVER_MY_PASSWORD` | URL or Secret value still contains the literal placeholder — follow Option 1 or 2 above |
| `get secret … forbidden` | Apply **`contrib/k8s/rbac-outside-cluster.yaml`** (Option 1) or use wrapper / manual kubectl (Options 2/4) |
| `secret … has no key "password"` | List keys: `kubectl get secret NAME -n NS -o jsonpath='{.data}' \| jq 'keys'` — set `kube.password_from_secret.key` |
| Bitnami / file-based password | Secret may use a different key; or store full `postgres://…` in Secret with `key: url` |
| Port-forward works but connect fails | Ensure `db.url` host is `localhost`/`127.0.0.1` and port matches `kube.local_port` |
| Cron job worked before upgrade | Inject password via wrapper or env; DISCOVER no longer exists |

---

## Secret key reference

| `kube.password_from_secret.key` | Secret content | `db.url` password |
|---------------------------------|----------------|-------------------|
| `password` (default) | Plain password string | Leave empty in URL; pgwd injects after read |
| `url` | Full `postgres://user:pass@host:5432/db?…` | Ignored when full DSN returned; host rewritten to localhost for port-forward |

---

## Why we removed `DISCOVER_MY_PASSWORD`

**Summary:** The placeholder made pgwd run **`printenv` inside the Postgres pod** via Kubernetes **`pods/exec`**. That is remote process execution on the database workload — not “read a Secret”. It required **`pods/exec` RBAC** (often over-granted), widened blast radius (password in pgwd memory/DSN), and conflicted with pgwd’s read-only monitoring role.

**Standard alternatives** read the **same Secret** the chart already uses:

- **`secrets/get`** on one named Secret (`password_from_secret`) — auditable, least privilege
- **Operator reads Secret** (wrapper, CI, `kubectl`) — no Secret API in pgwd
- **In-cluster `secretKeyRef`** — no port-forward

| Approach | Outside cluster | Password in git | pgwd needs `pods/exec` | pgwd needs `secrets get` |
|----------|-----------------|-----------------|-------------------------|---------------------------|
| ~~`DISCOVER_MY_PASSWORD`~~ (removed) | Yes | Placeholder only | **Yes** | No |
| Secret → env / URL (script or CI) | Yes | No | No | No |
| **`kube.password_from_secret`** | Yes | No | No | **Yes** (scoped) |
| In-cluster Secret env | N/A | No | No | No |

---

## References

- **Use-case index:** [use-cases.md](./use-cases.md)
- Profile: [contrib/profiles/kube-prod.yml](../contrib/profiles/kube-prod.yml)
- Multi-DB profile: [contrib/profiles/multi-db.yml](../contrib/profiles/multi-db.yml)
- RBAC sample: [contrib/k8s/rbac-outside-cluster.yaml](../contrib/k8s/rbac-outside-cluster.yaml)
- Wrapper: [contrib/k8s/pgwd-kube-run.sh](../contrib/k8s/pgwd-kube-run.sh)
- Outside-cluster README: [contrib/k8s/README.md](../contrib/k8s/README.md#pgwd-outside-the-cluster)
- Behavior contract: [SPECIFICATIONS.md](../SPECIFICATIONS.md) §9
- Removal plan: [plan-0.9.x.md](./plan-0.9.x.md) §10
