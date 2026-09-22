/**
 * PRICE-SERIES-STYLE-1 — catalog, sparse prefs, OHLC projection, one priceSeries.
 * Run: node web/price_series_style_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

function memStorage() {
  const map = Object.create(null);
  return {
    getItem(k) { return Object.prototype.hasOwnProperty.call(map, k) ? map[k] : null; },
    setItem(k, v) { map[k] = String(v); },
    removeItem(k) { delete map[k]; },
  };
}

global.localStorage = memStorage();
global.document = {
  getElementById() { return null; },
  addEventListener() {},
};

const { PriceStyleController: PSC } = require('./ui/price-style-controller.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

test('A. catalog is candles/bars/line only', () => {
  const ids = PSC.catalog.map((e) => e.id);
  assert.deepStrictEqual(ids, ['candles', 'bars', 'line']);
  assert.ok(PSC.catalog.every((e) => e.defaultFavorite === true));
  assert.strictEqual(PSC.DEFAULT_STYLE, 'candles');
  assert.strictEqual(PSC.normalizeStyle('renko'), 'candles');
});

test('B. line projects close; candles/bars keep OHLC', () => {
  const ohlc = [
    { time: 1, open: 10, high: 12, low: 9, close: 11 },
    { time: 2, open: 11, high: 13, low: 10, close: 12 },
  ];
  assert.strictEqual(PSC.projectPricePoints('candles', ohlc), ohlc);
  assert.strictEqual(PSC.projectPricePoints('bars', ohlc), ohlc);
  assert.deepStrictEqual(PSC.projectPricePoints('line', ohlc), [
    { time: 1, value: 11 },
    { time: 2, value: 12 },
  ]);
  assert.deepStrictEqual(PSC.projectPricePoint('line', ohlc[0]), { time: 1, value: 11 });
});

test('C. sparse prefs: default omitted; bars persisted; favorites only when changed', () => {
  global.localStorage = memStorage();
  PSC.init();
  assert.strictEqual(PSC.getCurrent(), 'candles');
  assert.strictEqual(global.localStorage.getItem(PSC.PREFS_KEY), null);
  const calls = [];
  global.ChartAdapter = {
    setPriceStyle(id) { calls.push(id); },
  };
  PSC.applyStyle('bars');
  assert.strictEqual(PSC.getCurrent(), 'bars');
  assert.deepStrictEqual(JSON.parse(global.localStorage.getItem(PSC.PREFS_KEY)), { currentStyle: 'bars' });
  assert.deepStrictEqual(calls, ['bars']);
  PSC.toggleFavorite('line');
  const saved = JSON.parse(global.localStorage.getItem(PSC.PREFS_KEY));
  assert.strictEqual(saved.currentStyle, 'bars');
  assert.deepStrictEqual(saved.favorites, ['candles', 'bars']);
});

test('D. chart-core: one priceSeries; swap uses painted window; no hydrate', () => {
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  assert.ok(!/\bcandleSeries\b/.test(core));
  assert.ok(core.includes('priceSeries'));
  assert.ok(core.includes('setPriceStyle'));
  assert.ok(core.includes('addBarSeries'));
  assert.ok(core.includes('addLineSeries'));
  assert.ok(core.includes('addCandlestickSeries'));
  const swap = core.slice(core.indexOf('setPriceStyle(id)'), core.indexOf('getChart(context'));
  assert.ok(swap.includes('_realCandles'));
  assert.ok(swap.includes('removeSeries'));
  assert.ok(!swap.includes('loadDashboard'));
  assert.ok(!swap.includes('/api/history'));
  assert.ok(!swap.includes('TimeCamera.commit'));
  assert.ok(!/\bstate\.barSeries\b|\bstate\.lineSeries\b|\bcandleSeries\b/.test(core));
});

test('E. toolbar dropped Reload and labeled type buttons; favorites live on the bar', () => {
  const html = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8');
  const toolbar = fs.readFileSync(path.join(__dirname, 'ui/toolbar-controller.js'), 'utf8');
  assert.ok(!html.includes('btn-clear-cache'));
  assert.ok(!html.includes('🔄 Reload'));
  assert.ok(!html.includes('chart-type-group'));
  assert.ok(html.includes('price-style-bar'));
  assert.ok(html.includes('price-style-favorites'));
  assert.ok(!html.includes('id="price-style-favs"'));
  assert.ok(!toolbar.includes('setChartType'));
  assert.ok(!toolbar.includes('btn-clear-cache'));
  const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
  assert.ok(boot.includes('reloadDashboard'));
});

test('F. starring updates toolbar favorites; independent of current style', () => {
  global.localStorage = memStorage();
  const ids = [];
  global.document = {
    getElementById(id) {
      if (id === 'price-style-favorites') {
        return {
          children: ids,
          set innerHTML(v) { if (v === '') ids.length = 0; },
          get innerHTML() { return ''; },
          appendChild(c) { this.children.push(c); return c; },
        };
      }
      return null;
    },
    createElement() {
      return { type: '', className: '', dataset: {}, title: '', innerHTML: '', setAttribute() {} };
    },
    addEventListener() {},
  };
  PSC.init();
  assert.deepStrictEqual(PSC.getFavorites(), ['candles', 'bars', 'line']);
  assert.strictEqual(ids.length, 3);
  PSC.applyStyle('line');
  assert.strictEqual(PSC.getCurrent(), 'line');
  assert.deepStrictEqual(PSC.getFavorites(), ['candles', 'bars', 'line']);
  PSC.toggleFavorite('bars');
  assert.strictEqual(PSC.getCurrent(), 'line');
  assert.deepStrictEqual(PSC.getFavorites(), ['candles', 'line']);
  assert.strictEqual(ids.length, 2);
});

test('G. line gear toggles without changing style or favorites', () => {
  global.localStorage = memStorage();
  global.document = { getElementById() { return null; }, addEventListener() {} };
  PSC.init();
  PSC.applyStyle('candles');
  const favs = PSC.getFavorites();
  assert.strictEqual(PSC.isLineGearOpen(), false);
  PSC.toggleLineGear();
  assert.strictEqual(PSC.isLineGearOpen(), true);
  assert.strictEqual(PSC.getCurrent(), 'candles');
  assert.deepStrictEqual(PSC.getFavorites(), favs);
  PSC.toggleLineGear();
  assert.strictEqual(PSC.isLineGearOpen(), false);
});

test('H. line paint is sparse, applyOptions-only, survives style switch, factory reset', () => {
  global.localStorage = memStorage();
  global.document = { getElementById() { return null; }, addEventListener() {} };
  const paints = [];
  const styles = [];
  global.ChartAdapter = {
    setPriceStyle(id) { styles.push(id); },
    applyPriceLinePaint(opts) { paints.push({ ...opts }); return true; },
  };
  PSC.init();
  const factory = PSC.factoryLinePaint();
  PSC.setLinePaint({ color: '#c58eac', lineWidth: 3 });
  assert.deepStrictEqual(PSC.resolveLinePaint(), { color: '#C58EAC', lineWidth: 3 });
  assert.deepStrictEqual(JSON.parse(global.localStorage.getItem(PSC.PREFS_KEY)).line, {
    color: '#C58EAC',
    lineWidth: 3,
  });
  PSC.applyStyle('candles');
  assert.deepStrictEqual(PSC.resolveLinePaint(), { color: '#C58EAC', lineWidth: 3 });
  PSC.applyStyle('line');
  assert.deepStrictEqual(PSC.resolveLinePaint(), { color: '#C58EAC', lineWidth: 3 });
  PSC.resetLinePaint();
  assert.deepStrictEqual(PSC.resolveLinePaint(), factory);
  const raw = global.localStorage.getItem(PSC.PREFS_KEY);
  if (raw) assert.ok(!JSON.parse(raw).line);
  assert.ok(paints.length >= 2);
  assert.ok(paints.every((p) => p.color && Number.isFinite(p.lineWidth)));
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  const paintFn = core.slice(core.indexOf('applyPriceLinePaint(opts)'), core.indexOf('getChart(context'));
  assert.ok(paintFn.includes('applyOptions'));
  assert.ok(!paintFn.includes('setData'));
  assert.ok(!paintFn.includes('removeSeries'));
  assert.ok(!paintFn.includes('loadDashboard'));
});

console.log('price_series_style_test.js passed');
