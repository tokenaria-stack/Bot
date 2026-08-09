/**
 * History continuation after successful prepend.
 * Preserve remaps logical `from` upward; continuation must still be re-noted
 * and consumable via Wave 2 pending (no timers / poll / second owner).
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
        const tip = barCount - 1;
        return Number(range.to) < tip - 1.5;
      }
      return range.from < 50;
    },
    shouldContinueLeftHistory: (range) => {
      if (!hasMore) return false;
      if (!range) return true;
      const tip = barCount - 1;
      return Number(range.to) < tip - 1.5;
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
  await test('prepend → remapped from → continuation re-noted → next chunk consumed', async () => {
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
    assert.ok(orch._test.fetchCount >= 1);
    assert.ok(orch._test.barCount > 3000);
    assert.ok(
      orch._test.notes.some((n) => n.options.continuation === true && n.options.force === true),
      'successful prepend must re-note continuation with force (no debounce race)',
    );

    // Preserve remaps logical from (audit: 5 → ~3004) before/while chain continues.
    orch._test.setVisible({ from: 3004, to: 3079 });
    // force:true may already have started chunk 2; allow debounce-free settle.
    await waitDebounce(50);
    assert.ok(
      orch._test.fetchCount >= 2,
      `next chunk must be consumable after remapped from (got fetchCount=${orch._test.fetchCount})`,
    );
    orch.reset();
  });

  await test('preserve remap must not cancel pending left need noted during busy', async () => {
    const orch = makeOrch();
    // One shot — avoid post-success continuation keeping the event loop alive.
    orch._test.fetchImpl = async () => ({
      times: [1, 2, 3],
      hasMore: false,
      candles: {},
    });
    orch._test.mergeAdded = 3;
    orch._test.setRenderBusy(true);
    orch.noteLeftHistoryIntent({ from: 5, to: 60 });
    assert.strictEqual(orch.hasPendingLeftIntent(), true);

    // Paint/preserve remaps live from above threshold while pending still encodes left need.
    orch._test.setVisible({ from: 3004, to: 3079 });
    orch._test.setRenderBusy(false);
    orch.tryConsumePending();
    await waitDebounce();

    assert.ok(
      orch._test.fetchCount >= 1,
      'pending left need must survive preserve remap of live from',
    );
    orch.reset();
  });

  await test('continuation with remapped from is consumable while paint busy then idle', async () => {
    const orch = makeOrch();
    orch._test.fetchImpl = async () => ({
      times: [1, 2, 3],
      hasMore: false,
      candles: {},
    });
    orch._test.mergeAdded = 3;
    orch._test.setBarCount(6000);
    orch._test.setVisible({ from: 3004, to: 3079 });
    orch._test.setRenderBusy(true);
    orch.noteLeftHistoryIntent({ from: 3004, to: 3079 }, { continuation: true });
    assert.strictEqual(orch.hasPendingLeftIntent(), true);

    orch._test.setRenderBusy(false);
    orch.tryConsumePending();
    await waitDebounce();
    assert.ok(orch._test.fetchCount >= 1, 'continuation must load without from < 50');
    orch.reset();
  });

  await test('continuation stops at authoritative EOF', async () => {
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
    orch._test.setVisible({ from: 5900, to: 5999 }); // tip in view
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
