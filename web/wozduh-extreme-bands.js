/**
 * WOZDUH-SCALE-1 — Pine urvol extreme bands as Wozduh pane chrome.
 * Private host + ISeriesPrimitive. Not DDR, not store, not settings.
 *
 * Public: attach / refresh / dispose. refresh() seeds one priced host point.
 */
(function (global) {
  'use strict';

  /** Pine urvol extremes + TV 27–30 / 50 / 67–70 chrome. */
  const LEVELS = Object.freeze({
    lowInner: 5,
    lowOuter: 8,
    redInner: 27,
    redOuter: 30,
    mid: 50,
    greenInner: 67,
    greenOuter: 70,
    highInner: 89,
    highOuter: 92,
  });

  /** Inert Y on the Wozduh domain; host does not contribute to Auto. */
  const HOST_VALUE = 50;

  const FILL_YELLOW = 'rgba(255, 255, 0, 0.2)';
  const GREEN_HI = 'rgba(8, 153, 129, 0.28)';
  const GREEN_LO = 'rgba(8, 153, 129, 0.06)';
  const RED_HI = 'rgba(242, 54, 69, 0.28)';
  const RED_LO = 'rgba(242, 54, 69, 0.06)';
  /** Pine hline has no color= — TV default grey. */
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

  class ExtremeBandsRenderer {
    constructor(source) {
      this._source = source;
    }

    draw(target) {
      const series = this._source._series;
      if (!series || typeof series.priceToCoordinate !== 'function') return;
      if (typeof target.useMediaCoordinateSpace !== 'function') return;
      const y5 = series.priceToCoordinate(LEVELS.lowInner);
      const y8 = series.priceToCoordinate(LEVELS.lowOuter);
      const y27 = series.priceToCoordinate(LEVELS.redInner);
      const y30 = series.priceToCoordinate(LEVELS.redOuter);
      const y50 = series.priceToCoordinate(LEVELS.mid);
      const y67 = series.priceToCoordinate(LEVELS.greenInner);
      const y70 = series.priceToCoordinate(LEVELS.greenOuter);
      const y89 = series.priceToCoordinate(LEVELS.highInner);
      const y92 = series.priceToCoordinate(LEVELS.highOuter);
      target.useMediaCoordinateSpace(({ context: ctx, mediaSize }) => {
        if (!ctx || !mediaSize) return;
        const w = mediaSize.width;
        if (!(w > 0)) return;
        fillSolidBand(ctx, w, y5, y8, FILL_YELLOW);
        fillSolidBand(ctx, w, y89, y92, FILL_YELLOW);
        fillGradientBand(ctx, w, y67, y70, GREEN_LO, GREEN_HI);
        fillGradientBand(ctx, w, y27, y30, RED_LO, RED_HI);
        strokeH(ctx, w, y5, 'solid');
        strokeH(ctx, w, y8, 'dotted');
        strokeH(ctx, w, y27, 'dotted');
        strokeH(ctx, w, y30, 'dotted');
        strokeH(ctx, w, y50, 'dotted');
        strokeH(ctx, w, y67, 'dotted');
        strokeH(ctx, w, y70, 'dotted');
        strokeH(ctx, w, y89, 'solid');
        strokeH(ctx, w, y92, 'dotted');
      });
    }
  }

  function fillSolidBand(ctx, width, ya, yb, color) {
    if (ya == null || yb == null || !Number.isFinite(ya) || !Number.isFinite(yb)) return;
    const top = Math.min(ya, yb);
    const h = Math.abs(yb - ya);
    if (!(h > 0)) return;
    ctx.fillStyle = color;
    ctx.fillRect(0, top, width, h);
  }

  function fillGradientBand(ctx, width, yInner, yOuter, colorInner, colorOuter) {
    if (yInner == null || yOuter == null || !Number.isFinite(yInner) || !Number.isFinite(yOuter)) return;
    if (typeof ctx.createLinearGradient !== 'function') return;
    const top = Math.min(yInner, yOuter);
    const bot = Math.max(yInner, yOuter);
    if (!(bot - top > 0)) return;
    const g = ctx.createLinearGradient(0, yOuter, 0, yInner);
    g.addColorStop(0, colorOuter);
    g.addColorStop(1, colorInner);
    ctx.fillStyle = g;
    ctx.fillRect(0, top, width, bot - top);
  }

  function dashFor(style) {
    if (style === 'dotted') return DOTTED_DASH;
    return [];
  }

  function strokeH(ctx, width, y, style) {
    if (y == null || !Number.isFinite(y)) return;
    ctx.strokeStyle = STROKE;
    ctx.lineWidth = 1;
    ctx.setLineDash(dashFor(style));
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(width, y);
    ctx.stroke();
    ctx.setLineDash([]);
  }

  class ExtremeBandsPaneView {
    constructor(source) {
      this._source = source;
      this._renderer = new ExtremeBandsRenderer(source);
    }

    update() {}

    renderer() {
      return this._renderer;
    }

    zOrder() {
      return 'bottom';
    }
  }

  class WozduhExtremeBandsPrimitive {
    constructor() {
      this._chart = null;
      this._series = null;
      this._requestUpdate = null;
      this._paneViews = [new ExtremeBandsPaneView(this)];
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

  /** @type {{ chart: object, series: object, primitive: WozduhExtremeBandsPrimitive }[]} */
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
    const primitive = new WozduhExtremeBandsPrimitive();
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

  /**
   * Seed the private host with one priced point so priceToCoordinate works.
   * @param {*} realTipTime painted candle tip (same identity as LWC candles)
   * @returns {boolean}
   */
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

  function _attachmentCountForTests() {
    return attachments.length;
  }

  const api = {
    attach,
    refresh,
    dispose,
    LEVELS,
    HOST_VALUE,
    FILL_YELLOW,
    STROKE,
    _resetForTests,
    _hostSeriesOptionsForTests: hostSeriesOptions,
    _attachmentCountForTests,
    _WozduhExtremeBandsPrimitive: WozduhExtremeBandsPrimitive,
  };

  global.WozduhExtremeBands = api;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  }
})(typeof window !== 'undefined' ? window : globalThis);
