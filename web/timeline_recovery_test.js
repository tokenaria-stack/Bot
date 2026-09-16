/**
 * ADR-018 + TIMELINE-RECOVERY-STATE-1 unit tests (Node).
 * Run: node web/timeline_recovery_test.js
 */
'use strict';

const assert = require('assert');
const TimelineRecovery = require('./timeline-recovery.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function withFakeTimers(fn) {
  const timers = [];
  const realSetTimeout = global.setTimeout;
  const realClearTimeout = global.clearTimeout;
  global.setTimeout = (cb, ms) => {
    const id = { fn: cb, ms, cleared: false };
    timers.push(id);
    return id;
  };
  global.clearTimeout = (id) => {
    if (id) id.cleared = true;
  };
  try {
    fn(timers);
  } finally {
    global.setTimeout = realSetTimeout;
    global.clearTimeout = realClearTimeout;
  }
}

test('enter is idempotent — watchdog not reset', () => {
  withFakeTimers((timers) => {
    let enters = 0;
    const tr = TimelineRecovery.create({
      watchdogMs: 1000,
      onEnter: () => { enters += 1; },
    });
    assert.strictEqual(tr.enter('a'), true);
    assert.strictEqual(tr.isHealing(), true);
    assert.strictEqual(tr.isSnapshotRequired(), true);
    assert.strictEqual(enters, 1);
    assert.strictEqual(timers.length, 1);
    assert.strictEqual(tr.enter('b'), false);
    assert.strictEqual(enters, 1);
    assert.strictEqual(timers.length, 1);
    assert.strictEqual(timers[0].cleared, false);
    const r = tr.onTimelineState(true);
    assert.strictEqual(r.action, 'replace');
    assert.strictEqual(tr.isHealing(), false);
    assert.strictEqual(tr.isSnapshotRequired(), true);
    assert.strictEqual(timers[0].cleared, true);
  });
});

test('onEnter runs only on first enter', () => {
  withFakeTimers(() => {
    const calls = [];
    const tr = TimelineRecovery.create({
      watchdogMs: 60_000,
      onEnter: () => calls.push('enter'),
    });
    tr.enter('ws');
    tr.enter('ws');
    tr.onTimelineState(true);
    assert.deepStrictEqual(calls, ['enter']);
  });
});

test('true without snapshotRequired is observation', () => {
  const tr = TimelineRecovery.create({});
  const r = tr.onTimelineState(true);
  assert.strictEqual(r.action, 'observe');
  assert.strictEqual(tr.isSnapshotRequired(), false);
  assert.strictEqual(tr.isHealing(), false);
});

test('false enters HEALING even from LIVE', () => {
  withFakeTimers(() => {
    let enters = 0;
    const tr = TimelineRecovery.create({
      watchdogMs: 60_000,
      onEnter: () => { enters += 1; },
    });
    const r = tr.onTimelineState(false);
    assert.strictEqual(r.action, 'heal');
    assert.strictEqual(tr.isHealing(), true);
    assert.strictEqual(tr.isSnapshotRequired(), true);
    assert.strictEqual(enters, 1);
  });
});

test('markSnapshotRequired + true → replace, not heal', () => {
  const tr = TimelineRecovery.create({ watchdogMs: 60_000 });
  const gen = tr.markSnapshotRequired('fe_gapDetected');
  assert.strictEqual(tr.isHealing(), false);
  const r = tr.onTimelineState(true);
  assert.strictEqual(r.action, 'replace');
  assert.strictEqual(r.generation, gen);
  assert.strictEqual(tr.isHealing(), false);
});

test('snapshotCommitted only for current generation', () => {
  const tr = TimelineRecovery.create({});
  const g1 = tr.markSnapshotRequired('a');
  tr.markSnapshotRequired('b');
  assert.strictEqual(tr.snapshotCommitted(g1), false);
  assert.strictEqual(tr.isSnapshotRequired(), true);
  assert.strictEqual(tr.snapshotCommitted(tr.currentGeneration()), true);
  assert.strictEqual(tr.isSnapshotRequired(), false);
});

test('watchdog Retry does not fake LIVE — onRetry + snapshotRequired', () => {
  withFakeTimers((timers) => {
    let retries = 0;
    const tr = TimelineRecovery.create({
      watchdogMs: 25,
      onRetry: () => { retries += 1; },
    });
    tr.enter('master_unpublishable');
    assert.strictEqual(timers.length, 1);
    timers[0].fn();
    assert.strictEqual(retries, 0, 'watchdog only shows Retry; does not fake recovery');
    assert.strictEqual(tr.isHealing(), true);
    tr.retry();
    assert.strictEqual(retries, 1);
    assert.strictEqual(tr.isSnapshotRequired(), true);
    assert.strictEqual(tr.lastRecoveryReason(), 'manual_retry');
    assert.strictEqual(tr.isHealing(), true);
  });
});

console.log('timeline_recovery_test: ALL PASS');
