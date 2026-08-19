# Stage E5 — Constitution Consistency Report

**Status:** Investigation complete.  
**Kind:** Constitutional audit only (documents as SSOT).  
**Not:** runtime inspection, implementation, repair waves, APIs, or Lifetime implementation.

**Audited (frozen):**

- `WORKING_SET_CONTRACT.md`
- `WORKING_SET_ACCEPTANCE.md`
- `CACHE_LIFETIME_CONTRACT.md`
- `CACHE_LIFETIME_ACCEPTANCE.md`

**Central question:** Are these constitutions complete enough to survive the next five years of architecture?

**Short answer:** The **guarantee cores** (WS-01…WS-05, CL-01…CL-07, P-01/P-02) are coherent and durable enough to survive store/transport/chart/replay replacement, **provided** a few ambiguities and acceptance-doc leaks are treated as known residual debt—not as license to rewrite the laws casually. They are **not** yet “perfectly sealed” against multi-VIEW or LOD identity questions.

---

## 1. Contradictory guarantees?

| Pair | Assessment |
|------|------------|
| WS-01…WS-05 vs CL-01…CL-07 | **No hard contradiction.** CL-01 subordinates Lifetime to Working Set. |
| WS-05 (pressure only outside VIEW) vs CL-03 (VIEW + neighborhood) | **Compatible.** Neighborhood is outside VIEW; CL-03 is stricter retention, not weaker VIEW protection. |
| WS-05 (“memory is an implementation detail”) vs Lifetime as constitution | **Soft tension.** WS-05’s phrase can be misread as “retention has no law.” Intent is “memory must not become navigation.” Lifetime then supplies law for retention *beyond* VIEW. |
| P-01/P-02 owned by WS vs upheld by CL-07 | **No conflict** — define once, uphold elsewhere. |
| Acceptance circularity: WS Gate 1 needs S6/S7; Lifetime L-B needs WS 7/7; S6/S7 need Lifetime | **Process circularity, not law contradiction.** Sequencing should be: WS S1–S5 → Lifetime L* → then S6/S7. |

**Verdict:** No fatal contradictory guarantees in the constitutions. One wording soft-tension (WS-05 “implementation detail”).

---

## 2. Can two engineers produce different architectural behavior?

**Yes — within intended unspecified zones — and in a few ambiguous zones.**

| Topic | Divergence risk | Intended? |
|-------|-----------------|-----------|
| Exploration neighborhood size/shape | High | Yes → Capacity later |
| What counts as “pressure” | High | Yes → Capacity/Emergency |
| When “continuous exploration” still “requires” a neighborhood (CL-05) | Medium | Partially — needs clearer observational test language over time |
| “Equivalent system commits” (WS-03 FreshLive class) | Medium | Open-ended by design; risk of smuggling navigation |
| “Chrome that does not alter market identity” (WS-04) | Medium | Ambiguous for overlays/LOD |
| “Commit-paired world replace” (CL §4) | Medium | Not defined in ownership tables |
| Tip update vs invalidation (WS-03) | Low if tip is unique | Assumes a forming tip concept |

**Verdict:** Independent implementations can differ on Capacity/Emergency and neighborhood sizing without violating the letter of the constitutions. That is acceptable **if** those layers stay outside. Divergence on “system commit” and “market identity” is the main **unintended** variability.

---

## 3. Ownership boundaries explicit?

| Concern | Explicit? | Note |
|---------|-----------|------|
| VIEW / CameraCommit | Yes | TimeCamera |
| Working Set vs Lifetime | Yes | Required data vs retention beyond |
| Store / Compositor / Adapter | Yes | Both constitutions |
| Hydration / fetch | Stronger in Lifetime | WS mentions hydration only in prose |
| Boot / detect-only | No | Relies on ADR-028 assumption |
| Who sizes exploration neighborhood | No | “Unspecified” without naming Capacity as owner of that decision |
| DataResolve outside TimeCamera | Yes (WS) | Good |
| Capacity / Emergency owners | Named as policies, not roles | Intentionally thin |

**Gaps (non-blocking for freeze, worth future doc polish):**

- Name **Capacity policy** as the owner of neighborhood *dimensions* and pressure *thresholds* (not Lifetime).  
- Optionally mention Boot as detect-only in WS ownership (or keep solely in ADR-028).

---

## 4. Does every guarantee belong in exactly one constitution?

| Guarantee | Home | Duplicate? |
|-----------|------|------------|
| VIEW ⊆ retained data | WS-01 | No |
| No prune of VIEW | WS-02 | CL restates via CL-01 (subordination, OK) |
| No invalidation without CameraCommit | WS-03 | CL-01/CL-06 align |
| Paint = VIEW | WS-04 | Lifetime correctly silent |
| Pressure ≠ navigation | WS-05 + CL-06 | **Aligned twin** — acceptable if read as WS (VIEW) + CL (retention work) |
| Eager expand / lazy contract | CL-02 | WS silent (correct) |
| Discard ≠ EOF | CL-04 | WS P-01 related, not duplicate definition |
| Anti-thrash | CL-05 | Feeds P-02; not a second P-02 |
| P-01 / P-02 definitions | WS §4 | CL-07 upholds only |

**Acceptance overlap:** S6/S7 and L7 both observe P-01/P-02 — correct bridging, not dual ownership of the *definition*.

---

## 5. Hidden assumptions

1. **Single committed VIEW** at a time (multi-pane / multi-TF VIEW not modeled).  
2. **Durable fetchable history** with an authoritative end-of-history signal.  
3. **Discrete time-indexed market bars** with stable identity.  
4. **CameraCommit** exists as the only intentional VIEW change mechanism (ADR-028).  
5. **Preserve-paired vs commit-paired** world updates (implied; named mainly in Lifetime §4).  
6. **Forming live tip** is a privileged mutable slot (WS-03).  
7. **Paint and retain planes** can be discussed independently of a specific chart library.  
8. Working Set Acceptance still assumes Track A can drive S6/S7 without stating Lifetime as a hard prerequisite in Gate 1 text (stale relative to Lifetime freeze).

---

## 6. Future features without modifying constitutions?

| Feature | Can add without rewriting constitutions? | Caveat |
|---------|------------------------------------------|--------|
| RESET_LIVE | Yes | As explicit CameraCommit (WS-03 class) |
| ReplayDAG swap | Yes | Out of both contracts |
| Transport / storage swap | Yes | Lifetime stability clause covers this |
| Indicators / overlays | Mostly | Must remain “chrome” under WS-04; if they invent market times, conflict |
| Multi-TF | Partially | Per-TF VIEW OK if still one committed VIEW per surface; true multi-VIEW needs clarification later |
| LOD / downsampling | **Risky without clarification** | Aggregating away bar identity may look like WS-03 invalidation unless LOD is defined as a different plane or an explicit CameraCommit/representation switch |
| Capacity / Emergency policies | Yes | Explicitly outside |
| New indicators math | Yes | Below constitutions |

---

## 7. Capacity and Emergency outside both?

**Yes.** Both constitutions explicitly defer soft/hard limits, OOM, FPS gates. Lifetime even states pressure’s *definition* comes from those policies.

No suggested move of Capacity/Emergency *into* either constitution at this time.

---

## 8. Implementation terminology leaks

| Document | Leaks | Severity |
|----------|-------|----------|
| WORKING_SET_CONTRACT | “today: ColumnarStore”, “LWC”, tip-window example, Stage E3 baseline line | Low–medium (examples / history in a constitution) |
| WORKING_SET_ACCEPTANCE | Track A, #69D, Wave 4, HARD_CAP, 16k/12k, windowMode, BTC, E3 IDs, commit mapping | **High** — this is a **progress gate**, not pure constitution |
| CACHE_LIFETIME_CONTRACT | “timeframe switch”, “commit-paired” | Low |
| CACHE_LIFETIME_ACCEPTANCE | E3 evidence IDs | Low–medium (gate, not law) |

**Distinction:** Contracts ≈ laws; Acceptance docs ≈ compliance gates. Leaks are mostly in Acceptance and in WS Contract illustrative nouns—not in CL invariant statements.

---

## Missing guarantees (optional future polish — not freeze blockers)

1. **Single-VIEW scope statement** — “These constitutions apply per chart surface with one committed VIEW.”  
2. **Representation / LOD clause** — either forbid silent identity replacement or allow an explicit representation CameraCommit.  
3. **Capacity owns dimensions** — one sentence: neighborhood size and pressure thresholds belong to Capacity/Emergency, not Lifetime.  
4. **WS-05 wording** — replace “Memory management is an implementation detail” with “Retention must not become navigation; laws for retention beyond VIEW live in Cache Lifetime.”  
5. **Acceptance sequencing** — Working Set Acceptance Gate 1 should note S6/S7 require Lifetime Acceptance (avoid circular “done”).

---

## Duplicate guarantees

- WS-05 ↔ CL-06 (pressure ≠ navigation): intentional alignment.  
- P-01/P-02 ↔ CL-07 / L7 / S6 / S7: definition once, checks many — OK.  
- No duplicate *conflicting* ownership of required-data vs retention-beyond.

---

## Ambiguous wording (priority)

1. “Exploration neighborhood” (existence without observational criteria beyond CL-05).  
2. “Pressure” (externalized but undefined).  
3. “Equivalent system commits” (WS-03).  
4. “Market identity” / chrome (WS-04).  
5. “Commit-paired world replace” (CL §4).  
6. WS-05 “implementation detail” vs Lifetime constitution.

---

## Suggested wording improvements only

*(Suggestions for a future constitutional errata pass — not applied in this audit.)*

1. **WS-05:** Drop “implementation detail”; point retention-beyond-VIEW to Cache Lifetime.  
2. **WS Purpose:** Prefer “Store (market display data)” without “today: ColumnarStore”; say “chart surface” instead of “LWC” in the ownership table.  
3. **WS-03:** Replace open “equivalent system commits” with “other explicit CameraCommits that intentionally replace VIEW.”  
4. **CL-03:** Add one line: “Dimensions of the exploration neighborhood are Capacity policy, not this contract.”  
5. **CL §4:** Define “commit-paired” as “retention discard or replace that follows an explicit CameraCommit replacing VIEW.”  
6. **WORKING_SET_ACCEPTANCE:** Relabel clearly as Track A progress gate; state S6/S7 Pass requires Lifetime Acceptance; remove or quarantine numeric HARD_CAP/16k examples into evidence annex.  
7. **Both contracts:** Add one-line scope: “One committed VIEW per chart surface.”

---

## Five-year durability verdict

| Layer | Durable? |
|-------|----------|
| WS-01…WS-05 core | **Yes** — survive store/paint/backend swap |
| CL-01…CL-07 core | **Yes** — survive retention-engine swap |
| P-01 / P-02 | **Yes** — product north star |
| Capacity / Emergency deferred | **Correct** |
| Acceptance docs as written | **Progress artifacts** — will age; should not be mistaken for the constitutions |
| Multi-VIEW / LOD identity | **Weakest long-term points** |

**Conclusion:** The constitutions are **complete enough to govern the next architectural era** for a single-VIEW market chart terminal with fetchable history, **if** engineers treat Capacity/Emergency as separate, honor CL subordination, and schedule a light errata pass for the ambiguities above. They are **not** a finished theory of multi-camera or LOD identity—and they should not pretend to be.

---

## Stop

Constitutional consistency audit only. No runtime changes. No Lifetime implementation. No repair waves.
