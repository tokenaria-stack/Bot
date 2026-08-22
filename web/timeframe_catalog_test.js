/**
 * Native TF catalog vs UI (TF-A).
 * Run: node web/timeframe_catalog_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

const NATIVE = [
  '1m', '3m', '5m', '15m', '30m',
  '1h', '2h', '4h', '6h', '8h', '12h',
  '1d', '1w', '1M',
];

const config = fs.readFileSync(path.join(__dirname, 'config.js'), 'utf8');
const controller = fs.readFileSync(path.join(__dirname, 'ui', 'timeframe-controller.js'), 'utf8');

test('config native list includes missing Binance TFs and 1M', () => {
  for (const id of ['2h', '6h', '8h', '12h', '1M']) {
    assert.ok(config.includes(`'${id}'`), `config missing ${id}`);
  }
  assert.ok(config.includes('NATIVE_BINANCE_TFS'));
});

test('2m 10m 45m 3h are live derived in TF_MENU', () => {
  const menuStart = config.indexOf('const TF_MENU');
  const menu = config.slice(menuStart, config.indexOf('const LS_FAV_KEY'));
  for (const id of ['2m', '10m', '45m', '3h']) {
    assert.ok(menu.includes(`'${id}'`), `menu missing ${id}`);
  }
  assert.ok(!menu.includes("'3d'"), '3d must not be in TF_MENU');
  assert.ok(!menu.includes('TICKS'), 'TICKS menu must stay hidden');
  assert.ok(!menu.includes('SECONDS'), 'SECONDS menu must stay hidden');
  assert.ok(menu.includes("'1M'"), '1M must be in DAYS menu');
  assert.ok(config.includes('LIVE_CHART_TFS'));
  assert.ok(config.includes('NATIVE_BINANCE_TFS'));
});

test('live switch uses live-chart allow-list; 1M is not blocked', () => {
  assert.ok(controller.includes('LIVE_CHART_TFS.includes(resolved)'));
  assert.ok(!controller.includes("if (resolved === '1M') return;"));
  assert.ok(!controller.includes("tfFavorites.filter((id) => id !== '1M')"));
  assert.ok(!controller.includes("if (currentTf === '1M')"));
});

test('native count matches Go catalog (14)', () => {
  assert.strictEqual(NATIVE.length, 14);
});

console.log('timeframe_catalog_test: ALL PASS');
