/**
 * Debt #69A — ColumnarStore memory budget (Node).
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
  store.replaceMonolith({
    times,
    candles: { open, high, low, close, volume },
    plots,
    annotations,
    timeframe: '1m',
  });
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

// Prepend past hard cap → FROM_NEWEST → history mode
const live = new ColumnarStore();
fillStore(live, 100);
const older = {
  times: [],
  candles: { open: [], high: [], low: [], close: [], volume: [] },
  plots: { line_rsx: [], line_woz: [] },
  annotations: [],
};
for (let i = 0; i < 50; i++) {
  const t = 1_700_000_000 - (50 - i) * 60;
  older.times.push(t);
  older.candles.open.push(1);
  older.candles.high.push(1);
  older.candles.low.push(1);
  older.candles.close.push(1);
  older.candles.volume.push(1);
  older.plots.line_rsx.push(1);
  older.plots.line_woz.push(1);
}
const beforePrependLast = live.lastTimeSec();
live.prependMonolith(older);
assert(live.barCount() === 100, `prepend prune to target, got ${live.barCount()}`);
assert(live.windowMode === 'history', 'FROM_NEWEST sets history mode');
assert(live.invariantOk(), 'invariant after FROM_NEWEST');
assert(live.lastTimeSec() < beforePrependLast, 'newest tip pruned away');
assert(live.firstTimeSec() < firstAfterOldest, 'older history retained on left');

// appendTick growth prune (live mode)
const tip = new ColumnarStore();
Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 10 });
Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 12 });
global.chartTime = (t) => Number(t);
tip.setTfInterval(60);
fillStore(tip, 12);
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
assert(tip.barCount() === 10, `appendTick enforces budget, got ${tip.barCount()}`);
assert(tip.windowMode === 'live', 'append prune keeps live');
assert(tip.invariantOk(), 'invariant after appendTick budget');

console.log('columnar-store_budget_test: OK');
