/**
 * WOZDUH-TIME-INDEX-PIN-1 — after oscillator setData, force TimeCamera VIEW on all panes.
 * Run: node web/wozduh_time_index_pin_test.js
 */
const fs = require('fs');
const path = require('path');
const { ChartCompositor } = require('./chart-compositor.js');

function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

const compositorSrc = fs.readFileSync(path.join(__dirname, 'chart-compositor.js'), 'utf8');
const coreSrc = fs.readFileSync(path.join(__dirname, 'chart-core.js'), 'utf8');

{
  const pinFn = coreSrc.slice(
    coreSrc.indexOf('pinLiveTimeView() {'),
    coreSrc.indexOf('forceVisibleLogicalRange(context, range) {'),
  );
  assert(pinFn.includes('TimeCamera.getCanonical'), 'pin reads canonical VIEW');
  assert(pinFn.includes('applyCommittedCamera'), 'pin writes LWC even when commit is not dirty');
  assert(!pinFn.includes('TimeCamera.commit'), 'pin must not no-op through identical commit');
}

function sliceMethod(src, startNeedle, endNeedle) {
  const start = src.indexOf(startNeedle);
  const end = src.indexOf(endNeedle);
  assert(start >= 0 && end > start, `slice ${startNeedle}`);
  return src.slice(start, end);
}

{
  const indicators = sliceMethod(
    compositorSrc,
    '  _flushIndicators(intent) {',
    '  _flushDelta(intent) {',
  );
  const ddrAt = indicators.indexOf('this._applyDdrPlots(snapshot)');
  const pinAt = indicators.indexOf('this._pinSharedTimeView()');
  const muteOff = indicators.indexOf('ChartAdapter.setLiveUpdating(false)');
  assert(ddrAt >= 0 && pinAt > ddrAt, 'indicators pin after DDR setData');
  assert(muteOff > pinAt, 'indicators pin while _liveUpdating is still true');
}

{
  const full = sliceMethod(
    compositorSrc,
    '  _flushFull(storeData, snapshot, intent) {',
    '  _flushPrepend(storeData, snapshot, intent) {',
  );
  const ddrAt = full.indexOf('this._applyDdrPlots(snapshot)');
  const pinAt = full.indexOf('this._pinSharedTimeView()');
  assert(ddrAt >= 0 && pinAt > ddrAt, 'full pin after DDR setData');
}

{
  const prepend = sliceMethod(
    compositorSrc,
    '  _flushPrepend(storeData, snapshot, intent) {',
    '  _applyMarketTimeViewportSync(paintSnapshot, viewportAnchor) {',
  );
  const ddrAt = prepend.indexOf('this._applyDdrPlots(snapshot)');
  const pinAt = prepend.indexOf('this._pinSharedTimeView()');
  assert(ddrAt >= 0 && pinAt > ddrAt, 'prepend pin after DDR setData (not before)');
}

{
  const settle = sliceMethod(
    compositorSrc,
    '  _settleAfterFullPrependPaint(intent) {',
    '  _flushIndicators(intent) {',
  );
  assert(settle.includes("intent.phase === 'F2'"), 'F1 prepend must not decoration-setData after mute off');
  assert(settle.includes('this._pinSharedTimeView()'), 'F2 prepend re-pins after decoration');
}

function stubAdapter(order) {
  global.ChartAdapter = {
    setLiveUpdating(flag) {
      order.push(flag ? 'on' : 'off');
    },
    getVisibleLogicalRange() {
      return { from: 0, to: 40 };
    },
    refreshOscillatorPaneChrome() {
      order.push('chrome');
    },
    pinLiveTimeView() {
      order.push('pin');
      return true;
    },
    applyFullData() {
      order.push('candles');
    },
    applyLiveAnnotationLayer() {},
    refreshLiveDecoration() {
      order.push('deco');
    },
    forceVisibleLogicalRange() {
      return true;
    },
  };
}

function mockStore(n = 50, t0 = 1_700_000_000) {
  const times = [];
  for (let i = 0; i < n; i++) times.push(t0 + i);
  const snapshot = {
    times,
    candles: {
      open: times.map(() => 1),
      high: times.map(() => 2),
      low: times.map(() => 1),
      close: times.map(() => 1.5),
      volume: times.map(() => 1),
    },
    plots: {},
    annotations: [],
  };
  return {
    barCount: () => n,
    timesSec: () => times,
    invariantOk: () => true,
    invariantMeta: () => ({}),
    snapshot: () => snapshot,
  };
}

{
  const origWin = global.window;
  const order = [];
  global.window = {
    DDRFactory: {
      cutoverActive: true,
      hydrateFromColumnar() { order.push('ddr'); },
      applyHydratedData() { order.push('ddrApply'); },
    },
  };
  stubAdapter(order);
  const compositor = new ChartCompositor({ store: mockStore(), onAfterFlush: () => {} });
  compositor.flush({ mode: 'indicators' });
  const pinAt = order.indexOf('pin');
  const ddrAt = order.indexOf('ddr');
  const offAt = order.lastIndexOf('off');
  assert(ddrAt >= 0 && pinAt > ddrAt, `indicators runtime pin after DDR, got ${order.join(',')}`);
  assert(offAt > pinAt, `indicators runtime pin before mute off, got ${order.join(',')}`);
  global.window = origWin;
}

{
  const origWin = global.window;
  global.window = { DDRFactory: { cutoverActive: false } };
  const order = [];
  stubAdapter(order);
  const compositor = new ChartCompositor({ store: mockStore(), onAfterFlush: () => {} });
  compositor.flush({ mode: 'prepend', phase: 'F2', edge: 'left' });
  assert(order.includes('deco'), 'F2 prepend still refreshes decoration');
  assert(order.includes('pin'), 'F2 prepend pins VIEW after decoration');
  const decoAt = order.indexOf('deco');
  const pinAt = order.indexOf('pin');
  const onAt = order.indexOf('on');
  const offAt = order.lastIndexOf('off');
  assert(onAt >= 0 && onAt < decoAt && pinAt > decoAt && offAt > pinAt,
    `F2 decoration+pin under mute, got ${order.join(',')}`);
  global.window = origWin;
}

console.log('wozduh_time_index_pin_test ok');
