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
    /** @type {boolean} */
    this._prependCameraPending = false;
    /** @type {Function[]} */
    this._prependCameraSettledCbs = [];
    /** @type {(() => void)|null} */
    this._invalidateQueuedDeltas = null;
    this._paintedAnnotationRevision = -1;
    this._paintedVisibilityMask = null;
    this._paintedMarkerSeries = null;
  }

  /**
   * PAINT-ORDER-1: scheduler binds this so F1 snapshot can drop stale pending deltas.
   * @param {() => void} fn
   */
  bindQueuedDeltaInvalidator(fn) {
    this._invalidateQueuedDeltas = typeof fn === 'function' ? fn : null;
  }

  /** True while prepend settle callbacks are waiting (RenderScheduler serialization). */
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

    // F2: paint+camera already committed on F1. Do not snapshot / columnarToCandles again.
    if (intent.phase === 'F2') {
      this._settleAfterFullPrependPaint(intent);
      return;
    }

    const snapshot = this._store.snapshot();
    // F1 full/prepend: S already contains ticks queued during the RAF wait.
    // Drop only those deltas; a queued FULL/PREPEND must survive.
    if (typeof this._invalidateQueuedDeltas === 'function') {
      this._invalidateQueuedDeltas();
    }
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
      this._settleAfterFullPrependPaint(intent);
    }
  }

  /**
   * Post-paint settle for full/prepend (F1 after paint, F2 without rebuilding store data).
   * Decoration / onAfterFlush / prepend-camera notify — not a second setData.
   */
  _settleAfterFullPrependPaint(intent) {
    if (intent.mode === 'prepend') {
      if (typeof ChartAdapter !== 'undefined'
        && typeof ChartAdapter.refreshLiveDecoration === 'function') {
        ChartAdapter.refreshLiveDecoration();
      }
      this._notifyPrependCameraSettled();
    }
    if (this._onAfterFlush) this._onAfterFlush(intent);
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
      if (this._annotationPaintNeeded()) {
        const storeData = ChartCompositor.snapshotToStoreData(snapshot);
        this._applyAnnotations(storeData);
      }
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
      const applied = [];
      for (let i = 0; i < chain.length; i++) {
        const delta = chain[i];
        if (!delta?.candle) continue;
        const payload = {
          ...delta,
          barCount: delta.barCount ?? this._store.barCount(),
        };
        const ok = ChartAdapter.applyDelta('live', payload);
        if (ok === false) continue;
        applied.push(delta);
        const tick = ticks[i] ?? (i === chain.length - 1 ? fallbackTick : null);
        if (tick?.plots && typeof window !== 'undefined' && window.DDRFactory?.cutoverActive) {
          window.DDRFactory.updateTick(tick.time, tick.plots);
        }
      }
      // Tip may advance on new bars — observe without cloning OHLC/plots/annotations.
      const n = typeof this._store.barCount === 'function' ? this._store.barCount() : 0;
      const timesSec = typeof this._store.timesSec === 'function' ? this._store.timesSec() : null;
      if (n > 0 && Array.isArray(timesSec) && timesSec.length) {
        this._observeShadowWorld({
          times: timesSec,
          tipLogical: n - 1,
        });
      }
      this._maybeProposeLiveEdgeGuard(applied, n);
    } finally {
      ChartAdapter.setLiveUpdating(false);
      if (this._annotationPaintNeeded() && typeof this._store.getForLightweightCharts === 'function') {
        this._applyAnnotations(this._store.getForLightweightCharts());
      }
      if (this._onAfterFlush) this._onAfterFlush(intent);
    }
  }

  /**
   * LIVE-EDGE-1: after a new bar on LIVE, one TimeCamera shift if slack < 1.
   * Same-bar / HISTORY: no-op. No TF-class gate.
   */
  _maybeProposeLiveEdgeGuard(chain, barCount) {
    let sawNewBar = false;
    for (let i = 0; i < chain.length; i++) {
      if (chain[i]?.isNewBar === true) {
        sawNewBar = true;
        break;
      }
    }
    if (!sawNewBar) return;
    if (typeof TimeCamera === 'undefined' || typeof TimeCamera.proposeLiveEdgeGuard !== 'function') {
      return;
    }
    const tipLogical = Number.isFinite(barCount) && barCount > 0 ? barCount - 1 : null;
    TimeCamera.proposeLiveEdgeGuard({ tipLogical });
  }

  _flushFull(storeData, snapshot, intent) {
    // Shot 7 atomic frame: Scheduler may still split F1/F2 RAF, but paint+camera
    // must commit in one call stack. F2 RAF is a no-op (already painted on F1).
    if (intent.phase === 'F2') return;

    ChartAdapter.applyFullData('live', storeData, { skipAnnotations: true });
    this._applyDdrPlots(snapshot);
    this._applyAnnotations(storeData);
    const nav = this._getNavigatorResult();
    if (nav) {
      ChartAdapter.setNavigatorOverlay('live', { navigators: nav }, storeData.candles, {
        context: 'live',
        updateLoadedCandles: false,
      });
    }
    this._navigateAfterPaint(intent, snapshot);
  }

  _flushPrepend(storeData, snapshot, intent) {
    // Shot 7 atomic frame: setData → market-time restore (LEFT) → DDR (F2 no-op).
    if (intent.phase === 'F2') return;

    const times = Array.isArray(snapshot?.times) ? snapshot.times : [];
    const leftEdge = intent?.edge !== 'right';

    ChartAdapter.applyFullData('live', storeData, {
      skipAnnotations: true,
      skipDecoration: true,
    });

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
    const resolved = TimeCamera.resolveMarketTimePreserve({
      leftTimeMs: viewportAnchor.anchorTimeMs,
      rightTimeMs: viewportAnchor.rightTimeMs,
      logicalOffset: viewportAnchor.logicalOffset,
      rightLogicalOffset: viewportAnchor.rightLogicalOffset,
      tipLogical: times.length - 1,
      timesSec: times,
    });
    if (!resolved || !Number.isFinite(resolved.from) || !Number.isFinite(resolved.to)
      || !(resolved.to > resolved.from)) {
      return false;
    }
    if (typeof ChartAdapter.forceVisibleLogicalRange !== 'function') return false;
    return ChartAdapter.forceVisibleLogicalRange('live', {
      from: resolved.from,
      to: resolved.to,
    });
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
   * @param {{ times?: number[], tipLogical?: number }} snapshot
   */
  _observeShadowWorld(snapshot) {
    if (typeof TimeCamera === 'undefined' || typeof TimeCamera.observeCommittedWorld !== 'function') {
      return;
    }
    const times = Array.isArray(snapshot?.times) ? snapshot.times : null;
    if (!times || !times.length) return;
    const tipRaw = Number(snapshot?.tipLogical);
    const tipLogical = Number.isFinite(tipRaw) ? tipRaw : times.length - 1;
    TimeCamera.observeCommittedWorld({
      tipLogical,
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
    // HISTORY TF: centerTime sacred. LIVE TF: keep bar count / spacing / pad — no FreshLive.
    const historyTfRestore = viewport === 'restore'
      && !!anchor
      && Number.isFinite(Number(anchor.centerTimeMs))
      && (anchor.intent === 'HISTORY' || anchor.isAtRightEdge === false);
    const liveTfRestore = viewport === 'restore'
      && !!anchor
      && (anchor.intent === 'LIVE' || anchor.isAtRightEdge === true)
      && Number.isFinite(Number(anchor.visibleBars))
      && Number(anchor.visibleBars) > 0;

    const runPropose = () => {
      this._observeShadowWorld(snapshot);
      this._bindDataResolve(times);

      // ADR-014: preserve = no navigation write after paint.
      if (viewport === 'preserve') return;

      if (viewport === 'fresh'
        || viewport == null
        || !(anchor && (historyTfRestore || liveTfRestore || anchor.centerTimeMs != null))) {
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
      if (!historyTfRestore && !liveTfRestore) {
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
    const clampedLeft = Math.max(0, Math.min(n - 1, Math.floor(range.from)));
    const clampedRight = Math.max(0, Math.min(n - 1, Math.floor(range.to)));
    const leftMs = ChartCompositor._chartSecToMs(timesSec[clampedLeft]);
    const rightMs = ChartCompositor._chartSecToMs(timesSec[clampedRight]);
    if (leftMs == null || rightMs == null) return null;
    return {
      anchorTimeMs: leftMs,
      rightTimeMs: rightMs,
      logicalOffset: range.from - clampedLeft,
      rightLogicalOffset: range.to - clampedRight,
      visibleBars: range.to - range.from,
    };
  }

  /** Nearest index in ascending unix-seconds array for target unix-ms. O(log n). */
  static findIndexByTimeMs(timesSec, timeMs) {
    if (!timesSec?.length || timeMs == null || !Number.isFinite(timeMs)) return 0;
    const CDS = typeof ChartDataStore !== 'undefined' ? ChartDataStore : globalThis.ChartDataStore;
    const targetSec = CDS.msToChartSec(timeMs);
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

  _visibilityMask() {
    if (typeof rsxVisibilityMask === 'function' && typeof RsxController !== 'undefined') {
      return rsxVisibilityMask(RsxController.getSettings('live'));
    }
    return 31;
  }

  _markerSeries() {
    if (typeof window === 'undefined' || typeof window.DDRFactory?.getSeries !== 'function') {
      return null;
    }
    return window.DDRFactory.getSeries('line_rsx') || null;
  }

  invalidateAnnotationPaintTarget() {
    this._paintedMarkerSeries = null;
  }

  _annotationPaintNeeded() {
    const mask = this._visibilityMask();
    const series = this._markerSeries();
    const rev = (typeof this._store?.annotationRevision === 'function')
      ? this._store.annotationRevision()
      : -1;
    return !(
      series === this._paintedMarkerSeries
      && rev === this._paintedAnnotationRevision
      && mask === this._paintedVisibilityMask
    );
  }

  _rememberAnnotationPaint() {
    this._paintedAnnotationRevision = (typeof this._store?.annotationRevision === 'function')
      ? this._store.annotationRevision()
      : -1;
    this._paintedVisibilityMask = this._visibilityMask();
    this._paintedMarkerSeries = this._markerSeries();
  }

  _applyAnnotations(storeData) {
    if (typeof ChartAdapter === 'undefined' || typeof ChartAdapter.applyLiveAnnotationLayer !== 'function') {
      return;
    }
    const visibilityMask = this._visibilityMask();
    ChartAdapter.applyLiveAnnotationLayer(storeData, { visibilityMask });
    this._rememberAnnotationPaint();
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

  /** Known chart seconds → geometry ms. Delegates to ChartDataStore.secToMs. */
  static _chartSecToMs(sec) {
    const s = Number(sec);
    if (!Number.isFinite(s)) return null;
    const CDS = typeof ChartDataStore !== 'undefined' ? ChartDataStore : globalThis.ChartDataStore;
    return CDS.secToMs(s);
  }

  static _annotationMapFromList(annotations) {
    const map = new Map();
    if (!Array.isArray(annotations)) return map;
    for (const ann of annotations) {
      const raw = ann?.time ?? ann?.Time;
      const ms = ChartCompositor._chartSecToMs(raw);
      if (ms == null) continue;
      map.set(ms, { ...ann, timeMs: ms });
    }
    return map;
  }
}

if (typeof window !== 'undefined') {
  window.ChartCompositor = ChartCompositor;
}

if (typeof module !== 'undefined' && module.exports) {
  if (typeof globalThis.ChartDataStore === 'undefined') {
    globalThis.ChartDataStore = require('./store.js').ChartDataStore;
  }
  module.exports = { ChartCompositor };
}
