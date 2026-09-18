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
      addLineSeries() { return fakeLine(events, 'woz_rsi_close_ema7'); },
    } } },
    { pane_osc: [{
      id: 'woz_rsi_close_ema7',
      hostId: 'wozduh',
      kind: 'line',
      renderOptions: { defaultVisible: false, scaleContribution: { type: 'ignore' } },
    }] },
  );
  factory.hydrateFromColumnar({
    times: [1, 2],
    plots: { woz_rsi_close_ema7: [10, 11], woz_vol_rsi_ema12: [40, 41] },
  });
  factory.applyHydratedData();
  factory.updateTick(3, { woz_rsi_close_ema7: 12, woz_vol_rsi_ema12: 42 });
  assert.strictEqual(factory.hydratedData.has('woz_rsi_close_ema7'), false);
  assert.ok(!events.some((e) => e.op === 'setData'));
  assert.ok(!events.some((e) => e.op === 'update'));
});

test('B. visible LineSeries still setData/update', () => {
  const events = [];
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine(events, 'woz_vol_rsi_ema5'); } } } },
    { pane_osc: [{ id: 'woz_vol_rsi_ema5', hostId: 'wozduh', kind: 'line', renderOptions: {} }] },
  );
  factory.hydrateFromColumnar({ times: [1], plots: { woz_vol_rsi_ema5: [50] } });
  factory.applyHydratedData();
  factory.updateTick(2, { woz_vol_rsi_ema5: 51 });
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
      { wozduh: { chart: { addCustomSeries() { return fakeChannel(events, 'woz_rsi_close_chan'); } } } },
      { pane_osc: [{
        id: 'woz_rsi_close_chan',
        hostId: 'wozduh',
        kind: 'channel',
        renderOptions: {
          defaultVisible: false,
          scaleContribution: { type: 'ignore' },
          plots: { upper: 'woz_rsi_close_chan_up', mid: 'woz_rsi_close_chan_mid', lower: 'woz_rsi_close_chan_dn' },
        },
      }] },
    );
    factory.hydrateFromColumnar({
      times: [1, 2],
      plots: {
        woz_rsi_close_chan_up: [80, 81],
        woz_rsi_close_chan_mid: [50, 51],
        woz_rsi_close_chan_dn: [20, 21],
      },
    });
    factory.applyHydratedData();
    factory.updateTick(3, {
      woz_rsi_close_chan_up: 82,
      woz_rsi_close_chan_mid: 52,
      woz_rsi_close_chan_dn: 22,
    });
    assert.strictEqual(zipCalls, 0);
    assert.ok(!events.some((e) => e.op === 'setData' || e.op === 'update'));
    assert.strictEqual(factory.hydratedData.has('woz_rsi_close_chan_up'), false);
  } finally {
    DDRFactory.zipChannelFromHydrated = orig;
  }
});

test('D. hidden → visible LineSeries hydrates CURRENT store then reveals', () => {
  const events = [];
  const factory = new DDRFactory({
    getColumnarSnapshot: () => ({
      times: [10, 11],
      plots: { woz_rsi_close_ema7: [7, 8] },
    }),
  });
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine(events, 'woz_rsi_close_ema7'); } } } },
    { pane_osc: [{
      id: 'woz_rsi_close_ema7',
      hostId: 'wozduh',
      kind: 'line',
      renderOptions: { defaultVisible: false },
    }] },
  );
  factory.setSeriesVisible('woz_rsi_close_ema7', true);
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
        woz_vol_rsi_ema5_chan_up: [90],
        woz_vol_rsi_ema5_chan_mid: [60],
        woz_vol_rsi_ema5_chan_dn: [30],
      },
    }),
  });
  factory.buildPanes(
    { wozduh: { chart: { addCustomSeries() { return fakeChannel(events, 'woz_vol_rsi_ema5_chan'); } } } },
    { pane_osc: [{
      id: 'woz_vol_rsi_ema5_chan',
      hostId: 'wozduh',
      kind: 'channel',
      renderOptions: {
        defaultVisible: false,
        plots: { upper: 'woz_vol_rsi_ema5_chan_up', mid: 'woz_vol_rsi_ema5_chan_mid', lower: 'woz_vol_rsi_ema5_chan_dn' },
      },
    }] },
  );
  factory.setSeriesVisible('woz_vol_rsi_ema5_chan', true);
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
      ? { times: [1], plots: { woz_rsi_close_ema7: [100] } }
      : { times: [9], plots: { woz_rsi_close_ema7: [200] } }),
  });
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine(events, 'woz_rsi_close_ema7'); } } } },
    { pane_osc: [{
      id: 'woz_rsi_close_ema7',
      hostId: 'wozduh',
      kind: 'line',
      renderOptions: { defaultVisible: false },
    }] },
  );
  factory.hydrateFromColumnar({ times: [1], plots: { woz_rsi_close_ema7: [100] } });
  factory.applyHydratedData();
  tf = 'B';
  factory.hydrateFromColumnar({ times: [9], plots: { woz_rsi_close_ema7: [200] } });
  factory.applyHydratedData();
  factory.setSeriesVisible('woz_rsi_close_ema7', true);
  const set = events.filter((e) => e.op === 'setData');
  assert.strictEqual(set.length, 1);
  assert.deepStrictEqual(set[0].points, [{ time: 9, value: 200 }]);
});

test('G. hidden woz_vol_rsi_ema5 and line_rsx still receive full/live data', () => {
  const events = [];
  const factory = new DDRFactory();
  factory.buildPanes(
    {
      wozduh: { chart: { addLineSeries() { return fakeLine(events, 'woz_vol_rsi_ema5'); } } },
      rsx: { chart: { addLineSeries() { return fakeLine(events, 'line_rsx'); } } },
    },
    {
      pane_osc: [{
        id: 'woz_vol_rsi_ema5',
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
  factory.setSeriesVisible('woz_vol_rsi_ema5', false);
  factory.setSeriesVisible('line_rsx', false);
  factory.hydrateFromColumnar({
    times: [1],
    plots: { woz_vol_rsi_ema5: [40], line_rsx: [55], woz_vol_rsi_ema12: [10] },
  });
  factory.applyHydratedData();
  factory.updateTick(2, { woz_vol_rsi_ema5: 41, line_rsx: 56, woz_vol_rsi_ema12: 11 });
  assert.ok(events.some((e) => e.op === 'setData' && e.id === 'woz_vol_rsi_ema5'));
  assert.ok(events.some((e) => e.op === 'setData' && e.id === 'line_rsx'));
  assert.ok(events.some((e) => e.op === 'update' && e.id === 'woz_vol_rsi_ema5' && e.pt.value === 41));
  assert.ok(events.some((e) => e.op === 'update' && e.id === 'line_rsx' && e.pt.value === 56));
});

test('H. hidden non-anchor Wozduh receives no LWC data work', () => {
  const events = [];
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine(events, 'woz_rsi_rsi_close'); } } } },
    { pane_osc: [{
      id: 'woz_rsi_rsi_close',
      hostId: 'wozduh',
      kind: 'line',
      renderOptions: { defaultVisible: false, scaleContribution: { type: 'ignore' } },
    }] },
  );
  factory.hydrateFromColumnar({ times: [1], plots: { woz_rsi_rsi_close: [33] } });
  factory.applyHydratedData();
  factory.updateTick(2, { woz_rsi_rsi_close: 34 });
  assert.ok(!events.some((e) => e.op === 'setData' || e.op === 'update'));
});

test('I. setSeriesVisible remains the visibility SSOT (no extra FSM)', () => {
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine([], 'woz_vol_rsi_ema12'); } } } },
    { pane_osc: [{ id: 'woz_vol_rsi_ema12', hostId: 'wozduh', kind: 'line', renderOptions: {} }] },
  );
  assert.strictEqual(factory.needsLwcData('woz_vol_rsi_ema12'), true);
  factory.setSeriesVisible('woz_vol_rsi_ema12', false);
  assert.strictEqual(factory.needsLwcData('woz_vol_rsi_ema12'), false);
  factory.setSeriesVisible('woz_vol_rsi_ema12', true);
  assert.strictEqual(factory.needsLwcData('woz_vol_rsi_ema12'), true);
  assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS.has('woz_vol_rsi_ema5'), true);
  assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS.has('woz_vol_rsi_ema12'), false);
});

console.log('hidden_render_skip_test: ALL PASS');
