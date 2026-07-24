/**
 * ADR-023 ChartAdapter.setBottomTimeAxis mirror tests (Node).
 * Run: node web/bottom_time_axis_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

// Source contract: ChartAdapter exposes setBottomTimeAxis; create path uses visible:false for non-owner.
test('chart-core source contract: setBottomTimeAxis + timeScale.visible', () => {
  const src = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  assert.ok(src.includes('setBottomTimeAxis'), 'ChartAdapter.setBottomTimeAxis present');
  assert.ok(src.includes('ADR-023'), 'ADR-023 marked');
  assert.ok(/visible:\s*show/.test(src) || src.includes('visible: show'), 'visible tied to show labels');
  assert.ok(!/hostId\s*===\s*['"]wozduh['"]/.test(src.split('setBottomTimeAxis')[1]?.slice(0, 800) || ''),
    'setBottomTimeAxis body must not hardcode wozduh owner');
});

test('layout-controller syncs ChartAdapter from PaneLayout owner', () => {
  const src = fs.readFileSync(path.join(__dirname, 'ui/layout-controller.js'), 'utf8');
  assert.ok(src.includes('syncBottomTimeAxis'), 'syncBottomTimeAxis present');
  assert.ok(src.includes('resolveBottomTimeAxisHostId'), 'uses PaneLayout resolver');
  assert.ok(src.includes('ChartAdapter.setBottomTimeAxis'), 'mirrors into ChartAdapter');
  assert.ok(src.includes('data-bottom-time-axis') || src.includes('bottomTimeAxis'), 'DOM marker for CSS');
});

// Behavioral fake: mirror owner → only one chart gets visible:true
test('setBottomTimeAxis semantics: exactly one visible timeScale', () => {
  function fakeTs() {
    let opts = { visible: false };
    return {
      applyOptions(o) { opts = { ...opts, ...o }; },
      _opts: () => opts,
    };
  }
  const scales = {
    price: fakeTs(),
    wozduh: fakeTs(),
    rsx: fakeTs(),
  };
  function applyOwner(owner) {
    for (const hostId of Object.keys(scales)) {
      const show = hostId === owner;
      scales[hostId].applyOptions({ visible: show, timeVisible: show });
    }
  }
  applyOwner('rsx');
  assert.strictEqual(scales.price._opts().visible, false);
  assert.strictEqual(scales.wozduh._opts().visible, false);
  assert.strictEqual(scales.rsx._opts().visible, true);

  applyOwner('price');
  assert.strictEqual(scales.price._opts().visible, true);
  assert.strictEqual(scales.rsx._opts().visible, false);
});

console.log('bottom_time_axis_test: ALL PASS');
