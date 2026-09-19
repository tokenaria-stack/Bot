/**
 * WOZDUH-PANE-AUTOSCALE-OWNER-1
 * Pane Auto domain is the Extreme Bands host, not a hideable DDR plot.
 * Run: node web/wozduh_pane_autoscale_owner_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
require('./ui/scale-contribution.js');
const WozduhExtremeBands = require('./wozduh-extreme-bands.js');
const { DDRFactory } = require('./series-factory.js');
const ScaleController = require('./ui/scale-controller.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function memoryStorage() {
  const map = new Map();
  return {
    getItem(k) { return map.has(k) ? map.get(k) : null; },
    setItem(k, v) { map.set(k, String(v)); },
    removeItem(k) { map.delete(k); },
  };
}

function fakeWozChart() {
  let auto = true;
  const created = [];
  return {
    created,
    addLineSeries(opts) {
      const series = {
        opts,
        visible: opts && opts.visible !== false,
        primitive: null,
        removed: false,
        data: null,
        attachPrimitive(p) {
          this.primitive = p;
          if (p && typeof p.attached === 'function') p.attached({ chart: this, series: this });
        },
        detachPrimitive() { this.primitive = null; },
        remove() { this.removed = true; },
        setData(d) { this.data = d; },
        applyOptions(patch) {
          if (patch && Object.prototype.hasOwnProperty.call(patch, 'visible')) {
            this.visible = patch.visible !== false;
          }
        },
        priceScale() { return { applyOptions() {} }; },
      };
      created.push(series);
      return series;
    },
    priceScale() {
      return {
        options: () => ({ autoScale: auto }),
        applyOptions: (opts) => {
          if (Object.prototype.hasOwnProperty.call(opts, 'autoScale')) auto = !!opts.autoScale;
        },
        width: () => 64,
        _auto: () => auto,
      };
    },
  };
}

test('A. host contributes bounded [-5,105] via ScaleContribution translator', () => {
  const opts = WozduhExtremeBands._hostSeriesOptionsForTests();
  assert.deepStrictEqual(opts.autoscaleInfoProvider(), {
    priceRange: { minValue: -5, maxValue: 105 },
  });
  assert.deepStrictEqual(WozduhExtremeBands.PANE_DOMAIN, { type: 'bounded', min: -5, max: 105 });
  assert.strictEqual(opts.lineVisible, false);
  assert.strictEqual(opts.priceScaleId, 'right');
});

test('B. DDR layout: every Wozduh plot/channel is ignore; no scaleBoundedOsc', () => {
  const layout = fs.readFileSync(path.join(__dirname, '../ui_config/wozduh_layout.go'), 'utf8');
  assert.ok(!layout.includes('scaleBoundedOsc'));
  assert.ok(layout.includes('wozLine("woz_vol_rsi_ema5", core.SlotWozduhVolRsiEma5, scaleIgnore'));
  const rsx = fs.readFileSync(path.join(__dirname, '../ui_config/rsx_layout.go'), 'utf8');
  assert.ok(rsx.includes('"scaleContribution":{"type":"bounded","min":-5,"max":105}'));
});

test('C. factory: hiding ema5/close/channel does not remove host domain; Auto still commands autoScale', () => {
  WozduhExtremeBands._resetForTests();
  const chart = fakeWozChart();
  const captured = [];
  const seriesById = new Map();
  const factory = new DDRFactory();
  factory.buildPanes(
    {
      wozduh: {
        chart: {
          addLineSeries(opts) {
            const id = captured.length;
            const s = chart.addLineSeries(opts);
            captured.push({ opts, series: s, i: id });
            return s;
          },
          addCustomSeries() {
            return chart.addLineSeries({ autoscaleInfoProvider: () => null });
          },
        },
      },
    },
    {
      pane_osc: [
        {
          id: 'woz_vol_rsi_ema5',
          hostId: 'wozduh',
          kind: 'line',
          renderOptions: { defaultVisible: true, scaleContribution: { type: 'ignore' } },
        },
        {
          id: 'woz_vol_rsi_ema12',
          hostId: 'wozduh',
          kind: 'line',
          renderOptions: { defaultVisible: true, scaleContribution: { type: 'ignore' } },
        },
        {
          id: 'woz_rsi_close',
          hostId: 'wozduh',
          kind: 'line',
          renderOptions: { defaultVisible: true, scaleContribution: { type: 'ignore' } },
        },
      ],
    },
  );
  seriesById.set('woz_vol_rsi_ema5', captured[0].series);
  seriesById.set('woz_vol_rsi_ema12', captured[1].series);
  seriesById.set('woz_rsi_close', captured[2].series);

  assert.ok(WozduhExtremeBands.attach(chart));
  const host = chart.created[chart.created.length - 1];
  assert.deepStrictEqual(host.opts.autoscaleInfoProvider(), {
    priceRange: { minValue: -5, maxValue: 105 },
  });

  factory.setSeriesVisible('woz_vol_rsi_ema5', false);
  factory.setSeriesVisible('woz_vol_rsi_ema12', false);
  factory.setSeriesVisible('woz_rsi_close', false);
  assert.strictEqual(seriesById.get('woz_vol_rsi_ema5').visible, false);
  assert.strictEqual(host.removed, false);
  assert.deepStrictEqual(host.opts.autoscaleInfoProvider(), {
    priceRange: { minValue: -5, maxValue: 105 },
  });
  const ddrBounded = captured.filter((c) => {
    const p = c.opts.autoscaleInfoProvider;
    return typeof p === 'function' && p() && p().priceRange;
  });
  assert.strictEqual(ddrBounded.length, 0);

  const store = memoryStorage();
  ScaleController._resetForTests({ storage: store });
  ScaleController.init({ storage: store });
  ScaleController.register({
    context: 'live',
    hostId: 'wozduh',
    chart,
    allowLog: false,
  });
  ScaleController.setPanePrefs('wozduh', { isAuto: false });
  assert.strictEqual(chart.priceScale()._auto(), false);
  ScaleController.toggleAuto('live', 'wozduh');
  assert.strictEqual(chart.priceScale()._auto(), true);
  assert.strictEqual(seriesById.get('woz_vol_rsi_ema5').visible, false);
  WozduhExtremeBands.dispose();
  ScaleController._resetForTests({ storage: memoryStorage() });
});

test('D. no range crutch / no Wozduh ScaleController branch / no force-visible ema5', () => {
  const sc = fs.readFileSync(path.join(__dirname, 'ui/scale-controller.js'), 'utf8');
  const bands = fs.readFileSync(path.join(__dirname, 'wozduh-extreme-bands.js'), 'utf8');
  const layout = fs.readFileSync(path.join(__dirname, '../ui_config/wozduh_layout.go'), 'utf8');
  const factory = fs.readFileSync(path.join(__dirname, 'series-factory.js'), 'utf8');
  assert.ok(!sc.includes('setVisibleRange'));
  assert.ok(!bands.includes('setVisibleRange'));
  assert.ok(!sc.includes("hostId === 'wozduh'"));
  assert.ok(!sc.includes('hostId === "wozduh"'));
  assert.ok(!/woz_vol_rsi_ema5[\s\S]{0,120}visible:\s*true/.test(factory));
  assert.ok(!layout.includes('scaleBoundedOsc'));
  assert.ok(!/autoscaleInfoProvider:\s*\(\)\s*=>\s*null/.test(bands));
});

test('E. RSX layout still plot-owned bounded; host is not a Style/DDR id', () => {
  const rsx = fs.readFileSync(path.join(__dirname, '../ui_config/rsx_layout.go'), 'utf8');
  assert.ok(rsx.includes('line_rsx'));
  assert.ok(rsx.includes('"type":"bounded"'));
  const settings = fs.readFileSync(path.join(__dirname, 'ui/settings-renderer.js'), 'utf8');
  assert.ok(!settings.includes('PANE_DOMAIN'));
  assert.ok(!settings.includes('WozduhExtremeBands'));
});

console.log('wozduh_pane_autoscale_owner_test: ALL PASS');
