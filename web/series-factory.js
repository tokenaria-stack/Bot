/**
 * DDRFactory — Data-Driven Rendering (DDR) module (Phase 6 cutover).
 * Manifest-driven series mount + columnar history hydration + live tick updates.
 */
const ScaleContributionApi = (typeof ScaleContribution !== 'undefined')
  ? ScaleContribution
  : (typeof require === 'function'
    ? (() => { try { return require('./ui/scale-contribution.js'); } catch { return null; } })()
    : null);

class DDRFactory {
  /** @type {number} Server-side sentinel (wire.HistoryAbsent / math.MaxFloat64). */
  static get HISTORY_ABSENT() {
    return 1.7976931348623157e+308;
  }

  constructor(options = {}) {
    /** @type {Map<string, { chart: import('lightweight-charts').IChartApi, series: import('lightweight-charts').ISeriesApi }>} */
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
   * Mounts manifest components onto charts via hostId routing from the backend manifest.
   *
   * @param {Record<string, { chart: import('lightweight-charts').IChartApi, defaultPriceScaleId?: string }>} hostMap
   * @param {Record<string, object[]>} [panesData]
   */
  buildPanes(hostMap, panesData) {
    const panes = panesData || this.manifest?.panes;
    if (!hostMap || !panes || typeof panes !== 'object') {
      return;
    }
    for (const components of Object.values(panes)) {
      if (!Array.isArray(components)) continue;
      for (const component of components) {
        const hostId = component.hostId || component.hostID;
        if (!hostId) continue;
        const entry = hostMap[hostId];
        if (!entry?.chart) continue;
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
      params.set('rsx_source', rsx.source ?? 'hlc3');
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
    for (const [id, entry] of this.seriesMap) {
      const series = DDRFactory._seriesFromEntry(entry);
      const points = source instanceof Map ? source.get(id) : this.hydratedData.get(id);
      if (!series || !points?.length) continue;
      try {
        series.setData(points);
      } catch {
        /* skip invalid setData during cutover */
      }
    }
  }

  /** @param {{ chart?: object, series?: object }|object|null} entry */
  static _seriesFromEntry(entry) {
    if (!entry) return null;
    if (entry.series && typeof entry.series.setData === 'function') return entry.series;
    if (typeof entry.setData === 'function') return entry;
    return null;
  }

  static columnToLWC(times, values, sentinel, normalizeTime) {
    const n = Math.min(times.length, values.length);
    if (n === 0) return [];
    const norm = typeof normalizeTime === 'function' ? normalizeTime : DDRFactory.defaultNormalizeTime;
    const isAbsent = (v) => !Number.isFinite(v) || v >= sentinel;

    const out = [];
    for (let i = 0; i < n; i++) {
      const t = norm(times[i]);
      if (t == null) continue;
      if (isAbsent(values[i])) {
        out.push({ time: t }); // Whitespace point
      } else {
        out.push({ time: t, value: values[i] });
      }
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
    const absent = DDRFactory.HISTORY_ABSENT;
    for (const [key, rawValue] of Object.entries(plots)) {
      const series = DDRFactory._seriesFromEntry(this.seriesMap.get(key));
      if (!series) continue;
      const value = Number(rawValue);
      // Shot 11D-HOTFIX: same isAbsent contract as columnToLWC — never feed Sentinel into LWC.
      if (!Number.isFinite(value) || value >= absent) continue;
      try {
        series.update({ time: chartTime, value });
      } catch {
        /* skip invalid LWC updates */
      }
    }
  }

  getSeries(id) {
    return DDRFactory._seriesFromEntry(this.seriesMap.get(id));
  }

  setSeriesVisible(id, visible) {
    const series = DDRFactory._seriesFromEntry(this.seriesMap.get(id));
    if (!series) return false;
    series.applyOptions({ visible: visible !== false });
    return true;
  }

  clear() {
    for (const entry of this.seriesMap.values()) {
      const chart = entry?.chart;
      const series = DDRFactory._seriesFromEntry(entry);
      if (!chart || !series) continue;
      try {
        chart.removeSeries(series);
      } catch {
        /* series may already be detached */
      }
    }
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
    const seriesOpts = {
      ...renderOpts,
      priceScaleId,
      crosshairMarkerVisible: false,
      crosshairMarkerRadius: 0,
    };
    delete seriesOpts.title;
    delete seriesOpts.defaultVisible;
    // scaleMargins belongs on PriceScale, not SeriesOptions.
    const scaleMargins = seriesOpts.scaleMargins;
    delete seriesOpts.scaleMargins;
    // ADR-022: DDR domain contribution — not an LWC series option key.
    const scaleContribution = seriesOpts.scaleContribution;
    delete seriesOpts.scaleContribution;
    if (ScaleContributionApi && typeof ScaleContributionApi.createAutoscaleProvider === 'function') {
      const provider = ScaleContributionApi.createAutoscaleProvider(scaleContribution);
      if (provider !== undefined) {
        seriesOpts.autoscaleInfoProvider = provider;
      }
    }

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
    // Own scale only — never push margins onto shared right/overlay host (would crush Wozduh).
    if (scaleMargins && priceScaleId !== '' && priceScaleId != null) {
      series.priceScale()?.applyOptions?.({ scaleMargins });
    }
    this.seriesMap.set(component.id, { chart, series });
  }

  static resolvePriceScaleId(component, chartEntry, renderOpts) {
    // Explicit "" means LWC overlay mode — must not fall through (falsy check).
    if (renderOpts && Object.prototype.hasOwnProperty.call(renderOpts, 'priceScaleId')) {
      return renderOpts.priceScaleId;
    }
    if (chartEntry?.defaultPriceScaleId) return chartEntry.defaultPriceScaleId;
    return 'right';
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
