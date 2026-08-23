/**
 * MICRO-2C — sparse off-screen live paint (Node).
 * Run: node web/sparse_offscreen_paint_test.js
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

function loadFn(config, name) {
  const m = config.match(new RegExp(`function ${name}\\([\\s\\S]*?\\n\\}`));
  assert.ok(m, `missing ${name}`);
  return new Function(`${m[0]}; return ${name};`)();
}

const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
const config = fs.readFileSync(path.join(__dirname, 'config.js'), 'utf8');
const shouldMarkDirtyLiveDelta = eval( // eslint-disable-line no-eval
  `(${extractFn(boot, 'shouldMarkDirtyLiveDelta')})`,
);
const sparseHistoryToLiveNeedsFullPaint = loadFn(config, 'sparseHistoryToLiveNeedsFullPaint');

test('A. sparse LIVE → live delta still paints', () => {
  assert.strictEqual(shouldMarkDirtyLiveDelta('LIVE', false, true), true);
  assert.strictEqual(shouldMarkDirtyLiveDelta('LIVE', true, true), true);
});

test('B. sparse HISTORY → LWC delta skipped (store ingest is elsewhere)', () => {
  assert.strictEqual(shouldMarkDirtyLiveDelta('HISTORY', false, true), false);
  assert.strictEqual(shouldMarkDirtyLiveDelta('HISTORY', true, true), false);
});

test('C. HISTORY→LIVE → exactly one full paint', () => {
  assert.strictEqual(sparseHistoryToLiveNeedsFullPaint('HISTORY', 'LIVE'), true);
  assert.strictEqual(sparseHistoryToLiveNeedsFullPaint('LIVE', 'LIVE'), false);
  assert.strictEqual(sparseHistoryToLiveNeedsFullPaint('HISTORY', 'HISTORY'), false);
  assert.strictEqual(sparseHistoryToLiveNeedsFullPaint(null, 'LIVE'), false);
  const maybe = extractFn(boot, 'maybeSparseHistoryToLivePaint');
  assert.ok(maybe.includes("mode: 'full'"));
  assert.ok(!maybe.includes("mode: 'delta'"));
});

test('D. after full paint, live deltas resume', () => {
  assert.strictEqual(sparseHistoryToLiveNeedsFullPaint('LIVE', 'LIVE'), false);
  assert.strictEqual(shouldMarkDirtyLiveDelta('LIVE', true, true), true);
});

test('E. native 1m behavior unchanged (2-arg / sparse=false)', () => {
  assert.strictEqual(shouldMarkDirtyLiveDelta('LIVE', true), true);
  assert.strictEqual(shouldMarkDirtyLiveDelta('HISTORY', false), false);
  assert.strictEqual(shouldMarkDirtyLiveDelta('HISTORY', true), true);
  assert.strictEqual(shouldMarkDirtyLiveDelta('HISTORY', true, false), true);
});

test('F. return-to-live paint restores VIEW; does not FreshLive', () => {
  const maybe = extractFn(boot, 'maybeSparseHistoryToLivePaint');
  assert.ok(maybe.includes("viewport: anchor ? 'restore' : 'preserve'"));
  assert.ok(!maybe.includes("'fresh'"));
  assert.ok(maybe.includes('captureReconnectViewportAnchor'));
  const push = extractFn(boot, 'pushLiveTickDelta');
  assert.ok(push.indexOf('appendTick') < push.indexOf('maybeSparseHistoryToLivePaint'));
  assert.ok(push.indexOf('maybeSparseHistoryToLivePaint') < push.indexOf('shouldMarkDirtyLiveDelta'));
});

test('scope: sparse class only; no 1s special-case; camera math untouched', () => {
  const maybe = extractFn(boot, 'maybeSparseHistoryToLivePaint');
  assert.ok(maybe.includes('sparse !== true') || maybe.includes('sparse === true'));
  assert.ok(!boot.includes("currentTf === '1s'"));
  const camera = fs.readFileSync(path.join(__dirname, 'ui', 'time-camera.js'), 'utf8');
  assert.ok(!camera.includes('maybeSparseHistoryToLivePaint'));
  assert.ok(!camera.includes('sparseHistoryToLiveNeedsFullPaint'));
});

console.log('sparse_offscreen_paint_test: ALL PASS');
