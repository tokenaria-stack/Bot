/**
 * STAR-RESEARCH-OVERLAY-1 / STAR-CHART-VISUAL-DISPLAY-V1.
 * Selected Star and path from the certified row. Population glyphs are canvas.
 * Does not compute a swing, ATR, stop, entry, or R, and does not move the camera.
 */
const StarResearchOverlay = (() => {
  const STAR_UP = '#00E676';
  const STAR_DOWN = '#FF1744';
  const STAR_SELECTED = '#f0b429';
  const WINDOW_BARS = 96;
  const BAR_SEC = 15 * 60;
  const OUTLINE = '#111111';

  function chartSec(ms) {
    const n = Number(ms);
    if (!Number.isFinite(n) || n <= 0) return null;
    return Math.floor(n / 1000);
  }

  function finite(n) {
    const v = Number(n);
    return Number.isFinite(v) ? v : null;
  }

  function pathWindowEndSec(decisionAt) {
    const t = chartSec(decisionAt);
    if (t == null) return null;
    return t + WINDOW_BARS * BAR_SEC;
  }

  function pathSpan(row) {
    if (!row) return { fromSec: null, toSec: null };
    const star = chartSec(row.decisionAt);
    const swing = chartSec(row.swingAt);
    return {
      fromSec: swing != null ? swing : star,
      toSec: pathWindowEndSec(row.decisionAt),
    };
  }

  function paintPrefs() {
    if (typeof StarPopulationDisplay !== 'undefined' && typeof StarPopulationDisplay.get === 'function') {
      return StarPopulationDisplay.get();
    }
    return { shape: 'star4', size: 9, upColor: STAR_UP, downColor: STAR_DOWN };
  }

  /**
   * Same four-pointed sparkle as WozduhCrossovers.pathStar4.
   */
  function pathStar4(ctx, x, y, r) {
    if (typeof WozduhCrossovers !== 'undefined' && typeof WozduhCrossovers.pathStar4 === 'function') {
      WozduhCrossovers.pathStar4(ctx, x, y, r);
      return;
    }
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

  function pathDiamond(ctx, x, y, r) {
    ctx.beginPath();
    ctx.moveTo(x, y - r);
    ctx.lineTo(x + r, y);
    ctx.lineTo(x, y + r);
    ctx.lineTo(x - r, y);
    ctx.closePath();
  }

  function pathTriangle(ctx, x, y, r, down) {
    const h = r * Math.sqrt(3);
    ctx.beginPath();
    if (down) {
      ctx.moveTo(x, y + (2 / 3) * h);
      ctx.lineTo(x + r, y - (1 / 3) * h);
      ctx.lineTo(x - r, y - (1 / 3) * h);
    } else {
      ctx.moveTo(x, y - (2 / 3) * h);
      ctx.lineTo(x + r, y + (1 / 3) * h);
      ctx.lineTo(x - r, y + (1 / 3) * h);
    }
    ctx.closePath();
  }

  function pathArrow(ctx, x, y, r, down) {
    const h = r * 1.6;
    ctx.beginPath();
    if (down) {
      ctx.moveTo(x, y + h);
      ctx.lineTo(x - r, y);
      ctx.lineTo(x - r * 0.35, y);
      ctx.lineTo(x - r * 0.35, y - h);
      ctx.lineTo(x + r * 0.35, y - h);
      ctx.lineTo(x + r * 0.35, y);
      ctx.lineTo(x + r, y);
    } else {
      ctx.moveTo(x, y - h);
      ctx.lineTo(x - r, y);
      ctx.lineTo(x - r * 0.35, y);
      ctx.lineTo(x - r * 0.35, y + h);
      ctx.lineTo(x + r * 0.35, y + h);
      ctx.lineTo(x + r * 0.35, y);
      ctx.lineTo(x + r, y);
    }
    ctx.closePath();
  }

  function strokeFill(ctx, fill, x, y, r, draw) {
    draw(ctx, x, y, r);
    ctx.fillStyle = fill;
    ctx.fill();
    ctx.lineWidth = 1;
    ctx.strokeStyle = OUTLINE;
    draw(ctx, x, y, r);
    ctx.stroke();
  }

  /**
   * Presentation of one supplied row. Prices are copied, not derived from ATR.
   * @param {object|null} row
   */
  function researchPlan(row) {
    const span = pathSpan(row);
    if (!row || !Number.isFinite(Number(row.decisionAt)) || Number(row.decisionAt) <= 0) {
      return { markers: [], lines: [], status: '', fromSec: span.fromSec, toSec: span.toSec };
    }
    const status = typeof row.status === 'string' ? row.status : '';
    const side = row.side === 'down' ? 'down' : (row.side === 'up' ? 'up' : '');
    const markers = [];
    const lines = [];
    const entries = [];
    const entryColor = side === 'up' ? STAR_UP : (side === 'down' ? STAR_DOWN : '');
    const starTime = chartSec(row.decisionAt);
    const markTime = starTime;
    if (starTime != null) {
      markers.push({
        time: starTime,
        position: side === 'down' ? 'aboveBar' : 'belowBar',
        color: STAR_SELECTED,
        shape: side === 'down' ? 'arrowDown' : 'arrowUp',
        text: 'Star',
      });
    }
    const swingTime = chartSec(row.swingAt);
    const swingWick = finite(row.swingWick);
    if (swingTime != null && swingWick != null) {
      markers.push({
        time: swingTime,
        position: side === 'down' ? 'aboveBar' : 'belowBar',
        color: '#5b8def',
        shape: 'circle',
      });
    }
    const entryTime = chartSec(row.entryAt);
    const entry = finite(row.entryPrice);
    if (entryTime != null && entry != null && entryColor) {
      entries.push({
        time: entryTime,
        price: entry,
        dir: side,
        color: entryColor,
        shape: side === 'down' ? 'triangleDown' : 'triangleUp',
      });
      lines.push({
        id: 'entry',
        price: entry,
        color: entryColor,
        title: 'entry',
        dash: [6, 4],
      });
    }
    const stop = finite(row.stopPrice);
    if (status === 'valid' && entry != null && stop != null && (side === 'up' || side === 'down')) {
      lines.push({
        id: 'stop',
        price: stop,
        color: '#ff5a6a',
        title: 'stop',
        dash: [],
      });
      const distance = Math.abs(entry - stop);
      const dir = side === 'up' ? 1 : -1;
      for (let n = 1; n <= 3; n++) {
        lines.push({
          id: 'r' + n,
          price: entry + dir * n * distance,
          color: n === 1 ? '#c9a227' : (n === 2 ? '#e0c36a' : '#f3e2a8'),
          title: n + 'R',
          dash: [2, 4],
        });
      }
    }
    markers.sort((a, b) => a.time - b.time);
    return {
      markers,
      lines,
      entries,
      status,
      markTime,
      fromSec: span.fromSec,
      toSec: span.toSec,
    };
  }

  /**
   * Population members for canvas. Y is resolved from the candle at draw time.
   */
  function populationPlan(members, prefs) {
    const paint = prefs || paintPrefs();
    if (!Array.isArray(members)) return [];
    const markers = [];
    for (let i = 0; i < members.length; i++) {
      const row = members[i];
      const time = chartSec(row && row.decisionAt);
      if (time == null) continue;
      const down = row.side === 'down';
      markers.push({
        time: time,
        side: down ? 'down' : 'up',
        color: down ? paint.downColor : paint.upColor,
        shape: paint.shape,
      });
    }
    return markers;
  }

  function drawEntryTriangles(ctx, series, chart, entries, hr, vr) {
    const ts = chart && typeof chart.timeScale === 'function' ? chart.timeScale() : null;
    if (!ts || typeof ts.timeToCoordinate !== 'function' || !entries.length) return;
    const hw = 3.5 * hr;
    const hh = 4.5 * vr;
    for (let i = 0; i < entries.length; i++) {
      const mark = entries[i];
      let x = null;
      let y = null;
      try { x = ts.timeToCoordinate(mark.time); } catch { x = null; }
      try { y = series.priceToCoordinate(mark.price); } catch { y = null; }
      if (x == null || y == null || !Number.isFinite(x) || !Number.isFinite(y)) continue;
      const px = x * hr;
      const py = y * vr;
      ctx.beginPath();
      ctx.fillStyle = mark.color;
      if (mark.dir === 'down') {
        ctx.moveTo(px, py + hh);
        ctx.lineTo(px - hw, py - hh);
        ctx.lineTo(px + hw, py - hh);
      } else {
        ctx.moveTo(px, py - hh);
        ctx.lineTo(px - hw, py + hh);
        ctx.lineTo(px + hw, py + hh);
      }
      ctx.closePath();
      ctx.fill();
    }
  }

  function drawPopulation(ctx, series, chart, glyphs, hr, vr) {
    const ts = chart && typeof chart.timeScale === 'function' ? chart.timeScale() : null;
    if (!ts || typeof ts.timeToCoordinate !== 'function' || !glyphs.length) return;
    const paint = paintPrefs();
    const r = Math.max(2, Number(paint.size) / 2) * Math.min(hr, vr);
    for (let i = 0; i < glyphs.length; i++) {
      const mark = glyphs[i];
      const candle = deps.candleAt(mark.time);
      const price = candle
        ? (mark.side === 'down' ? finite(candle.high) : finite(candle.low))
        : null;
      if (price == null) continue;
      let x = null;
      let y = null;
      try { x = ts.timeToCoordinate(mark.time); } catch { x = null; }
      try { y = series.priceToCoordinate(price); } catch { y = null; }
      if (x == null || y == null || !Number.isFinite(x) || !Number.isFinite(y)) continue;
      const px = x * hr;
      const py = y * vr;
      const down = mark.side === 'down';
      const shape = mark.shape;
      const fill = mark.color;
      if (shape === 'arrow') {
        strokeFill(ctx, fill, px, py, r, (c, xx, yy, rr) => pathArrow(c, xx, yy, rr, down));
      } else if (shape === 'triangle') {
        strokeFill(ctx, fill, px, py, r, (c, xx, yy, rr) => pathTriangle(c, xx, yy, rr, down));
      } else if (shape === 'square') {
        strokeFill(ctx, fill, px, py, r, pathSquare);
      } else if (shape === 'diamond') {
        strokeFill(ctx, fill, px, py, r, pathDiamond);
      } else if (shape === 'circle') {
        strokeFill(ctx, fill, px, py, r, pathCircle);
      } else {
        strokeFill(ctx, fill, px, py, r, pathStar4);
      }
    }
  }

  class LineRenderer {
    constructor(owner) {
      this._owner = owner;
    }

    draw(target) {
      const series = this._owner._series;
      const lines = this._owner.lines || [];
      const entries = this._owner.entries || [];
      const glyphs = this._owner.glyphs || [];
      const markTime = this._owner.markTime;
      if (!series || !target || typeof target.useBitmapCoordinateSpace !== 'function') return;
      if (!lines.length && !entries.length && !glyphs.length && markTime == null) return;
      target.useBitmapCoordinateSpace((scope) => {
        const ctx = scope.context;
        if (!ctx) return;
        const hr = scope.horizontalPixelRatio || 1;
        const vr = scope.verticalPixelRatio || 1;
        const height = scope.bitmapSize ? scope.bitmapSize.height : 0;
        const ts = this._owner._chart && typeof this._owner._chart.timeScale === 'function'
          ? this._owner._chart.timeScale() : null;
        if (markTime != null && ts && typeof ts.timeToCoordinate === 'function') {
          let x = null;
          try { x = ts.timeToCoordinate(markTime); } catch { x = null; }
          if (x != null && Number.isFinite(x)) {
            ctx.beginPath();
            ctx.strokeStyle = 'rgba(212, 180, 131, 0.85)';
            ctx.lineWidth = Math.max(1, hr);
            ctx.moveTo(Math.round(x * hr), 0);
            ctx.lineTo(Math.round(x * hr), height);
            ctx.stroke();
          }
        }
        drawPopulation(ctx, series, this._owner._chart, glyphs, hr, vr);
        drawEntryTriangles(ctx, series, this._owner._chart, entries, hr, vr);
        let x0 = null;
        let x1 = null;
        if (ts && typeof ts.timeToCoordinate === 'function') {
          try { x0 = ts.timeToCoordinate(this._owner.fromSec); } catch { x0 = null; }
          try { x1 = ts.timeToCoordinate(this._owner.toSec); } catch { x1 = null; }
        }
        const haveSpan = x0 != null && x1 != null && Number.isFinite(x0) && Number.isFinite(x1);
        ctx.font = (12 * vr) + 'px sans-serif';
        ctx.textAlign = 'right';
        ctx.textBaseline = 'bottom';
        for (let i = 0; i < lines.length; i++) {
          const line = lines[i];
          let y = null;
          try { y = series.priceToCoordinate(line.price); } catch { y = null; }
          if (y == null || !Number.isFinite(y) || !haveSpan) continue;
          const py = Math.round(y * vr);
          const left = Math.round(Math.min(x0, x1) * hr);
          const right = Math.round(Math.max(x0, x1) * hr);
          ctx.beginPath();
          ctx.strokeStyle = line.color;
          ctx.lineWidth = Math.max(1, vr);
          ctx.setLineDash(Array.isArray(line.dash) ? line.dash.map((d) => d * hr) : []);
          ctx.moveTo(left, py);
          ctx.lineTo(right, py);
          ctx.stroke();
          ctx.setLineDash([]);
          ctx.fillStyle = line.color;
          ctx.fillText(line.title, right, py - 2 * vr);
        }
      });
    }
  }

  class LinePaneView {
    constructor(owner) {
      this._owner = owner;
      this._renderer = new LineRenderer(owner);
    }

    renderer() {
      return this._renderer;
    }

    zOrder() {
      return 'top';
    }

    update() {}
  }

  class ResearchLinePrimitive {
    constructor() {
      this._chart = null;
      this._series = null;
      this._requestUpdate = null;
      this.lines = [];
      this.entries = [];
      this.glyphs = [];
      this.fromSec = null;
      this.toSec = null;
      this._paneViews = [new LinePaneView(this)];
    }

    attached(param) {
      this._chart = param && param.chart ? param.chart : null;
      this._series = param && param.series ? param.series : null;
      this._requestUpdate = param && param.requestUpdate ? param.requestUpdate : null;
      if (this._chart && typeof this._chart.timeScale === 'function' && !this._rangeHook) {
        const ts = this._chart.timeScale();
        if (ts && typeof ts.subscribeVisibleTimeRangeChange === 'function') {
          this._rangeHook = () => {
            if (typeof StarLens !== 'undefined' && typeof StarLens.noteViewport === 'function') {
              StarLens.noteViewport();
            } else {
              syncPopulation();
            }
            this.requestUpdate();
          };
          ts.subscribeVisibleTimeRangeChange(this._rangeHook);
        }
      }
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

    requestUpdate() {
      if (typeof this._requestUpdate === 'function') this._requestUpdate();
    }
  }

  const primitive = new ResearchLinePrimitive();

  const deps = {
    getRow() {
      if (typeof StarResearch === 'undefined' || typeof StarResearch.getState !== 'function') return null;
      const state = StarResearch.getState();
      return state && state.row ? state.row : null;
    },
    getTf() {
      if (typeof window === 'undefined' || window.currentTf == null) return '';
      return String(window.currentTf);
    },
    applyMarkers(markers) {
      if (typeof ChartAdapter === 'undefined' || typeof ChartAdapter.applyResearchMarkers !== 'function') return false;
      return ChartAdapter.applyResearchMarkers(markers);
    },
    attach(target) {
      if (typeof ChartAdapter === 'undefined' || typeof ChartAdapter.attachResearchPrimitive !== 'function') return false;
      return ChartAdapter.attachResearchPrimitive(target);
    },
    candleAt(timeSec) {
      if (typeof ChartAdapter !== 'undefined' && typeof ChartAdapter.candleHighLowAt === 'function') {
        return ChartAdapter.candleHighLowAt(timeSec);
      }
      return null;
    },
    requestDraw() {
      primitive.requestUpdate();
    },
    paintStatus(text) {
      if (typeof document === 'undefined') return;
      const el = document.getElementById('star-research-status');
      if (el) el.textContent = text || '';
    },
  };

  function bind(next) {
    if (!next) return;
    Object.keys(next).forEach((key) => {
      if (typeof next[key] === 'function') deps[key] = next[key];
    });
  }

  function noteUnresolved() {
    deps.paintStatus('not a star');
  }

  function syncPopulation() {
    const onChart = deps.getTf() === '15m';
    const crowd = onChart && typeof StarLens !== 'undefined' && typeof StarLens.getMarks === 'function'
      ? StarLens.getMarks() : [];
    primitive.glyphs = onChart ? populationPlan(crowd) : [];
    return primitive.glyphs;
  }

  function refresh() {
    if (typeof StarLens !== 'undefined' && typeof StarLens.noteViewport === 'function') {
      StarLens.noteViewport();
    }
    const row = deps.getRow();
    const onChart = deps.getTf() === '15m';
    const plan = onChart
      ? researchPlan(row)
      : { markers: [], lines: [], status: row && row.status ? String(row.status) : '' };
    primitive.lines = plan.lines || [];
    primitive.entries = plan.entries || [];
    primitive.markTime = plan.markTime == null ? null : plan.markTime;
    primitive.fromSec = plan.fromSec == null ? null : plan.fromSec;
    primitive.toSec = plan.toSec == null ? null : plan.toSec;
    syncPopulation();
    deps.applyMarkers(plan.markers || []);
    deps.attach(primitive);
    deps.requestDraw();
    const status = plan.status || '';
    deps.paintStatus(!row ? '' : (onChart ? status : (status ? status + ' · 15m' : '15m')));
    return plan;
  }

  return {
    researchPlan,
    populationPlan,
    pathWindowEndSec,
    pathSpan,
    WINDOW_BARS,
    BAR_SEC,
    STAR_UP,
    STAR_DOWN,
    refresh,
    syncPopulation,
    noteUnresolved,
    bind,
    primitive,
  };
})();

if (typeof StarResearch !== 'undefined' && typeof StarResearch.onSelected === 'function') {
  StarResearch.onSelected(() => { StarResearchOverlay.refresh(); });
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = StarResearchOverlay;
}
