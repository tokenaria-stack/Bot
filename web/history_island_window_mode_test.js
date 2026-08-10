/**
 * P0 — HISTORY island must not look like a live transport gap.
 * Run: node web/history_island_window_mode_test.js
 */
'use strict';

const assert = require('assert');
const { ColumnarStore } = require('./columnar-store.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function monolith(n, startSec) {
  const times = Array.from({ length: n }, (_, i) => startSec + i * 60);
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

test('commit-paired default stays live (FreshLive / tip hydrate)', () => {
  const store = new ColumnarStore();
  store.setTfInterval(60);
  store.replaceMonolith(monolith(10, 1_700_000_000), { commitPaired: true });
  assert.strictEqual(store.windowMode, 'live');
});

test('commit-paired windowMode:history marks Microscope island', () => {
  const store = new ColumnarStore();
  store.setTfInterval(60);
  // Sep 2025 island while wall clock is far ahead
  store.replaceMonolith(monolith(10, 1_757_120_400), {
    commitPaired: true,
    windowMode: 'history',
  });
  assert.strictEqual(store.windowMode, 'history');
});

test('history island stays history after replace (Boot skips WS ingest / gap-heal)', () => {
  const store = new ColumnarStore();
  store.setTfInterval(60);
  store.replaceMonolith(monolith(5, 1_757_120_400), {
    commitPaired: true,
    windowMode: 'history',
  });
  assert.strictEqual(store.windowMode, 'history');
  assert.strictEqual(store.isLiveWindow(), false);
  // pushLiveTickDelta returns early when windowMode==='history' — no Timeline heal.
});

test('appendMonolith promotes to live when tip catches wall clock', () => {
  const store = new ColumnarStore();
  store.setTfInterval(60);
  const now = Math.floor(Date.now() / 1000);
  const start = now - 10 * 60;
  store.replaceMonolith(monolith(5, start), {
    commitPaired: true,
    windowMode: 'history',
  });
  assert.strictEqual(store.windowMode, 'history');
  const tip = store.lastTimeSec();
  store.appendMonolith(monolith(5, tip + 60));
  assert.strictEqual(store.windowMode, 'live', 'near-live tip must rejoin LIVE ingest');
});

console.log('history_island_window_mode_test: ALL PASS');
