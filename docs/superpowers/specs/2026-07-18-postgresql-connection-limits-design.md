# Design: PostgreSQL connection limits documentation

**Date:** 2026-07-18  
**Status:** approved for implementation (pending user review of this file)  
**Repo:** pgwd

## Goal

Explain the **problem** of PostgreSQL connection saturation in a short README blurb, and ship a **practical long guide** (DBAs + app developers) for sizing `max_connections` and related resources—without growing the already-large README.

## Non-goals

- Academic papers or vendor-specific magic formulas presented as absolute truth
- Code / SPEC / behavior changes
- Fixing unrelated legacy doc contradictions (dry-run wording, old sequence diagrams) except a minimal clarity note if saturation text would otherwise mislead
- Expanding README with full sizing tables or flag/threshold duplication

## Audience

1. **DBAs / operators** — hardware, memory, CPU, WAL, reserved slots, standby, admin headroom  
2. **Application developers** — pools, idle leaks, multi-replica math, what pgwd sees behind a pooler  

One guide, sections by role.

## Depth

Practical + framing: what happens on saturation, sizing checklist, hardware/app heuristics as **starting points**, official Postgres citations. Not a deep reference manual.

## Approach (chosen)

**README short presentation + single long guide** (`docs/postgresql-connection-limits.md`).

Rejected: two separate guides (more nav duplication); README-only link with no problem framing.

## README changes

**Placement:** Immediately after the product one-liner (~line 30), before “Self-hosted deployment”.

**Length:** ~120–180 words, English.

**Content:**

- `max_connections` is a configured ceiling; reserved slots reduce ordinary capacity
- Saturation: new connections rejected (SQLSTATE `53300` / too many clients); memory/CPU pressure; possible OOM/thrashing under high concurrency
- pgwd alerts on connection pressure (% of configured `max_connections`) and connect failure; does not replace poolers or sizing work
- Link: [PostgreSQL connection limits and saturation](docs/postgresql-connection-limits.md)

**Also:** Add the same link to the existing “Documentation:” line; optional short ToC entry only if it stays one line—prefer no ToC bloat.

## Long guide

**Path:** `docs/postgresql-connection-limits.md`  
**Title:** `# PostgreSQL connection limits and saturation`

### Structure

1. **Shared foundation**  
   - One process per connection  
   - `max_connections`, `superuser_reserved_connections`, `reserved_connections` (PostgreSQL **16+**)  
   - Ordinary capacity ≠ configured ceiling  
   - Rejection: SQLSTATE **53300** (canonical; language-independent)

2. **For DBAs / operators**  
   - Sizing checklist (memory, CPU/OS processes, WAL/hot standby, admin slots)  
   - `shared_buffers`, `work_mem` × concurrent ops/sessions (cite resource docs)  
   - Prefer lower `max_connections` + external pooler when under pressure (cite kernel resources)  
   - Standby: `max_connections` ≥ primary  

3. **Choosing `max_connections` (heuristics)**  
   - Frame clearly: **starting points, measure and adjust**—not guarantees  
   - Hardware signals: RAM after OS/`shared_buffers`/headroom; per-backend process cost; idle connections still consume processes  
   - App types: OLTP + pooler (often moderate ceilings, e.g. illustrative **50–200** on mid-size hosts); serverless/many clients → do **not** jump to thousands—pool or limit clients; analytics/few heavy sessions → lower ceiling, prioritize `work_mem`/I/O  
   - Illustrative small / medium / large ranges labeled as examples  
   - Anti-pattern: `max_connections=1000` “just in case”

4. **For application developers**  
   - App pool vs PgBouncer/external pooler  
   - Idle connection leaks  
   - Behind a pooler, `pg_stat_activity` reflects pooler→Postgres backends, not every client  
   - Avoid blind `pool_size × replicas` multiplication into Postgres

5. **How pgwd fits**  
   - Levels use **configured** `max_connections` (does not subtract reserved slots)  
   - `too_many_clients` on connect rejection  
   - Complements poolers; see `docs/compare.md`

6. **Official references**  
   - Links to current Postgres docs (connection settings, resource consumption, kernel resources, hot standby)

7. **Actionable checklist**  
   - 8–12 bullets for operators and developers

## Indexing / changelog

- `docs/README.md`: link under a short **Operations** (or equivalent) entry  
- `CHANGELOG.md` `[Unreleased]` → Docs: README blurb + connection-limits guide  
- All new prose: **English only**

## Accuracy constraints

- Do not claim pgwd computes “effective” capacity after reserved slots  
- Do not claim pgwd sees all clients behind a pooler  
- Prefer SQLSTATE **53300** over English-only error text  
- Heuristic tables must say **illustrative / measure**

## Success criteria

- README grows by roughly one short subsection only  
- Guide stands alone for DBA + developer readers  
- Official Postgres docs cited for connection/reservation and resource guidance  
- No code changes

## Implementation notes

Follow-up: implementation plan → edit README, write guide, update `docs/README.md` + CHANGELOG; commit after message approval.
