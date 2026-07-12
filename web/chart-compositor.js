/**
 * ChartCompositor — sole live-chart paint authority (Core 2.3).
 * Reads ColumnarStore snapshots; paints via Sliding Render Window (bounded buffer).
 * Writes to ChartAdapter only.
 */
class ChartCompositor {
  /** Soft RAM cap for LWC; large enough for left-scroll history, not infinite. */
  static get RENDER_WINDOW_LIMIT() {
    return 15000;
  }

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
   * Synchronous Slice Rule: same tail index range for times, candles.*, and every plots[id].
   * @param {object} snapshot
   * @param {number} [limit=15000]
   * @returns {object}
   */
  static extractWindow(snapshot, limit = 15000) {
    if (!snapshot || !Array.isArray(snapshot.times)) return snapshot;
    const n = snapshot.times.length;
    if (n <= limit) return snapshot;

    const start = n - limit;
    const candlesSrc = snapshot.candles && typeof snapshot.candles === 'object' ? snapshot.candles : {};
    const candles = {};
    for (const key of ['open', 'high', 'low', 'close', 'volume']) {
      const col = candlesSrc[key];
      candles[key] = Array.isArray(col) ? col.slice(start) : [];
    }

    const plotsSrc = snapshot.plots && typeof snapshot.plots === 'object' ? snapshot.plots : {};
    const plots = {};
    for (const [id, col] of Object.entries(plotsSrc)) {
      plots[id] = Array.isArray(col) ? col.slice(start) : [];
    }

    const annotations = Array.isArray(snapshot.annotations)
      ? snapshot.annotations.slice(-limit)
      : [];

    return {
      ...snapshot,
      times: snapshot.times.slice(start),
      candles,
      plots,
      annotations,
    };
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
    const windowedSnapshot = ChartCompositor.extractWindow(
      snapshot,
      ChartCompositor.RENDER_WINDOW_LIMIT,
    );
    const storeData = ChartCompositor.snapshotToStoreData(windowedSnapshot);

    ChartAdapter.setLiveUpdating(true);
    try {
      if (intent.mode === 'prepend') {
        this._flushPrepend(storeData, windowedSnapshot, intent);
      } else {
        this._flushFull(storeData, windowedSnapshot, intent);
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
    const snapshot = ChartCompositor.extractWindow(
      this._store.snapshot(),
      ChartCompositor.RENDER_WINDOW_LIMIT,
    );
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
    const delta = intent.delta;
    if (!delta?.candle) return;

    ChartAdapter.setLiveUpdating(true);
    try {
      if (intent.tick?.plots && typeof window !== 'undefined' && window.DDRFactory?.cutoverActive) {
        window.DDRFactory.updateTick(intent.tick.time, intent.tick.plots);
      }
      ChartAdapter.applyDelta('live', {
        ...delta,
        barCount: delta.barCount ?? this._store.barCount(),
      });
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
    this._commitFullCamera(intent);
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
    this._commitPrependCamera(snapshot, prependAnchor);
  }

  /**
   * Anchor present (TF) → ViewportManager.restore (sanitized barSpacing).
   * Fresh / cold boot → healthy defaults (never fitContent).
   * @param {{ anchor?: object, viewport?: string }} intent
   */
  _commitFullCamera(intent) {
    const anchor = intent?.anchor;
    if (anchor?.centerTimeMs != null && typeof ViewportManager !== 'undefined') {
      ViewportManager.restore('live', anchor, this._store);
      return;
    }
    if (intent?.viewport === 'fresh' || intent?.viewport == null) {
      this._commitFreshCamera();
    }
  }

  /**
   * Cold boot camera: barSpacing 6 on all panes + last ~150 bars (no fitContent).
   */
  _commitFreshCamera() {
    const spacing = (typeof ViewportManager !== 'undefined'
      && Number.isFinite(ViewportManager.HEALTHY_BAR_SPACING))
      ? ViewportManager.HEALTHY_BAR_SPACING
      : 6;
    const visible = (typeof ViewportManager !== 'undefined'
      && Number.isFinite(ViewportManager.HEALTHY_VISIBLE_BARS))
      ? ViewportManager.HEALTHY_VISIBLE_BARS
      : 150;

    ['price', 'wozduh', 'rsx'].forEach((pane) => {
      const chart = typeof ChartAdapter !== 'undefined'
        ? ChartAdapter.getChart('live', pane)
        : null;
      chart?.timeScale()?.applyOptions({ barSpacing: spacing });
    });

    const n = typeof this._store?.barCount === 'function' ? this._store.barCount() : 0;
    if (n > 0 && typeof ChartAdapter.setVisibleLogicalRange === 'function') {
      const from = Math.max(0, n - visible);
      ChartAdapter.setVisibleLogicalRange('live', { from, to: n }, { animate: false });
      return;
    }

    ChartAdapter.getChart('live', 'price')?.timeScale()?.scrollToPosition(0, false);
  }

  /**
   * Re-align camera so the pre-prepend left-edge time stays at logical `from`.
   * Works when extractWindow truncates — index math cannot.
   * @param {object} windowedSnapshot
   * @param {{ leftTimeMs: number, visibleBars: number }|null} anchor
   */
  _commitPrependCamera(windowedSnapshot, anchor) {
    if (!anchor || anchor.leftTimeMs == null || !Number.isFinite(anchor.leftTimeMs)) return;
    const times = windowedSnapshot?.times;
    if (!Array.isArray(times) || !times.length) return;
    if (typeof ChartAdapter.setVisibleLogicalRange !== 'function') return;

    const fromIdx = ChartCompositor.findIndexByTimeMs(times, anchor.leftTimeMs);
    const visibleBars = Number.isFinite(anchor.visibleBars) && anchor.visibleBars > 0
      ? anchor.visibleBars
      : 80;
    const n = times.length;
    let from = fromIdx;
    let to = fromIdx + visibleBars;
    if (to > n) {
      const over = to - n;
      from = Math.max(0, from - over);
      to = n;
    }
    if (from >= to) {
      from = Math.max(0, n - visibleBars);
      to = n;
    }
    ChartAdapter.setVisibleLogicalRange('live', { from, to }, { animate: false });
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
