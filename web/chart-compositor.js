/**
 * ChartCompositor — sole live-chart paint authority (Core 3.0 Step 2 + Soft Updates).
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

  /**
   * Soft update: DDR plots (+ annotations) only — never setData on price candles.
   */
  _flushIndicators(intent) {
    if (!this._store.invariantOk()) {
      console.error('[ChartCompositor] invariant failed — skip indicators', this._store.invariantMeta());
      return;
    }
    const snapshot = this._store.snapshot();
    ChartAdapter.setLiveUpdating(true);
    try {
      this._applyDdrPlots(snapshot);
      const storeData = ChartCompositor.snapshotToStoreData(
        snapshot,
        this._store.getAnnotationsMap(),
      );
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

    if (intent.anchor && typeof ViewportManager !== 'undefined') {
      ViewportManager.restore('live', intent.anchor, this._store);
      return;
    }

    if (intent.viewport === 'fresh' || intent.viewport == null) {
      const charts = [
        ChartAdapter.getChart('live', 'price'),
        ChartAdapter.getChart('live', 'wozduh'),
        ChartAdapter.getChart('live', 'rsx'),
      ];
      charts.forEach((chart) => {
        chart?.timeScale()?.scrollToPosition(0, false);
      });
    }
  }

  _flushPrepend(storeData, snapshot, intent) {
    const chart = ChartAdapter.getChart('live', 'price');
    const prevRange = intent.viewportRange != null
      ? intent.viewportRange
      : ChartAdapter.getVisibleLogicalRange('live');

    ChartAdapter.applyFullData('live', storeData, { skipAnnotations: true });
    this._applyDdrPlots(snapshot);
    this._applyAnnotations(storeData);

    const addedBars = Number(intent.addedBars) || 0;
    if (
      prevRange != null
      && Number.isFinite(prevRange.from)
      && Number.isFinite(prevRange.to)
      && addedBars > 0
    ) {
      const range = {
        from: prevRange.from + addedBars,
        to: prevRange.to + addedBars,
      };
      if (typeof ChartAdapter.setVisibleLogicalRange === 'function') {
        ChartAdapter.setVisibleLogicalRange('live', range, { animate: false });
      } else {
        chart?.timeScale()?.setVisibleLogicalRange(range);
      }
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
