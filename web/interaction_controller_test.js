/**
 * ADR-024 InteractionController unit tests (Node).
 * Run: node web/interaction_controller_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

const InteractionController = require('./ui/interaction-controller.js');
const TimeCamera = require('./ui/time-camera.js');
const CrosshairController = require('./ui/crosshair-controller.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('no LWC imports in InteractionController source', () => {
  const src = fs.readFileSync(path.join(__dirname, 'ui/interaction-controller.js'), 'utf8');
  assert.ok(!/lightweight-charts/i.test(src), 'no lightweight-charts string');
  assert.ok(!/\bLightweightCharts\b/.test(src), 'no LightweightCharts global');
  assert.ok(!/subscribeVisibleLogicalRangeChange|subscribeCrosshairMove|setCrosshairPosition/.test(src),
    'no LWC API calls');
});

test('onPointerEnter / Leave route to CrosshairController only', () => {
  CrosshairController._resetForTests();
  InteractionController._resetForTests();
  const maps = [];
  CrosshairController.bind({
    applyHorzVisibility: (m) => maps.push({ ...m }),
    syncPeerTime: () => {},
    clearPeerCrosshairs: () => {},
  });
  assert.strictEqual(InteractionController.onPointerEnter('rsx'), true);
  assert.strictEqual(CrosshairController.getHovered(), 'rsx');
  assert.strictEqual(maps[maps.length - 1].rsx, true);
  assert.strictEqual(InteractionController.onPointerLeave(), true);
  assert.strictEqual(CrosshairController.getHovered(), null);
});

test('onRangeChanged routes to TimeCamera.proposeFromPane', () => {
  TimeCamera._resetForTests();
  InteractionController._resetForTests();
  const applied = [];
  TimeCamera.bind({
    applyCommitted: (s) => applied.push(s),
  });
  assert.strictEqual(
    InteractionController.onRangeChanged('wozduh', { from: 10, to: 20 }, 6),
    true,
  );
  assert.strictEqual(applied.length, 1);
  assert.deepStrictEqual(applied[0].visibleRange, { from: 10, to: 20 });
  assert.strictEqual(applied[0].barSpacing, 6);
  assert.strictEqual(applied[0].sourceHostId, 'wozduh');
});

test('onCrosshairMove routes syncTime; non-hovered source ignored by CrosshairController', () => {
  CrosshairController._resetForTests();
  InteractionController._resetForTests();
  const peers = [];
  CrosshairController.bind({
    applyHorzVisibility: () => {},
    syncPeerTime: (sourceHostId, time) => peers.push({ sourceHostId, time }),
    clearPeerCrosshairs: () => {},
  });
  InteractionController.onPointerEnter('price');
  assert.strictEqual(InteractionController.onCrosshairMove('price', 100), true);
  assert.strictEqual(InteractionController.onCrosshairMove('rsx', 200), false);
  assert.deepStrictEqual(peers, [{ sourceHostId: 'price', time: 100 }]);
  assert.strictEqual(CrosshairController.getHovered(), 'price');
});

test('chart-core interaction paths go through InteractionController', () => {
  const src = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  assert.ok(src.includes('InteractionController.onRangeChanged'));
  assert.ok(src.includes('InteractionController.onPointerEnter'));
  assert.ok(src.includes('InteractionController.onPointerLeave'));
  assert.ok(src.includes('InteractionController.onCrosshairMove'));
  // Interaction subscribe bodies must not call controllers directly.
  const rangeBlock = src.slice(
    src.indexOf('function subscribePaneTimeProposals'),
    src.indexOf('function crosshairSeriesForChart'),
  );
  assert.ok(!rangeBlock.includes('TimeCamera.proposeFromPane'),
    'subscribePaneTimeProposals must not call TimeCamera directly');
  const hoverBlock = src.slice(
    src.indexOf('function bindPointerHoverOwnership'),
    src.indexOf('function bindCrosshairController'),
  );
  assert.ok(!hoverBlock.includes('CrosshairController.setHovered'),
    'pointer hover must route via InteractionController');
});

console.log('interaction_controller_test: ALL PASS');
