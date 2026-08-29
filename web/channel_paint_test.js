/**
 * CHANNEL-PAINT-1 — zip, one ChannelSeries update, no phantom line ids.
 * Run: node web/channel_paint_test.js
 */
'use strict';

const assert = require('assert');
const { DDRFactory } = require('./series-factory.js');
const { ChannelSeries, isChannelPoint } = require('./channel-series.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('isChannelPoint requires upper+mid+lower', () => {
  assert.strictEqual(isChannelPoint({ upper: 1, mid: 2, lower: 3 }), true);
  assert.strictEqual(isChannelPoint({ time: 1, upper: 1, mid: 2 }), false);
  assert.strictEqual(isChannelPoint({ time: 1 }), false);
});

test('zip: all present → one composite point', () => {
  const hydrated = new Map([
    ['up', [{ time: 10, value: 80 }]],
    ['mid', [{ time: 10, value: 50 }]],
    ['dn', [{ time: 10, value: 20 }]],
  ]);
  const zipped = DDRFactory.zipChannelFromHydrated(hydrated, { upper: 'up', mid: 'mid', lower: 'dn' });
  assert.deepStrictEqual(zipped, [{ time: 10, upper: 80, mid: 50, lower: 20 }]);
});

test('zip: HISTORY_ABSENT / missing value → whitespace, no fill across hole', () => {
  const hydrated = new Map([
    ['up', [{ time: 1, value: 80 }, { time: 2 }, { time: 3, value: 81 }]],
    ['mid', [{ time: 1, value: 50 }, { time: 2, value: 51 }, { time: 3, value: 52 }]],
    ['dn', [{ time: 1, value: 20 }, { time: 2, value: 21 }, { time: 3, value: 22 }]],
  ]);
  const zipped = DDRFactory.zipChannelFromHydrated(hydrated, { upper: 'up', mid: 'mid', lower: 'dn' });
  assert.strictEqual(zipped.length, 3);
  assert.deepStrictEqual(zipped[0], { time: 1, upper: 80, mid: 50, lower: 20 });
  assert.deepStrictEqual(zipped[1], { time: 2 });
  assert.ok(!('upper' in zipped[1]));
  assert.deepStrictEqual(zipped[2], { time: 3, upper: 81, mid: 52, lower: 22 });
  assert.strictEqual(new ChannelSeries().isWhitespace(zipped[1]), true);
  assert.strictEqual(new ChannelSeries().isWhitespace(zipped[0]), false);
});

test('full paint: one setData per channel; plot columns stay source ids', () => {
  const setDataCalls = [];
  const fakeChart = {
    addLineSeries() {
      return { setData() {}, update() {}, applyOptions() {}, priceScale() { return { applyOptions() {} }; } };
    },
    addCustomSeries() {
      return {
        setData(points) { setDataCalls.push(points); },
        update() {},
        applyOptions() {},
        priceScale() { return { applyOptions() {} }; },
      };
    },
  };
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: fakeChart } },
    {
      pane_osc: [
        { id: 'woz_fast', hostId: 'wozduh', kind: 'line', renderOptions: { scaleContribution: { type: 'bounded', min: -5, max: 105 } } },
        { id: 'woz_vol_chan_up', hostId: 'wozduh', kind: 'plot', dataMode: 'scalar' },
        { id: 'woz_vol_chan_mid', hostId: 'wozduh', kind: 'plot', dataMode: 'scalar' },
        { id: 'woz_vol_chan_dn', hostId: 'wozduh', kind: 'plot', dataMode: 'scalar' },
        {
          id: 'woz_vol_chan',
          hostId: 'wozduh',
          kind: 'channel',
          dataMode: 'compose',
          renderOptions: {
            scaleContribution: { type: 'ignore' },
            plots: { upper: 'woz_vol_chan_up', mid: 'woz_vol_chan_mid', lower: 'woz_vol_chan_dn' },
          },
        },
      ],
    },
  );
  assert.strictEqual(factory.seriesMap.has('woz_vol_chan'), true);
  assert.strictEqual(factory.seriesMap.has('woz_vol_chan_up'), false);
  assert.strictEqual(factory.seriesMap.has('woz_vol_chan_mid'), false);
  assert.strictEqual(factory.seriesMap.has('woz_vol_chan_dn'), false);
  assert.deepStrictEqual(factory.requestedPlotIds().sort(), [
    'woz_fast', 'woz_vol_chan_dn', 'woz_vol_chan_mid', 'woz_vol_chan_up',
  ].sort());

  factory.hydrateFromColumnar({
    times: [100, 101],
    sentinel: DDRFactory.HISTORY_ABSENT,
    plots: {
      woz_fast: [40, 41],
      woz_vol_chan_up: [80, 81],
      woz_vol_chan_mid: [50, 51],
      woz_vol_chan_dn: [20, 21],
    },
  });
  factory.applyHydratedData();
  assert.strictEqual(setDataCalls.length, 1);
  assert.deepStrictEqual(setDataCalls[0], [
    { time: 100, upper: 80, mid: 50, lower: 20 },
    { time: 101, upper: 81, mid: 51, lower: 21 },
  ]);
});

test('LIVE: one update {time,upper,mid,lower}; no phantom Up/Mid/Dn updates', () => {
  const updates = [];
  const fakeChart = {
    addLineSeries() {
      return {
        setData() {},
        update(pt) { updates.push({ id: 'line', pt }); },
        applyOptions() {},
        priceScale() { return { applyOptions() {} }; },
      };
    },
    addCustomSeries() {
      return {
        setData() {},
        update(pt) { updates.push({ id: 'channel', pt }); },
        applyOptions() {},
        priceScale() { return { applyOptions() {} }; },
      };
    },
  };
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: fakeChart } },
    {
      pane_osc: [
        {
          id: 'woz_vol_chan',
          hostId: 'wozduh',
          kind: 'channel',
          renderOptions: {
            scaleContribution: { type: 'ignore' },
            plots: { upper: 'woz_vol_chan_up', mid: 'woz_vol_chan_mid', lower: 'woz_vol_chan_dn' },
          },
        },
      ],
    },
  );
  factory.updateTick(50, {
    woz_vol_chan_up: 80,
    woz_vol_chan_mid: 50,
    woz_vol_chan_dn: 20,
  });
  assert.strictEqual(updates.length, 1);
  assert.strictEqual(updates[0].id, 'channel');
  assert.deepStrictEqual(updates[0].pt, { time: 50, upper: 80, mid: 50, lower: 20 });
});

test('LIVE: any missing column → whitespace update, not interpolated band', () => {
  const updates = [];
  const fakeChart = {
    addCustomSeries() {
      return {
        setData() {},
        update(pt) { updates.push(pt); },
        applyOptions() {},
        priceScale() { return { applyOptions() {} }; },
      };
    },
  };
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: fakeChart } },
    {
      pane_osc: [{
        id: 'woz_price_chan',
        hostId: 'wozduh',
        kind: 'channel',
        renderOptions: {
          scaleContribution: { type: 'ignore' },
          plots: { upper: 'woz_price_chan_up', mid: 'woz_price_chan_mid', lower: 'woz_price_chan_dn' },
        },
      }],
    },
  );
  factory.updateTick(9, {
    woz_price_chan_up: 70,
    woz_price_chan_mid: DDRFactory.HISTORY_ABSENT,
    woz_price_chan_dn: 30,
  });
  assert.strictEqual(updates.length, 1);
  assert.deepStrictEqual(updates[0], { time: 9 });
});

test('ChannelSeries defaults: solid upper/lower (style 0)', () => {
  const opts = new ChannelSeries().defaultOptions();
  assert.strictEqual(opts.upperLineStyle, 0);
  assert.strictEqual(opts.lowerLineStyle, 0);
});

test('channel autoscaleInfoProvider is ignore (null)', () => {
  let captured;
  const fakeChart = {
    addCustomSeries(_view, opts) {
      captured = opts;
      return { setData() {}, update() {}, applyOptions() {}, priceScale() { return { applyOptions() {} }; } };
    },
  };
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: fakeChart } },
    {
      pane_osc: [{
        id: 'woz_vol_chan',
        hostId: 'wozduh',
        kind: 'channel',
        renderOptions: {
          scaleContribution: { type: 'ignore' },
          plots: { upper: 'u', mid: 'm', lower: 'l' },
        },
      }],
    },
  );
  assert.strictEqual(typeof captured.autoscaleInfoProvider, 'function');
  assert.strictEqual(captured.autoscaleInfoProvider(), null);
  assert.ok(!Object.prototype.hasOwnProperty.call(captured, 'plots'));
  assert.ok(!Object.prototype.hasOwnProperty.call(captured, 'scaleContribution'));
});

console.log('channel_paint_test: ALL PASS');
