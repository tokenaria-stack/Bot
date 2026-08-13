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

test('third click while finished: one-shot exit → idle + onActiveChange(false)', () => {
  RulerController._resetForTests();
  const activeEvents = [];
  RulerController.bind({
    render: () => {},
    onActiveChange: (active) => activeEvents.push(active),
  });
  RulerController.arm();
  assert.deepStrictEqual(activeEvents, [true]);
  RulerController.onPointerDown('price', { logical: 1, price: 100, time: null });
  RulerController.onPointerDown('price', { logical: 9, price: 120, time: null });
  assert.strictEqual(RulerController.getState(), 'finished');

  assert.strictEqual(
    RulerController.onPointerDown('price', { logical: 50, price: 200, time: null }),
    true,
  );
  assert.strictEqual(RulerController.getState(), 'idle');
  assert.strictEqual(RulerController.getGeometry(), null);
  assert.strictEqual(RulerController.isActive(), false);
  assert.deepStrictEqual(activeEvents, [true, false]);

  // Further clicks do nothing until re-armed via toolbar.
  assert.strictEqual(
    RulerController.onPointerDown('price', { logical: 50, price: 200, time: null }),
    false,
  );
  assert.strictEqual(RulerController.getState(), 'idle');
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

test('F5e toUnixSec: LWC Unix seconds only (no 1e12)', () => {
  assert.strictEqual(RulerMetrics.toUnixSec(1_502_928_000), 1_502_928_000);
  assert.strictEqual(RulerMetrics.toUnixSec(730_944_000), 730_944_000);
  assert.strictEqual(RulerMetrics.toUnixSec(1_502_928_000.9), 1_502_928_000);
  assert.strictEqual(RulerMetrics.toUnixSec({ timestamp: 1_700_000_000 }), 1_700_000_000);
  assert.strictEqual(RulerMetrics.toUnixSec({ year: 1993, month: 3, day: 1 }), null);
  assert.strictEqual(RulerMetrics.toUnixSec(null), null);

  const m = RulerMetrics.compute(
    { logical: 0, price: 100, time: 1_502_928_000 },
    { logical: 1, price: 101, time: 1_502_928_000 + 60 },
    {},
  );
  assert.strictEqual(m.durationEstimated, false);
  assert.strictEqual(m.durationMs, 60 * 1000);

  const metricsSrc = fs.readFileSync(path.join(__dirname, 'ui/ruler-metrics.js'), 'utf8');
  const body = metricsSrc.match(/function toUnixSec\(t\) \{[\s\S]*?\n  \}/)[0];
  assert.ok(!body.includes('1e12'), body);
  assert.ok(!body.includes('/ 1000'), body);
});

console.log('ruler_controller_test: ALL PASS');
