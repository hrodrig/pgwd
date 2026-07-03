# Kubernetes passwords — deprecating `DISCOVER_MY_PASSWORD`

**Status:** **Deprecated** in v0.6.x documentation (behavior unchanged until removal). **Removed** in **v0.9.x** ([plan-0.9.x.md](./plan-0.9.x.md) §10).

This document is the **decision record** for operators and contributors: why the placeholder exists today, why it must go, and what to use instead.

---

## Summary

When the Postgres DSN password is the literal string `DISCOVER_MY_PASSWORD`, pgwd runs **`printenv`** inside the **Postgres pod** via the Kubernetes **`pods/exec`** subresource, copies the value into the process, rewrites `-db-url`, and connects through port-forward.

That trades a convenience problem (“I don’t want the password in my config file”) for a **strictly more dangerous primitive** than every standard Kubernetes alternative. pgwd is a **read-only** Postgres connection monitor; `pods/exec` is **process execution inside the workload**, not secret retrieval.

**Verdict:** not an acceptable long-term trade-off. Deprecate now; remove in 0.9.x.

---

## What it does today

1. Operator sets `-db-url` (or config) with password `DISCOVER_MY_PASSWORD` and `-kube-postgres namespace/svc/name`.
2. At startup, `setupKube` detects the placeholder and calls `GetPasswordFromPod`:

```177:186:cmd/pgwd/main.go
	if kube.URLContainsDiscoverPassword(cfg.DBURL) {
		podName, err := kube.ResolvePod(ctx, cfg.KubeContext, namespace, resource)
		if err != nil {
			log.Fatalf("kube resolve pod: %v", err)
		}
		password, err = kube.GetPasswordFromPod(ctx, cfg.KubeContext, namespace, podName, cfg.KubePasswordContainer, cfg.KubePasswordVar)
		if err != nil {
			log.Fatal("kube: could not get password from pod (check namespace, pod name, container, and env var)")
		}
	}
```

3. `GetPasswordFromPod` opens an SPDY exec stream and runs `printenv <envVar>` in the target container (fallback to `PGPASSWORD`):

```187:225:internal/kube/kube.go
func GetPasswordFromPod(ctx context.Context, kubeContext, namespace, podName, container, envVar string) (string, error) {
	// ...
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").Namespace(namespace).Name(podName).
		SubResource("exec")
	opts := &corev1.PodExecOptions{
		Command: []string{"printenv", envVar},
		Stdout:  true,
		// ...
	}
	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	// ...
}
```

4. Password is injected into `cfg.DBURL`; port-forward starts; pgwd connects to `localhost`.

Related flags (also deprecated): `-kube-password-var`, `-kube-password-container`, YAML `kube.password_var` / `kube.password_container`.

---

## Problem it tried to solve

**Goal:** run pgwd **outside** the cluster (VM, cron, laptop) with `-kube-postgres`, without storing the Postgres password in git, plain config files, or shell history.

**Intent was reasonable.** The **mechanism was not.**

The password already lives in a Kubernetes **Secret** (or external secret operator) that populates `POSTGRES_PASSWORD` in the Postgres pod. DISCOVER skips that layer and reaches into the running pod instead.

---

## Why we are deprecating it

### 1. `pods/exec` is not “read a secret” — it is “run a process in the pod”

The exec subresource is the same API surface used for interactive shells and arbitrary commands. pgwd only runs `printenv` today, but the permission model is **`pods/exec` on the target pod**, not `secrets/get`.

From a security review perspective, you are granting **remote code execution capability** on the Postgres workload to whatever identity runs pgwd. The password is just the first command that occurred to implement; the primitive allows any command the container image supports.

**Risk:** a compromised pgwd kubeconfig or ServiceAccount becomes a **lateral movement path into Postgres pods**, not merely a credential leak.

### 2. RBAC was implicit, undocumented, and often over-granted

Port-forward alone needs roughly: `get/list` pods and services, `create` on `pods/portforward` (namespace-scoped Role is enough).

`GetPasswordFromPod` additionally requires **`pods/exec`** on the Postgres pod. That requirement was **not** documented in operator guides, `contrib/k8s/`, or the pgwd-selfhosted Helm chart when this analysis was done.

Typical failure mode:

1. Operator deploys pgwd with DISCOVER; startup fails on RBAC.
2. Operator grants **`pods/exec`** — often too broadly (ClusterRole, all pods) to “make it work”.
3. pgwd becomes a **durable exec proxy**; auditors see “monitoring tool” but the bound Role allows shell-equivalent access.

Least-privilege deployment is a stated product goal. DISCOVER works against that goal in practice.

### 3. Standard Kubernetes paths already solve the same problem — more safely

The canonical pattern:

- **In-cluster pgwd:** mount Secret → env (`secretKeyRef`) or config; connect via service DNS. No port-forward, no exec.
- **Outside cluster:** read the **same Secret** the Postgres chart uses (`kubectl get secret … | base64 -d`, wrapper script, CI secret injection, External Secrets, Vault Agent) and pass `PGWD_DB_URL` or config. RBAC is **`secrets/get` on one named Secret** (auditable, scoped) — or RBAC on the **human/CI** identity, not on a long-lived pgwd ServiceAccount with exec.

Postgres also supports cert auth, IAM (cloud), and `.pgpass` for non-Kubernetes paths. None require exec into the database pod.

DISCOVER does not enable a capability that Secret-backed URLs lack; it enables it with **worse worst-case** (exec vs read one Secret).

### 4. It pulls secrets through pgwd — widening the blast radius

Flow today:

```
Postgres pod env  →  SPDY exec stream  →  pgwd process memory  →  cfg.DBURL string
```

That password then exists in:

- Process memory for the lifetime of pgwd
- Potential log/panic/debug output (URL masking helps routine logs; it is not a secret boundary)
- Any tooling that dumps config, env, or core dumps
- The same host as the HTTP `/metrics` server when enabled

A Secret confined to the pod (or read once into env at deploy time) has a **smaller exposure surface** than a monitor process that **materializes** the credential on every startup.

### 5. It contradicts pgwd’s positioning

pgwd monitors **`pg_stat_activity`** — read-only SQL. Documentation emphasizes monitor, non-mutating, minimal container, non-root runtime.

`pods/exec` is the opposite axis: **mutation API** in Kubernetes terms (starting processes inside workloads). Keeping DISCOVER undermines the story operators tell auditors: “this binary only watches connections.”

### 6. Architectural dead end — single-DB, growing debt

Constraints today:

- DISCOVER is wired to **`cfg.DBURL`** / single-DB `-kube-postgres` only.
- **`kube.postgres` is not supported with `databases:`** (multi-DB is the canonical direction; legacy `db:` is deprecated for v1.0).

So DISCOVER sits on the **deprecated side** of the config model. Every future per-database kube feature would multiply exec targets (N pods, N exec permissions) from one outside-the-cluster binary.

Supporting it further invests in a path the project is explicitly leaving.

### 7. Operational fragility

Additional issues found in review:

| Issue | Effect |
|-------|--------|
| **SPDY/WebSocket exec** | Only exec uses this path; port-forward uses SPDY separately. Proxies/LBs that break exec break **startup entirely** (pgwd exits before monitoring). |
| **Image variance** | Bitnami and others use `POSTGRES_PASSWORD_FILE` (file mount), not env. Fallback chain (`kube.password_var` → `PGPASSWORD`) fails with opaque error: *“could not get password from pod”*. |
| **Inconsistent stack** | Rest of `internal/kube/` uses client-go list/get/port-forward. Exec is a legacy vestige of “discover from pod” ergonomics. |

---

## Comparison

| Approach | pgwd outside cluster | Password in git/plain config | pgwd needs `pods/exec` | pgwd needs `secrets get` | Fits multi-DB future |
|----------|------------------------|------------------------------|-------------------------|---------------------------|----------------------|
| **`DISCOVER_MY_PASSWORD`** (current) | Yes | Placeholder only | **Yes** | No | No |
| **Secret → env / URL** (script or CI) | Yes | No | No | No (human/CI reads Secret) | Yes |
| **`kube.password_from_secret`** (planned 0.9) | Yes | No | No | **Yes** (one named Secret) | Planned |
| **In-cluster Secret env** | N/A (inside) | No | No | No | Yes |
| **Direct TCP + Secret** | If reachable | No | No | No | Yes |

---

## What to use instead

Choose by **where pgwd runs**:

### A. In-cluster (preferred for daemon mode)

Deployment/DaemonSet; DSN from Secret. No `-kube-postgres`.

```yaml
env:
  - name: PGWD_DB_URL
    valueFrom:
      secretKeyRef:
        name: pgwd-db
        key: url
```

See [contrib/k8s/README.md](../contrib/k8s/README.md) and [pgwd-selfhosted](https://github.com/hrodrig/pgwd-selfhosted).

### B. Outside cluster — inject Secret before pgwd (no code change)

```bash
export PGPASSWORD="$(kubectl get secret postgres-credentials -n default \
  -o jsonpath='{.data.password}' | base64 -d)"
export PGWD_DB_URL="postgres://postgres:${PGPASSWORD}@localhost:5432/mydb?sslmode=disable"
pgwd -kube-postgres default/svc/postgres -client prod -interval 60 \
  -notifications-slack-webhook "$WEBHOOK"
```

Use in cron/systemd; RBAC on the **operator identity** (human or CI), not on pgwd with exec.

### C. Outside cluster — wrapper script (planned 0.9)

`contrib/k8s/pgwd-kube-run.sh` — reads Secret, builds URL, execs pgwd. Same security model as (B), packaged for operators.

### D. Outside cluster — `kube.password_from_secret` (planned 0.9.0)

Explicit, auditable config:

```yaml
kube:
  postgres: default/svc/postgres
  password_from_secret:
    namespace: default
    name: postgres-credentials
    key: password
db:
  url: "postgres://postgres@localhost:5432/mydb?sslmode=disable"
```

Implementation: client-go **`secrets.Get`** only — no exec. Namespace-scoped Role with `resourceNames` on the Secret. See [plan-0.9.x.md](./plan-0.9.x.md) §10.

### E. Enterprise secret sync

External Secrets Operator, Sealed Secrets, Vault Agent → file or env consumed by pgwd or a wrapper. No pgwd-specific exec required.

---

## Migration checklist

If you use `DISCOVER_MY_PASSWORD` today:

1. **Identify** where the Postgres chart stores credentials (Secret name/key).
2. **Choose** pattern A (in-cluster) or B/C/D (outside cluster).
3. **Replace** placeholder in URL with real password from Secret (never commit the value).
4. **Remove** `-kube-password-var` / `-kube-password-container` from config and docs.
5. **Tighten RBAC** — drop `pods/exec` from pgwd ServiceAccount if it was added only for DISCOVER.
6. **Test** `-validate-k8s-access` and a dry-run connect before production cutover.

After **0.9.x**, pgwd will **exit at startup** if the placeholder remains:

```text
DISCOVER_MY_PASSWORD was removed in pgwd 0.9.x (security).
Use a Secret-backed URL — see docs/kubernetes-passwords.md
```

---

## Timeline

| Release | Action |
|---------|--------|
| **0.6.x** (now) | Document deprecation; Secret-backed examples in README/SPEC; this decision record |
| **0.9.0** | Remove code (`GetPasswordFromPod`, placeholder detection); ship `password_from_secret` + wrapper + RBAC sample; migrate e2e tests |
| **1.0.0** | No DISCOVER surface remains |

---

## References

- Implementation: `internal/kube/kube.go` (`GetPasswordFromPod`), `cmd/pgwd/main.go` (`setupKube`)
- Removal plan: [plan-0.9.x.md](./plan-0.9.x.md) §10
- Behavior contract: [SPECIFICATIONS.md](../SPECIFICATIONS.md) §9
- Roadmap index: [docs/README.md](./README.md#roadmap-to-v100)
