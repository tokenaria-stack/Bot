'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

function read(rel) {
  return fs.readFileSync(path.join(__dirname, rel), 'utf8');
}

const html = read('index.html');
const toolbar = read('ui/toolbar-controller.js');
const mappers = read('mappers.js');
const boot = read('boot.js');
const theme = read('chart-theme.js');
const store = read('store.js');
const columnar = read('columnar-store.js');

assert.ok(!html.includes('indicator-bar'));
assert.ok(!html.includes('tog-jurik'));
assert.ok(!html.includes('tog-spike'));
assert.ok(!html.includes('tog-volume'));
assert.ok(!html.includes('tog-fib'));
assert.ok(!toolbar.includes('isSpikeEnabled'));
assert.ok(!toolbar.includes('isFibEnabled'));
assert.ok(!toolbar.includes('tog-spike'));
assert.ok(!mappers.includes('buildSpikeMarkers'));
assert.ok(!mappers.includes('volumeSpikeUp'));
assert.ok(!boot.includes('lastFibZones'));
assert.ok(!boot.includes('spikeMarkers'));
assert.ok(!boot.includes('renderFib'));
assert.ok(!boot.includes('setToggleSeriesVisible'));
assert.ok(!theme.includes('spikeUp:'));
assert.ok(!store.includes('volumeSpikeUp'));
assert.ok(!columnar.includes('volumeSpikeUp'));

const goMarket = fs.readFileSync(path.join(__dirname, '..', 'market', 'streaming.go'), 'utf8');
assert.ok(goMarket.includes('DetectWozduxVolumeSpikeUp'));

console.log('legacy_chart_toggle_cleanup_test: ALL PASS');
