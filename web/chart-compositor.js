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
    if (!this._store || !this._shouldPaint()) return;
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
    const phase = intent.phase;
    const runF1 = phase === 'F1' || !phase;
    const runF2 = phase === 'F2' || !phase;

    if (runF1) {
      ChartAdapter.applyFullData('live', storeData, { skipAnnotations: true });
      this._applyAnnotations(storeData);

      if (intent.anchor && typeof ViewportManager !== 'undefined') {
        ViewportManager.restore('live', intent.anchor, this._store);
      } else if (intent.viewport === 'fresh' || intent.viewport == null) {
        ChartAdapter.getChart('live', 'price')?.timeScale()?.fitContent();
      }
    }

    if (runF2) {
      this._applyDdrPlots(snapshot);
      const nav = this._getNavigatorResult();
      if (nav) {
        ChartAdapter.setNavigatorOverlay('live', { navigators: nav }, storeData.candles, {
          context: 'live',
          updateLoadedCandles: false,
        });
      }
    }
  }

  _flushPrepend(storeData, snapshot, intent) {
    const phase = intent.phase;
    const runF1 = phase === 'F1' || !phase;
    const runF2 = phase === 'F2' || !phase;

    if (runF1) {
      const prevRange = ChartAdapter.getVisibleLogicalRange('live');
      ChartAdapter.applyFullData('live', storeData, { skipAnnotations: true });
      this._applyAnnotations(storeData);

      const addedBars = Number(intent.addedBars) || 0;
      if (addedBars > 0 && typeof ChartAdapter.shiftCamera === 'function') {
        // Prefer pre-setData range: after setData LWC may reset to the right edge.
        ChartAdapter.shiftCamera('live', addedBars, prevRange);
      }
    }

    if (runF2) {
      this._applyDdrPlots(snapshot);
      const nav = this._getNavigatorResult();
      if (nav) {
        ChartAdapter.setNavigatorOverlay('live', { navigators: nav }, storeData.candles, {
          context: 'live',
          updateLoadedCandles: false,
        });
      }
    }
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
