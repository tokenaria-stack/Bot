/**
 * SECONDS-TF-SWITCH — asymmetric seconds TF (loupe down, map up).
 * Run: node web/seconds_tf_switch_test.js
 */
'use strict';

const fs = require('fs');
const path = require('path');
const assert = require('assert');

function extractFn(src, name) {
  const start = src.search(new RegExp(`(?:async\\s+)?function ${name}\\b`));
  assert.ok(start >= 0, `missing ${name}`);
  let i = src.indexOf('{', start);
  let depth = 0;
  for (; i < src.length; i++) {
    if (src[i] === '{') depth += 1;
    else if (src[i] === '}') {
      depth -= 1;
      if (depth === 0) return src.slice(start, i + 1);
    }
  }
  throw new Error(`unclosed ${name}`);
}

const cfg = fs.readFileSync(path.join(__dirname, 'config.js'), 'utf8');
const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
const tfCtl = fs.readFileSync(path.join(__dirname, 'ui/timeframe-controller.js'), 'utf8');

const vm = require('vm');
const sandbox = {};
vm.createContext(sandbox);
for (const name of [
  'isLiveSecondChart',
  'isSparseSecondChart',
  'isSecondsFamilyChart',
  'secondsFamilyIntervalSec',
  'applySecondsFamilyTfSwitchIntent',
]) {
  vm.runInContext(extractFn(cfg, name), sandbox);
}

const {
  secondsFamilyIntervalSec,
  applySecondsFamilyTfSwitchIntent,
} = sandbox;

assert.strictEqual(secondsFamilyIntervalSec('1s'), 1);
assert.strictEqual(secondsFamilyIntervalSec('5s'), 5);
assert.strictEqual(secondsFamilyIntervalSec('45s'), 45);
assert.strictEqual(secondsFamilyIntervalSec('1m'), null);
assert.ok(secondsFamilyIntervalSec('10s') < secondsFamilyIntervalSec('15s'));

const histSeed = {
  intent: 'HISTORY',
  centerTimeMs: 1_700_000_000_000,
  visibleBars: 2000,
  barSpacing: 3,
  isAtRightEdge: false,
};

{
  const live = applySecondsFamilyTfSwitchIntent({ ...histSeed, intent: 'LIVE' }, '1s', '5s');
  assert.strictEqual(live.intent, 'LIVE', 'LIVE seed stays LIVE');
}

{
  const m = applySecondsFamilyTfSwitchIntent(histSeed, '5s', '1s');
  assert.strictEqual(m.intent, 'HISTORY', '5s HISTORY → 1s is microscope');
  assert.strictEqual(m.centerTimeMs, histSeed.centerTimeMs);
  assert.strictEqual(m.visibleBars, 2000);
}

{
  const m = applySecondsFamilyTfSwitchIntent(histSeed, '45s', '5s');
  assert.strictEqual(m.intent, 'HISTORY', '45s HISTORY → 5s is microscope');
}

{
  const live = applySecondsFamilyTfSwitchIntent(histSeed, '1s', '5s');
  assert.strictEqual(live.intent, 'LIVE', '1s HISTORY → 5s is LIVE');
  assert.strictEqual(live.isAtRightEdge, true);
  assert.strictEqual(live.visibleBars, 2000, 'bar geometry kept');
}

{
  const live = applySecondsFamilyTfSwitchIntent(histSeed, '5s', '30s');
  assert.strictEqual(live.intent, 'LIVE', '5s HISTORY → 30s is LIVE');
}

{
  const keep = applySecondsFamilyTfSwitchIntent(histSeed, '1s', '1m');
  assert.strictEqual(keep.intent, 'HISTORY', 'native target is out of scope');
}

{
  const keep = applySecondsFamilyTfSwitchIntent(histSeed, '1m', '5m');
  assert.strictEqual(keep.intent, 'HISTORY', 'native pair is out of scope');
}

assert.ok(/applySecondsFamilyTfSwitchIntent\(viewportAnchor, prevTf, resolved\)/.test(tfCtl),
  'live TF switch applies seconds-family law');
assert.ok(/returnToLive\(\)/.test(extractFn(tfCtl, 'switchLiveTimeframe')),
  'same-TF seconds click remains RTL');

const contL = extractFn(boot, 'initHydrationOrchestrator');
assert.ok(/shouldContinueLeftHistory[\s\S]*isSecondsHistoryNavChart[\s\S]*liveHistoryScrollArmed/.test(contL),
  'seconds left continuation requires user-scroll arm');
assert.ok(/shouldContinueRightHistory[\s\S]*isSecondsHistoryNavChart[\s\S]*liveHistoryScrollArmed/.test(contL),
  'seconds right continuation requires user-scroll arm');

const arm = extractFn(boot, 'attachLiveHistoryScrollArm');
assert.ok(/addEventListener\('wheel'/.test(arm) && /addEventListener\('pointerdown'/.test(arm),
  'history arm is user gesture, not restore paint');
assert.ok(!/setTimeout/.test(extractFn(boot, 'scheduleHistoryLoad')),
  'no settle timer on prefetch');

console.log('seconds_tf_switch_test: ALL PASS');
