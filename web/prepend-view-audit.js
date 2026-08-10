/**
 * TEMPORARY P0 diagnostic — market-time VIEW immutability across prepend.
 * One record per prepend. Remove after the FAIL/PASS gate is resolved.
 * Does not change camera behavior.
 */
(function (global) {
  'use strict';

  let seq = 0;
  let active = null;
  let patched = false;
  let gestureBound = false;

  function storeTimes() {
    const store = global.liveColumnarStore;
    if (!store || typeof store.snapshot !== 'function') return null;
    const snap = store.snapshot();
    return Array.isArray(snap?.times) ? snap.times : null;
  }

  function lwcLogicalRange() {
    try {
      const chart = (typeof ChartAdapter !== 'undefined' && ChartAdapter.getChart)
        ? ChartAdapter.getChart('live', 'price')
        : null;
      const r = chart?.timeScale?.()?.getVisibleLogicalRange?.();
      if (r && Number.isFinite(r.from) && Number.isFinite(r.to)) {
        return { from: r.from, to: r.to };
      }
    } catch { /* */ }
    return null;
  }

  function canonicalRange() {
    if (typeof TimeCamera === 'undefined' || !TimeCamera.getCanonical) return null;
    const r = TimeCamera.getCanonical()?.visibleRange;
    if (r && Number.isFinite(r.from) && Number.isFinite(r.to)) {
      return { from: r.from, to: r.to };
    }
    return null;
  }

  function logicalRange() {
    return lwcLogicalRange() || canonicalRange();
  }

  /** Market-time edges from store × LWC logical range. Indices are diagnostic only. */
  function capturePhase(phase) {
    const times = storeTimes();
    const lwc = lwcLogicalRange();
    const canon = canonicalRange();
    const range = lwc || canon;
    const n = times ? times.length : (global.liveColumnarStore?.barCount?.() ?? 0);
    if (!times || !times.length || !range) {
      return {
        phase,
        leftTime: null,
        rightTime: null,
        leftIndex: range?.from ?? null,
        rightIndex: range?.to ?? null,
        lwcRange: lwc,
        canonicalRange: canon,
        storeBarCount: n,
      };
    }
    const clamp = (logical) => {
      if (!Number.isFinite(logical)) return null;
      if (logical < 0) return 0;
      return Math.min(times.length - 1, Math.max(0, Math.floor(logical)));
    };
    const li = clamp(range.from);
    const ri = clamp(range.to);
    const leftSec = li == null ? null : Number(times[li]);
    const rightSec = ri == null ? null : Number(times[ri]);
    return {
      phase,
      leftTime: Number.isFinite(leftSec) ? leftSec : null,
      rightTime: Number.isFinite(rightSec) ? rightSec : null,
      leftIndex: range.from,
      rightIndex: range.to,
      lwcRange: lwc,
      canonicalRange: canon,
      storeBarCount: n,
    };
  }

  function rangeEqual(a, b) {
    if (!a || !b) return false;
    return a.from === b.from && a.to === b.to;
  }

  /**
   * Classify setData boundary: A = LWC logical range moved; B = range stable, market-time mapping moved.
   */
  function classifySetData(before, after) {
    if (!before || !after) return null;
    const lwcSame = rangeEqual(before.lwcRange, after.lwcRange);
    const timeSame = edgesEqual(before, after);
    if (!lwcSame && !timeSame) return 'A_LWC_RANGE_CHANGED';
    if (lwcSame && !timeSame) return 'B_RANGE_STABLE_TIME_REMAPPED';
    if (lwcSame && timeSame) return 'STABLE';
    return 'A_LWC_RANGE_CHANGED_TIME_STABLE';
  }

  function iso(sec) {
    if (!Number.isFinite(sec)) return 'null';
    try {
      return new Date(sec * 1000).toISOString().replace('.000Z', 'Z');
    } catch {
      return String(sec);
    }
  }

  function edgesEqual(a, b) {
    if (!a || !b) return false;
    return a.leftTime === b.leftTime && a.rightTime === b.rightTime;
  }

  function beginBeforeMerge() {
    // Keep the earliest beforeMerge if coalesced prepends share one paint.
    if (active) return active.id;
    seq += 1;
    active = {
      id: seq,
      userGestureDuringTransaction: false,
      phases: { beforeMerge: capturePhase('beforeMerge') },
      writers: [],
    };
    return active.id;
  }

  function markPhase(phase) {
    if (!active) return null;
    const snap = capturePhase(phase);
    active.phases[phase] = snap;
    active.writers.push({ writer: phase, snap });
    return snap;
  }

  function onProposeFromPane(committed) {
    if (!active || !committed) return;
    const snap = capturePhase('proposeFromPane');
    active.writers.push({ writer: 'proposeFromPane', snap });
  }

  function noteUserGesture() {
    if (active) active.userGestureDuringTransaction = true;
  }

  /**
   * Attribute FAIL without treating afterSetData(old logical × new store) as culprit
   * when afterPreserve already restored market-time edges.
   */
  function attributeFail(before, phases, writers) {
    const afterSetData = phases.afterSetData || null;
    const afterPreserve = phases.afterPreserve || null;

    if (afterPreserve && !edgesEqual(before, afterPreserve)) {
      if (afterSetData && !edgesEqual(before, afterSetData)) {
        return { firstBadWriter: 'afterSetData', firstBadPhase: 'afterSetData', firstBad: afterSetData };
      }
      return {
        firstBadWriter: 'proposePreserveViewport',
        firstBadPhase: 'afterPreserve',
        firstBad: afterPreserve,
      };
    }

    // Preserve never ran — setData left market-time wrong through flush.
    if (!afterPreserve && afterSetData && !edgesEqual(before, afterSetData)) {
      return { firstBadWriter: 'afterSetData', firstBadPhase: 'afterSetData', firstBad: afterSetData };
    }

    for (let i = 0; i < writers.length; i++) {
      const w = writers[i];
      if (w.writer === 'proposeFromPane' && !edgesEqual(before, w.snap)) {
        return { firstBadWriter: 'proposeFromPane', firstBadPhase: 'proposeFromPane', firstBad: w.snap };
      }
    }

    const flushEnd = phases.flushEnd;
    return {
      firstBadWriter: 'flushEnd',
      firstBadPhase: 'flushEnd',
      firstBad: flushEnd,
    };
  }

  function endFlush() {
    if (!active) return null;
    const flushEnd = capturePhase('flushEnd');
    active.phases.flushEnd = flushEnd;
    active.writers.push({ writer: 'flushEnd', snap: flushEnd });

    const before = active.phases.beforeMerge;
    const afterSetData = active.phases.afterSetData || null;
    const afterPreserve = active.phases.afterPreserve || null;

    let resultTag;
    let firstBadWriter = null;
    let firstBadPhase = null;
    let firstBad = null;

    if (active.userGestureDuringTransaction) {
      resultTag = 'SKIP_USER_GESTURE';
    } else if (before && edgesEqual(before, flushEnd)) {
      resultTag = 'PASS';
    } else {
      resultTag = 'FAIL';
      const bad = attributeFail(before, active.phases, active.writers);
      firstBadWriter = bad.firstBadWriter;
      firstBadPhase = bad.firstBadPhase;
      firstBad = bad.firstBad;
    }

    const result = {
      id: active.id,
      result: resultTag,
      userGestureDuringTransaction: active.userGestureDuringTransaction,
      firstBadWriter,
      firstBadPhase,
      firstBad,
      beforeMerge: before,
      beforeSetData: active.phases.beforeSetData || null,
      afterCandleSetData: active.phases.afterCandleSetData || null,
      afterScaleApply: active.phases.afterScaleApply || null,
      afterSetData,
      afterPreserve,
      flushEnd,
      setDataCase: classifySetData(
        active.phases.beforeSetData || before,
        active.phases.afterCandleSetData || afterSetData,
      ),
      setDataCaseAfterScale: classifySetData(
        active.phases.beforeSetData || before,
        active.phases.afterScaleApply || afterSetData,
      ),
      preserveRecovered: !!(before && afterPreserve && edgesEqual(before, afterPreserve)),
      addedBars: (flushEnd?.storeBarCount ?? 0) - (before?.storeBarCount ?? 0),
    };

    global.__PREPEND_VIEW_LAST__ = result;
    global.__PREPEND_VIEW_LOG__ = global.__PREPEND_VIEW_LOG__ || [];
    global.__PREPEND_VIEW_LOG__.push(result);
    if (global.__PREPEND_VIEW_LOG__.length > 20) global.__PREPEND_VIEW_LOG__.shift();

    if (resultTag === 'PASS') {
      console.info(
        `PREPEND_VIEW PASS #${result.id} left ${iso(before.leftTime)} → ${iso(flushEnd.leftTime)} `
        + `right ${iso(before.rightTime)} → ${iso(flushEnd.rightTime)} bars +${result.addedBars}`,
      );
    } else if (resultTag === 'FAIL') {
      console.warn(
        `PREPEND_VIEW FAIL #${result.id} firstBadWriter=${firstBadWriter} `
        + `firstBadPhase=${firstBadPhase} setDataCase=${result.setDataCase} `
        + `preserveRecovered=${result.preserveRecovered} `
        + `left ${iso(before?.leftTime)} → ${iso(flushEnd?.leftTime)} `
        + `right ${iso(before?.rightTime)} → ${iso(flushEnd?.rightTime)}`,
        result,
      );
    } else {
      console.info(`PREPEND_VIEW SKIP_USER_GESTURE #${result.id} bars +${result.addedBars}`);
    }

    active = null;
    return result;
  }

  function install() {
    if (patched) return;
    if (typeof TimeCamera === 'undefined' || !TimeCamera.proposeFromPane) return;
    patched = true;
    const orig = TimeCamera.proposeFromPane.bind(TimeCamera);
    TimeCamera.proposeFromPane = function (hostId, visibleRange, barSpacing) {
      const ok = orig(hostId, visibleRange, barSpacing);
      try {
        onProposeFromPane(ok === true);
      } catch { /* diagnostic only */ }
      return ok;
    };
  }

  function attachGestureWatch() {
    if (gestureBound || typeof document === 'undefined') return;
    const root = document.getElementById('live-chart-container');
    if (!root) return;
    gestureBound = true;
    const mark = () => noteUserGesture();
    root.addEventListener('wheel', mark, { passive: true, capture: true });
    root.addEventListener('pointerdown', mark, { passive: true, capture: true });
  }

  function abort() {
    active = null;
  }

  const PrependViewAudit = {
    beginBeforeMerge,
    markPhase,
    endFlush,
    abort,
    noteUserGesture,
    install,
    attachGestureWatch,
    isActive: () => active != null,
  };

  global.PrependViewAudit = PrependViewAudit;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = { PrependViewAudit };
  }
})(typeof window !== 'undefined' ? window : global);
