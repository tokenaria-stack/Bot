/**
 * HISTORY → HISTORY TF switch (Microscope) — center hydrate + no FreshLive.
 * Run: node web/viewport_history_tf_switch_test.js
 */
'use strict';

const assert = require('assert');
const TimeCamera = require('./ui/time-camera.js');
const ViewportManager = require('./ui/viewport-manager.js');
const { ChartCompositor } = require('./chart-compositor.js');

globalThis.TimeCamera = TimeCamera;

function test(name, fn) {
  fn();
  console.log('OK', name);
}

/** 2026-05-30 12:00:00 UTC */
const CENTER_MS = Date.UTC(2026, 4, 30, 12, 0, 0);
/** 2026-08-10 — "today" tip (outside May window) */
const NOW_SEC = Math.floor(Date.UTC(2026, 7, 10, 12, 0, 0) / 1000);

test('resolveHistoryTfFetchEndSec: normal 1m chunk centered on HISTORY focus', () => {
  const limit = 3000;
  const intervalSec = 60;
  const end = ViewportManager.resolveHistoryTfFetchEndSec({
    intent: 'HISTORY',
    centerTimeMs: CENTER_MS,
    nowSec: NOW_SEC,
    limit,
    intervalSec,
  });
  const centerSec = Math.floor(CENTER_MS / 1000);
  assert.strictEqual(end, centerSec + Math.floor(limit / 2) * intervalSec);
  // Tip (Aug) must NOT be the fetch end — center must sit inside the window.
  assert.ok(end < NOW_SEC - 86400, 'fetch end must be near May, not live tip');
  const firstSec = end - limit * intervalSec;
  assert.ok(centerSec > firstSec && centerSec <= end, 'center inside hydrated window');
});

test('resolveHistoryTfFetchEndSec: LIVE / missing / invalid center → nowSec', () => {
  assert.strictEqual(ViewportManager.resolveHistoryTfFetchEndSec({
    intent: 'LIVE',
    centerTimeMs: CENTER_MS,
    nowSec: NOW_SEC,
    limit: 3000,
    intervalSec: 60,
  }), NOW_SEC);
  assert.strictEqual(ViewportManager.resolveHistoryTfFetchEndSec({
    intent: 'HISTORY',
    centerTimeMs: null,
    nowSec: NOW_SEC,
    limit: 3000,
    intervalSec: 60,
  }), NOW_SEC);
  assert.strictEqual(ViewportManager.resolveHistoryTfFetchEndSec({
    intent: 'HISTORY',
    centerTimeMs: NaN,
    nowSec: NOW_SEC,
    limit: 3000,
    intervalSec: 60,
  }), NOW_SEC);
  assert.strictEqual(ViewportManager.resolveHistoryTfFetchEndSec({
    intent: 'HISTORY',
    centerTimeMs: 0,
    nowSec: NOW_SEC,
    limit: 3000,
    intervalSec: 60,
  }), NOW_SEC);
  assert.strictEqual(ViewportManager.resolveHistoryTfFetchEndSec({
    intent: 'HISTORY',
    centerTimeMs: -1,
    nowSec: NOW_SEC,
    limit: 3000,
    intervalSec: 60,
  }), NOW_SEC);
});

test('cameraIntentForTfSwitch HISTORY: healthy bars, sacred center', () => {
  const seed = ViewportManager.cameraIntentForTfSwitch({
    centerTimeMs: CENTER_MS,
    visibleBars: 2992,
    intent: 'HISTORY',
    isAtRightEdge: false,
    rightOffset: 0,
    barSpacing: 2,
  });
  assert.strictEqual(seed.intent, 'HISTORY');
  assert.strictEqual(seed.centerTimeMs, CENTER_MS);
  assert.strictEqual(seed.visibleBars, TimeCamera.HEALTHY_VISIBLE_BARS);
  assert.notStrictEqual(seed.visibleBars, 2992);
});

test('cameraIntentForTfSwitch LIVE: keeps visibleBars and spacing', () => {
  const seed = ViewportManager.cameraIntentForTfSwitch({
    centerTimeMs: CENTER_MS,
    visibleBars: 220,
    intent: 'LIVE',
    isAtRightEdge: true,
    rightOffset: 4,
    rightPadding: 4,
    barSpacing: 8,
  });
  assert.strictEqual(seed.intent, 'LIVE');
  assert.strictEqual(seed.visibleBars, 220);
  assert.strictEqual(seed.barSpacing, 8);
  assert.strictEqual(seed.rightPadding, 4);
});

test('isPoisonCameraState: from < 0 is not poison', () => {
  assert.strictEqual(ViewportManager.isPoisonCameraState({
    from: -12,
    barSpacing: 8,
    visibleBars: 100,
  }), false);
  assert.strictEqual(ViewportManager.isPoisonCameraState({
    from: 0,
    barSpacing: 0.5,
    visibleBars: 100,
  }), true);
});

test('TF-2A LIVE switch fetch: 5000 VIEW → limit 5000', () => {
  assert.strictEqual(ViewportManager.resolveLiveTfSwitchFetchLimit(5000), 5000);
});

test('TF-2A LIVE switch fetch: 2000 VIEW → chunk 3000', () => {
  assert.strictEqual(ViewportManager.resolveLiveTfSwitchFetchLimit(2000), 3000);
});

test('TF-2A LIVE switch fetch: above store cap → MAX_STORE_BARS', () => {
  assert.strictEqual(ViewportManager.resolveLiveTfSwitchFetchLimit(12000), 9000);
});

test('TF-2A invalid visibleBars → default chunk 3000', () => {
  assert.strictEqual(ViewportManager.resolveLiveTfSwitchFetchLimit(NaN), 3000);
  assert.strictEqual(ViewportManager.resolveLiveTfSwitchFetchLimit(0), 3000);
});

test('TF-2A ordinary history fetchColumnar still uses HISTORY_CHUNK_LIMIT', () => {
  const fs = require('fs');
  const path = require('path');
  const boot = fs.readFileSync(path.join(__dirname, 'boot.js'), 'utf8');
  assert.ok(
    /fetchColumnar:[\s\S]*limit: typeof HISTORY_CHUNK_LIMIT/.test(boot),
    'scroll/prefetch fetchColumnar must keep HISTORY_CHUNK_LIMIT',
  );
  assert.ok(
    /userTfChange === true/.test(boot) && /resolveLiveTfSwitchFetchLimit/.test(boot),
    'LIVE TF first fetch must be gated on userTfChange',
  );
});

test('proposeAfterData HISTORY restores center inside hydrated 1m chunk', () => {
  TimeCamera._resetForTests();
  let seen = null;
  TimeCamera.bind({ applyCommitted: (s) => { seen = s; } });

  const limit = 3000;
  const intervalSec = 60;
  const centerSec = Math.floor(CENTER_MS / 1000);
  const end = centerSec + Math.floor(limit / 2) * intervalSec;
  const first = end - limit * intervalSec;
  const times = Array.from({ length: limit }, (_, i) => first + i * intervalSec);

  TimeCamera.bindDataResolve({
    nearestLogicalForTime: (ms) => ChartCompositor.findIndexByTimeMs(times, ms),
  });

  const ok = TimeCamera.proposeAfterData({
    tipLogical: times.length - 1,
    timesSec: times,
    seed: {
      intent: 'HISTORY',
      _liveEdge: false,
      centerTime: CENTER_MS,
      visibleBars: TimeCamera.HEALTHY_VISIBLE_BARS,
      barSpacing: TimeCamera.HEALTHY_BAR_SPACING,
    },
    mode: 'switch',
  });
  assert.strictEqual(ok, true);
  assert.ok(seen);
  const mid = (seen.visibleRange.from + seen.visibleRange.to) / 2;
  const midSec = times[Math.floor(mid)];
  assert.ok(Math.abs(midSec - centerSec) <= intervalSec, `center market-time drift: ${midSec} vs ${centerSec}`);
  const width = seen.visibleRange.to - seen.visibleRange.from;
  assert.ok(Math.abs(width - TimeCamera.HEALTHY_VISIBLE_BARS) < 1e-6);
});

test('HISTORY restore: tip-only store must NOT FreshLive (center missing)', () => {
  TimeCamera._resetForTests();
  let freshCalls = 0;
  const origFresh = TimeCamera.proposeFreshLive.bind(TimeCamera);
  TimeCamera.proposeFreshLive = (...args) => {
    freshCalls += 1;
    return origFresh(...args);
  };

  const tipStart = Math.floor(Date.UTC(2026, 7, 1) / 1000);
  const tipTimes = Array.from({ length: 3000 }, (_, i) => tipStart + i * 60);

  const origHL = ViewportManager.hostHasLayout;
  ViewportManager.hostHasLayout = () => true;

  const cc = Object.create(ChartCompositor.prototype);
  cc._observeShadowWorld = () => {};
  cc._bindDataResolve = () => {};
  cc._navigateAfterPaint({
    viewport: 'restore',
    anchor: {
      intent: 'HISTORY',
      centerTimeMs: CENTER_MS,
      visibleBars: 150,
      isAtRightEdge: false,
    },
  }, { times: tipTimes });

  ViewportManager.hostHasLayout = origHL;
  TimeCamera.proposeFreshLive = origFresh;

  assert.strictEqual(freshCalls, 0, 'FreshLive must not steal HISTORY TF restore');
});

test('fresh viewport still allows FreshLive (LIVE init)', () => {
  TimeCamera._resetForTests();
  let freshCalls = 0;
  const origFresh = TimeCamera.proposeFreshLive.bind(TimeCamera);
  TimeCamera.proposeFreshLive = (...args) => {
    freshCalls += 1;
    return origFresh(...args);
  };
  TimeCamera.bind({ applyCommitted: () => {} });

  const tipStart = Math.floor(Date.UTC(2026, 7, 1) / 1000);
  const tipTimes = Array.from({ length: 100 }, (_, i) => tipStart + i * 60);
  const origHL = ViewportManager.hostHasLayout;
  ViewportManager.hostHasLayout = () => true;

  const cc = Object.create(ChartCompositor.prototype);
  cc._observeShadowWorld = () => {};
  cc._bindDataResolve = () => {};
  cc._navigateAfterPaint({ viewport: 'fresh', anchor: null }, { times: tipTimes });

  ViewportManager.hostHasLayout = origHL;
  TimeCamera.proposeFreshLive = origFresh;

  assert.ok(freshCalls >= 1, 'legitimate LIVE fresh path must still call FreshLive');
});

test('LIVE restore: layout defer must NOT FreshLive', () => {
  TimeCamera._resetForTests();
  let freshCalls = 0;
  const origFresh = TimeCamera.proposeFreshLive.bind(TimeCamera);
  TimeCamera.proposeFreshLive = (...args) => {
    freshCalls += 1;
    return origFresh(...args);
  };
  TimeCamera.bind({ applyCommitted: () => {} });

  const tipStart = Math.floor(Date.UTC(2026, 7, 1) / 1000);
  const tipTimes = Array.from({ length: 400 }, (_, i) => tipStart + i * 60);
  const origHL = ViewportManager.hostHasLayout;
  const origWhen = ViewportManager.whenHostHasLayout;
  ViewportManager.hostHasLayout = () => false;
  ViewportManager.whenHostHasLayout = () => {};

  const cc = Object.create(ChartCompositor.prototype);
  cc._observeShadowWorld = () => {};
  cc._bindDataResolve = () => {};
  cc._navigateAfterPaint({
    viewport: 'restore',
    anchor: {
      intent: 'LIVE',
      isAtRightEdge: true,
      visibleBars: 220,
      barSpacing: 8,
      rightPadding: 4,
      centerTimeMs: CENTER_MS,
    },
  }, { times: tipTimes });

  ViewportManager.hostHasLayout = origHL;
  ViewportManager.whenHostHasLayout = origWhen;
  TimeCamera.proposeFreshLive = origFresh;

  assert.strictEqual(freshCalls, 0, 'FreshLive must not steal LIVE TF restore');
});

test('F5d resolveHistoryTfFetchEndSec: 1993 ms is milliseconds not seconds', () => {
  const end = ViewportManager.resolveHistoryTfFetchEndSec({
    intent: 'HISTORY',
    centerTimeMs: 730_944_000_000,
    nowSec: NOW_SEC,
    limit: 3000,
    intervalSec: 60,
  });
  // 730944000 + floor(3000/2)*60 = 731034000 (below NOW_SEC, so not clamped)
  assert.strictEqual(end, 730_944_000 + Math.floor(3000 / 2) * 60);
  assert.strictEqual(end, 731_034_000);
});

test('F5d resolveHistoryTfFetchEndSec: fractional ms floors via msToChartSec', () => {
  const end = ViewportManager.resolveHistoryTfFetchEndSec({
    intent: 'HISTORY',
    centerTimeMs: 1_502_928_000_500,
    nowSec: NOW_SEC,
    limit: 3000,
    intervalSec: 60,
  });
  assert.strictEqual(end, 1_502_928_000 + Math.floor(3000 / 2) * 60);
});

test('F5d resolveHistoryTfFetchEndSec: no 1e12 magnitude branch', () => {
  const fs = require('fs');
  const src = fs.readFileSync(require.resolve('./ui/viewport-manager.js'), 'utf8');
  const body = src.match(/function resolveHistoryTfFetchEndSec\(opts\) \{[\s\S]*?\n  \}/)[0];
  assert.ok(!body.includes('1e12'), body);
  assert.ok(!body.includes('centerMs / 1000'), body);
  assert.ok(body.includes('msToChartSec'), body);
});

console.log('All HISTORY TF switch tests passed.');
