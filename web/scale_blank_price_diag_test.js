/**
 * ADR-020 blank-price diagnostic (A / B / C) — NO FIX.
 *
 * Proves what the codebase can prove without a browser LWC visual:
 *   A — restored price.isAuto === false
 *   B — applyBinding never runs after setData
 *   C — same apply payload after data vs Auto click, but visuals differ (needs real LWC)
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

/** Recording fake chart — captures every priceScale applyOptions call. */
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
            stackHint: '',
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

/** Mirror paintCandles scale hook from chart-core (order contract). */
function paintCandlesLike(state, candles, log) {
  if (!state?.candleSeries || !Array.isArray(candles) || !candles.length) {
    log.push({ step: 'paintCandles', skipped: true, reason: 'no candles' });
    return;
  }
  state.candleSeries.setData(candles);
  log.push({ step: 'setData', bars: candles.length });
  if (typeof ScaleController.applyAll === 'function') {
    ScaleController.applyAll();
    log.push({ step: 'applyAll', afterSetData: true });
  }
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
  timeline.push({
    step: 1,
    name: 'restore prefs',
    price: restored,
  });

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
  paintCandlesLike(
    {
      candleSeries: {
        setData() { /* recorded via timeline */ },
      },
    },
    Array.from({ length: 3001 }, (_, i) => ({ time: i, open: 1, high: 2, low: 0.5, close: 1.5 })),
    timeline,
  );
  const applyCountAfterPaint = chart.applies.length;
  const step3Apply = chart.applies[chart.applies.length - 1];
  const step3Ran = applyCountAfterPaint > applyCountBeforePaint;
  timeline.push({
    step: 3,
    name: 'paintCandles setData → applyAll → applyBinding',
    applyBindingAfterData: step3Ran,
    applyCountDelta: applyCountAfterPaint - applyCountBeforePaint,
    lastApply: step3Apply ? { opts: step3Apply.opts, after: step3Apply.after } : null,
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

  const samePayload = payloadKey(step3Apply) === payloadKey(step4Apply);
  return {
    name,
    restored,
    step3Ran,
    samePayloadAsAutoClick: samePayload,
    step3Payload: step3Apply ? step3Apply.opts : null,
    step4Payload: step4Apply ? step4Apply.opts : null,
    timeline,
  };
}

// ─── Source contract: chart-core paintCandles order ─────────────────────────
const chartCorePath = path.join(__dirname, 'chart-core.js');
const chartCoreSrc = fs.readFileSync(chartCorePath, 'utf8');
const paintFn = chartCoreSrc.match(/function paintCandles\([\s\S]*?\n  \}/);
assert.ok(paintFn, 'paintCandles function found in chart-core.js');
const paintBody = paintFn[0];
const setDataIdx = paintBody.indexOf('setData(candles)');
const applyAllIdx = paintBody.indexOf('ScaleController.applyAll');
assert.ok(setDataIdx >= 0, 'paintCandles calls setData(candles)');
assert.ok(applyAllIdx >= 0, 'paintCandles calls ScaleController.applyAll');
assert.ok(applyAllIdx > setDataIdx, 'applyAll is AFTER setData in paintCandles (B ruled out for happy path)');

console.log('OK source contract: paintCandles setData → applyAll');

// ─── Scenario A seed: persisted Auto OFF (repaired on hydrate) ──────────────
const scenA = runScenario('persisted Auto OFF → repaired', {
  version: 3,
  panes: { price: { isAuto: false, isLog: true } },
});
assert.strictEqual(scenA.restored.isAuto, true, 'A seed repaired to isAuto true on hydrate');
assert.strictEqual(scenA.restored.isLog, true, 'A seed preserves isLog');
assert.strictEqual(scenA.step3Ran, true, 'A: applyBinding after data runs');
assert.strictEqual(scenA.step3Payload.autoScale, true, 'A: step3 applies Auto ON after repair');
console.log('OK scenario A (invalid Auto OFF): hydrate repairs to Auto ON, Log preserved');

// ─── Scenario default / Auto ON ─────────────────────────────────────────────
const scenOn = runScenario('default Auto ON', null);
assert.strictEqual(scenOn.restored.isAuto, true);
assert.strictEqual(scenOn.step3Ran, true, 'Auto ON: applyBinding after data runs');
assert.strictEqual(scenOn.step3Payload.autoScale, true);
assert.strictEqual(scenOn.step4Payload.autoScale, false, 'Auto ON: one click turns OFF (not a heal)');
assert.strictEqual(scenOn.samePayloadAsAutoClick, false);
console.log('OK scenario Auto ON: step3 autoScale=true; one Auto click → false (would not heal)');

// ─── Scenario Log ON + Auto ON ──────────────────────────────────────────────
const scenLog = runScenario('persisted Log ON', {
  version: 3,
  panes: { price: { isAuto: true, isLog: true } },
});
assert.strictEqual(scenLog.restored.isLog, true);
assert.strictEqual(scenLog.step3Payload.mode, MODE_LOG, 'Log pref applied at step3');
assert.strictEqual(scenLog.step3Ran, true);
console.log('OK scenario Log ON: step3 mode=Logarithmic, applyBinding after data runs');

// ─── Idempotent re-apply (C probe without LWC pixels) ───────────────────────
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
  assert.strictEqual(p1, p2, 'two applyAll with same prefs → identical payloads (C condition)');
  assert.strictEqual(chart.applies[n2].before.autoScale, chart.applies[n2].after.autoScale);
  console.log('OK C probe: idempotent applyAll sends identical autoScale/mode (no state transition)');
}

// ─── Verdict ────────────────────────────────────────────────────────────────
const report = {
  title: 'Blank price chart — A/B/C diagnostic (post repairScalePrefs)',
  proven_from_code_and_test: {
    B_step3_never_runs: {
      result: 'RULED OUT for paintCandles happy path',
      evidence: 'chart-core paintCandles: setData then ScaleController.applyAll',
    },
    A_persisted_Auto_OFF: {
      result: 'FIXED by repairScalePrefs on hydrate',
      evidence: 'Invalid Auto OFF without manualRange → Auto ON (preserve Log); dirty persist once',
    },
    C_idempotent_LWC: {
      result: 'Not needed for the blank-chart case (was Branch A)',
      evidence: 'Idempotent applyAll still true; no pulse shipped',
    },
  },
  no_lifecycle_hack: true,
};

console.log('\n========== DIAGNOSTIC REPORT ==========');
console.log(JSON.stringify(report, null, 2));
console.log('========================================\n');

// Human verdict line
console.log('VERDICT:');
console.log('  Branch A (invalid Auto OFF) = FIXED by repairScalePrefs on hydrate.');
console.log('  Branch B = still RULED OUT. Branch C = no pulse shipped.');
console.log('  Hard refresh once to heal existing localStorage; Log is preserved.');
console.log('scale_blank_price_diag_test: ALL PASS');
