/**
 * STAR-RESEARCH-NAV-1. Star identity and one seek per selection.
 * Run: node web/star_research_nav_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

const StarResearch = require('./ui/star-research.js');

function test(name, fn) {
  return Promise.resolve()
    .then(fn)
    .then(() => { console.log('OK', name); });
}

function row(index, count, decisionAt) {
  return { index, count, row: { decisionAt, status: 'valid' } };
}

async function main() {
  const src = fs.readFileSync(path.join(__dirname, 'ui', 'star-research.js'), 'utf8');
  const camera = fs.readFileSync(path.join(__dirname, 'ui', 'time-camera.js'), 'utf8');
  const html = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8');
  const shell = fs.readFileSync(path.join(__dirname, 'ui', 'star-inspection-panel.js'), 'utf8');
  assert.ok(!fs.existsSync(path.join(__dirname, 'star-stop.html')), 'temporary page must be gone');
  assert.ok(!src.includes('proposeFromPane'), 'selection must not propose from a pane');
  assert.ok(!src.includes('setData'), 'selection must not paint');
  assert.ok(!src.includes('paintCandles'), 'selection must not paint candles');
  assert.ok(!src.includes('setVisibleLogicalRange'), 'selection must not write the range itself');
  assert.ok(src.includes('seekHistoryIsland'), 'selection uses the existing history island');
  assert.ok(src.includes('noteTimeframe'), 'timeframe keeps identity without a seek');
  assert.ok(!camera.includes('StarResearch'), 'TimeCamera does not know about stars');
  assert.ok(!html.includes('id="star-research-no"'));
  assert.ok(!html.includes('id="star-research-prev"'));
  assert.ok(shell.includes('id="star-research-no"'));
  const css = fs.readFileSync(path.join(__dirname, 'style.css'), 'utf8');
  assert.ok(css.includes('#star-research-no::-webkit-inner-spin-button'));
  assert.ok(shell.includes('id="star-research-prev"'));
  assert.ok(!html.includes('star-stop.html'));

  await test('enter a number seeks that star once', async () => {
    StarResearch._resetForTests();
    const seeks = [];
    StarResearch.init({
      visibleBars: () => 80,
      fetchStar: async (index) => row(index, 8783, 1_000_000 + index),
      seek: (t, bars) => { seeks.push([t, bars]); return true; },
    });
    let notices = 0;
    StarResearch.onSelected(() => { notices += 1; });
    assert.strictEqual(await StarResearch.selectNumber(3456), true);
    assert.deepStrictEqual(seeks, [[1_000_000 + 3455, 80]]);
    assert.strictEqual(notices, 1);
    assert.strictEqual(StarResearch.getState().number, 3456);
    assert.strictEqual(StarResearch.getState().decisionAt, 1_000_000 + 3455);
    assert.strictEqual(StarResearch.getState().row.decisionAt, 1_000_000 + 3455);
  });

  await test('previous and next each seek once', async () => {
    StarResearch._resetForTests();
    const seeks = [];
    StarResearch.init({
      visibleBars: () => 40,
      fetchStar: async (index) => row(index, 10, 5000 + index),
      seek: (t) => { seeks.push(t); return true; },
    });
    await StarResearch.selectNumber(2);
    await StarResearch.next();
    await StarResearch.previous();
    assert.deepStrictEqual(seeks, [5001, 5002, 5001]);
    assert.strictEqual(StarResearch.getState().number, 2);
  });

  await test('a failed fetch does not seek', async () => {
    StarResearch._resetForTests();
    let seeks = 0;
    StarResearch.init({
      fetchStar: async () => null,
      seek: () => { seeks += 1; return true; },
    });
    assert.strictEqual(await StarResearch.selectNumber(4), false);
    assert.strictEqual(seeks, 0);
    assert.strictEqual(StarResearch.getState().index, null);
  });

  await test('an older reply cannot seek after a newer selection', async () => {
    StarResearch._resetForTests();
    const seeks = [];
    let releaseFirst;
    const firstGate = new Promise((resolve) => { releaseFirst = resolve; });
    StarResearch.init({
      fetchStar: (index) => {
        if (index === 0) return firstGate.then(() => row(0, 10, 11));
        return Promise.resolve(row(index, 10, 99));
      },
      seek: (t) => { seeks.push(t); return true; },
    });
    const early = StarResearch.selectNumber(1);
    const late = StarResearch.selectNumber(3);
    await late;
    releaseFirst();
    await early;
    assert.deepStrictEqual(seeks, [99]);
    assert.strictEqual(StarResearch.getState().number, 3);
  });

  await test('timeframe change does not seek or clear the star', async () => {
    StarResearch._resetForTests();
    let seeks = 0;
    StarResearch.init({
      fetchStar: async (index) => row(index, 10, 42),
      seek: () => { seeks += 1; return true; },
    });
    await StarResearch.selectNumber(8);
    assert.strictEqual(StarResearch.noteTimeframe(), 7);
    assert.strictEqual(StarResearch.noteTimeframe(), 7);
    assert.strictEqual(seeks, 1);
    assert.strictEqual(StarResearch.getState().number, 8);
  });

  await test('ends do not seek again', async () => {
    StarResearch._resetForTests();
    let seeks = 0;
    StarResearch.init({
      fetchStar: async (index) => row(index, 2, 7),
      seek: () => { seeks += 1; return true; },
    });
    assert.strictEqual(await StarResearch.previous(), false);
    await StarResearch.selectNumber(1);
    assert.strictEqual(await StarResearch.previous(), false);
    await StarResearch.selectNumber(2);
    assert.strictEqual(await StarResearch.next(), false);
    assert.strictEqual(seeks, 2);
  });

  await test('ordinal walks the population, not the global catalog', async () => {
    StarResearch._resetForTests();
    const seeks = [];
    StarResearch.init({
      fetchStar: async (index) => row(index, 8783, 100 + index),
      seek: (t) => { seeks.push(t); return true; },
    });
    StarResearch.setWalk([10, 20, 30]);
    assert.strictEqual(await StarResearch.selectOrdinal(2), true);
    assert.strictEqual(StarResearch.getState().index, 20);
    assert.strictEqual(StarResearch.getState().ordinal, 2);
    assert.strictEqual(StarResearch.getState().walkLength, 3);
    assert.strictEqual(await StarResearch.next(), true);
    assert.strictEqual(StarResearch.getState().index, 30);
    assert.strictEqual(await StarResearch.next(), false);
    StarResearch.clear();
    assert.strictEqual(StarResearch.getState().index, null);
    assert.strictEqual(StarResearch.getState().walkLength, 3);
    assert.strictEqual(await StarResearch.previous(), true);
    assert.strictEqual(StarResearch.getState().index, 30);
    StarResearch.clear();
    assert.strictEqual(await StarResearch.next(), true);
    assert.strictEqual(StarResearch.getState().index, 10);
  });

  await test('a new walk keeps the star when it still belongs', async () => {
    StarResearch._resetForTests();
    let seeks = 0;
    StarResearch.init({
      fetchStar: async (index) => row(index, 8783, index),
      seek: () => { seeks += 1; return true; },
    });
    await StarResearch.selectNumber(21);
    StarResearch.setWalk([10, 20, 30]);
    assert.strictEqual(StarResearch.getState().index, 20);
    assert.strictEqual(StarResearch.getState().ordinal, 2);
    const afterKeep = seeks;
    StarResearch.setWalk([20, 40]);
    assert.strictEqual(StarResearch.getState().index, 20);
    assert.strictEqual(seeks, afterKeep);
    await StarResearch.setWalk([1, 2]);
    assert.strictEqual(StarResearch.getState().index, 1);
  });

  console.log('star_research_nav_test: ALL PASS');
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
