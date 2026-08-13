/**
 * Debt #83 F2 — explicit FE timestamp primitives (no magnitude inference).
 * Run: node web/timestamp_contract_test.js
 */
'use strict';

const assert = require('assert');
const { ChartDataStore } = require('./store.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('msToChartSec: post-2017 Unix ms → chart seconds', () => {
  assert.strictEqual(ChartDataStore.msToChartSec(1_700_000_000_000), 1_700_000_000);
});

test('msToChartSec: pre-2001 Unix ms remains milliseconds-as-ms (1993-03-01)', () => {
  assert.strictEqual(ChartDataStore.msToChartSec(730_944_000_000), 730_944_000);
});

test('msToChartSec: floors leftover milliseconds', () => {
  assert.strictEqual(ChartDataStore.msToChartSec(1_700_000_000_500), 1_700_000_000);
});

test('secToMs: post-2017 Unix sec → milliseconds', () => {
  assert.strictEqual(ChartDataStore.secToMs(1_700_000_000), 1_700_000_000_000);
});

test('secToMs: pre-2001 Unix sec → milliseconds (1993-03-01)', () => {
  assert.strictEqual(ChartDataStore.secToMs(730_944_000), 730_944_000_000);
});

test('secToMs: floors fractional seconds', () => {
  assert.strictEqual(ChartDataStore.secToMs(1_700_000_000.9), 1_700_000_000_000);
});

const { chartTime } = require('./mappers.js');
const { ColumnarStore } = require('./columnar-store.js');
const { DDRFactory } = require('./series-factory.js');

test('F5a chartTime: post-2017 Unix seconds unchanged', () => {
  assert.strictEqual(chartTime(1_502_928_000), 1_502_928_000);
});

test('F5a chartTime: pre-2001 Unix seconds unchanged', () => {
  assert.strictEqual(chartTime(730_944_000), 730_944_000);
});

test('F5a chartTime: does not repair milliseconds', () => {
  assert.strictEqual(chartTime(1_502_928_000_000), 1_502_928_000_000);
  assert.strictEqual(chartTime(730_944_000_000), 730_944_000_000);
});

test('F5a _normTimeSec: seconds-only (no magnitude repair)', () => {
  assert.strictEqual(ColumnarStore._normTimeSec(1_502_928_000), 1_502_928_000);
  assert.strictEqual(ColumnarStore._normTimeSec(730_944_000), 730_944_000);
  assert.strictEqual(ColumnarStore._normTimeSec(1_502_928_000_000), 1_502_928_000_000);
  assert.strictEqual(ColumnarStore._normTimeSec(730_944_000_000), 730_944_000_000);
});

test('F5a defaultNormalizeTime: seconds-only (no magnitude repair)', () => {
  assert.strictEqual(DDRFactory.defaultNormalizeTime(1_502_928_000), 1_502_928_000);
  assert.strictEqual(DDRFactory.defaultNormalizeTime(730_944_000), 730_944_000);
  assert.strictEqual(DDRFactory.defaultNormalizeTime(1_502_928_000_000), 1_502_928_000_000);
  assert.strictEqual(DDRFactory.defaultNormalizeTime(730_944_000_000), 730_944_000_000);
});

global.ChartDataStore = ChartDataStore;
const { ChartCompositor } = require('./chart-compositor.js');
const TimeCamera = require('./ui/time-camera.js');
const fs = require('fs');

test('F5b secToMs: 1502928000 sec → 1502928000000 ms', () => {
  assert.strictEqual(ChartDataStore.secToMs(1_502_928_000), 1_502_928_000_000);
});

test('F5b secToMs: 730944000 sec → 730944000000 ms (pre-2001)', () => {
  assert.strictEqual(ChartDataStore.secToMs(730_944_000), 730_944_000_000);
});

test('F5b secToMs: floors fractional seconds', () => {
  assert.strictEqual(ChartDataStore.secToMs(1_502_928_000.9), 1_502_928_000_000);
});

test('F5b toMs / _toMs / capture / annotations / barTimeToMs delegate secToMs', () => {
  assert.strictEqual(ChartDataStore.toMs(1_502_928_000), 1_502_928_000_000);
  assert.strictEqual(ChartDataStore.toMs(730_944_000), 730_944_000_000);
  assert.strictEqual(ChartDataStore.toMs(1_502_928_000.9), 1_502_928_000_000);

  assert.strictEqual(ColumnarStore._toMs(1_502_928_000), 1_502_928_000_000);
  assert.strictEqual(ColumnarStore._toMs(730_944_000), 730_944_000_000);
  assert.strictEqual(ColumnarStore._toMs(1_502_928_000.9), 1_502_928_000_000);

  const times = [730_944_000, 730_944_060];
  const anchor = ChartCompositor.captureViewportAnchor(times, { from: 0, to: 1 });
  assert.strictEqual(anchor.anchorTimeMs, 730_944_000_000);
  assert.strictEqual(anchor.rightTimeMs, 730_944_060_000);

  const map = ChartCompositor._annotationMapFromList([{ time: 730_944_000, text: 'X' }]);
  assert.strictEqual(map.get(730_944_000_000).timeMs, 730_944_000_000);

  assert.strictEqual(TimeCamera._helpers.barTimeToMs(1_502_928_000), 1_502_928_000_000);
  assert.strictEqual(TimeCamera._helpers.barTimeToMs(730_944_000), 730_944_000_000);
  assert.strictEqual(TimeCamera._helpers.barTimeToMs(1_502_928_000.9), 1_502_928_000_000);
});

test('F5b migrated helpers contain no 1e12 heuristic', () => {
  const storeSrc = fs.readFileSync(require.resolve('./store.js'), 'utf8');
  const toMsBody = storeSrc.match(/static toMs\(t\) \{[\s\S]*?\n  \}/)[0];
  assert.ok(!toMsBody.includes('1e12'), toMsBody);

  const colSrc = fs.readFileSync(require.resolve('./columnar-store.js'), 'utf8');
  const colBody = colSrc.match(/static _toMs\(timeLike\) \{[\s\S]*?\n  \}/)[0];
  assert.ok(!colBody.includes('1e12'), colBody);

  const compSrc = fs.readFileSync(require.resolve('./chart-compositor.js'), 'utf8');
  const capBody = compSrc.match(/static captureViewportAnchor\(timesSec, range\) \{[\s\S]*?\n  \}/)[0];
  assert.ok(!capBody.includes('1e12'), capBody);
  const annBody = compSrc.match(/static _annotationMapFromList\(annotations\) \{[\s\S]*?\n  \}/)[0];
  assert.ok(!annBody.includes('1e12'), annBody);
  const chartSecBody = compSrc.match(/static _chartSecToMs\(sec\) \{[\s\S]*?\n  \}/)[0];
  assert.ok(!chartSecBody.includes('1e12'), chartSecBody);

  const camSrc = fs.readFileSync(require.resolve('./ui/time-camera.js'), 'utf8');
  const barBody = camSrc.match(/function barTimeToMs\(sec\) \{[\s\S]*?\n  \}/)[0];
  assert.ok(!barBody.includes('1e12'), barBody);
});

console.log('ALL PASS');
