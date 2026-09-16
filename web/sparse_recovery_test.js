/**
 * MICRO-2B — sparse chart recovery isolation (Node).
 * Run: node web/sparse_recovery_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function loadConfigFns() {
  const config = fs.readFileSync(path.join(__dirname, 'config.js'), 'utf8');
  const names = [
    'requiresDenseTimeContinuity',
    'isSparseLiveChart',
    'isSecondsTimeframe',
    'appendLiveTickBuffer',
  ];
  const parts = [];
  for (const name of names) {
    const m = config.match(new RegExp(`function ${name}\\([\\s\\S]*?\\n\\}`));
    assert.ok(m, `missing ${name} in config.js`);
    parts.push(m[0]);
  }
  return new Function(`${parts.join('\n')}; return { requiresDenseTimeContinuity, isSparseLiveChart, isSecondsTimeframe, appendLiveTickBuffer };`)();
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

const fns = loadConfigFns();
const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
const recovery = fs.readFileSync(path.join(__dirname, 'timeline-recovery.js'), 'utf8');

test('A/B. sparse Master heal/publishable do not enter TimelineRecovery or loadDashboard', () => {
  const dense = extractFn(boot, 'denseRecoveryApplies');
  assert.ok(dense.includes('isSparseLiveChart'));
  const heal = extractFn(boot, 'onTimelineHealingFromServer');
  const pub = extractFn(boot, 'onTimelinePublishableFromServer');
  assert.ok(heal.includes('denseRecoveryApplies'));
  assert.ok(heal.includes('return'));
  assert.ok(!heal.includes('loadDashboard'));
  assert.ok(pub.includes('denseRecoveryApplies'));
  assert.ok(pub.includes('return'));
  assert.ok(!pub.includes('loadDashboard'));
  const recovered = extractFn(boot, 'initTimelineRecovery');
  assert.ok(recovered.includes('beginAuthoritativeSnapshot'));
});

test('C. dense 1m still uses TimelineRecovery on Master heal; reconnect is state query', () => {
  const heal = extractFn(boot, 'onTimelineHealingFromServer');
  assert.ok(heal.includes("enterTimelineHealing('server_timeline_healing')"));
  const rec = extractFn(boot, 'onBrowserReconnect');
  assert.ok(rec.includes("requestDenseRecovery('browser_transport_loss')"));
  assert.ok(!rec.includes("enterTimelineHealing('browser_ws_reconnect')"));
  const enter = extractFn(boot, 'enterTimelineHealing');
  assert.ok(enter.includes('timelineRecovery.enter'));
});

test('D. browser reconnect on sparse is Shot 10B snapshot, not TimelineRecovery', () => {
  const rec = extractFn(boot, 'onBrowserReconnect');
  assert.ok(rec.includes('isSparseLiveChart'));
  assert.ok(rec.includes('loadDashboard'));
  assert.ok(rec.includes('viewportAnchor'));
  assert.ok(rec.includes('quiet: true'));
  const beforeDense = rec.slice(0, rec.indexOf('requestDenseRecovery'));
  assert.ok(beforeDense.includes('return'));
  assert.ok(!beforeDense.includes('requestDenseRecovery'));
});

test('E. reconnect captures viewportAnchor (VIEW preserve, not FreshLive)', () => {
  const cap = extractFn(boot, 'captureReconnectViewportAnchor');
  assert.ok(cap.includes('ViewportManager.capture'));
  assert.ok(cap.includes('cameraIntentForTfSwitch'));
  const rec = extractFn(boot, 'onBrowserReconnect');
  assert.ok(rec.includes('captureReconnectViewportAnchor'));
  const load = extractFn(boot, 'loadDashboard');
  assert.ok(load.includes("viewport: restoreLive ? 'restore' : (viewportAnchor ? 'restore' : 'fresh')"));
});

test('F. same-second buffered 1s updates coalesce', () => {
  const pending = [];
  fns.appendLiveTickBuffer(pending, { timeframe: '1s', time: 100, close: 1 }, 5000, '1s');
  fns.appendLiveTickBuffer(pending, { timeframe: '1s', time: 100, close: 2 }, 5000, '1s');
  assert.strictEqual(pending.length, 1);
  assert.strictEqual(pending[0].close, 2);
});

test('G. newer-second ticks survive the buffer (handoff survivors)', () => {
  const pending = [];
  fns.appendLiveTickBuffer(pending, { timeframe: '1s', time: 100, close: 1 }, 5000, '1s');
  fns.appendLiveTickBuffer(pending, { timeframe: '1s', time: 103, close: 2 }, 5000, '1s');
  assert.strictEqual(pending.length, 2);
  assert.strictEqual(pending[1].time, 103);
});

test('H. sparse gap does not enter TimelineRecovery; OpenTime coalesce is seconds-only', () => {
  const enter = extractFn(boot, 'enterTimelineHealing');
  assert.ok(enter.includes('denseRecoveryApplies'));
  assert.ok(fns.isSparseLiveChart('1s'));
  assert.ok(fns.isSparseLiveChart('5s'));
  assert.ok(fns.isSparseLiveChart('1tick'));
  assert.ok(!fns.isSparseLiveChart('1m'));
  assert.ok(!fns.isSparseLiveChart('2m'));
  assert.ok(fns.isSecondsTimeframe('1s'));
  assert.ok(fns.isSecondsTimeframe('5s'));
  assert.ok(!fns.isSecondsTimeframe('1tick'));
  const ticks = [];
  fns.appendLiveTickBuffer(ticks, { timeframe: '1tick', time: 100, close: 1 }, 5000);
  fns.appendLiveTickBuffer(ticks, { timeframe: '1tick', time: 100, close: 2 }, 5000);
  assert.strictEqual(ticks.length, 2, 'ticks must not coalesce by OpenTime');
  const bounded = [];
  for (let i = 0; i < 5; i++) {
    fns.appendLiveTickBuffer(bounded, { timeframe: '1s', time: 100 + i, close: i }, 3, '1s');
  }
  assert.strictEqual(bounded.length, 3);
  assert.strictEqual(bounded[0].time, 102);
});

test('boot wires appendLiveTickBuffer; TimelineRecovery module unchanged', () => {
  const buf = extractFn(boot, 'bufferLiveTick');
  assert.ok(buf.includes('appendLiveTickBuffer'));
  assert.ok(!boot.includes("currentTf === '1s'"));
  assert.ok(!boot.includes("timeframe === '1s'"));
  assert.ok(recovery.includes('watchdogMs'));
  assert.ok(!recovery.includes('isSparseLiveChart'));
});

console.log('sparse_recovery_test: ALL PASS');
