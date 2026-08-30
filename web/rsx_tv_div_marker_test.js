/**
 * RSX-SIGNAL-1 — Bull/Bear annotations map to RSX-pane LWC markers.
 * Run: node web/rsx_tv_div_marker_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const src = fs.readFileSync(path.join(__dirname, 'mappers.js'), 'utf8');
const ctx = {
  window: {},
  console,
  DEFAULT_STRATEGY_THRESHOLDS: { long: 1, short: 1 },
  SCORING_MATRIX_DEFAULTS: {},
  ChartTheme: undefined,
};
vm.createContext(ctx);
vm.runInContext(src, ctx);
const Mappers = ctx.window.Mappers;
assert.ok(Mappers && typeof Mappers.annotationToNativeMarker === 'function');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('bull maps to up marker, not L/LL', () => {
  const m = Mappers.annotationToNativeMarker({
    time: 1700000000,
    pane: 'rsx',
    label: 'Bull',
    color: '#26a69a',
    position: 'belowBar',
    shape: 'arrowUp',
  });
  assert.strictEqual(m.text, 'Bull');
  assert.strictEqual(m.shape, 'arrowUp');
  assert.strictEqual(m.position, 'belowBar');
  assert.strictEqual(m.color, '#26a69a');
});

test('bear maps to down marker, not S/SS', () => {
  const m = Mappers.annotationToNativeMarker({
    time: 1700000000,
    pane: 'rsx',
    label: 'Bear',
    color: '#ef5350',
    position: 'aboveBar',
    shape: 'arrowDown',
  });
  assert.strictEqual(m.text, 'Bear');
  assert.strictEqual(m.shape, 'arrowDown');
  assert.strictEqual(m.position, 'aboveBar');
});

test('legacy L/LL/S/SS still dropped', () => {
  assert.strictEqual(Mappers.annotationToNativeMarker({ time: 1, label: 'L' }), null);
  assert.strictEqual(Mappers.annotationToNativeMarker({ time: 1, label: 'SS' }), null);
});

test('normalize pane stays rsx', () => {
  assert.strictEqual(Mappers.normalizeAnnotationPane('rsx'), 'rsx');
  assert.strictEqual(Mappers.normalizeAnnotationPane('pane_osc'), 'rsx');
});

console.log('rsx_tv_div_marker_test.js passed');
