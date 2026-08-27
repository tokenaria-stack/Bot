/**
 * ISLAND-SLIDE-1 — history merge prunes opposite the acquisition edge.
 * VIEW bounds must not veto dropping the opposite island side.
 * Run: node web/island_slide_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const { ColumnarStore } = require('./columnar-store.js');

function monolith(n, startSec, step = 60) {
  const times = Array.from({ length: n }, (_, i) => startSec + i * step);
  return {
    times,
    candles: {
      open: times.map(() => 1),
      high: times.map(() => 2),
      low: times.map(() => 1),
      close: times.map(() => 1.5),
      volume: times.map(() => 1),
    },
    plots: {},
    annotations: [],
  };
}

function setCap(n) {
  Object.defineProperty(ColumnarStore, 'BUDGET_TARGET', { get: () => n, configurable: true });
  Object.defineProperty(ColumnarStore, 'BUDGET_HARD_CAP', {
    get: () => ColumnarStore.BUDGET_TARGET,
    configurable: true,
  });
}

setCap(100);

{
  const store = new ColumnarStore();
  store.replaceMonolith(monolith(100, 1_700_000_000), { commitPaired: true });
  const headBefore = store.firstTimeSec();
  const tipBefore = store.lastTimeSec();
  const viewFrom = tipBefore - 20 * 60;
  const viewTo = tipBefore;
  const older = monolith(40, headBefore - 40 * 60);
  const r = store.prependMonolith(older, { viewFromSec: viewFrom, viewToSec: viewTo });
  assert.ok(store.firstTimeSec() < headBefore, 'prepend: head advances older');
  assert.strictEqual(store.firstTimeSec(), older.times[0], 'prepend: acquired LEFT kept');
  assert.ok(store.lastTimeSec() < tipBefore, 'prepend: newest/right pruned');
  assert.ok(!store.timesSec().includes(tipBefore), 'prepend: live tip not glued');
  assert.strictEqual(store.barCount(), 100);
  assert.strictEqual(r.pruneDirection, ColumnarStore.PRUNE_FROM_NEWEST);
  assert.ok(store.windowMode === 'history');
}

{
  const store = new ColumnarStore();
  store.replaceMonolith(monolith(100, 1_700_000_000), { commitPaired: true });
  const headBefore = store.firstTimeSec();
  const tipBefore = store.lastTimeSec();
  const viewFrom = headBefore;
  const viewTo = headBefore + 20 * 60;
  const newer = monolith(40, tipBefore + 60);
  const r = store.appendMonolith(newer, { viewFromSec: viewFrom, viewToSec: viewTo });
  assert.ok(store.lastTimeSec() > tipBefore, 'append: tip advances newer');
  assert.strictEqual(store.lastTimeSec(), newer.times[newer.times.length - 1], 'append: acquired RIGHT kept');
  assert.ok(store.firstTimeSec() > headBefore, 'append: oldest/left pruned');
  assert.ok(!store.timesSec().includes(headBefore), 'append: opposite edge dropped');
  assert.strictEqual(store.barCount(), 100);
  assert.strictEqual(r.pruneDirection, ColumnarStore.PRUNE_FROM_OLDEST);
}

{
  const store = new ColumnarStore();
  store.setTfInterval(60);
  global.chartTime = (t) => Number(t);
  store.replaceMonolith(monolith(100, 1_700_000_000), { commitPaired: true });
  const headBefore = store.firstTimeSec();
  const viewTo = store.timesSec()[10];
  const tip = store.lastTimeSec();
  store.appendTick({
    time: tip + 60,
    open: 1, high: 2, low: 1, close: 1.5, volume: 1,
    plots: {},
  }, { viewFromSec: headBefore, viewToSec: viewTo });
  assert.strictEqual(store.lastTimeSec(), tip + 60, 'live appendTick still keeps new tip');
  assert.ok(store.timesSec().includes(headBefore), 'live appendTick VIEW-protects oldest');
}

const orch = fs.readFileSync(path.join(__dirname, 'hydration-orchestrator.js'), 'utf8');
assert.ok(orch.includes('ISLAND-SLIDE invariant: prepend added bars but head did not advance'));
assert.ok(orch.includes('ISLAND-SLIDE invariant: append added bars but tip did not advance'));
assert.ok(/added <= 0 \|\| !tipAdvanced/.test(orch) === false,
  'append must not treat tip-not-advanced as the same latch as added==0');

const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
assert.ok(/rightEmptyClearsDetached: \(\) => isSecondsHistoryNavChart/.test(boot),
  '1s true right tail clears detached (not sparse-child only)');
const oneSStart = boot.indexOf('1s right page added==0 but hasNewer=true');
const oneSEnd = boot.indexOf('if (typeof isSparseSecondChart', oneSStart);
assert.ok(oneSStart >= 0 && oneSEnd > oneSStart);
assert.ok(!/parentResumeAfterSec/.test(boot.slice(oneSStart, oneSEnd)),
  'must not add parentResumeAfter to 1s');

const storeSrc = fs.readFileSync(path.join(__dirname, 'columnar-store.js'), 'utf8');
assert.ok(storeSrc.includes('_slideHistoryIslandCap'));
const preIdx = storeSrc.indexOf('prependMonolith(columnarJson');
const preEnd = storeSrc.indexOf('appendMonolith(columnarJson');
const preBody = storeSrc.slice(preIdx, preEnd);
assert.ok(preBody.includes('_slideHistoryIslandCap'));
assert.ok(!preBody.includes('_enforceBudget'), 'prepend must not call generic _enforceBudget');
const appEnd = storeSrc.indexOf('_maybePromoteLiveWindow() {');
const appBody = storeSrc.slice(preEnd, appEnd);
assert.ok(appBody.includes('_slideHistoryIslandCap'));
assert.ok(!appBody.includes('_enforceBudget'), 'append must not call generic _enforceBudget');
assert.ok(storeSrc.includes('this._enforceBudget(ColumnarStore.PRUNE_FROM_OLDEST'),
  'live appendTick still uses _enforceBudget');

console.log('island_slide_test: ALL PASS');
