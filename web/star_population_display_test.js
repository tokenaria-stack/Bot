'use strict';

const assert = require('assert');
const Display = require('./ui/star-population-display.js');
const Overlay = require('./ui/star-research-overlay.js');
const fs = require('fs');
const path = require('path');

const mem = {};
global.localStorage = {
  getItem(k) { return Object.prototype.hasOwnProperty.call(mem, k) ? mem[k] : null; },
  setItem(k, v) { mem[k] = String(v); },
  removeItem(k) { delete mem[k]; },
};

function test(name, fn) {
  fn();
  console.log('ok', name);
}

test('defaults are star4 green/red', () => {
  Display._resetForTests();
  const prefs = Display.get();
  assert.strictEqual(prefs.shape, 'star4');
  assert.strictEqual(prefs.size, 9);
  assert.strictEqual(prefs.upColor, '#00E676');
  assert.strictEqual(prefs.downColor, '#FF1744');
});

test('supported shapes persist and bad values fall back', () => {
  Display._resetForTests();
  Display.SHAPES.forEach((shape) => {
    assert.strictEqual(Display.set({ shape }).shape, shape);
  });
  assert.strictEqual(Display.set({ shape: 'pentagon' }).shape, 'star4');
  assert.strictEqual(Display.set({ upColor: 'green' }).upColor, '#00E676');
  Display.set({ upColor: '#112233', downColor: '#aabbcc', size: 14 });
  Display._resetForTests();
  mem[Display.STORAGE_KEY] = JSON.stringify({
    shape: 'diamond',
    size: 14,
    upColor: '#112233',
    downColor: '#aabbcc',
  });
  assert.deepStrictEqual(Display.get(), {
    shape: 'diamond',
    size: 14,
    upColor: '#112233',
    downColor: '#aabbcc',
  });
  assert.strictEqual(Display.set({ size: 99 }).size, 9);
});

test('storage key is not the Wozduh crossover store', () => {
  assert.strictEqual(Display.STORAGE_KEY, 'star-population-display-v1');
  assert.ok(Display.STORAGE_KEY !== 'wozduh_crossover_prefs_v1');
});

test('arrow and triangle keep side as direction', () => {
  const arrow = Overlay.populationPlan(
    [{ decisionAt: 1000000, side: 'up' }, { decisionAt: 2000000, side: 'down' }],
    { shape: 'arrow', upColor: '#00E676', downColor: '#FF1744' }
  );
  assert.strictEqual(arrow[0].shape, 'arrow');
  assert.strictEqual(arrow[0].side, 'up');
  assert.strictEqual(arrow[1].side, 'down');
  const tri = Overlay.populationPlan(
    [{ decisionAt: 1000000, side: 'up' }],
    { shape: 'triangle', upColor: '#00E676', downColor: '#FF1744' }
  );
  assert.strictEqual(tri[0].shape, 'triangle');
});

test('overlay uses the Wozduh star4 geometry', () => {
  const overlay = fs.readFileSync(path.join(__dirname, 'ui/star-research-overlay.js'), 'utf8');
  const woz = fs.readFileSync(path.join(__dirname, 'wozduh-crossovers.js'), 'utf8');
  assert.ok(overlay.includes('inner = r * 0.32'));
  assert.ok(woz.includes('inner = r * 0.32'));
  assert.ok(overlay.includes('WozduhCrossovers.pathStar4'));
  assert.ok(!overlay.includes('wozduh_crossover_prefs'));
});

console.log('star_population_display_test: ALL PASS');
