/**
 * B2.1 / ADR-015 — ProjectionSnapshot atomic apply (Node).
 * Run: node web/projection_apply_test.js
 *
 * Proves soft settings path must use applyProjection, not updatePlots:
 *   REST projects Cap + forming (N+1)
 *   updatePlots → lostProjection (truncates)
 *   applyProjection → store keeps N+1 tip RSX
 *   first WS with identical market state → no tip delta
 */
const { ColumnarStore } = require('./columnar-store.js');

function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

global.chartTime = (t) => {
  const n = Number(t);
  if (!Number.isFinite(n)) return null;
  return n > 1e12 ? Math.floor(n / 1000) : Math.floor(n);
};

const intervalSec = 60;
const t0 = 1_784_730_360;
const t1 = t0 + intervalSec;
const tForm = t1 + intervalSec; // forming tip
const capRSX = 14.95964722;
const formRSX = 16.13161863;

function closedSnapshot() {
  return {
    timeframe: '1m',
    times: [t0, t1],
    candles: {
      open: [100, 101],
      high: [102, 103],
      low: [99, 100],
      close: [101, 102],
      volume: [10, 11],
    },
    plots: {
      line_rsx: [10, capRSX],
      line_woz: [1, 2],
    },
    annotations: [],
    hasMore: false,
  };
}

function projectedSnapshot() {
  return {
    timeframe: '1m',
    times: [t0, t1, tForm],
    candles: {
      open: [100, 101, 102.5],
      high: [102, 103, 103.1],
      low: [99, 100, 102.0],
      close: [101, 102, 102.8],
      volume: [10, 11, 3],
    },
    plots: {
      line_rsx: [10, capRSX, formRSX],
      line_woz: [1, 2, 2.5],
    },
    annotations: [],
    hasMore: false,
    projCont: {
      closedBars: 2,
      projectedForming: true,
      timesLen: 3,
      plotsLen: 3,
      lastOpenSec: tForm,
      lastRSX: formRSX,
    },
  };
}

function tipRSX(store) {
  const col = store.snapshot().plots.line_rsx;
  return col[col.length - 1];
}

// ── Case A: updatePlots loses ADR-010 tip (the bug we fixed) ───────────────
{
  const store = new ColumnarStore();
  store.setTfInterval(intervalSec);
  store.replaceMonolith(closedSnapshot());
  assert(store.barCount() === 2, 'closed store n=2');
  store.updatePlots(projectedSnapshot().plots);
  assert(store.barCount() === 2, 'updatePlots must not grow times');
  assert(Math.abs(tipRSX(store) - capRSX) < 1e-9, 'updatePlots keeps Cap RSX on tip (lost forming)');
  console.log('CASE A (legacy bug): updatePlots lostProjection — OK');
}

// ── Case B: applyProjection keeps N+1 ──────────────────────────────────────
{
  const store = new ColumnarStore();
  store.setTfInterval(intervalSec);
  store.replaceMonolith(closedSnapshot());
  const snap = projectedSnapshot();
  store.applyProjection(snap);
  assert(store.barCount() === 3, 'applyProjection keeps N+1');
  assert(store.lastTimeSec() === tForm, 'tip open = projected forming');
  assert(Math.abs(tipRSX(store) - formRSX) < 1e-9, 'tip RSX = Frame Cur from REST');
  assert(store.invariantOk(), 'invariant after applyProjection');
  assert(store._meta.projectedForming === true, 'meta projectedForming from projCont');
  console.log('CASE B: applyProjection preserves projected forming — OK');
}

// ── Case C: first WS identical market → no tip delta (ADR-015) ─────────────
{
  const store = new ColumnarStore();
  store.setTfInterval(intervalSec);
  store.applyProjection(projectedSnapshot());
  const before = tipRSX(store);
  const beforeN = store.barCount();
  const beforeOpen = store.lastTimeSec();

  const result = store.appendTick({
    time: tForm,
    open: 102.5,
    high: 103.1,
    low: 102.0,
    close: 102.8,
    volume: 3,
    plots: { line_rsx: formRSX, line_woz: 2.5 },
  });
  assert(result && !result.gapDetected, 'WS overwrite accepted');
  assert(result.isNewBar === false, 'same open → overwrite not append');
  assert(store.barCount() === beforeN, 'bar count unchanged');
  assert(store.lastTimeSec() === beforeOpen, 'open unchanged');
  assert(Math.abs(tipRSX(store) - before) < 1e-9, 'identical market → zero tip RSX delta');
  console.log('CASE C: first WS idempotent when market unchanged — OK');
}

// ── Case D: replaceMonolith aliases applyProjection ────────────────────────
{
  const store = new ColumnarStore();
  store.setTfInterval(intervalSec);
  store.replaceMonolith(projectedSnapshot());
  assert(store.barCount() === 3, 'replaceMonolith → applyProjection');
  console.log('CASE D: replaceMonolith aliases applyProjection — OK');
}

console.log('B2.1 projection_apply_test: ALL PASS');
