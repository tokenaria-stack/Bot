/**
 * WOZDUH-SCALE-1 — urvol extreme bands as pane chrome (not DDR).
 * Run: node web/wozduh_extreme_bands_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const WozduhExtremeBands = require('./wozduh-extreme-bands.js');
const { DDRFactory } = require('./series-factory.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function fakeWozChart() {
  const created = [];
  return {
    created,
    addLineSeries(opts) {
      const series = {
        opts,
        primitive: null,
        removed: false,
        data: null,
        attachPrimitive(p) { this.primitive = p; if (p && typeof p.attached === 'function') p.attached({ chart: this, series }); },
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
  const layout = fs.readFileSync(path.join(__dirname, '../ui_config/wozduh_layout.go'), 'utf8');
  assert.ok(!layout.includes('__wozduh_extreme_bands__'));
  assert.ok(!layout.includes('WozduhExtremeBands'));
  const settings = fs.readFileSync(path.join(__dirname, 'ui/settings-renderer.js'), 'utf8');
  assert.ok(!settings.includes('extreme'));
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return { setData() {}, applyOptions() {}, priceScale() { return { applyOptions() {} }; } }; } } } },
    { pane_osc: [{ id: 'woz_slow', hostId: 'wozduh', kind: 'line', renderOptions: {} }] },
  );
  assert.ok(!factory.seriesMap.has('__wozduh_extreme_bands__'));
  assert.ok(!factory.requestedPlotIds().includes('__wozduh_extreme_bands__'));
  const keys = Object.keys(WozduhExtremeBands).filter((k) => !k.startsWith('_'));
  assert.ok(keys.includes('attach'));
  assert.ok(keys.includes('refresh'));
  assert.ok(keys.includes('dispose'));
  assert.ok(!keys.includes('getSeries'));
  assert.ok(!keys.includes('setData'));
  assert.ok(!keys.includes('update'));
});

test('B. private host autoscale is null; woz_slow stays bounded owner in layout', () => {
  const opts = WozduhExtremeBands._hostSeriesOptionsForTests();
  assert.strictEqual(typeof opts.autoscaleInfoProvider, 'function');
  assert.strictEqual(opts.autoscaleInfoProvider(), null);
  assert.strictEqual(opts.lineVisible, false);
  assert.strictEqual(opts.priceLineVisible, false);
  assert.strictEqual(opts.lastValueVisible, false);
  assert.strictEqual(opts.priceScaleId, 'right');
  const layout = fs.readFileSync(path.join(__dirname, '../ui_config/wozduh_layout.go'), 'utf8');
  assert.ok(layout.includes('wozLine("woz_slow", core.SlotWozduhSlow, scaleBoundedOsc'));
  assert.ok(layout.includes('wozLine("woz_fast", core.SlotWozduhFast, scaleIgnore'));
});

test('C. primitive constants match Pine urvol bands', () => {
  assert.deepStrictEqual(WozduhExtremeBands.LEVELS, {
    lowInner: 5,
    lowOuter: 8,
    highInner: 89,
    highOuter: 92,
  });
});

test('D. styles: inner solid, outer dotted, yellow ~20% fill', () => {
  assert.strictEqual(WozduhExtremeBands.FILL, 'rgba(255, 255, 0, 0.2)');
  const src = fs.readFileSync(path.join(__dirname, 'wozduh-extreme-bands.js'), 'utf8');
  assert.ok(src.includes('strokeH(ctx, w, yLi, false)'));
  assert.ok(src.includes('strokeH(ctx, w, yLo, true)'));
  assert.ok(src.includes('strokeH(ctx, w, yHi, false)'));
  assert.ok(src.includes('strokeH(ctx, w, yHo, true)'));
  assert.ok(src.includes('DOTTED_DASH'));
  assert.ok(!src.includes('createPriceLine'));
});

test('E. renderer is O(1): no store/history/bar walk', () => {
  const src = fs.readFileSync(path.join(__dirname, 'wozduh-extreme-bands.js'), 'utf8');
  const draw = src.slice(src.indexOf('draw(target)'), src.indexOf('fillBand'));
  assert.ok(!/ColumnarStore|hydratedData|requestedPlotIds|visibleRange|bars\.length/.test(draw));
  assert.ok(!/for\s*\(/.test(draw));
  assert.ok(src.includes('priceToCoordinate(LEVELS.lowInner)'));
});

test('F. lifecycle: one host + one primitive; dispose clears', () => {
  WozduhExtremeBands._resetForTests();
  const chart = fakeWozChart();
  assert.strictEqual(WozduhExtremeBands.attach(chart), true);
  assert.strictEqual(chart.created.length, 1);
  assert.ok(chart.created[0].primitive);
  assert.strictEqual(WozduhExtremeBands._attachmentCountForTests(), 1);
  assert.strictEqual(WozduhExtremeBands.attach(chart), true);
  assert.strictEqual(chart.created.length, 1);
  assert.strictEqual(WozduhExtremeBands.dispose(), true);
  assert.strictEqual(chart.created[0].removed, true);
  assert.strictEqual(chart.created[0].primitive, null);
  assert.strictEqual(WozduhExtremeBands._attachmentCountForTests(), 0);
  const chart2 = fakeWozChart();
  assert.strictEqual(WozduhExtremeBands.attach(chart2), true);
  assert.strictEqual(chart2.created.length, 1);
  WozduhExtremeBands.dispose();
});

test('refresh seeds one priced point (not whitespace-only)', () => {
  WozduhExtremeBands._resetForTests();
  const chart = fakeWozChart();
  WozduhExtremeBands.attach(chart);
  assert.strictEqual(WozduhExtremeBands.refresh(1700000000), true);
  assert.deepStrictEqual(chart.created[0].data, [{ time: 1700000000, value: 50 }]);
  assert.strictEqual(WozduhExtremeBands.HOST_VALUE, 50);
  assert.strictEqual(WozduhExtremeBands.refresh(null), false);
  WozduhExtremeBands.dispose();
});

test('paintCandles reseeds even when skipDecoration; updateCandle does not refresh bands', () => {
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  const paint = core.slice(core.indexOf('function paintCandles'), core.indexOf('function isOlderThanPaintedTip'));
  const update = core.slice(core.indexOf('function updateCandle'), core.indexOf('const ChartAdapter'));
  assert.ok(paint.includes('WozduhExtremeBands.refresh(state._lastRealCandleTime)'));
  assert.ok(paint.indexOf('skipDecoration') < paint.indexOf('WozduhExtremeBands.refresh'));
  assert.ok(!update.includes('WozduhExtremeBands.refresh'));
});

test('zOrder is bottom; chart-core wires Wozduh chart only', () => {
  const Prim = WozduhExtremeBands._WozduhExtremeBandsPrimitive;
  const p = new Prim();
  assert.strictEqual(p.paneViews()[0].zOrder(), 'bottom');
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  assert.ok(core.includes('WozduhExtremeBands.attach(wozduhChart)'));
  assert.ok(core.includes('WozduhExtremeBands.dispose()'));
  assert.ok(!core.includes('WozduhExtremeBands.attach(priceChart)'));
  assert.ok(!core.includes('WozduhExtremeBands.attach(rsxChart)'));
});

console.log('wozduh_extreme_bands_test: ALL PASS');
