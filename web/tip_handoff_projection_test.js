/**
 * #67 Tip handoff — ColumnarStore History→Live projection seam (Node).
 * Run: node web/tip_handoff_projection_test.js
 *
 * Proves appendTick behavior at the F5 seam:
 *   same time  → OVERWRITE history tip plots (dangerous kink-on-tip)
 *   time+interval → APPEND new forming tip (Tip Protocol; kink = consecutive-bar Δ)
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

const tipSec = 1_784_606_940;
const intervalSec = 60;
const histRSX = 78.82314425;

const store = new ColumnarStore();
store.setTfInterval(intervalSec);
store.replaceMonolith({
  times: [tipSec - 120, tipSec - 60, tipSec],
  candles: {
    open: [1, 2, 3],
    high: [1, 2, 3],
    low: [1, 2, 3],
    close: [1, 2, 3],
    volume: [10, 10, 10],
  },
  plots: {
    line_rsx: [70, 75, histRSX],
    line_woz: [1, 2, 3],
  },
  annotations: [],
  timeframe: '1m',
});

assert(store.lastTimeSec() === tipSec, 'history tip time');
assert(store._plots.line_rsx[2] === histRSX, 'history tip RSX');

// ── Case OVERWRITE: live tick same open as history tip ─────────────────────
const ow = store.appendTick({
  time: tipSec,
  open: 3,
  high: 4,
  low: 2,
  close: 3.5,
  volume: 20,
  plots: { line_rsx: 70.69848236, line_woz: 9 },
});
assert(ow && !ow.gapDetected, 'overwrite accepted');
assert(ow.isNewBar === false, 'same time → not new bar');
assert(store.barCount() === 3, 'overwrite keeps bar count');
assert(store._plots.line_rsx[2] === 70.69848236, 'tip RSX overwritten');
assert(Math.abs(store._plots.line_rsx[2] - histRSX) > 1, 'overwrite creates tip kink on SAME bar');
console.log('CASE OVERWRITE: same openTime mutates history tip RSX', histRSX, '→', store._plots.line_rsx[2]);

// Reset tip RSX for append case
store._plots.line_rsx[2] = histRSX;

// ── Case APPEND: first forming bar = tip + interval ────────────────────────
const ap = store.appendTick({
  time: tipSec + intervalSec,
  open: 3.5,
  high: 5,
  low: 3,
  close: 4,
  volume: 30,
  plots: { line_rsx: 70.69848236, line_woz: 8 },
});
assert(ap && !ap.gapDetected, 'append accepted');
assert(ap.isNewBar === true, 'next open → new bar');
assert(store.barCount() === 4, 'append grows store');
assert(store._plots.line_rsx[2] === histRSX, 'history tip RSX preserved');
assert(store._plots.line_rsx[3] === 70.69848236, 'forming tip RSX on new bar');
assert(store.lastTimeSec() === tipSec + intervalSec, 'tip advanced');
console.log('CASE APPEND: hist tip RSX preserved; forming tip |ΔRSX|=',
  Math.abs(70.69848236 - histRSX));
console.log('VERDICT: F5 kink with deltaSec=interval is APPEND seam (Tip Protocol), not store bug.');
console.log('VERDICT: F5 kink with deltaSec=0 is OVERWRITE of Cap-closed tip — projection bug class.');
console.log('tip_handoff_projection_test.js OK');
