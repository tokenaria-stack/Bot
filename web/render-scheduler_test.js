/**
 * Shot 11E — NewBar boundary coalesce (Node).
 * Run: node web/render-scheduler_test.js
 */
const { RenderScheduler } = require('./render-scheduler.js');

function delta(time, isNewBar) {
  return {
    mode: 'delta',
    delta: { candle: { time, open: 1, high: 2, low: 1, close: 1.5 }, isNewBar, barCount: time },
    tick: { time, plots: { line_rsx: 50 } },
  };
}

function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

// Same-bar tips collapse to one.
let p = RenderScheduler._coalesce(null, delta(100, false));
p = RenderScheduler._coalesce(p, delta(100, false));
p = RenderScheduler._coalesce(p, delta(100, false));
assert(!p.deltas || p.deltas.length === 1, 'same-bar must stay length 1');
assert(p.delta.candle.time === 100, 'tip time 100');

// NewBar appends — old tip preserved.
p = RenderScheduler._coalesce(p, delta(101, true));
assert(p.deltas.length === 2, 'newBar must append');
assert(p.deltas[0].candle.time === 100 && p.deltas[1].candle.time === 101, 'chain 100→101');
assert(p.deltas[1].isNewBar === true, 'boundary flag kept');

// Updates on new bar coalesce tip only.
p = RenderScheduler._coalesce(p, delta(101, false));
p = RenderScheduler._coalesce(p, delta(101, false));
assert(p.deltas.length === 2, 'same-bar after boundary must not grow');
assert(p.deltas[1].candle.time === 101, 'tip stays 101');

// Second boundary in same frame (volatile minute roll).
p = RenderScheduler._coalesce(p, delta(102, true));
assert(p.deltas.length === 3, 'second newBar must append');
assert(
  p.deltas.map((d) => d.candle.time).join(',') === '100,101,102',
  'no hole in time chain',
);

console.log('render-scheduler_test: OK');
