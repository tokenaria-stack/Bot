/**
 * ChartTheme — semantic color tokens for client-owned chart chrome.
 *
 * Server/DDR colors win when present: use ChartTheme.resolve(serverColor, ChartTheme.bull).
 * ChartTheme covers fallbacks, UI chrome, and client-derived markers (spikes, trades).
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

  // ── Volume spikes ───────────────────────────────────────────────────────────
  spikeUp: '#089981',
  spikeDown: '#f23645',

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

  // ── Volume bars (semi-transparent OHLC direction) ─────────────────────────
  volumeUp: 'rgba(8,153,129,0.55)',
  volumeDown: 'rgba(242,54,69,0.55)',

  // ── Wozduh legacy line defaults (DDR manifest overrides at runtime) ─────────
  wozduhFast: 'blue',
  wozduhSlow: 'aqua',

  /**
   * Prefer dynamic server/DDR color; fall back to semantic token.
   * @param {string|undefined|null} serverColor
   * @param {string} fallbackToken — ChartTheme key or raw hex
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
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { ChartTheme };
}
