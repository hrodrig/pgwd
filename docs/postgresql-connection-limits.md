# PostgreSQL connection limits and saturation

Practical guide for **DBAs / operators** and **application developers** on why `max_connections` matters, what happens when it is exceeded, and how to choose a starting value. For how **pgwd** alerts on connection pressure, see the [README](../README.md) and [compare](./compare.md).

> **Illustrative only.** Ranges and heuristics below are starting points. Measure on your hardware and workload; official PostgreSQL documentation is authoritative.

## Shared foundation

PostgreSQL uses **one backend process per connection** (plus background workers). The server-wide ceiling is `max_connections`.

Ordinary application roles do **not** always get the full ceiling:

- `superuser_reserved_connections` — slots held for superusers as a last resort (default is typically 3).
- `reserved_connections` (PostgreSQL **16+**) — slots for roles with `pg_use_reserved_connections`.

When free slots fall into the reserved bands, non-privileged clients are refused even though `max_connections` has not been "used up" by apps alone. **Configured ceiling ≠ ordinary capacity.**

When no slot is available for the connecting role, PostgreSQL rejects the connection. Prefer identifying this as SQLSTATE **53300** (often surfaced in English as "too many clients already"). Language of the message can vary; the SQLSTATE does not.

High connection counts also stress:

- **Memory** — shared memory sized partly from connection-related settings; per-session work (`work_mem`, hash operations, etc.) multiplies with concurrent queries.
- **CPU / OS** — process and file-descriptor limits must cover backends plus other system needs.
- **WAL / replication** — standbys and senders need headroom; hot standby shared-memory related settings (including `max_connections`) must be ≥ primary.

## For DBAs / operators

### Sizing checklist

1. Know current `max_connections`, `superuser_reserved_connections`, and (PG 16+) `reserved_connections`.
2. Subtract reserved slots when estimating **application** capacity.
3. Leave headroom for admin, monitoring (e.g. pgwd), backups, and replication-related activity.
4. Size RAM for `shared_buffers` + OS + concurrent `work_mem`-class usage — not for "max connections × huge work_mem" blindly.
5. Confirm OS process / open-file limits can cover `max_connections` plus background workers.
6. On hot standby: set `max_connections` (and related shared-memory parameters) **≥ primary**; raise standbys first when increasing.
7. Prefer an **external connection pooler** when the problem is "too many clients," not "need a higher ceiling." Official docs note that reducing `max_connections` and using pooling is often better than raising the limit under memory pressure.

### Memory, CPU, WAL

- **`shared_buffers`** — major shared cache; competes with OS page cache and per-backend memory.
- **`work_mem`** — per *operation* (sort/hash), and many operations can run per query across many sessions; total can be many times `work_mem`.
- **CPU** — idle connections still occupy backends; active queries compete for cores.
- **WAL** — write-heavy workloads and replication increase I/O; connection storms amplify concurrent writers.

Official starting points: [Resource Consumption](https://www.postgresql.org/docs/current/runtime-config-resource.html), [Managing Kernel Resources](https://www.postgresql.org/docs/current/kernel-resources.html), [Hot Standby](https://www.postgresql.org/docs/current/hot-standby.html), [Connection Settings](https://www.postgresql.org/docs/current/runtime-config-connection.html).

## Choosing `max_connections` (heuristics)

Treat the table as **illustrative starting points**, then load-test and watch memory, CPU, and rejection rates.

| Context | Illustrative starting range | Notes |
|---------|----------------------------|--------|
| Small VM / laptop / single app + pooler | 50–100 | Default 100 is often enough; do not raise "just in case." |
| Mid-size OLTP host, 1–2 poolers | 100–200 | Size from `pool_size × poolers` + admin/replication margin. |
| Large OLTP with careful pooling | 200–400 | Rarely need thousands of *Postgres* backends if poolers are healthy. |
| Analytics / few heavy sessions | 20–80 | Prefer memory/I/O for queries over many idle slots. |
| Serverless / many short-lived clients | Keep Postgres low; add pooler | Do **not** set `max_connections` to thousands to absorb fan-out. |

**Hardware signals**

- RAM: after OS + `shared_buffers` + headroom, ask whether worst-case concurrent query memory still fits.
- CPU: more backends than cores is normal for OLTP, but idle bloat still hurts scheduling and memory.
- Disk / WAL: connection spikes that all write can stall checkpoints and replicas.

**Anti-pattern:** `max_connections = 1000` (or higher) without a pooler and without measuring. Official guidance under memory pressure: lower the limit and use external pooling software.

**App-type signals**

- **Web OLTP + PgBouncer/Odyssey/etc.** — Postgres `max_connections` ≈ sum of pooler pool sizes + reserved/admin/replication margin.
- **Many app instances each with a large pool** — multiply carefully; prefer smaller per-instance pools or a shared pooler tier.
- **Batch / analytics** — fewer connections; larger `work_mem` / I/O budget per session.
- **Mixed** — protect admin slots; alert before ordinary capacity is gone (this is where pgwd helps).

## For application developers

- Prefer a **pool** (in-process or external). Opening a new Postgres connection per request does not scale.
- Cap `pool_size` / `max_open_conns` intentionally; idle timeouts reduce leaks.
- Behind PgBouncer (transaction pooling especially), `pg_stat_activity` shows **pooler backends to Postgres**, not every application client. Saturation of the pooler is a different symptom than Postgres `53300`.
- Avoid `pool_size × number_of_replicas` landing as a silent requirement for thousands of Postgres backends.
- Treat connect errors with SQLSTATE **53300** as capacity incidents, not "retry forever" without backoff and alerting.

## How pgwd fits

- pgwd computes alert **levels** from current connection counts versus **configured** `SHOW max_connections` (or test override). It does **not** subtract `superuser_reserved_connections` / `reserved_connections` from the denominator.
- Connect failures that indicate too many clients are surfaced as **`too_many_clients`** (SQLSTATE **53300**), distinct from generic `connect_failure` when applicable.
- pgwd **complements** poolers and metrics stacks; it is not a replacement for pooler operations. See [compare](./compare.md).

## Official references

- [Connections and Authentication / Connection Settings](https://www.postgresql.org/docs/current/runtime-config-connection.html) — `max_connections`, `reserved_connections`, `superuser_reserved_connections`
- [Resource Consumption](https://www.postgresql.org/docs/current/runtime-config-resource.html) — `shared_buffers`, `work_mem`, related memory
- [Managing Kernel Resources](https://www.postgresql.org/docs/current/kernel-resources.html) — process limits; prefer pooling vs raising `max_connections` under memory pressure
- [Hot Standby](https://www.postgresql.org/docs/current/hot-standby.html) — standby ≥ primary for `max_connections` and related parameters

## Checklist

- [ ] Document configured `max_connections` and reserved-slot settings for each environment
- [ ] Estimate ordinary (non-reserved) capacity and alert below that, not only at 100% of ceiling
- [ ] Align app/pooler pool sizes with Postgres capacity + margin
- [ ] Confirm standby `max_connections` ≥ primary
- [ ] Load-test memory under realistic concurrent queries before raising the ceiling
- [ ] Prefer pooler + lower ceiling over "raise to 1000"
- [ ] Monitor connect rejections (SQLSTATE **53300**) and connection pressure (e.g. with **pgwd**)
- [ ] Separate admin/monitor roles from application login roles
- [ ] Review idle connections and pool leaks in application code
- [ ] Re-check after major version upgrades (e.g. `reserved_connections` on PG 16+)
