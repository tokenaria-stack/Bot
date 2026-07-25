/**
 * ADR-027 Phase 1 — TimelineDecoration unit tests (Node).
 * Run: node web/timeline_decoration_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const TimelineDecoration = require('./ui/timeline-decoration.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function fakeChart() {
  const created = [];
  return {
    created,
    addLineSeries(opts) {
      const series = {
        opts,
        data: null,
        removed: false,
        setData(d) { this.data = d; },
        remove() { this.removed = true; },
      };
      created.push(series);
      return series;
    },
  };
}

test('public surface is attach / refresh / dispose only (product API)', () => {
  const keys = Object.keys(TimelineDecoration).filter((k) => !k.startsWith('_'));
  assert.deepStrictEqual(keys.sort(), ['attach', 'dispose', 'refresh']);
  assert.strictEqual(typeof TimelineDecoration.update, 'undefined');
  assert.strictEqual(typeof TimelineDecoration.getSeries, 'undefined');
  assert.strictEqual(typeof TimelineDecoration.series, 'undefined');
});

test('attach creates internal series with mandatory chrome options', () => {
  TimelineDecoration._resetForTests();
  const chart = fakeChart();
  assert.strictEqual(TimelineDecoration.attach(chart), true);
  assert.strictEqual(chart.created.length, 1);
  const opts = chart.created[0].opts;
  assert.strictEqual(typeof opts.autoscaleInfoProvider, 'function');
  assert.strictEqual(opts.autoscaleInfoProvider(), null);
  assert.strictEqual(opts.lastValueVisible, false);
  assert.strictEqual(opts.priceLineVisible, false);
  assert.strictEqual(opts.crosshairMarkerVisible, false);
  assert.strictEqual(opts.lineVisible, false);
  assert.strictEqual(opts.title, '__timeline_decoration__');
});

test('attach is idempotent per chart', () => {
  TimelineDecoration._resetForTests();
  const chart = fakeChart();
  assert.strictEqual(TimelineDecoration.attach(chart), true);
  assert.strictEqual(TimelineDecoration.attach(chart), true);
  assert.strictEqual(chart.created.length, 1);
});

test('refresh setData whitespace; clear on empty times', () => {
  TimelineDecoration._resetForTests();
  const chart = fakeChart();
  TimelineDecoration.attach(chart);
  const series = chart.created[0];
  assert.strictEqual(
    TimelineDecoration.refresh({ times: [100, 160, { time: 220 }] }),
    true,
  );
  assert.deepStrictEqual(series.data, [
    { time: 100 },
    { time: 160 },
    { time: 220 },
  ]);
  assert.strictEqual(TimelineDecoration.refresh({ times: [] }), true);
  assert.deepStrictEqual(series.data, []);
});

test('refresh fans out to all attached charts', () => {
  TimelineDecoration._resetForTests();
  const a = fakeChart();
  const b = fakeChart();
  TimelineDecoration.attach(a);
  TimelineDecoration.attach(b);
  TimelineDecoration.refresh({ times: [1, 2] });
  assert.deepStrictEqual(a.created[0].data, [{ time: 1 }, { time: 2 }]);
  assert.deepStrictEqual(b.created[0].data, [{ time: 1 }, { time: 2 }]);
});

test('dispose removes series and clears attachments', () => {
  TimelineDecoration._resetForTests();
  const chart = fakeChart();
  TimelineDecoration.attach(chart);
  const series = chart.created[0];
  assert.strictEqual(TimelineDecoration.dispose(), true);
  assert.strictEqual(series.removed, true);
  // After dispose, refresh is a no-op until re-attach.
  assert.strictEqual(TimelineDecoration.refresh({ times: [9] }), false);
});

test('source: no update() / getSeries; never touches candleSeries', () => {
  const src = fs.readFileSync(path.join(__dirname, 'ui/timeline-decoration.js'), 'utf8');
  assert.ok(!/\.update\s*\(/.test(src), 'must never call series.update');
  assert.ok(!/function getSeries|\.getSeries\b/.test(src));
  assert.ok(!/\bcandleSeries\b|\bliveColumnarStore\b|\bColumnarStore\b/.test(src));
  assert.ok(!/require\s*\(|DisplayTimeline\./.test(src), 'no DisplayTimeline coupling');
  assert.ok(src.includes('autoscaleInfoProvider'));
  assert.ok(src.includes('attach') && src.includes('refresh') && src.includes('dispose'));
});

test('autoscaleInfoProvider factory returns null', () => {
  const opts = TimelineDecoration._decorationSeriesOptionsForTests();
  assert.strictEqual(opts.autoscaleInfoProvider(), null);
});

console.log('timeline_decoration_test: ALL PASS');
