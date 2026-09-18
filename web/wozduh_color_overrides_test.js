/**
 * WOZDUH-COLOR-OVERRIDES-1 — sparse prefs + applyOptions; no demand/history.
 * Run: node web/wozduh_color_overrides_test.js
 */
'use strict';

const assert = require('assert');
const WozduhColorPrefs = require('./wozduh-color-prefs.js');
const { DDRFactory } = require('./series-factory.js');

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
    lastCreate: null,
    lastOptions: null,
    setData(points) { events.push({ op: 'setData', id, points }); },
    update(pt) { events.push({ op: 'update', id, pt }); },
    applyOptions(opts) {
      events.push({ op: 'applyOptions', id, opts: { ...opts } });
      this.lastOptions = opts;
    },
    priceScale() { return { applyOptions() {} }; },
  };
}

function panes() {
  return {
    pane_osc: [
      {
        id: 'woz_vol_rsi_ema12',
        hostId: 'wozduh',
        kind: 'line',
        renderOptions: { color: 'blue', lineWidth: 2, title: 'Volume RSI EMA12', defaultVisible: true },
      },
      {
        id: 'woz_vol_rsi_ema5',
        hostId: 'wozduh',
        kind: 'line',
        renderOptions: { color: 'aqua', lineWidth: 2, title: 'Volume RSI EMA5', defaultVisible: true },
      },
      {
        id: 'line_rsx',
        hostId: 'rsx',
        kind: 'line',
        renderOptions: { color: '#512DA8', defaultVisible: true },
      },
      {
        id: 'woz_rsi_close_chan',
        hostId: 'wozduh',
        kind: 'channel',
        renderOptions: {
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

function mount(factory, events, createOpts) {
  factory.buildPanes(
    {
      wozduh: {
        chart: {
          addLineSeries(opts) {
            events.push({ op: 'addLineSeries', opts: { ...opts } });
            const s = fakeSeries(events, 'pending');
            s.createOpts = opts;
            (createOpts || []).push({ kind: 'line', opts });
            return s;
          },
          addCustomSeries(_ctor, opts) {
            events.push({ op: 'addCustomSeries', opts: { ...opts } });
            const s = fakeSeries(events, 'chan');
            s.createOpts = opts;
            (createOpts || []).push({ kind: 'channel', opts });
            return s;
          },
        },
      },
      rsx: {
        chart: {
          addLineSeries(opts) {
            events.push({ op: 'addLineRsx', opts: { ...opts } });
            const s = fakeSeries(events, 'line_rsx');
            s.createOpts = opts;
            (createOpts || []).push({ kind: 'rsx', opts });
            return s;
          },
          addCustomSeries() { return fakeSeries(events, 'rsx-chan'); },
        },
      },
    },
    panes(),
  );
}

function colorApplies(events) {
  return events.filter((e) => e.op === 'applyOptions' && (
    'color' in (e.opts || {})
    || 'upperColor' in (e.opts || {})
    || 'midColor' in (e.opts || {})
    || 'lowerColor' in (e.opts || {})
    || 'fillColor' in (e.opts || {})
  ));
}

async function run() {
  await test('A. empty prefs: factory strings on create; no color applyOptions', () => {
    WozduhColorPrefs.setStorage(memStorage());
    const events = [];
    const creates = [];
    const factory = new DDRFactory({
      onSubscriptionChange() { events.push({ op: 'subscribe' }); },
      fetchPlotColumns() { events.push({ op: 'history' }); return Promise.resolve(null); },
    });
    mount(factory, events, creates);
    const line = creates.find((c) => c.kind === 'line' && c.opts.color === 'blue');
    assert.ok(line, 'factory blue must be passed to addLineSeries');
    const aqua = creates.find((c) => c.kind === 'line' && c.opts.color === 'aqua');
    assert.ok(aqua);
    const chan = creates.find((c) => c.kind === 'channel');
    assert.strictEqual(chan.opts.upperColor, 'blue');
    assert.strictEqual(chan.opts.midColor, 'maroon');
    assert.strictEqual(chan.opts.fillColor, 'rgba(128,0,0,0.12)');
    assert.strictEqual(colorApplies(events).length, 0);
    assert.ok(!events.some((e) => e.op === 'subscribe'));
    assert.ok(!events.some((e) => e.op === 'history'));
    assert.strictEqual(WozduhColorPrefs.loadMap()['woz_vol_rsi_ema12'], undefined);
  });

  await test('B. override one line: applyOptions color only; factory create unchanged', () => {
    WozduhColorPrefs.setStorage(memStorage());
    const events = [];
    const creates = [];
    let sub = 0;
    let hist = 0;
    const factory = new DDRFactory({
      onSubscriptionChange() { sub += 1; },
      fetchPlotColumns() { hist += 1; return Promise.resolve(null); },
    });
    mount(factory, events, creates);
    const beforeIds = factory.requestedPlotIds().slice().sort().join(',');
    assert.ok(factory.setWozduhColor('woz_vol_rsi_ema12', 'color', '#c58eac'));
    assert.deepStrictEqual(WozduhColorPrefs.loadMap(), {
      woz_vol_rsi_ema12: { color: '#C58EAC' },
    });
    const paints = colorApplies(events);
    assert.strictEqual(paints.length, 1);
    assert.deepStrictEqual(paints[0].opts, { color: '#C58EAC' });
    assert.ok(!('visible' in paints[0].opts));
    assert.ok(!('lineWidth' in paints[0].opts));
    const blueCreate = creates.find((c) => c.kind === 'line' && c.opts.color === 'blue');
    assert.strictEqual(blueCreate.opts.color, 'blue');
    assert.strictEqual(sub, 0);
    assert.strictEqual(hist, 0);
    assert.strictEqual(factory.requestedPlotIds().slice().sort().join(','), beforeIds);
  });

  await test('C. reset line restores factory string and deletes store key', () => {
    WozduhColorPrefs.setStorage(memStorage());
    const events = [];
    const factory = new DDRFactory({
      onSubscriptionChange() { events.push({ op: 'subscribe' }); },
    });
    mount(factory, events);
    factory.setWozduhColor('woz_vol_rsi_ema12', 'color', '#C58EAC');
    events.length = 0;
    assert.ok(factory.resetWozduhColor('woz_vol_rsi_ema12', 'color'));
    assert.deepStrictEqual(WozduhColorPrefs.loadMap(), {});
    const paints = colorApplies(events);
    assert.strictEqual(paints.length, 1);
    assert.deepStrictEqual(paints[0].opts, { color: 'blue' });
    assert.ok(!events.some((e) => e.op === 'subscribe'));
  });

  await test('D. channel four fields; fill keeps factory alpha', () => {
    WozduhColorPrefs.setStorage(memStorage());
    const events = [];
    const factory = new DDRFactory({
      onSubscriptionChange() { events.push({ op: 'subscribe' }); },
      fetchPlotColumns() { events.push({ op: 'history' }); return Promise.resolve(null); },
    });
    mount(factory, events);
    assert.ok(factory.setWozduhColor('woz_rsi_close_chan', 'upperColor', '#55739A'));
    assert.ok(factory.setWozduhColor('woz_rsi_close_chan', 'midColor', '#800000'));
    assert.ok(factory.setWozduhColor('woz_rsi_close_chan', 'lowerColor', '#55739A'));
    assert.ok(factory.setWozduhColor('woz_rsi_close_chan', 'fillColor', '#8A7058'));
    const row = WozduhColorPrefs.loadMap().woz_rsi_close_chan;
    assert.strictEqual(row.fillColor, '#8A7058');
    const fills = events.filter((e) => e.op === 'applyOptions' && e.opts.fillColor);
    assert.strictEqual(fills.length, 1);
    assert.strictEqual(fills[0].opts.fillColor, 'rgba(138,112,88,0.12)');
    assert.ok(!events.some((e) => e.op === 'subscribe' || e.op === 'history'));
    assert.strictEqual(
      WozduhColorPrefs.fillRgbaFromHex('#8A7058', 'rgba(128,0,0,0.18)'),
      'rgba(138,112,88,0.18)',
    );
    assert.strictEqual(WozduhColorPrefs.parseRgbaAlpha('rgba(0,136,255,0.12)'), 0.12);
    assert.strictEqual(WozduhColorPrefs.parseRgbaAlpha(null), null);
    assert.strictEqual(WozduhColorPrefs.fillRgbaFromHex('#8A7058', null), null);
    const prefsSrc = require('fs').readFileSync(require('path').join(__dirname, 'wozduh-color-prefs.js'), 'utf8');
    assert.ok(!prefsSrc.includes('0.12'), 'override module must not hardcode fill alpha');
  });

  await test('E. reset component / reset all restore factory channel rgba', () => {
    WozduhColorPrefs.setStorage(memStorage());
    const events = [];
    const factory = new DDRFactory();
    mount(factory, events);
    factory.setWozduhColor('woz_vol_rsi_ema12', 'color', '#111111');
    factory.setWozduhColor('woz_rsi_close_chan', 'fillColor', '#8A7058');
    events.length = 0;
    factory.resetWozduhComponentColors('woz_rsi_close_chan');
    assert.ok(!WozduhColorPrefs.loadMap().woz_rsi_close_chan);
    assert.ok(WozduhColorPrefs.loadMap().woz_vol_rsi_ema12);
    const chanReset = colorApplies(events).pop();
    assert.strictEqual(chanReset.opts.fillColor, 'rgba(128,0,0,0.12)');
    assert.strictEqual(chanReset.opts.upperColor, 'blue');
    events.length = 0;
    factory.resetAllWozduhColors();
    assert.deepStrictEqual(WozduhColorPrefs.loadMap(), {});
    const afterAll = colorApplies(events);
    assert.ok(afterAll.some((e) => e.opts.color === 'blue' || e.opts.color === 'aqua'));
  });

  await test('F. visibility store and setSeriesVisible path stay independent', () => {
    const storage = memStorage({ wozduh_visibility_prefs_live: '{"woz_vol_rsi_ema12":true}' });
    WozduhColorPrefs.setStorage(storage);
    const events = [];
    let sub = 0;
    const factory = new DDRFactory({
      onSubscriptionChange() { sub += 1; },
    });
    mount(factory, events);
    factory.setWozduhColor('woz_vol_rsi_ema12', 'color', '#ABCDEF');
    assert.strictEqual(storage.dump().wozduh_visibility_prefs_live, '{"woz_vol_rsi_ema12":true}');
    assert.ok(!storage.dump().wozduh_color_prefs_v1.includes('visible'));
    const subBeforeToggle = sub;
    factory.setSeriesVisible('woz_rsi_close_chan', true);
    assert.ok(sub > subBeforeToggle);
    factory.setWozduhColor('woz_vol_rsi_ema5', 'color', '#00FF00');
    assert.strictEqual(sub, subBeforeToggle + 1);
  });

  await test('G. remount reapplies override; hydrate does not rewrite factory', () => {
    WozduhColorPrefs.setStorage(memStorage());
    const events = [];
    const creates = [];
    const factory = new DDRFactory({
      onSubscriptionChange() { events.push({ op: 'subscribe' }); },
    });
    mount(factory, events, creates);
    factory.setWozduhColor('woz_vol_rsi_ema12', 'color', '#C58EAC');
    factory.hydrateFromColumnar({
      times: [1, 2],
      plots: { woz_vol_rsi_ema12: [10, 11], woz_vol_rsi_ema5: [20, 21] },
    });
    factory.applyHydratedData();
    assert.ok(events.some((e) => e.op === 'setData'));
    const paintsBeforeClear = colorApplies(events).length;
    factory.clear();
    events.length = 0;
    creates.length = 0;
    mount(factory, events, creates);
    const remountLine = creates.find((c) => c.kind === 'line' && c.opts.color === 'blue');
    assert.ok(remountLine, 'recreate still uses factory string');
    const remountPaint = colorApplies(events);
    assert.ok(remountPaint.some((e) => e.opts.color === '#C58EAC'));
    assert.ok(!events.some((e) => e.op === 'subscribe'));
    assert.ok(paintsBeforeClear >= 1);
  });

  await test('H. unknown / stale keys ignored; RSX host not painted from Wozduh store', () => {
    WozduhColorPrefs.setStorage(memStorage({
      wozduh_color_prefs_v1: JSON.stringify({
        woz_fast: { color: '#FFFFFF' },
        line_rsx: { color: '#FF00FF' },
        woz_vol_rsi_ema12: { color: '#C58EAC', lineWidth: 9, visible: false },
      }),
    }));
    const events = [];
    const creates = [];
    const factory = new DDRFactory({
      onSubscriptionChange() { events.push({ op: 'subscribe' }); },
    });
    mount(factory, events, creates);
    const paints = colorApplies(events);
    assert.strictEqual(paints.length, 1);
    assert.deepStrictEqual(paints[0].opts, { color: '#C58EAC' });
    const rsxCreate = creates.find((c) => c.kind === 'rsx');
    assert.strictEqual(rsxCreate.opts.color, '#512DA8');
    assert.ok(!factory.setWozduhColor('line_rsx', 'color', '#000000'));
    assert.ok(!factory.setWozduhColor('woz_vol_rsi_ema12', 'lineWidth', '#000000'));
    assert.ok(!events.some((e) => e.op === 'subscribe'));
  });

  await test('I. invalid hex is rejected; empty store removes key', () => {
    WozduhColorPrefs.setStorage(memStorage());
    WozduhColorPrefs.setColor('woz_vol_rsi_ema12', 'color', 'blue');
    WozduhColorPrefs.setColor('woz_vol_rsi_ema12', 'color', 'rgba(1,2,3,0.5)');
    WozduhColorPrefs.setColor('woz_vol_rsi_ema12', 'color', '#abc');
    assert.deepStrictEqual(WozduhColorPrefs.loadMap(), {});
    WozduhColorPrefs.setColor('woz_vol_rsi_ema12', 'color', '#c58eac');
    WozduhColorPrefs.resetProperty('woz_vol_rsi_ema12', 'color');
    const storage = memStorage();
    WozduhColorPrefs.setStorage(storage);
    WozduhColorPrefs.setColor('woz_vol_rsi_ema12', 'color', '#C58EAC');
    WozduhColorPrefs.resetAll();
    assert.strictEqual(storage.getItem('wozduh_color_prefs_v1'), null);
  });

  WozduhColorPrefs.setStorage(null);
}

run().then(() => console.log('wozduh_color_overrides_test.js passed')).catch((err) => {
  console.error(err);
  process.exit(1);
});
