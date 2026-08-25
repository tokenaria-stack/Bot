/**
 * MICRO-HISTORY-1 — 1s bidirectional history (Node source contracts).
 * Run: node web/micro_history_nav_test.js
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
const api = fs.readFileSync(path.join(__dirname, 'api.js'), 'utf8');
const orch = fs.readFileSync(path.join(__dirname, 'hydration-orchestrator.js'), 'utf8');
const cfg = fs.readFileSync(path.join(__dirname, 'config.js'), 'utf8');
const tf = fs.readFileSync(path.join(__dirname, 'ui/timeframe-controller.js'), 'utf8');

assert.ok(/function isLiveSecondChart/.test(cfg), '1s capability is not all-sparse');
assert.ok(/=== '1s'/.test(extractFn(cfg, 'isLiveSecondChart')), 'gate is explicit 1s');

const right = extractFn(boot, 'canExtendHistoryRight');
assert.ok(/isLiveSecondChart/.test(right), 'right extend allowed for 1s');
assert.ok(/historyHasNewer !== false/.test(right), 'hasNewer=false stops right pages');
assert.ok(!/isSparseLiveChart/.test(right), 'must not unlock all sparse TFs');

assert.ok(/startTimeSec/.test(api), 'API accepts startTime cursor');
assert.ok(/exactly one of endTimeSec or startTimeSec/.test(api), 'XOR cursor on client');

assert.ok(/fetchRightColumnar/.test(orch), 'append uses optional right fetch');
assert.ok(/startTimeSec: cursorSec/.test(boot), '1s right fetch is startTime');

const rtl = extractFn(boot, 'returnToLive');
assert.ok(/userReturnToLive: true/.test(rtl), 'RTL is latest-tail hydrate');
assert.ok(/intent: 'LIVE'/.test(rtl), 'RTL moves VIEW to LIVE');
assert.ok(!/reloadDashboard/.test(rtl) && !/cache\/clear/.test(rtl), 'RTL must not clear HTF cache');

assert.ok(/options\.userReturnToLive === true/.test(boot), 'loadDashboard honors RTL jump');
assert.ok(/historyHasNewer = columnar.hasNewer === true/.test(boot), 'live tail publishes hasNewer');

assert.ok(/prunedRightCount/.test(boot) && /historyHasNewer = true/.test(boot),
  'left prune of live tip re-opens right paging');

assert.ok(/returnToLive\(\)/.test(tf), 'same-TF 1s click is explicit RTL');

assert.ok(!/TimeCamera/.test(extractFn(boot, 'canExtendHistoryRight')),
  'hasNewer is not a camera fact');

console.log('micro_history_nav_test: OK');
