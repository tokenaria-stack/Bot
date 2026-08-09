/**
 * Debt #69A — ColumnarStore memory budget (Node).
 * Track A Step 1 — VIEW-preserving prune (WS-01…WS-03).
 * Track B Step 1 — Mutation Set survives same-operation growth prune (CL-05).
 * Track B Step 2 — Retained Neighborhood survives across exploration growth.
 * Track B Step 3 — Lazy Contract: exploration events must not contract RN.
 * Run: node web/columnar-store_budget_test.js
 */
const { ColumnarStore } = require('./columnar-store.js');

function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

function fillStore(store, n, plotIds = ['line_rsx', 'line_woz']) {
  const times = [];
  const open = [];
  const high = [];
  const low = [];
  const close = [];
  const volume = [];
  const plots = {};
  for (const id of plotIds) plots[id] = [];
  for (let i = 0; i < n; i++) {
    const t = 1_700_000_000 + i * 60;
    times.push(t);
    open.push(100 + i);
    high.push(101 + i);
    low.push(99 + i);
    close.push(100.5 + i);
    volume.push(10);
    for (const id of plotIds) plots[id].push(50 + (i % 10));
  }
  const annotations = [
    { time: times[0], text: 'A' },
    { time: times[Math.floor(n / 2)], text: 'B' },
    { time: times[n - 1], text: 'C' },
  ];
  // World-fill helper: commit-paired so test fixtures can exercise prune without Mutation Set.
  store.replaceMonolith({
    times,
    candles: { open, high, low, close, volume },
    plots,
    annotations,
    timeframe: '1m',
  }, { commitPaired: true });
}

function makeOlderChunk(count, endExclusiveSec) {
  const older = {
    times: [],
    candles: { open: [], high: [], low: [], close: [], volume: [] },
    plots: { line_rsx: [], line_woz: [] },
    annotations: [],
  };
  for (let i = 0; i < count; i++) {
    const t = endExclusiveSec - (count - i) * 60;
    older.times.push(t);
    older.candles.open.push(1);
    older.candles.high.push(1);
    older.candles.low.push(1);
    older.candles.close.push(1);
    older.candles.volume.push(1);
    older.plots.line_rsx.push(1);
    older.plots.line_woz.push(1);
  }
  return older;
}

// Force tiny budget for fast tests via prototype override of getters.
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 100 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 120 });

const store = new ColumnarStore();
fillStore(store, 50);
assert(store.barCount() === 50, 'under hard cap stays intact');
assert(store.windowMode === 'live', 'replaceMonolith sets live');
assert(store.invariantOk(), 'invariant under cap');

// Soft replace over HARD_CAP: Mutation Set covers payload → no TARGET amputate
fillStore(store, 150);
assert(store.barCount() === 150, `soft replace retains full payload, got ${store.barCount()}`);
assert(store.windowMode === 'live', 'soft replace stays live');
assert(store.invariantOk(), 'invariant after soft over-cap');
assert(store.windowStartSec() === store.firstTimeSec(), 'windowStart getter');
assert(store.windowEndSec() === store.lastTimeSec(), 'windowEnd getter');

// Commit-paired over HARD_CAP still needs a FROM_OLDEST prune path for live growth —
// exercise via appendTick (below) and preserve-paired prepend. Soft fill leaves tip intact:
const firstAfterSoft = store.firstTimeSec();
assert(firstAfterSoft === 1_700_000_000, 'soft over-cap keeps oldest payload candle');

// Prepend past hard cap → FROM_NEWEST → history mode (no VIEW bounds = legacy path)
const live = new ColumnarStore();
fillStore(live, 100);
const older = makeOlderChunk(50, live.firstTimeSec());
const beforePrependLast = live.lastTimeSec();
live.prependMonolith(older);
assert(live.barCount() === 100, `prepend prune to target, got ${live.barCount()}`);
assert(live.windowMode === 'history', 'FROM_NEWEST sets history mode');
assert(live.invariantOk(), 'invariant after FROM_NEWEST');
assert(live.lastTimeSec() < beforePrependLast, 'newest tip pruned away');
assert(live.firstTimeSec() < firstAfterSoft, 'older history retained on left');

// Debt #69C: focal nearer right → FROM_OLDEST (keep tip)
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 100 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 120 });
const focalRight = new ColumnarStore();
fillStore(focalRight, 100);
const older2 = makeOlderChunk(50, focalRight.firstTimeSec());
const tipBefore = focalRight.lastTimeSec();
const r = focalRight.prependMonolith(older2, { focalTimeSec: tipBefore, atLiveEdge: false });
assert(r.pruneDirection === ColumnarStore.PRUNE_FROM_OLDEST, 'focal at right → FROM_OLDEST');
assert(focalRight.windowMode === 'live', 'FROM_OLDEST keeps live window');
assert(focalRight.lastTimeSec() === tipBefore, 'live tip retained when focal on right');
assert(focalRight.barCount() === 100, 'still at target');
assert(focalRight.invariantOk(), 'invariant focal-right prune');

// Debt #69C: atLiveEdge forces FROM_OLDEST
assert(
  ColumnarStore.pruneDirectionFromFocal(100, 200, 110, { atLiveEdge: true })
    === ColumnarStore.PRUNE_FROM_OLDEST,
  'atLiveEdge forces OLDEST',
);
assert(
  ColumnarStore.pruneDirectionFromFocal(100, 200, 105, {})
    === ColumnarStore.PRUNE_FROM_NEWEST,
  'focal near left → NEWEST',
);
assert(
  ColumnarStore.pruneDirectionFromFocal(100, 200, 195, {})
    === ColumnarStore.PRUNE_FROM_OLDEST,
  'focal near right → OLDEST',
);

// appendTick growth prune (live mode)
const tip = new ColumnarStore();
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 10 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 12 });
global.chartTime = (t) => Number(t);
tip.setTfInterval(60);
fillStore(tip, 13);
assert(tip.barCount() === 13, 'soft replace retains over hard cap (Mutation)');
tip.windowMode = 'live';
const base = tip.lastTimeSec();
for (let i = 1; i <= 5; i++) {
  tip.appendTick({
    time: base + i * 60,
    open: 1, high: 2, low: 1, close: 1.5, volume: 1,
    plots: { line_rsx: 40, line_woz: 41 },
  });
}
assert(tip.barCount() <= 12, `appendTick enforces hard cap, got ${tip.barCount()}`);
assert(tip.windowMode === 'live', 'append prune keeps live');
assert(tip.invariantOk(), 'invariant after appendTick budget');

// appendTick preserve-paired: VIEW covering oldest must survive FROM_OLDEST budget
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 10 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 12 });
const appendView = new ColumnarStore();
global.chartTime = (t) => Number(t);
appendView.setTfInterval(60);
fillStore(appendView, 12);
assert(appendView.barCount() === 12, 'at hard cap before append');
const oldest = appendView.firstTimeSec();
const viewTo = appendView.timesSec()[5];
const tipSec = appendView.lastTimeSec();
for (let i = 1; i <= 5; i++) {
  appendView.appendTick({
    time: tipSec + i * 60,
    open: 1, high: 2, low: 1, close: 1.5, volume: 1,
    plots: { line_rsx: 40, line_woz: 41 },
  }, { viewFromSec: oldest, viewToSec: viewTo });
}
assert(appendView.firstTimeSec() === oldest, 'WS-02: appendTick must not prune VIEW oldest');
assert(appendView.barCount() >= 6, 'at least VIEW span retained');
assert(appendView.invariantOk(), 'invariant appendTick VIEW prune');

// applyProjection preserve-paired: VIEW tip retained when over hard cap
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 100 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 120 });
const proj = new ColumnarStore();
fillStore(proj, 150);
const projFrom = proj.firstTimeSec();
const projTo = proj.lastTimeSec();
// Re-apply same-shaped monolith via applyProjection with full VIEW
const snap = proj.snapshot();
const proj2 = new ColumnarStore();
proj2.applyProjection({
  times: snap.times,
  candles: snap.candles,
  plots: snap.plots,
  annotations: snap.annotations,
}, { viewFromSec: projFrom, viewToSec: projTo });
assert(proj2.lastTimeSec() === projTo, 'applyProjection preserve-paired keeps VIEW tip');
assert(proj2.barCount() === 150, 'soft applyProjection: Mutation∪VIEW retain full series over HARD_CAP');
assert(proj2.invariantOk(), 'invariant applyProjection VIEW');

// ── Track A Step 1: VIEW-preserving prune (WS-01…WS-03 / S1–S3 / E3-01) ──
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 100 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 120 });

const fullView = new ColumnarStore();
fillStore(fullView, 100);
const fullFrom = fullView.firstTimeSec();
const fullTo = fullView.lastTimeSec();
const olderFull = makeOlderChunk(50, fullFrom);
fullView.prependMonolith(olderFull, {
  viewFromSec: fullFrom,
  viewToSec: fullTo,
  focalTimeSec: fullFrom,
  atLiveEdge: false,
});
assert(fullView.lastTimeSec() === fullTo, 'WS-02: full VIEW tip must survive prune');
assert(fullView.firstTimeSec() === olderFull.times[0], 'CL-05: Mutation Set (just-prepended) must survive');
assert(fullView.barCount() === 150, 'VIEW∪Mutation → nothing eligible; may exceed TARGET');
assert(fullView.invariantOk(), 'invariant after VIEW-preserving prune');

const leftView = new ColumnarStore();
fillStore(leftView, 100);
const leftFrom = leftView.firstTimeSec();
const mid = leftView.timesSec()[40];
const tipLeft = leftView.lastTimeSec();
const olderLeft = makeOlderChunk(50, leftFrom);
leftView.prependMonolith(olderLeft, {
  viewFromSec: leftFrom,
  viewToSec: mid,
  focalTimeSec: leftFrom,
  atLiveEdge: false,
});
assert(leftView.lastTimeSec() < tipLeft, 'tip outside VIEW may still prune (FROM_NEWEST)');
assert(leftView.firstTimeSec() === olderLeft.times[0], 'Mutation Set left retained');
assert(leftView.barCount() === 100, 'budget target when VIEW∪Mutation leave eligible room');
assert(leftView.invariantOk(), 'invariant left-VIEW prune');

const bounds = ColumnarStore.logicalRangeToViewTimes(
  [10, 20, 30, 40, 50],
  { from: 1.2, to: 3.8 },
);
assert(bounds.viewFromSec === 20 && bounds.viewToSec === 40, 'logicalRangeToViewTimes maps floors');

// ── Track B Step 1: Mutation Set same-operation prune guard (CL-05) ──
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 100 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 120 });

// prepend: growth → Mutation Set must not be discarded by that prune (thrash case)
const mutPre = new ColumnarStore();
fillStore(mutPre, 100);
const mutPreFrom = mutPre.firstTimeSec();
const mutPreTo = mutPre.lastTimeSec();
const mutOlder = makeOlderChunk(50, mutPreFrom);
mutPre.prependMonolith(mutOlder, {
  viewFromSec: mutPreFrom,
  viewToSec: mutPreTo,
  focalTimeSec: mutPreFrom,
  atLiveEdge: false,
});
assert(mutPre.firstTimeSec() === mutOlder.times[0], 'TB1 prepend: Mutation Set retained');
assert(mutPre.lastTimeSec() === mutPreTo, 'TB1 prepend: VIEW tip retained');
assert(mutPre.barCount() === 150, 'TB1 prepend: protected sets block same-op discard');

// appendTick: newly introduced tip is Mutation Set — survives even when VIEW is left-only
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 10 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 12 });
global.chartTime = (t) => Number(t);
const mutAppend = new ColumnarStore();
mutAppend.setTfInterval(60);
fillStore(mutAppend, 12);
const mutOldest = mutAppend.firstTimeSec();
const mutViewTo = mutAppend.timesSec()[4];
const mutTip = mutAppend.lastTimeSec();
const newTip = mutTip + 60;
mutAppend.appendTick({
  time: newTip,
  open: 1, high: 2, low: 1, close: 1.5, volume: 1,
  plots: { line_rsx: 40, line_woz: 41 },
}, { viewFromSec: mutOldest, viewToSec: mutViewTo });
assert(mutAppend.lastTimeSec() === newTip, 'TB1 append: Mutation Set tip retained');
assert(mutAppend.firstTimeSec() === mutOldest, 'TB1 append: VIEW oldest retained');
assert(mutAppend.invariantOk(), 'TB1 append invariant');

// applyProjection (preserve/growth): Mutation Set = entire applied series
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 100 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 120 });
const mutProj = new ColumnarStore();
const bigTimes = [];
const bigCandles = { open: [], high: [], low: [], close: [], volume: [] };
for (let i = 0; i < 150; i++) {
  const t = 1_800_000_000 + i * 60;
  bigTimes.push(t);
  bigCandles.open.push(1);
  bigCandles.high.push(1);
  bigCandles.low.push(1);
  bigCandles.close.push(1);
  bigCandles.volume.push(1);
}
mutProj.applyProjection({
  times: bigTimes,
  candles: bigCandles,
  plots: {},
  annotations: [],
});
assert(mutProj.barCount() === 150, 'TB1 soft applyProjection: Mutation Set blocks same-op prune');
assert(mutProj.firstTimeSec() === bigTimes[0] && mutProj.lastTimeSec() === bigTimes[149], 'TB1 soft series intact');

// replaceMonolith commit-paired: world replace — S6 accepts full monolith (no TARGET amputate)
const commitProj = new ColumnarStore();
commitProj.replaceMonolith({
  times: bigTimes,
  candles: bigCandles,
  plots: {},
  annotations: [],
  hasMore: true,
}, { commitPaired: true });
assert(commitProj.barCount() === 150, 'S6/TB1 commitPaired: retains full payload over HARD_CAP');
assert(commitProj.firstTimeSec() === bigTimes[0], 'S6: oldest retained = payload oldest');
assert(commitProj.lastTimeSec() === bigTimes[149], 'S6: newest retained = payload newest');
assert(commitProj.snapshot().meta.hasMore === true, 'S6: hasMore not forced to EOF by commit-paired accept');
assert(commitProj.windowMode === 'live', 'TB1 commitPaired stays live');
assert(commitProj.invariantOk(), 'TB1 commitPaired invariant');
assert(commitProj.retainedNeighborhoodBounds() == null, 'TB2: commitPaired clears Retained Neighborhood');

// ── Track B Step 2: Retained Neighborhood persists across growth (CL-03/CL-05) ──
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 100 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 120 });

const rnStore = new ColumnarStore();
fillStore(rnStore, 100);
const rnViewFrom = rnStore.firstTimeSec();
const rnViewTo = rnStore.lastTimeSec();
const chunkA = makeOlderChunk(50, rnViewFrom);
const aFirst = chunkA.times[0];
rnStore.prependMonolith(chunkA, {
  viewFromSec: rnViewFrom,
  viewToSec: rnViewTo,
  focalTimeSec: rnViewFrom,
  atLiveEdge: false,
});
assert(rnStore.firstTimeSec() === aFirst, 'TB2 after A: chunk A retained');
const rnAfterA = rnStore.retainedNeighborhoodBounds();
assert(rnAfterA && rnAfterA.fromSec === aFirst, 'TB2: RN absorbed Mutation A');

const chunkB = makeOlderChunk(50, rnStore.firstTimeSec());
const bFirst = chunkB.times[0];
rnStore.prependMonolith(chunkB, {
  viewFromSec: rnViewFrom,
  viewToSec: rnViewTo,
  focalTimeSec: rnViewFrom,
  atLiveEdge: false,
});
// Without RN, second growth would make A eligible and often discard it (multi-op thrash).
assert(rnStore.firstTimeSec() === bFirst, 'TB2 after B: newest left edge is B');
assert(rnStore.timesSec().includes(aFirst), 'TB2: prior Mutation A survives later growth prune');
assert(rnStore.barCount() === 200, 'TB2: VIEW∪RN∪Mutation leave nothing eligible');
const rnAfterB = rnStore.retainedNeighborhoodBounds();
assert(rnAfterB && rnAfterB.fromSec === bFirst && rnAfterB.toSec >= aFirst, 'TB2: RN expanded A∪B');

// World replacement resets RN; S6 retains full commit-paired payload (no TARGET amputate)
rnStore.replaceMonolith({
  times: bigTimes,
  candles: bigCandles,
  plots: {},
  annotations: [],
  hasMore: true,
}, { commitPaired: true });
assert(rnStore.retainedNeighborhoodBounds() == null, 'TB2: world replace clears RN');
assert(rnStore.barCount() === 150, 'S6/TB2: commit-paired retains full payload after RN clear');
assert(rnStore.firstTimeSec() === bigTimes[0] && rnStore.lastTimeSec() === bigTimes[149], 'S6/TB2: series span = payload');
assert(rnStore.snapshot().meta.hasMore === true, 'S6/TB2: hasMore unchanged (not EOF)');

// ── Track B Step 3: Lazy Contract — exploration ≠ contraction ──
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 100 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 120 });

const lazy = new ColumnarStore();
fillStore(lazy, 100);
const lazyViewFrom = lazy.firstTimeSec();
const lazyViewTo = lazy.lastTimeSec();
const lazyA = makeOlderChunk(40, lazyViewFrom);
lazy.prependMonolith(lazyA, {
  viewFromSec: lazyViewFrom,
  viewToSec: lazyViewTo,
  focalTimeSec: lazyViewFrom,
  atLiveEdge: false,
});
const rnBefore = lazy.retainedNeighborhoodBounds();
assert(rnBefore && rnBefore.fromSec === lazyA.times[0], 'TB3 setup: RN after prepend A');
const barsBefore = lazy.barCount();
const firstBefore = lazy.firstTimeSec();

// VIEW narrowing alone — no store contraction API; RN must remain unchanged.
const rnAfterNarrow = lazy.retainedNeighborhoodBounds();
assert(
  rnAfterNarrow
    && rnAfterNarrow.fromSec === rnBefore.fromSec
    && rnAfterNarrow.toSec === rnBefore.toSec,
  'TB3: VIEW narrow must not contract RN',
);
assert(lazy.barCount() === barsBefore && lazy.firstTimeSec() === firstBefore, 'TB3: VIEW narrow does not shrink store');

// Fetch completion alone (no merge) — store untouched.
assert(
  lazy.retainedNeighborhoodBounds().fromSec === rnBefore.fromSec
    && lazy.barCount() === barsBefore,
  'TB3: fetch completion alone must not contract RN',
);

// Successful prepend must not contract previously retained neighborhood.
const lazyB = makeOlderChunk(30, lazy.firstTimeSec());
const b0 = lazyB.times[0];
const a0 = lazyA.times[0];
lazy.prependMonolith(lazyB, {
  viewFromSec: lazyViewFrom,
  viewToSec: lazyViewTo,
  focalTimeSec: lazyViewFrom,
  atLiveEdge: false,
});
const rnAfterPrepend = lazy.retainedNeighborhoodBounds();
assert(rnAfterPrepend.fromSec === b0, 'TB3 prepend: RN expands left');
assert(rnAfterPrepend.toSec >= rnBefore.toSec, 'TB3 prepend: RN toSec never shrinks');
assert(lazy.timesSec().includes(a0), 'TB3 prepend: prior RN members retained');

// Successful append must not contract previously retained neighborhood.
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 10 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 12 });
global.chartTime = (t) => Number(t);
const lazyAppend = new ColumnarStore();
lazyAppend.setTfInterval(60);
fillStore(lazyAppend, 12);
const apOldest = lazyAppend.firstTimeSec();
const apMid = lazyAppend.timesSec()[4];
const apTip = lazyAppend.lastTimeSec();
// Seed RN via a left prepend under larger budget, then shrink budget for append pressure.
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 100 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 200 });
const apOlder = makeOlderChunk(20, apOldest);
lazyAppend.prependMonolith(apOlder, {
  viewFromSec: apOldest,
  viewToSec: apMid,
  focalTimeSec: apOldest,
  atLiveEdge: false,
});
const apRn = lazyAppend.retainedNeighborhoodBounds();
const apLeft = lazyAppend.firstTimeSec();
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 10 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 12 });
const apNew = lazyAppend.lastTimeSec() + 60;
lazyAppend.appendTick({
  time: apNew,
  open: 1, high: 2, low: 1, close: 1.5, volume: 1,
  plots: { line_rsx: 40, line_woz: 41 },
}, { viewFromSec: apOldest, viewToSec: apMid });
const apRnAfter = lazyAppend.retainedNeighborhoodBounds();
assert(apRnAfter.fromSec === apRn.fromSec, 'TB3 append: RN fromSec never shrinks');
assert(apRnAfter.toSec >= apRn.toSec, 'TB3 append: RN toSec expands or holds');
assert(lazyAppend.timesSec().includes(apLeft), 'TB3 append: prior RN left retained');
assert(lazyAppend.lastTimeSec() === apNew, 'TB3 append: new tip retained');

// Projection merge must not contract retained neighborhood (restore omitted RN bars).
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 100 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 200 });
const lazyProj = new ColumnarStore();
fillStore(lazyProj, 80);
const lpFrom = lazyProj.firstTimeSec();
const lpTo = lazyProj.lastTimeSec();
const lpOlder = makeOlderChunk(40, lpFrom);
lazyProj.prependMonolith(lpOlder, {
  viewFromSec: lpFrom,
  viewToSec: lpTo,
  focalTimeSec: lpFrom,
  atLiveEdge: false,
});
const lpRn = lazyProj.retainedNeighborhoodBounds();
const lpLeft = lazyProj.firstTimeSec();
// Soft apply a tip-only projection (omits left RN) — must restore RN bars.
const tipOnlyTimes = lazyProj.timesSec().slice(-50);
const tipCandles = { open: [], high: [], low: [], close: [], volume: [] };
for (let i = 0; i < tipOnlyTimes.length; i++) {
  tipCandles.open.push(1);
  tipCandles.high.push(1);
  tipCandles.low.push(1);
  tipCandles.close.push(1);
  tipCandles.volume.push(1);
}
lazyProj.applyProjection({
  times: tipOnlyTimes,
  candles: tipCandles,
  plots: {},
  annotations: [],
}, { viewFromSec: tipOnlyTimes[0], viewToSec: tipOnlyTimes[tipOnlyTimes.length - 1] });
assert(lazyProj.timesSec().includes(lpLeft), 'TB3 soft apply: omitted RN left restored');
const lpRnAfter = lazyProj.retainedNeighborhoodBounds();
assert(lpRnAfter.fromSec === lpRn.fromSec, 'TB3 soft apply: RN fromSec never shrinks');
assert(lpRnAfter.toSec >= lpRn.toSec, 'TB3 soft apply: RN toSec never shrinks');

// Explicit world replacement still resets correctly.
lazyProj.replaceMonolith({
  times: tipOnlyTimes,
  candles: tipCandles,
  plots: {},
  annotations: [],
}, { commitPaired: true });
assert(lazyProj.retainedNeighborhoodBounds() == null, 'TB3: world replace clears RN');
assert(!lazyProj.timesSec().includes(lpLeft), 'TB3: world replace may drop former RN');

// ── S6 repair: commit-paired retains N > HARD_CAP; preserve-paired pressure unchanged ──
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 100 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 120 });
assert(ColumnarStore.BUDGET_TARGET === 100 && ColumnarStore.BUDGET_HARD_CAP === 120,
  'S6: HARD_CAP/TARGET numeric policy unchanged by repair (test doubles intact)');

const s6N = 200; // > HARD_CAP(120)
const s6Times = [];
const s6Candles = { open: [], high: [], low: [], close: [], volume: [] };
for (let i = 0; i < s6N; i++) {
  s6Times.push(2_000_000_000 + i * 60);
  s6Candles.open.push(1);
  s6Candles.high.push(2);
  s6Candles.low.push(1);
  s6Candles.close.push(1.5);
  s6Candles.volume.push(1);
}
const s6 = new ColumnarStore();
// Seed RN so commit-paired must clear it.
fillStore(s6, 50);
s6.prependMonolith(makeOlderChunk(40, s6.firstTimeSec()), {
  viewFromSec: s6.firstTimeSec(),
  viewToSec: s6.lastTimeSec(),
  focalTimeSec: s6.firstTimeSec(),
  atLiveEdge: false,
});
assert(s6.retainedNeighborhoodBounds() != null, 'S6 setup: RN present before commit-paired');
s6.replaceMonolith({
  times: s6Times,
  candles: s6Candles,
  plots: {},
  annotations: [],
  hasMore: true,
}, { commitPaired: true });
assert(s6.barCount() === s6N, `S6: full monolith retained (${s6.barCount()} vs ${s6N})`);
assert(s6.firstTimeSec() === s6Times[0], 'S6: oldest = payload oldest');
assert(s6.lastTimeSec() === s6Times[s6N - 1], 'S6: newest = payload newest');
assert(s6.snapshot().meta.hasMore === true, 'S6: hasMore stays true (not EOF)');
assert(s6.retainedNeighborhoodBounds() == null, 'S6: RN cleared on commit-paired');
assert(s6.windowMode === 'live', 'S6: commit-paired stays live');
assert(s6.invariantOk(), 'S6: invariant after over-cap commit-paired');

// Preserve-paired pressure still Working-Set-safe (VIEW∪Mutation∪RN; may exceed HARD_CAP)
const s6p = new ColumnarStore();
fillStore(s6p, 100);
const s6pViewFrom = s6p.firstTimeSec();
const s6pViewTo = s6p.lastTimeSec();
const s6pA = makeOlderChunk(50, s6pViewFrom);
const s6pAFirst = s6pA.times[0];
s6p.prependMonolith(s6pA, {
  viewFromSec: s6pViewFrom,
  viewToSec: s6pViewTo,
  focalTimeSec: s6pViewFrom,
  atLiveEdge: false,
});
const s6pB = makeOlderChunk(50, s6p.firstTimeSec());
s6p.prependMonolith(s6pB, {
  viewFromSec: s6pViewFrom,
  viewToSec: s6pViewTo,
  focalTimeSec: s6pViewFrom,
  atLiveEdge: false,
});
assert(s6p.barCount() === 200, 'S6: preserve-paired still retains VIEW∪RN∪Mutation over HARD_CAP');
assert(s6p.timesSec().includes(s6pAFirst), 'S6: preserve-paired still keeps prior Mutation under RN');
assert(s6p.barCount() > ColumnarStore.BUDGET_HARD_CAP, 'S6: preserve over-cap accordion still allowed');

console.log('columnar-store_budget_test: OK');
