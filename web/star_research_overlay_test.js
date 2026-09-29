/**
 * STAR-RESEARCH-OVERLAY-1. Certified row painted as marks and rulers.
 * Run: node web/star_research_overlay_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

const Overlay = require('./ui/star-research-overlay.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

function line(plan, id) {
  return plan.lines.find((item) => item.id === id) || null;
}

const star2 = {
  status: 'valid',
  side: 'down',
  decisionAt: 1567997100000,
  entryAt: 1567998000000,
  entryPrice: 10295.81,
  swingAt: 1567985400000,
  swingWick: 10392.25,
  stopPrice: 10394.46831284707,
  atr: 14.788752313796204,
};

const star127 = {
  status: 'valid',
  side: 'up',
  decisionAt: 1571070600000,
  entryAt: 1571071500000,
  entryPrice: 8321.74,
  swingAt: 1571065200000,
  swingWick: 8254,
  stopPrice: 8249.779793596981,
  atr: 28.134709353461584,
};

const star1 = {
  status: 'no_structure',
  side: 'up',
  decisionAt: 1567965600000,
  entryAt: 1567966500000,
  entryPrice: 10000,
};

const star272 = {
  status: 'invalid_geometry',
  side: 'down',
  decisionAt: 1574474400000,
  entryAt: 1574475300000,
  entryPrice: 7232.41,
  swingAt: 1574471700000,
  swingWick: 7206.2,
  atr: 54.00886926723741,
};

function main() {
  const src = fs.readFileSync(path.join(__dirname, 'ui', 'star-research-overlay.js'), 'utf8');
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  const camera = fs.readFileSync(path.join(__dirname, 'ui', 'time-camera.js'), 'utf8');
  assert.ok(!src.includes('0.15'), 'overlay must not recompute the buffer');
  assert.ok(!src.includes('seekHistoryIsland'));
  assert.ok(!src.includes('proposeFromPane'));
  assert.ok(!src.includes('setVisibleLogicalRange'));
  assert.ok(!src.includes('setData'));
  assert.ok(!src.includes('TimeCamera'));
  assert.ok(!src.includes('paintCandles'));
  assert.ok(!camera.includes('StarResearch'), 'TimeCamera does not know about stars');
  assert.ok(!/function applyResearchMarkers[\s\S]{0,500}TimeCamera/.test(core));
  assert.ok(!/function attachResearchPrimitive[\s\S]{0,500}TimeCamera/.test(core));
  assert.ok(!/function applyResearchMarkers[\s\S]{0,500}seekHistoryIsland/.test(core));

  test('star, swing, and next-open entry use the supplied times', () => {
    const plan = Overlay.researchPlan(star2);
    const byText = Object.fromEntries(plan.markers.map((m) => [m.text.split(' ')[0], m]));
    assert.strictEqual(byText.Star.time, 1567997100);
    assert.strictEqual(byText.swing.time, 1567985400);
    assert.strictEqual(byText.swing.text, 'swing 10392.25');
    assert.strictEqual(byText.entry, undefined);
    assert.strictEqual(plan.entries[0].time, 1567998000);
    assert.strictEqual(plan.entries[0].price, 10295.81);
    assert.strictEqual(line(plan, 'entry').price, 10295.81);
    assert.strictEqual(line(plan, 'swing'), null);
  });

  test('entry mark follows side and the entry line uses that color', () => {
    const down = Overlay.researchPlan(star2);
    assert.strictEqual(down.entries[0].shape, 'triangleDown');
    assert.strictEqual(down.entries[0].color, '#FF1744');
    assert.strictEqual(line(down, 'entry').color, '#FF1744');
    assert.strictEqual(line(down, 'stop').color, '#ff5a6a');
    const up = Overlay.researchPlan(star127);
    assert.strictEqual(up.entries[0].shape, 'triangleUp');
    assert.strictEqual(up.entries[0].color, '#00E676');
    assert.strictEqual(line(up, 'entry').color, '#00E676');
    assert.ok(!down.markers.some((m) => m.shape === 'square'));
    assert.ok(!up.markers.some((m) => m.shape === 'square'));
  });

  test('down stop and rulers are the supplied prices', () => {
    const plan = Overlay.researchPlan(star2);
    const stop = line(plan, 'stop').price;
    assert.strictEqual(stop, star2.stopPrice);
    const distance = Math.abs(star2.entryPrice - star2.stopPrice);
    assert.strictEqual(line(plan, 'r1').price, star2.entryPrice - distance);
    assert.strictEqual(line(plan, 'r2').price, star2.entryPrice - 2 * distance);
    assert.strictEqual(line(plan, 'r3').price, star2.entryPrice - 3 * distance);
    assert.ok(line(plan, 'r1').price < star2.entryPrice);
    assert.ok(stop > star2.entryPrice);
  });

  test('up stop and rulers are the supplied prices', () => {
    const plan = Overlay.researchPlan(star127);
    const stop = line(plan, 'stop').price;
    assert.strictEqual(stop, star127.stopPrice);
    const distance = Math.abs(star127.entryPrice - star127.stopPrice);
    assert.strictEqual(line(plan, 'r1').price, star127.entryPrice + distance);
    assert.strictEqual(line(plan, 'r2').price, star127.entryPrice + 2 * distance);
    assert.strictEqual(line(plan, 'r3').price, star127.entryPrice + 3 * distance);
    assert.ok(stop < star127.entryPrice);
    assert.ok(stop < star127.swingWick);
  });

  test('the same row paints the same stop twice', () => {
    assert.deepStrictEqual(Overlay.researchPlan(star127), Overlay.researchPlan(star127));
    assert.strictEqual(Overlay.researchPlan.length, 1);
  });

  test('no structure paints the star and entry only', () => {
    const plan = Overlay.researchPlan(star1);
    assert.strictEqual(plan.status, 'no_structure');
    assert.strictEqual(plan.markers.length, 1);
    assert.strictEqual(plan.entries.length, 1);
    assert.deepStrictEqual(plan.lines.map((item) => item.id), ['entry']);
    assert.strictEqual(plan.markers[0].time, 1567965600);
  });

  test('invalid geometry paints no stop and no rulers', () => {
    const plan = Overlay.researchPlan(star272);
    assert.strictEqual(plan.status, 'invalid_geometry');
    assert.ok(plan.markers.some((m) => m.text === 'Star'));
    assert.ok(plan.markers.some((m) => m.text === 'swing 7206.2'));
    assert.ok(plan.lines.some((item) => item.id === 'entry'));
    assert.strictEqual(line(plan, 'swing'), null);
    assert.strictEqual(line(plan, 'stop'), null);
    assert.strictEqual(line(plan, 'r1'), null);
    assert.strictEqual(line(plan, 'r2'), null);
    assert.strictEqual(line(plan, 'r3'), null);
  });

  test('another timeframe paints nothing and does not move the camera', () => {
    let cameras = 0;
    let markers = null;
    Overlay.bind({
      getRow: () => star127,
      getTf: () => '1h',
      applyMarkers: (next) => { markers = next; return true; },
      attach: () => true,
      requestDraw: () => {},
      paintStatus: () => {},
      seekHistoryIsland: () => { cameras += 1; },
    });
    const plan = Overlay.refresh();
    assert.strictEqual(plan.markers.length, 0);
    assert.strictEqual(plan.lines.length, 0);
    assert.deepStrictEqual(markers, []);
    assert.strictEqual(cameras, 0);
  });

  test('painting on 15m does not move the camera', () => {
    let cameras = 0;
    let drawn = 0;
    Overlay.bind({
      getRow: () => star2,
      getTf: () => '15m',
      applyMarkers: () => true,
      attach: () => true,
      requestDraw: () => { drawn += 1; },
      paintStatus: (text) => { assert.strictEqual(text, 'valid'); },
      proposeFromPane: () => { cameras += 1; },
    });
    const plan = Overlay.refresh();
    assert.ok(plan.markers.some((m) => m.text === 'Star'));
    assert.strictEqual(line(plan, 'stop').price, star2.stopPrice);
    assert.strictEqual(drawn, 1);
    assert.strictEqual(cameras, 0);
    assert.strictEqual(Overlay.primitive.lines.length, plan.lines.length);
  });

  test('population markers sit on the decision candle, not a fixed Y', () => {
    const marks = Overlay.populationPlan([
      { decisionAt: 1567997100000, side: 'down' },
      { decisionAt: 1571070600000, side: 'up' },
    ]);
    assert.strictEqual(marks[0].time, 1567997100);
    assert.strictEqual(marks[0].position, 'aboveBar');
    assert.strictEqual(marks[0].shape, 'arrowDown');
    assert.strictEqual(marks[0].color, Overlay.STAR_DOWN);
    assert.strictEqual(marks[1].position, 'belowBar');
    assert.strictEqual(marks[1].shape, 'arrowUp');
    assert.strictEqual(marks[1].color, Overlay.STAR_UP);
    assert.ok(marks.every((m) => m.y == null && m.text == null));
    assert.ok(!src.includes('height * 0.12'));
    assert.ok(!src.includes('crowd.length'));
  });

  console.log('star research overlay tests passed');
}

main();
