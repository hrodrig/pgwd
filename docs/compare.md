# pgwd vs common Postgres monitoring options

> **Snapshot v1.0.0.** Competitor claims are best-effort summaries verified around the pgwd **v1.0.0** release date — re-check upstream docs before betting architecture on them.

## TL;DR

- **pgwd** is a **connection-focused watchdog**: total / active / idle / stale (and optional long-query) alerts from `pg_stat_activity`, with modern notifiers (Slack, Loki, PagerDuty, Teams, generic webhook), hysteresis, and optional `/metrics`.
- It is **not** a full Postgres metrics platform, APM, or query-performance suite.
- Prefer **pgwd** when you want a **single binary**, clear exit codes for cron, and alerts without standing up Prometheus first.
- Prefer **exporter / SaaS / cloud alarms** when you already run that stack or need broad SQL metrics and dashboards.

## pgwd is / pgwd is not

| pgwd **is** | pgwd **is not** |
|-------------|-----------------|
| Connection / saturation watchdog | Full `postgres_exporter`-style metric catalog |
| One-shot (cron) or daemon | Embedded Grafana / dashboards |
| Self-hosted, signed releases (SBOM + Cosign) | Hosted SaaS |
| Multi-DB in one process (`databases:`) | Per-target notifier routing (global notifiers only) |
| Optional long-query alerts | Slow-query / EXPLAIN / wait-event platform |
| Optional HTTP `/metrics` + SQLite/SQL history | Replacement for PgBouncer or pooler ops |

## Capability matrix

| Capability | pgwd | postgres_exporter + Alertmanager | pgwatch / pgwatch3 | Datadog / New Relic | Cloud DB alarms | cron + psql | Nagios / check_postgres |
|------------|------|----------------------------------|--------------------|---------------------|-----------------|-------------|-------------------------|
| **Focus** | Connections / stale / long-query | Broad SQL metrics | Dashboards + metrics | Full APM + infra | Provider limits / CPU / storage | Whatever you script | Plugin checks |
| **Deploy** | Single binary / container | Prometheus stack | Heavier suite | Agent + SaaS | Console / IaC | Cron + shell | Nagios/Icinga host |
| **Alert path** | Built-in notifiers | Alertmanager rules | Suite + optional | Product alerting | Cloud native | DIY | Plugin + contacts |
| **Hysteresis / resolve** | Yes (`confirm_*`, resolution) | Rules you write | Suite-dependent | Product features | Often simple thresholds | DIY | DIY / plugins |
| **Multi-DB** | `databases:` | Targets / jobs | Suite | Agent config | Per instance | N crons | N checks |
| **Kube-friendly** | Port-forward / in-cluster | Common | Possible | Common | N/A (managed) | Possible | Possible |
| **Cost model** | OSS self-host | OSS + ops time | OSS + ops | Subscription | Cloud bill | Ops time | Ops time |
| **Supply chain** | Distroless, Cosign, SBOM | Mixed | Mixed | Vendor | Vendor | None | Mixed |

## When to pick pgwd

- Need **connection-pressure and connect-failure** alerts with Slack / Loki / PagerDuty / Teams without a full metrics stack.
- Want **cron-friendly exit codes** (0/1/2/3/4) and a **daemon** with history / resolution when ready.
- Run Postgres on **VMs, bare metal, or Kubernetes** and want the **same tool** everywhere.
- Prefer a **narrow, auditable** contract ([SPECIFICATIONS.md](../SPECIFICATIONS.md)) over a large agent surface.

## When not pgwd

- You already run **Prometheus + Grafana + Alertmanager** and want **many** Postgres metrics / custom PromQL — use **postgres_exporter** (pgwd can still complement connection alerts).
- You need **rich dashboards and historical analytics** out of the box — look at **pgwatch** or a SaaS APM.
- You need **query performance**, wait events, or plan analysis as the primary product — not pgwd’s scope (optional long-query only).
- You want **per-database notifier routing** (DB A → Slack, DB B → Teams) in one process — not supported; run multiple processes or use a fan-out downstream.
- You only need the cloud provider’s built-in **max connections / CPU** alarms and never leave that cloud — cloud alarms may be enough.

## Detailed comparisons

### Prometheus `postgres_exporter` + Grafana / Alertmanager

Exporter scrapes a wide metric set; you own recording rules, alert rules, and the stack. **pgwd** ships connection-tier alerts and notifiers without Prometheus. Optional pgwd `/metrics` can feed Prometheus if you want both.

### pgwatch / pgwatch3

Richer Postgres monitoring suite (metrics, dashboards, heavier deploy). **pgwd** stays a small watchdog binary for connection/stale/long-query style alerts.

### Datadog / New Relic / hosted APM

Full platform, agents, cost, vendor lock-in. **pgwd** is self-hosted and narrow; pair with SaaS only if you need deep APM elsewhere.

### AWS CloudWatch / GCP / Azure managed DB alarms

Convenient per-cloud. **pgwd** is the same binary on-prem, multi-cloud, and Kubernetes, with shared notifier and hysteresis behavior.

### cron + `psql` / shell

Zero dependencies, you maintain everything. **pgwd** adds levels, resolution, multi-DB daemon, connect-failure notify, and a tested config contract.

### Nagios / check_postgres-style

Mature plugin ecosystems. **pgwd** is Postgres-native (`pg_stat_activity`) with modern chat/on-call notifiers rather than a generic check framework.

## Footnotes

- PgBouncer and other **poolers** are complementary, not alternatives.
- Incident-gather tools (e.g. Groot) solve a different problem.
- Landing mirror (when published): [pgwd.hermesrodriguez.com/compare](https://pgwd.hermesrodriguez.com/compare) (external site).

## Related

- [README — Compare](../README.md#compare)
- [Operator use cases](./use-cases.md) — how to deploy; this page is **what to pick**
- [SPECIFICATIONS.md](../SPECIFICATIONS.md)
- [ROADMAP.md](../ROADMAP.md)
