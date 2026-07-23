/**
 * ADR-018 TimelineRecovery unit tests (Node).
 * Run: node web/timeline_recovery_test.js
 */
'use strict';

const assert = require('assert');
const TimelineRecovery = require('./timeline-recovery.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('enter is idempotent — watchdog not reset', () => {
  let enters = 0;
  let recovered = 0;
  const timers = [];
  const realSetTimeout = global.setTimeout;
  const realClearTimeout = global.clearTimeout;
  global.setTimeout = (fn, ms) => {
    const id = { fn, ms, cleared: false };
    timers.push(id);
    return id;
  };
  global.clearTimeout = (id) => {
    if (id) id.cleared = true;
  };

  try {
    const tr = TimelineRecovery.create({
      watchdogMs: 1000,
      onEnter: () => { enters += 1; },
      onRecovered: () => { recovered += 1; },
    });
    assert.strictEqual(tr.enter('a'), true);
    assert.strictEqual(tr.isHealing(), true);
    assert.strictEqual(enters, 1);
    assert.strictEqual(timers.length, 1);
    assert.strictEqual(tr.enter('b'), false);
    assert.strictEqual(enters, 1);
    assert.strictEqual(timers.length, 1);
    assert.strictEqual(timers[0].cleared, false);
    assert.strictEqual(tr.publishable(), true);
    assert.strictEqual(tr.isHealing(), false);
    assert.strictEqual(recovered, 1);
    assert.strictEqual(timers[0].cleared, true);
    assert.strictEqual(tr.publishable(), false);
    assert.strictEqual(recovered, 1);
  } finally {
    global.setTimeout = realSetTimeout;
    global.clearTimeout = realClearTimeout;
  }
});

test('onEnter runs only on first enter', () => {
  const calls = [];
  const tr = TimelineRecovery.create({
    watchdogMs: 60_000,
    onEnter: () => calls.push('enter'),
    onRecovered: () => calls.push('recovered'),
  });
  tr.enter('ws');
  tr.enter('ws');
  tr.publishable();
  assert.deepStrictEqual(calls, ['enter', 'recovered']);
});

test('LIVE publishable is ignored', () => {
  let recovered = 0;
  const tr = TimelineRecovery.create({
    onRecovered: () => { recovered += 1; },
  });
  assert.strictEqual(tr.publishable(), false);
  assert.strictEqual(recovered, 0);
});

console.log('timeline_recovery_test: ALL PASS');
