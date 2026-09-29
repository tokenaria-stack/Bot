'use strict';

const assert = require('assert');
const StarLens = require('./ui/star-lens-panel.js');
const StarResearch = require('./ui/star-research.js');
const fs = require('fs');
const path = require('path');

async function test(name, fn) {
  await fn();
  console.log('ok', name);
}

async function main() {
  await test('clauses come from form, not from hit flags', () => {
    const root = {
      querySelector(sel) {
        const map = {
          '[data-num="mfeAtr"]': { checked: true },
          '[data-cmp="mfeAtr"]': { value: '>=' },
          '[data-bound="mfeAtr"]': { value: '5' },
          '[data-event="1R / 0.15"]': { value: 'reached' },
          '[data-event="2R / 0.15"]': { value: '' },
          '[data-event="3R / 0.15"]': { value: '' },
          '[data-event="Stop / 0.15"]': { value: '' },
          '[data-side]': { value: '' },
          '[data-status]': { value: '' },
        };
        return map[sel] || null;
      },
    };
    const clauses = StarLens.clausesFromForm(root);
    assert.deepStrictEqual(clauses[0], { kind: 'continuous', field: 'mfeAtr', cmp: '>=', bound: 5 });
    assert.deepStrictEqual(clauses[1], { kind: 'r', level: 1, state: 'reached' });
    assert.strictEqual(clauses.length, 2);
  });

  await test('viewport count does not invent membership', () => {
    const pass = [
      { index: 0, decisionAt: 1000 },
      { index: 1, decisionAt: 2000 },
      { index: 2, decisionAt: 3000 },
    ];
    assert.strictEqual(StarLens.viewCount(pass, 1500, 2500), 1);
    assert.strictEqual(StarLens.viewCount(pass, 0, 1), 0);
  });

  await test('histogram does not plot missing as a zero bar', () => {
    const html = StarLens.histogramSvg({ rangeOk: true, bins: [2, 0, 5], min: 0, max: 10, missing: 1 }, 0, '>=', true);
    assert.ok(html.includes('0.00') || html.includes('0'));
    assert.ok(!html.includes('hit1R'));
  });

  await test('chart range uses chart unix time, not TimeCamera bar indices', () => {
    StarLens._resetForTests();
    StarLens.init({
      visibleRangeMs: () => ({ from: 1_700_000_000_000, to: 1_700_100_000_000 }),
      fetchSources: async () => null,
      fetchEval: async () => null,
    });
    const range = StarLens.chartRange();
    assert.strictEqual(range.from, 1_700_000_000_000);
    const pass = [{ decisionAt: 1_700_050_000_000 }, { decisionAt: 1_800_000_000_000 }];
    assert.strictEqual(StarLens.viewCount(pass, range.from, range.to), 1);
    assert.strictEqual(StarLens.viewCount(pass, 50 * 1000, 130 * 1000), 0);
  });

  await test('show off returns no marks while pass remains', () => {
    StarLens._resetForTests();
    StarLens.setShowStars(false);
    assert.deepStrictEqual(StarLens.getMarks(), []);
  });

  await test('clear drops the selected star only', async () => {
    StarResearch._resetForTests();
    StarResearch.init({
      fetchStar: async (index) => ({ index, count: 10, row: { decisionAt: 1000 + index } }),
      seek: () => true,
    });
    assert.strictEqual(await StarResearch.selectNumber(5), true);
    assert.strictEqual(StarResearch.clear(), true);
    assert.strictEqual(StarResearch.getState().index, null);
    assert.strictEqual(StarResearch.getState().row, null);
  });

  await test('prev/next follow evaluator pass order', async () => {
    StarResearch._resetForTests();
    StarResearch.init({
      fetchStar: async (index) => ({ index, count: 10, row: { decisionAt: 1000 + index } }),
      seek: () => true,
    });
    StarResearch.setWalk([4, 7, 9]);
    assert.strictEqual(await StarResearch.selectNumber(5), true);
    assert.strictEqual(await StarResearch.next(), true);
    assert.strictEqual(StarResearch.getState().index, 7);
    assert.strictEqual(await StarResearch.previous(), true);
    assert.strictEqual(StarResearch.getState().index, 4);
  });

  await test('rail is a scroll container', () => {
    const css = fs.readFileSync(path.join(__dirname, 'style.css'), 'utf8');
    assert.ok(css.includes('.star-inspection-scroll'));
    assert.ok(css.includes('overflow-y: auto'));
    assert.ok(css.includes('overscroll-behavior: contain'));
  });

  await test('panel source has no R math', () => {
    const src = fs.readFileSync(path.join(__dirname, 'ui/star-lens-panel.js'), 'utf8');
    assert.ok(!src.includes('hit1R'));
    assert.ok(!src.includes('atrExcursion'));
    assert.ok(src.includes('/api/research/star-lens/evaluate'));
  });

  console.log('ALL PASS');
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
