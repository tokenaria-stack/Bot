/**
 * Minimal trade entry/exit markers for Lightweight Charts v4+ (ISeriesPrimitive).
 * Horizontal triangles point at exact entry price; exits render as orange dots.
 */
(function initTradeMarkerPlugin(global) {
  function themeColors() {
    const T = global.ChartTheme;
    return {
      long: T?.long ?? '#9C27B0',
      short: T?.short ?? '#AD1457',
      exit: T?.exit ?? '#FF9800',
    };
  }

  class TradeMarkerPaneRenderer {
    constructor(source) {
      this._source = source;
    }

    draw(target) {
      const source = this._source;
      if (!source._visible) return;
      const chart = source._chart;
      const series = source._series;
      const markers = source._markers;
      if (!chart || !series || !markers?.length) return;

      const timeScale = chart.timeScale();

      target.useMediaCoordinateSpace(({ context: ctx }) => {
        const COLORS = themeColors();
        markers.forEach((m) => {
          const x = timeScale.timeToCoordinate(m.time);
          const y = series.priceToCoordinate(m.price);
          if (x == null || y == null) return;

          if (m.kind === 'exit') {
            ctx.beginPath();
            ctx.fillStyle = COLORS.exit;
            ctx.arc(x, y, 3.5, 0, Math.PI * 2);
            ctx.fill();
            return;
          }

          const isLong = m.kind === 'long';
          const color = isLong ? COLORS.long : COLORS.short;
          const offset = isLong ? -10 : 10;
          const baseX = x + offset;

          ctx.beginPath();
          ctx.fillStyle = color;
          if (isLong) {
            ctx.moveTo(x, y);
            ctx.lineTo(baseX, y - 5);
            ctx.lineTo(baseX, y + 5);
          } else {
            ctx.moveTo(x, y);
            ctx.lineTo(baseX, y - 5);
            ctx.lineTo(baseX, y + 5);
          }
          ctx.closePath();
          ctx.fill();
        });
      });
    }
  }

  class TradeMarkerPaneView {
    constructor(source) {
      this._source = source;
      this._renderer = new TradeMarkerPaneRenderer(source);
    }

    update() {}

    renderer() {
      return this._renderer;
    }

    zOrder() {
      return 'top';
    }
  }

  class TradeMarkerPrimitive {
    constructor() {
      this._markers = [];
      this._visible = true;
      this._chart = null;
      this._series = null;
      this._requestUpdate = null;
      this._paneViews = [new TradeMarkerPaneView(this)];
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

    setData(markers) {
      this._markers = Array.isArray(markers) ? markers.slice() : [];
      this.requestUpdate();
    }

    setVisible(visible) {
      this._visible = visible !== false;
      this.requestUpdate();
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

  global.TradeMarkerPrimitive = TradeMarkerPrimitive;
})(typeof window !== 'undefined' ? window : globalThis);
