/**
 * WOZDUH-CROSSOVER-PAINT-1
 * One dedicated overlay for all four Wozduh crossover pairs.
 * Presentation chrome: Y = B numeric, Z above all Wozduh strokes. Not DDR. Not autoscale.
 */
(function (global) {
  'use strict';

  const HOST_VALUE = 50;

  function ignoreAutoscaleProvider() {
    const api = global.ScaleContribution;
    if (api && typeof api.createAutoscaleProvider === 'function') {
      const p = api.createAutoscaleProvider({ type: 'ignore' });
      if (typeof p === 'function') return p;
    }
    return () => null;
  }

  function hostSeriesOptions() {
    return {
      title: '',
      color: 'rgba(0,0,0,0)',
      lineWidth: 1,
      lineVisible: false,
      lastValueVisible: false,
      priceLineVisible: false,
      crosshairMarkerVisible: false,
      priceScaleId: 'right',
      autoscaleInfoProvider: ignoreAutoscaleProvider(),
    };
  }

  function prefsApi() {
    return global.WozduhCrossoverPrefs || null;
  }

  function pathCircle(ctx, x, y, r) {
    ctx.beginPath();
    ctx.arc(x, y, r, 0, Math.PI * 2);
    ctx.closePath();
  }

  function pathSquare(ctx, x, y, r) {
    ctx.beginPath();
    ctx.rect(x - r, y - r, r * 2, r * 2);
    ctx.closePath();
  }

  function pathTriangle(ctx, x, y, r) {
    const h = r * Math.sqrt(3);
    ctx.beginPath();
    ctx.moveTo(x, y - (2 / 3) * h);
    ctx.lineTo(x + r, y + (1 / 3) * h);
    ctx.lineTo(x - r, y + (1 / 3) * h);
    ctx.closePath();
  }

  /** Symmetric four-pointed sparkle (not a 5-point star, not a diamond). */
  function pathStar4(ctx, x, y, r) {
    const inner = r * 0.32;
    ctx.beginPath();
    for (let i = 0; i < 8; i++) {
      const ang = (-Math.PI / 2) + i * (Math.PI / 4);
      const rad = (i % 2 === 0) ? r : inner;
      const px = x + Math.cos(ang) * rad;
      const py = y + Math.sin(ang) * rad;
      if (i === 0) ctx.moveTo(px, py);
      else ctx.lineTo(px, py);
    }
    ctx.closePath();
  }

  function pathForShape(shape) {
    if (shape === 'square') return pathSquare;
    if (shape === 'triangle') return pathTriangle;
    if (shape === 'star4') return pathStar4;
    return pathCircle;
  }

  class CrossoverRenderer {
    constructor(source) {
      this._source = source;
    }

    draw(target) {
      const series = this._source._series;
      const chart = this._source._chart;
      if (!series || typeof series.priceToCoordinate !== 'function') return;
      const timeScale = chart && typeof chart.timeScale === 'function' ? chart.timeScale() : null;
      if (!timeScale || typeof timeScale.timeToCoordinate !== 'function') return;
      if (typeof target.useMediaCoordinateSpace !== 'function') return;
      const events = this._source._events;
      const prefs = prefsApi();
      if (!Array.isArray(events) || !events.length || !prefs) return;
      const byId = Object.create(null);
      for (const row of prefs.allResolved()) byId[row.id] = row;

      target.useMediaCoordinateSpace(({ context: ctx }) => {
        if (!ctx) return;
        for (let i = 0; i < events.length; i++) {
          const ev = events[i];
          if (!ev) continue;
          const cfg = byId[ev.pair];
          if (!cfg || !cfg.visible) continue;
          const x = timeScale.timeToCoordinate(ev.time);
          const y = series.priceToCoordinate(ev.y);
          if (x == null || y == null || !Number.isFinite(x) || !Number.isFinite(y)) continue;
          const r = Math.max(1.5, Number(cfg.size) / 2);
          const fill = ev.side === 'down' ? cfg.downFill : cfg.upFill;
          const path = pathForShape(cfg.shape);
          path(ctx, x, y, r);
          ctx.fillStyle = fill;
          ctx.fill();
          const ow = Number(cfg.outlineWidth);
          if (ow > 0) {
            ctx.lineWidth = ow;
            ctx.strokeStyle = cfg.outlineColor;
            ctx.setLineDash(cfg.outlineStyle === 'dashed' ? [Math.max(2, ow * 2), Math.max(2, ow * 2)] : []);
            path(ctx, x, y, r);
            ctx.stroke();
            ctx.setLineDash([]);
          }
        }
      });
    }
  }

  class CrossoverPaneView {
    constructor(source) {
      this._source = source;
      this._renderer = new CrossoverRenderer(source);
    }

    update() {}

    renderer() {
      return this._renderer;
    }

    zOrder() {
      return 'top';
    }
  }

  class WozduhCrossoverPrimitive {
    constructor() {
      this._chart = null;
      this._series = null;
      this._requestUpdate = null;
      this._events = [];
      this._paneViews = [new CrossoverPaneView(this)];
    }

    attached(param) {
      this._chart = param && param.chart ? param.chart : null;
      this._series = param && param.series ? param.series : null;
      this._requestUpdate = param && param.requestUpdate ? param.requestUpdate : null;
    }

    detached() {
      this._chart = null;
      this._series = null;
      this._requestUpdate = null;
    }

    paneViews() {
      return this._paneViews;
    }

    updateAllViews() {
      this._paneViews.forEach((view) => view.update());
    }

    setEvents(events) {
      this._events = Array.isArray(events) ? events : [];
      if (typeof this._requestUpdate === 'function') this._requestUpdate();
    }
  }

  /** @type {{ chart: object, series: object, primitive: WozduhCrossoverPrimitive }[]} */
  let attachments = [];

  function findAttachment(chart) {
    for (let i = 0; i < attachments.length; i++) {
      if (attachments[i].chart === chart) return attachments[i];
    }
    return null;
  }

  function attach(chart) {
    if (!chart || typeof chart.addLineSeries !== 'function') return false;
    if (findAttachment(chart)) return true;
    let series;
    try {
      series = chart.addLineSeries(hostSeriesOptions());
    } catch {
      return false;
    }
    if (!series || typeof series.attachPrimitive !== 'function') {
      try {
        if (typeof series.remove === 'function') series.remove();
      } catch { /* */ }
      return false;
    }
    const primitive = new WozduhCrossoverPrimitive();
    try {
      series.attachPrimitive(primitive);
    } catch {
      try {
        if (typeof series.remove === 'function') series.remove();
      } catch { /* */ }
      return false;
    }
    attachments.push({ chart, series, primitive });
    return true;
  }

  function refresh(realTipTime) {
    if (!attachments.length) return false;
    let time = realTipTime;
    if (time != null && typeof time === 'object' && Object.prototype.hasOwnProperty.call(time, 'time')) {
      time = time.time;
    }
    if (time == null || time === '') return false;
    const n = Number(time);
    if (Number.isFinite(n) && String(n) === String(time)) time = n;
    const point = { time, value: HOST_VALUE };
    let ok = false;
    for (let i = 0; i < attachments.length; i++) {
      const series = attachments[i].series;
      if (!series || typeof series.setData !== 'function') continue;
      try {
        series.setData([point]);
        ok = true;
      } catch { /* disposed */ }
    }
    return ok;
  }

  function setEvents(events) {
    const list = Array.isArray(events) ? events : [];
    for (let i = 0; i < attachments.length; i++) {
      const p = attachments[i].primitive;
      if (p && typeof p.setEvents === 'function') p.setEvents(list);
    }
  }

  function requestPaint() {
    for (let i = 0; i < attachments.length; i++) {
      const p = attachments[i].primitive;
      if (p && typeof p._requestUpdate === 'function') p._requestUpdate();
    }
  }

  function dispose() {
    if (!attachments.length) return false;
    const prev = attachments;
    attachments = [];
    for (let i = 0; i < prev.length; i++) {
      const { series, primitive } = prev[i];
      try {
        if (series && typeof series.detachPrimitive === 'function' && primitive) {
          series.detachPrimitive(primitive);
        }
      } catch { /* */ }
      try {
        if (series && typeof series.remove === 'function') series.remove();
      } catch { /* */ }
    }
    return true;
  }

  function _resetForTests() {
    attachments = [];
  }

  const api = {
    attach,
    refresh,
    setEvents,
    requestPaint,
    dispose,
    HOST_VALUE,
    pathStar4,
    pathCircle,
    pathSquare,
    pathTriangle,
    _resetForTests,
    _hostSeriesOptionsForTests: hostSeriesOptions,
    _attachmentCountForTests() { return attachments.length; },
    _WozduhCrossoverPrimitive: WozduhCrossoverPrimitive,
  };

  global.WozduhCrossovers = api;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  }
})(typeof window !== 'undefined' ? window : globalThis);
