/**
 * RSX-SIGNAL-1.1 — arrow-only TV div/pivot markers; Show Pivots filters paint.
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

test('empty caption stays arrow-only', () => {
  const m = Mappers.annotationToNativeMarker({
    time: 1700000000,
    pane: 'rsx',
    label: '',
    color: '#00e676',
    position: 'belowBar',
    shape: 'arrowUp',
    source: 'rsx_tv_div',
  });
  assert.strictEqual(m.text, '');
  assert.strictEqual(m.shape, 'arrowUp');
  assert.strictEqual(m.color, '#00e676');
});

test('pivot annotation keeps blue color, no Pivot text', () => {
  const m = Mappers.annotationToNativeMarker({
    time: 1700000000,
    pane: 'rsx',
    label: '',
    color: '#2979ff',
    position: 'aboveBar',
    shape: 'arrowDown',
    source: 'rsx_tv_pivot',
  });
  assert.strictEqual(m.text, '');
  assert.strictEqual(m.shape, 'arrowDown');
  assert.strictEqual(m.source, 'rsx_tv_pivot');
});

test('hidden zz keeps H Bull text', () => {
  const m = Mappers.annotationToNativeMarker({
    time: 1700000000,
    pane: 'rsx',
    label: 'H Bull',
    color: '#00e676',
    position: 'belowBar',
    shape: 'arrowUp',
    source: 'rsx_zz_div',
  });
  assert.strictEqual(m.text, 'H Bull');
  assert.strictEqual(m.source, 'rsx_zz_div');
});

test('legacy labels stay unpublished', () => {
  assert.strictEqual(Mappers.annotationToNativeMarker({ time: 1, label: 'L' }), null);
  assert.strictEqual(Mappers.annotationToNativeMarker({ time: 1, label: 'SS' }), null);
  assert.strictEqual(Mappers.annotationToNativeMarker({ time: 1, label: 'P' }), null);
});

test('normalize pane stays rsx', () => {
  assert.strictEqual(Mappers.normalizeAnnotationPane('rsx'), 'rsx');
  assert.strictEqual(Mappers.normalizeAnnotationPane('pane_osc'), 'rsx');
});

console.log('rsx_tv_div_marker_test.js passed');
