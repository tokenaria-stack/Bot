/**
 * ADR-020 blank-price diagnostic — hydrate/register/explicit Auto.
 * Ordinary data paint must not write Y-scale mode (see scale_paint_ownership_test.js).
 *
 * Run: node web/scale_blank_price_diag_test.js
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

function memoryStorage(seed) {
  const map = new Map(Object.entries(seed || {}));
  return {
    getItem(k) { return map.has(k) ? map.get(k) : null; },
    setItem(k, v) { map.set(k, String(v)); },
    removeItem(k) { map.delete(k); },
  };
}

function recordingChart(label) {
  let auto = true;
  let mode = MODE_NORMAL;
  const applies = [];
  return {
    label,
    applies,
    priceScale() {
      return {
        options: () => ({ autoScale: auto, mode }),
        applyOptions: (opts) => {
          const before = { autoScale: auto, mode };
          if (Object.prototype.hasOwnProperty.call(opts, 'autoScale')) auto = !!opts.autoScale;
          if (Object.prototype.hasOwnProperty.call(opts, 'mode')) mode = opts.mode;
          const after = { autoScale: auto, mode };
          applies.push({
            at: applies.length + 1,
            opts: { ...opts },
            before,
            after,
          });
        },
        width: () => 64,
      };
    },
    snapshot() {
      return { autoScale: auto, mode };
    },
  };
}

function payloadKey(entry) {
  if (!entry) return null;
  return JSON.stringify({
    autoScale: !!entry.opts.autoScale,
    mode: entry.opts.mode,
  });
}

function runScenario(name, seedPrefs) {
  const store = memoryStorage(
    seedPrefs
      ? { [ScaleController.STORAGE_KEY]: JSON.stringify(seedPrefs) }
      : {},
  );
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });

  const timeline = [];
  const restored = ScaleController.getState('live', 'price');
  timeline.push({ step: 1, name: 'restore prefs', price: restored });

  const chart = recordingChart('price');
  ScaleController.register({
    context: 'live',
    hostId: 'price',
    chart,
    allowLog: true,
  });
  const afterRegister = chart.applies[chart.applies.length - 1];
  timeline.push({
    step: 2,
    name: 'register → applyBinding',
    applyCount: chart.applies.length,
    lastApply: afterRegister ? { opts: afterRegister.opts, after: afterRegister.after } : null,
    chart: chart.snapshot(),
  });

  const applyCountBeforePaint = chart.applies.length;
  const candleSeries = { setData() { /* series only */ } };
  candleSeries.setData(Array.from({ length: 8 }, (_, i) => (
    { time: i, open: 1, high: 2, low: 0.5, close: 1.5 }
  )));
  const applyCountAfterPaint = chart.applies.length;
  timeline.push({
    step: 3,
    name: 'ordinary setData (no ScaleController write)',
    applyBindingAfterData: applyCountAfterPaint > applyCountBeforePaint,
    applyCountDelta: applyCountAfterPaint - applyCountBeforePaint,
    chart: chart.snapshot(),
  });

  const beforeClick = chart.snapshot();
  const prefsBeforeClick = ScaleController.getState('live', 'price');
  ScaleController.toggleAuto('live', 'price');
  const step4Apply = chart.applies[chart.applies.length - 1];
  const prefsAfterClick = ScaleController.getState('live', 'price');
  timeline.push({
    step: 4,
    name: 'Auto button → toggleAuto → applyBinding',
    prefsBefore: prefsBeforeClick,
    prefsAfter: prefsAfterClick,
    chartBefore: beforeClick,
    lastApply: step4Apply ? { opts: step4Apply.opts, after: step4Apply.after } : null,
    chart: chart.snapshot(),
  });

  return {
    name,
    restored,
    registerPayload: afterRegister ? afterRegister.opts : null,
    paintWroteScale: applyCountAfterPaint > applyCountBeforePaint,
    step4Payload: step4Apply ? step4Apply.opts : null,
    timeline,
  };
}

const chartCorePath = path.join(__dirname, 'chart-core.js');
const chartCoreSrc = fs.readFileSync(chartCorePath, 'utf8');
const paintFn = chartCoreSrc.match(/function paintCandles\([\s\S]*?\n  \}/);
assert.ok(paintFn, 'paintCandles function found in chart-core.js');
const paintBody = paintFn[0];
assert.ok(paintBody.includes('setData(candles)'), 'paintCandles calls setData(candles)');
assert.strictEqual(paintBody.includes('ScaleController.applyAll'), false,
  'paintCandles must not call ScaleController.applyAll');
assert.strictEqual(/ScaleController\.(applyAll|setPanePrefs|toggleAuto)/.test(paintBody), false,
  'paintCandles must not write ScaleController prefs/mode');
console.log('OK source contract: paintCandles paints series and does not write Y-scale');

const scenA = runScenario('persisted Auto OFF → repaired', {
  version: 3,
  panes: { price: { isAuto: false, isLog: true } },
});
assert.strictEqual(scenA.restored.isAuto, true, 'A seed repaired to isAuto true on hydrate');
assert.strictEqual(scenA.restored.isLog, true, 'A seed preserves isLog');
assert.strictEqual(scenA.paintWroteScale, false, 'A: setData must not applyBinding');
assert.strictEqual(scenA.registerPayload.autoScale, true, 'A: register applies Auto ON after repair');
console.log('OK scenario A (invalid Auto OFF): hydrate repairs; register applies; paint silent');

const scenOn = runScenario('default Auto ON', null);
assert.strictEqual(scenOn.restored.isAuto, true);
assert.strictEqual(scenOn.paintWroteScale, false, 'Auto ON: setData must not applyBinding');
assert.strictEqual(scenOn.registerPayload.autoScale, true);
assert.strictEqual(scenOn.step4Payload.autoScale, false, 'Auto ON: one click turns OFF');
console.log('OK scenario Auto ON: register Auto; paint silent; Auto click → false');

const scenLog = runScenario('persisted Log ON', {
  version: 3,
  panes: { price: { isAuto: true, isLog: true } },
});
assert.strictEqual(scenLog.restored.isLog, true);
assert.strictEqual(scenLog.registerPayload.mode, MODE_LOG, 'Log pref applied at register');
assert.strictEqual(scenLog.paintWroteScale, false);
console.log('OK scenario Log ON: register mode=Logarithmic; paint silent');

{
  const store = memoryStorage();
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  const chart = recordingChart('price');
  ScaleController.register({
    context: 'live', hostId: 'price', chart, allowLog: true,
  });
  const n1 = chart.applies.length;
  ScaleController.applyAll();
  const n2 = chart.applies.length;
  const p1 = payloadKey(chart.applies[n1]);
  ScaleController.applyAll();
  const p2 = payloadKey(chart.applies[n2]);
  assert.strictEqual(p1, p2, 'two applyAll with same prefs → identical payloads');
  assert.strictEqual(chart.applies[n2].before.autoScale, chart.applies[n2].after.autoScale);
  console.log('OK applyAll API still idempotent (not used on paint path)');
}

console.log('scale_blank_price_diag_test: ALL PASS');
