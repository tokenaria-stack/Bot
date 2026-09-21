/**
 * Wozduh family eye — batch mute/restore on existing visibility prefs.
 * Run: node web/wozduh_family_eye_test.js
 */
'use strict';

const assert = require('assert');
const { SettingsRenderer } = require('./ui/settings-renderer.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function memStorage() {
  const map = Object.create(null);
  return {
    getItem(k) { return Object.prototype.hasOwnProperty.call(map, k) ? map[k] : null; },
    setItem(k, v) { map[k] = String(v); },
    removeItem(k) { delete map[k]; },
  };
}

const components = [
  { id: 'woz_vol_rsi_ema5_chan', kind: 'channel', renderOptions: { defaultVisible: false, title: 'Volume RSI EMA5 channel' } },
  { id: 'woz_vol_rsi_ema5', kind: 'line', renderOptions: { defaultVisible: true, title: 'Volume RSI EMA5' } },
  { id: 'woz_vol_rsi_ema12', kind: 'line', renderOptions: { defaultVisible: true, title: 'Volume RSI EMA12' } },
  { id: 'woz_rsi_hl2_vwema', kind: 'line', renderOptions: { defaultVisible: false, title: 'RSI VWEMA(HL2)' } },
  { id: 'woz_rsi_close', kind: 'line', renderOptions: { defaultVisible: true, title: 'RSI close' } },
];

test('A. mute snapshots mix and hides every family member', () => {
  global.localStorage = memStorage();
  const prefs = {
    woz_vol_rsi_ema5_chan: true,
    woz_vol_rsi_ema5: true,
    woz_vol_rsi_ema12: false,
    woz_rsi_hl2_vwema: true,
    woz_rsi_close: true,
  };
  const next = SettingsRenderer.toggleFamilyEye('woz_vol_rsi_ema5_chan', components, prefs);
  assert.strictEqual(next.woz_vol_rsi_ema5_chan, false);
  assert.strictEqual(next.woz_vol_rsi_ema5, false);
  assert.strictEqual(next.woz_vol_rsi_ema12, false);
  assert.strictEqual(next.woz_rsi_hl2_vwema, false);
  assert.strictEqual(next.woz_rsi_close, true);
  assert.strictEqual(SettingsRenderer.familyHasVisibleMember('woz_vol_rsi_ema5_chan', components, next), false);
  const snap = JSON.parse(global.localStorage.getItem(SettingsRenderer.MUTE_SNAP_KEY));
  assert.deepStrictEqual(snap.woz_vol_rsi_ema5_chan, {
    woz_vol_rsi_ema5_chan: true,
    woz_vol_rsi_ema5: true,
    woz_vol_rsi_ema12: false,
    woz_rsi_hl2_vwema: true,
  });
});

test('B. unmute restores the snapshot, not factory', () => {
  const prefs = {
    woz_vol_rsi_ema5_chan: false,
    woz_vol_rsi_ema5: false,
    woz_vol_rsi_ema12: false,
    woz_rsi_hl2_vwema: false,
    woz_rsi_close: true,
  };
  const next = SettingsRenderer.toggleFamilyEye('woz_vol_rsi_ema5_chan', components, prefs);
  assert.strictEqual(next.woz_vol_rsi_ema5_chan, true);
  assert.strictEqual(next.woz_vol_rsi_ema5, true);
  assert.strictEqual(next.woz_vol_rsi_ema12, false);
  assert.strictEqual(next.woz_rsi_hl2_vwema, true);
  assert.strictEqual(next.woz_rsi_close, true);
  assert.ok(SettingsRenderer.familyHasVisibleMember('woz_vol_rsi_ema5_chan', components, next));
  assert.strictEqual(global.localStorage.getItem(SettingsRenderer.MUTE_SNAP_KEY), null);
});

test('C. no snapshot falls back to factory defaultVisible for that family only', () => {
  global.localStorage = memStorage();
  const prefs = {
    woz_vol_rsi_ema5_chan: false,
    woz_vol_rsi_ema5: false,
    woz_vol_rsi_ema12: false,
    woz_rsi_hl2_vwema: false,
    woz_rsi_close: false,
  };
  const next = SettingsRenderer.toggleFamilyEye('woz_vol_rsi_ema5_chan', components, prefs);
  assert.strictEqual(next.woz_vol_rsi_ema5_chan, false);
  assert.strictEqual(next.woz_vol_rsi_ema5, true);
  assert.strictEqual(next.woz_vol_rsi_ema12, true);
  assert.strictEqual(next.woz_rsi_hl2_vwema, false);
  assert.strictEqual(next.woz_rsi_close, false);
});

test('D. eye is derived; families do not include crossover ids', () => {
  const members = SettingsRenderer.FAMILY_MEMBERS;
  assert.deepStrictEqual(members.woz_rsi_close_chan, [
    'woz_rsi_close_chan',
    'woz_rsi_close',
    'woz_rsi_close_ema7',
    'woz_rsi_ad',
    'woz_rsi_hl2',
    'woz_rsi_rsi_close',
    'woz_macd_rsi_close',
  ]);
  assert.deepStrictEqual(members.woz_vol_rsi_ema5_chan, [
    'woz_vol_rsi_ema5_chan',
    'woz_vol_rsi_ema5',
    'woz_vol_rsi_ema12',
    'woz_rsi_hl2_vwema',
  ]);
  assert.ok(!Object.values(members).flat().some((id) => String(id).includes('_x_')));
  const mixed = {
    woz_vol_rsi_ema5_chan: false,
    woz_vol_rsi_ema5: true,
    woz_vol_rsi_ema12: false,
    woz_rsi_hl2_vwema: false,
  };
  assert.strictEqual(SettingsRenderer.familyHasVisibleMember('woz_vol_rsi_ema5_chan', components, mixed), true);
});

console.log('wozduh_family_eye_test.js passed');
