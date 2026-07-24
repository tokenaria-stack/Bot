/**
 * ADR-020 ScaleController unit tests (Node).
 * Run: node web/scale_controller_test.js
 */
'use strict';

const assert = require('assert');
const ScaleController = require('./ui/scale-controller.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function memoryStorage(seed) {
  const map = new Map(Object.entries(seed || {}));
  return {
    getItem(k) { return map.has(k) ? map.get(k) : null; },
    setItem(k, v) { map.set(k, String(v)); },
    removeItem(k) { map.delete(k); },
  };
}

function countingStorage(seed) {
  const store = memoryStorage(seed);
  let writes = 0;
  const innerSet = store.setItem.bind(store);
  store.setItem = (k, v) => {
    writes += 1;
    innerSet(k, v);
  };
  store.writes = () => writes;
  return store;
}

function fakeChart(initialAuto = true) {
  let auto = initialAuto;
  let mode = 0;
  return {
    priceScale() {
      return {
        options: () => ({ autoScale: auto }),
        applyOptions: (opts) => {
          if (Object.prototype.hasOwnProperty.call(opts, 'autoScale')) auto = !!opts.autoScale;
          if (Object.prototype.hasOwnProperty.call(opts, 'mode')) mode = opts.mode;
        },
        width: () => 64,
        _mode: () => mode,
        _auto: () => auto,
      };
    },
  };
}

// ─── Pure repairScalePrefs ───────────────────────────────────────────────────

test('repair: Auto OFF without range → Auto ON, preserve Log, dirty', () => {
  const { prefs, dirty } = ScaleController.repairScalePrefs({
    version: 3,
    panes: { price: { isAuto: false, isLog: true } },
  });
  assert.strictEqual(dirty, true);
  assert.strictEqual(prefs.panes.price.isAuto, true);
  assert.strictEqual(prefs.panes.price.isLog, true);
});

test('repair: Auto OFF with valid manualRange unchanged', () => {
  const input = {
    version: 3,
    panes: {
      price: { isAuto: false, isLog: false, manualRange: { min: 100, max: 200 } },
    },
  };
  const { prefs, dirty } = ScaleController.repairScalePrefs(input);
  assert.strictEqual(dirty, false);
  assert.strictEqual(prefs.panes.price.isAuto, false);
  assert.deepStrictEqual(prefs.panes.price.manualRange, { min: 100, max: 200 });
});

test('repair: Auto ON unchanged', () => {
  const { prefs, dirty } = ScaleController.repairScalePrefs({
    version: 3,
    panes: { price: { isAuto: true, isLog: true } },
  });
  assert.strictEqual(dirty, false);
  assert.strictEqual(prefs.panes.price.isAuto, true);
  assert.strictEqual(prefs.panes.price.isLog, true);
});

test('repair: footer Auto OFF without range → Auto ON', () => {
  const { prefs, dirty } = ScaleController.repairScalePrefs({
    version: 3,
    panes: { rsx: { isAuto: false, isLog: false } },
  });
  assert.strictEqual(dirty, true);
  assert.strictEqual(prefs.panes.rsx.isAuto, true);
});

test('repair: invalid manualRange (min>=max) repairs Auto ON', () => {
  const { prefs, dirty } = ScaleController.repairScalePrefs({
    version: 3,
    panes: { price: { isAuto: false, manualRange: { min: 5, max: 5 } } },
  });
  assert.strictEqual(dirty, true);
  assert.strictEqual(prefs.panes.price.isAuto, true);
  assert.deepStrictEqual(prefs.panes.price.manualRange, { min: 5, max: 5 });
});

test('repair: preserves unknown forward-compat fields', () => {
  const { prefs, dirty } = ScaleController.repairScalePrefs({
    version: 3,
    panes: {
      price: { isAuto: false, isLog: true, scaleGroup: 'macd', meta: { v: 1 } },
    },
  });
  assert.strictEqual(dirty, true);
  assert.strictEqual(prefs.panes.price.scaleGroup, 'macd');
  assert.deepStrictEqual(prefs.panes.price.meta, { v: 1 });
});

test('repair: idempotent second pass dirty=false', () => {
  const first = ScaleController.repairScalePrefs({
    version: 3,
    panes: { price: { isAuto: false, isLog: true } },
  });
  assert.strictEqual(first.dirty, true);
  const second = ScaleController.repairScalePrefs(first.prefs);
  assert.strictEqual(second.dirty, false);
  assert.strictEqual(second.prefs.panes.price.isAuto, true);
  assert.strictEqual(second.prefs.panes.price.isLog, true);
});

test('hasValidManualRange socket', () => {
  assert.strictEqual(ScaleController.hasValidManualRange({}), false);
  assert.strictEqual(ScaleController.hasValidManualRange({ manualRange: { min: 1, max: 2 } }), true);
  assert.strictEqual(ScaleController.hasValidManualRange({ manualRange: { min: 2, max: 1 } }), false);
});

// ─── Hydrate / migrate ───────────────────────────────────────────────────────

test('v2 migrate Auto OFF+Log → repair once to Auto ON; second init no rewrite', () => {
  const store = countingStorage({
    [ScaleController.STORAGE_KEY_LEGACY]: JSON.stringify({ isAuto: false, isLog: true }),
  });
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  assert.strictEqual(ScaleController.getState('live', 'price').isAuto, true);
  assert.strictEqual(ScaleController.getState('live', 'price').isLog, true);
  assert.strictEqual(ScaleController.getState('live', 'rsx').isAuto, true);
  const raw = JSON.parse(store.getItem(ScaleController.STORAGE_KEY));
  assert.strictEqual(raw.panes.price.isAuto, true);
  assert.strictEqual(raw.panes.price.isLog, true);
  const writesAfterFirst = store.writes();
  assert.ok(writesAfterFirst >= 1, 'first hydrate persists repair');

  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  assert.strictEqual(store.writes(), writesAfterFirst, 'second init does not rewrite healthy prefs');
});

test('v3 invalid Auto OFF repairs on init and persists once', () => {
  const store = countingStorage({
    [ScaleController.STORAGE_KEY]: JSON.stringify({
      version: 3,
      panes: { price: { isAuto: false, isLog: true } },
    }),
  });
  ScaleController._resetForTests({ storage: store });
  assert.strictEqual(ScaleController.getState('live', 'price').isAuto, true);
  assert.strictEqual(ScaleController.getState('live', 'price').isLog, true);
  const writes = store.writes();
  assert.strictEqual(writes, 1);
  ScaleController.init({ storage: store });
  assert.strictEqual(store.writes(), 1);
});

test('v3 Auto OFF + valid manualRange survives init', () => {
  const store = countingStorage({
    [ScaleController.STORAGE_KEY]: JSON.stringify({
      version: 3,
      panes: {
        price: { isAuto: false, isLog: false, manualRange: { min: 10, max: 20 } },
      },
    }),
  });
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  assert.strictEqual(ScaleController.getState('live', 'price').isAuto, false);
  assert.deepStrictEqual(ScaleController.getState('live', 'price').manualRange, { min: 10, max: 20 });
  assert.strictEqual(store.writes(), 0);
});

test('register HostID; toggleAuto independent; toggleLog blocked when allowLog false', () => {
  const store = memoryStorage();
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });

  const price = fakeChart(true);
  const rsx = fakeChart(true);
  ScaleController.register({
    context: 'live', hostId: 'price', chart: price, allowLog: true, scaleGroup: 'price',
  });
  ScaleController.register({
    context: 'live', hostId: 'rsx', chart: rsx, allowLog: false,
  });

  assert.strictEqual(ScaleController.toggleAuto('live', 'rsx'), true);
  assert.strictEqual(ScaleController.getState('live', 'rsx').isAuto, false);
  assert.strictEqual(ScaleController.getState('live', 'price').isAuto, true);
  assert.strictEqual(rsx.priceScale()._auto(), false);
  assert.strictEqual(price.priceScale()._auto(), true);

  assert.strictEqual(ScaleController.toggleLog('live', 'rsx'), false);
  assert.strictEqual(ScaleController.getState('live', 'rsx').isLog, false);
  assert.strictEqual(ScaleController.toggleLog('live', 'price'), true);
  assert.strictEqual(ScaleController.getState('live', 'price').isLog, true);
});

test('footer manual Auto OFF tracked; restore repair still heals without manualRange', () => {
  const store = countingStorage();
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  const rsx = fakeChart(true);
  ScaleController.register({
    context: 'live', hostId: 'rsx', chart: rsx, allowLog: false,
  });
  // Simulate LWC turning autoScale off after Y-axis drag (attachManualScaleWatch path).
  ScaleController.setPanePrefs('rsx', { isAuto: false });
  assert.strictEqual(ScaleController.getState('live', 'rsx').isAuto, false);
  assert.strictEqual(rsx.priceScale()._auto(), false);
  const mid = JSON.parse(store.getItem(ScaleController.STORAGE_KEY));
  assert.strictEqual(mid.panes.rsx.isAuto, false, 'in-session Manual persists');

  // Reload: invalid Manual (no manualRange) must repair — invariant unchanged.
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  assert.strictEqual(ScaleController.getState('live', 'rsx').isAuto, true);
  assert.strictEqual(JSON.parse(store.getItem(ScaleController.STORAGE_KEY)).panes.rsx.isAuto, true);
});

test('footer Auto OFF + valid manualRange survives restore', () => {
  const store = countingStorage({
    [ScaleController.STORAGE_KEY]: JSON.stringify({
      version: 3,
      panes: {
        rsx: { isAuto: false, isLog: false, manualRange: { min: -5, max: 105 } },
      },
    }),
  });
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  assert.strictEqual(ScaleController.getState('live', 'rsx').isAuto, false);
  assert.strictEqual(store.writes(), 0);
});

test('future hostId atr needs no controller change', () => {
  const store = memoryStorage();
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  const atr = fakeChart();
  ScaleController.register({
    context: 'live', hostId: 'atr', chart: atr, allowLog: false, scaleGroup: 'atr',
  });
  ScaleController.toggleAuto('live', 'atr');
  assert.strictEqual(ScaleController.getState('live', 'atr').isAuto, false);
  assert.strictEqual(atr.priceScale()._auto(), false);
});

test('legacy register(context, chart, host) maps to price', () => {
  const store = memoryStorage();
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  const chart = fakeChart();
  ScaleController.register('live', chart, null);
  ScaleController.toggleLog('live', 'price');
  assert.strictEqual(ScaleController.getState('live', 'price').isLog, true);
});

console.log('scale_controller_test: ALL PASS');
