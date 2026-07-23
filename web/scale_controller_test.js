/**
 * ADR-020 P1 ScaleController unit tests (Node).
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

test('v2 migrate → price prefs; new host defaults Auto ON', () => {
  const store = memoryStorage({
    [ScaleController.STORAGE_KEY_LEGACY]: JSON.stringify({ isAuto: false, isLog: true }),
  });
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  assert.deepStrictEqual(ScaleController.getState('live', 'price'), { isAuto: false, isLog: true });
  assert.deepStrictEqual(ScaleController.getState('live', 'rsx'), { isAuto: true, isLog: false });
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

test('prefs survive without binding (visibility must not reset)', () => {
  const store = memoryStorage();
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  ScaleController.register({
    context: 'live', hostId: 'wozduh', chart: fakeChart(), allowLog: false,
  });
  ScaleController.toggleAuto('live', 'wozduh');
  assert.strictEqual(ScaleController.getState('live', 'wozduh').isAuto, false);
  ScaleController.unregister('live', 'wozduh');
  assert.strictEqual(ScaleController.getState('live', 'wozduh').isAuto, false);
  const raw = JSON.parse(store.getItem(ScaleController.STORAGE_KEY));
  assert.strictEqual(raw.version, 3);
  assert.strictEqual(raw.panes.wozduh.isAuto, false);
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
