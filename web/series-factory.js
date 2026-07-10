/**
 * DDRFactory — Data-Driven Rendering (DDR) module (Phase 6 cutover).
 * Manifest-driven series mount + columnar history hydration + live tick updates.
 */
class DDRFactory {
  /** @type {number} Server-side sentinel (wire.HistoryAbsent / math.MaxFloat64). */
  static get HISTORY_ABSENT() {
    return 1.7976931348623157e+308;
  }

  constructor(options = {}) {
    /** @type {Map<string, import('lightweight-charts').ISeriesApi>} */
    this.seriesMap = new Map();
    /** @type {{ panes?: Record<string, object[]> } | null} */
    this.manifest = null;
    /** @type {Map<string, Array<{time: number, value: number}>>} */
    this.hydratedData = new Map();
    this.cutoverActive = false;
    this.normalizeTime = typeof options.normalizeTime === 'function'
      ? options.normalizeTime
      : DDRFactory.defaultNormalizeTime;
  }

  static defaultNormalizeTime(raw) {
    if (typeof chartTime === 'function') {
      return chartTime(raw);
    }
    const t = Number(raw);
    if (!Number.isFinite(t)) return null;
    return t >= 1e12 ? Math.floor(t / 1000) : Math.floor(t);
  }

  async fetchManifest(url = '/api/ui/manifest') {
    const res = await fetch(url);
    if (!res.ok) {
      throw new Error(`DDRFactory: manifest fetch failed (${res.status})`);
    }
    this.manifest = await res.json();
    return this.manifest;
  }

  /**
   * Mounts manifest components onto charts via pane registry.
   *
   * @param {Record<string, { chart: import('lightweight-charts').IChartApi, defaultPriceScaleId?: string }>} chartRegistry
   * @param {Record<string, object[]>} [manifestPanes]
   */
  buildPanes(chartRegistry, manifestPanes) {
    const panes = manifestPanes || this.manifest?.panes;
    if (!chartRegistry || !panes || typeof panes !== 'object') {
      return;
    }
    for (const [paneId, components] of Object.entries(panes)) {
      const entry = chartRegistry[paneId];
      if (!entry?.chart || !Array.isArray(components)) continue;
      for (const component of components) {
        this._mountComponent(entry, component);
      }
    }
    this.cutoverActive = this.seriesMap.size > 0;
  }

  async fetchAndHydrateHistory(symbol, tf, options = {}) {
    const endTimeSec = options.endTimeSec;
    if (!Number.isFinite(endTimeSec) || endTimeSec <= 0) {
      return null;
    }
    const params = new URLSearchParams({
      tf: tf || '1m',
      endTime: String(endTimeSec),
      limit: String(options.limit ?? (typeof HISTORY_CHUNK_LIMIT !== 'undefined' ? HISTORY_CHUNK_LIMIT : 3000)),
      format: 'columnar',
    });
    const slotKeys = options.slots ?? [...this.seriesMap.keys()];
    if (slotKeys.length > 0) {
      params.set('slots', slotKeys.join(','));
    }
    if (symbol) params.set('symbol', symbol);

    const rsx = options.rsxSettings;
    if (rsx) {
      params.set('rsx_length', String(rsx.length ?? 14));
      params.set('rsx_signal_length', String(rsx.signal_length ?? 9));
      params.set('rsx_source', rsx.source ?? 'close');
      params.set('rsx_method', rsx.div_method ?? 'fractal');
      params.set('rsx_pivot_radius', String(rsx.pivot_radius ?? 2));
      params.set('rsx_div_lookback', String(rsx.div_lookback ?? 90));
      params.set('min_price_delta_ratio', String(rsx.min_price_delta_ratio ?? 0));
      params.set('min_osc_delta', String(rsx.min_osc_delta ?? 0));
    }

    const res = await fetch(`/api/history?${params.toString()}`, { cache: 'no-store' });
    if (!res.ok) {
      throw new Error(`DDRFactory: columnar history failed (${res.status})`);
    }
    const data = await res.json();
    return this.hydrateFromColumnar(data);
  }

  /** Populate hydratedData from an in-memory columnar monolith (boot / TF switch). */
  hydrateFromColumnar(data) {
    if (!data) return null;
    const sentinel = Number(data.sentinel ?? DDRFactory.HISTORY_ABSENT);
    const times = Array.isArray(data.times) ? data.times : [];
    const plots = data.plots && typeof data.plots === 'object' ? data.plots : {};

    this.hydratedData.clear();
    for (const [plotId, values] of Object.entries(plots)) {
      if (!Array.isArray(values)) continue;
      this.hydratedData.set(
        plotId,
        DDRFactory.columnToLWC(times, values, sentinel, this.normalizeTime),
      );
    }
    return data;
  }

  applyHydratedData(plotsOverride) {
    const source = plotsOverride && typeof plotsOverride === 'object'
      ? plotsOverride
      : this.hydratedData;
    if (plotsOverride && typeof plotsOverride === 'object') {
      for (const [id, points] of Object.entries(plotsOverride)) {
        if (Array.isArray(points)) this.hydratedData.set(id, points);
      }
    }
    for (const [id, series] of this.seriesMap) {
      const points = source instanceof Map ? source.get(id) : this.hydratedData.get(id);
      if (!points?.length) continue;
      try {
        series.setData(points);
      } catch {
        /* skip invalid setData during cutover */
      }
    }
  }

  static columnToLWC(times, values, sentinel, normalizeTime) {
    const n = Math.min(times.length, values.length);
    if (n === 0) return [];

    const norm = typeof normalizeTime === 'function' ? normalizeTime : DDRFactory.defaultNormalizeTime;
    const isAbsent = (v) => !Number.isFinite(v) || v >= sentinel;

    let count = 0;
    for (let i = 0; i < n; i++) {
      if (isAbsent(values[i])) continue;
      const t = norm(times[i]);
      if (t == null) continue;
      count++;
    }
    if (count === 0) return [];

    const out = new Array(count);
    let j = 0;
    for (let i = 0; i < n; i++) {
      if (isAbsent(values[i])) continue;
      const t = norm(times[i]);
      if (t == null) continue;
      out[j++] = { time: t, value: values[i] };
    }
    return out;
  }

  getHydratedSeries(id) {
    return this.hydratedData.get(id);
  }

  updateTick(time, plots) {
    const chartTime = this.normalizeTime(time);
    if (chartTime == null || !plots || typeof plots !== 'object') {
      return;
    }
    for (const [key, rawValue] of Object.entries(plots)) {
      const series = this.seriesMap.get(key);
      if (!series) continue;
      const value = Number(rawValue);
      if (!Number.isFinite(value)) continue;
      try {
        series.update({ time: chartTime, value });
      } catch {
        /* skip invalid LWC updates */
      }
    }
  }

  getSeries(id) {
    return this.seriesMap.get(id);
  }

  clear() {
    this.seriesMap.clear();
    this.hydratedData.clear();
    this.manifest = null;
    this.cutoverActive = false;
  }

  _mountComponent(chartEntry, component) {
    const chart = chartEntry?.chart;
    if (!chart || !component?.id) return;
    if (this.seriesMap.has(component.id)) return;

    const kind = String(component.kind || 'line').toLowerCase();
    if (kind === 'marker' || component.dataMode === 'annotations') {
      return;
    }

    const renderOpts = DDRFactory._parseRenderOpts(component.renderOptions);
    const priceScaleId = DDRFactory.resolvePriceScaleId(component, chartEntry, renderOpts);
    const seriesOpts = { ...renderOpts, priceScaleId };
    delete seriesOpts.title;

    let series;
    switch (kind) {
      case 'area':
        series = chart.addAreaSeries(seriesOpts);
        break;
      case 'histogram':
        series = chart.addHistogramSeries(seriesOpts);
        break;
      case 'line':
      default:
        series = chart.addLineSeries(seriesOpts);
        break;
    }
    this.seriesMap.set(component.id, series);
  }

  static resolvePriceScaleId(component, chartEntry, renderOpts) {
    if (renderOpts?.priceScaleId) return renderOpts.priceScaleId;
    const byComponent = {
      line_rsx: 'rsx',
      line_rsx_signal: 'rsx',
      woz_fast: 'wozduh',
      woz_slow: 'wozduh',
      score_div_macro: 'wozduh',
      score_div_micro: 'wozduh',
      score_total: 'wozduh',
    };
    if (byComponent[component.id]) return byComponent[component.id];
    return chartEntry?.defaultPriceScaleId || 'right';
  }

  static _parseRenderOpts(raw) {
    if (!raw) return {};
    if (typeof raw === 'object') return { ...raw };
    try {
      const parsed = JSON.parse(raw);
      return parsed && typeof parsed === 'object' ? parsed : {};
    } catch {
      return {};
    }
  }
}

if (typeof window !== 'undefined') {
  window.DDRFactory = DDRFactory;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { DDRFactory };
}
