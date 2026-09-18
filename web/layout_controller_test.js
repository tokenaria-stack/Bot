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

test('maximize template is one minmax(0,1fr); ignores footer px and price min', () => {
  const rows = LayoutController.buildGridTemplateRows({
    visible: ['wozduh', 'rsx'],
    order: ['wozduh', 'rsx'],
    footerHeights: { wozduh: 180, rsx: 220 },
    fullscreenPaneId: 'wozduh',
  });
  assert.strictEqual(rows, 'minmax(0, 1fr)');
  assert.ok(!rows.includes('120px'));
  assert.ok(!rows.includes('180px'));
  assert.ok(!rows.includes('4px'));
});

test('price maximize also uses a single 0,1fr track', () => {
  const rows = LayoutController.buildGridTemplateRows({
    visible: ['rsx'],
    order: ['rsx'],
    footerHeights: { rsx: 200 },
    fullscreenPaneId: 'price',
  });
  assert.strictEqual(rows, 'minmax(0, 1fr)');
});

test('normal template ignores empty fullscreenPaneId', () => {
  const rows = LayoutController.buildGridTemplateRows({
    visible: ['rsx'],
    order: ['rsx'],
    footerHeights: { rsx: 200 },
    fullscreenPaneId: null,
  });
  assert.strictEqual(rows, 'minmax(120px, 1fr) 4px 200px');
});

test('participatesInStack: maximize is not visible[]', () => {
  const state = {
    visible: ['wozduh', 'rsx'],
    fullscreenPaneId: 'wozduh',
  };
  assert.strictEqual(LayoutController.participatesInStack(state, 'wozduh'), true);
  assert.strictEqual(LayoutController.participatesInStack(state, 'rsx'), false);
  assert.strictEqual(LayoutController.participatesInStack(state, 'price'), false);
  const normal = { visible: ['rsx'], fullscreenPaneId: null };
  assert.strictEqual(LayoutController.participatesInStack(normal, 'price'), true);
  assert.strictEqual(LayoutController.participatesInStack(normal, 'rsx'), true);
  assert.strictEqual(LayoutController.participatesInStack(normal, 'wozduh'), false);
});

function installFakeChartsDom() {
  function makeNode(id) {
    const classes = new Set();
    const node = {
      id: id || '',
      hidden: false,
      style: {},
      dataset: {},
      children: [],
      parentNode: null,
      classList: {
        add(c) { classes.add(c); },
        toggle(c, force) {
          if (force === true) classes.add(c);
          else if (force === false) classes.delete(c);
          else if (classes.has(c)) classes.delete(c);
          else classes.add(c);
        },
        contains(c) { return classes.has(c); },
      },
      querySelector() {
        return {
          dataset: {},
          classList: { add() {} },
          addEventListener() {},
        };
      },
      querySelectorAll(sel) {
        const out = [];
        const walk = (n) => {
          if (sel.includes('pane-splitter') && n.dataset && n.dataset.layoutGutter === '1') {
            out.push(n);
          }
          (n.children || []).forEach(walk);
        };
        (node.children || []).forEach(walk);
        return out;
      },
      addEventListener() {},
      setAttribute() {},
      insertBefore(child, ref) {
        child.parentNode = node;
        const i = node.children.indexOf(ref);
        if (i < 0) node.children.push(child);
        else node.children.splice(i, 0, child);
      },
      remove() {
        if (!node.parentNode) return;
        const kids = node.parentNode.children;
        const i = kids.indexOf(node);
        if (i >= 0) kids.splice(i, 1);
        node.parentNode = null;
      },
    };
    return node;
  }

  const stack = makeNode('charts-stack');
  const price = makeNode('price-wrap');
  const osc = makeNode('osc-wrap');
  const rsx = makeNode('rsx-wrap');
  [price, osc, rsx].forEach((w) => {
    w.parentNode = stack;
    stack.children.push(w);
  });
  const byId = {
    'charts-stack': stack,
    'price-wrap': price,
    'osc-wrap': osc,
    'rsx-wrap': rsx,
  };
  const previous = global.document;
  global.document = {
    getElementById(id) { return byId[id] || null; },
    createElement() {
      return makeNode('');
    },
    querySelectorAll() { return []; },
    body: { classList: { toggle() {} } },
  };
  return { stack, price, osc, rsx, restore() { global.document = previous; } };
}

test('applyStack maximize: one track, selected footer only, no splitters', () => {
  const dom = installFakeChartsDom();
  try {
    const state = {
      visible: ['wozduh', 'rsx'],
      order: ['wozduh', 'rsx'],
      footerHeights: { wozduh: 180, rsx: 220 },
      fullscreenPaneId: 'wozduh',
    };
    LayoutController.applyStack('live', state);
    assert.strictEqual(dom.stack.style.gridTemplateRows, 'minmax(0, 1fr)');
    assert.strictEqual(dom.osc.hidden, false);
    assert.strictEqual(dom.osc.style.display, 'flex');
    assert.strictEqual(dom.osc.style.gridRow, '1');
    assert.ok(dom.osc.classList.contains('fullscreen-pane'));
    assert.strictEqual(dom.price.hidden, true);
    assert.strictEqual(dom.price.style.display, 'none');
    assert.ok(!dom.price.classList.contains('fullscreen-pane'));
    assert.strictEqual(dom.rsx.hidden, true);
    const splitters = dom.stack.querySelectorAll('.pane-splitter[data-layout-gutter="1"]');
    assert.strictEqual(splitters.length, 0);
  } finally {
    dom.restore();
  }
});

test('applyStack restore: price + splitters + visible[] membership, not leftover maximize', () => {
  const dom = installFakeChartsDom();
  try {
    const maximized = {
      visible: ['wozduh', 'rsx'],
      order: ['wozduh', 'rsx'],
      footerHeights: { wozduh: 180, rsx: 220 },
      fullscreenPaneId: 'wozduh',
    };
    const normal = { ...maximized, fullscreenPaneId: null };
    LayoutController.applyStack('live', maximized);
    LayoutController.applyStack('live', normal);
    assert.ok(dom.stack.style.gridTemplateRows.includes('minmax(120px, 1fr)'));
    assert.ok(dom.stack.style.gridTemplateRows.includes('180px'));
    assert.strictEqual(dom.price.hidden, false);
    assert.strictEqual(dom.price.style.gridRow, '1');
    assert.strictEqual(dom.osc.hidden, false);
    assert.strictEqual(dom.rsx.hidden, false);
    assert.ok(!dom.osc.classList.contains('fullscreen-pane'));
    const splitters = dom.stack.querySelectorAll('.pane-splitter[data-layout-gutter="1"]');
    assert.strictEqual(splitters.length, 2);
  } finally {
    dom.restore();
  }
});

test('viewport overlay CSS is gone; no isolation stacking workaround', () => {
  const fs = require('fs');
  const path = require('path');
  const html = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8');
  const css = fs.readFileSync(path.join(__dirname, 'style.css'), 'utf8');
  const layout = fs.readFileSync(path.join(__dirname, 'ui/layout-controller.js'), 'utf8');
  assert.ok(!html.includes('width: 100vw !important'));
  assert.ok(!html.includes('height: 100vh !important'));
  assert.ok(!/fullscreen-pane[\s\S]{0,240}position:\s*fixed/.test(html));
  assert.ok(!/fullscreen-pane[\s\S]{0,200}z-index:\s*9999/.test(html));
  assert.ok(!html.includes('top: 82px'));
  assert.ok(!css.includes('isolation: isolate'));
  assert.ok(!layout.includes('setVisible('));
  assert.ok(!layout.includes('setSeriesVisible'));
});

console.log('layout_controller_test: ALL PASS');
