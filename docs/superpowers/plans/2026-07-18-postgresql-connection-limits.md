# PostgreSQL connection-limits documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a short README problem presentation and a practical long guide for sizing PostgreSQL `max_connections` (DBA + developer audiences) without bloating the README.

**Architecture:** README keeps a ~120–180 word English blurb + links; full content lives in `docs/postgresql-connection-limits.md`; index via `docs/README.md` and CHANGELOG Docs entry. No application code changes.

**Tech Stack:** Markdown; English-only project docs; citations to PostgreSQL current docs.

**Spec:** [docs/superpowers/specs/2026-07-18-postgresql-connection-limits-design.md](../specs/2026-07-18-postgresql-connection-limits-design.md)

## Global Constraints

- English only for all new prose
- README growth: one short subsection only (~120–180 words)
- Heuristics must be labeled illustrative / measure — not absolute guarantees
- pgwd levels use **configured** `max_connections` (do not claim reserved slots are subtracted)
- Prefer SQLSTATE **53300** over English-only error text as canonical rejection identity
- Do not claim pgwd sees every client behind a pooler
- No Go/code/SPEC changes in this plan
- Commit message review: show message and wait for user approval before `git commit`

## File map

| File | Responsibility |
|------|----------------|
| `README.md` | Problem blurb after product one-liner; Documentation line link |
| `docs/postgresql-connection-limits.md` | Long guide (foundation, DBA, heuristics, developers, pgwd, refs, checklist) |
| `docs/README.md` | Index entry under Operations |
| `CHANGELOG.md` | `[Unreleased]` → Documentation bullet |

---

### Task 1: README problem blurb + Documentation link

**Files:**
- Modify: `README.md` (after product one-liner ~line 30; Documentation line ~36)

**Interfaces:**
- Consumes: none
- Produces: anchor-free blurb; link target `docs/postgresql-connection-limits.md` (file created in Task 2 — link may 404 until Task 2 lands; prefer Task 1 then Task 2 in same session, or create stub in Task 2 first)

- [ ] **Step 1: Insert blurb after line 30**

After:

```markdown
Go CLI that checks PostgreSQL connection counts (active/idle) and notifies via **Slack** and/or **Loki** when configured thresholds are exceeded. It can also alert on **stale connections** (connections that stay open and never close).
```

Insert (adjust notifier list only if surrounding README already lists more notifiers in that sentence — keep blurb self-contained):

```markdown
### Why connection limits matter

PostgreSQL enforces a configured ceiling (`max_connections`). Slots reserved for privileged roles (`superuser_reserved_connections`, and on PostgreSQL 16+ `reserved_connections`) reduce how many ordinary application connections can succeed before the server starts refusing new sessions. When the limit is hit, clients fail to connect with SQLSTATE **53300** (“too many clients”). Under high concurrency, each backend is still a process: memory (`shared_buffers` plus per-session/`work_mem` pressure), CPU, and I/O can degrade even before hard rejection—or the OS can OOM.

**pgwd** watches connection pressure (and connect failures) so you can alert before or when saturation happens. It does **not** replace connection poolers or careful sizing. For hardware/app heuristics, reserved slots, WAL/standby notes, and developer pool pitfalls, see **[PostgreSQL connection limits and saturation](docs/postgresql-connection-limits.md)**.
```

Word count target: 120–180 words for the two paragraphs under the heading (heading excluded).

- [ ] **Step 2: Extend Documentation line**

Find the `**Documentation:**` line and add a link next to other docs, e.g.:

```markdown
**Documentation:** [ROADMAP.md](ROADMAP.md), [Operator use cases](docs/use-cases.md), [Connection limits](docs/postgresql-connection-limits.md), [SPECIFICATIONS.md](SPECIFICATIONS.md) (behavior contract), ...
```

Keep existing links; insert Connection limits after use-cases (or after Compare if that reads better).

- [ ] **Step 3: Verify placement**

Run:

```bash
rg -n "Why connection limits matter|postgresql-connection-limits" README.md
wc -w <<'EOF'
# paste the two blurb paragraphs only
EOF
```

Expected: heading + link present; blurb word count roughly 120–180.

- [ ] **Step 4: Commit (after user approves message)**

Proposed message:

```
docs: add README blurb for Postgres connection limits

Short problem presentation with link to the long sizing guide.
```

```bash
git add README.md
git commit -m "$(cat <<'EOF'
docs: add README blurb for Postgres connection limits

Short problem presentation with link to the long sizing guide.
EOF
)"
```

---

### Task 2: Write long guide

**Files:**
- Create: `docs/postgresql-connection-limits.md`

**Interfaces:**
- Consumes: accuracy constraints from Global Constraints; link from README
- Produces: standalone guide linked from README and docs index

- [ ] **Step 1: Create the file with this content**

Write `docs/postgresql-connection-limits.md` exactly structured as below (English). You may tighten wording for clarity but must keep all sections and accuracy constraints.

```markdown
# PostgreSQL connection limits and saturation

Practical guide for **DBAs / operators** and **application developers** on why `max_connections` matters, what happens when it is exceeded, and how to choose a starting value. For how **pgwd** alerts on connection pressure, see the [README](../README.md) and [compare](./compare.md).

> **Illustrative only.** Ranges and heuristics below are starting points. Measure on your hardware and workload; official PostgreSQL documentation is authoritative.

## Shared foundation

PostgreSQL uses **one backend process per connection** (plus background workers). The server-wide ceiling is `max_connections`.

Ordinary application roles do **not** always get the full ceiling:

- `superuser_reserved_connections` — slots held for superusers as a last resort (default is typically 3).
- `reserved_connections` (PostgreSQL **16+**) — slots for roles with `pg_use_reserved_connections`.

When free slots fall into the reserved bands, non-privileged clients are refused even though `max_connections` has not been “used up” by apps alone. **Configured ceiling ≠ ordinary capacity.**

When no slot is available for the connecting role, PostgreSQL rejects the connection. Prefer identifying this as SQLSTATE **53300** (often surfaced in English as “too many clients already”). Language of the message can vary; the SQLSTATE does not.

High connection counts also stress:

- **Memory** — shared memory sized partly from connection-related settings; per-session work (`work_mem`, hash operations, etc.) multiplies with concurrent queries.
- **CPU / OS** — process and file-descriptor limits must cover backends plus other system needs.
- **WAL / replication** — standbys and senders need headroom; hot standby shared-memory related settings (including `max_connections`) must be ≥ primary.

## For DBAs / operators

### Sizing checklist

1. Know current `max_connections`, `superuser_reserved_connections`, and (PG 16+) `reserved_connections`.
2. Subtract reserved slots when estimating **application** capacity.
3. Leave headroom for admin, monitoring (e.g. pgwd), backups, and replication-related activity.
4. Size RAM for `shared_buffers` + OS + concurrent `work_mem`-class usage — not for “max connections × huge work_mem” blindly.
5. Confirm OS process / open-file limits can cover `max_connections` plus background workers.
6. On hot standby: set `max_connections` (and related shared-memory parameters) **≥ primary**; raise standbys first when increasing.
7. Prefer an **external connection pooler** when the problem is “too many clients,” not “need a higher ceiling.” Official docs note that reducing `max_connections` and using pooling is often better than raising the limit under memory pressure.

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
| Small VM / laptop / single app + pooler | 50–100 | Default 100 is often enough; do not raise “just in case.” |
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
- Treat connect errors with SQLSTATE **53300** as capacity incidents, not “retry forever” without backoff and alerting.

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
- [ ] Prefer pooler + lower ceiling over “raise to 1000”
- [ ] Monitor connect rejections (SQLSTATE **53300**) and connection pressure (e.g. with **pgwd**)
- [ ] Separate admin/monitor roles from application login roles
- [ ] Review idle connections and pool leaks in application code
- [ ] Re-check after major version upgrades (e.g. `reserved_connections` on PG 16+)
```

- [ ] **Step 2: Accuracy grep**

```bash
rg -n "effective capacity|subtract|guaranteed|always set max_connections" docs/postgresql-connection-limits.md || true
rg -n "53300|illustrative|configured" docs/postgresql-connection-limits.md
```

Expected: no claim that pgwd subtracts reserved slots; `53300` and `illustrative`/`configured` present.

- [ ] **Step 3: Commit (after user approves message)**

```
docs: add PostgreSQL connection limits guide

Practical DBA/developer sizing guide linked from the README blurb.
```

```bash
git add docs/postgresql-connection-limits.md
git commit -m "$(cat <<'EOF'
docs: add PostgreSQL connection limits guide

Practical DBA/developer sizing guide linked from the README blurb.
EOF
)"
```

---

### Task 3: Index + CHANGELOG

**Files:**
- Modify: `docs/README.md`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Consumes: guide path from Task 2
- Produces: discoverable index + release-notes entry

- [ ] **Step 1: Add Operations section to `docs/README.md`**

After the Upgrading block (after the kubernetes use-cases / kubernetes-passwords / compare paragraphs, before “## Loki and Grafana alerts”), insert:

```markdown
## Operations

**[PostgreSQL connection limits and saturation](./postgresql-connection-limits.md)** — Why `max_connections` matters, reserved slots, memory/CPU/WAL considerations, illustrative sizing heuristics for hardware and app types, developer pool pitfalls, and how pgwd fits.
```

- [ ] **Step 2: CHANGELOG Documentation bullet**

Under `## [Unreleased]` → `### Documentation`, add:

```markdown
- **[docs/postgresql-connection-limits.md](docs/postgresql-connection-limits.md)** — practical guide to connection saturation and sizing heuristics; README short problem blurb + link.
```

- [ ] **Step 3: Smoke-check links**

```bash
test -f docs/postgresql-connection-limits.md && echo OK_guide
rg -n "postgresql-connection-limits" README.md docs/README.md CHANGELOG.md
```

Expected: file exists; three files reference it.

- [ ] **Step 4: Commit (after user approves message)**

```
docs: index connection-limits guide and changelog entry

Link from docs/README Operations and Unreleased Documentation.
```

```bash
git add docs/README.md CHANGELOG.md
git commit -m "$(cat <<'EOF'
docs: index connection-limits guide and changelog entry

Link from docs/README Operations and Unreleased Documentation.
EOF
)"
```

---

### Task 4: Final verification

**Files:** none new

- [ ] **Step 1: Spec coverage checklist**

Confirm each spec item is done:

| Spec item | Evidence |
|-----------|----------|
| README blurb 120–180 words | `wc -w` on blurb paragraphs |
| Link to long guide | README + docs/README |
| Guide sections 1–7 | headings in guide file |
| Heuristics table + anti-pattern | present |
| DBA + developer sections | present |
| pgwd accuracy constraints | grep |
| CHANGELOG | Unreleased Docs bullet |
| No code changes | `git diff --stat` shows md only |

- [ ] **Step 2: Report to user**

Summarize files changed, offer push of commits if any remain local.

---

## Self-review (plan vs spec)

1. **Spec coverage:** README blurb, long guide (foundation, DBA, heuristics, developers, pgwd, refs, checklist), docs index, CHANGELOG — all tasked. Official citations included in Task 2 body.
2. **Placeholders:** none intentional; full markdown provided for guide and blurb.
3. **Consistency:** path always `docs/postgresql-connection-limits.md`; SQLSTATE 53300; illustrative labeling; pgwd uses configured `max_connections`.
