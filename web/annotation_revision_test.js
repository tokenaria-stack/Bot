/**
 * RSX-SIGNAL-2A.1 — annotation revision gate (no fingerprint, no idle setMarkers).
 * Run: node web/annotation_revision_test.js
 */
'use strict';

const assert = require('assert');
const { ColumnarStore } = require('./columnar-store.js');
const { ChartCompositor } = require('./chart-compositor.js');

function seedStore(store, n = 4) {
  const times = [];
  const open = [];
  const high = [];
  const low = [];
  const close = [];
  const volume = [];
  for (let i = 0; i < n; i++) {
    times.push(1_700_000_000 + i * 60);
    open.push(1);
    high.push(2);
    low.push(1);
    close.push(1.5);
    volume.push(1);
  }
  store.applyProjection({
    times,
    candles: { open, high, low, close, volume },
    plots: { line_rsx: times.map(() => 50) },
    annotations: [{ time: times[0], pane: 'rsx', label: 'H Bull', shape: 'arrowUp', source: 'rsx_zz_div' }],
  });
}

function deltaIntent(time) {
  return {
    mode: 'delta',
    delta: { candle: { time, open: 1, high: 2, low: 1, close: 1.5 }, isNewBar: false, barCount: 4 },
    tick: { time, plots: { line_rsx: 50 } },
  };
}

{
  const store = new ColumnarStore();
  const r0 = store.annotationRevision();
  store.applyProjection({
    times: [1, 2],
    candles: { open: [1, 1], high: [1, 1], low: [1, 1], close: [1, 1], volume: [1, 1] },
    plots: {},
    annotations: [],
  });
  assert.notStrictEqual(store.annotationRevision(), r0, 'replace bumps');
  const afterReplace = store.annotationRevision();
  global.chartTime = (t) => Number(t);
  store.appendTick({
    time: 3,
    open: 1,
    high: 2,
    low: 1,
    close: 1,
    volume: 1,
    annotations: [],
  });
  assert.strictEqual(store.annotationRevision(), afterReplace, 'empty tick.annotations must not bump');
  store.mergeAnnotations([{ time: 1, label: 'H Bull' }]);
  assert.ok(store.annotationRevision() > afterReplace, 'real replace bumps');
  const beforeClear = store.annotationRevision();
  store.clear();
  assert.ok(store.annotationRevision() > beforeClear, 'clear with annotations bumps');
}

{
  global.window = global.window || {};
  let series = { id: 'line_rsx' };
  global.window.DDRFactory = { getSeries: () => series };
  global.rsxShowPivotsFrom = (_s, fb) => (typeof _s?.show_pivots === 'boolean' ? _s.show_pivots : fb);
  global.RsxController = { getSettings: () => ({ show_pivots: true }) };

  let layerCalls = 0;
  let sliceCalls = 0;
  global.ChartAdapter = {
    setLiveUpdating() {},
    applyDelta() { return true; },
    applyLiveAnnotationLayer() { layerCalls += 1; },
  };

  const store = new ColumnarStore();
  seedStore(store);
  const orig = store.getForLightweightCharts.bind(store);
  store.getForLightweightCharts = function wrapped() {
    sliceCalls += 1;
    return orig();
  };

  const compositor = new ChartCompositor({ store, shouldPaint: () => true });
  compositor.flush(deltaIntent(1_700_000_000 + 180));
  assert.strictEqual(layerCalls, 1, 'first delta paints markers');
  assert.strictEqual(sliceCalls, 1, 'first delta may slice');

  const slicesBefore = sliceCalls;
  const layersBefore = layerCalls;
  compositor.flush(deltaIntent(1_700_000_180));
  compositor.flush(deltaIntent(1_700_000_180));
  assert.strictEqual(sliceCalls, slicesBefore, 'idle ticks must not slice annotations');
  assert.strictEqual(layerCalls, layersBefore, 'idle ticks must not setMarkers');

  global.RsxController = { getSettings: () => ({ show_pivots: false }) };
  compositor.flush(deltaIntent(1_700_000_180));
  assert.strictEqual(layerCalls, layersBefore + 1, 'show_pivots change repaints once');

  const afterPivot = layerCalls;
  compositor.flush(deltaIntent(1_700_000_180));
  assert.strictEqual(layerCalls, afterPivot, 'same pivots skip');

  series = { id: 'line_rsx-recreated' };
  compositor.flush(deltaIntent(1_700_000_180));
  assert.strictEqual(layerCalls, afterPivot + 1, 'series recreation hydrates markers');
}

console.log('annotation_revision_test.js passed');
