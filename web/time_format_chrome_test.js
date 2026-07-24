/**
 * Debt #91 — datetime chrome source contracts (Node).
 * Run: node web/time_format_chrome_test.js
 */
'use strict';

const assert = require('assert');
const fs = require('fs');
const path = require('path');

function test(name, fn) {
  fn();
  console.log('OK', name);
}

const src = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');

test('detailed crosshair timeFormatter (day+month+year+hour+minute, no seconds-only)', () => {
  assert.ok(src.includes('formatCrosshairTime') || src.includes('dtfCrosshair'), 'detailed crosshair formatter');
  assert.ok(src.includes("hour: '2-digit'"), 'hour in formatters');
  assert.ok(src.includes("minute: '2-digit'"), 'minute in formatters');
  // Crosshair bundle must not be the sole HH:mm:ss timeFormatter assignment.
  assert.ok(/timeFormatter:\s*formatCrosshairTime/.test(src), 'timeFormatter is detailed crosshair');
});

test('tickMarkFormatter switches on TickMarkType (not currentTf)', () => {
  assert.ok(src.includes('tickMarkTypes'), 'TickMarkType helper');
  assert.ok(src.includes('T.Year') && src.includes('T.Month') && src.includes('T.DayOfMonth'), 'date ticks');
  assert.ok(src.includes('T.Time') && src.includes('T.TimeWithSeconds'), 'intraday ticks');
  assert.ok(!/currentTf/.test(src.split('chartTimeFormatBundle')[1]?.slice(0, 2500) || ''),
    'bundle must not branch on currentTf');
});

test('vertLineChrome shared by crosshairOptions and applyHorzVisibility', () => {
  assert.ok(src.includes('function vertLineChrome'), 'vertLineChrome helper');
  assert.ok(src.includes('labelBackgroundColor'), 'label background');
  assert.ok(src.includes('labelTextColor'), 'label text');
  const applyBody = src.split('function applyHorzVisibility')[1]?.slice(0, 900) || '';
  assert.ok(applyBody.includes('vertLineChrome()'), 'applyHorzVisibility re-asserts vert chrome');
  assert.ok(src.includes('vertLine: { ...(base.vertLine || {}), ...vertLineChrome() }')
    || /vertLine:\s*\{\s*\.\.\.\(base\.vertLine[^}]*\),\s*\.\.\.vertLineChrome\(\)/.test(src),
    'crosshairOptions uses vertLineChrome');
});

test('ADR-023 ownership untouched (no owner ifs in formatters)', () => {
  const bundle = src.split('function chartTimeFormatBundle')[1]?.slice(0, 2800) || '';
  assert.ok(!/isOwner|bottomTimeAxis|getBottomTimeAxis/.test(bundle),
    'formatters are global; visibility gates display');
});

console.log('time_format_chrome_test: ALL PASS');
