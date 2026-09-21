/**
 * CROSSHAIR-PANE-HOST-1 — oscillator crosshair is pane chrome, not DDR plots.
 * Run: node web/crosshair_pane_host_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const { DDRFactory } = require('./series-factory.js');
const WozduhCrossoverPrefs = require('./wozduh-crossover-prefs.js');
WozduhCrossoverPrefs.disableAllForTests();

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function fakeLine(events, id) {
  return {
    setData(p) { events.push({ id, p }); },
    update() {},
    applyOptions() {},
    priceScale() { return { applyOptions() {} }; },
  };
}

test('A. no CROSSHAIR_ANCHORS; hidden ema5/line_rsx skip hydrate', () => {
  assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS, undefined);
  const factory = new DDRFactory();
  const events = [];
  factory.buildPanes(
    {
      wozduh: { chart: { addLineSeries() { return fakeLine(events, 'ema5'); } } },
      rsx: { chart: { addLineSeries() { return fakeLine(events, 'rsx'); } } },
    },
    {
      pane_osc: [{ id: 'woz_vol_rsi_ema5', hostId: 'wozduh', kind: 'line', renderOptions: { defaultVisible: false } }],
      pane_rsx: [{ id: 'line_rsx', hostId: 'rsx', kind: 'line', renderOptions: { defaultVisible: false } }],
    },
  );
  factory.setSeriesVisible('woz_vol_rsi_ema5', false);
  factory.setSeriesVisible('line_rsx', false);
  factory.hydrateFromColumnar({
    times: [1],
    plots: { woz_vol_rsi_ema5: [40], line_rsx: [55] },
  });
  factory.applyHydratedData();
  assert.strictEqual(events.length, 0);
  assert.ok(!factory.requestedPlotIds().includes('woz_vol_rsi_ema5'));
  assert.ok(!factory.requestedPlotIds().includes('line_rsx'));
});

test('B. ChartAdapter routes osc panes to chrome applyCrosshairTime', () => {
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  const paint = core.slice(
    core.indexOf('function paintNativeCrosshairAtTime'),
    core.indexOf('function applyBottomAxisLabel'),
  );
  assert.ok(paint.includes('WozduhExtremeBands.applyCrosshairTime'));
  assert.ok(paint.includes('RsxScaleLines.applyCrosshairTime'));
  assert.ok(!paint.includes("getSeries('woz_vol_rsi_ema5')"));
  assert.ok(!paint.includes("getSeries('line_rsx')"));
  assert.ok(!paint.includes('hydratedValueAtTime'));
  assert.ok(core.includes('candleSeries'));
  assert.ok(!core.includes('function hydratedValueAtTime'));
  assert.ok(core.includes('applyPeerCrosshair'));
  assert.ok(core.includes('peer-crosshair-guide'));
});

test('C. chrome sockets stay private (no host getter)', () => {
  const woz = fs.readFileSync(path.join(__dirname, 'wozduh-extreme-bands.js'), 'utf8');
  const rsx = fs.readFileSync(path.join(__dirname, 'rsx-scale-lines.js'), 'utf8');
  assert.ok(woz.includes('function applyCrosshairTime'));
  assert.ok(rsx.includes('function applyCrosshairTime'));
  assert.ok(!/\bgetSeries\s*[:(]/.test(woz));
  assert.ok(!/\bgetSeries\s*[:(]/.test(rsx));
  const wozKeys = Object.keys(require('./wozduh-extreme-bands.js'));
  const rsxKeys = Object.keys(require('./rsx-scale-lines.js'));
  assert.ok(!wozKeys.includes('getSeries'));
  assert.ok(!rsxKeys.includes('getSeries'));
  assert.ok(wozKeys.includes('applyCrosshairTime'));
  assert.ok(rsxKeys.includes('applyCrosshairTime'));
});

test('D. hydration files not modified by this chapter', () => {
  const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
  const store = fs.readFileSync(path.join(__dirname, 'columnar-store.js'), 'utf8');
  assert.ok(boot.includes('updatePlots(plots, times)'));
  assert.ok(store.includes('indexByTime'));
});

test('E. hiding osc plots does not call fetchPlotColumns', () => {
  const fetched = [];
  const factory = new DDRFactory({
    fetchPlotColumns: async (ids) => {
      fetched.push(...ids);
      return { times: [1], plots: {} };
    },
  });
  factory.buildPanes(
    {
      wozduh: { chart: { addLineSeries() { return fakeLine([], 'ema5'); } } },
      rsx: { chart: { addLineSeries() { return fakeLine([], 'rsx'); } } },
    },
    {
      pane_osc: [{ id: 'woz_vol_rsi_ema5', hostId: 'wozduh', kind: 'line', renderOptions: { defaultVisible: true } }],
      pane_rsx: [{ id: 'line_rsx', hostId: 'rsx', kind: 'line', renderOptions: { defaultVisible: true } }],
    },
  );
  factory.setSeriesVisible('woz_vol_rsi_ema5', false);
  factory.setSeriesVisible('line_rsx', false);
  assert.deepStrictEqual(fetched, []);
  assert.ok(!factory.requestedPlotIds().includes('woz_vol_rsi_ema5'));
  assert.ok(!factory.requestedPlotIds().includes('line_rsx'));
});

console.log('crosshair_pane_host_test: ALL PASS');
