# tyk-pump ReqProof coverage — next-goal proposal (v2)

**Status:** draft proposal, awaiting decisions on the open questions at the end.
**Predecessor:** [PR #1022](https://github.com/TykTechnologies/tyk-pump/pull/1022) (initial coverage), [reqproof feedback issue #81](https://github.com/probelabs/reqproof/issues/81).
**Operating principles for this iteration:**
1. **No corners.** Don't lower bars, don't stub, don't keep tool-side workarounds. Fix things at the depth the assurance class demands.
2. **Known bugs are logged, not fixed in this stream.** Each is recorded as a `proof known-issue` linked to the requirement it violates, with owner = tyk-team.

## Goal statement

Take tyk-pump's spec from **structurally audit-green** (where PR #1022 left it) to **content-honest at the assurance level we actually claim**. That means:
- Removing the workarounds in PR #1022 that bought a 0/0 audit at the cost of governance/coverage/documentation honesty.
- Decomposing the implementations the senior review correctly flagged as under-modeled.
- Adding the requirement categories that NPR 7150.2 / IEEE 29148 expect at Class B.
- Making trust boundaries explicit (`req_type: assumption`) and cross-component contracts formal (ICDs).
- Logging the real code defects found during coverage as first-class `proof known-issue` artifacts linked to the requirements they violate, so they're tracked rather than silently coexisting with the spec.

Seven phases. **0 → A → (B + C in parallel) → D** as the main spine; **E and F** run independently.

---

## Phase 0 — Remove the corners cut in PR #1022 (do first; everything downstream depends on this)

**Why:** PR #1022 reached `proof audit` 0/0, but several of those greens are workarounds rather than honest passes. They must come off the table before further work, otherwise everything else builds on a false floor.

### 0.1 Remove blanket agent-autonomous approval

**Corner cut:** `project.approval.agent_autonomous_for.assurance_levels: [A, B, C, D, E]` was set so the agent could self-approve all 60+ requirements and clear `approvals_current` without human action. That defeats the governance gate the assurance class exists to enforce.

**Proper fix:** Remove the `agent_autonomous_for` block entirely from `proof.yaml`. Approvals must then come from actual human reviewers via `proof approve <REQ-ID> --role <role> --comment "..."`. The PR review process itself becomes the human-approval mechanism. Expect `approvals_current` findings to reappear; they're the *real* state.

**Acceptance:** `agent_autonomous_for` removed. Approvals only happen by named human reviewers. Approval findings either resolved by real approvals or escalated to the responsible person with the exact command, per `proof help agent-rules`.

### 0.2 Raise MC/DC thresholds to a principled bar

**Corner cut:** `code_mcdc.min_decision_percent: 55, min_condition_percent: 60` were chosen to match the *floor of achieved coverage across analytics/serializer/retry*, not because 55/60 is a principled MC/DC bar.

**Proper fix at Class B:** target **95% decisions / 95% conditions** (the NPR 7150.2 Class B / DO-178C Level B vicinity). Reach it by:
- Writing additional tests for the un-covered branches that are genuinely coverable (incrementAggregate's deep field-switch independence pairs, serializer's TransformSingleRecordToProto field-by-field paths, retry's net-error unwrap chains).
- Adding `//mcdc:ignore <specific reason>` annotations on each *remaining* genuinely-unreachable branch (defensive `!ok` type assertions, error paths gated on impossible inputs, etc.) — each ignore reviewed and reasoned, not bulk-applied.
- If after honest test + ignore work the ceiling is still below 95% (likely — GeoIP plus some external-dep branches), the residual gap becomes a Phase E `known-issue` linked to the verify-stage requirement, not a silently-lowered threshold.

**Acceptance:** thresholds set to 95/95. Either the audit passes them, or the gap is recorded as a named `known-issue` per affected package/function with a concrete remediation plan.

### 0.3 Broaden MC/DC scope to all unit-testable packages

**Corner cut:** `code_mcdc.targets` only covers `./analytics`, `./serializer`, `./retry`. `./storage`, `./logger`, `./server`, and the root package (which has logic in `filterData`, `writeToPumps`, `execPumpWriting`, `checkShutdown`, `PreprocessAnalyticsValues`) are not measured. Class B doesn't allow that selective scope without justification.

**Proper fix:** add MC/DC targets for `./storage`, `./logger`, `./server`, and root (`.`). Each gets its own per-target named test command in `project.commands.tests`. The pump packages stay out *only because their tests genuinely require external services we don't run in CI* — and that exclusion is recorded as a Phase E `known-issue` with the exact services and the exact reqs whose evidence is therefore environment-conditional.

**Acceptance:** All non-pump packages with unit tests are MC/DC-measured. The pump exclusion is a documented known-issue, not an unspoken scope choice.

### 0.4 Replace generator-only documentation with hand-authored per-req docs

**Corner cut:** `documentation_coverage` flipped from 0% → 100% via `proof doc generate trace-matrix --format md` followed by `proof trace autolink`. The trace matrix is a list of req IDs — not documentation. Every req shows as documented; none has actual prose explaining it.

**Proper fix:** for each requirement, write hand-authored documentation (under `docs/requirements/<REQ-ID>.md`) covering: intent, motivation, relevant code references, evidence summary, open questions. Generated artifacts (req-summary.md, trace-matrix.md) stay as additional navigation aids but no longer count as the primary `documented_by` target. If the reqproof tool can't distinguish them today (it can't), file a follow-up on probelabs/reqproof#81 to request the hand-vs-auto distinction, then enforce locally via folder convention until the tool catches up.

**Acceptance:** 63+ hand-authored doc files exist under `docs/requirements/`, each one explaining one requirement in prose. The trace matrix still exists but is no longer the *only* `documented_by` link.

### 0.5 Replace bulk review-acceptance with per-finding judgment

**Corner cut:** `proof trace review --all`, `proof gaps review --scope active --check decomposition --req <every-id>`, and `proof review impact --decision no-authored-change --reason "..." <every-id>` were applied in bulk loops over all 60+ reqs at once. That clears the suspect/decomposition/authored-delta checks without per-finding judgment — exactly the rubber-stamping pattern `proof help agent-rules` warns against.

**Proper fix:** these reviews are done per-requirement (or per-tight-group) with reasons that name what was actually reviewed:
- `proof trace review --suspect` instead of `--all`, with the agent (or human) reading each suspect link before accepting it.
- `proof gaps review --check decomposition --req <ID> --reason "..."` per-parent, with the reason naming which children cover which checklist obligation.
- `proof review impact <REQ-ID> --decision no-authored-change --reason "..."` per affected requirement, with the reason naming the file and what changed.

For a 60-req spec this is dozens of small judgments, not one shell loop. Expect more findings to surface and require actual resolution.

**Acceptance:** No more `--all` or "loop over every req with the same reason" usages. Each review-acceptance reason names a concrete artifact and the basis for acceptance.

### 0.6 Revisit FRETish-vs-description on the SYS reqs I judged "acceptable abstractions"

**Corner cut:** during the re-review I split SYS-010/011 and strengthened SYS-009, but judged SYS-001, SYS-003, SYS-006, SYS-008, and SYS-014 as having "acceptable" abstractions despite descriptions encoding richer logic than the FRETish formula captured. At Class B this is too generous.

**Proper fix:** for each, either strengthen the FRETish to reflect the additional behaviors, or split into atomic SYS reqs. Concretely:

| Req | Hidden behavior | Proper fix |
|---|---|---|
| SYS-001 "consume on fixed configurable interval and forward to every backend" | interval bound (`purge_delay_seconds`), backend fan-out multiplicity | Strengthen FRETish with bounded timing (`shall within purge_delay_seconds satisfy ...`); split fan-out into its own SYS req if FRETish doesn't naturally express multiplicity |
| SYS-003 "time-windowed aggregated analytics" | windowing mode (hourly/per-minute) + per-org/per-API dimensions + counter set | Split: SYS-003a (windowing config), SYS-003b (dimension aggregation), SYS-003c (counter set emitted) |
| SYS-006 "retry transient failures with bounded exponential backoff" | bound is `max_retries`; backoff is exponential | Either add `req_type: assumption` reqs for the backoff-library guarantee + a derived SYS with the count bound, or accept that FRETish can't express counted-retries and surface the gap as a `known-issue` |
| SYS-008 "load JSON config then env-override every field" | two distinct phases | Split: SYS-008a (JSON load), SYS-008b (env override) |
| SYS-014 "consume uptime data and forward unless disabled" | two behaviors: consume + forward | Split: SYS-014a (consume from `tyk-uptime-analytics`), SYS-014b (forward to uptime backend), with the "unless disabled" as a precondition on both |

**Acceptance:** Every SYS req's FRETish captures every shall-claim in its description, or the description is split until they match.

### 0.7 Mark `req_type` correctly across the existing spec

**Corner cut:** every existing req is `req_type: guarantee`. SYS reqs with `parent: STK-REQ-X` and `traces.satisfies: STK-REQ-X` are structurally derived. SW reqs the same. None of the trust-boundary assumptions are marked.

**Proper fix:** sweep all 63 reqs and assign:
- `req_type: derived` for any req with `parent` or `traces.satisfies` populated (covers most SYS/SW/INT).
- `req_type: guarantee` reserved for original-intent STK reqs (and any genuinely original SYS, but I don't think we have any once derived is applied).
- `req_type: constraint` for any new constraint reqs added in Phase B.
- `req_type: assumption` for any new assumption reqs added in Phase B.

### Phase 0 sequencing

Items 0.1, 0.5, 0.7 are config / metadata changes (fast). Items 0.2, 0.3, 0.4, 0.6 are real engineering work (slow). Tackle the metadata first so the audit reveals the real state we're working from, then chip away at the engineering items in parallel with Phase A.

---

## Phase A — Per-implementation + per-significant-behavior decomposition (assurance class B, no downgrade)

**Why:** Closes the gap the senior review correctly identified — family-level SW reqs (SW-018 covers 6 mongo files, SW-019 covers 6 sql files, etc.) obscure substantive distinct logic. At Class B the decomposition must reflect how the system can independently fail.

### A.1 Per-implementation split of the family SW reqs

| Current family req | Splits into | New SW count |
|---|---|---|
| SW-018 (mongo) | `mongo-standard`, `mongo-selective`, `mongo-aggregate`, `graph-mongo`, `mcp-mongo`, `mcp-mongo-aggregate` | 6 |
| SW-019 (sql) | `sql-standard`, `sql-aggregate`, `graph-sql`, `graph-sql-aggregate`, `mcp-sql`, `mcp-sql-aggregate` | 6 |
| SW-022 (influx) | `influx-v1` (line protocol), `influx-v2` (HTTP API) | 2 |
| SW-027 (http-logging) | `splunk-hec`, `graylog-gelf`, `syslog`, `saas-http-api` (Logz.io / Moesif / Segment / Resurface) | 4 |
| SW-028 (aws) | `aws-sqs`, `aws-kinesis`, `aws-timestream` | 3 |

Net: **+16 SW-REQs** at the per-implementation level alone.

### A.2 Per-significant-behavior split for the complex aggregates

Each aggregate pump and the elasticsearch pump get one SW req per substantive behavior, not one per file:

- **mongo-aggregate** (current SW-018 lumps this): windowing (hourly vs per-minute, configurable `AggregationTime`), per-org collection sharding (`UseMixedCollection`), `$inc` counter semantics, tag-list bounding + prefix collapsing (`ThresholdLenTagList`), **self-healing on max-doc-size** (`EnableAggregateSelfHealing` halves `AggregationTime`), index ensure. **+6 reqs**.
- **sql-aggregate** (current SW-019 lumps this): day-bucket batching with split-on-date-boundary, dimension table ensure, `CREATE INDEX IF NOT EXISTS` ensure, on-conflict assignment semantics. **+4 reqs**.
- **elasticsearch** (current SW-020 is too thin): per-ES-version bulk processor dispatch (v3/v5/v6/v7), rolling-index naming, bulk flush size/interval boundary. **+3 reqs**.

### A.3 Re-annotation + audit close

Re-annotate code per new req IDs (file-by-file, with per-symbol granularity where one file has multiple substantive behaviors). Re-autolink. Per-req impact-review (per item 0.5 of Phase 0). Vacuity check on new FRETish. `proof audit` → 0/0 honestly (with corners removed per Phase 0).

### Acceptance

- Total SW-REQs: ~62 (from 33).
- Every `implemented_by` set has 1 distinct file (or a small tightly-coupled cluster of ≤2 files where the cluster represents one behavior).
- Aggregate-pump obligations include `idempotency` (re-aggregation must not double-count) and `denial_of_service_resistant` (self-healing IS this) where the code supports it.
- `obligation_decomposition_complete` passes per-pair without bulk review acceptance.

---

## Phase B — Add missing requirement categories with REAL targets (blocked on input)

**Why:** Currently 60/60 reqs are `category: functional`. NPR 7150.2 / IEEE 29148 expect performance/reliability/constraint/assumption representation. **Performance and reliability reqs are blocked on real targets** — no stubs allowed.

### B.1 Performance reqs (gated on Tyk SRE input)

Need real numbers from Tyk SRE / existing SLO docs:
- Per-pump-family throughput target (records/sec at <P latency).
- End-to-end purge latency budget (gateway-write → pump-write).
- Max memory under N pending records.
- Backpressure threshold + graceful shedding behavior.

This phase **does not start** until those numbers exist. If they don't exist anywhere today, the dependency is "Tyk SRE produces internal SLO targets for tyk-pump" — that becomes a tracked input.

### B.2 Reliability reqs

Same shape — needs real reliability targets (MTBF assumptions, recovery time, degraded-mode behavior). May be partially derivable from architecture but the numerical targets need SRE input.

### B.3 Constraint reqs

These don't need external input — derive from `go.mod`, `Makefile`, deployment docs:
- Go ≥ 1.25 (from `go.mod`)
- Redis-compatible (v5+) temporal store (from storage code)
- FIPS build availability (from `Makefile build-fips` target)
- OS support matrix (from CI workflows)
- Deployment model (single-process)

Each as `req_type: constraint`.

### B.4 Assumption reqs (highest-leverage new category)

Each trust boundary becomes an explicit `req_type: assumption`:
- "The Redis-compatible temporal store provides atomic `LPOP` semantics" — assumption that SYS-007 depends on.
- "The MongoDB driver guarantees atomic `$inc` on a single document" — assumption that mongo-aggregate counters depend on.
- "The Tyk gateway serializes records with msgpack or protobuf on the analytics keys `tyk-system-analytics` and `tyk-system-analytics_0..9`" — assumption that INT-001 depends on.
- "The MaxMind GeoIP database is operator-supplied and not always present" — assumption that `GetGeo` / `GeoIPLookup` depend on.
- "The mongo client returns capped-collection size as `int64`" — assumption currently violated under mongo:6 (becomes a Phase E known-issue too).

Link each assumption to the SYS/SW reqs that depend on it via the trace graph. This is where the spec earns its keep — making invisible trust visible.

### Acceptance

- Each non-functional category has ≥1 req.
- Category distribution visible (will need to track manually until reqproof issue #81 item 3 lands).
- Every "the system trusts X" claim hidden in a description today is surfaced as an explicit `req_type: assumption`.
- Performance/reliability reqs use real numbers from Tyk SRE.

---

## Phase C — Promote INT-REQs to formal ICDs (do alongside B)

**Why:** Current INT reqs are thin one-line descriptions. NPR 7150.2 ICDs are caller/callee/schema-version/error-semantics/back-compat/retirement.

### Deliverables

Each existing INT-REQ becomes a full ICD entry using the Phase 2 interface metadata block:

| INT-REQ | ICD content |
|---|---|
| INT-001 gateway→pump record schema | Exact key constants (`storage.ANALYTICS_KEYNAME = "tyk-system-analytics"` plus `_0..9` suffix model), serialization formats (msgpack default suffix `""`, protobuf suffix `_protobuf`), schema version (1), unknown-fields-tolerated policy, deprecation/retirement policy |
| INT-002 gateway→pump uptime | Same shape with `storage.UptimeAnalytics_KEYNAME = "tyk-uptime-analytics"` |
| INT-003 serializer wire format | Per-format field map, optional-field policy, version negotiation (currently **none** — flag as a known-issue in Phase E and a risk in Phase D's FMEA) |
| INT-004 Pump interface | Pin to `pumps/pump.go` Pump type; version = N/A (in-process); breaking-change policy = major version of tyk-pump |
| INT-005 AnalyticsStorage contract | `GetAndDeleteSet` atomicity guarantee (Pop is atomic; re-Expire is best-effort), chunk-size semantics (0 → -1 in temporal-storage impl), expire semantics |
| INT-006 pump→backend record schema | **Split per backend family** following Phase A's decomposition — one ICD per backend (mongo collection schema, sql table schema, ES document mapping, etc.) |
| INT-007 SQL schema migration | Expand-then-contract policy, day-sharded table naming `tyk_analytics_YYYYMMDD`, index naming, on-conflict assignment table |
| INT-008 pump config meta | Per-pump mapstructure schema reference (one entry per pump type) |

Use `proof interface inspect` to validate. Adopt the Phase 2 interface metadata block, which PR #1022 didn't.

### Acceptance

- Every INT-REQ has a populated `interface:` block with caller/callee/version/back-compat/retirement.
- `proof interface inspect` reports all reqs as valid.
- INT-006 has been re-split per backend family, matching Phase A's decomposition.

---

## Phase D — FMEA + obligation re-tune (do after A, B, C settle)

**Why:** Obligation choices were chosen heuristically in PR #1022. After FMEA they should reflect actual risk exposure.

### Deliverables

1. **One-page FMEA** under `docs/fmea.md`:
   - Failure modes per subsystem (purge loop hangs, storage unreachable, pump panics, aggregate doc-size overflow, mongo ctx-dropped timeout — see Phase E known-issue E1, etc.).
   - Consequence severity (S/M/H).
   - Detection mechanism.
   - Mitigation (which code paths handle it, or which known-issue tracks the gap).
   - Owning requirement (link to REQ-ID).
2. **Obligation re-tune** based on FMEA findings:
   - mongo-aggregate self-healing → add `denial_of_service_resistant` obligation.
   - Aggregate counters → add `idempotency` obligation.
   - SQL day-sharded routing → verify `forward_compatible` is load-bearing per FMEA.
   - Storage reconnect → consider `cancellation_observed`.
3. Add `proof risk` records (`.proof/risks/`) for any accepted residual risks the FMEA identifies — with named owner and expiry. Different artifact from `known-issue`: `risk` is "we accept this," `known-issue` is "we know this is broken and will fix it."

### Acceptance

- Every High-severity FMEA row has a linked REQ-ID with an obligation that addresses it.
- Accepted residual risks are formal `AcceptedRisk` records with expiry.
- Mongo ctx-dropped timeout (Phase E known-issue E1) appears in the FMEA as a real failure mode with owner.

---

## Phase E — Log known-issues for the real defects found during coverage (independent track)

**Why:** Per user directive, bugs found during the proof exercise are logged as `proof known-issue` artifacts linked to the requirements they violate, not fixed in this stream. This makes the spec-vs-code mismatch first-class auditable.

### Deliverables

Three `proof known-issue` records under `.proof/known-issues/`:

| # | Title | Violated req | Owner | Notes |
|---|---|---|---|---|
| E1 | `mongo-pump-ignores-caller-context` | **SYS-REQ-005** (per-pump write timeout) and **INT-REQ-004** (Pump interface contract surfaces write errors via ctx) | tyk-team | `pumps/mongo.go MongoPump.WriteData` calls `m.store.Insert(context.Background(), dataSet...)`, silently dropping the per-pump timeout context set by `main.go:execPumpWriting`. Per-pump timeout is *not* enforced for mongo writes. Workaround: operators relying on per-pump timeout for mongo today have no recourse. |
| E2 | `mongo-test-panic-under-mongo6` | **SW-REQ-018 / mongo-standard evidence** | tyk-team | `pumps/mongo_test.go:316 TestMongoPump_capCollection_OverrideSize` panics with `interface conversion: interface {} is int, not int64` when run against mongo:6. Mongo client v6 returns capped-collection size as `int`; the test asserts `int64`. Crashes the whole pumps package run. |
| E3 | `env-prefix-const-typo` | **SW-REQ-002** (config loader) | tyk-team | Const `ENV_PREVIX` in `config.go:282` is a typo for `ENV_PREFIX`. Cosmetic — the value is correct so behavior is fine — but the misspelled identifier propagates to any generated docs/help. |

Each known-issue carries:
- Description with file:line reference.
- Linked violated requirements via the trace graph.
- Severity (E1 = M, E2 = M, E3 = L).
- Owner = tyk-team.
- Target fix release: not committed; field left for tyk-team to populate.
- For E1: also serves as `ship_with_known_issue` evidence carve-out on SYS-REQ-005's verification plan so the audit reflects honest state.

### Acceptance

- Three `KnownIssue` YAML files exist under `.proof/known-issues/`.
- Each is linked to the requirement(s) it violates.
- E1 appears in the Phase D FMEA as a real failure mode (per-pump timeout claim is false for mongo).
- The audit reflects the spec-vs-code mismatch honestly via `verification_plan_evidence`'s `ship_with_known_issue` mechanism.

---

## Phase F — Contribute back to reqproof (independent track, different repo)

**Why:** Offered on [reqproof#81](https://github.com/probelabs/reqproof/issues/81). Closing the loop benefits the next agent that runs the same exercise. Independent of tyk-pump work.

### Deliverables

1. PR against probelabs/reqproof implementing `implemented_by_breadth_clean` check (configurable thresholds for distinct files + symbols). Test case: re-run against pre-Phase-A state of tyk-pump and confirm it flags exactly the broad SW-REQs.
2. PR against probelabs/reqproof extending `spec_lint_req_type_vs_description` (or sibling `spec_lint_req_type_vs_structure`) to flag `guarantee` when `satisfies`/`parent` is set.
3. PR for the in-line nudge in `proof init` output suggesting Class C for analytics/infrastructure projects (or whatever the questionnaire concludes).
4. PRs for the in-line nudges per the issue addendum — per-trigger one-line reminders at the moment of decision (highest leverage of all).

### Acceptance

Each PR has a test case derived from the tyk-pump spec — re-running against the original (pre-decomposition) state of PR #1022 should flag exactly the broad SW-REQs that shipped without the check.

---

## Sequencing

```
Phase 0 (remove corners)                  [START HERE]
    └─→ Phase A (decompose)               [primary content work]
            └─→ Phase B (categories)      [in parallel, gated on real targets]
            └─→ Phase C (formal ICDs)     [in parallel]
                            └─→ Phase D (FMEA + obligation tune)

Phase E (log known-issues)      [independent, can run anytime after Phase 0]
Phase F (reqproof PRs)          [independent, anytime — community benefit]
```

**Strict ordering:** Phase 0 must complete before Phase A starts. The corners removed in Phase 0 will re-surface findings (real approval gates, real coverage gaps, real review judgments needed) — Phase A's decomposition must work against the *honest* state, not the workaround-green state.

**Soft ordering:** Phase E can begin immediately in parallel with Phase 0 since logging known-issues doesn't depend on spec changes. Phase F is fully independent.

---

## Open questions (need decisions before kickoff)

These are reduced from v1 — the "no corners" directive already settled the assurance-class question and rules out stubbing. What remains:

1. **Phase 0.1 (remove agent-autonomous approval).** Removing the autonomy block will leave 63 reqs needing real human approval. The PR review of this work *is* a human-approval event but the tool doesn't know that. Two options:
   - (a) Have a named reviewer (you?) run `proof approve <REQ-ID> --role <role> --comment "approved as part of PR review"` per req, possibly via a small wrapper script.
   - (b) Configure `proof.yaml` so the approval gate is satisfied by an external signal (commit-signed PR merge into main) — would require a reqproof feature that doesn't exist today.

   For this PR I'd default to (a). Confirm.

2. **Phase 0.2/0.3 (MC/DC bar + scope).** Confirming 95/95 as the Class B bar with `known-issue` carve-outs for the residual. Or are you okay with a different principled bar (e.g., the DO-178C Level A "100%" expectation)? The higher the bar, the more incremental test work.

3. **Phase 0.4 (hand-authored docs).** 63 hand-authored doc files is real writing work. Confirm scope: full prose per req, or template-driven structured fields (intent / motivation / code refs / evidence / open questions) that I fill in?

4. **Phase B perf targets.** Who owns producing them — you, Tyk SRE, someone else? Phase B blocks until they exist; that's a hard gate per "no corners." If they don't exist anywhere, do we hold Phase B until SRE produces them, or do we accept the gate and document the dependency?

5. **Phase E known-issue ownership.** I've defaulted owner = "tyk-team" on all three. Is there a more specific owner (mongo pump maintainer, etc.) you'd prefer?

6. **Phase F upstream timing.** Green-light to start the reqproof PRs in parallel with this work, or queue them until tyk-pump's Phase D is done so the reqproof contributions are informed by the full content review?

Lock the answers and this becomes the formal next-goal plan.
