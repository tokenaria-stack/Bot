/**
 * RSX-SCALE-1 — dotted 30 / 50 / 70 pane chrome.
 * Sibling of WozduhExtremeBands. Not DDR, not store, not settings.
 *
 * Public: attach / refresh / dispose.
 */
(function (global) {
  'use strict';

  const LEVELS = Object.freeze({ low: 30, mid: 50, high: 70 });
  const HOST_VALUE = 50;
  /** Match Wozduh dotted scale strokes. */
  const STROKE = 'rgba(120, 123, 134, 0.85)';
  const DOTTED_DASH = [1, 2];

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
      autoscaleInfoProvider: () => null,
    };
  }

  class RsxScaleLinesRenderer {
    constructor(source) {
      this._source = source;
    }

    draw(target) {
      const series = this._source._series;
      if (!series || typeof series.priceToCoordinate !== 'function') return;
      if (typeof target.useMediaCoordinateSpace !== 'function') return;
      const y30 = series.priceToCoordinate(LEVELS.low);
      const y50 = series.priceToCoordinate(LEVELS.mid);
      const y70 = series.priceToCoordinate(LEVELS.high);
      target.useMediaCoordinateSpace(({ context: ctx, mediaSize }) => {
        if (!ctx || !mediaSize) return;
        const w = mediaSize.width;
        if (!(w > 0)) return;
        strokeDotted(ctx, w, y30);
        strokeDotted(ctx, w, y50);
        strokeDotted(ctx, w, y70);
      });
    }
  }

  function strokeDotted(ctx, width, y) {
    if (y == null || !Number.isFinite(y)) return;
    ctx.strokeStyle = STROKE;
    ctx.lineWidth = 1;
    ctx.setLineDash(DOTTED_DASH);
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(width, y);
    ctx.stroke();
    ctx.setLineDash([]);
  }

  class RsxScaleLinesPaneView {
    constructor(source) {
      this._source = source;
      this._renderer = new RsxScaleLinesRenderer(source);
    }

    update() {}

    renderer() {
      return this._renderer;
    }

    zOrder() {
      return 'bottom';
    }
  }

  class RsxScaleLinesPrimitive {
    constructor() {
      this._chart = null;
      this._series = null;
      this._requestUpdate = null;
      this._paneViews = [new RsxScaleLinesPaneView(this)];
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
  }

  /** @type {{ chart: object, series: object, primitive: RsxScaleLinesPrimitive }[]} */
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
    const primitive = new RsxScaleLinesPrimitive();
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
    dispose,
    LEVELS,
    HOST_VALUE,
    STROKE,
    DOTTED_DASH,
    _resetForTests,
    _hostSeriesOptionsForTests: hostSeriesOptions,
    _attachmentCountForTests: () => attachments.length,
    _RsxScaleLinesPrimitive: RsxScaleLinesPrimitive,
  };

  global.RsxScaleLines = api;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  }
})(typeof window !== 'undefined' ? window : globalThis);
