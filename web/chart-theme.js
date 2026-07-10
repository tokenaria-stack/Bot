/**
 * ChartTheme — semantic color tokens for client-owned chart chrome.
 *
 * Server/DDR colors win when present: use ChartTheme.resolve(serverColor, ChartTheme.bull).
 * Plugins use ChartTheme.get('bull') — lazy runtime lookup (safe if theme loads first).
 * Call ChartTheme.applyCssVariables() on boot; re-call on theme switch (Light/Dark).
 */
const ChartTheme = {
  // ── Market direction ──────────────────────────────────────────────────────
  bull: '#089981',
  bear: '#f23645',

  // ── Trade primitives ────────────────────────────────────────────────────────
  long: '#9C27B0',
  short: '#AD1457',
  exit: '#FF9800',
  stopLoss: '#f23645',
  takeProfit: '#089981',
  buy: '#089981',
  sell: '#f23645',

  // ── RSX / divergence markers ───────────────────────────────────────────────
  rsxShort: '#b71c1c',
  rsxStrongShort: '#b71c1c',
  rsxLong: '#004d40',
  rsxStrongLong: '#004d40',
  rsxPivot: '#1565c0',
  regularDiv: '#2962ff',
  hiddenDiv: '#7b1fa2',
  rsxDefault: '#e1d2b5',
  rsxJurikDot: '#c8a882',

  // ── Volume spikes ───────────────────────────────────────────────────────────
  spikeUp: '#089981',
  spikeDown: '#f23645',

  // ── Navigator / trendline primitives ────────────────────────────────────────
  navHH: '#089981',
  navLL: '#f23645',
  navWickBreak: '#ff5d00',
  trendlineDefault: '#089981',
  trendlineZoneFill: 'rgba(8, 153, 129, 0.08)',
  trendlineCompletedAlpha: 0.55,

  // ── Chart chrome (TradingView dark) ─────────────────────────────────────────
  bg: '#131722',
  grid: '#1e222d',
  border: '#2a2e39',
  text: '#787b86',
  textBright: '#d1d4dc',
  accent: '#2962ff',
  cyan: '#00bcd4',
  gold: '#f7931a',
  fibGolden: '#e3b341',
  fibMuted: 'rgba(120,123,134,0.7)',
  crosshair: '#555555',
  crosshairMarkerBorder: '#90ee90',

  // ── Volume bars (semi-transparent OHLC direction) ─────────────────────────
  volumeUp: 'rgba(8,153,129,0.55)',
  volumeDown: 'rgba(242,54,69,0.55)',

  // ── Wozduh legacy line defaults (DDR manifest overrides at runtime) ─────────
  wozduhFast: 'blue',
  wozduhSlow: 'aqua',
  rsxSignalLine: '#ff9800',

  // ── MTF period accents (navigator overlays) ─────────────────────────────────
  mtfPeriodColors: {
    '1m': '#787b86',
    '3m': '#5b9cf6',
    '5m': '#2962ff',
    '15m': '#089981',
    '30m': '#00bcd4',
    '1h': '#9c27b0',
    '2h': '#e040fb',
    '4h': '#ff9800',
    '6h': '#ffb74d',
    '8h': '#ffa726',
    '12h': '#ff7043',
    '1d': '#f23645',
    '3d': '#e91e63',
    '1w': '#ab47bc',
    '1M': '#ab47bc',
  },

  _listeners: [],

  /**
   * Lazy token read — safe for plugins that render after theme init.
   * @param {string} token
   * @param {string} [hardFallback]
   */
  get(token, hardFallback) {
    if (token && Object.prototype.hasOwnProperty.call(ChartTheme, token)) {
      return ChartTheme[token];
    }
    return hardFallback ?? ChartTheme.text;
  },

  /**
   * Prefer dynamic server/DDR color; fall back to semantic token.
   */
  resolve(serverColor, fallbackToken) {
    if (serverColor != null && String(serverColor).trim() !== '') {
      return String(serverColor).trim();
    }
    if (fallbackToken && Object.prototype.hasOwnProperty.call(ChartTheme, fallbackToken)) {
      return ChartTheme[fallbackToken];
    }
    return fallbackToken || ChartTheme.text;
  },

  mtfPeriodColor(tf) {
    const key = String(tf || '').trim();
    const normalized = key === '1M' ? '1M' : key.toLowerCase();
    return ChartTheme.mtfPeriodColors[normalized] || ChartTheme.text;
  },

  navMarkerColor(type, serverColor) {
    const t = String(type || '').trim();
    if (t === 'HH') return ChartTheme.resolve(serverColor, 'navHH');
    if (t === 'LL') return ChartTheme.resolve(serverColor, 'navLL');
    if (t === 'WickBreak') return ChartTheme.resolve(serverColor, 'navWickBreak');
    return ChartTheme.resolve(serverColor, 'text');
  },

  trendlineStroke(line) {
    return ChartTheme.resolve(line?.color, 'trendlineDefault');
  },

  trendlineFill(zone) {
    return ChartTheme.resolve(zone?.color, 'trendlineZoneFill');
  },

  /** TV-compatible palette object for config.js / legacy callers. */
  palette() {
    return {
      bg: ChartTheme.bg,
      grid: ChartTheme.grid,
      border: ChartTheme.border,
      text: ChartTheme.text,
      green: ChartTheme.bull,
      red: ChartTheme.bear,
      blue: ChartTheme.accent,
      cyan: ChartTheme.cyan,
      gold: ChartTheme.gold,
    };
  },

  /**
   * Push semantic tokens to CSS custom properties (index.html / style.css).
   * Re-invoke after setMode('light'|'dark') in the future.
   */
  applyCssVariables(root = (typeof document !== 'undefined' ? document.documentElement : null)) {
    if (!root?.style) return;
    const vars = {
      '--theme-bull': ChartTheme.bull,
      '--theme-bear': ChartTheme.bear,
      '--theme-long': ChartTheme.long,
      '--theme-short': ChartTheme.short,
      '--theme-exit': ChartTheme.exit,
      '--theme-bg': ChartTheme.bg,
      '--theme-panel': ChartTheme.grid,
      '--theme-border': ChartTheme.border,
      '--theme-text': ChartTheme.textBright,
      '--theme-muted': ChartTheme.text,
      '--theme-accent': ChartTheme.accent,
      '--theme-gold': ChartTheme.gold,
      '--theme-fib': ChartTheme.fibGolden,
      '--tv-green': ChartTheme.bull,
      '--tv-red': ChartTheme.bear,
      '--tv-bg': ChartTheme.bg,
      '--tv-panel': ChartTheme.grid,
      '--tv-border': ChartTheme.border,
      '--tv-text': ChartTheme.textBright,
      '--tv-muted': ChartTheme.text,
      '--tv-blue': ChartTheme.accent,
      '--tv-cyan': ChartTheme.cyan,
      '--tv-gold': ChartTheme.gold,
      '--tv-accent': ChartTheme.accent,
    };
    for (const [name, value] of Object.entries(vars)) {
      root.style.setProperty(name, value);
    }
  },

  onChange(listener) {
    if (typeof listener === 'function') ChartTheme._listeners.push(listener);
  },

  _notify() {
    ChartTheme.applyCssVariables();
    ChartTheme._listeners.forEach((fn) => {
      try { fn(ChartTheme); } catch (err) { console.warn('[ChartTheme] listener error:', err); }
    });
  },

  rsxMarkerStyle(marker) {
    const m = String(marker || '').toUpperCase();
    if (m === 'S' || m === 'SS') {
      return {
        position: 'aboveBar',
        color: ChartTheme.rsxShort,
        shape: 'circle',
        size: 1,
      };
    }
    if (m === 'L' || m === 'LL') {
      return {
        position: 'belowBar',
        color: ChartTheme.rsxLong,
        shape: 'circle',
        size: 1,
      };
    }
    if (m === 'P') {
      return {
        position: 'belowBar',
        color: ChartTheme.rsxPivot,
        shape: 'circle',
        size: 1,
      };
    }
    return {
      position: 'belowBar',
      color: ChartTheme.regularDiv,
      shape: 'circle',
      size: 1,
    };
  },
};

if (typeof window !== 'undefined') {
  window.ChartTheme = ChartTheme;
  ChartTheme.applyCssVariables();
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { ChartTheme };
}
