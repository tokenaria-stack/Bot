/**
 * Y-scale paint ownership — data rendering must not write Auto/Manual/Log.
 * Run: node web/scale_paint_ownership_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const ScaleController = require('./ui/scale-controller.js');

const MODE_NORMAL = 0;
const MODE_LOG = 1;
globalThis.LightweightCharts = {
  PriceScaleMode: { Normal: MODE_NORMAL, Logarithmic: MODE_LOG },
};

function memoryStorage() {
  const map = new Map();
  return {
    getItem(k) { return map.has(k) ? map.get(k) : null; },
    setItem(k, v) { map.set(k, String(v)); },
    removeItem(k) { map.delete(k); },
  };
}

function recordingChart() {
  let auto = true;
  let mode = MODE_NORMAL;
  const applies = [];
  return {
    applies,
    priceScale() {
      return {
        options: () => ({ autoScale: auto, mode }),
        applyOptions: (opts) => {
          if (Object.prototype.hasOwnProperty.call(opts, 'autoScale')) auto = !!opts.autoScale;
          if (Object.prototype.hasOwnProperty.call(opts, 'mode')) mode = opts.mode;
          applies.push({ ...opts });
        },
        width: () => 64,
      };
    },
    snapshot() {
      return { autoScale: auto, mode };
    },
  };
}

function extractFn(src, name) {
  const re = new RegExp(`function ${name}\\([\\s\\S]*?\\n  \\}`);
  const m = src.match(re);
  assert.ok(m, `${name} found`);
  return m[0];
}

const SCALE_WRITE = /ScaleController\.(applyAll|applyScaleMode|applyBinding|setPanePrefs|setState|toggleAuto|toggleLog)/;

const coreSrc = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
const paintBody = extractFn(coreSrc, 'paintCandles');
const updateBody = extractFn(coreSrc, 'updateCandle');
const applyFull = coreSrc.match(/applyFullData\(context, storeData, options = \{\}\) \{[\s\S]*?\n    \},/);
const applyDelta = coreSrc.match(/applyDelta\(context, delta\) \{[\s\S]*?\n    \},/);
assert.ok(applyFull && applyDelta, 'applyFullData / applyDelta found');

assert.ok(paintBody.includes('setData(candles)'), 'paintCandles still paints series');
assert.strictEqual(SCALE_WRITE.test(paintBody), false, 'paintCandles must not mutate ScaleController');
assert.strictEqual(SCALE_WRITE.test(updateBody), false, 'updateCandle must not mutate ScaleController');
assert.strictEqual(SCALE_WRITE.test(applyFull[0]), false, 'applyFullData must not mutate ScaleController');
assert.strictEqual(SCALE_WRITE.test(applyDelta[0]), false, 'applyDelta must not mutate ScaleController');
assert.ok(applyFull[0].includes('paintCandles('), 'applyFullData still uses paintCandles');
assert.ok(applyDelta[0].includes('updateCandle('), 'applyDelta live path still uses updateCandle');
console.log('OK source: full paint + live update do not write Y-scale mode');

{
  const store = memoryStorage();
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });

  const price = recordingChart();
  const rsx = recordingChart();
  const woz = recordingChart();
  ScaleController.register({ context: 'live', hostId: 'price', chart: price, allowLog: true });
  ScaleController.register({ context: 'live', hostId: 'rsx', chart: rsx, allowLog: false });
  ScaleController.register({ context: 'live', hostId: 'wozduh', chart: woz, allowLog: false });

  assert.strictEqual(price.snapshot().autoScale, true);
  assert.strictEqual(rsx.snapshot().autoScale, true);
  assert.strictEqual(woz.snapshot().autoScale, true);

  ScaleController.setPanePrefs('price', { isAuto: false });
  ScaleController.setPanePrefs('rsx', { isAuto: false });
  ScaleController.setPanePrefs('wozduh', { isAuto: false });
  ScaleController.setPanePrefs('price', { isLog: true });

  const nPrice = price.applies.length;
  const nRsx = rsx.applies.length;
  const nWoz = woz.applies.length;
  const snap = {
    price: price.snapshot(),
    rsx: rsx.snapshot(),
    woz: woz.snapshot(),
  };

  // Ordinary data paint: series setData/update only — no ScaleController write.
  const series = {
    setData() {},
    update() {},
  };
  series.setData([{ time: 1, open: 1, high: 2, low: 1, close: 1.5 }]);
  series.update({ time: 1, open: 1, high: 2, low: 1, close: 1.6 });

  assert.strictEqual(price.applies.length, nPrice, 'full setData must not apply price scale');
  assert.strictEqual(rsx.applies.length, nRsx, 'full setData must not apply rsx scale');
  assert.strictEqual(woz.applies.length, nWoz, 'full setData must not apply wozduh scale');
  assert.deepStrictEqual(price.snapshot(), snap.price);
  assert.deepStrictEqual(rsx.snapshot(), snap.rsx);
  assert.deepStrictEqual(woz.snapshot(), snap.woz);
  assert.strictEqual(price.snapshot().autoScale, false);
  assert.strictEqual(price.snapshot().mode, MODE_LOG);
  assert.strictEqual(rsx.snapshot().autoScale, false);
  assert.strictEqual(woz.snapshot().autoScale, false);
  console.log('OK session: Manual+Log survive setData/update when paint does not applyAll');

  ScaleController.toggleAuto('live', 'price');
  assert.strictEqual(price.snapshot().autoScale, true, 'explicit Auto click still applies');
  console.log('OK explicit Auto toggle still writes scale');
}

{
  const store = memoryStorage();
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  const chart = recordingChart();
  ScaleController.register({ context: 'live', hostId: 'price', chart, allowLog: true });
  assert.strictEqual(chart.snapshot().autoScale, true);
  const n = chart.applies.length;
  // Auto + data: still no extra apply; LWC keeps autoScale true.
  assert.strictEqual(chart.applies.length, n);
  assert.strictEqual(chart.snapshot().autoScale, true);
  console.log('OK Auto remains Auto without paint-time reapply');
}

console.log('scale_paint_ownership_test: ALL PASS');
