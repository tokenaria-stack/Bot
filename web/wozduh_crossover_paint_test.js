/**
 * WOZDUH-CROSSOVER-PAINT-1
 * Run: node web/wozduh_crossover_paint_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
global.chartTime = (t) => Number(t);
require('./ui/scale-contribution.js');
const WozduhCrossoverPrefs = require('./wozduh-crossover-prefs.js');
const WozduhCrossovers = require('./wozduh-crossovers.js');
const { DDRFactory } = require('./series-factory.js');
const { ColumnarStore } = require('./columnar-store.js');
const { SettingsRenderer } = require('./ui/settings-renderer.js');

function test(name, fn) {
  const ret = fn();
  if (ret && typeof ret.then === 'function') {
    return ret.then(() => console.log('OK', name));
  }
  console.log('OK', name);
  return Promise.resolve();
}

function memStorage() {
  const map = Object.create(null);
  return {
    getItem(k) { return Object.prototype.hasOwnProperty.call(map, k) ? map[k] : null; },
    setItem(k, v) { map[k] = String(v); },
    removeItem(k) { delete map[k]; },
    dump() { return { ...map }; },
  };
}

function fakeChart() {
  const created = [];
  const chart = {
    created,
    addLineSeries(opts) {
      const series = {
        opts,
        primitive: null,
        data: null,
        attachPrimitive(p) {
          this.primitive = p;
          if (p && typeof p.attached === 'function') p.attached({ chart, series, requestUpdate() {} });
        },
        detachPrimitive() { this.primitive = null; },
        remove() {},
        setData(d) { this.data = d; },
        priceToCoordinate(price) { return Number(price); },
      };
      created.push(series);
      return series;
    },
    timeScale() {
      return { timeToCoordinate(t) { return Number(t); } };
    },
  };
  return chart;
}

async function run() {
  await test('A. four pair identities match Go names; no VolCross revival', () => {
    const ids = WozduhCrossoverPrefs.PAIR_IDS.slice();
    assert.deepStrictEqual(ids, [
      'woz_vol_rsi_ema12_x_ema5',
      'woz_rsi_hl2_vwema_x_ema5',
      'woz_rsi_hl2_vwema_x_ema12',
      'woz_rsi_hl2_vwema_x_ema5_chan_mid',
    ]);
    const go = fs.readFileSync(path.join(__dirname, '../core/nodes/wozduh_crossover.go'), 'utf8');
    for (const id of ids) assert.ok(go.includes(id), id);
    const layout = fs.readFileSync(path.join(__dirname, '../ui_config/wozduh_layout.go'), 'utf8');
    assert.ok(!layout.includes('woz_vol_cross'));
    const mask = fs.readFileSync(path.join(__dirname, '../core/nodes/wozduh_mask.go'), 'utf8');
    assert.ok(!mask.includes('woz_vol_cross'));
    assert.ok(!go.includes('VolCross'));
    assert.ok(!go.includes('wt11'));
    assert.ok(!go.includes('Fast'));
  });

  await test('B. sparse prefs + per-pair shape persistence', () => {
    WozduhCrossoverPrefs.setStorage(memStorage());
    WozduhCrossoverPrefs.resetAll();
    assert.strictEqual(WozduhCrossoverPrefs.loadMap()['woz_vol_rsi_ema12_x_ema5'], undefined);
    WozduhCrossoverPrefs.patch('woz_rsi_hl2_vwema_x_ema5', { visible: true, shape: 'star4', size: 12 });
    const stored = JSON.parse(WozduhCrossoverPrefs.setStorage && memStorage() ? WozduhCrossoverPrefs.loadMap() && JSON.stringify(WozduhCrossoverPrefs.loadMap()) : '{}');
    const row = WozduhCrossoverPrefs.loadMap()['woz_rsi_hl2_vwema_x_ema5'];
    assert.strictEqual(row.visible, true);
    assert.strictEqual(row.shape, 'star4');
    assert.strictEqual(row.size, 12);
    assert.ok(!row.upFill);
    const resolved = WozduhCrossoverPrefs.resolved('woz_rsi_hl2_vwema_x_ema5');
    assert.strictEqual(resolved.shape, 'star4');
    assert.strictEqual(resolved.upFill, '#00E676');
    void stored;
  });

  await test('C. demand includes source plots when enabled, even if lines hidden', () => {
    WozduhCrossoverPrefs.setStorage(memStorage());
    WozduhCrossoverPrefs.resetAll();
    for (const id of WozduhCrossoverPrefs.PAIR_IDS) {
      WozduhCrossoverPrefs.patch(id, { visible: false });
    }
    const factory = new DDRFactory();
    factory.buildPanes(
      { wozduh: { chart: { addLineSeries() { return { setData() {}, applyOptions() {}, priceScale() { return { applyOptions() {} }; } }; } } } },
      { pane_osc: [{ id: 'woz_vol_rsi_ema5', hostId: 'wozduh', kind: 'line', renderOptions: { defaultVisible: false } }] },
    );
    factory.setSeriesVisible('woz_vol_rsi_ema5', false);
    assert.ok(!factory.requestedPlotIds().includes('woz_vol_rsi_ema5'));
    WozduhCrossoverPrefs.patch('woz_rsi_hl2_vwema_x_ema5_chan_mid', { visible: true });
    const ids = factory.requestedPlotIds();
    assert.ok(ids.includes('woz_rsi_hl2_vwema'));
    assert.ok(ids.includes('woz_vol_rsi_ema5_chan_mid'));
    assert.ok(!ids.includes('woz_vol_rsi_ema5'));
    assert.ok(!factory.requestedPlotIds().includes('CROSSHAIR_ANCHORS'));
    assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS, undefined);
  });

  await test('D. overlay ignore autoscale; z-order top; star4 path exists', () => {
    WozduhCrossovers._resetForTests();
    const opts = WozduhCrossovers._hostSeriesOptionsForTests();
    assert.strictEqual(typeof opts.autoscaleInfoProvider, 'function');
    assert.strictEqual(opts.autoscaleInfoProvider(), null);
    assert.strictEqual(opts.lineVisible, false);
    const chart = fakeChart();
    assert.ok(WozduhCrossovers.attach(chart));
    const prim = chart.created[0].primitive;
    assert.strictEqual(prim.paneViews()[0].zOrder(), 'top');
    const bands = fs.readFileSync(path.join(__dirname, 'wozduh-extreme-bands.js'), 'utf8');
    assert.ok(bands.includes("return 'bottom'"));
    const src = fs.readFileSync(path.join(__dirname, 'wozduh-crossovers.js'), 'utf8');
    assert.ok(src.includes('function pathStar4'));
    assert.ok(src.includes('i % 2 === 0'));
    assert.ok(typeof WozduhCrossovers.pathStar4 === 'function');
    const ops = [];
    const ctx = {
      beginPath() { ops.push('begin'); },
      moveTo() { ops.push('move'); },
      lineTo() { ops.push('line'); },
      closePath() { ops.push('close'); },
      arc() { ops.push('arc'); },
      rect() { ops.push('rect'); },
    };
    WozduhCrossovers.pathStar4(ctx, 0, 0, 8);
    assert.ok(ops.includes('begin') && ops.includes('close'));
    assert.ok(ops.filter((x) => x === 'line').length >= 7);
    assert.ok(!ops.includes('arc'));
    const layout = fs.readFileSync(path.join(__dirname, '../ui_config/wozduh_layout.go'), 'utf8');
    assert.ok(!layout.includes('WozduhCrossovers'));
    assert.ok(!factoryHasCrossoverDdr());
  });

  await test('E. store keeps events; overlay not DDR series; compositor owns paint', () => {
    const store = new ColumnarStore();
    store.applyProjection({
      times: [10, 20],
      candles: { open: [1, 1], high: [1, 1], low: [1, 1], close: [1, 1], volume: [1, 1] },
      plots: {},
      wozduhCrossovers: [
        { pair: 'woz_vol_rsi_ema12_x_ema5', side: 'up', time: 20, y: 55 },
      ],
    });
    assert.strictEqual(store.wozduhCrossovers().length, 1);
    assert.strictEqual(store.wozduhCrossovers()[0].y, 55);
    store.appendTick({
      time: 30, open: 1, high: 1, low: 1, close: 1, volume: 1, isClosed: true,
      wozduhCrossovers: [{ pair: 'woz_vol_rsi_ema12_x_ema5', side: 'down', time: 30, y: 40 }],
    });
    assert.strictEqual(store.wozduhCrossovers().length, 2);
    store.appendTick({
      time: 30, open: 1, high: 1, low: 1, close: 1, volume: 1, isClosed: false,
      wozduhCrossovers: [{ pair: 'woz_vol_rsi_ema12_x_ema5', side: 'up', time: 30, y: 99 }],
    });
    assert.strictEqual(store.wozduhCrossovers().length, 2);
    const snap = store.snapshot();
    assert.ok(Array.isArray(snap.wozduhCrossovers));
    const compositor = fs.readFileSync(path.join(__dirname, 'chart-compositor.js'), 'utf8');
    assert.ok(compositor.includes('_applyWozduhCrossovers'));
    const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
    assert.ok(core.includes('WozduhCrossovers.attach'));
    assert.ok(core.includes('remountWozduhCrossovers'));
    const remount = core.slice(
      core.indexOf('remountWozduhCrossovers()'),
      core.indexOf('applyFullData'),
    );
    assert.ok(remount.includes('_liveUpdating = true'));
    assert.ok(remount.includes('finally'));
    assert.ok(core.indexOf('WozduhExtremeBands.attach') < core.indexOf('WozduhCrossovers.attach'));
    const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
    assert.ok(boot.indexOf('buildPanes') < boot.indexOf('remountWozduhCrossovers'));
  });

  await test('F. menu compact row + settings; no channel-boundary revival', () => {
    WozduhCrossoverPrefs.setStorage(memStorage());
    WozduhCrossoverPrefs.resetAll();
    const menu = {
      children: [],
      replaceChildren() { this.children = []; },
      appendChild(c) { this.children.push(c); return c; },
    };
    const origCreate = global.document && global.document.createElement;
    function el(tag) {
      const node = {
        tagName: String(tag).toUpperCase(),
        children: [],
        className: '',
        dataset: {},
        appendChild(c) { this.children.push(c); return c; },
        addEventListener() {},
        setAttribute() {},
      };
      return node;
    }
    global.document = { createElement: el };
    SettingsRenderer.rebuildMenu(menu, [], {});
    const src = fs.readFileSync(path.join(__dirname, 'ui/settings-renderer.js'), 'utf8');
    assert.ok(src.includes('appendCrossoverSection'));
    assert.ok(src.includes('wozduh-xover-gear'));
    assert.ok(src.includes("'⚙'"));
    assert.ok(!src.includes('<summary'));
    assert.ok(!src.includes("createElement('details')"));
    assert.ok(!/textContent = 'Settings'/.test(src));
    assert.ok(src.includes('Four-pointed star'));
    assert.ok(!src.includes('channel boundary'));
    assert.ok(src.includes('Outline style'));
    if (origCreate) global.document.createElement = origCreate;
  });
}

function factoryHasCrossoverDdr() {
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return { setData() {}, applyOptions() {}, priceScale() { return { applyOptions() {} }; } }; } } } },
    { pane_osc: [{ id: 'woz_vol_rsi_ema12_x_ema5', hostId: 'wozduh', kind: 'crossover', renderOptions: {} }] },
  );
  return factory.seriesMap.has('woz_vol_rsi_ema12_x_ema5');
}

run().then(() => console.log('wozduh_crossover_paint_test.js passed')).catch((err) => {
  console.error(err);
  process.exit(1);
});
