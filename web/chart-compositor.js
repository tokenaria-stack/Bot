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
      if (this._onAfterFlush) this._onAfterFlush(intent);
    }
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

  _flushPrepend(storeData, snapshot, intent) {
    // Shot 7 atomic frame: capture → setData → DDR → camera in one stack (F2 no-op).
    if (intent.phase === 'F2') return;

    const prependAnchor = ChartCompositor._captureLeftEdgeAnchor();
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
    // Wave 1: publish facts only — TimeCamera decides preserve VIEW (no Compositor nav policy).
    this._observeShadowWorld(snapshot);
    this._bindDataResolve(Array.isArray(snapshot?.times) ? snapshot.times : []);
    this._publishPrependViewportFacts(snapshot, prependAnchor);
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

    const runPropose = () => {
      this._observeShadowWorld(snapshot);
      this._bindDataResolve(times);
      const viewport = intent?.viewport;
      const anchor = intent?.anchor;

      // ADR-014: preserve = no navigation write after paint.
      if (viewport === 'preserve') return;

      if (viewport === 'fresh'
        || viewport == null
        || !(anchor && anchor.centerTimeMs != null)) {
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
      TimeCamera.proposeAfterData({
        tipLogical,
        timesSec: times,
        seed,
        mode: 'switch',
      });
    };

    // Debt #80: defer propose until host has layout; still via TimeCamera (no raw LWC).
    if (typeof ViewportManager !== 'undefined'
      && ViewportManager.hostHasLayout
      && !ViewportManager.hostHasLayout('live')
      && ViewportManager.whenHostHasLayout) {
      // Spacing-only cold commit so LWC has healthy defaults before layout.
      TimeCamera.proposeFreshLive({});
      ViewportManager.whenHostHasLayout('live', runPropose);
      return;
    }

    runPropose();
  }

  /**
   * Wave 1 — prepend camera facts for TimeCamera (translator only).
   * Captures left-edge time + visibleBars before setData; does not choose VIEW.
   * @param {object} paintSnapshot
   * @param {{ leftTimeMs: number, visibleBars: number }|null} anchor
   */
  _publishPrependViewportFacts(paintSnapshot, anchor) {
    if (!anchor || anchor.leftTimeMs == null || !Number.isFinite(anchor.leftTimeMs)) return;
    if (typeof TimeCamera === 'undefined' || typeof TimeCamera.proposePreserveViewport !== 'function') {
      return;
    }
    const times = paintSnapshot?.times;
    if (!Array.isArray(times) || !times.length) return;
    TimeCamera.proposePreserveViewport({
      leftTimeMs: anchor.leftTimeMs,
      visibleBars: anchor.visibleBars,
      tipLogical: times.length - 1,
      timesSec: times,
    });
  }

  /**
   * Left visible bar time from live LWC *before* setData (logical → pixel → time).
   * @returns {{ leftTimeMs: number, visibleBars: number }|null}
   */
  static _captureLeftEdgeAnchor() {
    const chart = typeof ChartAdapter !== 'undefined'
      ? ChartAdapter.getChart('live', 'price')
      : null;
    const ts = chart?.timeScale?.();
    if (!ts) return null;
    const range = ts.getVisibleLogicalRange();
    if (!ChartCompositor._isFiniteLogicalRange(range)) return null;

    const fromFloor = Math.floor(range.from);
    const coord = typeof ts.logicalToCoordinate === 'function'
      ? ts.logicalToCoordinate(fromFloor)
      : null;
    if (coord == null || !Number.isFinite(coord) || typeof ts.coordinateToTime !== 'function') {
      return null;
    }
    const time = ts.coordinateToTime(coord);
    const leftTimeMs = ChartCompositor._timeLikeToMs(time);
    if (leftTimeMs == null) return null;
    return {
      leftTimeMs,
      visibleBars: range.to - range.from,
    };
  }

  /** @param {unknown} time */
  static _timeLikeToMs(time) {
    if (time == null) return null;
    if (typeof time === 'object' && time.timestamp != null) {
      const n = Number(time.timestamp);
      return Number.isFinite(n) ? (n < 1e12 ? Math.floor(n * 1000) : Math.floor(n)) : null;
    }
    const n = Number(time);
    if (!Number.isFinite(n) || n <= 0) return null;
    return n < 1e12 ? Math.floor(n * 1000) : Math.floor(n);
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
