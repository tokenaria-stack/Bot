/**
 * PAINT-ORDER-1 — full/prepend snapshot supersedes stale deltas.
 * Run: node web/paint_order_test.js
 */
const fs = require('fs');
const path = require('path');
const { RenderScheduler } = require('./render-scheduler.js');
const { ChartCompositor } = require('./chart-compositor.js');

function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

function delta(time, isNewBar) {
  return {
    mode: 'delta',
    delta: { candle: { time, open: 1, high: 2, low: 1, close: 1.5 }, isNewBar, barCount: 50 },
    tick: { time, plots: { line_rsx: 50 } },
  };
}

function isOlderThanPaintedTip(state, candle) {
  if (!state || !candle) return false;
  const t = Number(candle.time);
  const tip = Number(state._lastRealCandleTime);
  return Number.isFinite(t) && Number.isFinite(tip) && t < tip;
}

{
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  assert(core.includes('function isOlderThanPaintedTip'), 'belt is in chart-core apply path');
  assert(core.includes('t < tip'), 'reject only strictly older times');
  assert(isOlderThanPaintedTip({ _lastRealCandleTime: 200 }, { time: 100 }) === true, 'older rejected');
  assert(isOlderThanPaintedTip({ _lastRealCandleTime: 200 }, { time: 200 }) === false, 'same-time allowed');
  assert(isOlderThanPaintedTip({ _lastRealCandleTime: 200 }, { time: 201 }) === false, 'newer allowed');
  assert(isOlderThanPaintedTip({ _lastRealCandleTime: null }, { time: 100 }) === false, 'no tip yet');
}

function withRaf(fn) {
  const orig = global.requestAnimationFrame;
  const queue = [];
  global.requestAnimationFrame = (cb) => {
    queue.push(cb);
    return queue.length;
  };
  try {
    fn(queue);
  } finally {
    global.requestAnimationFrame = orig;
  }
}

function stubAdapter(extra = {}) {
  if (global.window && global.window.DDRFactory) {
    global.window.DDRFactory.cutoverActive = false;
  }
  global.ChartAdapter = {
    setLiveUpdating() {},
    applyFullData() {},
    applyLiveAnnotationLayer() {},
    refreshLiveDecoration() {},
    getVisibleLogicalRange() { return null; },
    applyDelta() { return true; },
    ...extra,
  };
}

function mockStore(n = 50, t0 = 1_700_000_000) {
  const times = [];
  const open = [];
  const high = [];
  const low = [];
  const close = [];
  const volume = [];
  for (let i = 0; i < n; i++) {
    times.push(t0 + i);
    open.push(1);
    high.push(2);
    low.push(1);
    close.push(1.5);
    volume.push(1);
  }
  const snapshot = {
    times,
    candles: { open, high, low, close, volume },
    plots: {},
    annotations: [],
  };
  return {
    barCount: () => n,
    timesSec: () => times,
    invariantOk: () => true,
    invariantMeta: () => ({}),
    snapshot: () => snapshot,
  };
}

{
  stubAdapter();
  const applied = [];
  global.ChartAdapter.applyDelta = (_ctx, d) => {
    applied.push(d.candle.time);
    return true;
  };
  const compositor = new ChartCompositor({ store: mockStore(), onAfterFlush: () => {} });
  const scheduler = new RenderScheduler(compositor);

  withRaf((queue) => {
    scheduler.markDirty({ mode: 'full', viewport: 'preserve' });
    assert(queue.length === 1, 'F1 RAF armed');
    scheduler.markDirty(delta(100, false));
    assert(scheduler._pending && scheduler._pending.mode === 'delta', 'stale delta queued during F1 wait');
    queue[0]();
    assert(scheduler._pending === null, 'F1 snapshot discards pre-snapshot deltas');
    scheduler.markDirty(delta(101, false));
    assert(scheduler._pending && scheduler._pending.delta.candle.time === 101, 'post-snapshot delta survives');
    queue[1]();
    queue[2]();
    assert(applied.join(',') === '101', `only post-snapshot delta flushed, got ${applied.join(',')}`);
  });
}

{
  stubAdapter();
  const compositor = new ChartCompositor({ store: mockStore(), onAfterFlush: () => {} });
  const scheduler = new RenderScheduler(compositor);

  withRaf((queue) => {
    scheduler.markDirty({ mode: 'full', viewport: 'preserve' });
    scheduler.markDirty(delta(50, false));
    scheduler.markDirty({ mode: 'prepend', edge: 'left', addedBars: 3 });
    assert(scheduler._pending && scheduler._pending.mode === 'prepend', 'prepend coalesced over delta');
    queue[0]();
    assert(scheduler._pending && scheduler._pending.mode === 'prepend', 'delta invalidation must not eat PREPEND');
  });
}

{
  stubAdapter();
  const compositor = new ChartCompositor({ store: mockStore(), onAfterFlush: () => {} });
  const scheduler = new RenderScheduler(compositor);

  withRaf((queue) => {
    scheduler.markDirty({ mode: 'full', viewport: 'preserve' });
    scheduler.markDirty(delta(50, false));
    scheduler.markDirty({ mode: 'full', viewport: 'fresh' });
    assert(scheduler._pending && scheduler._pending.mode === 'full', 'second FULL coalesced over delta');
    queue[0]();
    assert(scheduler._pending && scheduler._pending.mode === 'full', 'delta invalidation must not eat FULL');
  });
}

console.log('paint_order_test: OK');
