/**
 * History continuation after successful prepend.
 * Continue LEFT only while canonical VIEW is still near the loaded left edge
 * (same runway as prefetch). HISTORY / tip-outside-VIEW is not sufficient.
 */
const assert = require('assert');
const { HydrationOrchestrator } = require('./hydration-orchestrator.js');
const ViewportManager = require('./ui/viewport-manager.js');

function approachingLeft(range) {
  return ViewportManager.isWithinLeftEdgePrefetch(range, { hardMin: 50, frac: 0.25 });
}

function test(name, fn) {
  return Promise.resolve()
    .then(fn)
    .then(() => console.log(`OK ${name}`));
}

function makeOrch(overrides = {}) {
  const orch = new HydrationOrchestrator();
  let hasMore = true;
  let barCount = 3000;
  let visible = { from: 5, to: 80 };
  let renderBusy = false;
  const state = {
    fetchCount: 0,
    notes: [],
    mergeAdded: 2999,
    hasMore: () => hasMore,
    setHasMore: (v) => { hasMore = !!v; },
    getVisible: () => visible,
    setVisible: (r) => { visible = r; },
    setBarCount: (n) => { barCount = n; },
    setRenderBusy: (v) => { renderBusy = !!v; },
    fetchImpl: async () => ({
      times: Array.from({ length: 10 }, (_, i) => i + 1),
      hasMore: true,
      candles: {},
    }),
  };

  orch.init({
    getEpoch: () => 1,
    getReqId: () => 1,
    getHistoryHasMore: () => hasMore,
    setHistoryHasMore: (v) => { hasMore = !!v; },
    isRenderBusy: () => renderBusy,
    isDashboardLoading: () => false,
    getVisibleRange: () => visible,
    getAnchorEndTimeSec: () => 1_700_000_000,
    shouldLoad: (range, options = {}) => {
      if (!hasMore) return false;
      if (!range) return false;
      if (options.continuation === true) {
        return approachingLeft(range);
      }
      return range.from < 50;
    },
    shouldContinueLeftHistory: (range) => {
      if (!hasMore) return false;
      return approachingLeft(range);
    },
    fetchColumnar: async (...args) => {
      state.fetchCount += 1;
      return state.fetchImpl(...args);
    },
    mergeIntoStore: () => {
      barCount += state.mergeAdded;
      return { added: state.mergeAdded, viewportRange: visible };
    },
    markDirty: () => {},
    processTick: () => {},
    ...overrides,
  });

  const origNote = orch.noteLeftHistoryIntent.bind(orch);
  orch.noteLeftHistoryIntent = (range, options = {}) => {
    state.notes.push({ range: { ...range }, options: { ...options } });
    return origNote(range, options);
  };

  orch._test = state;
  Object.defineProperty(state, 'hasMoreFlag', {
    get: () => hasMore,
    set: (v) => { hasMore = v; },
  });
  Object.defineProperty(state, 'barCount', {
    get: () => barCount,
  });
  return orch;
}

async function waitDebounce(ms = 250) {
  await new Promise((r) => setTimeout(r, ms));
}

async function run() {
  await test('A: merge does not eager-continue; restored VIEW still at LEFT may load next page', async () => {
    const orch = makeOrch();
    let chunks = 0;
    orch._test.fetchImpl = async () => {
      chunks += 1;
      return {
        times: Array.from({ length: 10 }, (_, i) => i + 1),
        hasMore: chunks < 2,
        candles: {},
      };
    };

    await orch.requestPrepend({ from: 5, to: 80 }, {});
    assert.strictEqual(orch._test.fetchCount, 1, 'first prepend only');
    assert.ok(
      !orch._test.notes.some((n) => n.options.continuation === true),
      'must not merge-finally re-note continuation',
    );

    // Simulate post-restore: VIEW still at loaded left edge (canonical geometry).
    orch._test.setVisible({ from: 5, to: 80 });
    orch.noteLeftHistoryIntent({ from: 5, to: 80 }, { force: true });
    await waitDebounce(50);
    assert.ok(
      orch._test.fetchCount >= 2,
      `canonical post-restore LEFT may load next chunk (got fetchCount=${orch._test.fetchCount})`,
    );
    orch.reset();
  });

  await test('stale pending range must not override restored VIEW off the left edge', async () => {
    const orch = makeOrch();
    orch._test.fetchImpl = async () => ({
      times: [1, 2, 3],
      hasMore: true,
      candles: {},
    });
    orch._test.mergeAdded = 3;
    orch._test.setRenderBusy(true);
    orch.noteLeftHistoryIntent({ from: 5, to: 60 });
    assert.strictEqual(orch.hasPendingLeftIntent(), true);

    // Market-time restore shifted logical from; canonical VIEW is no longer at LEFT.
    orch._test.setVisible({ from: 3004, to: 3079 });
    orch._test.setRenderBusy(false);
    orch.tryConsumePending();
    await waitDebounce();

    assert.strictEqual(
      orch._test.fetchCount,
      0,
      'must not fetch LEFT from pre-restore pending geometry',
    );
    assert.strictEqual(orch.hasPendingLeftIntent(), false);
    orch.reset();
  });

  await test('B: historical middle (tip outside VIEW) does not continue LEFT', async () => {
    const orch = makeOrch();
    orch._test.setBarCount(25000);
    // Canonical VIEW is in the historical middle (tip still outside). Initial
    // prepend uses a left-edge request range; continuation must read live VIEW.
    orch._test.setVisible({ from: 6000, to: 24000 });
    await orch.requestPrepend({ from: 5, to: 80 }, {});
    const contNotes = orch._test.notes.filter((n) => n.options.continuation === true);
    assert.strictEqual(contNotes.length, 0, 'tip-outside-VIEW must not chain LEFT chunks');
    assert.strictEqual(orch._test.fetchCount, 1);
    assert.strictEqual(orch.hasPendingLeftIntent(), false);
    orch.reset();
  });

  await test('C: VIEW moves rightward → LEFT continuation stops', async () => {
    const orch = makeOrch();
    orch._test.setBarCount(25000);
    orch._test.setVisible({ from: 8000, to: 24000 });
    orch.noteLeftHistoryIntent({ from: 8000, to: 24000 }, { continuation: true });
    assert.strictEqual(orch.hasPendingLeftIntent(), false);
    assert.strictEqual(orch._test.fetchCount, 0);
    orch.reset();
  });

  await test('D: continuation stops at authoritative EOF', async () => {
    const orch = makeOrch();
    orch._test.fetchImpl = async () => ({
      times: [1, 2, 3],
      hasMore: false,
      candles: {},
    });
    orch._test.mergeAdded = 3;
    await orch.requestPrepend({ from: 5, to: 80 }, {});
    assert.strictEqual(orch._test.hasMoreFlag, false);
    assert.strictEqual(orch.hasPendingLeftIntent(), false);
    const contNotes = orch._test.notes.filter((n) => n.options.continuation === true);
    assert.strictEqual(contNotes.length, 0, 'EOF must not re-note continuation');
    orch.reset();
  });

  await test('continuation cancels when VIEW returns to live tip', async () => {
    const orch = makeOrch();
    orch._test.setBarCount(6000);
    orch._test.setVisible({ from: 5900, to: 5999 }); // tip in view, not near left
    orch.noteLeftHistoryIntent({ from: 5900, to: 5999 }, { continuation: true });
    assert.strictEqual(orch.hasPendingLeftIntent(), false);
    assert.strictEqual(orch._test.fetchCount, 0);
    orch.reset();
  });

  console.log('history_continuation_test: ALL PASS');
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
