/**
 * STRUCTURAL-STOP-1 overlay — paints a backend assignment on the price pane.
 * Does not recompute RSX, fractals, or stops.
 */
(function initStructuralStopOverlay(global) {
  'use strict';

  function sec(ms) {
    if (!Number.isFinite(ms) || ms <= 0) return null;
    return ms > 1e12 ? Math.floor(ms / 1000) : Math.floor(ms);
  }

  class Renderer {
    constructor(source) {
      this._source = source;
    }
    draw(target) {
      const src = this._source;
      const a = src._assignment;
      const chart = src._chart;
      const series = src._series;
      if (!a || !chart || !series) return;
      const ts = chart.timeScale();
      target.useMediaCoordinateSpace(({ context: ctx }) => {
        const xAt = (ms) => {
          const t = sec(ms);
          if (t == null) return null;
          return ts.timeToCoordinate(t);
        };
        const yAt = (px) => (Number.isFinite(px) ? series.priceToCoordinate(px) : null);
        const x0 = xAt(a.at);
        const xH = xAt(a.h_end_at);
        const yEntry = yAt(a.entry);
        const stroke = (x1, y1, x2, y2, color, dash) => {
          if (x1 == null || y1 == null || x2 == null || y2 == null) return;
          ctx.beginPath();
          ctx.strokeStyle = color;
          ctx.lineWidth = 1;
          ctx.setLineDash(dash || []);
          ctx.moveTo(x1, y1);
          ctx.lineTo(x2, y2);
          ctx.stroke();
          ctx.setLineDash([]);
        };
        const mark = (x, y, color, r) => {
          if (x == null || y == null) return;
          ctx.beginPath();
          ctx.fillStyle = color;
          ctx.arc(x, y, r || 4, 0, Math.PI * 2);
          ctx.fill();
        };
        const label = (x, y, text, color, dx, dy) => {
          if (x == null || y == null || !text) return;
          ctx.fillStyle = color;
          ctx.font = '11px Inter, sans-serif';
          ctx.fillText(text, x + (dx == null ? 6 : dx), y + (dy == null ? -4 : dy));
        };
        const canvasH = ctx.canvas ? ctx.canvas.height : 400;
        if (x0 != null && yEntry != null) {
          mark(x0, yEntry, '#2962ff', 5);
          label(x0, yEntry, 'ENTRY 50x', '#2962ff', 8, 14);
        }
        if (a.status === 'NO_STRUCTURE') {
          if (x0 != null && yEntry != null) label(x0, yEntry, 'NO STRUCTURE', '#ff1744', 8, 28);
          return;
        }
        if (a.status === 'INVALID_GEOMETRY') {
          if (x0 != null && yEntry != null) label(x0, yEntry, 'INVALID STRUCTURE', '#ff1744', 8, 28);
          return;
        }
        const xA = xAt(a.pivot_anchor_at);
        const xC = xAt(a.pivot_confirmed_at);
        const yStop = yAt(a.stop);
        if (xA != null && yStop != null) {
          mark(xA, yStop, '#2979ff', 4);
          label(xA, yStop, 'PIVOT AnchorAt', '#2979ff', 8, -8);
        }
        if (xC != null) {
          stroke(xC, 8, xC, canvasH - 8, '#80d8ff', [2, 3]);
          label(xC, 14, 'ConfirmedAt (time)', '#80d8ff', 6, 0);
        }
        const xRight = xH != null ? xH : x0;
        stroke(x0, yStop, xRight, yStop, '#f23645', []);
        label(xRight, yStop, src._wickOwner === 'price_k2' ? 'STOP +0.15 ATR' : 'STOP', '#f23645', -92, -8);
        if (src._wickOwner === 'price_k2' && Number.isFinite(a.wick) && a.wick !== a.stop) {
          const yW = yAt(a.wick);
          stroke(x0, yW, xRight, yW, '#787b86', [3, 3]);
          label(xRight, yW, 'S0 wick', '#787b86', -52, 12);
        }
        if (xH != null) {
          stroke(xH, 0, xH, canvasH, '#787b86', [4, 4]);
          label(xH, 28, 'H=72', '#787b86', 6, 0);
        }
        if (src._wickOwner === 'price_k2') {
          const levels = [
            [a.plus_1r, '+1R', '#089981'],
            [a.plus_2r, '+2R', '#26a69a'],
            [a.plus_3r, '+3R', '#80cbc4'],
          ];
          for (const [px, name, col] of levels) {
            const y = yAt(px);
            stroke(x0, y, xRight, y, col, [2, 2]);
            label(xRight, y, name, col, -52, -8);
          }
          const xMb = xAt(a.mfe_before_at);
          const yMb = yAt(a.mfe_before_price);
          const xMf = xAt(a.mfe_full_at);
          const yMf = yAt(a.mfe_full_price);
          const mfeSame = a.mfe_before_at && a.mfe_before_at === a.mfe_full_at
            && Number(a.mfe_before_price) === Number(a.mfe_full_price);
          const mfeClose = xMb != null && xMf != null && yMb != null && yMf != null
            && Math.hypot(xMb - xMf, yMb - yMf) < 16;
          if (mfeSame || mfeClose) {
            mark(xMb, yMb, '#aeea00', 5);
            label(xMb, yMb, 'MFE before = full H', '#aeea00', 8, -10);
          } else {
            mark(xMb, yMb, '#00e676', 4);
            label(xMb, yMb, 'MFE before stop', '#00e676', 8, -10);
            mark(xMf, yMf, '#c6ff00', 3);
            label(xMf, yMf, 'MFE full H', '#c6ff00', 8, 14);
          }
          if (a.stop_hit) mark(xAt(a.stop_hit_at), yStop, '#ff1744', 6);
          return;
        }
        if (xH != null) {
          stroke(xH, 0, xH, canvasH, '#787b86', [4, 4]);
          label(xH, 28, 'H=72', '#787b86', 6, 0);
        }
        const levels = [
          [a.plus_1r, '+1R', '#089981'],
          [a.plus_2r, '+2R', '#26a69a'],
          [a.plus_3r, '+3R', '#80cbc4'],
        ];
        for (const [px, name, col] of levels) {
          const y = yAt(px);
          stroke(x0, y, xRight, y, col, [2, 2]);
          label(xRight, y, name, col, -52, -8);
        }
        const xMb = xAt(a.mfe_before_at);
        const yMb = yAt(a.mfe_before_price);
        const xMf = xAt(a.mfe_full_at);
        const yMf = yAt(a.mfe_full_price);
        const mfeSame = a.mfe_before_at && a.mfe_before_at === a.mfe_full_at
          && Number(a.mfe_before_price) === Number(a.mfe_full_price);
        const mfeClose = xMb != null && xMf != null && yMb != null && yMf != null
          && Math.hypot(xMb - xMf, yMb - yMf) < 16;
        if (mfeSame || mfeClose) {
          mark(xMb, yMb, '#aeea00', 5);
          label(xMb, yMb, 'MFE before = full H', '#aeea00', 8, -10);
        } else {
          mark(xMb, yMb, '#00e676', 4);
          label(xMb, yMb, 'MFE before stop', '#00e676', 8, -10);
          mark(xMf, yMf, '#c6ff00', 3);
          label(xMf, yMf, 'MFE full H', '#c6ff00', 8, 14);
        }
        if (a.stop_hit) mark(xAt(a.stop_hit_at), yStop, '#ff1744', 6);
        if (src._wickOwner === 'price_k2') {
          return;
        }
        const sig = src._significance;
        const cols = ['#ff9100', '#e040fb', '#26c6da', '#ffee58'];
        if (sig && Array.isArray(sig.swings) && sig.swings.length) {
          sig.swings.forEach((sw, j) => {
            if (!sw) return;
            const x = xAt(sw.anchor_at);
            const y = yAt(sw.wick);
            const col = cols[j] || '#9e9e9e';
            mark(x, y, col, 4);
            stroke(x, y, xRight, y, col, [5, 4]);
            const lab = (sw.id || ('S' + j))
              + ' p=' + (Number.isFinite(+sw.prominence_atr) ? Number(sw.prominence_atr).toFixed(1) : '?')
              + ' R=' + (Number.isFinite(+sw.dist_atr) ? Number(sw.dist_atr).toFixed(1) : '?');
            label(x, y, lab, col, 8, j % 2 === 0 ? -12 : 16);
          });
        } else {
          const ps = src._priceSwing;
          const paintSwing = (sw, color, name, dy) => {
            if (!sw || !sw.found) return;
            const x = xAt(sw.anchor_at);
            const y = yAt(sw.wick);
            mark(x, y, color, 4);
            stroke(x, y, xRight, y, color, [5, 4]);
            label(x, y, name, color, 8, dy);
          };
          if (ps) {
            paintSwing(ps.p1, '#ff9100', 'P1 k=1 price', -12);
            paintSwing(ps.p2, '#e040fb', 'P2 k=2 price', 16);
          }
        }
      });
    }
  }

  class View {
    constructor(source) {
      this._source = source;
      this._renderer = new Renderer(source);
    }
    update() {}
    renderer() { return this._renderer; }
    zOrder() { return 'top'; }
  }

  class Primitive {
    constructor() {
      this._chart = null;
      this._series = null;
      this._assignment = null;
      this._priceSwing = null;
      this._significance = null;
      this._wickOwner = '';
      this._paneViews = [new View(this)];
    }
    attached(params) {
      this._chart = params.chart;
      this._series = params.series;
    }
    detached() {
      this._chart = null;
      this._series = null;
    }
    paneViews() { return this._paneViews; }
    setAssignment(a, ps, sig, owner) {
      this._assignment = a;
      this._priceSwing = ps || null;
      this._significance = sig || null;
      this._wickOwner = owner || '';
      if (this._series && typeof this._series.applyOptions === 'function') {
        this._series.applyOptions({});
      }
    }
  }

  let primitive = null;
  let panel = null;

  function ensurePanel() {
    if (panel) return panel;
    panel = document.createElement('div');
    panel.id = 'structural-stop-panel';
    panel.style.cssText = 'position:absolute;z-index:40;top:8px;left:8px;background:#1e222d;border:1px solid #2a2e39;color:#d1d4dc;padding:8px 10px;font:12px Inter,sans-serif;max-width:420px;';
    panel.innerHTML = '<div id="ss1-body">STRUCTURAL-STOP-1</div><div style="margin-top:6px;"><button type="button" id="ss1-prev">Prev [</button> <button type="button" id="ss1-next">Next ]</button></div>';
    document.body.appendChild(panel);
    return panel;
  }

  const Review = {
    i: 0,
    n: 0,
    loaded: false,
    attach(chart, series) {
      if (!chart || !series || typeof series.attachPrimitive !== 'function') return;
      if (primitive) {
        try { series.detachPrimitive(primitive); } catch (_) { /* */ }
      }
      primitive = new Primitive();
      series.attachPrimitive(primitive);
      this._chart = chart;
      this._series = series;
    },
    setAssignment(a, ps, sig, owner) {
      if (primitive) primitive.setAssignment(a, ps, sig, owner);
      if (this._chart && a && a.at && a.h_end_at) {
        let lookback = 80;
        if (Number.isFinite(+a.anchor_age_bars) && a.anchor_age_bars + 8 > lookback) {
          lookback = a.anchor_age_bars + 8;
        }
        const from = sec(a.at) - lookback * 900;
        const to = sec(a.h_end_at) + 10 * 900;
        try {
          this._chart.timeScale().setVisibleRange({ from, to });
        } catch (_) { /* */ }
      }
    },
    async loadSample(i) {
      const res = await fetch('/api/research/structural-stop-1?sample=' + i);
      if (!res.ok) return;
      const body = await res.json();
      this.i = body.sample_i;
      this.n = body.sample_count;
      const a = body.assignment;
      ensurePanel();
      const el = document.getElementById('ss1-body');
      const tf = String(global.currentTf || '');
      if (tf && tf !== '15m') {
        if (el) {
          el.textContent = 'STRUCTURAL-STOP-1 needs the 15m chart. Switch TF, then Next.';
        }
        return;
      }
      if (typeof global.seekHistoryIsland === 'function' && a && a.at) {
        await global.seekHistoryIsland(a.at, 140);
      }
      this.setAssignment(a, body.price_swing, body.significance, body.wick_owner);
      if (el && a) {
        if (body.wick_owner === 'price_k2') {
            el.textContent = 'STOP-2  LONG ' + (this.i + 1) + '/' + this.n
              + '  ' + (a.status || '')
              + '  red = S0 + 0.15 ATR15(entry)  grey = S0 wick'
              + '  R/ATR15=' + (a.r_over_atr15 != null ? Number(a.r_over_atr15).toFixed(2) : 'n/a');
        } else {
          const sig = body.significance;
          const parts = (sig && Array.isArray(sig.swings) ? sig.swings : []).map((sw) => {
            return (sw.id || '') + ' p=' + Number(sw.prominence_atr).toFixed(1)
              + ' R=' + Number(sw.dist_atr).toFixed(1);
          });
          el.textContent = 'LONG ' + (this.i + 1) + '/' + this.n
            + (sig && sig.disputed ? ' DISPUTED' : (sig && sig.control ? ' CONTROL' : ''))
            + '  ' + (parts.join(' | ') || 'no S-swings')
            + '  THIS ONE? A=S0 B=S1 C=prominence D=origin';
        }
      }
    },
    async start() {
      const meta = await fetch('/api/research/structural-stop-1');
      if (!meta.ok) return;
      const body = await meta.json();
      this.n = body.sample_count || 0;
      if (!this.n) return;
      ensurePanel();
      const el = document.getElementById('ss1-body');
      if (el) {
        el.textContent = 'STOP-2 frozen: S0 wick + 0.15 ATR15(entry). 15m Next / ].';
      }
      document.getElementById('ss1-prev').onclick = () => this.step(-1);
      document.getElementById('ss1-next').onclick = () => this.step(1);
      document.addEventListener('keydown', (e) => {
        if (e.key === '[') this.step(-1);
        if (e.key === ']') this.step(1);
      });
    },
    step(dir) {
      if (!this.loaded) {
        this.loaded = true;
        return this.loadSample(0);
      }
      const next = Math.max(0, Math.min(this.n - 1, this.i + dir));
      return this.loadSample(next);
    },
  };

  global.StructuralStopOverlay = Review;
})(typeof window !== 'undefined' ? window : globalThis);
