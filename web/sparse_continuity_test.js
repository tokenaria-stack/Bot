/**
 * MICRO-1.1 — sparse micro TFs must not report live chronology gaps.
 * Run: node web/sparse_continuity_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const { ColumnarStore } = require('./columnar-store.js');

global.chartTime = (t) => {
  const n = Number(t);
  return Number.isFinite(n) ? n : null;
};

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function loadRequiresDense() {
  const config = fs.readFileSync(path.join(__dirname, 'config.js'), 'utf8');
  const m = config.match(/function requiresDenseTimeContinuity\(tf\) \{[\s\S]*?\n\}/);
  assert.ok(m, 'requiresDenseTimeContinuity missing from config.js');
  return new Function(`${m[0]}; return requiresDenseTimeContinuity;`)();
}

function tick(time) {
  return { time, open: 1, high: 2, low: 1, close: 1.5, volume: 1 };
}

function seed(store, time) {
  store.replaceMonolith({
    times: [time],
    candles: { open: [1], high: [1], low: [1], close: [1], volume: [1] },
    plots: {},
  }, { commitPaired: true });
}

const requiresDenseTimeContinuity = loadRequiresDense();

test('F. catalog class: native/derived dense; seconds/ticks sparse (no 1s special-case)', () => {
  assert.strictEqual(requiresDenseTimeContinuity('1m'), true);
  assert.strictEqual(requiresDenseTimeContinuity('2m'), true);
  assert.strictEqual(requiresDenseTimeContinuity('10m'), true);
  assert.strictEqual(requiresDenseTimeContinuity('45m'), true);
  assert.strictEqual(requiresDenseTimeContinuity('3h'), true);
  assert.strictEqual(requiresDenseTimeContinuity('1s'), false);
  assert.strictEqual(requiresDenseTimeContinuity('5s'), false);
  assert.strictEqual(requiresDenseTimeContinuity('15s'), false);
  assert.strictEqual(requiresDenseTimeContinuity('1tick'), false);
  assert.strictEqual(requiresDenseTimeContinuity('100tick'), false);
  const src = fs.readFileSync(path.join(__dirname, 'config.js'), 'utf8');
  assert.ok(!/if\s*\(\s*(tf|id)\s*===\s*['"]1s['"]/.test(src), 'do not special-case 1s');
});

test('A. 1s 100→101 is a normal sparse append', () => {
  const store = new ColumnarStore();
  store.setTfInterval(1);
  store.setDenseContinuity(false);
  seed(store, 100);
  const r = store.appendTick(tick(101));
  assert.ok(r && !r.gapDetected, 'adjacent second is not a gap');
  assert.strictEqual(store.lastTimeSec(), 101);
});

test('B. 1s 100→103 is a legal sparse hole (no-trade second)', () => {
  const store = new ColumnarStore();
  store.setTfInterval(1);
  store.setDenseContinuity(false);
  seed(store, 100);
  const r = store.appendTick(tick(103));
  assert.ok(r && !r.gapDetected, 'skipped second must append, not heal');
  assert.strictEqual(r.candle.time, 103);
  assert.strictEqual(store.lastTimeSec(), 103);
  assert.strictEqual(store.barCount(), 2);
});

test('C. multiple updates on the same second overwrite the forming bar', () => {
  const store = new ColumnarStore();
  store.setTfInterval(1);
  store.setDenseContinuity(false);
  seed(store, 100);
  const first = store.appendTick({ time: 100, open: 1, high: 2, low: 1, close: 1.2, volume: 2 });
  const second = store.appendTick({ time: 100, open: 1, high: 3, low: 0.5, close: 2.0, volume: 3 });
  assert.ok(first && !first.gapDetected && !first.isNewBar);
  assert.ok(second && !second.gapDetected && !second.isNewBar);
  assert.strictEqual(store.barCount(), 1);
  assert.strictEqual(store.lastTimeSec(), 100);
});

test('D. native 1m missing a minute still reports a gap (default dense)', () => {
  const store = new ColumnarStore();
  store.setTfInterval(60);
  seed(store, 1_700_000_000);
  const r = store.appendTick(tick(1_700_000_000 + 120));
  assert.ok(r && r.gapDetected, 'missing 1m bucket must still heal');
  assert.strictEqual(store.lastTimeSec(), 1_700_000_000);
});

test('E. derived 2m missing an expected child keeps dense policy', () => {
  const store = new ColumnarStore();
  store.setTfInterval(120);
  store.setDenseContinuity(true);
  seed(store, 1_700_000_000);
  const r = store.appendTick(tick(1_700_000_000 + 240));
  assert.ok(r && r.gapDetected, 'missing 2m bucket must still heal');
});

test('F2. future seconds/ticks class is sparse without new special cases', () => {
  for (const [tf, step] of [['5s', 5], ['1tick', 1]]) {
    assert.strictEqual(requiresDenseTimeContinuity(tf), false, tf);
    const store = new ColumnarStore();
    store.setTfInterval(step);
    store.setDenseContinuity(false);
    seed(store, 100);
    const r = store.appendTick(tick(100 + step * 3));
    assert.ok(r && !r.gapDetected, `${tf} sparse append`);
  }
});

test('boot wires dense flag; never special-cases 1s for gap heal', () => {
  const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
  assert.ok(boot.includes('setDenseContinuity'));
  assert.ok(boot.includes('requiresDenseTimeContinuity'));
  assert.ok(!boot.includes("timeframe === '1s'"));
  assert.ok(!boot.includes('currentTf === \'1s\''));
});

console.log('sparse_continuity_test: ALL PASS');
