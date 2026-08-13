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

console.log('ALL PASS');
