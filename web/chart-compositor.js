/**
 * ChartCompositor — sole live-chart paint authority (Core 3.0 Step 2).
 * Reads ColumnarStore snapshots; writes to ChartAdapter only.
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
   * @param {{ mode: 'full'|'prepend'|'delta', addedBars?: number, viewport?: string, viewportRange?: object|null, anchor?: object, tick?: object, delta?: object }} intent
   */
  flush(intent) {
    if (!this._store || !this._shouldPaint()) return;
    if (typeof ChartAdapter === 'undefined') return;

    if (intent.mode === 'delta') {
      this._flushDelta(intent);
      return;
    }

    if (!this._store.invariantOk()) {
      console.error('[ChartCompositor] invariant failed — skip paint', this._store.invariantMeta());
      return;
    }

    const snapshot = this._store.snapshot();
    const storeData = ChartCompositor.snapshotToStoreData(
      snapshot,
      this._store.getAnnotationsMap(),
    );

    ChartAdapter.setLiveUpdating(true);
    try {
      if (intent.mode === 'prepend') {
        this._flushPrepend(storeData, snapshot, intent);
      } else {
        this._flushFull(storeData, snapshot, intent);
      }
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
    ChartAdapter.applyFullData('live', storeData);
    this._applyDdrPlots(snapshot);

    const nav = this._getNavigatorResult();
    if (nav) {
      ChartAdapter.setNavigatorOverlay('live', { navigators: nav }, storeData.candles, {
        context: 'live',
        updateLoadedCandles: false,
      });
    }

    if (intent.anchor && typeof ViewportManager !== 'undefined') {
      ViewportManager.restore('live', intent.anchor, this._store);
      return;
    }

    if (intent.viewport === 'fresh' || intent.viewport == null) {
      const charts = [
        ChartAdapter.getChart('live', 'price'),
        ChartAdapter.getChart('live', 'osc'),
        ChartAdapter.getChart('live', 'rsx'),
      ];
      charts.forEach((chart) => {
        chart?.timeScale()?.scrollToPosition(0, false);
      });
    }
  }

  _flushPrepend(storeData, snapshot, intent) {
    const chart = ChartAdapter.getChart('live', 'price');
    const ts = chart?.timeScale();
    const prevRange = intent.viewportRange != null
      ? intent.viewportRange
      : ChartAdapter.getVisibleLogicalRange('live');

    ChartAdapter.applyFullData('live', storeData);
    this._applyDdrPlots(snapshot);

    const addedBars = Number(intent.addedBars) || 0;
    if (
      ts
      && prevRange != null
      && Number.isFinite(prevRange.from)
      && Number.isFinite(prevRange.to)
      && addedBars > 0
    ) {
      ts.setVisibleLogicalRange({
        from: prevRange.from + addedBars,
        to: prevRange.to + addedBars,
      });
    }

    const nav = this._getNavigatorResult();
    if (nav) {
      ChartAdapter.setNavigatorOverlay('live', { navigators: nav }, storeData.candles, {
        context: 'live',
        updateLoadedCandles: false,
      });
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
