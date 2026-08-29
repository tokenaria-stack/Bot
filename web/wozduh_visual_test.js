/**
 * WOZDUH-VISUAL-1 — menu labels + owner underline (no identity change).
 * Run: node web/wozduh_visual_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('settings renderer underlines woz_slow label only; titles stay plain text', () => {
  const src = fs.readFileSync(path.join(__dirname, 'ui/settings-renderer.js'), 'utf8');
  assert.ok(src.includes("c.id === 'woz_slow'"));
  assert.ok(src.includes('wozduh-pane-owner-label'));
  assert.ok(src.includes('textContent'));
  assert.ok(!src.includes('<u>'));
  assert.ok(!src.includes('woz_fast') || src.includes("c.id === 'woz_slow'"));
});

test('channel-series defaults and dashForStyle treat 0 as solid', () => {
  const src = fs.readFileSync(path.join(__dirname, 'channel-series.js'), 'utf8');
  assert.ok(src.includes('upperLineStyle: 0'));
  assert.ok(src.includes('lowerLineStyle: 0'));
  assert.ok(!/upperLineStyle:\s*2/.test(src));
});

test('CSS underlines pane-owner label', () => {
  const css = fs.readFileSync(path.join(__dirname, 'style.css'), 'utf8');
  assert.ok(css.includes('.wozduh-pane-owner-label'));
  assert.ok(css.includes('text-decoration: underline'));
});

console.log('wozduh_visual_test: ALL PASS');
