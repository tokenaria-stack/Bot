'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

function read(rel) {
  return fs.readFileSync(path.join(__dirname, rel), 'utf8');
}

const html = read('index.html');
const css = read('style.css');
const dockSrc = read('ui/tool-dock.js');
const panelSrc = read('ui/star-inspection-panel.js');
const Dock = require('./ui/tool-dock.js');

function test(name, fn) {
  fn();
  console.log('ok', name);
}

test('one research tool, no second Ind, no panel OS', () => {
  assert.ok(html.includes('tool-dock.js'));
  assert.ok(dockSrc.includes('data-tool="research"'));
  assert.ok(!dockSrc.includes('data-tool="indicators"'));
  assert.ok(!dockSrc.includes('activePanel'));
  assert.ok(!dockSrc.includes('evaluate('));
  assert.ok(!dockSrc.includes('StarLens'));
  assert.strictEqual((html.match(/id="btn-ind-menu"/g) || []).length, 1);
  assert.ok(!html.includes('data-tool="indicators"'));
});

test('geometry is chart | research | dock', () => {
  assert.ok(css.includes('right: var(--tool-dock-width'));
  assert.ok(css.includes('#star-inspection[hidden]'));
  assert.ok(css.includes('--tool-dock-width'));
  assert.ok(css.includes('calc(var(--tool-dock-width'));
  assert.ok(panelSrc.includes('root.hidden'));
  assert.ok(panelSrc.includes('applyWidth'));
});

test('chartInset is dock only when closed, dock plus panel when open', () => {
  Dock._resetForTests();
  assert.strictEqual(Dock.WIDTH, 40);
  assert.strictEqual(Dock.isOpen(), true);
  assert.strictEqual(Dock.chartInset(), 40);

  const panel = {
    hidden: false,
    classList: { contains() { return false; } },
    style: { width: '380px' },
  };
  global.document = {
    body: { style: { setProperty() {} }, classList: { remove() {}, toggle() {} } },
    getElementById(id) { return id === 'star-inspection' ? panel : null; },
  };
  Dock.setOpen(true);
  panel.hidden = false;
  assert.strictEqual(Dock.chartInset(), 420);
  Dock.setOpen(false);
  panel.hidden = true;
  assert.strictEqual(Dock.isOpen(), false);
  assert.strictEqual(Dock.chartInset(), 40);
  panel.classList.contains = (c) => c === 'is-overlay';
  Dock.setOpen(true);
  panel.hidden = false;
  assert.strictEqual(Dock.chartInset(), 40);
  delete global.document;
  Dock._resetForTests();
});

test('open/close does not evaluate or unmount', () => {
  assert.ok(!dockSrc.includes('evaluate('));
  assert.ok(!dockSrc.includes('removeChild'));
  assert.ok(!dockSrc.includes('innerHTML = \'\''));
  assert.ok(dockSrc.includes('panel.hidden'));
});

console.log('tool_dock_shell_test: ALL PASS');
