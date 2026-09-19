/**
 * CHANNEL-SPLIT-FILLS-1 — mid-split interior fills + boundColor.
 * Run: node web/channel_split_fills_test.js
 */
'use strict';

const assert = require('assert');
const {
  ChannelSeries,
  presentPaint,
  fillModelConflict,
  strokeModelConflict,
} = require('./channel-series.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function recordingTarget(ops, mediaHeight) {
  return {
    useMediaCoordinateSpace(fn) {
      const ctx = {
        fillStyle: '',
        strokeStyle: '',
        lineWidth: 1,
        canvas: { width: 100, height: mediaHeight || 80 },
        beginPath() { ops.push(['beginPath']); },
        moveTo(x, y) { ops.push(['moveTo', x, y]); },
        lineTo(x, y) { ops.push(['lineTo', x, y]); },
        closePath() { ops.push(['closePath']); },
        fill() { ops.push(['fill', this.fillStyle]); },
        stroke() { ops.push(['stroke', this.strokeStyle]); },
        setLineDash() {},
      };
      fn({ context: ctx, mediaSize: { width: 100, height: mediaHeight || 80 } });
    },
  };
}

function twoBars() {
  return {
    bars: [
      { x: 10, originalData: { upper: 20, mid: 40, lower: 60 } },
      { x: 16, originalData: { upper: 22, mid: 42, lower: 62 } },
    ],
    visibleRange: { from: 0, to: 2 },
    barSpacing: 6,
  };
}

function draw(opts, data) {
  const ops = [];
  const series = new ChannelSeries();
  const r = series.renderer();
  r.update(data || twoBars(), opts || {});
  r.draw(recordingTarget(ops, 80), (p) => p);
  return ops;
}

test('presentPaint: missing/blank is not a region', () => {
  assert.strictEqual(presentPaint({}, 'fillColor'), '');
  assert.strictEqual(presentPaint({ upperFillColor: 'rgba(1,2,3,0.1)' }, 'upperFillColor'), 'rgba(1,2,3,0.1)');
});

test('defaultOptions does not resurrect fill or bound capabilities', () => {
  const opts = new ChannelSeries().defaultOptions();
  for (const k of ['fillColor', 'upperFillColor', 'lowerFillColor', 'boundColor', 'upperColor', 'lowerColor']) {
    assert.ok(!Object.prototype.hasOwnProperty.call(opts, k), k);
  }
});

test('mutual exclusion helpers', () => {
  assert.strictEqual(fillModelConflict({ fillColor: 'a', upperFillColor: 'b' }), true);
  assert.strictEqual(fillModelConflict({ fillColor: 'a' }), false);
  assert.strictEqual(fillModelConflict({ upperFillColor: 'a', lowerFillColor: 'b' }), false);
  assert.strictEqual(strokeModelConflict({ boundColor: 'a', upperColor: 'b' }), true);
  assert.strictEqual(strokeModelConflict({ boundColor: 'a' }), false);
  assert.strictEqual(strokeModelConflict({ upperColor: 'a', lowerColor: 'b' }), false);
});

test('whole-band fillColor: one interior fill, no split', () => {
  const ops = draw({
    fillColor: 'rgba(0,136,255,0.12)',
    upperColor: 'blue',
    midColor: 'orange',
    lowerColor: 'navy',
  });
  const fills = ops.filter((op) => op[0] === 'fill');
  assert.strictEqual(fills.length, 1);
  assert.strictEqual(fills[0][1], 'rgba(0,136,255,0.12)');
});

test('split fills: upper↔mid then mid↔lower; no pane-edge y=0/80 fill path required', () => {
  const ops = draw({
    upperFillColor: 'up',
    lowerFillColor: 'dn',
    boundColor: 'edge',
    midColor: 'mid',
  });
  const fills = ops.filter((op) => op[0] === 'fill').map((op) => op[1]);
  assert.deepStrictEqual(fills, ['up', 'dn']);
});

test('upper fill only', () => {
  const ops = draw({ upperFillColor: 'up', boundColor: 'e', midColor: 'm' });
  assert.deepStrictEqual(ops.filter((op) => op[0] === 'fill').map((op) => op[1]), ['up']);
});

test('lower fill only', () => {
  const ops = draw({ lowerFillColor: 'dn', boundColor: 'e', midColor: 'm' });
  assert.deepStrictEqual(ops.filter((op) => op[0] === 'fill').map((op) => op[1]), ['dn']);
});

test('fillColor + split fills: draw neither fill model', () => {
  const ops = draw({
    fillColor: 'whole',
    upperFillColor: 'up',
    lowerFillColor: 'dn',
    boundColor: 'e',
    midColor: 'm',
  });
  assert.strictEqual(ops.filter((op) => op[0] === 'fill').length, 0);
});

test('Volume contract: boundColor + whole fillColor; mid last', () => {
  const ops = draw({
    fillColor: 'in',
    boundColor: 'EDGE',
    midColor: 'MID',
  });
  const kinds = ops.filter((op) => op[0] === 'fill' || op[0] === 'stroke').map((op) => op[0] + ':' + op[1]);
  assert.deepStrictEqual(kinds, [
    'fill:in',
    'stroke:EDGE',
    'stroke:EDGE',
    'stroke:MID',
  ]);
});

test('boundColor: both edges same; mid stroke last', () => {
  const ops = draw({
    upperFillColor: 'up',
    lowerFillColor: 'dn',
    boundColor: 'EDGE',
    midColor: 'MID',
  });
  const kinds = ops.filter((op) => op[0] === 'fill' || op[0] === 'stroke').map((op) => op[0] + ':' + op[1]);
  assert.deepStrictEqual(kinds, [
    'fill:up',
    'fill:dn',
    'stroke:EDGE',
    'stroke:EDGE',
    'stroke:MID',
  ]);
});

test('gap flushes split polygons', () => {
  const data = {
    bars: [
      { x: 10, originalData: { upper: 20, mid: 40, lower: 60 } },
      { x: 16, originalData: { upper: 21, mid: 41, lower: 61 } },
      { x: 30 },
      { x: 40, originalData: { upper: 24, mid: 44, lower: 64 } },
      { x: 46, originalData: { upper: 25, mid: 45, lower: 65 } },
    ],
    visibleRange: { from: 0, to: 5 },
    barSpacing: 6,
  };
  const ops = draw({
    upperFillColor: 'up',
    lowerFillColor: 'dn',
    boundColor: 'e',
    midColor: 'm',
  }, data);
  assert.strictEqual(ops.filter((op) => op[0] === 'fill').length, 4);
});

test('split options do not change priceValueBuilder', () => {
  const series = new ChannelSeries();
  const row = { upper: 80, mid: 50, lower: 20 };
  assert.deepStrictEqual(series.priceValueBuilder(row), [20, 80, 50]);
  series.update(twoBars(), {
    upperFillColor: 'rgba(1,2,3,0.2)',
    lowerFillColor: 'rgba(4,5,6,0.2)',
    boundColor: 'blue',
  });
  assert.deepStrictEqual(series.priceValueBuilder(row), [20, 80, 50]);
});

console.log('channel_split_fills_test: ALL PASS');
