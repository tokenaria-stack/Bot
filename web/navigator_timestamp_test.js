/**
 * Debt #83 F3 — Navigator DTO times are Unix ms; mapping must not use chartTime().
 * Run: node web/navigator_timestamp_test.js
 */
'use strict';

const assert = require('assert');
const { ChartDataStore } = require('./store.js');
global.ChartDataStore = ChartDataStore;

const {
  chartTime,
  navigatorMsToChartSec,
  mapNavigatorLinesForChart,
  mapNavigatorBackgroundZones,
  navigatorBarColorMap,
} = require('./mappers.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

const MS_POST = 1_700_000_000_000;
const SEC_POST = 1_700_000_000;
const MS_PRE2001 = 730_944_000_000;
const SEC_PRE2001 = 730_944_000;

test('navigatorMsToChartSec: post-2017 ms → LWC sec', () => {
  assert.strictEqual(navigatorMsToChartSec(MS_POST), SEC_POST);
});

test('navigatorMsToChartSec: pre-2001 ms → LWC sec (no 1e12 guess)', () => {
  assert.strictEqual(navigatorMsToChartSec(MS_PRE2001), SEC_PRE2001);
});

test('mapNavigatorLinesForChart: time1/time2 ms → sec', () => {
  const got = mapNavigatorLinesForChart([
    { time1: MS_POST, time2: MS_POST + 60_000, y1: 1, y2: 2 },
  ]);
  assert.strictEqual(got.length, 1);
  assert.strictEqual(got[0].time1, SEC_POST);
  assert.strictEqual(got[0].time2, SEC_POST + 60);
});

test('mapNavigatorLinesForChart: pre-2001 ms → sec', () => {
  const got = mapNavigatorLinesForChart([
    { time1: MS_PRE2001, time2: MS_PRE2001 + 60_000, y1: 1, y2: 2 },
  ]);
  assert.strictEqual(got.length, 1);
  assert.strictEqual(got[0].time1, SEC_PRE2001);
  assert.strictEqual(got[0].time2, SEC_PRE2001 + 60);
});

test('mapNavigatorBackgroundZones: startTime/endTime ms → sec', () => {
  const got = mapNavigatorBackgroundZones([
    { startTime: MS_POST, endTime: MS_POST + 3_600_000, color: '#089981' },
  ]);
  assert.strictEqual(got.length, 1);
  assert.strictEqual(got[0].startTime, SEC_POST);
  assert.strictEqual(got[0].endTime, SEC_POST + 3600);
  assert.strictEqual(got[0].time1, SEC_POST);
  assert.strictEqual(got[0].time2, SEC_POST + 3600);
});

test('navigator marker time uses navigatorMsToChartSec (ms → sec)', () => {
  assert.strictEqual(navigatorMsToChartSec(MS_POST), SEC_POST);
  assert.strictEqual(navigatorMsToChartSec(MS_PRE2001), SEC_PRE2001);
});

test('navigatorBarColorMap: map keys are Unix ms → LWC sec', () => {
  const map = navigatorBarColorMap({ [String(MS_POST)]: '#00ff00' });
  assert.strictEqual(map.get(SEC_POST), '#00ff00');
});

test('Navigator ms path does not use chartTime heuristic (pre-2001 divergence)', () => {
  // chartTime(< 1e12) keeps the integer; msToChartSec divides.
  assert.strictEqual(chartTime(MS_PRE2001), MS_PRE2001);
  const lines = mapNavigatorLinesForChart([
    { time1: MS_PRE2001, time2: MS_PRE2001 + 60_000, y1: 1, y2: 2 },
  ]);
  const zones = mapNavigatorBackgroundZones([
    { startTime: MS_PRE2001, endTime: MS_PRE2001 + 60_000 },
  ]);
  assert.strictEqual(lines[0].time1, SEC_PRE2001);
  assert.strictEqual(zones[0].startTime, SEC_PRE2001);
  assert.notStrictEqual(lines[0].time1, chartTime(MS_PRE2001));
});

test('chartTime itself is unchanged (still heuristic)', () => {
  assert.strictEqual(chartTime(SEC_POST), SEC_POST);
  assert.strictEqual(chartTime(MS_POST), SEC_POST);
  assert.strictEqual(chartTime(MS_PRE2001), MS_PRE2001);
});

console.log('ALL PASS');
