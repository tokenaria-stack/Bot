/**
 * WOZDUH-OWNER-1 — pane owner is woz_slow (wt22 Aqua), not woz_fast (wt11 Blue).
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

test('B. Wozduh crosshair anchor is woz_slow; RSX remains line_rsx', () => {
  const src = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  const seriesFn = src.slice(
    src.indexOf('function crosshairSeriesForChart'),
    src.indexOf('function crosshairAnchorId'),
  );
  const anchorFn = src.slice(
    src.indexOf('function crosshairAnchorId'),
    src.indexOf('function hydratedValueAtTime'),
  );
  assert.ok(seriesFn.includes("getSeries('woz_slow')"));
  assert.ok(!seriesFn.includes("getSeries('woz_fast')"));
  assert.ok(seriesFn.includes("getSeries('line_rsx')"));
  assert.ok(anchorFn.includes("return 'woz_slow'"));
  assert.ok(!anchorFn.includes("return 'woz_fast'"));
  assert.ok(anchorFn.includes("return 'line_rsx'"));
  assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS.has('woz_slow'), true);
  assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS.has('woz_fast'), false);
  assert.strictEqual(DDRFactory.CROSSHAIR_ANCHORS.has('line_rsx'), true);
});

test('C. hidden woz_fast skips; hidden woz_slow still fed', () => {
  const events = [];
  const order = ['woz_fast', 'woz_slow'];
  let i = 0;
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine(events, order[i++]); } } } },
    {
      pane_osc: [
        { id: 'woz_fast', hostId: 'wozduh', kind: 'line', renderOptions: { defaultVisible: false } },
        {
          id: 'woz_slow',
          hostId: 'wozduh',
          kind: 'line',
          renderOptions: { defaultVisible: false, scaleContribution: { type: 'bounded', min: -5, max: 105 } },
        },
      ],
    },
  );
  factory.setSeriesVisible('woz_fast', false);
  factory.setSeriesVisible('woz_slow', false);
  factory.hydrateFromColumnar({ times: [1], plots: { woz_fast: [10], woz_slow: [40] } });
  factory.applyHydratedData();
  factory.updateTick(2, { woz_fast: 11, woz_slow: 41 });
  assert.ok(!events.some((e) => e.id === 'woz_fast' && (e.op === 'setData' || e.op === 'update')));
  assert.ok(events.some((e) => e.op === 'setData' && e.id === 'woz_slow'));
  assert.ok(events.some((e) => e.op === 'update' && e.id === 'woz_slow' && e.pt.value === 41));
});

test('D. wt22 checked, wt11 unchecked: enabled peers still get LWC data; slow owns bounded Auto', () => {
  const events = [];
  const captured = [];
  const order = ['woz_fast', 'woz_slow', 'woz_rsi_price'];
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
          id: 'woz_fast',
          hostId: 'wozduh',
          kind: 'line',
          renderOptions: { defaultVisible: false, scaleContribution: { type: 'ignore' } },
        },
        {
          id: 'woz_slow',
          hostId: 'wozduh',
          kind: 'line',
          renderOptions: { defaultVisible: true, scaleContribution: { type: 'bounded', min: -5, max: 105 } },
        },
        {
          id: 'woz_rsi_price',
          hostId: 'wozduh',
          kind: 'line',
          renderOptions: { defaultVisible: true, scaleContribution: { type: 'ignore' } },
        },
      ],
    },
  );
  const slow = captured.find((c) => c.id === 'woz_slow');
  const fast = captured.find((c) => c.id === 'woz_fast');
  const peer = captured.find((c) => c.id === 'woz_rsi_price');
  assert.deepStrictEqual(slow.opts.autoscaleInfoProvider(), {
    priceRange: { minValue: -5, maxValue: 105 },
  });
  assert.strictEqual(fast.opts.autoscaleInfoProvider(), null);
  assert.strictEqual(peer.opts.autoscaleInfoProvider(), null);
  const boundedOwners = captured.filter((c) => {
    const p = c.opts.autoscaleInfoProvider;
    if (typeof p !== 'function') return false;
    const info = p();
    return info && info.priceRange;
  });
  assert.strictEqual(boundedOwners.length, 1);
  assert.strictEqual(boundedOwners[0].id, 'woz_slow');

  factory.setSeriesVisible('woz_fast', false);
  events.length = 0;
  factory.hydrateFromColumnar({
    times: [1, 2],
    plots: { woz_fast: [10, 11], woz_slow: [40, 41], woz_rsi_price: [70, 71] },
  });
  factory.applyHydratedData();
  assert.ok(!events.some((e) => e.id === 'woz_fast' && e.op === 'setData'));
  assert.deepStrictEqual(
    events.find((e) => e.op === 'setData' && e.id === 'woz_slow').points,
    [{ time: 1, value: 40 }, { time: 2, value: 41 }],
  );
  assert.deepStrictEqual(
    events.find((e) => e.op === 'setData' && e.id === 'woz_rsi_price').points,
    [{ time: 1, value: 70 }, { time: 2, value: 71 }],
  );
});

test('E. both checked: values unchanged; both receive setData', () => {
  const events = [];
  const order = ['woz_fast', 'woz_slow'];
  let i = 0;
  const factory = new DDRFactory();
  factory.buildPanes(
    { wozduh: { chart: { addLineSeries() { return fakeLine(events, order[i++]); } } } },
    {
      pane_osc: [
        { id: 'woz_fast', hostId: 'wozduh', kind: 'line', renderOptions: {} },
        { id: 'woz_slow', hostId: 'wozduh', kind: 'line', renderOptions: {} },
      ],
    },
  );
  factory.hydrateFromColumnar({ times: [5], plots: { woz_fast: [12.5], woz_slow: [44.5] } });
  factory.applyHydratedData();
  const fast = events.find((e) => e.op === 'setData' && e.id === 'woz_fast');
  const slow = events.find((e) => e.op === 'setData' && e.id === 'woz_slow');
  assert.deepStrictEqual(fast.points, [{ time: 5, value: 12.5 }]);
  assert.deepStrictEqual(slow.points, [{ time: 5, value: 44.5 }]);
});

console.log('wozduh_owner_test: ALL PASS');
