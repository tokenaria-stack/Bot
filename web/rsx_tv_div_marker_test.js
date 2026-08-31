/**
 * RSX-VISIBILITY-1 — arrow-only markers; FE visibility mask filters by source.
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

test('fractal class captions stay on the marker', () => {
  const m = Mappers.annotationToNativeMarker({
    time: 1700000000,
    pane: 'rsx',
    label: 'A Bull',
    color: '#00e676',
    position: 'belowBar',
    shape: 'arrowUp',
    source: 'rsx_fractal_div',
  });
  assert.strictEqual(m.text, 'A Bull');
  assert.strictEqual(m.source, 'rsx_fractal_div');
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

test('visibility mask hides only the matching source', () => {
  const anns = [
    { time: 1, pane: 'rsx', label: '', source: 'rsx_tv_div' },
    { time: 2, pane: 'rsx', label: '', source: 'rsx_tv_pivot' },
    { time: 3, pane: 'rsx', label: 'A Bull', source: 'rsx_fractal_div' },
    { time: 4, pane: 'rsx', label: '', source: 'rsx_fractal_pivot' },
    { time: 5, pane: 'rsx', label: 'H Bull', source: 'rsx_zz_div' },
  ];
  const paint = (settings) => anns.filter((ann) => {
    const mask = Mappers.rsxVisibilityMask(settings);
    return Mappers.rsxAnnotationSourceVisible(ann.source, mask)
      && Mappers.annotationToNativeMarker(ann) != null;
  });
  const allOn = paint({});
  assert.strictEqual(allOn.length, 5);
  assert.strictEqual(paint({ show_tv_div: false }).map((a) => a.source).join(','),
    'rsx_tv_pivot,rsx_fractal_div,rsx_fractal_pivot,rsx_zz_div');
  assert.strictEqual(paint({ show_zz_div: false }).map((a) => a.source).join(','),
    'rsx_tv_div,rsx_tv_pivot,rsx_fractal_div,rsx_fractal_pivot');
  assert.strictEqual(paint({ show_fractal_pivot: false }).map((a) => a.source).join(','),
    'rsx_tv_div,rsx_tv_pivot,rsx_fractal_div,rsx_zz_div');
  assert.strictEqual(paint({ show_tv_pivot: false, show_fractal_pivot: true }).map((a) => a.source).join(','),
    'rsx_tv_div,rsx_fractal_div,rsx_fractal_pivot,rsx_zz_div');
  assert.strictEqual(paint({ show_tv_pivot: true, show_fractal_pivot: false }).map((a) => a.source).join(','),
    'rsx_tv_div,rsx_tv_pivot,rsx_fractal_div,rsx_zz_div');
  const allOff = paint({
    show_tv_div: false,
    show_tv_pivot: false,
    show_zz_div: false,
    show_fractal_div: false,
    show_fractal_pivot: false,
  });
  assert.strictEqual(allOff.length, 0);
  assert.strictEqual(anns.length, 5, 'facts in the store stay unchanged');
  assert.strictEqual(paint({}).length, 5, 're-enable paints stored markers without new facts');
  assert.strictEqual(Mappers.rsxVisibilityMask({ show_pivots: false }), 31, 'old show_pivots is ignored');
  assert.strictEqual(Mappers.rsxAnnotationSourceVisible('future_src', 0), true);
  assert.strictEqual(
    Mappers.rsxVisibleFactSources({
      show_tv_div: true,
      show_tv_pivot: false,
      show_zz_div: false,
      show_fractal_div: true,
      show_fractal_pivot: false,
    }).join(','),
    'rsx_tv_div,rsx_fractal_div',
  );
  assert.strictEqual(
    Mappers.rsxVisibleFactSources({
      show_tv_div: false,
      show_tv_pivot: false,
      show_zz_div: false,
      show_fractal_div: false,
      show_fractal_pivot: false,
    }).join(','),
    '',
  );
});

test('normalize pane stays rsx', () => {
  assert.strictEqual(Mappers.normalizeAnnotationPane('rsx'), 'rsx');
  assert.strictEqual(Mappers.normalizeAnnotationPane('pane_osc'), 'rsx');
});

console.log('rsx_tv_div_marker_test.js passed');
