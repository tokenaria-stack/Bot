/**
 * Wave 3 — completion outcomes: EOF only from authoritative hasMore === false.
 */
const assert = require('assert');
const { HydrationOrchestrator } = require('./hydration-orchestrator.js');

function test(name, fn) {
  return Promise.resolve()
    .then(fn)
    .then(() => console.log(`OK ${name}`));
}

function makeOrch(overrides = {}) {
  const orch = new HydrationOrchestrator();
  let hasMore = true;
  let headSec = 1000;
  const state = {
    hasMore: () => hasMore,
    setHasMore: (v) => { hasMore = !!v; },
    fetchCount: 0,
    fetchImpl: async () => ({ times: [1, 2], hasMore: true }),
    mergeImpl: () => ({ added: 2 }),
    setHead: (v) => { headSec = v; },
    head: () => headSec,
  };
  orch.init({
    getEpoch: () => 1,
    getReqId: () => 1,
    shouldLoad: () => true,
    getAnchorEndTimeSec: () => headSec,
    isRenderBusy: () => false,
    isDashboardLoading: () => false,
    getVisibleRange: () => ({ from: 5, to: 50 }),
    fetchColumnar: async (...args) => {
      state.fetchCount += 1;
      return state.fetchImpl(...args);
    },
    mergeIntoStore: (...args) => state.mergeImpl(...args),
    setHistoryHasMore: (v) => { hasMore = !!v; },
    getHistoryHasMore: () => hasMore,
    markDirty: () => {},
    processTick: () => {},
    ...overrides,
  });
  // expose for tests
  orch._test = state;
  Object.defineProperty(state, 'hasMoreFlag', {
    get: () => hasMore,
    set: (v) => { hasMore = v; },
  });
  return orch;
}

async function run() {
  await test('true EOF after successful merge clears hasMore and pending', async () => {
    const orch = makeOrch();
    orch._test.fetchImpl = async () => ({
      times: [1, 2, 3],
      hasMore: false,
      candles: {},
    });
    orch._test.mergeImpl = () => ({ added: 3 });
    orch._pendingLeftIntent = { range: { from: 1, to: 40 }, options: {} };
    await orch.requestPrepend({ from: 1, to: 40 }, {});
    assert.strictEqual(orch._test.hasMoreFlag, false);
    assert.strictEqual(orch.hasPendingLeftIntent(), false);
  });

  await test('empty payload without hasMore false does not set EOF', async () => {
    const orch = makeOrch();
    orch._test.hasMoreFlag = true;
    orch._test.fetchImpl = async () => ({ times: [], hasMore: true });
    await orch.requestPrepend({ from: 1, to: 40 }, {});
    assert.strictEqual(orch._test.hasMoreFlag, true);
  });

  await test('empty payload with authoritative hasMore false is EOF', async () => {
    const orch = makeOrch();
    orch._test.hasMoreFlag = true;
    orch._pendingLeftIntent = { range: { from: 1, to: 40 }, options: {} };
    orch._test.fetchImpl = async () => ({ times: [], hasMore: false });
    await orch.requestPrepend({ from: 1, to: 40 }, {});
    assert.strictEqual(orch._test.hasMoreFlag, false);
    assert.strictEqual(orch.hasPendingLeftIntent(), false);
  });

  await test('zero-overlap does not permanently disable history (not EOF)', async () => {
    const orch = makeOrch();
    orch._test.hasMoreFlag = true;
    orch._test.fetchImpl = async () => ({ times: [1, 2, 3], hasMore: true });
    orch._test.mergeImpl = () => ({ added: 0 });
    await orch.requestPrepend({ from: 1, to: 40 }, {});
    assert.strictEqual(orch._test.hasMoreFlag, true);
  });

  await test('fetch throw does not masquerade as EOF', async () => {
    const orch = makeOrch();
    orch._test.hasMoreFlag = true;
    orch._test.fetchImpl = async () => { throw new Error('network'); };
    await orch.requestPrepend({ from: 1, to: 40 }, {});
    assert.strictEqual(orch._test.hasMoreFlag, true);
  });

  await test('A: zero-add blocks identical retry at the same head', async () => {
    const orch = makeOrch();
    orch._test.mergeImpl = () => ({ added: 0 });
    await orch.requestPrepend({ from: 1, to: 40 }, {});
    assert.strictEqual(orch._test.fetchCount, 1);
    await orch.requestPrepend({ from: 1, to: 40 }, {});
    assert.strictEqual(orch._test.fetchCount, 1, 'same head must not refetch');
    assert.strictEqual(orch.isLeftHeadBlocked(), true);
  });

  await test('B: new head clears the zero-progress block', async () => {
    const orch = makeOrch();
    orch._test.mergeImpl = () => ({ added: 0 });
    await orch.requestPrepend({ from: 1, to: 40 }, {});
    assert.strictEqual(orch._test.fetchCount, 1);

    orch._test.setHead(900);
    orch._test.mergeImpl = () => ({ added: 3 });
    await orch.requestPrepend({ from: 1, to: 40 }, {});
    assert.strictEqual(orch._test.fetchCount, 2, 'moved head must be fetchable');
    assert.strictEqual(orch.isLeftHeadBlocked(), false);
  });

  await test('C: range echo cannot replay the stalled cursor', async () => {
    const orch = makeOrch();
    orch._test.mergeImpl = () => ({ added: 0 });
    await orch.requestPrepend({ from: 1, to: 40 }, {});
    orch.noteLeftHistoryIntent({ from: 2, to: 41 }, { force: true });
    orch.tryConsumePending();
    await new Promise((r) => setTimeout(r, 50));
    assert.strictEqual(orch._test.fetchCount, 1);
    assert.strictEqual(orch.hasPendingLeftIntent(), false);
  });

  await test('D: zero-add is not EOF', async () => {
    const orch = makeOrch();
    orch._test.hasMoreFlag = true;
    orch._test.mergeImpl = () => ({ added: 0 });
    await orch.requestPrepend({ from: 1, to: 40 }, {});
    assert.strictEqual(orch._test.hasMoreFlag, true);
  });

  await test('recoverable overlap preserves ability to note pending (Wave 2)', async () => {
    const orch = makeOrch();
    orch._test.hasMoreFlag = true;
    orch._test.fetchImpl = async () => ({ times: [1], hasMore: true });
    orch._test.mergeImpl = () => null;
    await orch.requestPrepend({ from: 1, to: 40 }, {});
    assert.strictEqual(orch._test.hasMoreFlag, true);
    orch.noteLeftHistoryIntent({ from: 2, to: 41 });
    // Same head after zero-add: do not re-arm fetch (Fix E). hasMore still true.
    assert.strictEqual(orch.hasPendingLeftIntent(), false);
    assert.strictEqual(orch._test.fetchCount, 1);
  });

  await test('source gate: zero-overlap branch must not call setHistoryHasMore(false)', () => {
    const fs = require('fs');
    const path = require('path');
    const src = fs.readFileSync(path.join(__dirname, 'hydration-orchestrator.js'), 'utf8');
    assert.ok(/zero overlap \(recoverable, not EOF\)/.test(src));
    // No forced false immediately after zero-overlap warn
    const idx = src.indexOf('zero overlap (recoverable, not EOF)');
    const slice = src.slice(idx, idx + 120);
    assert.ok(!/setHistoryHasMore\(false\)/.test(slice));
  });

  console.log('wave3_completion_outcome_test: ALL PASS');
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
