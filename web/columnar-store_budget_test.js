/**
 * Debt #69A — ColumnarStore memory budget (Node).
 * Track A Step 1 — VIEW-preserving prune (WS-01…WS-03).
 * Track B Step 1 — Mutation Set survives same-operation growth prune (CL-05).
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

fillStore(store, 150);
assert(store.barCount() === 100, `append-path replace prune to target, got ${store.barCount()}`);
assert(store.windowMode === 'live', 'FROM_OLDEST keeps live mode');
assert(store.invariantOk(), 'invariant after FROM_OLDEST');
assert(store.windowStartSec() === store.firstTimeSec(), 'windowStart getter');
assert(store.windowEndSec() === store.lastTimeSec(), 'windowEnd getter');
const firstAfterOldest = store.firstTimeSec();
assert(firstAfterOldest === 1_700_000_000 + 50 * 60, 'dropped 50 oldest');

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
assert(live.firstTimeSec() < firstAfterOldest, 'older history retained on left');

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
assert(tip.barCount() === 10, 'replace defensive prune');
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
assert(proj2.barCount() === 100, 'soft applyProjection under HARD_CAP stays intact');
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

// replaceMonolith commit-paired: world replace — Mutation Set excluded; prune allowed
const commitProj = new ColumnarStore();
commitProj.replaceMonolith({
  times: bigTimes,
  candles: bigCandles,
  plots: {},
  annotations: [],
}, { commitPaired: true });
assert(commitProj.barCount() === 100, 'TB1 commitPaired: may prune without Mutation Set');
assert(commitProj.windowMode === 'live', 'TB1 commitPaired FROM_OLDEST stays live');
assert(commitProj.invariantOk(), 'TB1 commitPaired invariant');

console.log('columnar-store_budget_test: OK');
