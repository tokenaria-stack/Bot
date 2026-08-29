/**
 * HIDDEN-RENDER-SKIP-1 — DDR skips LWC data work for unchecked renderers.
 * Run: node web/hidden_render_skip_test.js
 */
'use strict';

const assert = require('assert');
const { DDRFactory } = require('./series-factory.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
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

test('A. hidden LineSeries: no extraction/setData/update', () => {
  const events = [];
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: {
      addLineSeries() { return fakeLine(events, 'woz_ema_rsi'); },
    } } },
    { pane_osc: [{
      id: 'woz_ema_rsi',
      hostId: 'wozduh',
      kind: 'line',
      renderOptions: { defaultVisible: false, scaleContribution: { type: 'ignore' } },
    }] },
  );
  factory.hydrateFromColumnar({
    times: [1, 2],
    plots: { woz_ema_rsi: [10, 11], woz_fast: [40, 41] },
  });
  factory.applyHydratedData();
  factory.updateTick(3, { woz_ema_rsi: 12, woz_fast: 42 });
  assert.strictEqual(factory.hydratedData.has('woz_ema_rsi'), false);
  assert.ok(!events.some((e) => e.op === 'setData'));
  assert.ok(!events.some((e) => e.op === 'update'));
});

test('B. visible LineSeries still setData/update', () => {
  const events = [];
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine(events, 'woz_slow'); } } } },
    { pane_osc: [{ id: 'woz_slow', hostId: 'wozduh', kind: 'line', renderOptions: {} }] },
  );
  factory.hydrateFromColumnar({ times: [1], plots: { woz_slow: [50] } });
  factory.applyHydratedData();
  factory.updateTick(2, { woz_slow: 51 });
  assert.ok(events.some((e) => e.op === 'setData'));
  assert.ok(events.some((e) => e.op === 'update' && e.pt.value === 51));
});

test('C. hidden ChannelSeries: no zip, no setData, no live compose', () => {
  const events = [];
  let zipCalls = 0;
  const orig = DDRFactory.zipChannelFromHydrated;
  DDRFactory.zipChannelFromHydrated = (...args) => {
    zipCalls += 1;
    return orig(...args);
  };
  try {
    const factory = new DDRFactory();
    factory.buildPanes(
      { wozduh: { chart: { addCustomSeries() { return fakeChannel(events, 'woz_price_chan'); } } } },
      { pane_osc: [{
        id: 'woz_price_chan',
        hostId: 'wozduh',
        kind: 'channel',
        renderOptions: {
          defaultVisible: false,
          scaleContribution: { type: 'ignore' },
          plots: { upper: 'woz_price_chan_up', mid: 'woz_price_chan_mid', lower: 'woz_price_chan_dn' },
        },
      }] },
    );
    factory.hydrateFromColumnar({
      times: [1, 2],
      plots: {
        woz_price_chan_up: [80, 81],
        woz_price_chan_mid: [50, 51],
        woz_price_chan_dn: [20, 21],
      },
    });
    factory.applyHydratedData();
    factory.updateTick(3, {
      woz_price_chan_up: 82,
      woz_price_chan_mid: 52,
      woz_price_chan_dn: 22,
    });
    assert.strictEqual(zipCalls, 0);
    assert.ok(!events.some((e) => e.op === 'setData' || e.op === 'update'));
    assert.strictEqual(factory.hydratedData.has('woz_price_chan_up'), false);
  } finally {
    DDRFactory.zipChannelFromHydrated = orig;
  }
});

test('D. hidden → visible LineSeries hydrates CURRENT store then reveals', () => {
  const events = [];
  const factory = new DDRFactory({
    getColumnarSnapshot: () => ({
      times: [10, 11],
      plots: { woz_ema_rsi: [7, 8] },
    }),
  });
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine(events, 'woz_ema_rsi'); } } } },
    { pane_osc: [{
      id: 'woz_ema_rsi',
      hostId: 'wozduh',
      kind: 'line',
      renderOptions: { defaultVisible: false },
    }] },
  );
  factory.setSeriesVisible('woz_ema_rsi', true);
  const set = events.filter((e) => e.op === 'setData');
  const vis = events.filter((e) => e.op === 'visible');
  assert.strictEqual(set.length, 1);
  assert.deepStrictEqual(set[0].points, [{ time: 10, value: 7 }, { time: 11, value: 8 }]);
  const reveal = vis.filter((e) => e.visible === true);
  assert.ok(reveal.length >= 1);
  assert.ok(events.indexOf(set[0]) < events.indexOf(reveal[reveal.length - 1])
    || events.findIndex((e) => e.op === 'setData') < events.findIndex((e) => e.op === 'visible' && e.visible === true));
});

test('E. hidden → visible ChannelSeries zips CURRENT three columns once then reveals', () => {
  const events = [];
  const factory = new DDRFactory({
    getColumnarSnapshot: () => ({
      times: [5],
      plots: {
        woz_vol_chan_up: [90],
        woz_vol_chan_mid: [60],
        woz_vol_chan_dn: [30],
      },
    }),
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
  factory.setSeriesVisible('woz_vol_chan', true);
  const set = events.filter((e) => e.op === 'setData');
  assert.strictEqual(set.length, 1);
  assert.deepStrictEqual(set[0].points, [{ time: 5, upper: 90, mid: 60, lower: 30 }]);
  assert.ok(events.some((e) => e.op === 'visible' && e.visible === true));
});

test('F. TF switch while hidden: enable gets TF B, never TF A', () => {
  const events = [];
  let tf = 'A';
  const factory = new DDRFactory({
    getColumnarSnapshot: () => (tf === 'A'
      ? { times: [1], plots: { woz_ema_rsi: [100] } }
      : { times: [9], plots: { woz_ema_rsi: [200] } }),
  });
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine(events, 'woz_ema_rsi'); } } } },
    { pane_osc: [{
      id: 'woz_ema_rsi',
      hostId: 'wozduh',
      kind: 'line',
      renderOptions: { defaultVisible: false },
    }] },
  );
  factory.hydrateFromColumnar({ times: [1], plots: { woz_ema_rsi: [100] } });
  factory.applyHydratedData();
  tf = 'B';
  factory.hydrateFromColumnar({ times: [9], plots: { woz_ema_rsi: [200] } });
  factory.applyHydratedData();
  factory.setSeriesVisible('woz_ema_rsi', true);
  const set = events.filter((e) => e.op === 'setData');
  assert.strictEqual(set.length, 1);
  assert.deepStrictEqual(set[0].points, [{ time: 9, value: 200 }]);
});

test('G. hidden woz_slow and line_rsx still receive full/live data', () => {
  const events = [];
  const factory = new DDRFactory();
  factory.buildPanes(
    {
      wozduh: { chart: { addLineSeries() { return fakeLine(events, 'woz_slow'); } } },
      rsx: { chart: { addLineSeries() { return fakeLine(events, 'line_rsx'); } } },
    },
    {
      pane_osc: [{
        id: 'woz_slow',
        hostId: 'wozduh',
        kind: 'line',
        renderOptions: { defaultVisible: false, scaleContribution: { type: 'bounded', min: -5, max: 105 } },
      }],
      pane_rsx: [{
        id: 'line_rsx',
        hostId: 'rsx',
        kind: 'line',
        renderOptions: { defaultVisible: false, scaleContribution: { type: 'bounded', min: -5, max: 105 } },
      }],
    },
  );
  factory.setSeriesVisible('woz_slow', false);
  factory.setSeriesVisible('line_rsx', false);
  factory.hydrateFromColumnar({
    times: [1],
    plots: { woz_slow: [40], line_rsx: [55], woz_fast: [10] },
  });
  factory.applyHydratedData();
  factory.updateTick(2, { woz_slow: 41, line_rsx: 56, woz_fast: 11 });
  assert.ok(events.some((e) => e.op === 'setData' && e.id === 'woz_slow'));
  assert.ok(events.some((e) => e.op === 'setData' && e.id === 'line_rsx'));
  assert.ok(events.some((e) => e.op === 'update' && e.id === 'woz_slow' && e.pt.value === 41));
  assert.ok(events.some((e) => e.op === 'update' && e.id === 'line_rsx' && e.pt.value === 56));
});

test('H. hidden non-anchor Wozduh receives no LWC data work', () => {
  const events = [];
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine(events, 'woz_rsi_rsi'); } } } },
    { pane_osc: [{
      id: 'woz_rsi_rsi',
      hostId: 'wozduh',
      kind: 'line',
      renderOptions: { defaultVisible: false, scaleContribution: { type: 'ignore' } },
    }] },
  );
  factory.hydrateFromColumnar({ times: [1], plots: { woz_rsi_rsi: [33] } });
  factory.applyHydratedData();
  factory.updateTick(2, { woz_rsi_rsi: 34 });
  assert.ok(!events.some((e) => e.op === 'setData' || e.op === 'update'));
});

test('I. setSeriesVisible remains the visibility SSOT (no extra FSM)', () => {
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine([], 'woz_fast'); } } } },
    { pane_osc: [{ id: 'woz_fast', hostId: 'wozduh', kind: 'line', renderOptions: {} }] },
  );
  assert.strictEqual(factory.needsLwcData('woz_fast'), true);
  factory.setSeriesVisible('woz_fast', false);
  assert.strictEqual(factory.needsLwcData('woz_fast'), false);
  factory.setSeriesVisible('woz_fast', true);
  assert.strictEqual(factory.needsLwcData('woz_fast'), true);
  assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS.has('woz_slow'), true);
  assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS.has('woz_fast'), false);
});

console.log('hidden_render_skip_test: ALL PASS');
