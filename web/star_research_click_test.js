/**
 * STAR-RESEARCH-CLICK-1. A Wozduh star click selects the certified row and does not seek.
 * Run: node web/star_research_click_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

const StarResearch = require('./ui/star-research.js');
const Overlay = require('./ui/star-research-overlay.js');

function test(name, fn) {
  return Promise.resolve().then(fn).then(() => { console.log('OK', name); });
}

function body(index, count, row) {
  return { index, count, row };
}

const star3456 = {
  decisionAt: 1655156700000,
  status: 'valid',
  side: 'down',
  entryAt: 1655157600000,
  entryPrice: 23050,
  swingAt: 1655152200000,
  swingWick: 23780,
  stopPrice: 23825.329640813703,
};

const starNoSwing = {
  decisionAt: 1567965600000,
  status: 'no_structure',
  side: 'up',
  entryAt: 1567966500000,
  entryPrice: 10000,
};

async function main() {
  const src = fs.readFileSync(path.join(__dirname, 'ui', 'star-research.js'), 'utf8');
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  const cross = fs.readFileSync(path.join(__dirname, 'wozduh-crossovers.js'), 'utf8');
  assert.ok(!/function selectOpenTime[\s\S]{0,500}seek\(/.test(src));
  assert.ok(!src.includes('0.15'));
  assert.ok(!src.includes('ATR'));
  assert.ok(!/function onWozduhClick[\s\S]{0,700}TimeCamera/.test(core));
  assert.ok(!/function onWozduhClick[\s\S]{0,700}seekHistoryIsland/.test(core));
  assert.ok(cross.includes("STAR_PAIR = 'woz_rsi_hl2_vwema_x_ema5_chan_mid'"));
  assert.ok(!cross.includes('DetectCrossoverEdge'));

  await test('a 15m star click selects that certified number and does not seek', async () => {
    StarResearch._resetForTests();
    const seeks = [];
    StarResearch.init({
      getTf: () => '15m',
      canonicalOpenSec: () => 1655156700,
      fetchAt: async (ms) => {
        assert.strictEqual(ms, 1655156700000);
        return body(3455, 8783, star3456);
      },
      seek: (t) => { seeks.push(t); return true; },
    });
    assert.strictEqual(await StarResearch.onPaneClick({ x: 10, y: 10 }), true);
    assert.deepStrictEqual(seeks, []);
    assert.strictEqual(StarResearch.getState().number, 3456);
    assert.strictEqual(StarResearch.getState().decisionAt, 1655156700000);
    assert.strictEqual(StarResearch.getState().row.stopPrice, star3456.stopPrice);
  });

  await test('previous and next walk from the clicked star and those still center', async () => {
    StarResearch._resetForTests();
    const seeks = [];
    StarResearch.init({
      getTf: () => '15m',
      canonicalOpenSec: () => 5000,
      fetchAt: async () => body(4, 10, { decisionAt: 5000000, status: 'valid', side: 'up', entryAt: 5001000, entryPrice: 1 }),
      fetchStar: async (index) => body(index, 10, { decisionAt: 1000 + index, status: 'valid', side: 'up', entryAt: 2000 + index, entryPrice: 1 }),
      seek: (t) => { seeks.push(t); return true; },
    });
    await StarResearch.onPaneClick({ x: 1, y: 1 });
    assert.deepStrictEqual(seeks, []);
    await StarResearch.next();
    await StarResearch.previous();
    assert.deepStrictEqual(seeks, [1005, 1004]);
    assert.strictEqual(StarResearch.getState().number, 5);
  });

  await test('a non-star point and another timeframe do not select', async () => {
    StarResearch._resetForTests();
    let fetches = 0;
    StarResearch.init({
      getTf: () => '15m',
      canonicalOpenSec: () => null,
      fetchAt: async () => { fetches += 1; return body(1, 10, star3456); },
      seek: () => { throw new Error('seek'); },
    });
    assert.strictEqual(await StarResearch.onPaneClick({ x: 3, y: 3 }), false);
    assert.strictEqual(fetches, 0);
    assert.strictEqual(StarResearch.getState().index, null);
    StarResearch.init({ getTf: () => '1h', canonicalOpenSec: () => 100 });
    assert.strictEqual(await StarResearch.onPaneClick({ x: 3, y: 3 }), false);
    assert.strictEqual(fetches, 0);
  });

  await test('a second click replaces the overlay and a miss keeps the previous star', async () => {
    StarResearch._resetForTests();
    const plans = [];
    StarResearch.onSelected(() => {
      plans.push(Overlay.researchPlan(StarResearch.getState().row).lines.map((item) => item.id));
    });
    let mode = 'valid';
    StarResearch.init({
      getTf: () => '15m',
      canonicalOpenSec: (pt) => pt.y,
      fetchAt: async (ms) => {
        if (mode === 'miss') return null;
        if (ms === 2000) return body(0, 8783, starNoSwing);
        return body(1, 8783, star3456);
      },
      seek: () => { throw new Error('seek'); },
    });
    assert.strictEqual(await StarResearch.onPaneClick({ x: 0, y: 1 }), true);
    assert.strictEqual(await StarResearch.onPaneClick({ x: 0, y: 2 }), true);
    assert.deepStrictEqual(plans[0], ['entry', 'stop', 'r1', 'r2', 'r3']);
    assert.deepStrictEqual(plans[1], ['entry']);
    assert.strictEqual(Overlay.researchPlan(StarResearch.getState().row).entries[0].shape, 'triangleUp');
    mode = 'miss';
    assert.strictEqual(await StarResearch.onPaneClick({ x: 0, y: 9 }), false);
    assert.strictEqual(StarResearch.getState().number, 1);
    assert.strictEqual(StarResearch.getState().row.status, 'no_structure');
  });

  await test('number selection still centers', async () => {
    StarResearch._resetForTests();
    const seeks = [];
    StarResearch.init({
      fetchStar: async (index) => body(index, 8783, { decisionAt: 1000 + index, status: 'valid', side: 'down', entryAt: 2000, entryPrice: 3, stopPrice: 4 }),
      seek: (t) => { seeks.push(t); return true; },
    });
    assert.strictEqual(await StarResearch.selectNumber(3456), true);
    assert.deepStrictEqual(seeks, [1000 + 3455]);
    const plan = Overlay.researchPlan(StarResearch.getState().row);
    assert.strictEqual(plan.entries[0].shape, 'triangleDown');
    assert.strictEqual(plan.entries[0].color, plan.lines.find((item) => item.id === 'entry').color);
  });

  console.log('star research click tests passed');
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
