/**
 * Wave 2 — pending left-history intent (not a queue).
 */
const assert = require('assert');
const { HydrationOrchestrator } = require('./hydration-orchestrator.js');

function test(name, fn) {
  fn();
  console.log(`OK ${name}`);
}

test('busy noteLeftHistoryIntent remembers newest; tryConsume starts once idle', async () => {
  const orch = new HydrationOrchestrator();
  let fetchCount = 0;
  let renderBusy = true;
  let should = true;
  orch.init({
    getEpoch: () => 1,
    getReqId: () => 1,
    shouldLoad: () => should,
    getAnchorEndTimeSec: () => 1000,
    isRenderBusy: () => renderBusy,
    isDashboardLoading: () => false,
    getVisibleRange: () => ({ from: 10, to: 80 }),
    fetchColumnar: async () => {
      fetchCount += 1;
      return { times: [1, 2, 3], hasMore: true, candles: { open: [], high: [], low: [], close: [], volume: [] } };
    },
    mergeIntoStore: () => ({ added: 3 }),
    markDirty: () => {},
    processTick: () => {},
  });

  orch.noteLeftHistoryIntent({ from: 5, to: 60 });
  orch.noteLeftHistoryIntent({ from: 2, to: 55 });
  assert.strictEqual(orch.hasPendingLeftIntent(), true);
  assert.strictEqual(fetchCount, 0);

  renderBusy = false;
  orch.tryConsumePending();
  // debounce 200ms
  await new Promise((r) => setTimeout(r, 250));
  assert.ok(fetchCount >= 1);
});

test('rapid notes supersede — only one pending slot', () => {
  const orch = new HydrationOrchestrator();
  orch.init({
    getEpoch: () => 1,
    shouldLoad: () => true,
    getAnchorEndTimeSec: () => 1000,
    isRenderBusy: () => true,
    isDashboardLoading: () => false,
    getVisibleRange: () => null,
    fetchColumnar: async () => ({ times: [] }),
    mergeIntoStore: () => null,
    markDirty: () => {},
    processTick: () => {},
  });
  orch.noteLeftHistoryIntent({ from: 40, to: 100 });
  orch.noteLeftHistoryIntent({ from: 10, to: 90 });
  orch.noteLeftHistoryIntent({ from: 1, to: 80 });
  assert.strictEqual(orch.hasPendingLeftIntent(), true);
  assert.strictEqual(orch._pendingLeftIntent.range.from, 1);
});

test('reset cancels pending (epoch/TF)', () => {
  const orch = new HydrationOrchestrator();
  orch.init({
    getEpoch: () => 1,
    shouldLoad: () => true,
    getAnchorEndTimeSec: () => 1000,
    isRenderBusy: () => true,
    fetchColumnar: async () => ({ times: [] }),
    mergeIntoStore: () => null,
    markDirty: () => {},
    processTick: () => {},
  });
  orch.noteLeftHistoryIntent({ from: 1, to: 50 });
  assert.ok(orch.hasPendingLeftIntent());
  orch.reset();
  assert.strictEqual(orch.hasPendingLeftIntent(), false);
});

test('shouldLoad false while idle cancels pending (not busy-drop)', () => {
  const orch = new HydrationOrchestrator();
  orch.init({
    getEpoch: () => 1,
    shouldLoad: () => false,
    getAnchorEndTimeSec: () => 1000,
    isRenderBusy: () => false,
    isDashboardLoading: () => false,
    getVisibleRange: () => ({ from: 100, to: 200 }),
    fetchColumnar: async () => ({ times: [] }),
    mergeIntoStore: () => null,
    markDirty: () => {},
    processTick: () => {},
  });
  orch._pendingLeftIntent = { range: { from: 100, to: 200 }, options: {} };
  orch.tryConsumePending();
  assert.strictEqual(orch.hasPendingLeftIntent(), false);
});

test('boot scheduleHistoryLoad must not busy-drop (source gate)', () => {
  const fs = require('fs');
  const path = require('path');
  const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
  const m = boot.match(/function scheduleHistoryLoad\s*\([^)]*\)\s*\{[\s\S]*?\n  \}/);
  assert.ok(m, 'scheduleHistoryLoad missing');
  const body = m[0];
  assert.ok(/noteLeftHistoryIntent/.test(body), 'Boot must forward to noteLeftHistoryIntent');
  assert.ok(!/isBusy\(\)/.test(body), 'Boot must not gate on isBusy (Hydration owns pending)');
  assert.ok(!/liveRenderScheduler\?\.isBusy/.test(body), 'Boot must not drop on scheduler busy');
});

console.log('wave2_pending_intent_test: ALL PASS');
