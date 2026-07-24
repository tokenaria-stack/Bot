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

console.log('time_camera_test: ALL PASS');
