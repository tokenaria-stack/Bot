/**
 * WOZDUH-WIRE-1 — subscribe/ship only requested Wozduh plot columns.
 * Run: node web/wozduh_wire_test.js
 */
'use strict';

global.chartTime = (t) => Number(t);

const assert = require('assert');
const { DDRFactory } = require('./series-factory.js');
const { ColumnarStore } = require('./columnar-store.js');

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

function fakeChannel(events, id) {
  return {
    setData(points) { events.push({ op: 'setData', id, points }); },
    update(pt) { events.push({ op: 'update', id, pt }); },
    applyOptions(opts) { events.push({ op: 'visible', id, visible: opts.visible }); },
    priceScale() { return { applyOptions() {} }; },
  };
}

function mountMixed(factory, events) {
  factory.buildPanes(
    {
      wozduh: {
        chart: {
          addLineSeries() { return fakeLine(events, 'line'); },
          addCustomSeries() { return fakeChannel(events, 'chan'); },
        },
      },
    },
    {
      pane_osc: [
        { id: 'woz_fast', hostId: 'wozduh', kind: 'line', renderOptions: { defaultVisible: true } },
        { id: 'woz_slow', hostId: 'wozduh', kind: 'line', renderOptions: { defaultVisible: true } },
        { id: 'woz_ema_rsi', hostId: 'wozduh', kind: 'line', renderOptions: { defaultVisible: false } },
        {
          id: 'woz_vol_chan',
          hostId: 'wozduh',
          kind: 'channel',
          renderOptions: {
            defaultVisible: false,
            plots: { upper: 'woz_vol_chan_up', mid: 'woz_vol_chan_mid', lower: 'woz_vol_chan_dn' },
          },
        },
      ],
    },
  );
}

async function run() {
  await test('A. hidden scalar omitted; woz_slow stays subscribed', () => {
    const factory = new DDRFactory();
    mountMixed(factory, []);
    factory.setSeriesVisible('woz_ema_rsi', false);
    factory.setSeriesVisible('woz_fast', true);
    const ids = factory.requestedPlotIds();
    assert.ok(ids.includes('woz_fast'));
    assert.ok(ids.includes('woz_slow'));
    assert.ok(!ids.includes('woz_ema_rsi'));
    assert.ok(!ids.includes('woz_vol_chan'));
    assert.ok(!ids.includes('woz_vol_chan_up'));
  });

  await test('B. visible channel expands to three scalar sources, not compose id', () => {
    const factory = new DDRFactory();
    mountMixed(factory, []);
    factory.setSeriesVisible('woz_vol_chan', true);
    const ids = factory.requestedPlotIds();
    assert.ok(ids.includes('woz_vol_chan_up'));
    assert.ok(ids.includes('woz_vol_chan_mid'));
    assert.ok(ids.includes('woz_vol_chan_dn'));
    assert.ok(!ids.includes('woz_vol_chan'));
  });

  await test('C. shared source is listed once', () => {
    const factory = new DDRFactory();
    factory.buildPanes(
      { wozduh: { chart: { addCustomSeries() { return fakeChannel([], 'chan'); } } } },
      {
        pane_osc: [{
          id: 'woz_vol_chan',
          hostId: 'wozduh',
          kind: 'channel',
          renderOptions: {
            defaultVisible: true,
            plots: { upper: 'woz_vol_chan_up', mid: 'woz_vol_chan_mid', lower: 'woz_vol_chan_dn' },
          },
        }],
      },
    );
    const ids = factory.requestedPlotIds();
    assert.strictEqual(ids.filter((x) => x === 'woz_vol_chan_mid').length, 1);
  });

  await test('D. woz_slow hidden still requested (pane owner)', () => {
    const factory = new DDRFactory();
    mountMixed(factory, []);
    factory.setSeriesVisible('woz_slow', false);
    assert.ok(factory.requestedPlotIds().includes('woz_slow'));
    assert.strictEqual(factory.needsLwcData('woz_slow'), true);
  });

  await test('F. omitted live key does not write zero into store', () => {
    const store = new ColumnarStore();
    store.applyProjection({
      times: [1],
      candles: { open: [1], high: [1], low: [1], close: [1], volume: [1] },
      plots: { woz_fast: [10], line_rsx: [50] },
    }, { commitPaired: true });
    store.appendTick({
      time: 1,
      open: 1, high: 1, low: 1, close: 1, volume: 1,
      plots: { line_rsx: 51 },
    });
    assert.strictEqual(store.snapshot().plots.woz_fast[0], 10);
    assert.strictEqual(store.snapshot().plots.line_rsx[0], 51);
  });

  await test('G. enable scalar fetches then one setData then reveal', async () => {
    const events = [];
    let fetched = null;
    const plots = { woz_ema_rsi: [1, 2, 3] };
    const factory = new DDRFactory({
      getColumnarSnapshot: () => ({ times: [1, 2, 3], plots }),
      fetchPlotColumns: async (ids) => {
        fetched = ids.slice();
        return { plots: { woz_ema_rsi: [7, 8, 9] } };
      },
      onMergePlots: (incoming) => {
        Object.assign(plots, incoming);
      },
    });
    factory.buildPanes(
      { wozduh: { chart: { addLineSeries() { return fakeLine(events, 'woz_ema_rsi'); } } } },
      { pane_osc: [{ id: 'woz_ema_rsi', hostId: 'wozduh', kind: 'line', renderOptions: { defaultVisible: false } }] },
    );
    await factory.setSeriesVisible('woz_ema_rsi', true);
    await new Promise((r) => setImmediate(r));
    assert.deepStrictEqual(fetched, ['woz_ema_rsi']);
    const sets = events.filter((e) => e.op === 'setData');
    assert.strictEqual(sets.length, 1);
    assert.deepStrictEqual(sets[0].points.map((p) => p.value), [7, 8, 9]);
    const visTrue = events.filter((e) => e.op === 'visible' && e.visible === true);
    assert.ok(visTrue.length >= 1);
    assert.ok(events.findIndex((e) => e.op === 'setData') < events.findIndex((e) => e.op === 'visible' && e.visible === true));
  });

  await test('H. enable channel hydrates three sources then one setData', async () => {
    const events = [];
    let fetched = null;
    const plots = {};
    const factory = new DDRFactory({
      getColumnarSnapshot: () => ({
        times: [1],
        plots,
      }),
      fetchPlotColumns: async (ids) => {
        fetched = ids.slice();
        return {
          plots: {
            woz_vol_chan_up: [90],
            woz_vol_chan_mid: [60],
            woz_vol_chan_dn: [30],
          },
        };
      },
      onMergePlots: (incoming) => Object.assign(plots, incoming),
    });
    factory.buildPanes(
      { wozduh: { chart: { addCustomSeries() { return fakeChannel(events, 'woz_vol_chan'); } } } },
      { pane_osc: [{
        id: 'woz_vol_chan',
        hostId: 'wozduh',
        kind: 'channel',
        renderOptions: {
          defaultVisible: false,
          plots: { upper: 'woz_vol_chan_up', mid: 'woz_vol_chan_mid', lower: 'woz_vol_chan_dn' },
        },
      }] },
    );
    await factory.setSeriesVisible('woz_vol_chan', true);
    await new Promise((r) => setImmediate(r));
    assert.deepStrictEqual(fetched.sort(), ['woz_vol_chan_dn', 'woz_vol_chan_mid', 'woz_vol_chan_up']);
    const sets = events.filter((e) => e.op === 'setData');
    assert.strictEqual(sets.length, 1);
    assert.deepStrictEqual(sets[0].points, [{ time: 1, upper: 90, mid: 60, lower: 30 }]);
  });

  await test('I. disable drops unsubscribed woz column; no RSX drop', () => {
    const store = new ColumnarStore();
    store.applyProjection({
      times: [1],
      candles: { open: [1], high: [1], low: [1], close: [1], volume: [1] },
      plots: { woz_fast: [10], woz_ema_rsi: [3], line_rsx: [50] },
    }, { commitPaired: true });
    store.dropPlotsNotIn(['woz_fast', 'line_rsx']);
    const snap = store.snapshot();
    assert.ok(snap.plots.woz_fast);
    assert.ok(snap.plots.line_rsx);
    assert.strictEqual(snap.plots.woz_ema_rsi, undefined);
  });

  await test('G2. enable fetch failure stays hidden (no stale reveal)', async () => {
    const events = [];
    const factory = new DDRFactory({
      getColumnarSnapshot: () => ({ times: [1], plots: { woz_ema_rsi: [99] } }),
      fetchPlotColumns: async () => {
        throw new Error('history down');
      },
    });
    factory.buildPanes(
      { wozduh: { chart: { addLineSeries() { return fakeLine(events, 'woz_ema_rsi'); } } } },
      { pane_osc: [{ id: 'woz_ema_rsi', hostId: 'wozduh', kind: 'line', renderOptions: { defaultVisible: false } }] },
    );
    await factory.setSeriesVisible('woz_ema_rsi', true);
    await new Promise((r) => setImmediate(r));
    assert.ok(!events.some((e) => e.op === 'setData'));
    assert.ok(!events.some((e) => e.op === 'visible' && e.visible === true));
  });
}

run().then(() => console.log('wozduh_wire_test.js passed')).catch((err) => {
  console.error(err);
  process.exit(1);
});
