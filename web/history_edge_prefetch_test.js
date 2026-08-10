/**
 * P1 edge prefetch threshold math (zoom-aware runway).
 * Run: node web/history_edge_prefetch_test.js
 */
'use strict';

const assert = require('assert');
const ViewportManager = require('./ui/viewport-manager.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('historyEdgePrefetchBars: hard floor at 50', () => {
  assert.strictEqual(ViewportManager.historyEdgePrefetchBars(100, 50, 0.25), 50);
  assert.strictEqual(ViewportManager.historyEdgePrefetchBars(40, 50, 0.25), 50);
});

test('historyEdgePrefetchBars: 25% of wide viewport (not 25% of store)', () => {
  // visible 2000 → runway 500 (not 0.25 * 3000 store)
  assert.strictEqual(ViewportManager.historyEdgePrefetchBars(2000, 50, 0.25), 500);
});

test('right prefetch: wide zoom triggers far from tip', () => {
  const tip = 2999;
  const wide = { from: 0, to: 2000 };
  // bars to tip = 999; runway = 500 → not yet
  assert.strictEqual(
    ViewportManager.isWithinRightEdgePrefetch(wide, tip, { hardMin: 50, frac: 0.25 }),
    false,
  );
  const nearer = { from: 500, to: 2500 };
  // bars to tip = 499; runway = 500 → yes
  assert.strictEqual(
    ViewportManager.isWithinRightEdgePrefetch(nearer, tip, { hardMin: 50, frac: 0.25 }),
    true,
  );
});

test('right prefetch: tight zoom still uses hard min 50', () => {
  const tip = 2999;
  const tight = { from: 2900, to: 3050 }; // visible 150 → runway max(50, 37.5)=50
  assert.strictEqual(
    ViewportManager.isWithinRightEdgePrefetch(
      { from: 2900, to: 2940 },
      tip,
      { hardMin: 50, frac: 0.25 },
    ),
    false,
  );
  assert.strictEqual(
    ViewportManager.isWithinRightEdgePrefetch(
      { from: 2900, to: 2950 },
      tip,
      { hardMin: 50, frac: 0.25 },
    ),
    true,
  );
});

test('left prefetch: zoom-aware runway', () => {
  assert.strictEqual(
    ViewportManager.isWithinLeftEdgePrefetch({ from: 100, to: 250 }, { hardMin: 50, frac: 0.25 }),
    false,
  );
  assert.strictEqual(
    ViewportManager.isWithinLeftEdgePrefetch({ from: 40, to: 190 }, { hardMin: 50, frac: 0.25 }),
    true,
  );
  // wide visible 800 → runway 200
  assert.strictEqual(
    ViewportManager.isWithinLeftEdgePrefetch({ from: 150, to: 950 }, { hardMin: 50, frac: 0.25 }),
    true,
  );
});

console.log('history_edge_prefetch_test: ALL PASS');
