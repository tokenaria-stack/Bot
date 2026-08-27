/**
 * P2 — MAX_STORE_BARS moving window + VIEW protection.
 * Run: node web/store_working_set_cap_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const { ColumnarStore } = require('./columnar-store.js');
const TimeCamera = require('./ui/time-camera.js');

test('production contract: 9k store cap, 5k visible cap', () => {
  const cfg = fs.readFileSync(path.join(__dirname, 'config.js'), 'utf8');
  assert.ok(/const MAX_STORE_BARS = 9000/.test(cfg), 'MAX_STORE_BARS = 9000');
  assert.ok(/const MAX_VISIBLE_BARS = 5000/.test(cfg), 'MAX_VISIBLE_BARS = 5000');
  const storeSrc = fs.readFileSync(path.join(__dirname, 'columnar-store.js'), 'utf8');
  assert.ok(/: 9000;/.test(storeSrc), 'ColumnarStore unconfigured fallback matches 9k');
});

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function monolith(n, startSec, step = 60) {
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

function setCap(n) {
  Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => n, configurable: true });
  Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', {
    get: () => ColumnarStore.BUDGET_TARGET,
    configurable: true,
  });
}

test('RIGHT append over cap prunes LEFT; tip advances; store <= cap', () => {
  setCap(100);
  const store = new ColumnarStore();
  store.setTfInterval(60);
  store.replaceMonolith(monolith(95, 1_700_000_000), { commitPaired: true });
  const viewFrom = store.firstTimeSec() + 40 * 60;
  const viewTo = store.firstTimeSec() + 90 * 60;
  const tipBefore = store.lastTimeSec();
  const firstBefore = store.firstTimeSec();

  store.appendMonolith(monolith(30, tipBefore + 60), {
    viewFromSec: viewFrom,
    viewToSec: viewTo,
  });

  assert.ok(store.barCount() <= 100, `store ${store.barCount()} <= 100`);
  assert.ok(store.lastTimeSec() > tipBefore, 'RIGHT tip advanced');
  assert.ok(store.firstTimeSec() > firstBefore, 'LEFT pruned (oldest discarded)');
  // Viewport time span still in store
  assert.ok(store.firstTimeSec() <= viewFrom, 'view left still covered or left of series start');
  assert.ok(store.lastTimeSec() >= viewTo || store.lastTimeSec() > tipBefore,
    'view right / new data retained');
});

test('LEFT prepend over cap prunes RIGHT; head retreats left; store <= cap', () => {
  setCap(100);
  const store = new ColumnarStore();
  store.setTfInterval(60);
  store.replaceMonolith(monolith(95, 1_700_000_000), { commitPaired: true });
  const firstBefore = store.firstTimeSec();
  const tipBefore = store.lastTimeSec();
  const viewFrom = firstBefore + 10 * 60;
  const viewTo = firstBefore + 50 * 60;

  store.prependMonolith(monolith(30, firstBefore - 30 * 60), {
    viewFromSec: viewFrom,
    viewToSec: viewTo,
  });

  assert.ok(store.barCount() <= 100, `store ${store.barCount()} <= 100`);
  assert.ok(store.firstTimeSec() < firstBefore, 'LEFT head grew older');
  assert.ok(store.lastTimeSec() < tipBefore, 'RIGHT tip pruned');
  assert.ok(store.firstTimeSec() <= viewFrom);
  assert.ok(store.lastTimeSec() >= viewTo);
});

test('ISLAND-SLIDE: RIGHT append over cap prunes LEFT even if VIEW sat on oldest', () => {
  setCap(80);
  const store = new ColumnarStore();
  store.replaceMonolith(monolith(70, 1_700_000_000), { commitPaired: true });
  const firstBefore = store.firstTimeSec();
  const viewFrom = firstBefore + 20 * 60;
  const viewTo = firstBefore + 60 * 60;
  const tip = store.lastTimeSec();

  const r = store.appendMonolith(monolith(40, tip + 60), {
    viewFromSec: viewFrom,
    viewToSec: viewTo,
  });

  assert.ok(store.barCount() <= 80, `store ${store.barCount()} <= 80`);
  assert.ok(store.lastTimeSec() > tip, 'acquired RIGHT tip kept');
  assert.ok(store.firstTimeSec() > firstBefore, 'LEFT pruned');
  assert.strictEqual(r.pruneDirection, ColumnarStore.PRUNE_FROM_OLDEST);
});

test('continued RIGHT appends stay at cap (moving window)', () => {
  setCap(100);
  const store = new ColumnarStore();
  store.replaceMonolith(monolith(50, 1_700_000_000), { commitPaired: true });
  let tip = store.lastTimeSec();
  for (let i = 0; i < 8; i++) {
    store.appendMonolith(monolith(20, tip + 60), {
      viewFromSec: tip - 10 * 60,
      viewToSec: tip + 5 * 60,
    });
    tip = store.lastTimeSec();
    assert.ok(store.barCount() <= 100, `iter ${i}: ${store.barCount()}`);
  }
  assert.strictEqual(store.barCount() <= 100, true);
  assert.ok(tip > 1_700_000_000 + 50 * 60, 'window moved right through history');
});

test('proposeFromPane clamps visible width to MAX_VISIBLE_LOGICAL_BARS', () => {
  TimeCamera._resetForTests();
  let seen = null;
  TimeCamera.bind({ applyCommitted: (s) => { seen = s; } });
  const max = TimeCamera.MAX_VISIBLE_LOGICAL_BARS;
  assert.ok(Number.isFinite(max) && max === 5000, `MAX_VISIBLE_LOGICAL_BARS === 5000, got ${max}`);
  TimeCamera.proposeFromPane('price-chart', { from: 0, to: max + 5000 }, 6);
  assert.ok(seen);
  const w = seen.visibleRange.to - seen.visibleRange.from;
  assert.ok(w <= max + 1e-9, `width ${w} <= ${max}`);
  assert.ok(Math.abs(seen.visibleRange.to - (max + 5000)) < 1e-9
    || Math.abs(seen.visibleRange.to - seen.visibleRange.from - max) < 1e-9);
  // Right edge preserved when clamping
  assert.ok(Math.abs(seen.visibleRange.to - (0 + max + 5000)) < 1e-6
    || Math.abs(seen.visibleRange.to - (max + 5000)) < 1e-6);
});

console.log('store_working_set_cap_test: ALL PASS');
