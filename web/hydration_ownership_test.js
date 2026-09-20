/**
 * HYDRATION-OWNERSHIP-1 — prefs-first slots, time-keyed plot merge, no redundant refetch.
 * Run: node web/hydration_ownership_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const { ColumnarStore } = require('./columnar-store.js');
const { DDRFactory } = require('./series-factory.js');

global.chartTime = (t) => Number(t);

function test(name, fn) {
  const ret = fn();
  if (ret && typeof ret.then === 'function') {
    return ret.then(() => console.log('OK', name));
  }
  console.log('OK', name);
  return Promise.resolve();
}

function fakeLine(events, id) {
  return {
    setData(points) { events.push({ op: 'setData', id, points }); },
    update(pt) { events.push({ op: 'update', id, pt }); },
    applyOptions(opts) { events.push({ op: 'visible', id, visible: opts.visible }); },
    priceScale() { return { applyOptions() {} }; },
  };
}

function closedStore() {
  const store = new ColumnarStore();
  store.applyProjection({
    times: [10, 20, 30],
    candles: {
      open: [1, 2, 3], high: [1, 2, 3], low: [1, 2, 3], close: [1, 2, 3], volume: [1, 1, 1],
    },
    plots: {
      woz_vol_rsi_ema12: [10, 20, 30],
      woz_vol_rsi_ema5: [11, 21, 31],
      woz_rsi_close: [12, 22, 32],
      line_rsx: [40, 50, 60],
    },
  }, { commitPaired: true });
  return store;
}

async function run() {
  await test('A. loadDashboard mounts DDR before the authoritative history fetch', () => {
    const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
    const loadStart = boot.indexOf('async function loadDashboard');
    assert.ok(loadStart >= 0);
    const load = boot.slice(loadStart, boot.indexOf('window.seekHistoryIsland'));
    const firstMount = load.indexOf('await mountDDRLiveCutover()');
    const firstHist = load.indexOf('API.fetchColumnarHistory');
    assert.ok(firstMount >= 0 && firstHist >= 0);
    assert.ok(firstMount < firstHist, 'prefs/series must resolve before first /api/history');
    const firstWs = load.indexOf('wsSubscribeTf(window.currentTf)');
    assert.ok(firstWs > firstMount, 'WS slots follow resolved requestedPlotIds');
    assert.ok(/updatePlots\(plots, times\)/.test(boot), 'enable merge passes response times');
  });

  await test('B. right-aligned plot window merges by OpenTime, not index 0', () => {
    const store = closedStore();
    store.updatePlots(
      { woz_vol_rsi_ema12: [20, 30, 40] },
      [20, 30, 40],
    );
    const col = store.snapshot().plots.woz_vol_rsi_ema12;
    assert.strictEqual(col[0], ColumnarStore.plotAbsent());
    assert.strictEqual(col[1], 20);
    assert.strictEqual(col[2], 30);
    assert.ok(store.invariantOk());
  });

  await test('B2. plots without times do not positional-paste over existing columns', () => {
    const store = closedStore();
    store.updatePlots({ woz_vol_rsi_ema12: [1, 2, 3, 4] });
    assert.deepStrictEqual(store.snapshot().plots.woz_vol_rsi_ema12, [10, 20, 30]);
  });

  await test('C. visibility does not refetch already-hydrated plots (any id)', async () => {
    const events = [];
    let fetches = 0;
    const plots = {
      woz_rsi_close_ema7: [7, 8],
      woz_vol_rsi_ema5: [11, 21],
      line_rsx: [40, 50],
    };
    const ids = ['woz_rsi_close_ema7', 'woz_vol_rsi_ema5', 'line_rsx'];
    let i = 0;
    const factory = new DDRFactory({
      getColumnarSnapshot: () => ({ times: [10, 20], plots }),
      fetchPlotColumns: async () => {
        fetches += 1;
        throw new Error('must not refetch hydrated columns');
      },
    });
    factory.buildPanes(
      {
        wozduh: { chart: { addLineSeries() { return fakeLine(events, ids[i++]); } } },
        rsx: { chart: { addLineSeries() { return fakeLine(events, ids[i++]); } } },
      },
      {
        pane_osc: [
          { id: 'woz_rsi_close_ema7', hostId: 'wozduh', kind: 'line', renderOptions: { defaultVisible: false } },
          { id: 'woz_vol_rsi_ema5', hostId: 'wozduh', kind: 'line', renderOptions: { defaultVisible: true } },
        ],
        pane_rsx: [
          { id: 'line_rsx', hostId: 'rsx', kind: 'line', renderOptions: { defaultVisible: true } },
        ],
      },
    );
    await factory.setSeriesVisible('woz_rsi_close_ema7', true);
    await factory.setSeriesVisible('woz_vol_rsi_ema5', true);
    await factory.setSeriesVisible('line_rsx', true);
    assert.strictEqual(fetches, 0);
    const ema7 = events.filter((e) => e.op === 'setData' && e.id === 'woz_rsi_close_ema7');
    assert.strictEqual(ema7.length, 1);
    assert.deepStrictEqual(ema7[0].points.map((p) => p.value), [7, 8]);
  });

  await test('D. enable path is plotsReady, not CROSSHAIR_ANCHORS', () => {
    const src = fs.readFileSync(path.join(__dirname, 'series-factory.js'), 'utf8');
    const visStart = src.indexOf('setSeriesVisible(id, visible)');
    const vis = src.slice(visStart, src.indexOf('async _enableRenderComponent'));
    assert.ok(vis.includes('_plotsReadyForRender'));
    assert.ok(!src.includes('CROSSHAIR_ANCHORS'));
    assert.ok(!src.includes('_CROSSHAIR_ANCHORS'));
  });

  await test('E. boot still uses requestedPlotIds for the history slots list', () => {
    const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
    assert.ok(boot.includes('slots: resolveLiveSlotIds()'));
    assert.ok(boot.includes('DDRFactory.requestedPlotIds'));
  });
}

run().then(() => console.log('hydration_ownership_test.js passed')).catch((err) => {
  console.error(err);
  process.exit(1);
});
