'use strict';

const assert = require('assert');
const StarLens = require('./ui/star-lens-panel.js');
const StarResearch = require('./ui/star-research.js');
const fs = require('fs');
const path = require('path');

function test(name, fn) {
  fn();
  console.log('ok', name);
}

test('clauses come from form, not from hit flags', () => {
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

test('viewport count does not invent membership', () => {
  const pass = [
    { index: 0, decisionAt: 1000 },
    { index: 1, decisionAt: 2000 },
    { index: 2, decisionAt: 3000 },
  ];
  assert.strictEqual(StarLens.viewCount(pass, 1500, 2500), 1);
  assert.strictEqual(StarLens.viewCount(pass, 0, 1), 0);
});

test('histogram does not plot missing as a zero bar', () => {
  const html = StarLens.histogramSvg({ rangeOk: true, bins: [2, 0, 5], min: 0, max: 10, missing: 1 }, 0, '>=');
  assert.ok(html.includes('missing 1'));
  assert.ok(!html.includes('hit1R'));
});

test('prev/next follow evaluator pass order', async () => {
  StarResearch._resetForTests();
  const seen = [];
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
  seen.push(StarResearch.getState().index);
  assert.strictEqual(seen[0], 4);
});

test('panel source has no R math', () => {
  const src = fs.readFileSync(path.join(__dirname, 'ui/star-lens-panel.js'), 'utf8');
  assert.ok(!src.includes('hit1R'));
  assert.ok(!src.includes('atrExcursion'));
  assert.ok(src.includes('/api/research/star-lens/evaluate'));
});

console.log('ALL PASS');
