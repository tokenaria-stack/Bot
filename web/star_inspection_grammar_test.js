/**
 * Run: node web/star_inspection_grammar_test.js
 */
const assert = require('assert');
const fs = require('fs');
const path = require('path');
const G = require('./ui/star-inspection-grammar.js');

function channel(color, i) {
  const m = color.match(/rgb\((\d+), (\d+), (\d+)\)/);
  assert.ok(m, color);
  return Number(m[i + 1]);
}

function test(name, fn) {
  fn();
  console.log('ok', name);
}

test('positive position is on the cyan side', () => {
  const color = G.dotColor(1);
  assert.ok(channel(color, 2) > channel(color, 0));
});

test('negative position is on the amber side', () => {
  const color = G.dotColor(0);
  assert.ok(channel(color, 0) > channel(color, 2));
});

test('center is gray and a signed zero arrow is flat', () => {
  const color = G.dotColor(0.5);
  assert.ok(Math.abs(channel(color, 0) - 120) < 5);
  assert.strictEqual(G.arrowGlyph('slope', 0, true), '→');
  assert.strictEqual(G.formatValue(0, true, 'signed'), '0.00');
});

test('missing has no dot color and no arrow', () => {
  assert.strictEqual(G.dotColor(null), null);
  assert.strictEqual(G.arrowGlyph('slope', 0, false), '');
  assert.strictEqual(G.formatValue(0, false, 'signed'), '—');
});

test('acceleration uses a different arrow from slope', () => {
  assert.strictEqual(G.arrowGlyph('slope', 0.82, true), '↑');
  assert.strictEqual(G.arrowGlyph('accel', 0.12, true), '↗');
  assert.strictEqual(G.arrowGlyph('accel', -0.12, true), '↘');
  assert.strictEqual(G.arrowGlyph('', 71, true), '');
});

test('path text does not take a state color', () => {
  const src = fs.readFileSync(path.join(__dirname, 'ui/star-inspection-grammar.js'), 'utf8');
  assert.ok(!/favorable/.test(src));
  assert.strictEqual(G.formatPath(4.82, true), '4.820 ATR');
  assert.strictEqual(G.formatPath(null, false), '—');
});

test('a custom palette paints the same ends', () => {
  const palette = G.paletteFromHex({ low: '#ff0000', mid: '#0000ff', high: '#00ff00' });
  assert.strictEqual(G.dotColor(0, palette), 'rgb(255, 0, 0)');
  assert.strictEqual(G.dotColor(0.5, palette), 'rgb(0, 0, 255)');
  assert.strictEqual(G.dotColor(1, palette), 'rgb(0, 255, 0)');
  assert.strictEqual(G.dotColor(0), G.dotColor(0, null));
});

test('a bad hex keeps that stop on the default', () => {
  const palette = G.paletteFromHex({ low: 'nope', mid: '#111111', high: '#222222' });
  assert.strictEqual(G.dotColor(0, palette), G.dotColor(0));
  assert.strictEqual(G.defaultHex().low, '#d4943d');
  assert.strictEqual(G.defaultHex().mid, '#787b86');
  assert.strictEqual(G.defaultHex().high, '#26c6da');
});

test('grammar does not rank a list', () => {
  const src = fs.readFileSync(path.join(__dirname, 'ui/star-inspection-grammar.js'), 'utf8');
  assert.ok(!/sort\(/.test(src));
  assert.ok(!/percentile/.test(src));
  assert.strictEqual(G.dotColor(0.2), G.dotColor(0.2));
});

test('cursor time does not replace the selected Star', () => {
  const Panel = require('./ui/star-inspection-panel.js');
  Panel.noteCursor(10);
  const before = Panel.getWorkspace().selectedIndex;
  Panel.noteCursor(99);
  Panel.noteViewport({ from: 1, to: 2 });
  assert.strictEqual(Panel.getWorkspace().selectedIndex, before);
  assert.strictEqual(Panel.getWorkspace().cursorTime, 99);
});

console.log('star_inspection_grammar_test: ALL PASS');
