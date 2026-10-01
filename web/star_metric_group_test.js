/**
 * Run: node web/star_metric_group_test.js
 */
'use strict';

const assert = require('assert');
const G = require('./ui/star-metric-group.js');

function test(name, fn) {
  fn();
  console.log('ok', name);
}

test('VWEMA is three rows: line, acceleration, slope — no value child', () => {
  const rows = G.bundleReadings([
    { id: 'm15.vwema', label: 'VWEMA', group: '15m', value: 1, ok: true },
    { id: 'm15.vwemaSlope', label: 'VWEMA slope', group: '15m', value: 0.2, ok: true },
    { id: 'm15.vwemaAccel', label: 'VWEMA acceleration', group: '15m', value: 0.1, ok: true },
  ]);
  assert.strictEqual(rows.length, 1);
  assert.strictEqual(G.rowCount(rows[0]), 3);
  assert.ok(rows[0].level);
  assert.ok(rows[0].slope);
  assert.ok(rows[0].accel);
});

test('catalog VWEMA uses Schema 3 Field IDs and stays at three rows', () => {
  const sections = G.bundleCatalog([
    { id: 'M15.Vwema', label: '15m VWEMA', group: 'Schema 3', section: '15m' },
    { id: 'M15.Slope', label: '15m VWEMA slope', group: 'Schema 3', section: '15m' },
    { id: 'M15.VwemaAccel', label: '15m VWEMA accel', group: 'Schema 3', section: '15m' },
    { id: 'M15.Ema12', label: '15m EMA12', group: 'Schema 3', section: '15m' },
    { id: 'M15.Ema12Slope', label: '15m EMA12 slope', group: 'Schema 3', section: '15m' },
  ]);
  const tf = sections.find((s) => s.title === '15m');
  const vwema = tf.groups.find((g) => g.key === 'M15.Vwema');
  assert.strictEqual(G.rowCount(vwema), 3);
  assert.strictEqual(G.childLabel('slope'), 'slope');
  assert.strictEqual(G.childLabel('accel'), 'acceleration');
  const ema12 = tf.groups.find((g) => g.key === 'M15.Ema12');
  assert.strictEqual(G.rowCount(ema12), 2);
});

test('timeframe is a section above the line', () => {
  const sections = G.bundleCatalog([
    { id: 'H1.Vwema', label: '1h VWEMA', group: 'Schema 3', section: '1h' },
    { id: 'H1.Slope', label: '1h slope', group: 'Schema 3', section: '1h' },
    { id: 'H1.VwemaAccel', label: '1h accel', group: 'Schema 3', section: '1h' },
  ]);
  assert.strictEqual(sections[0].title, '1h');
  assert.strictEqual(sections[0].groups[0].label, 'VWEMA');
});

console.log('star_metric_group_test: ALL PASS');
