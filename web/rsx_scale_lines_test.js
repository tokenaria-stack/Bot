/**
 * RSX-SCALE-1 — dotted 30 / 50 / 70 pane chrome (not DDR).
 * Run: node web/rsx_scale_lines_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const RsxScaleLines = require('./rsx-scale-lines.js');
const WozduhExtremeBands = require('./wozduh-extreme-bands.js');
const { DDRFactory } = require('./series-factory.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function fakeRsxChart() {
  const created = [];
  return {
    created,
    addLineSeries(opts) {
      const series = {
        opts,
        primitive: null,
        removed: false,
        data: null,
        attachPrimitive(p) {
          this.primitive = p;
          if (p && typeof p.attached === 'function') p.attached({ chart: this, series });
        },
        detachPrimitive() { this.primitive = null; },
        remove() { this.removed = true; },
        setData(d) { this.data = d; },
        priceToCoordinate(price) { return Number(price); },
      };
      created.push(series);
      return series;
    },
  };
}

test('A. decoration is not a DDR/settings/store identity', () => {
  const layout = fs.readFileSync(path.join(__dirname, '../ui_config/rsx_layout.go'), 'utf8');
  assert.ok(!layout.includes('RsxScaleLines'));
  assert.ok(!layout.includes('__rsx_scale'));
  const settings = fs.readFileSync(path.join(__dirname, 'ui/settings-renderer.js'), 'utf8');
  assert.ok(!settings.includes('RsxScaleLines'));
  const factory = new DDRFactory();
  factory.buildPanes(
    { rsx: { chart: { addLineSeries() { return { setData() {}, applyOptions() {}, priceScale() { return { applyOptions() {} }; } }; } } } },
    { pane_osc: [{ id: 'line_rsx', hostId: 'rsx', kind: 'line', renderOptions: {} }] },
  );
  assert.ok(!factory.seriesMap.has('__rsx_scale_lines__'));
  assert.ok(!factory.requestedPlotIds().includes('__rsx_scale_lines__'));
  const keys = Object.keys(RsxScaleLines).filter((k) => !k.startsWith('_'));
  assert.ok(keys.includes('attach'));
  assert.ok(keys.includes('refresh'));
  assert.ok(keys.includes('dispose'));
  assert.ok(!keys.includes('getSeries'));
  assert.ok(!keys.includes('setData'));
  assert.ok(!keys.includes('update'));
});

test('B. private host autoscale is null; line_rsx stays bounded owner', () => {
  const opts = RsxScaleLines._hostSeriesOptionsForTests();
  assert.strictEqual(opts.title, '');
  assert.strictEqual(typeof opts.autoscaleInfoProvider, 'function');
  assert.strictEqual(opts.autoscaleInfoProvider(), null);
  assert.strictEqual(opts.lineVisible, false);
  assert.strictEqual(opts.priceLineVisible, false);
  assert.strictEqual(opts.lastValueVisible, false);
  assert.strictEqual(opts.crosshairMarkerVisible, false);
  assert.strictEqual(opts.priceScaleId, 'right');
  const layout = fs.readFileSync(path.join(__dirname, '../ui_config/rsx_layout.go'), 'utf8');
  assert.ok(layout.includes('"ID":         "line_rsx"') || layout.includes('ID:         "line_rsx"'));
  assert.ok(layout.includes('"scaleContribution":{"type":"bounded","min":-5,"max":105}'));
  assert.ok(layout.includes('"lastValueVisible":false,"priceLineVisible":false,"scaleContribution":{"type":"bounded"'));
  assert.ok(layout.includes('"ID":         "line_rsx_signal"') || layout.includes('ID:         "line_rsx_signal"'));
  assert.ok(layout.includes('"scaleContribution":{"type":"ignore"}'));
  assert.ok(layout.includes('"lastValueVisible":false,"priceLineVisible":false,"scaleContribution":{"type":"ignore"}'));
  assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS.has('line_rsx'), true);
});

test('C. product law is exactly 30 / 50 / 70 dotted; no fill, no 20/80', () => {
  assert.deepStrictEqual(RsxScaleLines.LEVELS, { low: 30, mid: 50, high: 70 });
  const src = fs.readFileSync(path.join(__dirname, 'rsx-scale-lines.js'), 'utf8');
  assert.ok(!src.includes('createPriceLine'));
  assert.ok(!src.includes('fillRect'));
  assert.ok(!src.includes('createLinearGradient'));
  assert.ok(!src.includes('INDICATOR_CONFIG'));
  assert.ok(!src.includes('rsxLevels'));
  assert.ok(!src.includes('price: 20'));
  assert.ok(!src.includes('price: 80'));
  assert.ok(!src.includes('priceToCoordinate(20)'));
  assert.ok(!src.includes('priceToCoordinate(80)'));
});

test('D. dotted style matches Wozduh scale strokes', () => {
  assert.strictEqual(RsxScaleLines.STROKE, WozduhExtremeBands.STROKE);
  assert.deepStrictEqual(RsxScaleLines.DOTTED_DASH, [1, 2]);
  const woz = fs.readFileSync(path.join(__dirname, 'wozduh-extreme-bands.js'), 'utf8');
  assert.ok(woz.includes('const DOTTED_DASH = [1, 2]'));
  assert.ok(woz.includes("const STROKE = 'rgba(120, 123, 134, 0.85)'"));
});

test('E. renderer is O(1): no store/history/bar walk', () => {
  const src = fs.readFileSync(path.join(__dirname, 'rsx-scale-lines.js'), 'utf8');
  const draw = src.slice(src.indexOf('draw(target)'), src.indexOf('function strokeDotted'));
  assert.ok(!/ColumnarStore|hydratedData|requestedPlotIds|visibleRange|bars\.length/.test(draw));
  assert.ok(!/for\s*\(/.test(draw));
  assert.ok(src.includes('priceToCoordinate(LEVELS.low)'));
  assert.ok(src.includes('priceToCoordinate(LEVELS.mid)'));
  assert.ok(src.includes('priceToCoordinate(LEVELS.high)'));
});

test('F. lifecycle: one host + one primitive; dispose clears', () => {
  RsxScaleLines._resetForTests();
  const chart = fakeRsxChart();
  assert.strictEqual(RsxScaleLines.attach(chart), true);
  assert.strictEqual(chart.created.length, 1);
  assert.ok(chart.created[0].primitive);
  assert.strictEqual(RsxScaleLines._attachmentCountForTests(), 1);
  assert.strictEqual(RsxScaleLines.attach(chart), true);
  assert.strictEqual(chart.created.length, 1);
  assert.strictEqual(RsxScaleLines.dispose(), true);
  assert.strictEqual(chart.created[0].removed, true);
  assert.strictEqual(chart.created[0].primitive, null);
  assert.strictEqual(RsxScaleLines._attachmentCountForTests(), 0);
  const chart2 = fakeRsxChart();
  assert.strictEqual(RsxScaleLines.attach(chart2), true);
  assert.strictEqual(chart2.created.length, 1);
  RsxScaleLines.dispose();
});

test('refresh seeds one priced point (not whitespace-only)', () => {
  RsxScaleLines._resetForTests();
  const chart = fakeRsxChart();
  RsxScaleLines.attach(chart);
  assert.strictEqual(RsxScaleLines.refresh(1700000000), true);
  assert.deepStrictEqual(chart.created[0].data, [{ time: 1700000000, value: 50 }]);
  assert.strictEqual(RsxScaleLines.HOST_VALUE, 50);
  assert.strictEqual(RsxScaleLines.refresh(null), false);
  RsxScaleLines.dispose();
});

test('paintCandles reseeds even when skipDecoration; updateCandle does not', () => {
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  const paint = core.slice(core.indexOf('function paintCandles'), core.indexOf('function isOlderThanPaintedTip'));
  const update = core.slice(core.indexOf('function updateCandle'), core.indexOf('const ChartAdapter'));
  assert.ok(paint.includes('RsxScaleLines.refresh(state._lastRealCandleTime)'));
  assert.ok(paint.indexOf('skipDecoration') < paint.indexOf('RsxScaleLines.refresh'));
  assert.ok(!update.includes('RsxScaleLines.refresh'));
  assert.ok(!update.includes('RsxScaleLines'));
});

test('zOrder is bottom; chart-core wires RSX chart only', () => {
  const Prim = RsxScaleLines._RsxScaleLinesPrimitive;
  const p = new Prim();
  assert.strictEqual(p.paneViews()[0].zOrder(), 'bottom');
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  assert.ok(core.includes('RsxScaleLines.attach(rsxChart)'));
  assert.ok(core.includes('RsxScaleLines.dispose()'));
  assert.ok(!core.includes('RsxScaleLines.attach(priceChart)'));
  assert.ok(!core.includes('RsxScaleLines.attach(wozduhChart)'));
});

test('HIDDEN-RENDER-SKIP and series-factory stay unaware of the host', () => {
  const factory = fs.readFileSync(path.join(__dirname, 'series-factory.js'), 'utf8');
  const skip = fs.readFileSync(path.join(__dirname, 'hidden_render_skip_test.js'), 'utf8');
  assert.ok(!factory.includes('RsxScaleLines'));
  assert.ok(!skip.includes('RsxScaleLines'));
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  const seriesFn = core.slice(
    core.indexOf('function crosshairSeriesForChart'),
    core.indexOf('function crosshairAnchorId'),
  );
  assert.ok(seriesFn.includes("getSeries('line_rsx')"));
  assert.ok(!seriesFn.includes('RsxScaleLines'));
});

console.log('rsx_scale_lines_test: ALL PASS');
