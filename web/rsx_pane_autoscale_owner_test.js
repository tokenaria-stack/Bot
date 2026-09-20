/**
 * RSX-PANE-AUTOSCALE-OWNER-1
 * Pane Auto domain is the RsxScaleLines host, not hideable line_rsx.
 * Run: node web/rsx_pane_autoscale_owner_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
require('./ui/scale-contribution.js');
const RsxScaleLines = require('./rsx-scale-lines.js');
const WozduhExtremeBands = require('./wozduh-extreme-bands.js');
const { DDRFactory } = require('./series-factory.js');
const ScaleController = require('./ui/scale-controller.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function memoryStorage() {
  const map = new Map();
  return {
    getItem(k) { return map.has(k) ? map.get(k) : null; },
    setItem(k, v) { map.set(k, String(v)); },
    removeItem(k) { map.delete(k); },
  };
}

function fakeRsxChart() {
  let auto = true;
  const created = [];
  const chart = {
    created,
    calls: [],
    addLineSeries(opts) {
      const series = {
        opts,
        visible: opts && opts.visible !== false,
        primitive: null,
        removed: false,
        data: null,
        attachPrimitive(p) {
          this.primitive = p;
          if (p && typeof p.attached === 'function') p.attached({ chart, series });
        },
        detachPrimitive() { this.primitive = null; },
        remove() { this.removed = true; },
        setData(d) { this.data = d; },
        applyOptions(patch) {
          if (patch && Object.prototype.hasOwnProperty.call(patch, 'visible')) {
            this.visible = patch.visible !== false;
          }
        },
        priceScale() { return { applyOptions() {} }; },
      };
      created.push(series);
      return series;
    },
    setCrosshairPosition(price, time, series) {
      chart.calls.push({ price, time, series });
    },
    priceScale() {
      return {
        options: () => ({ autoScale: auto }),
        applyOptions: (opts) => {
          if (Object.prototype.hasOwnProperty.call(opts, 'autoScale')) auto = !!opts.autoScale;
        },
        width: () => 64,
        _auto: () => auto,
      };
    },
  };
  return chart;
}

const DOMAIN = { priceRange: { minValue: -5, maxValue: 105 } };

test('A. line_rsx DDR is ignore; host contributes bounded [-5,105]', () => {
  const layout = fs.readFileSync(path.join(__dirname, '../ui_config/rsx_layout.go'), 'utf8');
  assert.ok(layout.includes('"scaleContribution":{"type":"ignore"}'));
  assert.ok(!layout.includes('"type":"bounded"'));
  const opts = RsxScaleLines._hostSeriesOptionsForTests();
  assert.deepStrictEqual(opts.autoscaleInfoProvider(), DOMAIN);
  assert.deepStrictEqual(RsxScaleLines.PANE_DOMAIN, { type: 'bounded', min: -5, max: 105 });
  assert.deepStrictEqual(WozduhExtremeBands.PANE_DOMAIN, { type: 'bounded', min: -5, max: 105 });
});

test('C. compositor refreshes RSX chrome after DDR setData, not on ticks', () => {
  const compositor = fs.readFileSync(path.join(__dirname, 'chart-compositor.js'), 'utf8');
  const full = compositor.slice(
    compositor.indexOf('  _flushFull(storeData, snapshot, intent) {'),
    compositor.indexOf('  _flushPrepend(storeData, snapshot, intent) {'),
  );
  const ddrAt = full.indexOf('this._applyDdrPlots(snapshot)');
  const chromeAt = full.indexOf('ChartAdapter.refreshOscillatorPaneChrome()');
  assert.ok(ddrAt >= 0 && chromeAt > ddrAt);
  const delta = compositor.slice(
    compositor.indexOf('  _flushDelta(intent) {'),
    compositor.indexOf('  _flushFull(storeData, snapshot, intent) {'),
  );
  assert.ok(!delta.includes('refreshOscillatorPaneChrome'));
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  const update = core.slice(core.indexOf('function updateCandle'), core.indexOf('const ChartAdapter'));
  assert.ok(!update.includes('RsxScaleLines.refresh'));
  const helper = core.slice(
    core.indexOf('function refreshOscillatorPaneChrome'),
    core.indexOf('function isOlderThanPaintedTip'),
  );
  assert.ok(helper.includes('RsxScaleLines.refresh'));
  assert.ok(!helper.includes('WozduhExtremeBands'));
});

test('D. post-DDR refresh leaves a host point so LWC firstValue is non-null', () => {
  RsxScaleLines._resetForTests();
  const chart = fakeRsxChart();
  const factory = new DDRFactory();
  factory.buildPanes(
    { rsx: { chart: { addLineSeries() { return chart.addLineSeries({}); } } } },
    { pane_osc: [{ id: 'line_rsx', hostId: 'rsx', kind: 'line', renderOptions: { scaleContribution: { type: 'ignore' } } }] },
  );
  const ddr = chart.created[0];
  ddr.setData([{ time: 10, value: 44 }, { time: 20, value: 91 }, { time: 30, value: 70 }]);
  RsxScaleLines.attach(chart);
  const host = chart.created[chart.created.length - 1];
  assert.strictEqual(host.data, null);
  RsxScaleLines.refresh(30);
  assert.deepStrictEqual(host.data, [{ time: 30, value: 50 }]);
  const visibleLeft = 10;
  const hit = host.data.find((p) => Number(p.time) >= visibleLeft);
  assert.ok(hit);
  assert.strictEqual(hit.value, 50);
  RsxScaleLines.dispose();
});

test('K. full flush (TF reload) keeps DDR then chrome order; no primitive autoscaleInfo', () => {
  const compositor = fs.readFileSync(path.join(__dirname, 'chart-compositor.js'), 'utf8');
  const src = fs.readFileSync(path.join(__dirname, 'rsx-scale-lines.js'), 'utf8');
  assert.ok(compositor.includes('_flushFull'));
  assert.ok(!src.includes('autoscaleInfo()'));
  const Prim = RsxScaleLines._RsxScaleLinesPrimitive;
  assert.strictEqual(typeof new Prim().autoscaleInfo, 'undefined');
});

test('E/F/G. hide/show line_rsx does not change host domain or require plot setData', () => {
  RsxScaleLines._resetForTests();
  const chart = fakeRsxChart();
  const events = [];
  const factory = new DDRFactory();
  factory.buildPanes(
    {
      rsx: {
        chart: {
          addLineSeries(opts) {
            const s = chart.addLineSeries(opts);
            const orig = s.setData.bind(s);
            s.setData = (d) => { events.push({ op: 'setData', d }); orig(d); };
            return s;
          },
        },
      },
    },
    {
      pane_osc: [{
        id: 'line_rsx',
        hostId: 'rsx',
        kind: 'line',
        renderOptions: { defaultVisible: true, scaleContribution: { type: 'ignore' } },
      }],
    },
  );
  const ddr = chart.created[0];
  assert.ok(RsxScaleLines.attach(chart));
  const host = chart.created[chart.created.length - 1];
  assert.notStrictEqual(host, ddr);
  assert.deepStrictEqual(host.opts.autoscaleInfoProvider(), DOMAIN);
  const ddrProv = ddr.opts.autoscaleInfoProvider;
  assert.ok(typeof ddrProv !== 'function' || ddrProv() == null);

  factory.setSeriesVisible('line_rsx', true);
  assert.deepStrictEqual(host.opts.autoscaleInfoProvider(), DOMAIN);
  factory.setSeriesVisible('line_rsx', false);
  assert.strictEqual(ddr.visible, false);
  assert.strictEqual(host.removed, false);
  assert.deepStrictEqual(host.opts.autoscaleInfoProvider(), DOMAIN);
  factory.setSeriesVisible('line_rsx', true);
  assert.deepStrictEqual(host.opts.autoscaleInfoProvider(), DOMAIN);
  assert.ok(!events.some((e) => e.op === 'setData'));
  RsxScaleLines.dispose();
});

test('F. Auto does not require line_rsx hydration', () => {
  const factory = fs.readFileSync(path.join(__dirname, 'series-factory.js'), 'utf8');
  const host = fs.readFileSync(path.join(__dirname, 'rsx-scale-lines.js'), 'utf8');
  assert.ok(!host.includes('getHydratedSeries'));
  assert.ok(!host.includes('line_rsx'));
  assert.ok(!factory.includes('RsxScaleLines'));
});

test('G. guides remain 30/50/70', () => {
  assert.deepStrictEqual(RsxScaleLines.LEVELS, { low: 30, mid: 50, high: 70 });
  assert.strictEqual(RsxScaleLines.HOST_VALUE, 50);
});

test('H. crosshair socket unchanged; no public host getter', () => {
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  assert.ok(core.includes('RsxScaleLines.applyCrosshairTime'));
  assert.ok(!core.includes('function crosshairAnchorId'));
  RsxScaleLines._resetForTests();
  const chart = fakeRsxChart();
  RsxScaleLines.attach(chart);
  RsxScaleLines.refresh(99);
  assert.strictEqual(RsxScaleLines.applyCrosshairTime(chart, 10, 50), true);
  assert.deepStrictEqual(chart.created[0].data, [{ time: 99, value: 50 }]);
  const keys = Object.keys(RsxScaleLines).filter((k) => !k.startsWith('_'));
  assert.ok(!keys.includes('getSeries'));
  RsxScaleLines.dispose();
});

test('I. Wozduh Auto host unchanged', () => {
  const woz = WozduhExtremeBands._hostSeriesOptionsForTests();
  assert.deepStrictEqual(woz.autoscaleInfoProvider(), DOMAIN);
  const layout = fs.readFileSync(path.join(__dirname, '../ui_config/wozduh_layout.go'), 'utf8');
  assert.ok(layout.includes('wozLine("woz_vol_rsi_ema5", core.SlotWozduhVolRsiEma5, scaleIgnore'));
});

test('J. ScaleController stays generic; markers still on line_rsx', () => {
  const sc = fs.readFileSync(path.join(__dirname, 'ui/scale-controller.js'), 'utf8');
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  const compositor = fs.readFileSync(path.join(__dirname, 'chart-compositor.js'), 'utf8');
  assert.ok(!sc.includes('line_rsx'));
  assert.ok(!sc.includes('RsxScaleLines'));
  assert.ok(!sc.includes('setVisibleRange'));
  assert.ok(!sc.includes("hostId === 'rsx'"));
  assert.ok(core.includes("getSeries('line_rsx')"));
  assert.ok(core.includes('refreshOscillatorPaneChrome'));
  assert.ok(compositor.includes('refreshOscillatorPaneChrome'));
  assert.ok(compositor.includes("getSeries('line_rsx')"));
});

test('ScaleController Auto toggle still commands the RSX chart, not the plot', () => {
  RsxScaleLines._resetForTests();
  const chart = fakeRsxChart();
  RsxScaleLines.attach(chart);
  const store = memoryStorage();
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  ScaleController.register({
    context: 'live',
    hostId: 'rsx',
    chart,
    allowLog: false,
  });
  ScaleController.setPanePrefs('rsx', { isAuto: false });
  assert.strictEqual(chart.priceScale()._auto(), false);
  ScaleController.toggleAuto('live', 'rsx');
  assert.strictEqual(chart.priceScale()._auto(), true);
  RsxScaleLines.dispose();
  ScaleController._resetForTests({ storage: memoryStorage() });
});

console.log('rsx_pane_autoscale_owner_test: ALL PASS');
