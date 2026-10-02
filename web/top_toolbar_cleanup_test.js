'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

function read(rel) {
  return fs.readFileSync(path.join(__dirname, rel), 'utf8');
}

const html = read('index.html');
const toolbar = read('ui/toolbar-controller.js');
const tf = read('ui/timeframe-controller.js');
const header = html.split('<header')[1].split('</header>')[0];

assert.strictEqual((header.match(/class="toolbar"/g) || []).length, 1);
assert.ok(!header.includes('command-bar'));
assert.ok(!html.includes('id="ui-threshold-long"'));
assert.ok(!html.includes('id="ui-threshold-short"'));
assert.ok(!html.includes('id="matrix-open-btn"'));
assert.ok(!html.includes('id="btn-risk-menu"'));
assert.ok(!html.includes('id="risk-settings-menu"'));
assert.ok(!html.includes('id="regime"'));
assert.ok(!html.includes('id="timeframe-label"'));
assert.ok(!html.includes('score-block'));
assert.ok(html.includes('data-tab="tab-live"'));
assert.ok(html.includes('id="symbol"'));
assert.ok(html.includes('id="tf-bar"'));
assert.ok(html.includes('id="tf-current-btn"'));
assert.ok(html.includes('id="ruler-btn"'));
assert.ok(html.includes('id="price-style-bar"'));
assert.ok(html.includes('id="btn-ind-menu"'));
assert.ok(html.includes('id="ind-menu"'));
assert.ok(html.includes('id="sandbox-badge"'));
assert.ok(header.indexOf('tab-live') < header.indexOf('id="symbol"'));
assert.ok(header.indexOf('id="symbol"') < header.indexOf('id="tf-bar"'));
assert.ok(header.indexOf('id="tf-bar"') < header.indexOf('id="ruler-btn"'));
assert.ok(header.indexOf('id="ruler-btn"') < header.indexOf('id="price-style-bar"'));
assert.ok(header.indexOf('id="price-style-bar"') < header.indexOf('id="btn-ind-menu"'));
assert.ok(!toolbar.includes('volatilityRegime'));
assert.ok(!toolbar.includes('timeframe-label'));
assert.ok(!tf.includes('timeframe-label'));
assert.ok(html.includes('flex-wrap: nowrap'));

console.log('top_toolbar_cleanup_test: ALL PASS');
