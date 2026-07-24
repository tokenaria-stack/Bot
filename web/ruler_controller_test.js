/**
 * ADR-025 Ruler completion tests (Node).
 * Run: node web/ruler_controller_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

const RulerController = require('./ui/ruler-controller.js');
const RulerMetrics = require('./ui/ruler-metrics.js');
const InteractionController = require('./ui/interaction-controller.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('no LWC / DOM / x,y storage in RulerController', () => {
  const src = fs.readFileSync(path.join(__dirname, 'ui/ruler-controller.js'), 'utf8');
  assert.ok(!/LightweightCharts|document\.|getElementById|priceToCoordinate|logicalToCoordinate/.test(src));
  assert.ok(!/\bx:\s*|\by:\s*/.test(src) || src.includes('never x/y'));
});

test('RulerMetrics bars from logical only (weekend-safe)', () => {
  const m = RulerMetrics.compute(
    { logical: 100, price: 100, time: 1_700_000_000 },
    { logical: 145, price: 100.06, time: 1_700_000_000 + 48 * 3600 },
    { intervalMs: 60_000, minMove: 0.01 },
  );
  assert.strictEqual(m.bars, 45);
  assert.ok(Math.abs(m.deltaPrice - 0.06) < 1e-9);
  assert.ok(Math.abs(m.deltaPercent - 0.06) < 1e-6);
  assert.strictEqual(m.ticks, 6);
  assert.strictEqual(m.durationEstimated, false);
});

test('RulerMetrics estimates duration when time missing', () => {
  const m = RulerMetrics.compute(
    { logical: 10, price: 50, time: null },
    { logical: 20, price: 55, time: null },
    { intervalMs: 60_000 },
  );
  assert.strictEqual(m.bars, 10);
  assert.strictEqual(m.durationEstimated, true);
  assert.strictEqual(m.durationMs, 10 * 60_000);
  const lines = RulerMetrics.tooltipLines(m);
  assert.ok(lines.line1.includes('%'));
  assert.ok(lines.line2.includes('bars'));
});

test('two-click FSM: armed → placing → finished; pointerUp ignored', () => {
  RulerController._resetForTests();
  const geos = [];
  RulerController.bind({ render: (g) => geos.push(g) });
  RulerController.arm();

  assert.strictEqual(
    RulerController.onPointerDown('price', { logical: 1, price: 100, time: null }),
    true,
  );
  assert.strictEqual(RulerController.getState(), 'placing');
  assert.strictEqual(RulerController.onPointerUp('price'), false);
  assert.strictEqual(RulerController.getState(), 'placing');

  assert.strictEqual(
    RulerController.onPointerMove('price', { logical: 5, price: 110, time: null }),
    true,
  );
  assert.strictEqual(RulerController.getGeometry().anchorB.logical, 5);

  assert.strictEqual(
    RulerController.onPointerDown('price', { logical: 9, price: 120, time: 42 }),
    true,
  );
  assert.strictEqual(RulerController.getState(), 'finished');
  const geo = RulerController.getGeometry();
  assert.strictEqual(geo.anchorA.logical, 1);
  assert.strictEqual(geo.anchorB.logical, 9);
  assert.strictEqual(geo.preview, false);
  assert.ok(!Object.prototype.hasOwnProperty.call(geo.anchorA, 'x'));
});

test('third click while finished: clear → armed (no new A)', () => {
  RulerController._resetForTests();
  RulerController.bind({ render: () => {} });
  RulerController.arm();
  RulerController.onPointerDown('price', { logical: 1, price: 100, time: null });
  RulerController.onPointerDown('price', { logical: 9, price: 120, time: null });
  assert.strictEqual(RulerController.getState(), 'finished');

  assert.strictEqual(
    RulerController.onPointerDown('price', { logical: 50, price: 200, time: null }),
    true,
  );
  assert.strictEqual(RulerController.getState(), 'armed');
  assert.strictEqual(RulerController.getGeometry(), null);
  assert.strictEqual(RulerController.isActive(), true);

  // Fourth click starts a fresh measure at the new point.
  assert.strictEqual(
    RulerController.onPointerDown('price', { logical: 50, price: 200, time: null }),
    true,
  );
  assert.strictEqual(RulerController.getState(), 'placing');
  assert.strictEqual(RulerController.getGeometry().anchorA.logical, 50);
});

test('cancel mid-placing → armed; geometry cleared', () => {
  RulerController._resetForTests();
  RulerController.bind({ render: () => {} });
  RulerController.arm();
  RulerController.onPointerDown('price', { logical: 1, price: 1, time: null });
  assert.strictEqual(RulerController.cancel(), true);
  assert.strictEqual(RulerController.getState(), 'armed');
  assert.strictEqual(RulerController.getGeometry(), null);
  assert.strictEqual(RulerController.isActive(), true);
});

test('empty-space anchors: time null accepted', () => {
  RulerController._resetForTests();
  RulerController.bind({ render: () => {} });
  RulerController.arm();
  assert.strictEqual(
    RulerController.onPointerDown('price', { logical: 9999, price: 1.23, time: null }),
    true,
  );
  assert.strictEqual(
    RulerController.onPointerDown('price', { logical: 10010, price: 1.5, time: null }),
    true,
  );
  assert.strictEqual(RulerController.getGeometry().anchorA.time, null);
});

test('IC routes cancel + pointer; chart-core projects via logical', () => {
  RulerController._resetForTests();
  InteractionController._resetForTests();
  RulerController.bind({ render: () => {} });
  RulerController.arm();
  assert.strictEqual(
    InteractionController.onPointerDown('price', { logical: 2, price: 10, time: null }),
    true,
  );
  assert.strictEqual(InteractionController.onCancel(), true);
  assert.strictEqual(RulerController.getState(), 'armed');

  const src = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  assert.ok(src.includes('logicalToCoordinate'));
  assert.ok(src.includes('coordinateToLogical'));
  assert.ok(src.includes('RulerMetrics'));
  assert.ok(src.includes('InteractionController.onCancel'));
  assert.ok(!src.includes('ruler-guide-v') || src.includes('finite'));
});

console.log('ruler_controller_test: ALL PASS');
