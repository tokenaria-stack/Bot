/**
 * Run: node web/star_inspection_swatch_test.js
 */
const assert = require('assert');
const S = require('./ui/star-inspection-swatches.js');
const WozduhColorPrefs = require('./wozduh-color-prefs.js');
const { DDRFactory } = require('./series-factory.js');

function test(name, fn) {
  fn();
  console.log('ok', name);
}

function ids(readingId) {
  return S.specs(readingId).map((spec) => spec.seriesId + ':' + spec.field);
}

test('Schema 3 Field IDs use the same series as inspection ids', () => {
  assert.deepStrictEqual(ids('M15.Vwema'), ids('m15.vwema'));
  assert.deepStrictEqual(ids('M15.ChanMid'), ['woz_vol_rsi_ema5_chan:midColor']);
  assert.deepStrictEqual(ids('M15.Ema5'), ids('m15.ema5'));
  assert.deepStrictEqual(ids('H1RSX.Value'), ['line_rsx:color']);
  assert.deepStrictEqual(ids('H1RSX.Slope'), ['line_rsx:color']);
  assert.deepStrictEqual(ids('M15.Slope'), ['woz_rsi_hl2_vwema:color']);
});

test('a line and its slope share one series', () => {
  assert.deepStrictEqual(ids('m15.vwema'), ['woz_rsi_hl2_vwema:color']);
  assert.deepStrictEqual(ids('h4.vwemaSlope'), ['woz_rsi_hl2_vwema:color']);
  assert.deepStrictEqual(ids('d1.orangeAccel'), ['woz_vol_rsi_ema5_chan:midColor']);
});

test('a relation names both lines in subtraction order', () => {
  assert.deepStrictEqual(ids('rel.m15h1.vwema'), [
    'woz_rsi_hl2_vwema:color',
    'woz_rsi_hl2_vwema:color',
  ]);
  assert.deepStrictEqual(ids('rel.m15.slope'), [
    'woz_rsi_hl2_vwema:color',
    'woz_vol_rsi_ema5_chan:midColor',
  ]);
  assert.deepStrictEqual(ids('rel.h1.rsx'), ['line_rsx:color', 'line_rsx_signal:color']);
  assert.deepStrictEqual(ids('m15.rsxMinus'), ['line_rsx:color', 'line_rsx_signal:color']);
});

test('the factory swatch follows the paint override', () => {
  const memory = {
    getItem() { return null; },
    setItem() {},
    removeItem() {},
  };
  const saved = {};
  memory.getItem = (key) => (Object.prototype.hasOwnProperty.call(saved, key) ? saved[key] : null);
  memory.setItem = (key, value) => { saved[key] = String(value); };
  memory.removeItem = (key) => { delete saved[key]; };
  WozduhColorPrefs.setStorage(memory);
  const factory = new DDRFactory();
  factory.manifest = {
    panes: {
      wozduh: [{ id: 'woz_rsi_hl2_vwema', kind: 'line', renderOptions: { color: '#112233' } }],
    },
  };
  assert.strictEqual(factory.swatchHex('woz_rsi_hl2_vwema', 'color'), '#112233');
  WozduhColorPrefs.setColor('woz_rsi_hl2_vwema', 'color', '#ABCDEF');
  assert.strictEqual(factory.swatchHex('woz_rsi_hl2_vwema', 'color'), '#abcdef');
  assert.strictEqual(factory.swatchHex('missing', 'color'), null);
  WozduhColorPrefs.setStorage(null);
});

console.log('star_inspection_swatch_test: ALL PASS');
