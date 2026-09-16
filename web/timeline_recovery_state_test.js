/**
 * TIMELINE-RECOVERY-STATE-1 — boot.js recovery contract (Node, source extract).
 * Run: node web/timeline_recovery_state_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function extractFn(src, name) {
  const re = new RegExp(`function ${name}\\s*\\([^)]*\\)\\s*\\{`);
  const m = src.match(re);
  assert.ok(m, `missing function ${name}`);
  const start = m.index + m[0].length - 1;
  let depth = 0;
  for (let i = start; i < src.length; i++) {
    if (src[i] === '{') depth += 1;
    else if (src[i] === '}') {
      depth -= 1;
      if (depth === 0) return src.slice(m.index, i + 1);
    }
  }
  assert.fail(`unclosed ${name}`);
}

const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
const ws = fs.readFileSync(path.join(__dirname, 'ws.js'), 'utf8');
const recovery = fs.readFileSync(path.join(__dirname, 'timeline-recovery.js'), 'utf8');

test('one recovery entry for reconnect and gapDetected', () => {
  const rec = extractFn(boot, 'onBrowserReconnect');
  assert.ok(rec.includes("requestDenseRecovery('browser_transport_loss')"));
  assert.ok(!rec.includes("enterTimelineHealing('browser_ws_reconnect')"));
  const push = extractFn(boot, 'pushLiveTickDelta');
  assert.ok(push.includes("requestDenseRecovery('fe_gapDetected')"));
  assert.ok(!push.includes("enterTimelineHealing('fe_gapDetected')"));
  const req = extractFn(boot, 'requestDenseRecovery');
  assert.ok(req.includes('markSnapshotRequired'));
  assert.ok(req.includes('requestTimelineState'));
});

test('Master true + snapshotRequired uses replace path, not HEALING enter', () => {
  const apply = extractFn(boot, 'applyMasterTimelineState');
  assert.ok(apply.includes("result.action === 'replace'"));
  assert.ok(apply.includes('beginAuthoritativeSnapshot'));
  assert.ok(apply.includes("result.action === 'observe'"));
});

test('timeline_healing marks snapshot suspect and bumps epoch', () => {
  const heal = extractFn(boot, 'onTimelineHealingFromServer');
  assert.ok(heal.includes("markSnapshotRequired('master_timeline_healing')"));
  assert.ok(heal.includes('bumpProjectionEpoch'));
  assert.ok(heal.includes("enterTimelineHealing('server_timeline_healing')"));
});

test('timeline_publishable uses current-state apply, not exit-LIVE-only', () => {
  const pub = extractFn(boot, 'onTimelinePublishableFromServer');
  assert.ok(pub.includes('applyMasterTimelineState(true'));
  assert.ok(!pub.includes('timelineRecovery.publishable'));
  assert.ok(!pub.includes("if (window.__isDashboardLoading) return"));
});

test('loadDashboard does not drop recovery on in-flight load', () => {
  const init = extractFn(boot, 'initTimelineRecovery');
  assert.ok(!init.includes('__isDashboardLoading'));
  const load = extractFn(boot, 'loadDashboard');
  assert.ok(load.includes('recoveryGeneration'));
  assert.ok(load.includes('snapshotCommitted'));
  assert.ok(load.includes('tickBufferEpoch === epoch'));
});

test('dense deltas gated while snapshotRequired', () => {
  const push = extractFn(boot, 'pushLiveTickDelta');
  assert.ok(push.includes('isSnapshotRequired'));
});

test('viewport capture on authoritative snapshot', () => {
  const snap = extractFn(boot, 'beginAuthoritativeSnapshot');
  assert.ok(snap.includes('captureReconnectViewportAnchor'));
  assert.ok(snap.includes('loadDashboard'));
});

test('wire: requestTimelineState and timeline_state dispatch; no subscribe echo in FE', () => {
  assert.ok(ws.includes('requestTimelineState'));
  assert.ok(ws.includes("type: 'timeline_state_request'"));
  assert.ok(ws.includes("msg.type === 'timeline_state'"));
  assert.ok(ws.includes("type: 'subscribe'"));
});

test('Retry uses same recovery contract', () => {
  const init = extractFn(boot, 'initTimelineRecovery');
  assert.ok(init.includes("requestDenseRecovery('manual_retry')"));
  assert.ok(recovery.includes('onRetry'));
  assert.ok(!recovery.includes('exitToLive(\'WATCHDOG retry\')'));
});

test('FEGap diagnostics present', () => {
  assert.ok(boot.includes('[FEGap]'));
  assert.ok(boot.includes('[FEGapRecovered]'));
});

console.log('timeline_recovery_state_test: ALL PASS');
