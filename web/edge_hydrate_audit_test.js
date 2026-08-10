/**
 * EDGE_HYDRATE audit — timing math only (no LWC).
 * Run: node web/edge_hydrate_audit_test.js
 */
'use strict';

const assert = require('assert');
const EdgeHydrateAudit = require('./edge-hydrate-audit.js');

function test(name, fn) {
  return Promise.resolve()
    .then(fn)
    .then(() => console.log('OK', name));
}

async function run() {
  await test('reports orchestration + fetch + merge + schedule + paint(RAF)', async () => {
    EdgeHydrateAudit.abort();
    EdgeHydrateAudit.noteIntent('RIGHT');
    await new Promise((r) => setTimeout(r, 25));
    EdgeHydrateAudit.markFetchStart('RIGHT');
    await new Promise((r) => setTimeout(r, 15));
    EdgeHydrateAudit.markFetchEnd();
    EdgeHydrateAudit.markMergeStart({
      tipBefore: 100,
      storeBefore: 3000,
      tf: '1m',
    });
    await new Promise((r) => setTimeout(r, 5));
    EdgeHydrateAudit.markMergeEnd({
      barsAdded: 2999,
      tipAfter: 200,
      storeAfter: 5999,
    });
    const intent = { mode: 'prepend' };
    EdgeHydrateAudit.attachToPaintIntent(intent);
    assert.ok(intent._edgeHydrate);
    await new Promise((r) => setTimeout(r, 10));
    EdgeHydrateAudit.markPaintStart(intent._edgeHydrate);

    await new Promise((resolve) => {
      EdgeHydrateAudit.completeAfterPaintRaf(intent._edgeHydrate);
      // Node has no rAF — audit falls back to sync finish; polyfill for clarity.
      if (typeof requestAnimationFrame !== 'function') {
        // completeAfterPaintRaf already finished sync
        resolve();
        return;
      }
      requestAnimationFrame(() => resolve());
    });

    const last = globalThis.__EDGE_HYDRATE_LAST__;
    assert.ok(last);
    assert.strictEqual(last.direction, 'RIGHT');
    assert.ok(last.orchestrationMs >= 20, `orchestrationMs=${last.orchestrationMs}`);
    assert.ok(last.fetchMs >= 10, `fetchMs=${last.fetchMs}`);
    assert.ok(last.mergeMs >= 0);
    assert.ok(last.scheduleMs >= 5, `scheduleMs=${last.scheduleMs}`);
    assert.ok(last.paintMs >= 0);
    assert.ok(last.totalMs >= last.orchestrationMs);
    assert.strictEqual(last.barsAdded, 2999);
    assert.strictEqual(last.tf, '1m');
    assert.strictEqual(last.tipBefore, 100);
    assert.strictEqual(last.tipAfter, 200);
  });

  await test('double completeAfterPaintRaf does not double-log', async () => {
    EdgeHydrateAudit.abort();
    EdgeHydrateAudit.noteIntent('LEFT');
    EdgeHydrateAudit.markFetchStart('LEFT');
    EdgeHydrateAudit.markFetchEnd();
    EdgeHydrateAudit.markMergeStart({ tipBefore: 1, storeBefore: 1, tf: '15m' });
    EdgeHydrateAudit.markMergeEnd({ barsAdded: 1, tipAfter: 1, storeAfter: 2 });
    const intent = {};
    EdgeHydrateAudit.attachToPaintIntent(intent);
    EdgeHydrateAudit.markPaintStart(intent._edgeHydrate);
    EdgeHydrateAudit.completeAfterPaintRaf(intent._edgeHydrate);
    const first = globalThis.__EDGE_HYDRATE_LAST__;
    EdgeHydrateAudit.completeAfterPaintRaf(intent._edgeHydrate);
    assert.strictEqual(globalThis.__EDGE_HYDRATE_LAST__, first);
  });

  console.log('edge_hydrate_audit_test: ALL PASS');
}

run().catch((err) => {
  console.error(err);
  process.exit(1);
});
