/**
 * WOZDUH-SCALE-1 — Pine urvol extreme bands as Wozduh pane chrome.
 * Private host + ISeriesPrimitive. Not DDR, not store, not settings.
 *
 * Public: attach / dispose. No series getters, no setData/update pipeline.
 */
(function (global) {
  'use strict';

  const HOST_TITLE = '__wozduh_extreme_bands__';

  /** Pine: urvol=8; inner = urvol-3; high = 100-urvol. */
  const LEVELS = Object.freeze({
    lowInner: 5,
    lowOuter: 8,
    highInner: 89,
    highOuter: 92,
  });

  const FILL = 'rgba(255, 255, 0, 0.2)';
  /** Pine hline has no color= — TV default grey. */
  const STROKE = 'rgba(120, 123, 134, 0.85)';
  const DOTTED_DASH = [1, 2];

  function hostSeriesOptions() {
    return {
      title: HOST_TITLE,
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
      const yLi = series.priceToCoordinate(LEVELS.lowInner);
      const yLo = series.priceToCoordinate(LEVELS.lowOuter);
      const yHi = series.priceToCoordinate(LEVELS.highInner);
      const yHo = series.priceToCoordinate(LEVELS.highOuter);
      target.useMediaCoordinateSpace(({ context: ctx, mediaSize }) => {
        if (!ctx || !mediaSize) return;
        const w = mediaSize.width;
        if (!(w > 0)) return;
        fillBand(ctx, w, yLi, yLo);
        fillBand(ctx, w, yHi, yHo);
        strokeH(ctx, w, yLi, false);
        strokeH(ctx, w, yLo, true);
        strokeH(ctx, w, yHi, false);
        strokeH(ctx, w, yHo, true);
      });
    }
  }

  function fillBand(ctx, width, ya, yb) {
    if (ya == null || yb == null || !Number.isFinite(ya) || !Number.isFinite(yb)) return;
    const top = Math.min(ya, yb);
    const h = Math.abs(yb - ya);
    if (!(h > 0)) return;
    ctx.fillStyle = FILL;
    ctx.fillRect(0, top, width, h);
  }

  function strokeH(ctx, width, y, dotted) {
    if (y == null || !Number.isFinite(y)) return;
    ctx.strokeStyle = STROKE;
    ctx.lineWidth = 1;
    ctx.setLineDash(dotted ? DOTTED_DASH : []);
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
    dispose,
    LEVELS,
    FILL,
    STROKE,
    HOST_TITLE,
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
