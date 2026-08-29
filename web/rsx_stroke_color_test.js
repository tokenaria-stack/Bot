/**
 * RSX-STROKE-1 — Pine OB/OS stroke color on line_rsx only.
 * Run: node web/rsx_stroke_color_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const { rsxStrokeColor, GREEN, RED, MID } = require('./rsx-stroke-color.js');
const { DDRFactory } = require('./series-factory.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function fakeLine(events, id) {
  return {
    setData(points) { events.push({ op: 'setData', id, points }); },
    update(pt) { events.push({ op: 'update', id, pt }); },
    applyOptions() {},
    priceScale() { return { applyOptions() {} }; },
  };
}

test('rsxStrokeColor boundaries and invalid', () => {
  assert.strictEqual(rsxStrokeColor(29.99), RED);
  assert.strictEqual(rsxStrokeColor(30), MID);
  assert.strictEqual(rsxStrokeColor(30.01), MID);
  assert.strictEqual(rsxStrokeColor(69.99), MID);
  assert.strictEqual(rsxStrokeColor(70), MID);
  assert.strictEqual(rsxStrokeColor(70.01), GREEN);
  assert.strictEqual(rsxStrokeColor(NaN), undefined);
  assert.strictEqual(rsxStrokeColor(undefined), undefined);
  assert.strictEqual(rsxStrokeColor(null), undefined);
  assert.strictEqual(rsxStrokeColor('x'), undefined);
  assert.strictEqual(GREEN, '#0ebb23');
  assert.strictEqual(RED, '#ff0000');
  assert.strictEqual(MID, '#512DA8');
});

test('A/B. full hydration colors valid line_rsx; whitespace has no color', () => {
  const events = [];
  const factory = new DDRFactory();
  factory.buildPanes(
    { rsx: { chart: { addLineSeries() { return fakeLine(events, 'line_rsx'); } } } },
    { pane_osc: [{ id: 'line_rsx', hostId: 'rsx', kind: 'line', renderOptions: {} }] },
  );
  const absent = DDRFactory.HISTORY_ABSENT;
  factory.hydrateFromColumnar({
    times: [1, 2, 3, 4],
    plots: { line_rsx: [29.99, 30, 70, absent] },
    sentinel: absent,
  });
  const stored = factory.getHydratedSeries('line_rsx');
  assert.ok(stored.every((p) => !Object.prototype.hasOwnProperty.call(p, 'color')), 'hydratedData is not a color cache');
  factory.applyHydratedData();
  const painted = events.find((e) => e.op === 'setData' && e.id === 'line_rsx').points;
  assert.deepStrictEqual(painted, [
    { time: 1, value: 29.99, color: RED },
    { time: 2, value: 30, color: MID },
    { time: 3, value: 70, color: MID },
    { time: 4 },
  ]);
  assert.ok(!Object.prototype.hasOwnProperty.call(painted[3], 'color'));
  assert.ok(!Object.prototype.hasOwnProperty.call(painted[3], 'value'));
});

test('C. _hydrateRenderComponent paints line_rsx colors from store snapshot', () => {
  const events = [];
  const factory = new DDRFactory({
    getColumnarSnapshot() {
      return {
        times: [10, 11],
        plots: { line_rsx: [70.01, 50] },
        sentinel: DDRFactory.HISTORY_ABSENT,
      };
    },
  });
  factory.buildPanes(
    { rsx: { chart: { addLineSeries() { return fakeLine(events, 'line_rsx'); } } } },
    { pane_osc: [{ id: 'line_rsx', hostId: 'rsx', kind: 'line', renderOptions: {} }] },
  );
  factory._hydrateRenderComponent('line_rsx');
  const painted = events.find((e) => e.op === 'setData').points;
  assert.deepStrictEqual(painted, [
    { time: 10, value: 70.01, color: GREEN },
    { time: 11, value: 50, color: MID },
  ]);
});

test('D. live update includes color for line_rsx only', () => {
  const events = [];
  const factory = new DDRFactory();
  factory.buildPanes(
    {
      rsx: { chart: { addLineSeries() { return fakeLine(events, 'picked'); } } },
    },
    {
      pane_osc: [
        { id: 'line_rsx', hostId: 'rsx', kind: 'line', renderOptions: {} },
      ],
    },
  );
  factory.updateTick(9, { line_rsx: 71 });
  const upd = events.find((e) => e.op === 'update');
  assert.deepStrictEqual(upd.pt, { time: 9, value: 71, color: GREEN });
});

test('E. isolation: signal and Wozduh do not get RSX colors', () => {
  const events = [];
  let n = 0;
  const ids = ['line_rsx', 'line_rsx_signal', 'woz_slow'];
  const factory = new DDRFactory();
  factory.buildPanes(
    {
      rsx: { chart: { addLineSeries() { return fakeLine(events, ids[n++]); } } },
      wozduh: { chart: { addLineSeries() { return fakeLine(events, ids[n++]); } } },
    },
    {
      pane_osc: [
        { id: 'line_rsx', hostId: 'rsx', kind: 'line', renderOptions: {} },
        { id: 'line_rsx_signal', hostId: 'rsx', kind: 'line', renderOptions: {} },
      ],
      pane_woz: [{ id: 'woz_slow', hostId: 'wozduh', kind: 'line', renderOptions: {} }],
    },
  );
  factory.hydrateFromColumnar({
    times: [1],
    plots: { line_rsx: [80], line_rsx_signal: [80], woz_slow: [80] },
  });
  factory.applyHydratedData();
  factory.updateTick(2, { line_rsx: 20, line_rsx_signal: 20, woz_slow: 20 });
  const rsxSet = events.find((e) => e.op === 'setData' && e.id === 'line_rsx').points[0];
  const sigSet = events.find((e) => e.op === 'setData' && e.id === 'line_rsx_signal').points[0];
  const wozSet = events.find((e) => e.op === 'setData' && e.id === 'woz_slow').points[0];
  assert.strictEqual(rsxSet.color, GREEN);
  assert.strictEqual(sigSet.color, undefined);
  assert.deepStrictEqual(sigSet, { time: 1, value: 80 });
  assert.deepStrictEqual(wozSet, { time: 1, value: 80 });
  const sigUpd = events.find((e) => e.op === 'update' && e.id === 'line_rsx_signal');
  const wozUpd = events.find((e) => e.op === 'update' && e.id === 'woz_slow');
  const rsxUpd = events.find((e) => e.op === 'update' && e.id === 'line_rsx');
  assert.deepStrictEqual(sigUpd.pt, { time: 2, value: 20 });
  assert.deepStrictEqual(wozUpd.pt, { time: 2, value: 20 });
  assert.deepStrictEqual(rsxUpd.pt, { time: 2, value: 20, color: RED });
});

test('F. layout default color is Pine mid purple', () => {
  const layout = fs.readFileSync(path.join(__dirname, '../ui_config/rsx_layout.go'), 'utf8');
  assert.ok(layout.includes('"color":"#512DA8"'));
  assert.ok(!layout.includes('#E1D2B5'));
  const factorySrc = fs.readFileSync(path.join(__dirname, 'series-factory.js'), 'utf8');
  assert.ok(!factorySrc.includes('columnToLWC') || factorySrc.includes('withRsxStrokeColors'));
  const col = factorySrc.slice(factorySrc.indexOf('static columnToLWC'), factorySrc.indexOf('getHydratedSeries'));
  assert.ok(!col.includes('rsxStrokeColor'));
  assert.ok(!col.includes('#0ebb23'));
});

console.log('rsx_stroke_color_test: ALL PASS');
