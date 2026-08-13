/**
 * Track C — paint selects the retained Working Set (full snapshot).
 * No soft RENDER_WINDOW_LIMIT / tip-tail amputate.
 * Run: node web/chart_compositor_extract_window_test.js
 */
const { ChartCompositor } = require('./chart-compositor.js');

function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

function makeSnapshot(n, t0 = 1_700_000_000) {
  const times = [];
  const open = [];
  const high = [];
  const low = [];
  const close = [];
  const volume = [];
  const plots = { line_rsx: [] };
  for (let i = 0; i < n; i++) {
    const t = t0 + i * 60;
    times.push(t);
    open.push(1);
    high.push(2);
    low.push(1);
    close.push(1.5);
    volume.push(1);
    plots.line_rsx.push(50);
  }
  return {
    times,
    candles: { open, high, low, close, volume },
    plots,
    annotations: [
      { time: times[0], text: 'L' },
      { time: times[Math.floor(n / 2)], text: 'M' },
      { time: times[n - 1], text: 'R' },
    ],
  };
}

// Full retained series painted (no 15k soft wall)
{
  const snap = makeSnapshot(40000);
  const viewFrom = snap.times[100];
  const viewTo = snap.times[200];
  const out = ChartCompositor.selectPaintSnapshot(snap, {
    viewFromSec: viewFrom,
    viewToSec: viewTo,
  });
  assert(out.times.length === 40000, `paint full store, got ${out.times.length}`);
  assert(out.times[0] === snap.times[0] && out.times[39999] === snap.times[39999], 'series ends intact');
  assert(out.times.includes(viewFrom) && out.times.includes(viewTo), 'VIEW ⊆ paint');
}

// Mid-history VIEW: still full store (not a 50-bar tip window)
{
  const snap = makeSnapshot(200);
  const viewFrom = snap.times[40];
  const viewTo = snap.times[60];
  const out = ChartCompositor.selectPaintSnapshot(snap, {
    viewFromSec: viewFrom,
    viewToSec: viewTo,
  });
  assert(out.times.length === 200, 'no soft truncation around mid VIEW');
  assert(out.times.includes(viewFrom) && out.times.includes(viewTo), 'VIEW covered');
}

// No VIEW opts: full snapshot
{
  const snap = makeSnapshot(200);
  const out = ChartCompositor.selectPaintSnapshot(snap, {});
  assert(out.times.length === 200, 'no VIEW still paints full retained store');
}

// Wide VIEW (> former soft 15k): full store, not capped
{
  const snap = makeSnapshot(20000);
  const out = ChartCompositor.selectPaintSnapshot(snap, {
    viewFromSec: snap.times[0],
    viewToSec: snap.times[19999],
  });
  assert(out.times.length === 20000, 'wide VIEW paints full series');
}

// Tip VIEW (live edge)
{
  const snap = makeSnapshot(200);
  const out = ChartCompositor.selectPaintSnapshot(snap, {
    viewFromSec: snap.times[160],
    viewToSec: snap.times[199],
  });
  assert(out.times[out.times.length - 1] === snap.times[199], 'tip retained');
  assert(out.times.length === 200, 'tip VIEW does not tip-tail-amputate left store');
}

// Annotations remain on full snapshot
{
  const snap = makeSnapshot(200);
  const out = ChartCompositor.selectPaintSnapshot(snap, {
    viewFromSec: snap.times[40],
    viewToSec: snap.times[60],
  });
  assert(out.annotations.length === 3, 'full annotation set retained with full paint');
}

// VIEW absent from store: still returns snapshot (report-only; no camera invent)
{
  const snap = makeSnapshot(50);
  const errs = [];
  const prev = console.error;
  console.error = (...args) => { errs.push(args); };
  const out = ChartCompositor.selectPaintSnapshot(snap, {
    viewFromSec: 1,
    viewToSec: 2,
  });
  console.error = prev;
  assert(out.times.length === 50, 'missing VIEW still paints store (no silent clamp)');
  assert(errs.length >= 1, 'contract failure reported');
}

function stubAdapter(extra = {}) {
  global.ChartAdapter = {
    setLiveUpdating() {},
    applyFullData() {},
    applyLiveAnnotationLayer() {},
    refreshLiveDecoration() {},
    getVisibleLogicalRange() { return null; },
    ...extra,
  };
}

function mockStore(snapshot) {
  let snapCalls = 0;
  return {
    snapCalls: () => snapCalls,
    barCount: () => snapshot.times.length,
    timesSec: () => snapshot.times,
    invariantOk: () => true,
    invariantMeta: () => ({}),
    snapshot() {
      snapCalls += 1;
      return snapshot;
    },
  };
}

{
  stubAdapter();
  const origBuild = ChartCompositor.snapshotToStoreData;
  let builds = 0;
  ChartCompositor.snapshotToStoreData = function wrappedSnapshotToStoreData(...args) {
    builds += 1;
    return origBuild.apply(this, args);
  };
  const store = mockStore(makeSnapshot(200));
  let afterFlush = 0;
  let deco = 0;
  global.ChartAdapter.refreshLiveDecoration = () => { deco += 1; };
  const compositor = new ChartCompositor({
    store,
    onAfterFlush: () => { afterFlush += 1; },
  });
  compositor.flush({ mode: 'prepend', phase: 'F2', edge: 'left' });
  ChartCompositor.snapshotToStoreData = origBuild;
  assert(store.snapCalls() === 0, 'F2 prepend must not snapshot the store');
  assert(builds === 0, 'F2 prepend must not rebuild snapshotToStoreData');
  assert(deco === 1, 'F2 prepend still refreshes decoration');
  assert(afterFlush === 1, 'F2 prepend still calls onAfterFlush');
}

{
  stubAdapter();
  const origBuild = ChartCompositor.snapshotToStoreData;
  let builds = 0;
  ChartCompositor.snapshotToStoreData = function wrappedSnapshotToStoreData(...args) {
    builds += 1;
    return origBuild.apply(this, args);
  };
  const store = mockStore(makeSnapshot(50));
  let paints = 0;
  global.ChartAdapter.applyFullData = () => { paints += 1; };
  const compositor = new ChartCompositor({ store, onAfterFlush: () => {} });
  compositor.flush({ mode: 'full', phase: 'F1', viewport: 'preserve' });
  const f1Builds = builds;
  const f1Snaps = store.snapCalls();
  compositor.flush({ mode: 'full', phase: 'F2', viewport: 'preserve' });
  ChartCompositor.snapshotToStoreData = origBuild;
  assert(paints === 1, 'F1 is the only full setData paint');
  assert(f1Builds === 1 && f1Snaps === 1, 'F1 builds store data once');
  assert(builds === 1, 'F2 full must not call snapshotToStoreData again');
  assert(store.snapCalls() === 1, 'F2 full must not snapshot again');
}

{
  const snap = makeSnapshot(50);
  const store = mockStore(snap);
  let deltaCalls = 0;
  let fullPaints = 0;
  let ddrTicks = 0;
  let observed = null;
  stubAdapter({
    applyDelta(_ctx, delta) {
      deltaCalls += 1;
      assert(delta && delta.candle, 'delta carries one candle');
      assert(delta.barCount === 50, 'delta barCount is store length');
    },
    applyFullData() { fullPaints += 1; },
  });
  global.TimeCamera = {
    observeCommittedWorld(world) { observed = world; },
  };
  global.window = global.window || global;
  global.window.DDRFactory = {
    cutoverActive: true,
    updateTick() { ddrTicks += 1; },
  };
  const compositor = new ChartCompositor({ store, onAfterFlush: () => {} });
  compositor.flush({
    mode: 'delta',
    delta: {
      candle: { time: snap.times[49], open: 1, high: 2, low: 1, close: 1.5 },
      barCount: 50,
    },
    tick: { time: snap.times[49], plots: { line_rsx: 50 } },
  });
  assert(store.snapCalls() === 0, 'delta must not call store.snapshot');
  assert(deltaCalls === 1, 'delta uses applyDelta');
  assert(fullPaints === 0, 'delta must not applyFullData / setData');
  assert(ddrTicks === 1, 'delta still updates DDR from the tick');
  assert(observed, 'delta still observes TimeCamera');
  assert(observed.tipLogical === 49, 'tipLogical = barCount - 1');
  assert(observed.timesSec === snap.times, 'timesSec is live store times, not a snapshot clone');
}

console.log('chart_compositor_extract_window_test: OK');
