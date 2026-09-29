/**
 * STAR-RESEARCH-OVERLAY-1.
 * Paints one selected Star from the certified row onto the existing price series.
 * Does not compute a swing, ATR, stop, entry, or R, and does not move the camera.
 */
const StarResearchOverlay = (() => {
  function chartSec(ms) {
    const n = Number(ms);
    if (!Number.isFinite(n) || n <= 0) return null;
    return Math.floor(n / 1000);
  }

  function finite(n) {
    const v = Number(n);
    return Number.isFinite(v) ? v : null;
  }

  /**
   * Presentation of one supplied row. Prices are copied, not derived from ATR.
   * @param {object|null} row
   */
  function researchPlan(row) {
    if (!row || !Number.isFinite(Number(row.decisionAt)) || Number(row.decisionAt) <= 0) {
      return { markers: [], lines: [], status: '' };
    }
    const status = typeof row.status === 'string' ? row.status : '';
    const side = row.side === 'down' ? 'down' : (row.side === 'up' ? 'up' : '');
    const markers = [];
    const lines = [];
    const entries = [];
    const entryColor = side === 'up' ? '#00E676' : (side === 'down' ? '#FF1744' : '');
    const starTime = chartSec(row.decisionAt);
    const markTime = starTime;
    if (starTime != null) {
      markers.push({
        time: starTime,
        position: side === 'down' ? 'aboveBar' : 'belowBar',
        color: '#f0b429',
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
        text: 'swing ' + swingWick,
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
    return { markers, lines, entries, status, markTime };
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

  class LineRenderer {
    constructor(owner) {
      this._owner = owner;
    }

    draw(target) {
      const series = this._owner._series;
      const lines = this._owner.lines || [];
      const entries = this._owner.entries || [];
    const markTime = this._owner.markTime;
    const crowd = this._owner.crowd || [];
    if (!series || !target || typeof target.useBitmapCoordinateSpace !== 'function') return;
    if (!lines.length && !entries.length && markTime == null && !crowd.length) return;
    target.useBitmapCoordinateSpace((scope) => {
        const ctx = scope.context;
        if (!ctx) return;
        const hr = scope.horizontalPixelRatio || 1;
        const vr = scope.verticalPixelRatio || 1;
        const width = scope.bitmapSize ? scope.bitmapSize.width : 0;
        const height = scope.bitmapSize ? scope.bitmapSize.height : 0;
        if (markTime != null) {
          let x = null;
          try { x = this._owner._chart.timeScale().timeToCoordinate(markTime); } catch { x = null; }
          if (x != null && Number.isFinite(x)) {
            ctx.beginPath();
            ctx.strokeStyle = 'rgba(212, 180, 131, 0.85)';
            ctx.lineWidth = Math.max(1, hr);
            ctx.moveTo(Math.round(x * hr), 0);
            ctx.lineTo(Math.round(x * hr), height);
            ctx.stroke();
          }
        }
        if (crowd.length) {
          const ts = this._owner._chart && typeof this._owner._chart.timeScale === 'function'
            ? this._owner._chart.timeScale() : null;
          if (ts && typeof ts.timeToCoordinate === 'function') {
            ctx.fillStyle = 'rgba(212, 180, 131, 0.55)';
            const r = Math.max(2, 2 * hr);
            const y = height - 8 * vr;
            for (let i = 0; i < crowd.length; i++) {
              const sec = Number(crowd[i].decisionAt);
              if (!Number.isFinite(sec) || sec <= 0) continue;
              const t = sec > 1e11 ? Math.floor(sec / 1000) : Math.floor(sec);
              let x = null;
              try { x = ts.timeToCoordinate(t); } catch { x = null; }
              if (x == null || !Number.isFinite(x)) continue;
              ctx.beginPath();
              ctx.arc(x * hr, y, r, 0, Math.PI * 2);
              ctx.fill();
            }
          }
        }
        ctx.font = (12 * vr) + 'px sans-serif';
        ctx.textAlign = 'right';
        ctx.textBaseline = 'bottom';
        for (let i = 0; i < lines.length; i++) {
          const line = lines[i];
          let y = null;
          try { y = series.priceToCoordinate(line.price); } catch { y = null; }
          if (y == null || !Number.isFinite(y)) continue;
          const py = Math.round(y * vr);
          ctx.beginPath();
          ctx.strokeStyle = line.color;
          ctx.lineWidth = Math.max(1, vr);
          ctx.setLineDash(Array.isArray(line.dash) ? line.dash.map((d) => d * hr) : []);
          ctx.moveTo(0, py);
          ctx.lineTo(width, py);
          ctx.stroke();
          ctx.setLineDash([]);
          ctx.fillStyle = line.color;
          ctx.fillText(line.title, Math.max(0, width - 8 * hr), py - 2 * vr);
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
      this._paneViews = [new LinePaneView(this)];
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

  function refresh() {
    if (typeof StarLens !== 'undefined' && typeof StarLens.noteViewport === 'function') {
      StarLens.noteViewport();
    }
    const row = deps.getRow();
    const onChart = deps.getTf() === '15m';
    const plan = onChart
      ? researchPlan(row)
      : { markers: [], lines: [], status: row && row.status ? String(row.status) : '' };
    primitive.lines = plan.lines;
    primitive.entries = plan.entries || [];
    primitive.markTime = plan.markTime == null ? null : plan.markTime;
    primitive.crowd = onChart && typeof StarLens !== 'undefined' && typeof StarLens.getMarks === 'function'
      ? StarLens.getMarks() : [];
    deps.applyMarkers(plan.markers);
    deps.attach(primitive);
    deps.requestDraw();
    const status = plan.status || '';
    deps.paintStatus(!row ? '' : (onChart ? status : (status ? status + ' · 15m' : '15m')));
    return plan;
  }

  return {
    researchPlan,
    refresh,
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
