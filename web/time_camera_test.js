/**
 * ADR-021 P0/P1 TimeCamera unit tests (Node).
 * Run: node web/time_camera_test.js
 */
'use strict';

const assert = require('assert');
const TimeCamera = require('./ui/time-camera.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('echo lock: nested commit during apply is ignored', () => {
  TimeCamera._resetForTests();
  let applyCount = 0;
  let nested = false;
  TimeCamera.bind({
    applyCommitted: () => {
      applyCount += 1;
      nested = TimeCamera.commit({
        visibleRange: { from: 99, to: 199 },
        sourceHostId: 'system',
      });
    },
  });
  const ok = TimeCamera.commit({
    visibleRange: { from: 0, to: 100 },
    barSpacing: 6,
    sourceHostId: 'price',
  });
  assert.strictEqual(ok, true);
  assert.strictEqual(applyCount, 1);
  assert.strictEqual(nested, false);
  assert.deepStrictEqual(TimeCamera.getCanonical().visibleRange, { from: 0, to: 100 });
  assert.strictEqual(TimeCamera.getCanonical().barSpacing, 6);
});

test('two panes propose sequentially; canonical follows last commit; no recurse', () => {
  TimeCamera._resetForTests();
  const applied = [];
  TimeCamera.bind({
    applyCommitted: (state) => {
      applied.push({
        from: state.visibleRange?.from,
        to: state.visibleRange?.to,
        source: state.sourceHostId,
      });
      // Simulate peer LWC echo while syncing
      const echo = TimeCamera.proposeFromPane('price', { from: 1, to: 2 }, 6);
      assert.strictEqual(echo, false);
    },
  });
  assert.strictEqual(
    TimeCamera.proposeFromPane('rsx', { from: 10, to: 110 }, 5),
    true,
  );
  assert.strictEqual(
    TimeCamera.proposeFromPane('wozduh', { from: 20, to: 120 }, 5),
    true,
  );
  assert.strictEqual(applied.length, 2);
  assert.strictEqual(applied[0].source, 'rsx');
  assert.strictEqual(applied[1].source, 'wozduh');
  assert.deepStrictEqual(TimeCamera.getCanonical().visibleRange, { from: 20, to: 120 });
});

test('propose ignored while shouldSkip (live updating)', () => {
  TimeCamera._resetForTests();
  let applies = 0;
  let skip = true;
  TimeCamera.bind({
    applyCommitted: () => { applies += 1; },
    shouldSkip: () => skip,
  });
  assert.strictEqual(TimeCamera.proposeFromPane('price', { from: 0, to: 50 }, 6), false);
  assert.strictEqual(applies, 0);
  skip = false;
  assert.strictEqual(TimeCamera.proposeFromPane('price', { from: 0, to: 50 }, 6), true);
  assert.strictEqual(applies, 1);
});

test('system commit still works while shouldSkip would block proposes', () => {
  TimeCamera._resetForTests();
  let applies = 0;
  TimeCamera.bind({
    applyCommitted: () => { applies += 1; },
    shouldSkip: () => true,
  });
  assert.strictEqual(
    TimeCamera.commit({
      visibleRange: { from: 0, to: 80 },
      sourceHostId: 'system',
    }),
    true,
  );
  assert.strictEqual(applies, 1);
});

test('atomic commit carries range + spacing + rightOffset together', () => {
  TimeCamera._resetForTests();
  let seen = null;
  TimeCamera.bind({
    applyCommitted: (state) => { seen = state; },
  });
  TimeCamera.commit({
    visibleRange: { from: 5, to: 55 },
    barSpacing: 8,
    rightOffset: 0,
    sourceHostId: 'system',
  });
  assert.deepStrictEqual(seen.visibleRange, { from: 5, to: 55 });
  assert.strictEqual(seen.barSpacing, 8);
  assert.strictEqual(seen.rightOffset, 0);
});

test('identical commit is no-op (no apply churn)', () => {
  TimeCamera._resetForTests();
  let applies = 0;
  TimeCamera.bind({
    applyCommitted: () => { applies += 1; },
  });
  const patch = { visibleRange: { from: 1, to: 11 }, barSpacing: 6, sourceHostId: 'price' };
  assert.strictEqual(TimeCamera.commit(patch), true);
  assert.strictEqual(TimeCamera.commit(patch), false);
  assert.strictEqual(applies, 1);
});

test('footer-then-price gestures stay synchronized in canonical state', () => {
  TimeCamera._resetForTests();
  TimeCamera.bind({ applyCommitted: () => {} });
  for (let i = 0; i < 5; i++) {
    TimeCamera.proposeFromPane('rsx', { from: i * 10, to: i * 10 + 100 }, 6);
    TimeCamera.proposeFromPane('price', { from: i * 10 + 1, to: i * 10 + 101 }, 6);
  }
  const c = TimeCamera.getCanonical();
  assert.deepStrictEqual(c.visibleRange, { from: 41, to: 141 });
  assert.strictEqual(c.barSpacing, 6);
});

// ─── ADR-028 D1 shadow (pure helpers + capture; no LWC behavior change) ─────

test('classifyViewIntent: LIVE when tip in frame or within SLACK', () => {
  const { classifyViewIntent } = TimeCamera._helpers;
  assert.strictEqual(classifyViewIntent(100, 99, 1.5), 'LIVE'); // overhang +1
  assert.strictEqual(classifyViewIntent(100, 100, 1.5), 'LIVE'); // on tip
  assert.strictEqual(classifyViewIntent(98.6, 100, 1.5), 'LIVE'); // overhang -1.4 >= -1.5
  assert.strictEqual(classifyViewIntent(200, 100, 1.5), 'LIVE'); // large future still LIVE
});

test('classifyViewIntent: HISTORY when tip clearly off the right', () => {
  const { classifyViewIntent } = TimeCamera._helpers;
  assert.strictEqual(classifyViewIntent(90, 100, 1.5), 'HISTORY');
  assert.strictEqual(classifyViewIntent(98, 100, 1.5), 'HISTORY'); // -2 < -1.5
  assert.strictEqual(classifyViewIntent(NaN, 100, 1.5), null);
});

test('computeCenterLogical is midpoint not left edge', () => {
  const { computeCenterLogical } = TimeCamera._helpers;
  assert.strictEqual(computeCenterLogical({ from: 10, to: 30 }), 20);
  assert.strictEqual(computeCenterLogical({ from: 0, to: 1 }), 0.5);
  assert.strictEqual(computeCenterLogical(null), null);
});

test('computeCenterTimeMs uses center bar from supplied times (pure)', () => {
  const { computeCenterTimeMs } = TimeCamera._helpers;
  const times = [1000, 1060, 1120, 1180, 1240]; // sec
  // center logical ~2 → 1120s → 1120000 ms
  assert.strictEqual(computeCenterTimeMs(times, { from: 0, to: 4 }), 1120 * 1000);
  assert.strictEqual(computeCenterTimeMs([], { from: 0, to: 4 }), null);
});

test('clampRightPadding caps void across density (ADR-029)', () => {
  const { clampRightPadding } = TimeCamera._helpers;
  assert.strictEqual(clampRightPadding(1000, 100), 25); // min(1000, min(50, max(5, 25)))
  assert.strictEqual(clampRightPadding(3, 100), 3);
  assert.strictEqual(clampRightPadding(-1, 100), 0);
  assert.strictEqual(clampRightPadding(100, 8), 5); // floor(8/4)=2 → max(5,2)=5 → min(100,5)=5
});

test('shadow capture after commit: LIVE + geometry from tip/rightOffset', () => {
  TimeCamera._resetForTests();
  TimeCamera.bind({ applyCommitted: () => {} });
  TimeCamera.noteTipLogical(99);
  TimeCamera.commit({
    visibleRange: { from: 50, to: 110 },
    barSpacing: 6,
    rightOffset: 11,
    sourceHostId: 'system',
  });
  const shadow = TimeCamera._getShadowView();
  assert.strictEqual(shadow.intent, 'LIVE');
  assert.strictEqual(shadow.geometry.visibleBars, 60);
  assert.strictEqual(shadow.geometry.barSpacing, 6);
  assert.strictEqual(shadow.geometry.rightPadding, 11); // 110 - 99
  assert.strictEqual(shadow.geometry.centerLogical, 80);
  assert.strictEqual(shadow.geometry.centerTime, null); // no times until observeCommittedWorld
});

test('D1.5 observeCommittedWorld fills centerTime after committed range', () => {
  TimeCamera._resetForTests();
  TimeCamera.bind({ applyCommitted: () => {} });
  // Production-like: camera commit first…
  TimeCamera.commit({
    visibleRange: { from: 0, to: 4 },
    barSpacing: 6,
    rightOffset: 0,
    sourceHostId: 'system',
  });
  // …then compositor publishes committed market world.
  const times = [1000, 1060, 1120, 1180, 1240];
  TimeCamera.observeCommittedWorld({ tipLogical: 4, timesSec: times });
  const shadow = TimeCamera._getShadowView();
  assert.strictEqual(shadow.intent, 'LIVE');
  assert.strictEqual(shadow.geometry.centerTime, 1120 * 1000);
  assert.strictEqual(shadow.geometry.rightPadding, 0);
  assert.strictEqual(shadow.geometry.centerLogical, 2);
});

test('D1.5 never infers tip from rightOffset alone', () => {
  TimeCamera._resetForTests();
  TimeCamera.bind({ applyCommitted: () => {} });
  TimeCamera.commit({
    visibleRange: { from: 50, to: 110 },
    barSpacing: 6,
    rightOffset: 11,
    sourceHostId: 'system',
  });
  const shadow = TimeCamera._getShadowView();
  assert.strictEqual(shadow.intent, null);
  assert.strictEqual(shadow.geometry.rightPadding, null);
  assert.strictEqual(shadow.geometry.centerTime, null);
});

test('observeCommittedWorld does not call applyCommitted (observation only)', () => {
  TimeCamera._resetForTests();
  let applies = 0;
  TimeCamera.bind({ applyCommitted: () => { applies += 1; } });
  TimeCamera.commit({
    visibleRange: { from: 0, to: 10 },
    barSpacing: 6,
    sourceHostId: 'system',
  });
  assert.strictEqual(applies, 1);
  TimeCamera.observeCommittedWorld({
    tipLogical: 10,
    timesSec: Array.from({ length: 11 }, (_, i) => 1000 + i * 60),
  });
  assert.strictEqual(applies, 1);
});

test('shadow capture: HISTORY when tip pulled off right', () => {
  TimeCamera._resetForTests();
  TimeCamera.bind({ applyCommitted: () => {} });
  TimeCamera.noteTipLogical(200);
  TimeCamera.commit({
    visibleRange: { from: 10, to: 100 },
    barSpacing: 6,
    sourceHostId: 'price',
  });
  assert.strictEqual(TimeCamera._getShadowView().intent, 'HISTORY');
});

test('shadow helpers poison / null inputs stay safe', () => {
  const h = TimeCamera._helpers;
  assert.strictEqual(h.computeRightPadding(NaN, 10), null);
  assert.strictEqual(h.clampRightPadding(NaN, 10), 0);
  assert.strictEqual(h.classifyViewIntent(10, NaN), null);
});

test('DataResolve seam unbound returns null; bind works without LWC', () => {
  TimeCamera._resetForTests();
  assert.strictEqual(TimeCamera.resolveNearestLogical(1_700_000_000_000), null);
  TimeCamera.bindDataResolve({
    nearestLogicalForTime: (ms) => (ms > 0 ? 42 : null),
  });
  assert.strictEqual(TimeCamera.resolveNearestLogical(1_700_000_000_000), 42);
  TimeCamera.bindDataResolve(null);
  assert.strictEqual(TimeCamera.resolveNearestLogical(1_700_000_000_000), null);
});

test('D1 commit still applies only via bind hook (no LWC APIs on TimeCamera)', () => {
  const src = require('fs').readFileSync(require('path').join(__dirname, 'ui/time-camera.js'), 'utf8');
  assert.ok(!/setVisibleLogicalRange|scrollToPosition|scrollToRealTime|fitContent/.test(src));
  assert.ok(!/applyOptions/.test(src));
});

console.log('time_camera_test: ALL PASS');
