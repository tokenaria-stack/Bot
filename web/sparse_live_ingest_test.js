/**
 * SPARSE-LIVE-INGEST-1 — 5s–45s WS ingest uses windowMode, not historyHasNewer.
 * Run: node web/sparse_live_ingest_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const { ColumnarStore } = require('./columnar-store.js');

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

function isLiveSecondChart(tf) {
  return String(tf || '').trim() === '1s';
}

function isSparseSecondChart(tf) {
  switch (String(tf || '').trim()) {
    case '5s':
    case '10s':
    case '15s':
    case '30s':
    case '45s':
      return true;
    default:
      return false;
  }
}

const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
const shouldVetoSecondsLiveIngest = eval(`(${extractFn(boot, 'shouldVetoSecondsLiveIngest')})`); // eslint-disable-line no-eval

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('A. LIVE 5s + hasNewer=true → accept', () => {
  assert.strictEqual(shouldVetoSecondsLiveIngest('5s', 'live', true), false);
});

test('B. LIVE 45s + hasNewer=true → accept', () => {
  assert.strictEqual(shouldVetoSecondsLiveIngest('45s', 'live', true), false);
});

test('C. HISTORY 5s → reject now tick', () => {
  assert.strictEqual(shouldVetoSecondsLiveIngest('5s', 'history', true), true);
  assert.strictEqual(shouldVetoSecondsLiveIngest('5s', 'history', false), true);
});

test('D. 1s detached hasNewer=true → still reject', () => {
  assert.strictEqual(shouldVetoSecondsLiveIngest('1s', 'live', true), true);
  assert.strictEqual(shouldVetoSecondsLiveIngest('1s', 'history', true), true);
  assert.strictEqual(shouldVetoSecondsLiveIngest('1s', 'history', false), false);
});

test('LIVE 15s + hasNewer=true → accept', () => {
  assert.strictEqual(shouldVetoSecondsLiveIngest('15s', 'live', true), false);
});

test('E. 5s prepend drops newest → windowMode history → now tick vetoed', () => {
  Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => 100, configurable: true });
  Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', {
    get: () => ColumnarStore.BUDGET_TARGET,
    configurable: true,
  });
  const step = 5;
  const start = 1_700_000_000;
  const monolith = (n, t0) => {
    const times = Array.from({ length: n }, (_, i) => t0 + i * step);
    return {
      times,
      candles: {
        open: times.map(() => 1),
        high: times.map(() => 2),
        low: times.map(() => 1),
        close: times.map(() => 1.5),
        volume: times.map(() => 1),
      },
      plots: {},
      annotations: [],
    };
  };
  const store = new ColumnarStore();
  store.setTfInterval(step);
  store.replaceMonolith(monolith(100, start), { commitPaired: true, windowMode: 'live' });
  const tipBefore = store.lastTimeSec();
  const older = monolith(40, start - 40 * step);
  store.prependMonolith(older);
  assert.ok(store.lastTimeSec() < tipBefore, 'newest/live tip pruned');
  assert.strictEqual(store.windowMode, 'history');
  assert.strictEqual(shouldVetoSecondsLiveIngest('5s', store.windowMode, true), true);
});

const push = extractFn(boot, 'pushLiveTickDelta');
assert.ok(push.indexOf('shouldVetoSecondsLiveIngest') < push.indexOf('appendTick'));
assert.ok(/isSparseSecondChart/.test(push));
assert.ok(/isLiveSecondChart/.test(extractFn(boot, 'shouldVetoSecondsLiveIngest')));

console.log('sparse_live_ingest_test: ALL PASS');
