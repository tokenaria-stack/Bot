const fs = require('fs');
const path = require('path');
const assert = require('assert');

const src = fs.readFileSync(path.join(__dirname, 'structural-stop-overlay.js'), 'utf8');
assert.ok(src.includes('/api/research/structural-stop-1'), 'overlay must fetch research assignment');
assert.ok(src.includes('seekHistoryIsland'), 'overlay must reuse HISTORY island, not a second research chart');
assert.ok(!src.includes('rsx_settings'), 'overlay must not read live RSX settings');
assert.ok(!src.includes('fractalFacts'), 'overlay must not recompute fractals');
assert.ok(src.includes('ConfirmedAt (time)'), 'ConfirmedAt is a time tick, not a price owner');
assert.ok(src.includes('price_swing'), 'overlay must paint backend P1/P2, not recompute');
assert.ok(src.includes('prominence_atr'), 'overlay paints backend prominence, does not recompute');
assert.ok(src.includes('STOP +0.15 ATR'), 'frozen buffer is 0.15');
assert.ok(src.includes('a.wick'), 'overlay paints backend S0 wick vs buffered stop');
assert.ok(!src.includes('+0.10 ATR'), 'do not keep buffer picker 0.10');
assert.ok(!src.includes('+0.20 ATR'), 'do not keep buffer picker 0.20');
console.log('structural-stop-overlay ownership ok');
