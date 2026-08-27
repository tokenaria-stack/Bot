/**
 * SECONDS-HISTORY — 5s–45s bidirectional history (Node source contracts).
 * Run: node web/seconds_history_nav_test.js
 */
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

const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
const cfg = fs.readFileSync(path.join(__dirname, 'config.js'), 'utf8');
const orch = fs.readFileSync(path.join(__dirname, 'hydration-orchestrator.js'), 'utf8');
const tf = fs.readFileSync(path.join(__dirname, 'ui/timeframe-controller.js'), 'utf8');

assert.ok(/=== '1s'/.test(extractFn(cfg, 'isLiveSecondChart')), 'isLiveSecondChart stays 1s-only');
assert.ok(/function isSparseSecondChart/.test(cfg), 'sparse-second child gate exists');
const sparseGate = extractFn(cfg, 'isSparseSecondChart');
for (const id of ['5s', '10s', '15s', '30s', '45s']) {
  assert.ok(sparseGate.includes(`'${id}'`), `gate lists ${id}`);
}
assert.ok(!sparseGate.includes("'1s'"), 'gate must not include 1s');

const right = extractFn(boot, 'canExtendHistoryRight');
assert.ok(/isSecondsHistoryNavChart/.test(right), 'right extend includes sparse-second children');
assert.ok(/historyHasNewer !== false/.test(right), 'hasNewer=false stops right pages');
assert.ok(!/isSparseLiveChart/.test(right), 'must not unlock all sparse TFs');

const cursor = extractFn(boot, 'sparseChildRightCursorSec');
assert.ok(/open \+ iv - 1/.test(cursor), 'right cursor is child CloseTime, not OpenTime');

assert.ok(/parentResumeAfterSec/.test(boot) && /onSparseRightNoProgress/.test(boot),
  'zero-child page continues from parent watermark');
assert.ok(/parentResumeAfterSec/.test(fs.readFileSync(path.join(__dirname, 'api.js'), 'utf8')),
  'API carries parentResumeAfterSec');
assert.ok(/action === 'continue'/.test(orch), 'watermark advance must not latch the child tip');

const load = boot;
const newerAt = load.indexOf('historyHasNewer = columnar.hasNewer');
const flushAt = load.indexOf('flushLiveTickBuffer();');
assert.ok(/includeForming: !historyIsland/.test(boot),
  'HISTORY sparse-child hydrate is closed-only');
assert.ok(/includeForming: false/.test(boot),
  'sparse-child prepend/append never request live forming');
assert.ok(/includeForming/.test(fs.readFileSync(path.join(__dirname, 'api.js'), 'utf8')),
  'API carries includeForming');
assert.ok(/rightEmptyClearsDetached/.test(boot) && /onRightSourceTail/.test(boot),
  'forming-only 1s tail clears detached even if folded added==0');
assert.ok(/rightEmptyClearsDetached: \(\) => isSecondsHistoryNavChart/.test(boot),
  '1s and sparse-second children both clear detach on hasNewer=false');
assert.ok(/_rightReachedSourceTail/.test(orch), 'orchestrator honors source-tail empty right page');
const tailFn = orch.indexOf('_clearRightDetachedOnSourceTail');
assert.ok(tailFn >= 0, 'missing _clearRightDetachedOnSourceTail');
assert.ok(!/TimeCamera/.test(orch.slice(tailFn, tailFn + 280)),
  'reaching 1s tail must not move TimeCamera');

assert.ok(/isSparseSecondChart/.test(tf) && /returnToLive\(\)/.test(tf),
  'same-TF sparse-second click is explicit RTL');

const rtl = extractFn(boot, 'returnToLive');
assert.ok(/userReturnToLive: true/.test(rtl), 'RTL is latest-tail hydrate');
assert.ok(!/reloadDashboard/.test(rtl) && !/cache\/clear/.test(rtl), 'RTL must not clear HTF cache');

const pushStart = boot.indexOf('function pushLiveTickDelta');
assert.ok(pushStart >= 0, 'missing pushLiveTickDelta');
const push = boot.slice(pushStart, boot.indexOf('function liveCameraViewIntent', pushStart));
assert.ok(/isSecondsHistoryNavChart/.test(push) && /historyHasNewer === true/.test(push),
  'detached sparse-second island must not ingest live ticks');
assert.ok(push.indexOf('historyHasNewer') < push.indexOf('appendTick'),
  'detach gate is before appendTick');

console.log('seconds_history_nav_test: OK');
