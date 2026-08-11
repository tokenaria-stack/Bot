/**
 * TEMPORARY — LEFT prepend Mute & Sync + 5-stage slow-motion + RANGE WRITER TRAP.
 *
 * Stages (single txn id):
 *   1. before → 2. afterSetData → 3. afterForce → 4. endFlush → 5. afterUnmute
 *
 * Writer trap: monkey-patches LWC timeScale setters + logs logical-change events.
 * Fingerprint: delta ≈ -prunedRightCount → console.trace (sniper).
 *
 * Enable: LEFT_PREPEND_CAMERA_DIAG === true (config.js).
 * Does NOT fix the camera.
 */
(function initLeftPrependDiag(global) {
  'use strict';

  const TOL = 1e-6;
  let seq = 0;
  let txn = null;
  let muted = false;
  let phase = 'idle';
  let eventHooksInstalled = false;
  let writerHooksInstalled = false;
  let rangeTrapInstalled = false;
  let origCommit = null;
  let origProposeFromPane = null;
  let origProposePreserve = null;
  /** @type {Map<object, { method: string, original: Function }[]>} */
  const trappedScales = new WeakMap();
  let lastSeenLogical = null;
  let lastSetterAt = 0;

  function enabled() {
    try {
      if (typeof LEFT_PREPEND_CAMERA_DIAG !== 'undefined' && LEFT_PREPEND_CAMERA_DIAG === true) {
        return true;
      }
      if (typeof localStorage !== 'undefined' && localStorage.getItem('LEFT_PREPEND_DIAG') === '1') {
        return true;
      }
      if (typeof location !== 'undefined' && /(?:\?|&)leftDiag=1(?:&|$)/.test(location.search || '')) {
        return true;
      }
    } catch { /* */ }
    return false;
  }

  function now() {
    return (typeof performance !== 'undefined' && performance.now)
      ? performance.now()
      : Date.now();
  }

  function cloneRange(r) {
    if (!r || !Number.isFinite(r.from) || !Number.isFinite(r.to)) return null;
    return { from: r.from, to: r.to };
  }

  function rangesEqual(a, b) {
    if (!a || !b) return false;
    return Math.abs(a.from - b.from) <= TOL && Math.abs(a.to - b.to) <= TOL;
  }

  function shiftRange(r, n) {
    if (!r || !Number.isFinite(n)) return null;
    return { from: r.from + n, to: r.to + n };
  }

  function lwcLogical() {
    try {
      const chart = (typeof ChartAdapter !== 'undefined' && ChartAdapter.getChart)
        ? ChartAdapter.getChart('live', 'price')
        : null;
      return cloneRange(chart?.timeScale?.()?.getVisibleLogicalRange?.());
    } catch {
      return null;
    }
  }

  function lwcTimeRange() {
    try {
      const chart = (typeof ChartAdapter !== 'undefined' && ChartAdapter.getChart)
        ? ChartAdapter.getChart('live', 'price')
        : null;
      const r = chart?.timeScale?.()?.getVisibleRange?.();
      if (!r) return null;
      const from = r.from?.timestamp ?? r.from;
      const to = r.to?.timestamp ?? r.to;
      if (![from, to].every(Number.isFinite)) return null;
      return { from, to };
    } catch {
      return null;
    }
  }

  function lwcRightOffset() {
    try {
      const chart = (typeof ChartAdapter !== 'undefined' && ChartAdapter.getChart)
        ? ChartAdapter.getChart('live', 'price')
        : null;
      const o = chart?.timeScale?.()?.options?.()?.rightOffset;
      return Number.isFinite(o) ? o : null;
    } catch {
      return null;
    }
  }

  function canonicalRightOffset() {
    try {
      if (typeof TimeCamera === 'undefined' || !TimeCamera.getCanonical) return null;
      const o = TimeCamera.getCanonical()?.rightOffset;
      return Number.isFinite(o) ? o : null;
    } catch {
      return null;
    }
  }

  function storeBounds() {
    const store = global.liveColumnarStore;
    if (!store) {
      return { dataFirst: null, dataLast: null, storeCount: null };
    }
    const first = typeof store.firstTimeSec === 'function' ? Number(store.firstTimeSec()) : null;
    const last = typeof store.lastTimeSec === 'function' ? Number(store.lastTimeSec()) : null;
    const count = typeof store.barCount === 'function' ? store.barCount() : null;
    return {
      dataFirst: Number.isFinite(first) ? first : null,
      dataLast: Number.isFinite(last) ? last : null,
      storeCount: Number.isFinite(count) ? count : null,
    };
  }

  function timeFromLogical(logical) {
    const store = global.liveColumnarStore;
    if (!logical || !store || typeof store.snapshot !== 'function') return null;
    const times = store.snapshot()?.times;
    if (!Array.isArray(times) || !times.length) return null;
    const clamp = (i) => {
      if (!Number.isFinite(i)) return null;
      const idx = Math.max(0, Math.min(times.length - 1, Math.floor(i)));
      const sec = Number(times[idx]);
      return Number.isFinite(sec) ? sec : null;
    };
    const left = clamp(logical.from);
    const right = clamp(logical.to);
    if (left == null || right == null) return null;
    return { from: left, to: right };
  }

  function cloneTimePayload(r) {
    if (!r) return null;
    const from = r.from?.timestamp ?? r.from;
    const to = r.to?.timestamp ?? r.to;
    if (![from, to].every(Number.isFinite)) return null;
    return { from, to };
  }

  /** Full stage snapshot — logical + market + rightOffset + data bounds. */
  function captureStage(name) {
    const logical = lwcLogical();
    const marketFromStore = timeFromLogical(logical);
    const marketFromLwc = lwcTimeRange();
    const bounds = storeBounds();
    return {
      stage: String(name),
      t: now(),
      logicalRange: logical,
      marketRange: marketFromStore || marketFromLwc,
      marketRangeLwc: marketFromLwc,
      marketRangeFromLogical: marketFromStore,
      rightOffsetLwc: lwcRightOffset(),
      rightOffsetCanonical: canonicalRightOffset(),
      dataFirst: bounds.dataFirst,
      dataLast: bounds.dataLast,
      storeCount: bounds.storeCount,
    };
  }

  function setPhase(name) {
    phase = String(name || 'idle');
  }

  function isMuted() {
    return muted === true;
  }

  function isActive() {
    return !!txn;
  }

  function noteEvent(name, payload) {
    if (!txn) return;
    const logical = payload?.logical
      || (name.indexOf('Logical') >= 0 ? cloneRange(payload) : lwcLogical());
    const time = payload?.time
      || (name.indexOf('Time') >= 0 ? cloneTimePayload(payload) : lwcTimeRange());
    const prev = lastSeenLogical;
    const deltaFrom = (prev && logical) ? logical.from - prev.from : null;
    const deltaTo = (prev && logical) ? logical.to - prev.to : null;
    const fingerprint = name.indexOf('Logical') >= 0 && isFingerprintDelta(deltaFrom, deltaTo);
    txn.eventSequence.push({
      seq: txn.eventSequence.length + 1,
      t: now(),
      name: String(name),
      phase,
      logical,
      time,
      deltaFrom,
      deltaTo,
      fingerprint,
    });
    if (fingerprint) {
      noteRangeWriter({
        source: 'event',
        method: String(name),
        pane: String(name).split(':')[1] || 'price',
        oldRange: cloneRange(prev),
        newRange: cloneRange(logical),
        deltaFrom,
        deltaTo,
        fingerprint: true,
        msSinceSetter: lastSetterAt ? (now() - lastSetterAt) : null,
        likelyInternal: !lastSetterAt || (now() - lastSetterAt) > 0.25,
        stack: stackSnippet(),
      });
    }
    if (logical) lastSeenLogical = cloneRange(logical);
  }

  function stackSnippet() {
    try {
      return String(new Error('range-writer').stack).split('\n').slice(2, 14).join('\n');
    } catch {
      return null;
    }
  }

  function isFingerprintDelta(deltaFrom, deltaTo) {
    if (!txn) return false;
    const p = Number(txn.prunedRightCount);
    if (!Number.isFinite(p) || p <= 0) return false;
    const d0 = Number(deltaFrom);
    const d1 = Number(deltaTo);
    if (![d0, d1].every(Number.isFinite)) return false;
    return Math.abs(d0 + p) < 0.75 && Math.abs(d1 + p) < 0.75;
  }

  function noteRangeWriter(entry) {
    if (!txn) return;
    const row = {
      seq: txn.rangeWriters.length + 1,
      t: now(),
      phase,
      ...entry,
    };
    txn.rangeWriters.push(row);
    const fp = row.fingerprint === true;
    const tag = fp ? '🔥 FINGERPRINT' : 'range-writer';
    console.log(`[LEFT_PREPEND_DIAG] ${tag}`, {
      id: txn.id,
      method: row.method,
      pane: row.pane,
      phase: row.phase,
      oldRange: row.oldRange,
      newRange: row.newRange,
      deltaFrom: row.deltaFrom,
      deltaTo: row.deltaTo,
      prunedRightCount: txn.prunedRightCount,
      source: row.source,
    });
    if (fp) {
      console.trace(
        `[LEFT_PREPEND_DIAG] FINGERPRINT −${txn.prunedRightCount} via ${row.method} @ ${row.phase}`,
        row,
      );
      txn.fingerprintHits.push(row);
    }
  }

  function trapTimeScaleMethod(ts, pane, method) {
    if (!ts || typeof ts[method] !== 'function') return false;
    const list = trappedScales.get(ts) || [];
    if (list.some((x) => x.method === method)) return true;
    const original = ts[method].bind(ts);
    if (method === 'setVisibleLogicalRange') {
      ts.__rawSetVisibleLogicalRange = original;
    }
    ts[method] = function diagTrappedRangeWriter(arg0) {
      const oldRange = lwcLogical();
      let newRange = null;
      if (method === 'setVisibleLogicalRange') {
        newRange = cloneRange(arg0);
      } else if (method === 'setVisibleRange') {
        newRange = null; // time-based; still log
      }
      const deltaFrom = (oldRange && newRange) ? newRange.from - oldRange.from : null;
      const deltaTo = (oldRange && newRange) ? newRange.to - oldRange.to : null;
      const fingerprint = isFingerprintDelta(deltaFrom, deltaTo);
      noteRangeWriter({
        source: 'setter',
        method: `timeScale.${method}`,
        pane,
        oldRange,
        newRange,
        deltaFrom,
        deltaTo,
        fingerprint,
        arg: method.indexOf('scroll') >= 0 || method === 'fitContent'
          ? arg0
          : cloneRange(arg0) || arg0,
        stack: stackSnippet(),
      });
      lastSetterAt = now();
      const result = original.apply(ts, arguments);
      lastSeenLogical = lwcLogical() || newRange || lastSeenLogical;
      return result;
    };
    list.push({ method, original });
    trappedScales.set(ts, list);
    return true;
  }

  function installRangeSetterTrap() {
    if (typeof ChartAdapter === 'undefined' || typeof ChartAdapter.getChart !== 'function') {
      return false;
    }
    const panes = ['price', 'wozduh', 'rsx'];
    let n = 0;
    panes.forEach((pane) => {
      const chart = ChartAdapter.getChart('live', pane);
      if (!chart?.timeScale) return;
      let ts;
      try { ts = chart.timeScale(); } catch { return; }
      if (!ts) return;
      ['setVisibleLogicalRange', 'setVisibleRange', 'scrollToPosition', 'scrollToRealTime', 'fitContent']
        .forEach((m) => {
          if (trapTimeScaleMethod(ts, pane, m)) n += 1;
        });
    });
    if (n > 0) rangeTrapInstalled = true;
    return rangeTrapInstalled;
  }

  /**
   * Micro-probe between paint steps. Detects LWC-internal mutations (no setter).
   */
  function probeLogical(label) {
    if (!txn) return null;
    setPhase(label);
    const logical = lwcLogical();
    const prev = lastSeenLogical;
    const deltaFrom = (prev && logical) ? logical.from - prev.from : null;
    const deltaTo = (prev && logical) ? logical.to - prev.to : null;
    const changed = !rangesEqual(prev, logical);
    const fingerprint = isFingerprintDelta(deltaFrom, deltaTo);
    const msSinceSetter = lastSetterAt ? (now() - lastSetterAt) : null;
    const row = {
      source: changed && (msSinceSetter == null || msSinceSetter > 0.05) && !fingerprint
        ? 'probe'
        : (fingerprint ? 'probe' : 'probe'),
      method: `probe:${label}`,
      pane: 'price',
      oldRange: cloneRange(prev),
      newRange: cloneRange(logical),
      deltaFrom,
      deltaTo,
      fingerprint,
      changed,
      msSinceSetter,
      stack: fingerprint || (changed && Math.abs(deltaFrom || 0) > 1) ? stackSnippet() : null,
      likelyInternal: changed && (msSinceSetter == null || msSinceSetter > 0.25),
    };
    if (changed || fingerprint) {
      noteRangeWriter(row);
      if (row.likelyInternal && fingerprint) {
        console.warn(
          '[LEFT_PREPEND_DIAG] fingerprint with NO recent setter → LWC-internal or untrapped path',
          { id: txn.id, label, msSinceSetter, prunedRightCount: txn.prunedRightCount },
        );
      }
    }
    lastSeenLogical = cloneRange(logical);
    return logical;
  }

  function noteWriterAttempt(writer, requestedLogical, extra) {
    if (!txn) return;
    txn.cameraWriterAttempts.push({
      t: now(),
      phase,
      writer: String(writer),
      requestedLogical: cloneRange(requestedLogical),
      extra: extra || null,
      stack: stackSnippet(),
    });
  }

  function shouldBlockCommit(patch, options) {
    if (!muted) return false;
    if (options && options.diagForcePin === true) return false;
    if (options && options.diagAuthoritative === true) return false;
    noteWriterAttempt(
      'TimeCamera.commit',
      patch?.visibleRange || null,
      { force: !!options?.force, rangeOnly: !!patch?.rangeOnly, sourceHostId: patch?.sourceHostId },
    );
    return true;
  }

  function shouldBlockProposeFromPane(hostId, visibleRange) {
    if (!muted) return false;
    noteWriterAttempt('TimeCamera.proposeFromPane', visibleRange, { hostId });
    return true;
  }

  function shouldBlockProposePreserve(opts) {
    if (!muted) return false;
    noteWriterAttempt('TimeCamera.proposePreserveViewport', null, {
      anchorTimeMs: opts?.anchorTimeMs,
      edge: opts?.edge,
    });
    return true;
  }

  function begin(meta) {
    if (!enabled()) return false;
    installRangeSetterTrap();
    installEventSpies();
    seq += 1;
    const logicalBefore = cloneRange(meta?.logicalBefore) || lwcLogical();
    const prependedCount = Number(meta?.prependedCount);
    const prunedRightCount = Number(meta?.prunedRightCount);
    const tipBefore = Number(meta?.tipBefore);
    const tipAfter = Number(meta?.tipAfter);
    const beforeSnap = captureStage('before');
    // Prefer pre-mutation logical from intent when provided.
    if (logicalBefore) beforeSnap.logicalRange = logicalBefore;
    beforeSnap.marketRange = timeFromLogical(logicalBefore) || beforeSnap.marketRange;

    txn = {
      id: seq,
      prependedCount: Number.isFinite(prependedCount) ? prependedCount : null,
      prunedRightCount: Number.isFinite(prunedRightCount) ? prunedRightCount : null,
      tipBefore: Number.isFinite(tipBefore) ? tipBefore : null,
      tipAfter: Number.isFinite(tipAfter) ? tipAfter : null,
      storeBefore: meta?.storeBefore ?? null,
      storeAfter: meta?.storeAfter ?? null,
      expectedLogical: Number.isFinite(prependedCount) && logicalBefore
        ? shiftRange(logicalBefore, prependedCount)
        : null,
      stages: {
        before: beforeSnap,
        afterSetData: null,
        afterForce: null,
        endFlush: null,
        afterUnmute: null,
      },
      logicalBefore,
      logicalAfterSetData: null,
      logicalAfterForcePin: null,
      logicalAfterUnmute: null,
      eventSequence: [],
      cameraWriterAttempts: [],
      rangeWriters: [],
      fingerprintHits: [],
      case: null,
      divergence: null,
    };
    lastSeenLogical = cloneRange(logicalBefore) || lwcLogical();
    lastSetterAt = 0;
    muted = false;
    setPhase('before');
    return true;
  }

  function mute() {
    if (!txn) return;
    muted = true;
    setPhase('muted');
  }

  function unmute() {
    muted = false;
    setPhase('unmuted');
  }

  function markAfterSetData() {
    if (!txn) return;
    setPhase('afterSetData');
    const snap = captureStage('afterSetData');
    txn.stages.afterSetData = snap;
    txn.logicalAfterSetData = snap.logicalRange;
  }

  function markAfterForcePin() {
    if (!txn) return;
    setPhase('afterForce');
    const snap = captureStage('afterForce');
    txn.stages.afterForce = snap;
    txn.logicalAfterForcePin = snap.logicalRange;
    lastSeenLogical = cloneRange(snap.logicalRange);
  }

  /** Called from compositor flush finally — same stack as PREPEND_VIEW FAIL. */
  function markEndFlush() {
    if (!txn) return;
    setPhase('endFlush');
    const snap = captureStage('endFlush');
    txn.stages.endFlush = snap;
  }

  function marketDeltaHours(a, b) {
    if (!a || !b) return null;
    if (![a.from, b.from].every(Number.isFinite)) return null;
    return (b.from - a.from) / 3600;
  }

  function analyzeDivergence() {
    if (!txn?.stages) return null;
    const s = txn.stages;
    const out = {
      prunedRightCount: txn.prunedRightCount,
      tipDeltaSec: (Number.isFinite(txn.tipBefore) && Number.isFinite(txn.tipAfter))
        ? txn.tipAfter - txn.tipBefore
        : null,
      marketLeftDeltaHours: {
        afterSetData: marketDeltaHours(s.before?.marketRange, s.afterSetData?.marketRange),
        afterForce: marketDeltaHours(s.before?.marketRange, s.afterForce?.marketRange),
        endFlush: marketDeltaHours(s.before?.marketRange, s.endFlush?.marketRange),
        afterUnmute: marketDeltaHours(s.before?.marketRange, s.afterUnmute?.marketRange),
      },
      rightOffset: {
        before: s.before?.rightOffsetCanonical ?? s.before?.rightOffsetLwc,
        afterSetData: s.afterSetData?.rightOffsetCanonical ?? s.afterSetData?.rightOffsetLwc,
        afterForce: s.afterForce?.rightOffsetCanonical ?? s.afterForce?.rightOffsetLwc,
        endFlush: s.endFlush?.rightOffsetCanonical ?? s.endFlush?.rightOffsetLwc,
        afterUnmute: s.afterUnmute?.rightOffsetCanonical ?? s.afterUnmute?.rightOffsetLwc,
      },
      dataLast: {
        before: s.before?.dataLast,
        afterSetData: s.afterSetData?.dataLast,
        afterForce: s.afterForce?.dataLast,
        endFlush: s.endFlush?.dataLast,
        afterUnmute: s.afterUnmute?.dataLast,
      },
      /** Where market left edge first diverges from before while logical still matches expected. */
      firstMarketDivergeStage: null,
      logicalMatchesExpected: {},
    };

    const stages = ['afterSetData', 'afterForce', 'endFlush', 'afterUnmute'];
    for (let i = 0; i < stages.length; i++) {
      const key = stages[i];
      const snap = s[key];
      const logicalOk = rangesEqual(snap?.logicalRange, txn.expectedLogical);
      out.logicalMatchesExpected[key] = logicalOk;
      const mDelta = out.marketLeftDeltaHours[key];
      if (out.firstMarketDivergeStage == null
        && Number.isFinite(mDelta)
        && Math.abs(mDelta) > 1) {
        out.firstMarketDivergeStage = key;
        out.firstMarketDivergeHours = mDelta;
        out.firstMarketDivergeWhileLogicalOk = logicalOk;
      }
    }

    // Gemini hypothesis check: market jump ≈ -prunedRightCount (hours on 1h TF).
    if (Number.isFinite(txn.prunedRightCount) && txn.prunedRightCount > 0) {
      const endDelta = out.marketLeftDeltaHours.endFlush;
      const unmuteDelta = out.marketLeftDeltaHours.afterUnmute;
      out.geminiRightOffsetHypothesis = {
        prunedRightCount: txn.prunedRightCount,
        endFlushMarketDeltaHours: endDelta,
        afterUnmuteMarketDeltaHours: unmuteDelta,
        matchesPruneAtEndFlush: Number.isFinite(endDelta)
          && Math.abs(endDelta + txn.prunedRightCount) < 2,
        matchesPruneAtAfterUnmute: Number.isFinite(unmuteDelta)
          && Math.abs(unmuteDelta + txn.prunedRightCount) < 2,
        rightOffsetUnchangedAbsolute:
          out.rightOffset.before != null
          && out.rightOffset.endFlush != null
          && Math.abs(out.rightOffset.before - out.rightOffset.endFlush) < TOL,
      };
    }
    return out;
  }

  function classify() {
    if (!txn) return 'CASE_D';
    const forceOk = rangesEqual(txn.logicalAfterForcePin, txn.expectedLogical);
    const unmuteOk = rangesEqual(txn.logicalAfterUnmute, txn.expectedLogical);
    if (forceOk && !unmuteOk) return 'CASE_A_FORCE_PIN_SUCCEEDED_BUT_WAS_OVERWRITTEN';
    if (!forceOk) return 'CASE_B_FORCE_PIN_FAILED';
    if (forceOk && unmuteOk) return 'CASE_C_FORCE_PIN_SURVIVED';
    return 'CASE_D_UNCLASSIFIED';
  }

  function report() {
    if (!txn) return null;
    const forcePinCorrect = rangesEqual(txn.logicalAfterForcePin, txn.expectedLogical);
    const unmutePreserved = rangesEqual(txn.logicalAfterUnmute, txn.expectedLogical);
    txn.case = classify();
    txn.divergence = analyzeDivergence();
    const report = {
      id: txn.id,
      case: txn.case,
      forcePinCorrect,
      unmutePreserved,
      prependedCount: txn.prependedCount,
      prunedRightCount: txn.prunedRightCount,
      tipBefore: txn.tipBefore,
      tipAfter: txn.tipAfter,
      storeBefore: txn.storeBefore,
      storeAfter: txn.storeAfter,
      expectedLogical: txn.expectedLogical,
      stages: {
        before: txn.stages.before,
        afterSetData: txn.stages.afterSetData,
        afterForce: txn.stages.afterForce,
        endFlush: txn.stages.endFlush,
        afterUnmute: txn.stages.afterUnmute,
      },
      divergence: txn.divergence,
      rangeWriters: txn.rangeWriters.slice(),
      fingerprintHits: txn.fingerprintHits.slice(),
      eventSequence: txn.eventSequence.slice(),
      cameraWriterAttempts: txn.cameraWriterAttempts.slice(),
    };
    global.__LEFT_PREPEND_DIAG_LAST__ = report;
    global.__LEFT_PREPEND_DIAG_LOG__ = global.__LEFT_PREPEND_DIAG_LOG__ || [];
    global.__LEFT_PREPEND_DIAG_LOG__.push(report);
    if (global.__LEFT_PREPEND_DIAG_LOG__.length > 20) global.__LEFT_PREPEND_DIAG_LOG__.shift();

    console.log('[LEFT_PREPEND_DIAG]', report.case, {
      id: report.id,
      prependedCount: report.prependedCount,
      prunedRightCount: report.prunedRightCount,
      forcePinCorrect: report.forcePinCorrect,
      unmutePreserved: report.unmutePreserved,
      firstMarketDivergeStage: report.divergence?.firstMarketDivergeStage,
      fingerprintHits: report.fingerprintHits.length,
      geminiHypothesis: report.divergence?.geminiRightOffsetHypothesis,
    });
    console.log('[LEFT_PREPEND_DIAG] stages', report.stages);
    console.log('[LEFT_PREPEND_DIAG] divergence', report.divergence);
    if (report.fingerprintHits.length) {
      console.warn('[LEFT_PREPEND_DIAG] fingerprintHits', report.fingerprintHits);
    } else {
      console.log('[LEFT_PREPEND_DIAG] rangeWriters', report.rangeWriters);
    }
    return report;
  }

  function releaseAndMeasureAfterRaf() {
    if (!txn) return;
    unmute();
    setPhase('awaitRaf');
    const finish = () => {
      // Overlapping prepends / settle can null txn before this double-rAF runs.
      if (!txn || !txn.stages) return;
      setPhase('afterUnmute');
      const snap = captureStage('afterUnmute');
      txn.stages.afterUnmute = snap;
      txn.logicalAfterUnmute = snap.logicalRange;
      report();
      txn = null;
      muted = false;
      phase = 'idle';
    };
    if (typeof requestAnimationFrame === 'function') {
      requestAnimationFrame(() => {
        requestAnimationFrame(finish);
      });
    } else {
      finish();
    }
  }

  function abort() {
    txn = null;
    muted = false;
    phase = 'idle';
  }

  function installEventSpies() {
    if (eventHooksInstalled) return;
    if (typeof ChartAdapter === 'undefined' || typeof ChartAdapter.getChart !== 'function') return;
    const panes = ['price', 'wozduh', 'rsx'];
    let attached = 0;
    panes.forEach((pane) => {
      const chart = ChartAdapter.getChart('live', pane);
      if (!chart?.timeScale) return;
      const ts = chart.timeScale();
      try {
        ts.subscribeVisibleLogicalRangeChange((range) => {
          if (!txn) return;
          noteEvent(`visibleLogicalRangeChanged:${pane}`, { logical: cloneRange(range) });
        });
        if (typeof ts.subscribeVisibleTimeRangeChange === 'function') {
          ts.subscribeVisibleTimeRangeChange((range) => {
            if (!txn) return;
            noteEvent(`visibleTimeRangeChanged:${pane}`, { time: cloneTimePayload(range) });
          });
        }
        attached += 1;
      } catch { /* */ }
    });
    if (attached > 0) eventHooksInstalled = true;
  }

  function installWriterHooks() {
    if (writerHooksInstalled) return;
    if (typeof TimeCamera === 'undefined') return;
    origCommit = TimeCamera.commit.bind(TimeCamera);
    origProposeFromPane = TimeCamera.proposeFromPane.bind(TimeCamera);
    origProposePreserve = TimeCamera.proposePreserveViewport
      ? TimeCamera.proposePreserveViewport.bind(TimeCamera)
      : null;

    TimeCamera.commit = function diagCommit(patch, options) {
      if (shouldBlockCommit(patch, options)) return false;
      return origCommit(patch, options);
    };
    TimeCamera.proposeFromPane = function diagPropose(hostId, visibleRange, barSpacing) {
      if (shouldBlockProposeFromPane(hostId, visibleRange)) return false;
      return origProposeFromPane(hostId, visibleRange, barSpacing);
    };
    if (origProposePreserve) {
      TimeCamera.proposePreserveViewport = function diagPreserve(opts) {
        if (shouldBlockProposePreserve(opts)) return false;
        return origProposePreserve(opts);
      };
    }
    writerHooksInstalled = true;
  }

  function install() {
    if (!enabled()) return false;
    installWriterHooks();
    installEventSpies();
    installRangeSetterTrap();
    return true;
  }

  const LeftPrependDiag = {
    enabled,
    install,
    begin,
    mute,
    unmute,
    isMuted,
    isActive,
    setPhase,
    markAfterSetData,
    markAfterForcePin,
    markEndFlush,
    probeLogical,
    releaseAndMeasureAfterRaf,
    abort,
    noteEvent,
    noteWriterAttempt,
    shouldBlockCommit,
    lwcLogical,
    cloneRange,
    captureStage,
  };

  global.LeftPrependDiag = LeftPrependDiag;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = LeftPrependDiag;
  }
})(typeof window !== 'undefined' ? window : globalThis);
