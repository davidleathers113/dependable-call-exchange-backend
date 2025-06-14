# DCE System-Improve: End-to-End Improvement & Review Pipeline (v1.1)

Parse CLI-style switches from **$ARGUMENTS**

| Flag | Values | Default | Description |
|------|--------|---------|-------------|
| `--phase` | 0,1,1a,1b,2,3,3.5,4,5,all | all | Run specific phases |
| `--depth` | quick, thorough, exhaustive | thorough | Analysis/implementation depth |
| `--output` | <path> | ./.claude/improve | Root for generated artefacts |
| `--incremental` | true, false | false | Use `.claude/state/*` for delta runs |
| `--review` | auto, skip | auto | Enable self-review report |

> **Mission** Implement every action item in *SYSTEM_IMPROVEMENT_GUIDE.md* located here `/Users/davidleathers/projects/DependableCallExchangeBackEnd/.claude/docs/SYSTEM_IMPROVEMENT_GUIDE.md`, update `dce-master-plan` as per Option A bridge, and generate a comprehensive self-review + optimisation recommendations.

---

## PHASE 0 – 🚨 Critical Handoff Fix (Serial)

1. **Update-Master-Plan Task**  
   *Modify* `dce-master-plan.md` (Phase 5b) to emit `.claude/context/feature-context.yaml` & `implementation-plan.md` **in addition to** existing planning files :contentReference[oaicite:33]{index=33}.  
2. **Bridge-Generator Task**  
   Back-convert any *legacy* `.claude/planning/specs/*.md` into the new context format for backward compatibility.

*Success criteria* – New bridge files present **and** `dce-master-plan` passes a dry-run integration test with `dce-feature`.

---

## PHASE 1 – Foundation Upgrades

### 1a State Engine (Serial)

* Create/refresh:  
  `.claude/state/system-snapshot.yaml`, `analysis-history.yaml`, `feature-progress.yaml`, `dependency-graph.yaml`, `performance-metrics.yaml` :contentReference[oaicite:34]{index=34}.

### 1b Smart Work Discovery (Parallel)

* **Work-Discovery Engine** → builds `/dce-find-work` and writes `.claude/work-discovery/criteria.yaml` :contentReference[oaicite:35]{index=35}.  
* **Queue-Builder** → generates `.claude/context/execution-queue.yaml` and initial batch order :contentReference[oaicite:36]{index=36}.

---

## PHASE 2 – Progress Tracking & Resumption

* **Progress-Tracker Task** → continually updates `feature-progress.yaml` during downstream phases.  
* **Resume-Command Generator** → emits helper commands `/dce-feature-resume|continue|retry` with templates and docs :contentReference[oaicite:37]{index=37}.

---

## PHASE 3 – Inter-Wave Coordination

* Run WAVE orchestrator that writes/reads `.claude/context/wave-coordination.yaml` and enforces the five-step **Conflict Resolution Protocol** when overlaps detected :contentReference[oaicite:38]{index=38}.  
* Parallelism cap: **max 5 tasks**; excess queued via Execution Queue.

---

## PHASE 3.5 – Implementation-Detail Generation (High-Priority Specs Only)

Spawn **five specialised generators per spec** (Code Structure, DB Schema, API Contract, Test Template, Documentation) as outlined in the Guide :contentReference[oaicite:39]{index=39}.

---

## PHASE 4 – Enhanced Coordination & Dependency Scheduling

* Run **Dependency Manager** to reorder pending queue entries, fill parallel batches, and update `.claude/context/execution-queue.yaml` with actual start/end timestamps.

---

## PHASE 5 – Self-Review + Continuous Improvement Loop

1. **Audit Reviewer**  
   * Scores artefacts against: implementation-detail specs, monitoring & optimisation guidelines, and acceptance-criteria tables.  
   * Writes `${OUTPUT_DIR}/reviews/review-report.md` with per-feature scores and **blockers if <80%**.  
2. **Metrics Dashboard**  
   * Logs execution durations, token usage, success rates; persists to `.claude/monitoring/metrics.db`.  
3. **Weekly Optimiser**  
   * If `--review=auto` and run on *main* branch, schedule a Task to execute the improvement loop described in `optimization/improvement-loop.yaml` :contentReference[oaicite:40]{index=40}.

---

## FINAL OUTPUTS

| File/Folder | Purpose |
|-------------|---------|
| `${OUTPUT_DIR}/state/*` | Persisted system state & history |
| `${OUTPUT_DIR}/context/*` | Bridge, queue, wave coordination |
| `${OUTPUT_DIR}/specs-implementation/` | Code-ready specs & migrations |
| `${OUTPUT_DIR}/reviews/review-report.md` | Self-audit, blockers, next steps |
| `${OUTPUT_DIR}/metrics/` | Raw execution metrics for dashboard |
| `${OUTPUT_DIR}/execution-log.json` | Timing, tokens, errors |

*Invoke example*  
```bash
/dce-system-improve --phase=0-2 --depth=exhaustive --incremental=true
