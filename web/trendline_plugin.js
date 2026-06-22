/**
 * Trendline segment + background zone primitives for Lightweight Charts v4+.
 * Coordinates: Unix time (sec) on X, price on Y — interpolate in price/time space, then map to pixels.
 * Active lines extend to the viewport edge (LuxAlgo live ray). Completed lines are fixed segments to the break bar.
 */
(function initTrendlinePlugin(global) {
  function toChartTimeSec(raw) {
    const n = Number(raw);
    if (!Number.isFinite(n)) return null;
    return n > 1e12 ? Math.floor(n / 1000) : Math.floor(n);
  }

  function linePriceAtTime(t1, p1, t2, p2, t) {
    if (t2 === t1) return p1;
    return p1 + ((p2 - p1) * (t - t1)) / (t2 - t1);
  }

  function visibleTimeRange(timeScale) {
    const range = timeScale.getVisibleRange?.();
    if (!range) return null;
    const from = typeof range.from === 'number' ? range.from : range.from?.timestamp ?? range.from;
    const to = typeof range.to === 'number' ? range.to : range.to?.timestamp ?? range.to;
    if (!Number.isFinite(from) || !Number.isFinite(to)) return null;
    return { from, to };
  }

  function priceToPixel(series, price) {
    const y = series.priceToCoordinate(price);
    if (y === null || y === undefined || !Number.isFinite(y) || Number.isNaN(y)) {
      return null;
    }
    return y;
  }

  function timeToPixel(timeScale, t) {
    const x = timeScale.timeToCoordinate(t);
    if (x === null || x === undefined || !Number.isFinite(x) || Number.isNaN(x)) {
      return null;
    }
    return x;
  }

  function isValidEndpoint(x, y) {
    return x !== null && y !== null && Number.isFinite(x) && Number.isFinite(y);
  }

  /** Completed line: segment [time1, time2] only — Pine keeps the line object, no extend after break. */
  function resolveCompletedSegment(timeScale, series, t1, t2, p1, p2, canvasWidth) {
    const segFrom = Math.min(t1, t2);
    const segTo = Math.max(t1, t2);
    const vis = visibleTimeRange(timeScale);

    let drawT1 = segFrom;
    let drawT2 = segTo;
    if (vis) {
      if (segTo < vis.from || segFrom > vis.to) return null;
      drawT1 = Math.max(segFrom, vis.from);
      drawT2 = Math.min(segTo, vis.to);
    }

    const price1 = linePriceAtTime(t1, p1, t2, p2, drawT1);
    const price2 = linePriceAtTime(t1, p1, t2, p2, drawT2);
    const x1 = timeToPixel(timeScale, drawT1);
    const x2 = timeToPixel(timeScale, drawT2);
    const y1 = priceToPixel(series, price1);
    const y2 = priceToPixel(series, price2);
    if (!isValidEndpoint(x1, y1) || !isValidEndpoint(x2, y2)) return null;

    const left = Math.min(x1, x2);
    const right = Math.max(x1, x2);
    if (right < 0 || left > canvasWidth) return null;

    return { x1, y1, x2, y2 };
  }

  /** Active line: extrapolate along price/time slope to visible edges when endpoints are off-screen. */
  function resolveActiveLineEndpoints(timeScale, series, t1, t2, p1, p2, width) {
    const vis = visibleTimeRange(timeScale);
    let drawT1 = t1;
    let drawT2 = t2;

    if (vis) {
      if (t2 < vis.from || t1 > vis.to) return null;
      if (t1 < vis.from) drawT1 = vis.from;
      if (t2 > vis.to) drawT2 = vis.to;
    }

    const price1 = linePriceAtTime(t1, p1, t2, p2, drawT1);
    const price2 = linePriceAtTime(t1, p1, t2, p2, drawT2);
    let x1 = timeToPixel(timeScale, drawT1);
    let x2 = timeToPixel(timeScale, drawT2);

    if (x1 == null && x2 == null) return null;

    if (x1 == null) {
      const edgeT = t1 < (vis?.from ?? drawT1) ? (vis?.from ?? drawT1) : (vis?.to ?? drawT2);
      x1 = timeToPixel(timeScale, edgeT);
      if (x1 == null) x1 = t1 < (vis?.from ?? 0) ? 0 : width;
      drawT1 = edgeT;
    }
    if (x2 == null) {
      const edgeT = t2 > (vis?.to ?? drawT2) ? (vis?.to ?? drawT2) : (vis?.from ?? drawT1);
      x2 = timeToPixel(timeScale, edgeT);
      if (x2 == null) x2 = t2 > (vis?.to ?? width) ? width : 0;
      drawT2 = edgeT;
    }

    const y1 = priceToPixel(series, linePriceAtTime(t1, p1, t2, p2, drawT1));
    const y2 = priceToPixel(series, linePriceAtTime(t1, p1, t2, p2, drawT2));
    if (!isValidEndpoint(x1, y1) || !isValidEndpoint(x2, y2)) return null;

    return { x1, y1, x2, y2 };
  }

  function lineWidthForStyle(style) {
    const s = String(style || 'solid').toLowerCase();
    return s === 'solid' ? 2 : 1;
  }

  function applyLineDash(ctx, style) {
    const s = String(style || 'solid').toLowerCase();
    if (s === 'dashed') {
      ctx.setLineDash([6, 4]);
    } else if (s === 'dotted') {
      ctx.setLineDash([2, 4]);
    } else {
      ctx.setLineDash([]);
    }
  }

  class TrendlinePaneRenderer {
    constructor(source) {
      this._source = source;
    }

    draw(target) {
      const source = this._source;
      if (!source._linesVisible) return;
      const chart = source._chart;
      const series = source._series;
      const lines = source._lines;
      if (!chart || !series || !lines?.length) return;

      const timeScale = chart.timeScale();

      target.useMediaCoordinateSpace(({ context: ctx, mediaSize }) => {
        lines.forEach((line) => {
          const t1 = toChartTimeSec(line.time1);
          const t2 = toChartTimeSec(line.time2);
          const p1 = Number(line.y1);
          const p2 = Number(line.y2);
          if (t1 == null || t2 == null || !Number.isFinite(p1) || !Number.isFinite(p2)) return;

          const isActive = line.isActive === true;
          const endpoints = isActive
            ? resolveActiveLineEndpoints(timeScale, series, t1, t2, p1, p2, mediaSize.width)
            : resolveCompletedSegment(timeScale, series, t1, t2, p1, p2, mediaSize.width);
          if (!endpoints) return;

          const { x1, y1, x2, y2 } = endpoints;
          if (!isValidEndpoint(x1, y1) || !isValidEndpoint(x2, y2)) return;

          const style = line.style || 'solid';
          applyLineDash(ctx, style);
          ctx.beginPath();
          ctx.strokeStyle = line.color || '#089981';
          ctx.lineWidth = line.width || lineWidthForStyle(style);
          ctx.globalAlpha = isActive ? 1 : 0.55;
          ctx.moveTo(x1, y1);
          ctx.lineTo(x2, y2);
          ctx.stroke();
          ctx.setLineDash([]);
          ctx.globalAlpha = 1;
        });
      });
    }
  }

  class TrendlineBackgroundRenderer {
    constructor(source) {
      this._source = source;
    }

    draw(target) {
      const source = this._source;
      if (!source._backgroundVisible) return;
      const chart = source._chart;
      const zones = source._backgroundZones;
      if (!chart || !zones?.length) return;

      const timeScale = chart.timeScale();

      target.useMediaCoordinateSpace(({ context: ctx, mediaSize }) => {
        const height = mediaSize.height;
        const canvasWidth = mediaSize.width;
        zones.forEach((zone) => {
          const t1 = toChartTimeSec(zone.startTime ?? zone.time1);
          const t2 = toChartTimeSec(zone.endTime ?? zone.time2);
          if (t1 == null || t2 == null) return;

          let startX = timeScale.timeToCoordinate(t1);
          let endX = timeScale.timeToCoordinate(t2);
          if (startX == null) startX = 0;
          if (endX == null) endX = canvasWidth;

          const left = Math.min(startX, endX);
          const width = Math.max(Math.abs(endX - startX), 1);
          ctx.fillStyle = zone.color || 'rgba(8, 153, 129, 0.08)';
          ctx.fillRect(left, 0, width, height);
        });
      });
    }
  }

  class TrendlineBackgroundView {
    constructor(source) {
      this._source = source;
      this._renderer = new TrendlineBackgroundRenderer(source);
    }

    update() {}

    renderer() {
      return this._renderer;
    }

    zOrder() {
      return 'bottom';
    }
  }

  class TrendlinePaneView {
    constructor(source) {
      this._source = source;
      this._renderer = new TrendlinePaneRenderer(source);
    }

    update() {}

    renderer() {
      return this._renderer;
    }

    zOrder() {
      return 'normal';
    }
  }

  class TrendlinePrimitive {
    constructor() {
      this._lines = [];
      this._backgroundZones = [];
      this._linesVisible = true;
      this._backgroundVisible = true;
      this._chart = null;
      this._series = null;
      this._requestUpdate = null;
      this._paneViews = [
        new TrendlineBackgroundView(this),
        new TrendlinePaneView(this),
      ];
    }

    attached(param) {
      this._chart = param.chart;
      this._series = param.series;
      this._requestUpdate = param.requestUpdate;
    }

    detached() {
      this._chart = null;
      this._series = null;
      this._requestUpdate = null;
    }

    setData(lines) {
      this._lines = Array.isArray(lines) ? lines.slice() : [];
      this.requestUpdate();
    }

    setBackgroundZones(zones) {
      this._backgroundZones = Array.isArray(zones) ? zones.slice() : [];
      this.requestUpdate();
    }

    setLinesVisible(visible) {
      this._linesVisible = visible !== false;
      this.requestUpdate();
    }

    setBackgroundVisible(visible) {
      this._backgroundVisible = visible !== false;
      this.requestUpdate();
    }

    setVisible(visible) {
      this.setLinesVisible(visible);
    }

    paneViews() {
      return this._paneViews;
    }

    updateAllViews() {
      this._paneViews.forEach((view) => view.update());
    }

    requestUpdate() {
      this.updateAllViews();
      if (typeof this._requestUpdate === 'function') {
        this._requestUpdate();
      }
    }
  }

  global.TrendlinePrimitive = TrendlinePrimitive;
})(typeof window !== 'undefined' ? window : globalThis);
