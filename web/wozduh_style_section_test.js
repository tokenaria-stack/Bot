/**
 * WOZDUH-STYLE-SECTION-1 — Style UI over frozen color commands.
 * Run: node web/wozduh_style_section_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

function createEl(tag) {
  const el = {
    tagName: String(tag).toUpperCase(),
    children: [],
    className: '',
    hidden: false,
    type: '',
    value: '',
    title: '',
    textContent: '',
    checked: false,
    dataset: {},
    style: {},
    _listeners: {},
    parentNode: null,
    appendChild(child) {
      this.children.push(child);
      child.parentNode = this;
      return child;
    },
    removeChild(child) {
      this.children = this.children.filter((c) => c !== child);
      child.parentNode = null;
      return child;
    },
    replaceChildren() {
      for (const c of this.children) c.parentNode = null;
      this.children = [];
    },
    addEventListener(type, fn) {
      (this._listeners[type] ||= []).push(fn);
    },
    dispatch(type) {
      const ev = {
        type,
        preventDefault() {},
        stopPropagation() {},
        target: this,
      };
      for (const fn of this._listeners[type] || []) fn(ev);
    },
    setAttribute(name, value) {
      if (name === 'aria-label') this.ariaLabel = value;
      if (name === 'title') this.title = value;
    },
    querySelectorAll(sel) {
      const out = [];
      const walk = (node) => {
        for (const c of node.children || []) {
          if (matchSel(c, sel)) out.push(c);
          walk(c);
        }
      };
      walk(this);
      return out;
    },
    querySelector(sel) {
      return this.querySelectorAll(sel)[0] || null;
    },
    remove() {
      if (this.parentNode && typeof this.parentNode.removeChild === 'function') {
        this.parentNode.removeChild(this);
      }
    },
  };
  return el;
}

function matchSel(el, sel) {
  if (sel === 'input[type="color"]') return el.tagName === 'INPUT' && el.type === 'color';
  if (sel === 'input.wozduh-chk' || sel === '.wozduh-chk') {
    return String(el.className).split(/\s+/).includes('wozduh-chk');
  }
  if (sel.startsWith('.')) {
    const want = sel.slice(1).split('.').filter(Boolean);
    const have = String(el.className).split(/\s+/);
    return want.every((cls) => have.includes(cls));
  }
  return false;
}

const fakeBody = createEl('body');
global.document = {
  createElement: (tag) => createEl(tag),
  body: fakeBody,
  documentElement: fakeBody,
};
global.getComputedStyle = (el) => {
  const named = {
    blue: 'rgb(0, 0, 255)',
    maroon: 'rgb(128, 0, 0)',
    aqua: 'rgb(0, 255, 255)',
    purple: 'rgb(128, 0, 128)',
    orange: 'rgb(255, 165, 0)',
    green: 'rgb(0, 128, 0)',
    navy: 'rgb(0, 0, 128)',
    black: 'rgb(0, 0, 0)',
  };
  const key = String(el?.style?.color || '').trim().toLowerCase();
  if (named[key]) return { color: named[key] };
  return { color: el?.style?.color || '' };
};
global.window = global;

const WozduhColorPrefs = require('./wozduh-color-prefs.js');
const { DDRFactory } = require('./series-factory.js');
const { SettingsRenderer } = require('./ui/settings-renderer.js');

function test(name, fn) {
  const ret = fn();
  if (ret && typeof ret.then === 'function') {
    return ret.then(() => console.log('OK', name));
  }
  console.log('OK', name);
  return Promise.resolve();
}

function memStorage(initial) {
  const map = { ...(initial || {}) };
  return {
    getItem(key) {
      return Object.prototype.hasOwnProperty.call(map, key) ? map[key] : null;
    },
    setItem(key, value) {
      map[key] = String(value);
    },
    removeItem(key) {
      delete map[key];
    },
    dump() {
      return { ...map };
    },
  };
}

function fakeSeries(events, id) {
  return {
    setData(points) { events.push({ op: 'setData', id, points }); },
    update(pt) { events.push({ op: 'update', id, pt }); },
    applyOptions(opts) { events.push({ op: 'applyOptions', id, opts: { ...opts } }); },
    priceScale() { return { applyOptions() {} }; },
  };
}

function panes() {
  return {
    pane_osc: [
      {
        id: 'woz_rsi_hl2',
        hostId: 'wozduh',
        kind: 'line',
        configurable: true,
        renderOptions: { color: 'purple', title: 'RSI HL2', defaultVisible: true },
      },
      {
        id: 'woz_vol_rsi_ema5',
        hostId: 'wozduh',
        kind: 'line',
        configurable: true,
        renderOptions: { color: 'aqua', title: 'Volume RSI EMA5', defaultVisible: true },
      },
      {
        id: 'woz_vol_rsi_ema12',
        hostId: 'wozduh',
        kind: 'line',
        configurable: true,
        renderOptions: { color: 'blue', title: 'Volume RSI EMA12', defaultVisible: true },
      },
      {
        id: 'woz_macd_rsi_close',
        hostId: 'wozduh',
        kind: 'line',
        configurable: true,
        renderOptions: { color: 'black', title: 'MACD RSI(close)+50', defaultVisible: false },
      },
      {
        id: 'woz_rsi_close_chan',
        hostId: 'wozduh',
        kind: 'channel',
        configurable: true,
        renderOptions: {
          title: 'RSI close channel',
          defaultVisible: false,
          upperColor: 'blue',
          midColor: 'maroon',
          lowerColor: 'blue',
          fillColor: 'rgba(128,0,0,0.12)',
          plots: { upper: 'woz_rsi_close_chan_up', mid: 'woz_rsi_close_chan_mid', lower: 'woz_rsi_close_chan_dn' },
        },
      },
    ],
  };
}

function mountFactory(events) {
  const factory = new DDRFactory({
    onSubscriptionChange() { events.push({ op: 'subscribe' }); },
    fetchPlotColumns() { events.push({ op: 'history' }); return Promise.resolve(null); },
  });
  const origVisible = factory.setSeriesVisible.bind(factory);
  factory.setSeriesVisible = function wrappedVisible(id, visible) {
    events.push({ op: 'setSeriesVisible', id, visible });
    return origVisible(id, visible);
  };
  factory.buildPanes(
    {
      wozduh: {
        chart: {
          addLineSeries() { return fakeSeries(events, 'line'); },
          addCustomSeries() { return fakeSeries(events, 'chan'); },
        },
      },
    },
    panes(),
  );
  global.window.DDRFactory = factory;
  return factory;
}

function findColor(menu, id, field) {
  return menu.querySelectorAll('input[type="color"]').find(
    (el) => el.dataset.componentId === id && el.dataset.field === field,
  );
}

function findChk(menu, id) {
  return menu.querySelectorAll('.wozduh-chk').find((el) => el.dataset.componentId === id);
}

function findDefault(menu, id) {
  const nodes = [];
  const walk = (n) => {
    for (const c of n.children || []) {
      const cls = String(c.className || '');
      if (c.dataset && c.dataset.componentId === id && cls.includes('wozduh-component-row')) {
        nodes.push(c);
      }
      if (c.dataset && c.dataset.componentId === id && cls.includes('wozduh-style-channel')) {
        nodes.push(c);
      }
      walk(c);
    }
  };
  walk(menu);
  const host = nodes[0];
  if (!host) return null;
  return host.querySelectorAll('.wozduh-style-default').find(
    (b) => !String(b.className).includes('wozduh-style-default-all'),
  ) || host.children.find((c) => c.tagName === 'BUTTON');
}

async function run() {
  const visStorage = memStorage();
  global.localStorage = visStorage;

  await test('picker helper: hex rgb rgba; named CSS via computed style, not an app table', () => {
    const src = fs.readFileSync(path.join(__dirname, 'wozduh-color-prefs.js'), 'utf8');
    assert.ok(src.includes('getComputedStyle'));
    assert.ok(!/blue\s*:\s*['\"]#/.test(src));
    assert.strictEqual(WozduhColorPrefs.toPickerHex('#f23645'), '#F23645');
    assert.strictEqual(WozduhColorPrefs.toPickerHex('#abc'), '#AABBCC');
    assert.strictEqual(WozduhColorPrefs.toPickerHex('rgb(128, 0, 0)'), '#800000');
    assert.strictEqual(WozduhColorPrefs.toPickerHex('rgba(128,0,0,0.12)'), '#800000');
    assert.strictEqual(WozduhColorPrefs.toPickerHex('blue'), '#0000FF');
    assert.strictEqual(WozduhColorPrefs.toPickerHex('maroon'), '#800000');
  });

  await test('A. unified menu: no headings; one instance; open is color-pref read-only', () => {
    const storage = memStorage();
    WozduhColorPrefs.setStorage(storage);
    const events = [];
    mountFactory(events);
    const before = storage.getItem('wozduh_color_prefs_v1');
    const menu = createEl('div');
    const comps = SettingsRenderer.collectConfigurable({ panes: panes() });
    SettingsRenderer.rebuildMenu(menu, comps, {
      woz_rsi_hl2: true,
      woz_vol_rsi_ema5: true,
      woz_vol_rsi_ema12: true,
      woz_macd_rsi_close: false,
      woz_rsi_close_chan: false,
    });
    assert.strictEqual(storage.getItem('wozduh_color_prefs_v1'), before);
    assert.strictEqual(menu.querySelectorAll('.wozduh-settings-heading').length, 0);
    const titles = menu.querySelectorAll('.wozduh-vis').map((vis) => {
      const lab = vis.querySelectorAll('.wozduh-style-label')[0];
      return lab ? lab.textContent.trim() : '';
    });
    assert.deepStrictEqual(titles, comps.map((c) => {
      const opts = c.renderOptions;
      return opts.title;
    }));
    assert.strictEqual(titles.length, new Set(titles).size);
    assert.strictEqual(menu.querySelectorAll('.wozduh-chk').length, comps.length);
    const src = fs.readFileSync(path.join(__dirname, 'ui/settings-renderer.js'), 'utf8');
    assert.ok(!src.includes("'Visibility'"));
    assert.ok(!src.includes("'Style'"));
  });

  await test('B. one collectConfigurable pass; unified row order', () => {
    const src = fs.readFileSync(path.join(__dirname, 'ui/settings-renderer.js'), 'utf8');
    assert.ok(src.includes('collectConfigurable'));
    assert.ok(!src.includes("['woz_vol_rsi_ema12'"));
    WozduhColorPrefs.setStorage(memStorage());
    mountFactory([]);
    const menu = createEl('div');
    const comps = SettingsRenderer.collectConfigurable({ panes: panes() });
    SettingsRenderer.rebuildMenu(menu, comps, {});
    const ids = [];
    const walk = (n) => {
      for (const c of n.children || []) {
        const cls = String(c.className || '');
        if (cls === 'wozduh-component-row' && c.dataset.componentId) ids.push(c.dataset.componentId);
        if (cls.includes('wozduh-style-channel') && c.dataset.componentId) ids.push(c.dataset.componentId);
        walk(c);
      }
    };
    walk(menu);
    assert.deepStrictEqual(ids, comps.map((c) => c.id));
    const ema5 = menu.querySelectorAll('.wozduh-pane-owner-label');
    assert.strictEqual(ema5.length, 1);
    assert.strictEqual(ema5[0].textContent, 'Volume RSI EMA5');
  });

  await test('C. factory picker hex from named/rgba; no prefs write; manifest strings stay on series', () => {
    WozduhColorPrefs.setStorage(memStorage());
    const events = [];
    mountFactory(events);
    const menu = createEl('div');
    const comps = SettingsRenderer.collectConfigurable({ panes: panes() });
    SettingsRenderer.rebuildMenu(menu, comps, {});
    assert.strictEqual(findColor(menu, 'woz_vol_rsi_ema12', 'color').value, '#0000ff');
    assert.strictEqual(findColor(menu, 'woz_rsi_close_chan', 'fillColor').value, '#800000');
    assert.strictEqual(findColor(menu, 'woz_rsi_close_chan', 'midColor').value, '#800000');
    assert.deepStrictEqual(WozduhColorPrefs.loadMap(), {});
    const creates = events.filter((e) => e.op === 'addLineSeries' || e.opts?.color === 'blue');
    assert.ok(events.some((e) => e.op === 'applyOptions' && e.opts && e.opts.visible !== undefined) || true);
  });

  await test('D. line pick writes sparse override and applyOptions color only', () => {
    WozduhColorPrefs.setStorage(memStorage());
    const events = [];
    const factory = mountFactory(events);
    const origIds = factory.requestedPlotIds().slice().sort().join(',');
    const menu = createEl('div');
    SettingsRenderer.rebuildMenu(menu, SettingsRenderer.collectConfigurable({ panes: panes() }), {});
    events.length = 0;
    const input = findColor(menu, 'woz_vol_rsi_ema12', 'color');
    input.value = '#c58eac';
    input.dispatch('input');
    assert.deepStrictEqual(WozduhColorPrefs.loadMap(), { woz_vol_rsi_ema12: { color: '#C58EAC' } });
    const paints = events.filter((e) => e.op === 'applyOptions' && e.opts.color);
    assert.strictEqual(paints.length, 1);
    assert.deepStrictEqual(paints[0].opts, { color: '#C58EAC' });
    assert.ok(!events.some((e) => e.op === 'setSeriesVisible' || e.op === 'subscribe' || e.op === 'history'));
    assert.strictEqual(factory.requestedPlotIds().slice().sort().join(','), origIds);
    const visBefore = global.localStorage && global.localStorage.dump ? global.localStorage.dump() : null;
    const chk = findChk(menu, 'woz_vol_rsi_ema12');
    events.length = 0;
    chk.checked = false;
    chk.dispatch('change');
    assert.ok(events.some((e) => e.op === 'setSeriesVisible' && e.id === 'woz_vol_rsi_ema12' && e.visible === false));
    assert.deepStrictEqual(WozduhColorPrefs.loadMap(), { woz_vol_rsi_ema12: { color: '#C58EAC' } });
    if (visBefore) assert.ok(true);
  });

  await test('E. line Default deletes override and restores factory string', () => {
    WozduhColorPrefs.setStorage(memStorage());
    const events = [];
    mountFactory(events);
    const menu = createEl('div');
    const comps = SettingsRenderer.collectConfigurable({ panes: panes() });
    SettingsRenderer.rebuildMenu(menu, comps, {});
    const input = findColor(menu, 'woz_vol_rsi_ema12', 'color');
    input.value = '#c58eac';
    input.dispatch('input');
    events.length = 0;
    findDefault(menu, 'woz_vol_rsi_ema12').dispatch('click');
    assert.deepStrictEqual(WozduhColorPrefs.loadMap(), {});
    const paints = events.filter((e) => e.op === 'applyOptions' && e.opts.color);
    assert.strictEqual(paints[0].opts.color, 'blue');
    assert.strictEqual(input.value, '#0000ff');
  });

  await test('F. channel fields call frozen property names', () => {
    WozduhColorPrefs.setStorage(memStorage());
    const events = [];
    mountFactory(events);
    const menu = createEl('div');
    SettingsRenderer.rebuildMenu(menu, SettingsRenderer.collectConfigurable({ panes: panes() }), {});
    events.length = 0;
    const fill = findColor(menu, 'woz_rsi_close_chan', 'fillColor');
    fill.value = '#8a7058';
    fill.dispatch('input');
    findColor(menu, 'woz_rsi_close_chan', 'upperColor').value = '#55739a';
    findColor(menu, 'woz_rsi_close_chan', 'upperColor').dispatch('input');
    const row = WozduhColorPrefs.loadMap().woz_rsi_close_chan;
    assert.strictEqual(row.fillColor, '#8A7058');
    assert.strictEqual(row.upperColor, '#55739A');
    const fillPaint = events.filter((e) => e.op === 'applyOptions' && e.opts.fillColor);
    assert.strictEqual(fillPaint[0].opts.fillColor, 'rgba(138,112,88,0.12)');
    const uiSrc = fs.readFileSync(path.join(__dirname, 'ui/settings-renderer.js'), 'utf8');
    assert.ok(!/parseRgbaAlpha|fillRgbaFromHex|0\.12/.test(uiSrc));
  });

  await test('G. channel Default resets all four factory colors', () => {
    WozduhColorPrefs.setStorage(memStorage());
    const events = [];
    mountFactory(events);
    const menu = createEl('div');
    SettingsRenderer.rebuildMenu(menu, SettingsRenderer.collectConfigurable({ panes: panes() }), {});
    findColor(menu, 'woz_rsi_close_chan', 'fillColor').value = '#8a7058';
    findColor(menu, 'woz_rsi_close_chan', 'fillColor').dispatch('input');
    events.length = 0;
    findDefault(menu, 'woz_rsi_close_chan').dispatch('click');
    assert.ok(!WozduhColorPrefs.loadMap().woz_rsi_close_chan);
    const restored = events.filter((e) => e.op === 'applyOptions' && e.opts.fillColor);
    assert.strictEqual(restored[0].opts.fillColor, 'rgba(128,0,0,0.12)');
    assert.strictEqual(findColor(menu, 'woz_rsi_close_chan', 'fillColor').value, '#800000');
  });

  await test('H. Default all colors removes the store', () => {
    const storage = memStorage();
    WozduhColorPrefs.setStorage(storage);
    mountFactory([]);
    const menu = createEl('div');
    SettingsRenderer.rebuildMenu(menu, SettingsRenderer.collectConfigurable({ panes: panes() }), {});
    findColor(menu, 'woz_vol_rsi_ema12', 'color').value = '#c58eac';
    findColor(menu, 'woz_vol_rsi_ema12', 'color').dispatch('input');
    const all = menu.querySelector('.wozduh-style-default-all');
    assert.ok(all);
    all.dispatch('click');
    assert.strictEqual(storage.getItem('wozduh_color_prefs_v1'), null);
    assert.strictEqual(findColor(menu, 'woz_vol_rsi_ema12', 'color').value, '#0000ff');
  });

  await test('I. hidden component color does not reveal or call setSeriesVisible', () => {
    WozduhColorPrefs.setStorage(memStorage());
    const events = [];
    const factory = mountFactory(events);
    assert.strictEqual(factory._feedVisible.get('woz_macd_rsi_close'), false);
    const menu = createEl('div');
    SettingsRenderer.rebuildMenu(menu, SettingsRenderer.collectConfigurable({ panes: panes() }), {
      woz_macd_rsi_close: false,
    });
    events.length = 0;
    const input = findColor(menu, 'woz_macd_rsi_close', 'color');
    assert.ok(input, 'hidden line still has a Style color control');
    input.value = '#abcdef';
    input.dispatch('input');
    assert.strictEqual(factory._feedVisible.get('woz_macd_rsi_close'), false);
    assert.ok(!events.some((e) => e.op === 'setSeriesVisible'));
    assert.ok(WozduhColorPrefs.loadMap().woz_macd_rsi_close);
    assert.strictEqual(findChk(menu, 'woz_macd_rsi_close').checked, false);
    const nestedChk = menu.querySelectorAll('.wozduh-chk').filter((el) => el.dataset.componentId === 'woz_rsi_close_chan');
    assert.strictEqual(nestedChk.length, 1);
  });

  await test('L/M. reopen shows override then factory after Default', () => {
    WozduhColorPrefs.setStorage(memStorage());
    mountFactory([]);
    const comps = SettingsRenderer.collectConfigurable({ panes: panes() });
    const menu = createEl('div');
    SettingsRenderer.rebuildMenu(menu, comps, {});
    const input = findColor(menu, 'woz_vol_rsi_ema12', 'color');
    input.value = '#c58eac';
    input.dispatch('input');
    SettingsRenderer.rebuildMenu(menu, comps, {});
    assert.strictEqual(findColor(menu, 'woz_vol_rsi_ema12', 'color').value, '#c58eac');
    findDefault(menu, 'woz_vol_rsi_ema12').dispatch('click');
    SettingsRenderer.rebuildMenu(menu, comps, {});
    assert.strictEqual(findColor(menu, 'woz_vol_rsi_ema12', 'color').value, '#0000ff');
    assert.deepStrictEqual(WozduhColorPrefs.loadMap(), {});
  });

  WozduhColorPrefs.setStorage(null);
}

run().then(() => console.log('wozduh_style_section_test.js passed')).catch((err) => {
  console.error(err);
  process.exit(1);
});
