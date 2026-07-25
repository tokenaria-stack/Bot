/**
 * ADR-027 DisplayTimeline unit tests (Node).
 * Run: node web/display_timeline_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');
const DisplayTimeline = require('./ui/display-timeline.js');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

const getIntervalMs = (tf) => {
  const m = /^(\d+)([smhdwM])$/.exec(String(tf));
  if (!m) return 60_000;
  const n = Number(m[1]);
  const u = m[2];
  if (u === 's') return n * 1000;
  if (u === 'm') return n * 60_000;
  if (u === 'h') return n * 3_600_000;
  if (u === 'd') return n * 86_400_000;
  if (u === 'w') return n * 7 * 86_400_000;
  if (u === 'M') return 30 * 86_400_000; // unused for 1M calendar path
  return 60_000;
};

test('fixed 1m next opens step by 60s', () => {
  const last = 1_700_000_040; // already on a 1m open
  const times = DisplayTimeline.buildFutureTimes({
    lastTimeSec: last,
    count: 3,
    tf: '1m',
    getIntervalMs,
  });
  assert.deepStrictEqual(times, [last + 60, last + 120, last + 180]);
});

test('countFutureBars uses visibleTo and rightOffset; caps max', () => {
  assert.strictEqual(
    DisplayTimeline.countFutureBars({ lastLogical: 100, visibleTo: 110, minBuffer: 0 }),
    10,
  );
  assert.strictEqual(
    DisplayTimeline.countFutureBars({ lastLogical: 100, rightOffset: 25, minBuffer: 0 }),
    25,
  );
  assert.strictEqual(
    DisplayTimeline.countFutureBars({
      lastLogical: 0,
      visibleTo: 10_000,
      minBuffer: 0,
      maxBars: 50,
    }),
    50,
  );
});

test('buildWhitespaceBars are time-only (no OHLC)', () => {
  const bars = DisplayTimeline.buildWhitespaceBars({
    lastTimeSec: 1_000,
    lastLogical: 5,
    visibleTo: 8,
    tf: '1m',
    getIntervalMs,
    minBuffer: 0,
  });
  assert.strictEqual(bars.length, 3);
  bars.forEach((b) => {
    assert.deepStrictEqual(Object.keys(b).sort(), ['time']);
    assert.strictEqual(typeof b.time, 'number');
  });
});

test('1w next is +7 UTC days from Monday open', () => {
  // 2024-01-01 is Monday
  const monday = Date.UTC(2024, 0, 1) / 1000;
  const times = DisplayTimeline.buildFutureTimes({
    lastTimeSec: monday,
    count: 1,
    tf: '1w',
    getIntervalMs,
  });
  assert.strictEqual(times[0], monday + 7 * 86400);
});

test('1M next is first of next month UTC', () => {
  const feb = Date.UTC(2024, 1, 1) / 1000;
  const times = DisplayTimeline.buildFutureTimes({
    lastTimeSec: feb,
    count: 1,
    tf: '1M',
    getIntervalMs,
  });
  assert.strictEqual(times[0], Date.UTC(2024, 2, 1) / 1000);
});

test('DisplayTimeline is pure; Phase 0 candle pipeline has no whitespace merge', () => {
  const helper = fs.readFileSync(path.join(__dirname, 'ui/display-timeline.js'), 'utf8');
  assert.ok(!/\bliveColumnarStore\b/.test(helper));
  assert.ok(!/\.setData\s*\(/.test(helper));
  assert.ok(!/\.update\s*\(/.test(helper));
  const core = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');
  // Phase 0: DisplayTimeline module remains, but must not feed candleSeries.
  assert.ok(!core.includes('applyCandlesWithWhitespace'));
  assert.ok(!core.includes('mergeCandlesWithWhitespace'));
  assert.ok(!core.includes('buildWhitespaceBars'));
  assert.ok(!/setData\([^)]*concat|real\.concat/.test(core));
});

console.log('display_timeline_test: ALL PASS');
