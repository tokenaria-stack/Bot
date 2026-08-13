/**
 * Viewport market-time preserve — focused Node tests (no MCP/LWC).
 * Aligns with current LEFT contract: capture → resolve → logical range
 * (runtime applies via forceVisibleLogicalRange). RIGHT still uses proposePreserveViewport.
 * Run: node web/viewport_preserve_prepend_test.js
 */
'use strict';

const assert = require('assert');
const { ChartDataStore } = require('./store.js');
global.ChartDataStore = ChartDataStore;
const TimeCamera = require('./ui/time-camera.js');
const { ChartCompositor } = require('./chart-compositor.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function timesSec(n, startSec = 1_700_000_000) {
  return Array.from({ length: n }, (_, i) => startSec + i);
}

test('captureViewportAnchor: fractional from >= 0', () => {
  const times = timesSec(100);
  const anchor = ChartCompositor.captureViewportAnchor(times, { from: 5.25, to: 85.25 });
  assert.ok(anchor);
  assert.strictEqual(anchor.anchorTimeMs, (1_700_000_000 + 5) * 1000);
  assert.strictEqual(anchor.rightTimeMs, (1_700_000_000 + 85) * 1000);
  assert.strictEqual(anchor.logicalOffset, 0.25);
  assert.strictEqual(anchor.rightLogicalOffset, 0.25);
  assert.strictEqual(anchor.visibleBars, 80);
});

test('captureViewportAnchor: from < 0 preserves void offset', () => {
  const times = timesSec(100);
  const anchor = ChartCompositor.captureViewportAnchor(times, { from: -50, to: 100 });
  assert.ok(anchor);
  assert.strictEqual(anchor.anchorTimeMs, 1_700_000_000 * 1000);
  assert.strictEqual(anchor.logicalOffset, -50);
  assert.strictEqual(anchor.visibleBars, 150);
});

test('findIndexByTimeMs: nearest ascending unix-sec', () => {
  const times = timesSec(10, 1_700_000_000);
  assert.strictEqual(ChartCompositor.findIndexByTimeMs(times, 1_700_000_000 * 1000), 0);
  assert.strictEqual(ChartCompositor.findIndexByTimeMs(times, (1_700_000_000 + 5) * 1000), 5);
  assert.strictEqual(ChartCompositor.findIndexByTimeMs(times, (1_700_000_000 + 4.6) * 1000), 5);
  assert.strictEqual(ChartCompositor.findIndexByTimeMs(times, (1_700_000_000 + 4.4) * 1000), 4);
});

test('A: from=5.25 prepend +3000 preserves market-time + fraction (RIGHT propose path)', () => {
  TimeCamera._resetForTests();
  let seen = null;
  TimeCamera.bind({ applyCommitted: (s) => { seen = s; } });

  const oldN = 3000;
  const added = 3000;
  const oldTimes = timesSec(oldN);
  const range = { from: 5.25, to: 85.25 };
  const anchor = ChartCompositor.captureViewportAnchor(oldTimes, range);
  assert.ok(anchor);

  const newTimes = timesSec(oldN + added, 1_700_000_000 - added);
  TimeCamera.bindDataResolve({
    nearestLogicalForTime: (ms) => ChartCompositor.findIndexByTimeMs(newTimes, ms),
  });

  const ok = TimeCamera.proposePreserveViewport({
    anchorTimeMs: anchor.anchorTimeMs,
    logicalOffset: anchor.logicalOffset,
    visibleBars: anchor.visibleBars,
    tipLogical: newTimes.length - 1,
    timesSec: newTimes,
  });
  assert.strictEqual(ok, true);
  assert.ok(seen);
  assert.ok(Math.abs(seen.visibleRange.from - (added + 5.25)) < 1e-9);
  assert.ok(Math.abs(seen.visibleRange.to - (added + 85.25)) < 1e-9);
  const leftIdx = Math.floor(seen.visibleRange.from);
  assert.strictEqual(newTimes[leftIdx], 1_700_000_000 + 5);
});

test('B: from=-50 prepend +3000 keeps void offset (RIGHT propose path)', () => {
  TimeCamera._resetForTests();
  let seen = null;
  TimeCamera.bind({ applyCommitted: (s) => { seen = s; } });

  const oldN = 1000;
  const added = 3000;
  const oldTimes = timesSec(oldN);
  const anchor = ChartCompositor.captureViewportAnchor(oldTimes, { from: -50, to: 100 });
  assert.strictEqual(anchor.logicalOffset, -50);

  const newTimes = timesSec(oldN + added, 1_700_000_000 - added);
  TimeCamera.bindDataResolve({
    nearestLogicalForTime: (ms) => ChartCompositor.findIndexByTimeMs(newTimes, ms),
  });

  const ok = TimeCamera.proposePreserveViewport({
    anchorTimeMs: anchor.anchorTimeMs,
    logicalOffset: anchor.logicalOffset,
    visibleBars: anchor.visibleBars,
    tipLogical: newTimes.length - 1,
    timesSec: newTimes,
  });
  assert.strictEqual(ok, true);
  assert.strictEqual(seen.visibleRange.from, added - 50);
  assert.strictEqual(seen.visibleRange.to, added - 50 + 150);
  assert.ok(seen.visibleRange.from < added, 'void remains left of former index-0 bar');
});

test('E: wide visibleBars (>400) must not be clamped on preserve', () => {
  TimeCamera._resetForTests();
  let seen = null;
  TimeCamera.bind({ applyCommitted: (s) => { seen = s; } });
  TimeCamera.bindDataResolve({
    nearestLogicalForTime: (ms) => ChartCompositor.findIndexByTimeMs(
      timesSec(6000, 1_700_000_000 - 3000),
      ms,
    ),
  });

  const ok = TimeCamera.proposePreserveViewport({
    anchorTimeMs: (1_700_000_000 + 8) * 1000,
    logicalOffset: 0,
    visibleBars: 2992,
    tipLogical: 5999,
    timesSec: timesSec(6000, 1_700_000_000 - 3000),
  });
  assert.strictEqual(ok, true);
  assert.ok(seen);
  const width = seen.visibleRange.to - seen.visibleRange.from;
  assert.ok(Math.abs(width - 2992) < 1e-9, `width must stay 2992, got ${width}`);
});

test('C: stale proposeFromPane cannot overwrite system preserve', () => {
  TimeCamera._resetForTests();
  const commits = [];
  TimeCamera.bind({ applyCommitted: (s) => { commits.push({ ...s.visibleRange }); } });
  TimeCamera.bindDataResolve({
    nearestLogicalForTime: (ms) => (ms === 1_700_000_005_000 ? 3005 : 0),
  });

  const ok = TimeCamera.proposePreserveViewport({
    anchorTimeMs: 1_700_000_005_000,
    logicalOffset: 0.25,
    visibleBars: 80,
    tipLogical: 5999,
    timesSec: timesSec(6000, 1_700_000_000 - 3000),
  });
  assert.strictEqual(ok, true);
  assert.strictEqual(TimeCamera.hasOpenPreserveTransaction(), true);
  const preserved = { ...TimeCamera.getCanonical().visibleRange };

  const stale = TimeCamera.proposeFromPane('price', { from: 5.25, to: 85.25 }, 6);
  assert.strictEqual(stale, false, 'stale echo must be ignored');
  assert.deepStrictEqual(TimeCamera.getCanonical().visibleRange, preserved);
  assert.strictEqual(TimeCamera.hasOpenPreserveTransaction(), false);
  assert.strictEqual(commits.length, 1);
});

test('D: user gesture releases txn; subsequent proposeFromPane accepted', () => {
  TimeCamera._resetForTests();
  const commits = [];
  TimeCamera.bind({ applyCommitted: (s) => { commits.push({ ...s.visibleRange }); } });
  TimeCamera.bindDataResolve({
    nearestLogicalForTime: () => 100,
  });

  TimeCamera.proposePreserveViewport({
    anchorTimeMs: 1_700_000_000_000,
    logicalOffset: 0,
    visibleBars: 80,
    tipLogical: 500,
    timesSec: timesSec(501),
  });
  assert.strictEqual(TimeCamera.hasOpenPreserveTransaction(), true);

  TimeCamera.releasePreserveTransaction();
  assert.strictEqual(TimeCamera.hasOpenPreserveTransaction(), false);

  const userOk = TimeCamera.proposeFromPane('price', { from: 200, to: 280 }, 6);
  assert.strictEqual(userOk, true);
  assert.deepStrictEqual(TimeCamera.getCanonical().visibleRange, { from: 200, to: 280 });
  assert.strictEqual(commits.length, 2);
});

test('LEFT at 25k cap: market-time resolve preserves viewport (not net-length shift)', () => {
  // 25000 + prepend 2999 → prune right ≈2999 → still 25000. Net length delta = 0.
  const { ColumnarStore } = require('./columnar-store.js');
  Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 25000, configurable: true });
  Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', { get: () => 25000, configurable: true });

  function monolith(n, startSec, step = 3600) {
    const times = Array.from({ length: n }, (_, i) => startSec + i * step);
    return {
      times,
      candles: {
        open: times.map(() => 1),
        high: times.map(() => 1),
        low: times.map(() => 1),
        close: times.map(() => 1),
        volume: times.map(() => 1),
      },
      plots: {},
    };
  }

  const store = new ColumnarStore();
  store.setTfInterval(3600);
  const start = 1_700_000_000;
  store.replaceMonolith(monolith(25000, start), { commitPaired: true });

  const range = { from: 18300.25, to: 20300.25 };
  const preTimes = store.snapshot().times;
  const anchor = ChartCompositor.captureViewportAnchor(preTimes, range);
  assert.ok(anchor);
  const leftSecBefore = preTimes[Math.floor(range.from)];
  const rightSecBefore = preTimes[Math.floor(range.to)];
  const lengthBefore = store.barCount();

  const prependStart = store.firstTimeSec() - 2999 * 3600;
  const merge = store.prependMonolith(monolith(2999, prependStart), {
    viewFromSec: leftSecBefore,
    viewToSec: rightSecBefore,
  });

  assert.strictEqual(merge.prependedCount, 2999);
  assert.strictEqual(store.barCount(), 25000);
  assert.strictEqual(store.barCount() - lengthBefore, 0, 'net length unchanged at cap');
  assert.ok(merge.prunedRightCount >= 2990, `prunedRight≈2999 got ${merge.prunedRightCount}`);

  const newTimes = store.snapshot().times;
  TimeCamera._resetForTests();
  TimeCamera.bindDataResolve({
    nearestLogicalForTime: (ms) => ChartCompositor.findIndexByTimeMs(newTimes, ms),
  });

  const resolved = TimeCamera.resolveMarketTimePreserve({
    leftTimeMs: anchor.anchorTimeMs,
    rightTimeMs: anchor.rightTimeMs,
    logicalOffset: anchor.logicalOffset,
    rightLogicalOffset: anchor.rightLogicalOffset,
    tipLogical: newTimes.length - 1,
    timesSec: newTimes,
  });
  assert.ok(resolved);
  assert.strictEqual(resolved.caseId, 'B1');

  const wrongNet = {
    from: range.from + (store.barCount() - lengthBefore),
    to: range.to + (store.barCount() - lengthBefore),
  };
  assert.notDeepStrictEqual(
    { from: resolved.from, to: resolved.to },
    wrongNet,
    'market-time resolve must differ from net-length (0) shift at cap',
  );

  const leftIdx = Math.floor(resolved.from);
  const rightIdx = Math.floor(resolved.to);
  assert.strictEqual(newTimes[leftIdx], leftSecBefore, 'left market-time preserved');
  assert.strictEqual(newTimes[rightIdx], rightSecBefore, 'right market-time preserved');
  assert.notStrictEqual(newTimes[Math.floor(wrongNet.from)], leftSecBefore,
    'net-length (0) shift leaves camera on wrong bars');
});

test('resolve B1: both edges survive → exact time restore', () => {
  TimeCamera._resetForTests();
  const finalTimes = timesSec(1000, 1_700_100_000);
  const leftSec = 1_700_100_000 + 100;
  const rightSec = 1_700_100_000 + 500;
  TimeCamera.bindDataResolve({
    nearestLogicalForTime: (ms) => ChartCompositor.findIndexByTimeMs(finalTimes, ms),
  });

  const resolved = TimeCamera.resolveMarketTimePreserve({
    leftTimeMs: leftSec * 1000,
    rightTimeMs: rightSec * 1000,
    logicalOffset: 0.25,
    rightLogicalOffset: 0.25,
    tipLogical: finalTimes.length - 1,
    timesSec: finalTimes,
  });
  assert.ok(resolved);
  assert.strictEqual(resolved.caseId, 'B1');
  assert.ok(Math.abs(resolved.from - (100 + 0.25)) < 1e-9);
  assert.ok(Math.abs(resolved.to - (500 + 0.25)) < 1e-9);
});

test('resolve B2: right evicted, left survives → left-anchored width (not tip snap)', () => {
  TimeCamera._resetForTests();
  const finalTimes = timesSec(800, 1_700_000_000);
  const leftSec = 1_700_000_000 + 100;
  const rightSec = 1_700_000_000 + 900;
  TimeCamera.bindDataResolve({
    nearestLogicalForTime: (ms) => ChartCompositor.findIndexByTimeMs(finalTimes, ms),
  });

  const resolved = TimeCamera.resolveMarketTimePreserve({
    leftTimeMs: leftSec * 1000,
    rightTimeMs: rightSec * 1000,
    logicalOffset: 0,
    tipLogical: finalTimes.length - 1,
    timesSec: finalTimes,
  });
  assert.ok(resolved);
  assert.strictEqual(resolved.caseId, 'B2');
  assert.ok(Math.abs(resolved.from - 100) < 1e-9);
  assert.ok(Math.abs(resolved.to - 799) < 1e-9);
  assert.ok(resolved.from > 50, 'must keep left history anchor, not tip-snap');
});

test('resolve B3: left also gone → tip-anchored width fallback', () => {
  TimeCamera._resetForTests();
  const finalTimes = timesSec(500, 1_700_000_000);
  const leftSec = 1_700_000_000 + 800;
  const rightSec = 1_700_000_000 + 1200;
  TimeCamera.bindDataResolve({
    nearestLogicalForTime: (ms) => ChartCompositor.findIndexByTimeMs(finalTimes, ms),
  });

  const resolved = TimeCamera.resolveMarketTimePreserve({
    leftTimeMs: leftSec * 1000,
    rightTimeMs: rightSec * 1000,
    tipLogical: finalTimes.length - 1,
    timesSec: finalTimes,
  });
  assert.ok(resolved);
  assert.strictEqual(resolved.caseId, 'B3');
  assert.ok(Math.abs(resolved.to - 499) < 1e-9);
  assert.ok(Math.abs(resolved.from - 99) < 1e-9);
});

test('resolveMarketTimePreserve returns logical range without requiring commit', () => {
  TimeCamera._resetForTests();
  const finalTimes = timesSec(1000, 1_700_100_000);
  TimeCamera.bindDataResolve({
    nearestLogicalForTime: (ms) => ChartCompositor.findIndexByTimeMs(finalTimes, ms),
  });
  const resolved = TimeCamera.resolveMarketTimePreserve({
    leftTimeMs: (1_700_100_000 + 100) * 1000,
    rightTimeMs: (1_700_100_000 + 500) * 1000,
    logicalOffset: 0.25,
    rightLogicalOffset: 0.25,
    tipLogical: finalTimes.length - 1,
    timesSec: finalTimes,
  });
  assert.ok(resolved);
  assert.strictEqual(resolved.caseId, 'B1');
  assert.ok(Math.abs(resolved.from - 100.25) < 1e-9);
  assert.ok(Math.abs(resolved.to - 500.25) < 1e-9);
});

console.log('viewport_preserve_prepend_test: ALL PASS');
