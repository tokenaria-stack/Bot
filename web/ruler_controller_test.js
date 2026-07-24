/**
 * ADR-025 RulerController + IC routing tests (Node).
 * Run: node web/ruler_controller_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

const RulerController = require('./ui/ruler-controller.js');
const InteractionController = require('./ui/interaction-controller.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('no LWC / DOM in RulerController source', () => {
  const src = fs.readFileSync(path.join(__dirname, 'ui/ruler-controller.js'), 'utf8');
  assert.ok(!/LightweightCharts|document\.|getElementById|priceToCoordinate|addEventListener/.test(src));
});

test('lifecycle Idle → Armed → Dragging → Finished', () => {
  RulerController._resetForTests();
  const geos = [];
  RulerController.bind({ render: (g) => geos.push(g) });

  assert.strictEqual(RulerController.getState(), 'idle');
  assert.strictEqual(RulerController.onPointerDown('price', { time: 1, price: 100 }), false);

  RulerController.arm();
  assert.strictEqual(RulerController.getState(), 'armed');
  assert.strictEqual(RulerController.isActive(), true);

  assert.strictEqual(
    RulerController.onPointerDown('price', { time: 10, price: 100 }),
    true,
  );
  assert.strictEqual(RulerController.getState(), 'dragging');
  assert.deepStrictEqual(RulerController.getGeometry(), {
    hostId: 'price',
    startTime: 10,
    endTime: 10,
    startPrice: 100,
    endPrice: 100,
  });

  assert.strictEqual(
    RulerController.onPointerMove('price', { time: 20, price: 110 }),
    true,
  );
  assert.strictEqual(RulerController.getGeometry().endPrice, 110);

  assert.strictEqual(RulerController.onPointerUp('price'), true);
  assert.strictEqual(RulerController.getState(), 'finished');
  assert.ok(geos.some((g) => g && g.endPrice === 110));
});

test('Phase 1 rejects non-price hostId', () => {
  RulerController._resetForTests();
  RulerController.arm();
  assert.strictEqual(
    RulerController.onPointerDown('rsx', { time: 1, price: 50 }),
    false,
  );
  assert.strictEqual(RulerController.getState(), 'armed');
});

test('InteractionController routes pointer down/move/up to RulerController', () => {
  RulerController._resetForTests();
  InteractionController._resetForTests();
  RulerController.bind({ render: () => {} });
  RulerController.arm();

  assert.strictEqual(
    InteractionController.onPointerDown('price', { time: 5, price: 200 }),
    true,
  );
  assert.strictEqual(RulerController.getState(), 'dragging');
  assert.strictEqual(
    InteractionController.onPointerMove('price', { time: 8, price: 210 }),
    true,
  );
  assert.strictEqual(
    InteractionController.onPointerUp('price'),
    true,
  );
  assert.strictEqual(RulerController.getState(), 'finished');
});

test('disarm clears geometry; toggle round-trip', () => {
  RulerController._resetForTests();
  RulerController.bind({ render: () => {} });
  RulerController.arm();
  RulerController.onPointerDown('price', { time: 1, price: 1 });
  RulerController.onPointerUp('price');
  assert.ok(RulerController.getGeometry());
  RulerController.disarm();
  assert.strictEqual(RulerController.getState(), 'idle');
  assert.strictEqual(RulerController.getGeometry(), null);
  RulerController.toggle();
  assert.strictEqual(RulerController.getState(), 'armed');
  RulerController.toggle();
  assert.strictEqual(RulerController.getState(), 'idle');
});

test('chart-core wires IC pointer routes and renderRuler (source contract)', () => {
  const src = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  assert.ok(src.includes('InteractionController.onPointerDown'));
  assert.ok(src.includes('InteractionController.onPointerMove'));
  assert.ok(src.includes('InteractionController.onPointerUp'));
  assert.ok(src.includes('function renderRuler'));
  assert.ok(src.includes('bindRulerController'));
  assert.ok(!/RulerController\.onPointerDown\(/.test(
    src.slice(src.indexOf('function bindRulerPointerRouting'), src.indexOf('function bindRulerController')),
  ), 'pointer routing must go through InteractionController');
});

console.log('ruler_controller_test: ALL PASS');
