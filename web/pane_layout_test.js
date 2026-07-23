/**
 * ADR-019 PaneLayout unit tests (Node).
 * Run: node web/pane_layout_test.js
 */
'use strict';

const assert = require('assert');
const PaneLayout = require('./ui/pane-layout.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function memoryStorage() {
  const map = new Map();
  return {
    getItem(key) {
      return map.has(key) ? map.get(key) : null;
    },
    setItem(key, value) {
      map.set(key, String(value));
    },
  };
}

function sampleManifest() {
  return {
    panes: {
      pane_osc: [
        { id: 'line_rsx', hostId: 'rsx', renderOptions: { title: 'RSX' } },
        { id: 'line_rsx_signal', hostId: 'rsx', renderOptions: { title: 'RSX Signal' } },
        { id: 'woz_fast', hostId: 'wozduh', renderOptions: { title: 'wt11 (Blue)' } },
      ],
    },
  };
}

test('collectCatalog unique HostIDs, skips price, titles without hardcode', () => {
  const cat = PaneLayout.collectCatalog({
    panes: {
      a: [
        { id: '1', hostId: 'rsx' },
        { id: '2', hostId: 'rsx' },
        { id: '3', hostId: 'wozduh', renderOptions: { paneTitle: 'Woz' } },
        { id: '4', hostId: 'price' },
        { id: '5', hostId: 'atr' },
      ],
    },
  });
  assert.deepStrictEqual(
    cat.map((c) => c.hostId),
    ['rsx', 'wozduh', 'atr'],
  );
  assert.strictEqual(cat.find((c) => c.hostId === 'rsx').title, 'RSX');
  assert.strictEqual(cat.find((c) => c.hostId === 'wozduh').title, 'Woz');
  assert.strictEqual(cat.find((c) => c.hostId === 'atr').title, 'ATR');
});

test('restore ∩ manifest drops unknown hosts', () => {
  const catalog = PaneLayout.collectCatalog(sampleManifest());
  const restored = PaneLayout.restoreFromSaved(
    {
      version: 1,
      visible: ['rsx', 'wozduh', 'gone'],
      order: ['gone', 'wozduh', 'rsx'],
      footerHeights: { rsx: 200, wozduh: 160, gone: 99 },
      fullscreenPaneId: 'gone',
    },
    catalog,
  );
  assert.deepStrictEqual(restored.order, ['wozduh', 'rsx']);
  assert.deepStrictEqual(restored.visible, ['rsx', 'wozduh']);
  assert.strictEqual(restored.footerHeights.gone, undefined);
  assert.strictEqual(restored.footerHeights.rsx, 200);
  assert.strictEqual(restored.fullscreenPaneId, null);
});

test('restore appends new HostIDs with default height; keeps hidden heights', () => {
  const catalog = PaneLayout.collectCatalog({
    panes: {
      p: [
        { id: 'a', hostId: 'rsx' },
        { id: 'b', hostId: 'wozduh' },
        { id: 'c', hostId: 'atr' },
      ],
    },
  });
  const restored = PaneLayout.restoreFromSaved(
    {
      version: 1,
      visible: ['rsx'],
      order: ['rsx', 'wozduh'],
      footerHeights: { rsx: 220, wozduh: 150 },
      fullscreenPaneId: null,
    },
    catalog,
    180,
  );
  assert.deepStrictEqual(restored.order, ['rsx', 'wozduh', 'atr']);
  assert.deepStrictEqual(restored.visible, ['rsx']);
  assert.strictEqual(restored.footerHeights.wozduh, 150);
  assert.strictEqual(restored.footerHeights.atr, 180);
});

test('version mismatch → defaults (all visible)', () => {
  const catalog = PaneLayout.collectCatalog(sampleManifest());
  const restored = PaneLayout.restoreFromSaved(
    {
      version: 99,
      visible: [],
      order: ['rsx'],
      footerHeights: { rsx: 10 },
      fullscreenPaneId: 'rsx',
    },
    catalog,
    180,
  );
  assert.strictEqual(restored.version, 1);
  assert.deepStrictEqual(restored.visible.slice().sort(), ['rsx', 'wozduh']);
  assert.deepStrictEqual(restored.order.slice().sort(), ['rsx', 'wozduh']);
  assert.strictEqual(restored.footerHeights.rsx, 180);
  assert.strictEqual(restored.fullscreenPaneId, null);
});

test('fullscreen invalidation when host removed from manifest', () => {
  const catalog = PaneLayout.collectCatalog({
    panes: { p: [{ id: '1', hostId: 'rsx' }] },
  });
  const restored = PaneLayout.restoreFromSaved(
    {
      version: 1,
      visible: ['rsx', 'wozduh'],
      order: ['rsx', 'wozduh'],
      footerHeights: { rsx: 200, wozduh: 180 },
      fullscreenPaneId: 'wozduh',
    },
    catalog,
  );
  assert.deepStrictEqual(restored.order, ['rsx']);
  assert.strictEqual(restored.fullscreenPaneId, null);
});

test('create persists mutations and intersects on re-init', () => {
  const storage = memoryStorage();
  const layout = PaneLayout.create({ storage, defaultFooterHeightPx: 180 });
  layout.init({ manifest: sampleManifest() });
  assert.strictEqual(layout.isVisible('rsx'), true);
  layout.setVisible('wozduh', false);
  assert.strictEqual(layout.isVisible('wozduh'), false);
  assert.strictEqual(layout.getState().footerHeights.wozduh, 180);

  const raw = JSON.parse(storage.getItem(PaneLayout.LS_KEY));
  assert.deepStrictEqual(raw.visible, ['rsx']);

  const layout2 = PaneLayout.create({ storage });
  layout2.init({
    manifest: {
      panes: {
        p: [
          { id: '1', hostId: 'rsx' },
          { id: '2', hostId: 'atr' },
        ],
      },
    },
  });
  const st = layout2.getState();
  assert.deepStrictEqual(st.visible, ['rsx']);
  assert.ok(st.order.includes('atr'));
  assert.strictEqual(st.footerHeights.atr, 180);
  assert.strictEqual(st.footerHeights.wozduh, undefined);
});

test('setFullscreen + toggle clears fullscreen for hidden host', () => {
  const layout = PaneLayout.create({ storage: memoryStorage() });
  layout.init({ manifest: sampleManifest() });
  assert.strictEqual(layout.setFullscreen('rsx'), true);
  assert.strictEqual(layout.getFullscreen(), 'rsx');
  layout.toggle('rsx');
  assert.strictEqual(layout.isVisible('rsx'), false);
  assert.strictEqual(layout.getFullscreen(), null);
});

test('subscribe fires on mutation', () => {
  const layout = PaneLayout.create({ storage: memoryStorage() });
  layout.init({ manifest: sampleManifest() });
  const seen = [];
  layout.subscribe((s) => seen.push(s.visible.slice()));
  layout.setVisible('rsx', false);
  assert.ok(seen.length >= 1);
  assert.ok(!seen[seen.length - 1].includes('rsx'));
});

test('setFooterHeight clamps and persists; price never stored', () => {
  const storage = memoryStorage();
  const layout = PaneLayout.create({ storage, defaultFooterHeightPx: 180 });
  layout.init({ manifest: sampleManifest() });
  assert.strictEqual(layout.setFooterHeight('rsx', 250), true);
  assert.strictEqual(layout.getState().footerHeights.rsx, 250);
  assert.strictEqual(layout.getState().footerHeights.price, undefined);
  assert.strictEqual(layout.setFooterHeight('rsx', 250), false);
  assert.strictEqual(layout.setFooterHeight('rsx', 5), true);
  assert.strictEqual(layout.getState().footerHeights.rsx, PaneLayout.FOOTER_HEIGHT_MIN_PX);
  assert.strictEqual(layout.setFooterHeight('rsx', 9999), true);
  assert.strictEqual(layout.getState().footerHeights.rsx, PaneLayout.FOOTER_HEIGHT_MAX_PX);
  assert.strictEqual(layout.setFooterHeight('missing', 200), false);

  const raw = JSON.parse(storage.getItem(PaneLayout.LS_KEY));
  assert.strictEqual(raw.footerHeights.rsx, PaneLayout.FOOTER_HEIGHT_MAX_PX);
});

test('setOrder permutes; moveHostBefore among visible preserves hidden slots', () => {
  const layout = PaneLayout.create({ storage: memoryStorage() });
  layout.init({
    manifest: {
      panes: {
        p: [
          { id: '1', hostId: 'rsx' },
          { id: '2', hostId: 'wozduh' },
          { id: '3', hostId: 'atr' },
        ],
      },
    },
  });
  layout.setVisible('atr', false);
  assert.deepStrictEqual(layout.getState().order, ['rsx', 'wozduh', 'atr']);
  assert.strictEqual(layout.setOrder(['wozduh', 'rsx', 'atr']), true);
  assert.deepStrictEqual(layout.getState().order, ['wozduh', 'rsx', 'atr']);
  assert.strictEqual(layout.setOrder(['wozduh', 'rsx', 'atr']), false);

  // Visible: wozduh, rsx — move rsx before wozduh; atr stays in hidden slot after visibles.
  assert.strictEqual(layout.moveHostBefore('rsx', 'wozduh'), true);
  assert.deepStrictEqual(layout.getState().order, ['rsx', 'wozduh', 'atr']);
  assert.strictEqual(layout.getFullscreen(), null);

  layout.setFullscreen('rsx');
  assert.strictEqual(layout.setOrder(['wozduh', 'rsx', 'atr']), true);
  assert.strictEqual(layout.moveHostBefore('wozduh', null), true);
  assert.deepStrictEqual(layout.getState().order, ['rsx', 'wozduh', 'atr']);
  assert.strictEqual(layout.getFullscreen(), 'rsx');
});

test('price allowed as fullscreen target', () => {
  const layout = PaneLayout.create({ storage: memoryStorage() });
  layout.init({ manifest: sampleManifest() });
  assert.strictEqual(layout.setFullscreen('price'), true);
  assert.strictEqual(layout.getFullscreen(), 'price');
});

console.log('pane_layout_test: ALL PASS');
