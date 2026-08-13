/**
 * Right-edge history island fill after Microscope TF center hydrate.
 * Run: node web/viewport_right_edge_append_test.js
 */
'use strict';

const assert = require('assert');
const { ColumnarStore } = require('./columnar-store.js');
const { HydrationOrchestrator } = require('./hydration-orchestrator.js');
const ViewportManager = require('./ui/viewport-manager.js');
const TimeCamera = require('./ui/time-camera.js');

globalThis.TimeCamera = TimeCamera;

function test(name, fn) {
  return Promise.resolve()
    .then(fn)
    .then(() => console.log('OK', name));
}

/** 2026-05-30 12:00 UTC */
const CENTER_MS = Date.UTC(2026, 4, 30, 12, 0, 0);
const NOW_SEC = Math.floor(Date.UTC(2026, 7, 10, 12, 0, 0) / 1000);

function makeIslandStore(n = 100, startSec = Math.floor(CENTER_MS / 1000) - 50 * 60) {
  const times = Array.from({ length: n }, (_, i) => startSec + i * 60);
  const store = new ColumnarStore();
  store.replaceMonolith({
    times,
    candles: {
      open: times.map(() => 1),
      high: times.map(() => 1),
      low: times.map(() => 1),
      close: times.map(() => 1),
      volume: times.map(() => 1),
    },
    plots: {},
    hasMore: true,
  }, { commitPaired: true });
  return store;
}

async function run() {
  await test('TF center hydrate unchanged (not tip, not stuck into right fetch)', () => {
    const end = ViewportManager.resolveHistoryTfFetchEndSec({
      intent: 'HISTORY',
      centerTimeMs: CENTER_MS,
      nowSec: NOW_SEC,
      limit: 3000,
      intervalSec: 60,
    });
    const centerSec = Math.floor(CENTER_MS / 1000);
    assert.strictEqual(end, centerSec + 1500 * 60);
    assert.ok(end < NOW_SEC - 86400);
  });

  await test('resolveRightHistoryFetchEndSec uses store tip, NOT TF center', () => {
    const last = Math.floor(CENTER_MS / 1000) + 1500 * 60; // island right edge ~May 31
    const end = ViewportManager.resolveRightHistoryFetchEndSec({
      lastTimeSec: last,
      nowSec: NOW_SEC,
      limit: 3000,
      intervalSec: 60,
    });
    assert.ok(end != null);
    // Must advance past island tip — not re-center on May 30.
    assert.ok(end > last);
    assert.strictEqual(end, Math.min(last + 3000 * 60, NOW_SEC));
    const centerSec = Math.floor(CENTER_MS / 1000);
    assert.ok(end !== centerSec + 1500 * 60, 'must not reuse TF-switch center window');
  });

  await test('appendMonolith adds only bars after tip', () => {
    const store = makeIslandStore(10, 1_700_000_000);
    const tip = store.lastTimeSec();
    const incoming = {
      times: [tip - 60, tip, tip + 60, tip + 120],
      candles: {
        open: [1, 1, 1, 1],
        high: [1, 1, 1, 1],
        low: [1, 1, 1, 1],
        close: [1, 1, 1, 1],
        volume: [1, 1, 1, 1],
      },
      plots: {},
    };
    const { added } = store.appendMonolith(incoming);
    assert.strictEqual(added, 2);
    assert.strictEqual(store.lastTimeSec(), tip + 120);
    assert.strictEqual(store.barCount(), 12);
  });

  await test('right-edge approach → fetch end after store tip (not center)', async () => {
    const store = makeIslandStore(3000);
    const islandLast = store.lastTimeSec();
    let fetchedEnd = null;
    let appendCalls = 0;
    const orch = new HydrationOrchestrator();
    orch.debounceMs = 0;
    orch.init({
      getEpoch: () => 1,
      getReqId: () => 1,
      getHistoryHasMore: () => true,
      setHistoryHasMore: () => {},
      isRenderBusy: () => false,
      isDashboardLoading: () => false,
      getVisibleRange: () => ({ from: 2900, to: 2990 }),
      getAnchorEndTimeSec: () => store.firstTimeSec(),
      getRightTipSec: () => store.lastTimeSec(),
      shouldLoad: () => false,
      shouldLoadRight: (range) => Number(range.to) >= store.barCount() - 1 - 50,
      shouldContinueRightHistory: () => false,
      getRightFetchEndSec: () => ViewportManager.resolveRightHistoryFetchEndSec({
        lastTimeSec: store.lastTimeSec(),
        nowSec: NOW_SEC,
        limit: 3000,
        intervalSec: 60,
      }),
      fetchColumnar: async (endTimeSec) => {
        fetchedEnd = endTimeSec;
        const start = endTimeSec - 3000 * 60;
        const times = [];
        for (let t = start; t <= endTimeSec; t += 60) times.push(t);
        return {
          times,
          candles: {
            open: times.map(() => 1),
            high: times.map(() => 1),
            low: times.map(() => 1),
            close: times.map(() => 1),
            volume: times.map(() => 1),
          },
          plots: {},
          hasMore: true,
        };
      },
      mergeIntoStore: () => null,
      mergeAppendIntoStore: (data) => {
        appendCalls += 1;
        const { added } = store.appendMonolith(data);
        return added > 0 ? { added } : null;
      },
      markDirty: () => {},
      processTick: () => {},
    });

    orch.noteRightHistoryIntent({ from: 2900, to: 2990 }, { force: true });
    await new Promise((r) => setTimeout(r, 20));

    assert.ok(fetchedEnd != null, 'right fetch must run');
    assert.ok(fetchedEnd > islandLast, 'fetch must extend past island tip');
    const centerWindow = Math.floor(CENTER_MS / 1000) + 1500 * 60;
    assert.ok(fetchedEnd !== centerWindow, 'must not re-fetch TF center window');
    assert.ok(appendCalls >= 1);
    assert.ok(store.lastTimeSec() > islandLast, 'store tip must advance toward live');
  });

  await test('left historyHasMore=false must NOT block right append', async () => {
    const store = makeIslandStore(100);
    let rightFetch = 0;
    const orch = new HydrationOrchestrator();
    orch.debounceMs = 0;
    orch.init({
      getEpoch: () => 1,
      getReqId: () => 1,
      getHistoryHasMore: () => false, // left EOF
      setHistoryHasMore: () => {},
      isRenderBusy: () => false,
      isDashboardLoading: () => false,
      getVisibleRange: () => ({ from: 40, to: 95 }),
      getAnchorEndTimeSec: () => store.firstTimeSec(),
      getRightTipSec: () => store.lastTimeSec(),
      shouldLoad: () => false,
      shouldLoadRight: () => true,
      shouldContinueRightHistory: () => false,
      getRightFetchEndSec: () => store.lastTimeSec() + 100 * 60,
      fetchColumnar: async (end) => {
        rightFetch += 1;
        const tip = store.lastTimeSec();
        const times = [tip + 60, tip + 120];
        return {
          times,
          candles: {
            open: [1, 1], high: [1, 1], low: [1, 1], close: [1, 1], volume: [1, 1],
          },
          plots: {},
        };
      },
      mergeIntoStore: () => null,
      mergeAppendIntoStore: (data) => {
        const { added } = store.appendMonolith(data);
        return added > 0 ? { added } : null;
      },
      markDirty: () => {},
      processTick: () => {},
    });

    orch.noteRightHistoryIntent({ from: 40, to: 95 }, { force: true });
    await new Promise((r) => setTimeout(r, 20));
    assert.strictEqual(rightFetch, 1);
  });

  function makeTipOrch(state) {
    const orch = new HydrationOrchestrator();
    orch.debounceMs = 0;
    orch.init({
      getEpoch: () => 1,
      getReqId: () => 1,
      getHistoryHasMore: () => true,
      setHistoryHasMore: () => {},
      isRenderBusy: () => false,
      isDashboardLoading: () => false,
      getVisibleRange: () => ({ from: 40, to: 95 }),
      getAnchorEndTimeSec: () => 1,
      getRightTipSec: () => state.tip,
      shouldLoad: () => false,
      shouldLoadRight: () => true,
      shouldContinueRightHistory: () => state.continueRight === true && state.fetchCount < 2,
      getRightFetchEndSec: () => state.fetchEnd,
      fetchColumnar: async (endTimeSec) => {
        state.fetchCount += 1;
        state.lastFetchedEnd = endTimeSec;
        return {
          times: [state.tip + 60],
          candles: {
            open: [1], high: [1], low: [1], close: [1], volume: [1],
          },
          plots: {},
          hasMore: true,
        };
      },
      mergeIntoStore: () => null,
      mergeAppendIntoStore: () => state.merge(),
      markDirty: () => { state.dirty += 1; },
      processTick: () => {},
    });
    return orch;
  }

  await test('A. real tip advancement is success and may continue', async () => {
    const state = {
      tip: 100,
      fetchEnd: 1781360100,
      fetchCount: 0,
      lastFetchedEnd: null,
      dirty: 0,
      continueRight: true,
      merge: () => {
        state.tip += 100;
        return { added: 10 };
      },
    };
    const orch = makeTipOrch(state);
    orch.noteRightHistoryIntent({ from: 40, to: 95 }, { force: true });
    await new Promise((r) => setTimeout(r, 20));
    assert.ok(state.fetchCount >= 2, 'real progress may continue RIGHT');
    assert.ok(state.tip > 100);
    assert.ok(state.dirty >= 1, 'real progress must paint');
    assert.strictEqual(orch.isRightTipBlocked(), false);
  });

  await test('B. added>0 but tip unchanged is zero-progress (no paint, no continue)', async () => {
    const state = {
      tip: 100,
      fetchEnd: 1781360100,
      fetchCount: 0,
      lastFetchedEnd: null,
      dirty: 0,
      continueRight: true,
      merge: () => ({ added: 3000 }),
    };
    const orch = makeTipOrch(state);
    orch.noteRightHistoryIntent({ from: 40, to: 95 }, { force: true });
    await new Promise((r) => setTimeout(r, 20));
    assert.strictEqual(state.fetchCount, 1);
    assert.strictEqual(state.tip, 100);
    assert.strictEqual(state.dirty, 0, 'fake success must not markDirty');
    assert.strictEqual(orch.isRightTipBlocked(), true);
    orch.noteRightHistoryIntent({ from: 40, to: 95 }, { force: true });
    await new Promise((r) => setTimeout(r, 20));
    assert.strictEqual(state.fetchCount, 1, 'blocked cursor must not refetch');
  });

  await test('C. empty merge still Fix E zero-add', async () => {
    const state = {
      tip: 100,
      fetchEnd: 1781360100,
      fetchCount: 0,
      lastFetchedEnd: null,
      dirty: 0,
      continueRight: true,
      merge: () => ({ added: 0 }),
    };
    const orch = makeTipOrch(state);
    orch.noteRightHistoryIntent({ from: 40, to: 95 }, { force: true });
    await new Promise((r) => setTimeout(r, 20));
    assert.strictEqual(state.fetchCount, 1);
    assert.strictEqual(state.dirty, 0);
    assert.strictEqual(orch.isRightTipBlocked(), true);
    orch.noteRightHistoryIntent({ from: 40, to: 95 }, { force: true });
    await new Promise((r) => setTimeout(r, 20));
    assert.strictEqual(state.fetchCount, 1);
  });

  await test('D. backward tip is no RIGHT progress', async () => {
    const state = {
      tip: 200,
      fetchEnd: 1781360100,
      fetchCount: 0,
      lastFetchedEnd: null,
      dirty: 0,
      continueRight: true,
      merge: () => {
        state.tip = 100;
        return { added: 5 };
      },
    };
    const orch = makeTipOrch(state);
    orch.noteRightHistoryIntent({ from: 40, to: 95 }, { force: true });
    await new Promise((r) => setTimeout(r, 20));
    assert.strictEqual(state.fetchCount, 1);
    assert.strictEqual(state.dirty, 0);
    assert.strictEqual(orch.isRightTipBlocked(), true);
    orch.noteRightHistoryIntent({ from: 40, to: 95 }, { force: true });
    await new Promise((r) => setTimeout(r, 20));
    assert.strictEqual(state.fetchCount, 1);
  });

  await test('E. now-clamped identical RIGHT request blocked when tip did not move', async () => {
    const nowClamped = 1781360100;
    const state = {
      tip: 100,
      fetchEnd: nowClamped,
      fetchCount: 0,
      lastFetchedEnd: null,
      dirty: 0,
      continueRight: true,
      merge: () => ({ added: 3000 }),
    };
    const orch = makeTipOrch(state);
    orch.noteRightHistoryIntent({ from: 40, to: 95 }, { force: true });
    await new Promise((r) => setTimeout(r, 20));
    assert.strictEqual(state.lastFetchedEnd, nowClamped);
    assert.strictEqual(state.fetchCount, 1);
    orch.noteRightHistoryIntent({ from: 40, to: 95 }, { force: true });
    await new Promise((r) => setTimeout(r, 20));
    assert.strictEqual(state.fetchCount, 1);
    assert.strictEqual(state.lastFetchedEnd, nowClamped);
    assert.strictEqual(orch.isRightTipBlocked(), true);
  });

  console.log('All right-edge append tests passed.');
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
