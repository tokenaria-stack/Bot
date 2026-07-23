/**
 * ADR-019 Phase 2 — LayoutController grid track builder tests (Node).
 * Run: node web/layout_controller_test.js
 */
'use strict';

const assert = require('assert');
const LayoutController = require('./ui/layout-controller.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('price alone is minmax 1fr', () => {
  const rows = LayoutController.buildGridTemplateRows({
    visible: [],
    order: ['rsx', 'wozduh'],
    footerHeights: { rsx: 200, wozduh: 180 },
  });
  assert.strictEqual(rows, 'minmax(120px, 1fr)');
});

test('visible footers add gutter + px tracks in order', () => {
  const rows = LayoutController.buildGridTemplateRows({
    visible: ['wozduh', 'rsx'],
    order: ['wozduh', 'rsx'],
    footerHeights: { wozduh: 180, rsx: 220 },
  });
  assert.strictEqual(rows, 'minmax(120px, 1fr) 4px 180px 4px 220px');
});

test('hidden footer omitted entirely (no 0px row)', () => {
  const rows = LayoutController.buildGridTemplateRows({
    visible: ['rsx'],
    order: ['wozduh', 'rsx'],
    footerHeights: { wozduh: 180, rsx: 220 },
  });
  assert.strictEqual(rows, 'minmax(120px, 1fr) 4px 220px');
});

test('missing height falls back to 180', () => {
  const rows = LayoutController.buildGridTemplateRows({
    visible: ['atr'],
    order: ['atr'],
    footerHeights: {},
  });
  assert.strictEqual(rows, 'minmax(120px, 1fr) 4px 180px');
});

test('maxFooterHeightFor reserves price min + other footers', () => {
  const stackEl = { clientHeight: 1000 };
  const max = LayoutController.maxFooterHeightFor(
    stackEl,
    {
      visible: ['wozduh', 'rsx'],
      order: ['wozduh', 'rsx'],
      footerHeights: { wozduh: 200, rsx: 180 },
    },
    'rsx',
    { wozduh: 'osc-wrap', rsx: 'rsx-wrap' },
  );
  // 1000 - 120 (price min) - 200 (wozduh) - 8 (2 gutters) = 672
  assert.strictEqual(max, 672);
});

console.log('layout_controller_test: ALL PASS');
