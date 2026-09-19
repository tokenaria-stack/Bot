/**
 * CHANNEL-PAINT-1 + CHANNEL-SPLIT-FILLS-1
 * LWC 4.2 custom series: one {time, upper, mid, lower} channel.
 * Optional fills/strokes are capability-from-options only.
 * Visible-range renderer only. Does not own history or indicator math.
 */
(function (global) {
  'use strict';

  function isChannelPoint(data) {
    return data
      && Number.isFinite(data.upper)
      && Number.isFinite(data.mid)
      && Number.isFinite(data.lower);
  }

  function dashForStyle(style) {
    const n = Number(style);
    if (n === 1) return [2, 2];
    if (n === 2) return [6, 4];
    return [];
  }

  /** Non-empty CSS string = draw that region. Missing/blank = skip. */
  function presentPaint(opts, field) {
    if (!opts || typeof opts !== 'object') return '';
    const v = opts[field];
    if (typeof v !== 'string') return '';
    const s = v.trim();
    return s || '';
  }

  function fillModelConflict(opts) {
    const whole = !!presentPaint(opts, 'fillColor');
    const split = !!presentPaint(opts, 'upperFillColor')
      || !!presentPaint(opts, 'lowerFillColor');
    return whole && split;
  }

  function strokeModelConflict(opts) {
    const bound = !!presentPaint(opts, 'boundColor');
    const edges = !!presentPaint(opts, 'upperColor')
      || !!presentPaint(opts, 'lowerColor');
    return bound && edges;
  }

  class ChannelRenderer {
    constructor() {
      this._data = null;
      this._options = null;
    }

    update(data, options) {
      this._data = data;
      this._options = options;
    }

    draw(target, priceToCoordinate) {
      const data = this._data;
      const opts = this._options || {};
      if (!data || !data.bars || !data.visibleRange || typeof priceToCoordinate !== 'function') {
        return;
      }
      const convert = typeof target.useMediaCoordinateSpace === 'function'
        ? (fn) => target.useMediaCoordinateSpace(fn)
        : (fn) => fn({ context: target });
      convert((scope) => {
        const ctx = scope.context;
        if (!ctx) return;
        const bars = data.bars;
        const from = data.visibleRange.from;
        const to = data.visibleRange.to;
        const spacing = Number(data.barSpacing) > 0 ? Number(data.barSpacing) : 6;
        const segments = [];
        let seg = [];
        const flush = () => {
          if (seg.length) segments.push(seg);
          seg = [];
        };
        for (let i = from; i < to; i++) {
          const bar = bars[i];
          const d = bar && bar.originalData;
          if (!isChannelPoint(d)) {
            flush();
            continue;
          }
          const yu = priceToCoordinate(d.upper);
          const ym = priceToCoordinate(d.mid);
          const yl = priceToCoordinate(d.lower);
          if (yu == null || ym == null || yl == null) {
            flush();
            continue;
          }
          if (seg.length) {
            const prev = seg[seg.length - 1];
            if (bar.x - prev.x > spacing * 1.6) flush();
          }
          seg.push({ x: bar.x, yu, ym, yl });
        }
        flush();

        const conflictFills = fillModelConflict(opts);
        const upperFill = conflictFills ? '' : presentPaint(opts, 'upperFillColor');
        const lowerFill = conflictFills ? '' : presentPaint(opts, 'lowerFillColor');
        const interior = conflictFills ? '' : presentPaint(opts, 'fillColor');
        const bound = presentPaint(opts, 'boundColor');
        const upperColor = bound || presentPaint(opts, 'upperColor') || 'blue';
        const lowerColor = bound || presentPaint(opts, 'lowerColor') || 'blue';
        const midColor = presentPaint(opts, 'midColor') || 'orange';
        const lineWidth = Number(opts.lineWidth) > 0 ? Number(opts.lineWidth) : 1;
        const midWidth = Number(opts.midLineWidth) > 0 ? Number(opts.midLineWidth) : lineWidth;
        const upperDash = dashForStyle(opts.upperLineStyle);
        const lowerDash = dashForStyle(opts.lowerLineStyle);

        for (let s = 0; s < segments.length; s++) {
          const pts = segments[s];
          if (pts.length >= 2) {
            if (upperFill) {
              ctx.beginPath();
              ctx.moveTo(pts[0].x, pts[0].yu);
              for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x, pts[i].yu);
              for (let i = pts.length - 1; i >= 0; i--) ctx.lineTo(pts[i].x, pts[i].ym);
              ctx.closePath();
              ctx.fillStyle = upperFill;
              ctx.fill();
            }
            if (lowerFill) {
              ctx.beginPath();
              ctx.moveTo(pts[0].x, pts[0].ym);
              for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x, pts[i].ym);
              for (let i = pts.length - 1; i >= 0; i--) ctx.lineTo(pts[i].x, pts[i].yl);
              ctx.closePath();
              ctx.fillStyle = lowerFill;
              ctx.fill();
            }
            if (interior) {
              ctx.beginPath();
              ctx.moveTo(pts[0].x, pts[0].yu);
              for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x, pts[i].yu);
              for (let i = pts.length - 1; i >= 0; i--) ctx.lineTo(pts[i].x, pts[i].yl);
              ctx.closePath();
              ctx.fillStyle = interior;
              ctx.fill();
            }
          }
          ctx.lineWidth = lineWidth;
          ctx.setLineDash(upperDash);
          ctx.strokeStyle = upperColor;
          ctx.beginPath();
          ctx.moveTo(pts[0].x, pts[0].yu);
          for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x, pts[i].yu);
          if (pts.length === 1) ctx.lineTo(pts[0].x + 0.01, pts[0].yu);
          ctx.stroke();

          ctx.setLineDash(lowerDash);
          ctx.strokeStyle = lowerColor;
          ctx.beginPath();
          ctx.moveTo(pts[0].x, pts[0].yl);
          for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x, pts[i].yl);
          if (pts.length === 1) ctx.lineTo(pts[0].x + 0.01, pts[0].yl);
          ctx.stroke();

          ctx.setLineDash([]);
          ctx.lineWidth = midWidth;
          ctx.strokeStyle = midColor;
          ctx.beginPath();
          ctx.moveTo(pts[0].x, pts[0].ym);
          for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i].x, pts[i].ym);
          if (pts.length === 1) ctx.lineTo(pts[0].x + 0.01, pts[0].ym);
          ctx.stroke();
        }
      });
    }
  }

  class ChannelSeries {
    constructor() {
      this._renderer = new ChannelRenderer();
    }

    defaultOptions() {
      return {
        lastValueVisible: false,
        priceLineVisible: false,
        crosshairMarkerVisible: false,
        lineWidth: 1,
        midLineWidth: 1,
        upperLineStyle: 0,
        lowerLineStyle: 0,
      };
    }

    isWhitespace(data) {
      return !isChannelPoint(data);
    }

    priceValueBuilder(plotRow) {
      if (!isChannelPoint(plotRow)) return [];
      return [plotRow.lower, plotRow.upper, plotRow.mid];
    }

    renderer() {
      return this._renderer;
    }

    update(data, seriesOptions) {
      this._renderer.update(data, seriesOptions);
    }

    destroy() {}
  }

  const api = {
    ChannelSeries,
    isChannelPoint,
    presentPaint,
    fillModelConflict,
    strokeModelConflict,
  };

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  }
  global.ChannelSeriesApi = api;
}(typeof window !== 'undefined' ? window : globalThis));
