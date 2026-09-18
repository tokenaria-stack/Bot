/**
 * WOZDUH-NUMERIC-NAMES-1 — one-shot visibility pref remap.
 * Run: node web/wozduh_prefs_migrate_test.js
 */
'use strict';

const assert = require('assert');
const { SettingsRenderer } = require('./ui/settings-renderer.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

const components = [
  { id: 'woz_vol_rsi_ema12', renderOptions: { defaultVisible: true } },
  { id: 'woz_vol_rsi_ema5', renderOptions: { defaultVisible: true } },
  { id: 'woz_rsi_close', renderOptions: { defaultVisible: true } },
  { id: 'woz_rsi_close_ema7', renderOptions: { defaultVisible: false } },
  { id: 'woz_vol_rsi_ema5_chan', renderOptions: { defaultVisible: false } },
  { id: 'woz_rsi_close_chan', renderOptions: { defaultVisible: false } },
];

test('woz_fast / woz_slow remap once; old keys dropped', () => {
  const out = SettingsRenderer.migrateLegacyPrefs(
    { woz_fast: false, woz_slow: true },
    components,
  );
  assert.strictEqual(out.woz_vol_rsi_ema12, false);
  assert.strictEqual(out.woz_vol_rsi_ema5, true);
  assert.ok(!Object.prototype.hasOwnProperty.call(out, 'woz_fast'));
  assert.ok(!Object.prototype.hasOwnProperty.call(out, 'woz_slow'));
});

test('rsiVol fills both EMA lines only when new keys absent', () => {
  const fromFalcon = SettingsRenderer.migrateLegacyPrefs({ rsiVol: false }, components);
  assert.strictEqual(fromFalcon.woz_vol_rsi_ema12, false);
  assert.strictEqual(fromFalcon.woz_vol_rsi_ema5, false);

  const preferWoz = SettingsRenderer.migrateLegacyPrefs(
    { rsiVol: true, woz_fast: false, woz_slow: false },
    components,
  );
  assert.strictEqual(preferWoz.woz_vol_rsi_ema12, false);
  assert.strictEqual(preferWoz.woz_vol_rsi_ema5, false);
});

test('other retired Wozduh IDs remap', () => {
  const out = SettingsRenderer.migrateLegacyPrefs(
    { woz_rsi_price: false, woz_ema_rsi: true, woz_vol_chan: true, woz_price_chan: false },
    components,
  );
  assert.strictEqual(out.woz_rsi_close, false);
  assert.strictEqual(out.woz_rsi_close_ema7, true);
  assert.strictEqual(out.woz_vol_rsi_ema5_chan, true);
  assert.strictEqual(out.woz_rsi_close_chan, false);
});

console.log('wozduh_prefs_migrate_test: ALL PASS');
