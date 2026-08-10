/**
 * ViewportAnchor prepend preserve — focused Node tests (no MCP/LWC).
 * Run: node web/viewport_preserve_prepend_test.js
 */
'use strict';

const assert = require('assert');
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
  assert.strictEqual(anchor.logicalOffset, 0.25);
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

test('A: from=5.25 prepend +3000 preserves market-time + fraction', () => {
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
  // After prepend, old index 5 is at index 3005; its open is still 1_700_000_005
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
  // Same market-time under left edge: newTimes[floor(from)] ≈ anchor bar
  const leftIdx = Math.floor(seen.visibleRange.from);
  assert.strictEqual(newTimes[leftIdx], 1_700_000_000 + 5);
});

test('B: from=-50 prepend +3000 keeps void offset', () => {
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
  // old index 0 → new index 3000; newFrom = 3000 + (-50) = 2950
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

  // User wheel/pointer releases ownership before range event.
  TimeCamera.releasePreserveTransaction();
  assert.strictEqual(TimeCamera.hasOpenPreserveTransaction(), false);

  const userOk = TimeCamera.proposeFromPane('price', { from: 200, to: 280 }, 6);
  assert.strictEqual(userOk, true);
  assert.deepStrictEqual(TimeCamera.getCanonical().visibleRange, { from: 200, to: 280 });
  assert.strictEqual(commits.length, 2);
});

console.log('viewport_preserve_prepend_test: ALL PASS');
