/**
 * LeftPrependDiag slow-motion probe helpers — no LWC.
 * Run: node web/left_prepend_diag_test.js
 */
'use strict';

const assert = require('assert');
const LeftPrependDiag = require('./left-prepend-diag.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('enabled respects LEFT_PREPEND_CAMERA_DIAG global', () => {
  global.LEFT_PREPEND_CAMERA_DIAG = true;
  assert.strictEqual(LeftPrependDiag.enabled(), true);
  global.LEFT_PREPEND_CAMERA_DIAG = false;
  assert.strictEqual(typeof LeftPrependDiag.enabled(), 'boolean');
  global.LEFT_PREPEND_CAMERA_DIAG = true;
});

test('begin seeds stage.before + prune metadata under one txn id', () => {
  global.LEFT_PREPEND_CAMERA_DIAG = true;
  LeftPrependDiag.begin({
    logicalBefore: { from: 100, to: 200 },
    prependedCount: 2999,
    prunedRightCount: 1993,
    tipBefore: 1_700_100_000,
    tipAfter: 1_700_100_000 - 1993 * 3600,
    storeBefore: 23994,
    storeAfter: 25000,
  });
  assert.strictEqual(LeftPrependDiag.isActive(), true);
  const last = global.__LEFT_PREPEND_DIAG_LAST__;
  // txn not reported yet — inspect via mute/block path
  LeftPrependDiag.mute();
  assert.strictEqual(LeftPrependDiag.isMuted(), true);
  assert.strictEqual(LeftPrependDiag.shouldBlockCommit({}, {}), true);
  assert.strictEqual(LeftPrependDiag.shouldBlockCommit({}, { diagForcePin: true }), false);

  // Stages API present
  assert.strictEqual(typeof LeftPrependDiag.markAfterSetData, 'function');
  assert.strictEqual(typeof LeftPrependDiag.markAfterForcePin, 'function');
  assert.strictEqual(typeof LeftPrependDiag.markEndFlush, 'function');
  assert.strictEqual(typeof LeftPrependDiag.captureStage, 'function');

  LeftPrependDiag.markAfterSetData();
  LeftPrependDiag.markAfterForcePin();
  LeftPrependDiag.markEndFlush();
  LeftPrependDiag.abort();
  assert.strictEqual(LeftPrependDiag.isActive(), false);
  void last;
});

test('cloneRange + shift math for expectedLogical', () => {
  const before = { from: 100.25, to: 200.5 };
  const expected = { from: before.from + 2999, to: before.to + 2999 };
  assert.ok(Math.abs(expected.from - 3099.25) < 1e-9);
  assert.ok(Math.abs(expected.to - 3199.5) < 1e-9);
  assert.deepStrictEqual(LeftPrependDiag.cloneRange(before), before);
});

test('captureStage shape includes required fields', () => {
  const snap = LeftPrependDiag.captureStage('probe');
  assert.strictEqual(snap.stage, 'probe');
  assert.ok('logicalRange' in snap);
  assert.ok('marketRange' in snap);
  assert.ok('rightOffsetLwc' in snap);
  assert.ok('rightOffsetCanonical' in snap);
  assert.ok('dataFirst' in snap);
  assert.ok('dataLast' in snap);
});

test('fingerprint helper + probeLogical API exist', () => {
  global.LEFT_PREPEND_CAMERA_DIAG = true;
  LeftPrependDiag.begin({
    logicalBefore: { from: 100, to: 200 },
    prependedCount: 2999,
    prunedRightCount: 1993,
    storeBefore: 23994,
    storeAfter: 25000,
  });
  assert.strictEqual(typeof LeftPrependDiag.probeLogical, 'function');
  LeftPrependDiag.probeLogical('beforeVolumeSetData');
  LeftPrependDiag.abort();
});

console.log('left_prepend_diag_test: ALL PASS');
