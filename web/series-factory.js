/**
 * SeriesFactory — Data-Driven Rendering (DDR) shadow module (Phase 2).
 * Not wired into app.js or chart-adapter.js yet.
 *
 * Consumes GET /api/ui/manifest and projects tick plots onto LWC series.
 */
class SeriesFactory {
  constructor(options = {}) {
    /** @type {Map<string, import('lightweight-charts').ISeriesApi>} */
    this.seriesMap = new Map();
    /** @type {{ panes?: Record<string, object[]> } | null} */
    this.manifest = null;
    this.normalizeTime = typeof options.normalizeTime === 'function'
      ? options.normalizeTime
      : SeriesFactory.defaultNormalizeTime;
  }

  /**
   * Delegates to global chartTime (mappers.js) when present; otherwise ms/sec → Unix seconds.
   * Daily business-day strings are NOT produced here — inject normalizeTime for that path.
   */
  static defaultNormalizeTime(raw) {
    if (typeof chartTime === 'function') {
      return chartTime(raw);
    }
    const t = Number(raw);
    if (!Number.isFinite(t)) return null;
    return t >= 1e12 ? Math.floor(t / 1000) : Math.floor(t);
  }

  /**
   * @param {string} [url='/api/ui/manifest']
   * @returns {Promise<object>}
   */
  async fetchManifest(url = '/api/ui/manifest') {
    const res = await fetch(url);
    if (!res.ok) {
      throw new Error(`SeriesFactory: manifest fetch failed (${res.status})`);
    }
    this.manifest = await res.json();
    return this.manifest;
  }

  /**
   * Mounts manifest components onto a chart instance.
   *
   * @param {import('lightweight-charts').IChartApi} chartInstance
   * @param {Record<string, object[]>} manifestPanes  e.g. manifest.panes
   */
  buildPanes(chartInstance, manifestPanes) {
    if (!chartInstance || !manifestPanes || typeof manifestPanes !== 'object') {
      return;
    }
    for (const components of Object.values(manifestPanes)) {
      if (!Array.isArray(components)) continue;
      for (const component of components) {
        this._mountComponent(chartInstance, component);
      }
    }
  }

  /**
   * @param {number|string} time  raw tick time (normalized once per call)
   * @param {Record<string, number>} plots  e.g. { line_rsx: 45.2 }
   */
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
        /* shadow mode: skip invalid LWC updates */
      }
    }
  }

  /** @returns {import('lightweight-charts').ISeriesApi | undefined} */
  getSeries(id) {
    return this.seriesMap.get(id);
  }

  clear() {
    this.seriesMap.clear();
    this.manifest = null;
  }

  /**
   * @param {import('lightweight-charts').IChartApi} chart
   * @param {object} component
   */
  _mountComponent(chart, component) {
    if (!chart || !component?.id) return;
    if (this.seriesMap.has(component.id)) return;

    const kind = String(component.kind || 'line').toLowerCase();
    if (kind === 'marker' || component.dataMode === 'annotations') {
      return;
    }

    const opts = SeriesFactory._parseRenderOpts(component.renderOptions);
    let series;
    switch (kind) {
      case 'area':
        series = chart.addAreaSeries(opts);
        break;
      case 'histogram':
        series = chart.addHistogramSeries(opts);
        break;
      case 'line':
      default:
        series = chart.addLineSeries(opts);
        break;
    }
    this.seriesMap.set(component.id, series);
  }

  /**
   * @param {string|object} raw
   * @returns {object}
   */
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
  window.SeriesFactory = SeriesFactory;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { SeriesFactory };
}
