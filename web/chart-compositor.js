/**
 * ChartCompositor — sole live-chart paint authority (Core 2.3).
 * Reads ColumnarStore snapshots; paints via ChartAdapter only.
 *
 * Track C / WS-04: paint the retained Working Set covering the committed VIEW.
 * LWC logical indices ≡ store indices — no soft tip-window that clamps the camera.
 * TimeCamera remains sole VIEW owner; compositor only translates times → series indices.
 */
class ChartCompositor {
  /**
   * @param {object} options
   * @param {ColumnarStore} options.store
   * @param {() => boolean} [options.shouldPaint]
   * @param {() => void} [options.onAfterFlush]
   * @param {() => object|null} [options.getNavigatorResult]
   */
  constructor(options = {}) {
    this._store = options.store;
    this._shouldPaint = typeof options.shouldPaint === 'function' ? options.shouldPaint : () => true;
    this._onAfterFlush = typeof options.onAfterFlush === 'function' ? options.onAfterFlush : null;
    this._getNavigatorResult = typeof options.getNavigatorResult === 'function'
      ? options.getNavigatorResult
      : () => null;
    /** @type {number} */
    this._prependCameraGen = 0;
    /** @type {boolean} */
    this._prependCameraPending = false;
    /** @type {Function[]} */
    this._prependCameraSettledCbs = [];
  }

  /** True while LEFT-prepend camera restore rAF chain is in flight. */
  isPrependCameraPending() {
    return this._prependCameraPending === true;
  }

  /**
   * RenderScheduler waits here before releasing busy after prepend F2.
   * @param {() => void} cb
   */
  whenPrependCameraSettled(cb) {
    if (typeof cb !== 'function') return;
    if (!this._prependCameraPending) {
      cb();
      return;
    }
    this._prependCameraSettledCbs.push(cb);
  }

  _notifyPrependCameraSettled() {
    this._prependCameraPending = false;
    const cbs = this._prependCameraSettledCbs.splice(0);
    for (let i = 0; i < cbs.length; i++) {
      try { cbs[i](); } catch { /* */ }
    }
  }

  /**
   * Resolve VIEW time bounds for paint from logical range + snapshot times.
   * @param {number[]} timesSec
   * @param {{ from: number, to: number }|null|undefined} range
   * @returns {{ viewFromSec: number, viewToSec: number }|null}
   */
  static viewTimesFromLogicalRange(timesSec, range) {
    if (typeof ColumnarStore !== 'undefined'
      && typeof ColumnarStore.logicalRangeToViewTimes === 'function') {
      return ColumnarStore.logicalRangeToViewTimes(timesSec, range);
    }
    if (!Array.isArray(timesSec) || !timesSec.length || !range) return null;
    if (!Number.isFinite(range.from) || !Number.isFinite(range.to) || !(range.to > range.from)) {
      return null;
    }
    const n = timesSec.length;
    let i0 = Math.floor(range.from);
    let i1 = Math.floor(range.to);
    if (i1 < i0) i1 = i0;
    i0 = Math.max(0, Math.min(n - 1, i0));
    i1 = Math.max(0, Math.min(n - 1, i1));
    const viewFromSec = Number(timesSec[i0]);
    const viewToSec = Number(timesSec[i1]);
    if (![viewFromSec, viewToSec].every(Number.isFinite)) return null;
    return { viewFromSec, viewToSec };
  }

  /**
   * Half-open index range covering [viewFromSec, viewToSec] in snapshot.times.
   * @returns {{ start: number, end: number }|null}
   */
  static viewIndexRange(timesSec, viewFromSec, viewToSec) {
    if (!Array.isArray(timesSec) || !timesSec.length) return null;
    const a = Number(viewFromSec);
    const b = Number(viewToSec);
    if (![a, b].every(Number.isFinite) || b < a) return null;
    const n = timesSec.length;
    let start = 0;
    while (start < n && Number(timesSec[start]) < a) start += 1;
    let end = n;
    while (end > start && Number(timesSec[end - 1]) > b) end -= 1;
    if (start >= end) return null;
    return { start, end };
  }

  /**
   * Track C / WS-04: series to paint for the committed VIEW.
   * Working Set retention already holds required bars — paint the full retained
   * snapshot so painted logical indices match store indices (no soft 15k wall).
   *
   * @param {object} snapshot
   * @param {{ viewFromSec?: number|null, viewToSec?: number|null }} [viewOpts]
   * @returns {object}
   */
  static selectPaintSnapshot(snapshot, viewOpts = {}) {
    if (!snapshot || !Array.isArray(snapshot.times)) return snapshot;
    ChartCompositor._reportIfViewMissing(snapshot, viewOpts);
    return snapshot;
  }

  /**
   * Contract check: committed VIEW times must exist in the painted store.
   * Reports failure; does not invent a camera move or nearest-snap.
   */
  static _reportIfViewMissing(snapshot, viewOpts = {}) {
    const from = Number(viewOpts?.viewFromSec);
    const to = Number(viewOpts?.viewToSec);
    if (![from, to].every(Number.isFinite) || to < from) return;
    const view = ChartCompositor.viewIndexRange(snapshot.times, from, to);
    if (view) return;
    console.error('[ChartCompositor] VIEW times absent from store — paint/data contract failure', {
      viewFromSec: from,
      viewToSec: to,
      storeFirst: snapshot.times[0],
      storeLast: snapshot.times[snapshot.times.length - 1],
      barCount: snapshot.times.length,
    });
  }

  /**
   * Capture VIEW times for the current live logical range against a snapshot.
   * @param {object} snapshot
   * @returns {{ viewFromSec: number, viewToSec: number }|null}
   */
  static capturePaintViewTimes(snapshot) {
    const times = snapshot?.times;
    if (!Array.isArray(times) || !times.length) return null;
    if (typeof ChartAdapter === 'undefined'
      || typeof ChartAdapter.getVisibleLogicalRange !== 'function') {
      return null;
    }
    return ChartCompositor.viewTimesFromLogicalRange(
      times,
      ChartAdapter.getVisibleLogicalRange('live'),
    );
  }

  /**
   * @param {{ mode: 'full'|'prepend'|'delta'|'indicators', addedBars?: number, viewport?: string, viewportRange?: object|null, anchor?: object, tick?: object, delta?: object }} intent
   */
  flush(intent) {
    // Lonely candle: LWC cannot build a sane X-axis from <2 points (WS race vs REST).
    if (!this._store || (typeof this._store.barCount === 'function' && this._store.barCount() < 2)) {
      return;
    }
    if (!this._shouldPaint()) return;
    if (typeof ChartAdapter === 'undefined') return;

    if (intent.mode === 'delta') {
      this._flushDelta(intent);
      return;
    }

    if (intent.mode === 'indicators') {
      this._flushIndicators(intent);
      return;
    }

    if (!this._store.invariantOk()) {
      console.error('[ChartCompositor] invariant failed — skip paint', this._store.invariantMeta());
      return;
    }

    const snapshot = this._store.snapshot();
    const viewTimes = ChartCompositor.capturePaintViewTimes(snapshot);
    const paintSnapshot = ChartCompositor.selectPaintSnapshot(snapshot, viewTimes || {});
    const storeData = ChartCompositor.snapshotToStoreData(paintSnapshot);

    ChartAdapter.setLiveUpdating(true);
    try {
      if (intent.mode === 'prepend') {
        this._flushPrepend(storeData, paintSnapshot, intent);
      } else {
        this._flushFull(storeData, paintSnapshot, intent);
      }
    } finally {
      ChartAdapter.setLiveUpdating(false);
      // LEFT prepend: market-time restore already applied sync in _flushPrepend.
      // No rAF / applyRaw / Mode A. Decoration + scheduler settle only.
      const leftPrepend = intent.mode === 'prepend' && intent?.edge !== 'right';
      if (leftPrepend) {
        this._pendingLogicalRestore = null;
        if (typeof ChartAdapter !== 'undefined'
          && typeof ChartAdapter.refreshLiveDecoration === 'function') {
          ChartAdapter.refreshLiveDecoration();
        }
        if (typeof PrependViewAudit !== 'undefined' && PrependViewAudit.isActive()) {
          PrependViewAudit.markPhase('afterPreserve');
          PrependViewAudit.endFlush();
        }
        if (typeof LeftPrependDiag !== 'undefined'
          && LeftPrependDiag.isActive
          && LeftPrependDiag.isActive()) {
          if (typeof LeftPrependDiag.markEndFlush === 'function') {
            LeftPrependDiag.markEndFlush();
          }
          if (typeof LeftPrependDiag.releaseAndMeasureAfterRaf === 'function') {
            LeftPrependDiag.releaseAndMeasureAfterRaf();
          }
        }
        this._notifyPrependCameraSettled();
      } else if (intent.mode === 'prepend' && this._pendingLogicalRestore) {
        const pending = this._pendingLogicalRestore;
        this._pendingLogicalRestore = null;
        ChartCompositor.applyRawVisibleLogicalRange(pending);
        if (typeof ChartAdapter !== 'undefined'
          && typeof ChartAdapter.refreshLiveDecoration === 'function') {
          ChartAdapter.refreshLiveDecoration();
        }
        if (typeof PrependViewAudit !== 'undefined' && PrependViewAudit.isActive()) {
          PrependViewAudit.markPhase('afterPreserve');
          PrependViewAudit.endFlush();
        }
        if (typeof LeftPrependDiag !== 'undefined'
          && LeftPrependDiag.isActive
          && LeftPrependDiag.isActive()
          && typeof LeftPrependDiag.markEndFlush === 'function') {
          LeftPrependDiag.markEndFlush();
        }
        this._notifyPrependCameraSettled();
      } else {
        if (this._pendingLogicalRestore) {
          ChartCompositor.applyRawVisibleLogicalRange(this._pendingLogicalRestore);
          this._pendingLogicalRestore = null;
        }
        if (intent.mode === 'prepend'
          && typeof PrependViewAudit !== 'undefined'
          && PrependViewAudit.isActive()) {
          PrependViewAudit.markPhase('afterPreserve');
          PrependViewAudit.endFlush();
        }
        if (intent.mode === 'prepend'
          && typeof LeftPrependDiag !== 'undefined'
          && LeftPrependDiag.isActive
          && LeftPrependDiag.isActive()
          && typeof LeftPrependDiag.markEndFlush === 'function') {
          LeftPrependDiag.markEndFlush();
        }
        if (intent.mode === 'prepend' && !this._prependCameraPending) {
          this._notifyPrependCameraSettled();
        }
      }
      if (this._onAfterFlush) this._onAfterFlush(intent);
    }
  }

  /**
   * Emergency LEFT-prepend camera transport: raw LWC write after data txn.
   * Must clear pathological rightOffset/barSpacing — after tip-prune setData, LWC
   * often ignores bare setVisibleLogicalRange while rightOffset stays ≈ prunedRight.
   * @param {{ from: number, to: number }} range
   * @param {{ tipLogical?: number|null }} [opts]
   */
  static applyRawVisibleLogicalRange(range, opts = {}) {
    if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to) || !(range.to > range.from)) {
      return false;
    }
    if (typeof ChartAdapter === 'undefined' || typeof ChartAdapter.getChart !== 'function') {
      return false;
    }
    const expected = { from: range.from, to: range.to };
    const width = expected.to - expected.from;
    let tipLogical = Number(opts.tipLogical);
    if (!Number.isFinite(tipLogical) || tipLogical < 0) {
      tipLogical = NaN;
    }
    const rightOffset = Number.isFinite(tipLogical)
      ? Math.max(0, expected.to - tipLogical)
      : 0;
    let hostWidth = 0;
    try {
      const host = typeof document !== 'undefined'
        ? document.getElementById('price-chart')
        : null;
      hostWidth = host && host.clientWidth > 0 ? host.clientWidth : 0;
    } catch { /* */ }
    const barSpacing = hostWidth > 0 && width > 0 ? hostWidth / width : null;

    let ok = false;
    ['price', 'wozduh', 'rsx'].forEach((pane) => {
      try {
        const chart = ChartAdapter.getChart('live', pane);
        const ts = chart?.timeScale?.();
        if (!ts) return;
        const scaleOpts = { rightOffset, shiftVisibleRangeOnNewBar: false };
        if (Number.isFinite(barSpacing) && barSpacing > 0) {
          scaleOpts.barSpacing = barSpacing;
        }
        if (typeof ts.applyOptions === 'function') {
          ts.applyOptions(scaleOpts);
        }
        const raw = typeof ts.__rawSetVisibleLogicalRange === 'function'
          ? ts.__rawSetVisibleLogicalRange
          : (typeof ts.setVisibleLogicalRange === 'function'
            ? ts.setVisibleLogicalRange.bind(ts)
            : null);
        if (!raw) return;
        raw(expected);
        // Time-based fallback when logical write is ignored in the same turn.
        const timesSec = opts.timesSec;
        if (Array.isArray(timesSec) && timesSec.length
          && typeof ts.setVisibleRange === 'function'
          && expected.from >= 0) {
          try {
            const cur = typeof ts.getVisibleLogicalRange === 'function'
              ? ts.getVisibleLogicalRange()
              : null;
            const stuck = cur
              && Math.abs(cur.from - expected.from) > 0.5
              && Math.abs(cur.to - expected.to) > 0.5;
            if (stuck) {
              const i0 = Math.max(0, Math.min(timesSec.length - 1, Math.floor(expected.from)));
              const i1 = Math.max(i0 + 1, Math.min(timesSec.length - 1, Math.floor(expected.to)));
              const t0 = Number(timesSec[i0]);
              const t1 = Number(timesSec[i1]);
              if (Number.isFinite(t0) && Number.isFinite(t1) && t1 > t0) {
                ts.setVisibleRange({ from: t0, to: t1 });
              }
            }
          } catch { /* */ }
        }
        ok = true;
      } catch { /* */ }
    });
    // Keep TimeCamera canonical in lockstep — stale canonical re-applies unshifted range later.
    if (ok && typeof TimeCamera !== 'undefined' && typeof TimeCamera.commit === 'function') {
      try {
        if (typeof TimeCamera.beginPreserveTransaction === 'function') {
          TimeCamera.beginPreserveTransaction();
        }
        const patch = {
          visibleRange: expected,
          sourceHostId: 'system',
          rangeOnly: true,
          rightOffset,
        };
        if (Number.isFinite(barSpacing) && barSpacing > 0) {
          patch.barSpacing = barSpacing;
        }
        TimeCamera.commit(patch, {
          force: true,
          diagForcePin: true,
        });
      } catch { /* */ }
    }
    return ok;
  }

  /**
   * Soft update: DDR plots (+ annotations) only — never setData on price candles.
   */
  _flushIndicators(intent) {
    if (!this._store.invariantOk()) {
      console.error('[ChartCompositor] invariant failed — skip indicators', this._store.invariantMeta());
      return;
    }
    const raw = this._store.snapshot();
    const viewTimes = ChartCompositor.capturePaintViewTimes(raw);
    const snapshot = ChartCompositor.selectPaintSnapshot(raw, viewTimes || {});
    ChartAdapter.setLiveUpdating(true);
    try {
      this._applyDdrPlots(snapshot);
      const storeData = ChartCompositor.snapshotToStoreData(snapshot);
      this._applyAnnotations(storeData);
    } finally {
      ChartAdapter.setLiveUpdating(false);
      if (this._onAfterFlush) this._onAfterFlush(intent);
    }
  }

  _flushDelta(intent) {
    const chain = Array.isArray(intent?.deltas) && intent.deltas.length
      ? intent.deltas
      : (intent?.delta?.candle ? [intent.delta] : []);
    if (!chain.length) return;

    const ticks = Array.isArray(intent?.ticks) ? intent.ticks : [];
    const fallbackTick = intent?.tick ?? null;

    ChartAdapter.setLiveUpdating(true);
    try {
      for (let i = 0; i < chain.length; i++) {
        const delta = chain[i];
        if (!delta?.candle) continue;
        const tick = ticks[i] ?? (i === chain.length - 1 ? fallbackTick : null);
        if (tick?.plots && typeof window !== 'undefined' && window.DDRFactory?.cutoverActive) {
          window.DDRFactory.updateTick(tick.time, tick.plots);
        }
        ChartAdapter.applyDelta('live', {
          ...delta,
          barCount: delta.barCount ?? this._store.barCount(),
        });
      }
      // Tip may advance on new bars — refresh observation cache after paint (no camera policy).
      if (typeof this._store.snapshot === 'function') {
        const raw = this._store.snapshot();
        const viewTimes = ChartCompositor.capturePaintViewTimes(raw);
        const snap = ChartCompositor.selectPaintSnapshot(raw, viewTimes || {});
        this._observeShadowWorld(snap);
      }
    } finally {
      ChartAdapter.setLiveUpdating(false);
      if (this._onAfterFlush) this._onAfterFlush(intent);
    }
  }

  _flushFull(storeData, snapshot, intent) {
    // Shot 7 atomic frame: Scheduler may still split F1/F2 RAF, but paint+camera
    // must commit in one call stack. F2 RAF is a no-op (already painted on F1).
    if (intent.phase === 'F2') return;

    ChartAdapter.applyFullData('live', storeData, { skipAnnotations: true });
    this._applyAnnotations(storeData);
    this._applyDdrPlots(snapshot);
    const nav = this._getNavigatorResult();
    if (nav) {
      ChartAdapter.setNavigatorOverlay('live', { navigators: nav }, storeData.candles, {
        context: 'live',
        updateLoadedCandles: false,
      });
    }
    this._navigateAfterPaint(intent, snapshot);
  }

  /**
   * LEFT-prepend camera plan (pure). Mode A when tip stable OR left edge survives
   * right-prune in index space; Mode B only when shifted left is past new tip.
   * @param {object} intent
   * @param {number} [timesLength]
   * @returns {{
   *   mode: 'logical'|'market'|'none',
   *   shift: number,
   *   expectedRange: { from: number, to: number }|null,
   *   rightBoundaryChanged: boolean,
   *   leftSurvives: boolean,
   * }}
   */
  static planLeftPrependRestore(intent, timesLength = 0) {
    const shiftRaw = Number(intent?.prependedCount);
    const shift = (Number.isFinite(shiftRaw) && shiftRaw > 0)
      ? shiftRaw
      : Number(intent?.addedBars);
    const base = intent?.viewportRange;
    const leftEdge = intent?.edge !== 'right';
    const leftLogicalOk = leftEdge
      && Number.isFinite(shift) && shift > 0
      && base
      && Number.isFinite(base.from) && Number.isFinite(base.to)
      && base.to > base.from;
    const tipBeforeRaw = intent?.tipBefore;
    const tipAfterRaw = intent?.tipAfter;
    const tipBefore = tipBeforeRaw == null ? NaN : Number(tipBeforeRaw);
    const tipAfter = tipAfterRaw == null ? NaN : Number(tipAfterRaw);
    const rightBoundaryChanged = intent?.rightBoundaryChanged === true
      || (Number.isFinite(tipBefore) && Number.isFinite(tipAfter) && tipBefore !== tipAfter);
    const tipLogicalAfter = Number.isFinite(Number(intent?.storeAfter))
      ? Number(intent.storeAfter) - 1
      : (Number.isFinite(timesLength) && timesLength > 0 ? timesLength - 1 : NaN);
    const leftSurvives = leftLogicalOk
      && Number.isFinite(tipLogicalAfter)
      && (base.from + shift) <= tipLogicalAfter;
    const useLogicalMode = leftLogicalOk && (!rightBoundaryChanged || leftSurvives);
    const useMarketMode = leftEdge
      && rightBoundaryChanged
      && !useLogicalMode
      && intent?.viewportAnchor
      && Number.isFinite(Number(intent.viewportAnchor.anchorTimeMs))
      && Number.isFinite(Number(intent.viewportAnchor.rightTimeMs));
    return {
      mode: useLogicalMode ? 'logical' : (useMarketMode ? 'market' : 'none'),
      shift: Number.isFinite(shift) ? shift : NaN,
      expectedRange: useLogicalMode
        ? { from: base.from + shift, to: base.to + shift }
        : null,
      rightBoundaryChanged,
      leftSurvives: !!leftSurvives,
    };
  }

  _flushPrepend(storeData, snapshot, intent) {
    // Shot 7 atomic frame: setData → camera restore → DDR (F2 no-op).
    if (intent.phase === 'F2') return;

    if (intent?._edgeHydrate && typeof EdgeHydrateAudit !== 'undefined') {
      EdgeHydrateAudit.markPaintStart(intent._edgeHydrate);
    }

    const times = Array.isArray(snapshot?.times) ? snapshot.times : [];

    // LEFT: market-time preserve only (not Mode A / prependedCount).
    // Mid-window (#13): LWC often already correct — resolve is idempotent on times.
    // Left boundary: LWC freezes logical indices → data slides under → infinite hydrate.
    // Capture is intent.viewportAnchor (pre-merge); resolve against FINAL times; one force.
    const leftEdge = intent?.edge !== 'right';
    const base = intent?.viewportRange;

    // TEMP — Mute & Sync diagnostic (evidence only; no new preserve algorithm).
    const diagOn = typeof LeftPrependDiag !== 'undefined'
      && LeftPrependDiag.enabled()
      && leftEdge;
    if (diagOn) {
      LeftPrependDiag.install();
      const logicalBefore = (typeof LeftPrependDiag.cloneRange === 'function'
        ? LeftPrependDiag.cloneRange(base)
        : null)
        || (typeof LeftPrependDiag.lwcLogical === 'function' ? LeftPrependDiag.lwcLogical() : null)
        || (base ? { from: base.from, to: base.to } : null);
      LeftPrependDiag.begin({
        logicalBefore,
        prependedCount: Number.isFinite(Number(intent?.prependedCount))
          ? Number(intent.prependedCount)
          : null,
        prunedRightCount: intent?.prunedRightCount ?? 0,
        tipBefore: intent?.tipBefore ?? null,
        tipAfter: intent?.tipAfter ?? null,
        storeBefore: intent?.storeBefore ?? null,
        storeAfter: intent?.storeAfter ?? null,
      });
      LeftPrependDiag.mute();
      LeftPrependDiag.setPhase('before');
    }

    ChartAdapter.applyFullData('live', storeData, {
      skipAnnotations: true,
      skipDecoration: true,
    });
    if (typeof PrependViewAudit !== 'undefined' && PrependViewAudit.isActive()) {
      PrependViewAudit.markPhase('afterSetData');
    }

    this._observeShadowWorld(snapshot);
    this._bindDataResolve(times);

    if (leftEdge) {
      this._applyMarketTimeViewportSync(snapshot, intent?.viewportAnchor ?? null);
    } else {
      this._publishPrependViewportFacts(snapshot, intent?.viewportAnchor ?? null, intent);
    }

    this._applyAnnotations(storeData);
    this._applyDdrPlots(snapshot);
    const nav = this._getNavigatorResult();
    if (nav) {
      ChartAdapter.setNavigatorOverlay('live', { navigators: nav }, storeData.candles, {
        context: 'live',
        updateLoadedCandles: false,
      });
    }
  }

  /**
   * LEFT prepend — one sync market-time restore.
   * saved [leftTime, rightTime] → nearest indices in FINAL contiguous times → forceVisibleLogicalRange.
   * No prependedCount, rightOffset, prunedRight, or rAF.
   */
  _applyMarketTimeViewportSync(paintSnapshot, viewportAnchor) {
    if (!viewportAnchor || viewportAnchor.anchorTimeMs == null
      || !Number.isFinite(viewportAnchor.anchorTimeMs)
      || !Number.isFinite(viewportAnchor.rightTimeMs)) {
      return false;
    }
    const times = paintSnapshot?.times;
    if (!Array.isArray(times) || !times.length) return false;
    if (typeof TimeCamera === 'undefined'
      || typeof TimeCamera.resolveMarketTimePreserve !== 'function') {
      return false;
    }
    this._bindDataResolve(times);
    const saved = {
      leftTimeMs: viewportAnchor.anchorTimeMs,
      rightTimeMs: viewportAnchor.rightTimeMs,
      logicalOffset: viewportAnchor.logicalOffset,
      rightLogicalOffset: viewportAnchor.rightLogicalOffset,
    };
    const resolved = TimeCamera.resolveMarketTimePreserve({
      leftTimeMs: saved.leftTimeMs,
      rightTimeMs: saved.rightTimeMs,
      logicalOffset: saved.logicalOffset,
      rightLogicalOffset: saved.rightLogicalOffset,
      tipLogical: times.length - 1,
      timesSec: times,
    });
    if (!resolved || !Number.isFinite(resolved.from) || !Number.isFinite(resolved.to)
      || !(resolved.to > resolved.from)) {
      try {
        console.debug('[LEFT_MT_PRESERVE] resolve-fail', { saved, times0: times[0], timesN: times[times.length - 1] });
      } catch { /* */ }
      return false;
    }
    if (typeof ChartAdapter.forceVisibleLogicalRange !== 'function') return false;
    const ok = ChartAdapter.forceVisibleLogicalRange('live', {
      from: resolved.from,
      to: resolved.to,
    });
    try {
      const finalRange = (typeof ChartAdapter.getVisibleLogicalRange === 'function')
        ? ChartAdapter.getVisibleLogicalRange('live')
        : null;
      console.debug('[LEFT_MT_PRESERVE]', {
        saved,
        resolved: { from: resolved.from, to: resolved.to, caseId: resolved.caseId },
        finalLogical: finalRange,
      });
    } catch { /* */ }
    if (ok && typeof LeftPrependDiag !== 'undefined' && LeftPrependDiag.isActive()) {
      LeftPrependDiag.markAfterForcePin();
    }
    return ok;
  }

  /**
   * @deprecated LEFT uses _applyMarketTimeViewportSync (sync force). Kept for tests.
   */
  _restoreMarketTimeViewport(paintSnapshot, viewportAnchor, intent = null) {
    return this._applyMarketTimeViewportSync(paintSnapshot, viewportAnchor);
  }

  /**
   * RIGHT-append: market-time ViewportAnchor restore via TimeCamera.propose.
   * LEFT uses _applyMarketTimeViewportSync (one forceVisibleLogicalRange).
   */
  _publishPrependViewportFacts(paintSnapshot, viewportAnchor, intent = null) {
    if (typeof TimeCamera === 'undefined' || typeof TimeCamera.proposePreserveViewport !== 'function') {
      return;
    }
    if (!viewportAnchor || viewportAnchor.anchorTimeMs == null
      || !Number.isFinite(viewportAnchor.anchorTimeMs)) {
      return;
    }
    const times = paintSnapshot?.times;
    if (!Array.isArray(times) || !times.length) return;

    TimeCamera.proposePreserveViewport({
      anchorTimeMs: viewportAnchor.anchorTimeMs,
      rightTimeMs: viewportAnchor.rightTimeMs,
      logicalOffset: viewportAnchor.logicalOffset,
      rightLogicalOffset: viewportAnchor.rightLogicalOffset,
      visibleBars: viewportAnchor.visibleBars,
      tipLogical: times.length - 1,
      timesSec: times,
      force: true,
      edge: intent?.edge || 'left',
    });
  }

  /**
   * ADR-028 — publish tipLogical + seriesTimes (observation).
   * @param {object} snapshot painted snapshot (store indices ≡ LWC indices)
   */
  _observeShadowWorld(snapshot) {
    if (typeof TimeCamera === 'undefined' || typeof TimeCamera.observeCommittedWorld !== 'function') {
      return;
    }
    const times = Array.isArray(snapshot?.times) ? snapshot.times : null;
    if (!times || !times.length) return;
    TimeCamera.observeCommittedWorld({
      tipLogical: times.length - 1,
      timesSec: times,
    });
  }

  /**
   * Bind DataResolve for this painted series (compositor owns time→logical).
   * @param {number[]} timesSec
   */
  _bindDataResolve(timesSec) {
    if (typeof TimeCamera === 'undefined' || typeof TimeCamera.bindDataResolve !== 'function') return;
    if (!Array.isArray(timesSec) || !timesSec.length) {
      TimeCamera.bindDataResolve(null);
      return;
    }
    TimeCamera.bindDataResolve({
      nearestLogicalForTime: (centerTimeMs) => ChartCompositor.findIndexByTimeMs(timesSec, centerTimeMs),
    });
  }

  /**
   * ADR-028/029 D2 — observe → TimeCamera.propose → CameraCommit.
   * @param {object} intent
   * @param {object} snapshot
   */
  _navigateAfterPaint(intent, snapshot) {
    const times = Array.isArray(snapshot?.times) ? snapshot.times : [];
    if (!times.length || typeof TimeCamera === 'undefined') return;

    this._observeShadowWorld(snapshot);
    this._bindDataResolve(times);
    const tipLogical = times.length - 1;

    const viewport = intent?.viewport;
    const anchor = intent?.anchor;
    // HISTORY TF switch: centerTime is sacred — never let FreshLive own the VIEW.
    const historyTfRestore = viewport === 'restore'
      && !!anchor
      && Number.isFinite(Number(anchor.centerTimeMs))
      && (anchor.intent === 'HISTORY' || anchor.isAtRightEdge === false);

    const runPropose = () => {
      this._observeShadowWorld(snapshot);
      this._bindDataResolve(times);

      // ADR-014: preserve = no navigation write after paint.
      if (viewport === 'preserve') return;

      if (viewport === 'fresh'
        || viewport == null
        || !(anchor && anchor.centerTimeMs != null)) {
        if (historyTfRestore) return;
        TimeCamera.proposeFreshLive({ tipLogical });
        return;
      }

      const isHistory = anchor.intent === 'HISTORY' || anchor.isAtRightEdge === false;
      const seed = {
        intent: isHistory ? 'HISTORY' : 'LIVE',
        _liveEdge: !isHistory,
        centerTime: anchor.centerTimeMs,
        visibleBars: anchor.visibleBars,
        barSpacing: anchor.barSpacing,
        rightPadding: Number.isFinite(anchor.rightPadding)
          ? anchor.rightPadding
          : anchor.rightOffset,
      };
      const ok = TimeCamera.proposeAfterData({
        tipLogical,
        timesSec: times,
        seed,
        mode: 'switch',
      });
      // Failed HISTORY restore must not fall back to FreshLive (May → August jump).
      if (!ok && historyTfRestore) return;
    };

    // Debt #80: defer propose until host has layout; still via TimeCamera (no raw LWC).
    if (typeof ViewportManager !== 'undefined'
      && ViewportManager.hostHasLayout
      && !ViewportManager.hostHasLayout('live')
      && ViewportManager.whenHostHasLayout) {
      // Spacing-only cold commit — skip for HISTORY TF restore (would steal VIEW to tip).
      if (!historyTfRestore) {
        TimeCamera.proposeFreshLive({});
      }
      ViewportManager.whenHostHasLayout('live', runPropose);
      return;
    }

    runPropose();
  }

  /**
   * Capture ViewportAnchor from store times + committed logical VIEW (before prepend).
   * Store time is identity; logicalOffset may be negative (intentional left void).
   * Captures BOTH edges as market-time so post-setData restore does not depend on
   * array-length delta or LWC's auto logical shift.
   * @param {number[]} timesSec
   * @param {{ from: number, to: number }|null|undefined} range
   * @returns {{
   *   anchorTimeMs: number,
   *   rightTimeMs: number,
   *   logicalOffset: number,
   *   rightLogicalOffset: number,
   *   visibleBars: number,
   * }|null}
   */
  static captureViewportAnchor(timesSec, range) {
    if (!Array.isArray(timesSec) || !timesSec.length) return null;
    if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to)
      || !(range.to > range.from)) {
      return null;
    }
    const n = timesSec.length;
    const toMs = (sec) => {
      const s = Number(sec);
      if (!Number.isFinite(s)) return null;
      return s > 1e12 ? Math.floor(s) : Math.floor(s * 1000);
    };
    const clampedLeft = Math.max(0, Math.min(n - 1, Math.floor(range.from)));
    const clampedRight = Math.max(0, Math.min(n - 1, Math.floor(range.to)));
    const leftMs = toMs(timesSec[clampedLeft]);
    const rightMs = toMs(timesSec[clampedRight]);
    if (leftMs == null || rightMs == null) return null;
    return {
      anchorTimeMs: leftMs,
      rightTimeMs: rightMs,
      logicalOffset: range.from - clampedLeft,
      rightLogicalOffset: range.to - clampedRight,
      visibleBars: range.to - range.from,
    };
  }

  /** Nearest index in ascending unix-seconds (or ms) array for target unix-ms. O(log n). */
  static findIndexByTimeMs(timesSec, timeMs) {
    if (!timesSec?.length || timeMs == null || !Number.isFinite(timeMs)) return 0;
    const first = Number(timesSec[0]);
    const targetSec = first > 1e12 ? timeMs : timeMs / 1000;
    let lo = 0;
    let hi = timesSec.length - 1;
    while (lo < hi) {
      const mid = (lo + hi) >> 1;
      if (Number(timesSec[mid]) < targetSec) lo = mid + 1;
      else hi = mid;
    }
    if (lo > 0) {
      const prevDelta = Math.abs(Number(timesSec[lo - 1]) - targetSec);
      const currDelta = Math.abs(Number(timesSec[lo]) - targetSec);
      if (prevDelta < currDelta) return lo - 1;
    }
    return lo;
  }

  /** @param {{ from: number, to: number }|null|undefined} range */
  static _isFiniteLogicalRange(range) {
    return range
      && Number.isFinite(range.from)
      && Number.isFinite(range.to)
      && range.to > range.from;
  }

  _applyDdrPlots(snapshot) {
    if (typeof window === 'undefined' || !window.DDRFactory?.cutoverActive) return;
    window.DDRFactory.hydrateFromColumnar({
      times: snapshot.times,
      plots: snapshot.plots,
      sentinel: typeof DDRFactory !== 'undefined' ? DDRFactory.HISTORY_ABSENT : undefined,
    });
    window.DDRFactory.applyHydratedData();
  }

  _applyAnnotations(storeData) {
    if (typeof ChartAdapter === 'undefined' || typeof ChartAdapter.applyLiveAnnotationLayer !== 'function') {
      return;
    }
    const showPivots = (typeof rsxShowPivotsFrom === 'function' && typeof RsxController !== 'undefined')
      ? rsxShowPivotsFrom(RsxController.getSettings('live'), true)
      : true;
    ChartAdapter.applyLiveAnnotationLayer(storeData, { showPivots });
  }

  static snapshotToStoreData(snapshot, annotationMap) {
    const columnar = {
      times: snapshot.times,
      candles: snapshot.candles,
    };
    const candles = typeof columnarToCandles === 'function'
      ? columnarToCandles(columnar)
      : [];
    const map = annotationMap instanceof Map
      ? annotationMap
      : ChartCompositor._annotationMapFromList(snapshot.annotations);
    return {
      candles,
      osc: [],
      annotations: snapshot.annotations || [],
      annotationMap: map,
    };
  }

  static _annotationMapFromList(annotations) {
    const map = new Map();
    if (!Array.isArray(annotations)) return map;
    for (const ann of annotations) {
      const raw = ann?.time ?? ann?.Time;
      const n = Number(raw);
      if (!Number.isFinite(n)) continue;
      const ms = n > 1e12 ? Math.floor(n) : Math.floor(n * 1000);
      map.set(ms, { ...ann, timeMs: ms });
    }
    return map;
  }
}

if (typeof window !== 'undefined') {
  window.ChartCompositor = ChartCompositor;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { ChartCompositor };
}
