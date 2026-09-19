/**
 * WOZDUH-OWNER-1 — pane owner is woz_vol_rsi_ema5, not woz_vol_rsi_ema12.
 * Run: node web/wozduh_owner_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
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

test('B. Wozduh crosshair anchor is woz_vol_rsi_ema5; RSX remains line_rsx', () => {
  const src = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  const seriesFn = src.slice(
    src.indexOf('function crosshairSeriesForChart'),
    src.indexOf('function crosshairAnchorId'),
  );
  const anchorFn = src.slice(
    src.indexOf('function crosshairAnchorId'),
    src.indexOf('function hydratedValueAtTime'),
  );
  assert.ok(seriesFn.includes("getSeries('woz_vol_rsi_ema5')"));
  assert.ok(!seriesFn.includes("getSeries('woz_vol_rsi_ema12')"));
  assert.ok(seriesFn.includes("getSeries('line_rsx')"));
  assert.ok(anchorFn.includes("return 'woz_vol_rsi_ema5'"));
  assert.ok(!anchorFn.includes("return 'woz_vol_rsi_ema12'"));
  assert.ok(anchorFn.includes("return 'line_rsx'"));
  assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS.has('woz_vol_rsi_ema5'), true);
  assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS.has('woz_vol_rsi_ema12'), false);
  assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS.has('line_rsx'), true);
});

test('C. hidden woz_vol_rsi_ema12 skips; hidden woz_vol_rsi_ema5 still fed', () => {
  const events = [];
  const order = ['woz_vol_rsi_ema12', 'woz_vol_rsi_ema5'];
  let i = 0;
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine(events, order[i++]); } } } },
    {
      pane_osc: [
        { id: 'woz_vol_rsi_ema12', hostId: 'wozduh', kind: 'line', renderOptions: { defaultVisible: false } },
        {
          id: 'woz_vol_rsi_ema5',
          hostId: 'wozduh',
          kind: 'line',
          renderOptions: { defaultVisible: false, scaleContribution: { type: 'bounded', min: -5, max: 105 } },
        },
      ],
    },
  );
  factory.setSeriesVisible('woz_vol_rsi_ema12', false);
  factory.setSeriesVisible('woz_vol_rsi_ema5', false);
  factory.hydrateFromColumnar({ times: [1], plots: { woz_vol_rsi_ema12: [10], woz_vol_rsi_ema5: [40] } });
  factory.applyHydratedData();
  factory.updateTick(2, { woz_vol_rsi_ema12: 11, woz_vol_rsi_ema5: 41 });
  assert.ok(!events.some((e) => e.id === 'woz_vol_rsi_ema12' && (e.op === 'setData' || e.op === 'update')));
  assert.ok(events.some((e) => e.op === 'setData' && e.id === 'woz_vol_rsi_ema5'));
  assert.ok(events.some((e) => e.op === 'update' && e.id === 'woz_vol_rsi_ema5' && e.pt.value === 41));
});

test('D. ema5 checked, ema12 unchecked: enabled peers still get LWC data; DDR plots ignore Auto', () => {
  const events = [];
  const captured = [];
  const order = ['woz_vol_rsi_ema12', 'woz_vol_rsi_ema5', 'woz_rsi_close'];
  let i = 0;
  const factory = new DDRFactory();
  factory.buildPanes(
    {
      wozduh: {
        chart: {
          addLineSeries(opts) {
            captured.push({ id: order[i], opts });
            return fakeLine(events, order[i++]);
          },
        },
      },
    },
    {
      pane_osc: [
        {
          id: 'woz_vol_rsi_ema12',
          hostId: 'wozduh',
          kind: 'line',
          renderOptions: { defaultVisible: false, scaleContribution: { type: 'ignore' } },
        },
        {
          id: 'woz_vol_rsi_ema5',
          hostId: 'wozduh',
          kind: 'line',
          renderOptions: { defaultVisible: true, scaleContribution: { type: 'ignore' } },
        },
        {
          id: 'woz_rsi_close',
          hostId: 'wozduh',
          kind: 'line',
          renderOptions: { defaultVisible: true, scaleContribution: { type: 'ignore' } },
        },
      ],
    },
  );
  const slow = captured.find((c) => c.id === 'woz_vol_rsi_ema5');
  const fast = captured.find((c) => c.id === 'woz_vol_rsi_ema12');
  const peer = captured.find((c) => c.id === 'woz_rsi_close');
  assert.strictEqual(slow.opts.autoscaleInfoProvider(), null);
  assert.strictEqual(fast.opts.autoscaleInfoProvider(), null);
  assert.strictEqual(peer.opts.autoscaleInfoProvider(), null);
  const boundedOwners = captured.filter((c) => {
    const p = c.opts.autoscaleInfoProvider;
    if (typeof p !== 'function') return false;
    const info = p();
    return info && info.priceRange;
  });
  assert.strictEqual(boundedOwners.length, 0);

  factory.setSeriesVisible('woz_vol_rsi_ema12', false);
  events.length = 0;
  factory.hydrateFromColumnar({
    times: [1, 2],
    plots: { woz_vol_rsi_ema12: [10, 11], woz_vol_rsi_ema5: [40, 41], woz_rsi_close: [70, 71] },
  });
  factory.applyHydratedData();
  assert.ok(!events.some((e) => e.id === 'woz_vol_rsi_ema12' && e.op === 'setData'));
  assert.deepStrictEqual(
    events.find((e) => e.op === 'setData' && e.id === 'woz_vol_rsi_ema5').points,
    [{ time: 1, value: 40 }, { time: 2, value: 41 }],
  );
  assert.deepStrictEqual(
    events.find((e) => e.op === 'setData' && e.id === 'woz_rsi_close').points,
    [{ time: 1, value: 70 }, { time: 2, value: 71 }],
  );
});

test('E. both checked: values unchanged; both receive setData', () => {
  const events = [];
  const order = ['woz_vol_rsi_ema12', 'woz_vol_rsi_ema5'];
  let i = 0;
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine(events, order[i++]); } } } },
    {
      pane_osc: [
        { id: 'woz_vol_rsi_ema12', hostId: 'wozduh', kind: 'line', renderOptions: {} },
        { id: 'woz_vol_rsi_ema5', hostId: 'wozduh', kind: 'line', renderOptions: {} },
      ],
    },
  );
  factory.hydrateFromColumnar({ times: [5], plots: { woz_vol_rsi_ema12: [12.5], woz_vol_rsi_ema5: [44.5] } });
  factory.applyHydratedData();
  const fast = events.find((e) => e.op === 'setData' && e.id === 'woz_vol_rsi_ema12');
  const slow = events.find((e) => e.op === 'setData' && e.id === 'woz_vol_rsi_ema5');
  assert.deepStrictEqual(fast.points, [{ time: 5, value: 12.5 }]);
  assert.deepStrictEqual(slow.points, [{ time: 5, value: 44.5 }]);
});

console.log('wozduh_owner_test: ALL PASS');
