/**
 * Fix F — same-bar delta paint suppressed while TimeCamera VIEW is HISTORY.
 * Run: node web/live_delta_view_gate_test.js
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

function extractFn(src, name) {
  const re = new RegExp(`function ${name}\\s*\\([^)]*\\)\\s*\\{`);
  const m = src.match(re);
  assert.ok(m, `missing function ${name}`);
  const start = m.index + m[0].length - 1;
  let depth = 0;
  for (let i = start; i < src.length; i++) {
    if (src[i] === '{') depth += 1;
    else if (src[i] === '}') {
      depth -= 1;
      if (depth === 0) return src.slice(m.index, i + 1);
    }
  }
  assert.fail(`unclosed ${name}`);
}

const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
const pushBody = extractFn(boot, 'pushLiveTickDelta');
const shouldMarkDirtyLiveDelta = eval( // eslint-disable-line no-eval
  `(${extractFn(boot, 'shouldMarkDirtyLiveDelta')})`,
);

test('A. LIVE + same bar → paint', () => {
  assert.strictEqual(shouldMarkDirtyLiveDelta('LIVE', false), true);
});

test('A2. LIVE + new bar → paint', () => {
  assert.strictEqual(shouldMarkDirtyLiveDelta('LIVE', true), true);
});

test('B. HISTORY + same bar → no paint', () => {
  assert.strictEqual(shouldMarkDirtyLiveDelta('HISTORY', false), false);
});

test('C. HISTORY + new bar → paint (native / dense)', () => {
  assert.strictEqual(shouldMarkDirtyLiveDelta('HISTORY', true), true);
});

test('MICRO-2C. sparse HISTORY skips new-bar delta too', () => {
  assert.strictEqual(shouldMarkDirtyLiveDelta('HISTORY', true, true), false);
});

test('D. null / unknown intent → fail open (paint)', () => {
  assert.strictEqual(shouldMarkDirtyLiveDelta(null, false), true);
  assert.strictEqual(shouldMarkDirtyLiveDelta(undefined, false), true);
  assert.strictEqual(shouldMarkDirtyLiveDelta('history', false), true);
});

test('E. HISTORY same-bar: store still updates; paint gate is off', () => {
  const store = new ColumnarStore();
  store.replaceMonolith({
    times: [100, 160],
    candles: {
      open: [1, 2],
      high: [1, 2],
      low: [1, 2],
      close: [1, 2],
      volume: [1, 1],
    },
    plots: {},
  }, { commitPaired: true });
  assert.strictEqual(store.windowMode, 'live');
  const before = store.barCount();
  const result = store.appendTick({
    time: 160,
    open: 2,
    high: 3,
    low: 2,
    close: 2.5,
    volume: 9,
  });
  assert.ok(result?.candle);
  assert.strictEqual(result.isNewBar, false);
  assert.strictEqual(store.barCount(), before);
  assert.strictEqual(store._candles.close[store.barCount() - 1], 2.5);
  assert.strictEqual(shouldMarkDirtyLiveDelta('HISTORY', result.isNewBar), false);
});

test('boot: ingest still precedes the VIEW paint gate', () => {
  const appendAt = pushBody.indexOf('appendTick');
  const gateAt = pushBody.indexOf('shouldMarkDirtyLiveDelta');
  const dirtyAt = pushBody.indexOf('markDirty');
  assert.ok(appendAt >= 0 && gateAt > appendAt && dirtyAt > gateAt,
    'appendTick → shouldMarkDirtyLiveDelta → markDirty');
  assert.ok(/windowMode === 'history'/.test(pushBody),
    'existing windowMode data gate must remain for dense TFs');
  assert.ok(/isSparseLiveChart/.test(pushBody),
    'sparse 1s must still ingest when windowMode is history');
  assert.ok(/liveCameraViewIntent/.test(pushBody),
    'paint gate must read TimeCamera VIEW, not windowMode');
});

test('boot: TimeCamera shadow is the VIEW source', () => {
  const reader = extractFn(boot, 'liveCameraViewIntent');
  assert.ok(/_getShadowView/.test(reader), 'must use existing shadow view-intent');
  assert.ok(!/windowMode/.test(reader), 'VIEW reader must not use windowMode');
  assert.ok(!/classifyViewIntent/.test(reader),
    'must not recompute slack; shadow already classified');
});

test('boot: sparse live-entry hydrate ignores lagged HISTORY; explicit history remains', () => {
  const load = extractFn(boot, 'loadDashboard');
  assert.ok(/sparseExplicitHistoryHydrate/.test(load),
    '1s HISTORY acquisition must use explicit-history predicate');
  assert.ok(!/historyIsland = !sparseTf/.test(load),
    'must not blanket-FreshLive every 1s hydrate');
  const pred = eval(`(${extractFn(boot, 'sparseExplicitHistoryHydrate')})`); // eslint-disable-line no-eval
  const now = 1_800_000_000;
  assert.strictEqual(pred({ intent: 'LIVE', centerTimeMs: now * 1000, visibleBars: 300 }, now), false);
  assert.strictEqual(pred({
    intent: 'HISTORY',
    centerTimeMs: (now - 10) * 1000,
    visibleBars: 300,
  }, now), false, 'lagged live edge is not explicit history');
  assert.strictEqual(pred({
    intent: 'HISTORY',
    centerTimeMs: (now - 900) * 1000,
    visibleBars: 300,
  }, now), true, 'focus well behind now is explicit 1s history');
});

console.log('live_delta_view_gate_test: ALL PASS');
