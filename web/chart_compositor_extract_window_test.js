/**
 * Track A Step 2 — WS-04 extractWindow must contain committed VIEW.
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

// Under soft limit: unchanged
{
  const snap = makeSnapshot(100);
  const out = ChartCompositor.extractWindow(snap, 15000, {
    viewFromSec: snap.times[10],
    viewToSec: snap.times[20],
  });
  assert(out.times.length === 100, 'under limit returns full snapshot');
}

// No VIEW + over limit: must NOT tip-tail (WS-04 fail-safe → full snapshot)
{
  const snap = makeSnapshot(200);
  const out = ChartCompositor.extractWindow(snap, 50, {});
  assert(out.times.length === 200, 'no VIEW must not tip-tail amputate');
  assert(out.times[0] === snap.times[0], 'keeps left of store');
}

// Mid-history VIEW: must include VIEW, not tip-tail
{
  const snap = makeSnapshot(200);
  const viewFrom = snap.times[40];
  const viewTo = snap.times[60];
  const out = ChartCompositor.extractWindow(snap, 50, {
    viewFromSec: viewFrom,
    viewToSec: viewTo,
  });
  assert(out.times.includes(viewFrom), 'contains VIEW left');
  assert(out.times.includes(viewTo), 'contains VIEW right');
  assert(out.times[out.times.length - 1] !== snap.times[199]
    || out.times.includes(viewFrom), 'not forced tip-only when VIEW is mid');
  assert(out.times.length === 50, `soft window size, got ${out.times.length}`);
  assert(out.times[0] <= viewFrom && out.times[out.times.length - 1] >= viewTo, 'VIEW ⊆ paint');
}

// VIEW larger than soft limit: expand to VIEW span
{
  const snap = makeSnapshot(200);
  const viewFrom = snap.times[10];
  const viewTo = snap.times[150];
  const out = ChartCompositor.extractWindow(snap, 50, {
    viewFromSec: viewFrom,
    viewToSec: viewTo,
  });
  assert(out.times.includes(viewFrom) && out.times.includes(viewTo), 'expanded for large VIEW');
  assert(out.times.length >= 141, `VIEW span floor, got ${out.times.length}`);
}

// Tip VIEW still works (live edge)
{
  const snap = makeSnapshot(200);
  const viewFrom = snap.times[160];
  const viewTo = snap.times[199];
  const out = ChartCompositor.extractWindow(snap, 50, {
    viewFromSec: viewFrom,
    viewToSec: viewTo,
  });
  assert(out.times[out.times.length - 1] === snap.times[199], 'tip retained for live VIEW');
  assert(out.times.includes(viewFrom), 'left of tip VIEW retained');
}

// Annotations follow paint window times (not tip-tail list)
{
  const snap = makeSnapshot(200);
  const viewFrom = snap.times[40];
  const viewTo = snap.times[60];
  const out = ChartCompositor.extractWindow(snap, 50, {
    viewFromSec: viewFrom,
    viewToSec: viewTo,
  });
  const t0 = out.times[0];
  const t1 = out.times[out.times.length - 1];
  for (const ann of out.annotations) {
    const t = Number(ann.time);
    assert(t >= t0 && t <= t1, 'annotation inside paint window');
  }
  assert(!out.annotations.some((a) => a.text === 'R') || out.times.includes(snap.times[199]),
    'right-tip annotation only if tip painted');
}

console.log('chart_compositor_extract_window_test: OK');
