/** Declarative configuration — styles, limits, selectors, defaults. */
/* ── TradingView palette (sourced from ChartTheme) ── */
const TV = (typeof ChartTheme !== 'undefined' && ChartTheme.palette)
  ? ChartTheme.palette()
  : {
    bg: '#131722',
    grid: '#1e222d',
    border: '#2a2e39',
    text: '#787b86',
    green: '#089981',
    red: '#f23645',
    blue: '#2962ff',
    cyan: '#00bcd4',
    gold: '#f7931a',
  };

const THRESHOLDS_KEY = 'bot_thresholds';
const SCORING_MATRIX_KEY = 'bot_scoring_matrix';
const LS_LIVE_STRATEGY_KEY = 'dashboard_live_strategy';
const LS_BT_STRATEGY_KEY = 'dashboard_backtest_strategy';

const DEFAULT_STRATEGY_THRESHOLDS = { long: 70, short: 70 };

const SCORING_MATRIX_DEFAULTS = {
  useRSX: false,
  useWozduhCross: false,
  useTrendlines: false,
  useJurikTrend: false,
  useWozduhSpike: false,
};

// Phase F: scoring matrix UI purged — labels retained only as inert constants if referenced.
const SCORING_MATRIX_LABELS = [];

const LS_NAV_SETTINGS_PREFIX = 'dashboard_nav_settings_';
const LS_NAV_SETTINGS_LIVE_PREFIX = 'dashboard_nav_settings_live_';
const LS_NAV_POPUP_POS_PREFIX = 'dashboard_nav_popup_pos_';
const LS_NAV_DEFAULTS_PREFIX = 'nav_defaults_';

const NAVIGATOR_PANES = ['price', 'rsx', 'wozduh'];

const NAVIGATOR_SOURCE_MAP = {
  price: 'Price',
  rsx: 'RSX',
  wozduh: 'Wozduh',
};

const CHART_LEGEND_DEFS = {
  price: [{ id: 'price', label: 'Price', kind: 'price' }],
  wozduh: [{ id: 'wozduh', label: 'Wozd', kind: 'wozduh' }],
  rsx: [{ id: 'rsx', label: 'RSX', kind: 'rsx' }],
};
function defaultNavigatorPaneSettings(pane = 'price') {
  return {
    targetPrice: pane === 'price',
    targetRSX: false,
    targetWozduh: false,
    periods: [],
    trendType: 'Wicks',
    useLong: true,
    longLen: 60,
    useMedium: true,
    mediumLen: 30,
    useShort: true,
    shortLen: 10,
    term: 'Long',
    hhll: 'None',
    momentumEnabled: false,
    momentumBars: 14,
    momentumPercent: 100,
    timeHoldEnabled: false,
    timeHoldBars: 2,
    backgroundColor: false,
    barColor: false,
    linesVisible: true,
    backgroundVisible: true,
  };
}

function defaultRiskSettings() {
  // Phase F socket — risk settings purged with trading FSM.
  return null;
}

const NATIVE_BINANCE_TFS = [
  '1m', '3m', '5m', '15m', '30m',
  '1h', '2h', '4h', '6h', '8h', '12h',
  '1d', '1w', '1M',
];

const LIVE_CHART_TFS = [
  '1s', '5s', '10s', '15s', '30s', '45s',
  '1m', '2m', '3m', '5m', '10m', '15m', '30m', '45m',
  '1h', '2h', '3h', '4h', '6h', '8h', '12h',
  '1d', '1w', '1M',
];

/** Native + derived time bars are dense. Seconds and ticks are sparse (no-trade buckets are legal). */
function requiresDenseTimeContinuity(tf) {
  const id = String(tf || '').trim();
  if (!id) return true;
  if (/tick/i.test(id)) return false;
  if (/^\d+s$/i.test(id)) return false;
  return true;
}

function isSparseLiveChart(tf) {
  return typeof requiresDenseTimeContinuity === 'function'
    && !requiresDenseTimeContinuity(tf);
}

/** Activated 1s live series (micro_klines). Not 5s–45s, not ticks. */
function isLiveSecondChart(tf) {
  return String(tf || '').trim() === '1s';
}

/** Folded sparse-second children of 1s. Not 1s. Do not widen isLiveSecondChart. */
function isSparseSecondChart(tf) {
  switch (String(tf || '').trim()) {
    case '5s':
    case '10s':
    case '15s':
    case '30s':
    case '45s':
      return true;
    default:
      return false;
  }
}

/** Activated seconds family: 1s plus folded 5s–45s. Not native minutes. */
function isSecondsFamilyChart(tf) {
  return isLiveSecondChart(tf) || isSparseSecondChart(tf);
}

/** Interval duration in seconds for the seconds family. Null if not in family. */
function secondsFamilyIntervalSec(tf) {
  if (!isSecondsFamilyChart(tf)) return null;
  const m = /^(\d+)s$/i.exec(String(tf || '').trim());
  if (!m) return null;
  const n = Number(m[1]);
  return Number.isFinite(n) && n > 0 ? n : null;
}

/**
 * SECONDS-TF-SWITCH: HISTORY + smaller→bigger interval → LIVE tail.
 * HISTORY + bigger→smaller stays HISTORY (microscope). LIVE seed unchanged.
 * Native TFs: no-op. Does not convert wall-time span.
 */
function applySecondsFamilyTfSwitchIntent(seed, prevTf, nextTf) {
  if (!seed || seed.intent !== 'HISTORY') return seed;
  const prevIv = secondsFamilyIntervalSec(prevTf);
  const nextIv = secondsFamilyIntervalSec(nextTf);
  if (prevIv == null || nextIv == null) return seed;
  if (nextIv > prevIv) {
    return { ...seed, intent: 'LIVE', isAtRightEdge: true };
  }
  return seed;
}

/** Seconds bucket identity (one bar per OpenTime). Not ticks. */
function isSecondsTimeframe(tf) {
  return /^\d+s$/i.test(String(tf || '').trim());
}

/**
 * Shot 10B buffer append. Seconds: same OpenTime replaces the forming update.
 * Ticks must not use this coalesce (identity is not OpenTime).
 * @param {object[]} pending
 * @param {object} tick
 * @param {number} max
 * @param {string} [tf]
 * @returns {object[]}
 */
function appendLiveTickBuffer(pending, tick, max, tf) {
  const list = Array.isArray(pending) ? pending : [];
  const cap = Number.isFinite(max) && max > 0 ? Math.floor(max) : 5000;
  if (!tick) return list;
  const idTf = String(tick.timeframe || tf || '');
  const open = Number(tick.time);
  if (isSecondsTimeframe(idTf) && list.length && Number.isFinite(open)) {
    const lastOpen = Number(list[list.length - 1]?.time);
    if (lastOpen === open) {
      list[list.length - 1] = tick;
      return list;
    }
  }
  list.push(tick);
  if (list.length > cap) list.splice(0, list.length - cap);
  return list;
}

/** MICRO-2C: one authoritative full paint when sparse VIEW returns to LIVE. */
function sparseHistoryToLiveNeedsFullPaint(prevIntent, nextIntent) {
  return prevIntent === 'HISTORY' && nextIntent === 'LIVE';
}

const TF_DISPLAY = {
  '1s': '1s',
  '1m': '1m', '2m': '2m', '3m': '3m', '5m': '5m', '10m': '10m', '15m': '15m', '30m': '30m', '45m': '45m',
  '1h': '1H', '2h': '2H', '3h': '3H', '4h': '4H', '6h': '6H', '8h': '8H', '12h': '12H',
  '1d': 'D', '1w': 'W', '1M': '1M',
};

const TF_MENU = {
  SECONDS: [
    { id: '1s', label: '1 second' },
    { id: '5s', label: '5 seconds' },
    { id: '10s', label: '10 seconds' },
    { id: '15s', label: '15 seconds' },
    { id: '30s', label: '30 seconds' },
    { id: '45s', label: '45 seconds' },
  ],
  MINUTES: [
    { id: '1m', label: '1 minute' }, { id: '2m', label: '2 minutes' },
    { id: '3m', label: '3 minutes' }, { id: '5m', label: '5 minutes' },
    { id: '10m', label: '10 minutes' }, { id: '15m', label: '15 minutes' },
    { id: '30m', label: '30 minutes' }, { id: '45m', label: '45 minutes' },
  ],
  HOURS: [
    { id: '1h', label: '1 hour' }, { id: '2h', label: '2 hours' },
    { id: '3h', label: '3 hours' }, { id: '4h', label: '4 hours' },
    { id: '6h', label: '6 hours' }, { id: '8h', label: '8 hours' },
    { id: '12h', label: '12 hours' },
  ],
  DAYS: [
    { id: '1d', label: '1 day' }, { id: '1w', label: '1 week' },
    { id: '1M', label: '1 month' },
  ],
};

const LS_FAV_KEY = 'dashboard_tf_favorites';
const LS_TF_KEY = 'dashboard_tf_current';
const LS_PANE_KEY = 'dashboard_pane_heights';
const LS_PANE_KEY_BT = 'dashboard_pane_heights_bt';
const LS_RSX_SETTINGS_LIVE_KEY = 'dashboard_rsx_settings_live';
const LS_RSX_SETTINGS_BACKTEST_KEY = 'dashboard_rsx_settings_backtest';
const LS_RSX_LOOKBACK_KEY = 'dashboard_rsx_lookback';
const LS_RSX_SIGNAL_LENGTH_KEY = 'dashboard_rsx_signal_length';
const LS_RSX_LENGTH_KEY = 'dashboard_rsx_length';
const WOZDUH_PREFS_LIVE_KEY = 'wozduh_visibility_prefs_live';
const WOZDUH_PREFS_BACKTEST_KEY = 'wozduh_visibility_prefs_backtest';
const WOZDUH_PREFS_KEY = 'wozduh_visibility_prefs';
const DEFAULT_FAVS = ['1m', '3m', '15m', '1h', '4h', '1d', '1w'];
/** Higher-TF quick-sync toggles in trendlines menu (any period can be added here). */
const MTF_SYNC_QUICK_PERIODS = ['4h', '1d', '1w'];
const MTF_PERIOD_COLORS = (typeof ChartTheme !== 'undefined' && ChartTheme.mtfPeriodColors)
  ? ChartTheme.mtfPeriodColors
  : {
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
    '1w': '#ab47bc',
  };
/** Fast RAM tail for /api/state; deep history comes from pre-fetch assembly. */
const LIVE_STATE_CANDLE_LIMIT = 300;
const HISTORY_CHUNK_LIMIT = 3000;
/**
 * P2 working-set policy (replaces obsolete TARGET 12k / HARD_CAP 16k).
 * MAX_STORE_BARS — in-memory ColumnarStore cap (moving window).
 * MAX_VISIBLE_BARS — max zoom-out visible logical span (not "always show N").
 */
const MAX_STORE_BARS = 9000;
/** Max zoom-out (logical bars). */
const MAX_VISIBLE_BARS = 5000;
/** @deprecated aliases — map to MAX_STORE_BARS (single hard working-set). */
const STORE_BUDGET_TARGET = MAX_STORE_BARS;
const STORE_BUDGET_HARD_CAP = MAX_STORE_BARS;
const MAX_STORE_CAPACITY = MAX_STORE_BARS;
const STORE_PRUNE_CHUNK = 5000;
const LIVE_POLL_CANDLE_LIMIT = 5;
const DEFAULT_RSX_LOOKBACK = 90;
const DEFAULT_RSX_SIGNAL_LENGTH = 9;
const DEFAULT_RSX_LENGTH = 14;
const MIN_RSX_LENGTH = 3;
const MAX_RSX_LENGTH = 100;
const MIN_RSX_DIV_LOOKBACK = 10;
const MAX_RSX_DIV_LOOKBACK = 200;
const MIN_RSX_SIGNAL_LENGTH = 2;
const MAX_RSX_SIGNAL_LENGTH = 50;

const RSX_DEFAULT_COLOR = (typeof ChartTheme !== 'undefined') ? ChartTheme.rsxDefault : '#e1d2b5';

/** Lazy-built after LightweightCharts CDN loads (see ensureChartLibraryStyles). */
let CHART_STYLES = null;
let INDICATOR_CONFIG = null;
let SHARED_CROSSHAIR = null;
let WOZDUX_LINE_DEFS = null;
let WOZDUX_LINE_KEYS = [];
function ensureChartLibraryStyles() {
  if (CHART_STYLES) return true;
  if (typeof LightweightCharts === 'undefined') return false;

  const LC = LightweightCharts;
  const oscMidline50Level = {
    price: 50,
    color: 'rgba(204, 85, 0, 0.55)',
    lineStyle: LC.LineStyle.Dashed,
    lineWidth: 1,
    axisLabelVisible: true,
  };

  CHART_STYLES = {
  seriesDefaults: {
    priceLineVisible: false,
    lastValueVisible: false,
    crosshairMarkerRadius: 1,
    crosshairMarkerBorderWidth: 1,
    crosshairMarkerBorderColor: (typeof ChartTheme !== 'undefined') ? ChartTheme.crosshairMarkerBorder : '#90ee90',
  },
  candle: {
    upColor: TV.green,
    downColor: TV.red,
    borderVisible: false,
    wickUpColor: TV.green,
    wickDownColor: TV.red,
  },
  bar: {
    upColor: TV.green,
    downColor: TV.red,
    visible: false,
  },
  priceLine: {
    color: TV.blue,
    lineWidth: 2,
    visible: false,
    priceLineVisible: false,
    lastValueVisible: true,
    autoscaleInfoProvider: () => null,
  },
  volume: {
    priceFormat: { type: 'volume' },
    priceScaleId: 'volume',
    lastValueVisible: false,
    priceLineVisible: false,
  },
  volumeBar: {
    upColor: (typeof ChartTheme !== 'undefined') ? ChartTheme.volumeUp : 'rgba(8,153,129,0.55)',
    downColor: (typeof ChartTheme !== 'undefined') ? ChartTheme.volumeDown : 'rgba(242,54,69,0.55)',
  },
  rsx: {
    color: RSX_DEFAULT_COLOR,
    lineWidth: 2,
    priceLineVisible: false,
    lastValueVisible: false,
  },
  rsxSignal: {
    color: (typeof ChartTheme !== 'undefined') ? ChartTheme.rsxSignalLine : '#ff9800',
    lineWidth: 1,
    lineStyle: LC.LineStyle.Dashed,
    priceLineVisible: false,
    lastValueVisible: false,
  },
  wozduhUp: {
    color: (typeof ChartTheme !== 'undefined') ? ChartTheme.wozduhFast : 'blue',
    lineWidth: 2,
    lineStyle: LC.LineStyle.Solid,
    title: 'wt11 (Blue)',
    priceLineVisible: false,
    lastValueVisible: false,
  },
  wozduhDown: {
    color: (typeof ChartTheme !== 'undefined') ? ChartTheme.wozduhSlow : 'aqua',
    lineWidth: 2,
    lineStyle: LC.LineStyle.Solid,
    title: 'wt22 (Aqua)',
    priceLineVisible: false,
    lastValueVisible: false,
  },
  wozduhLevels: [
    { price: 70, color: 'rgba(255, 255, 255, 0.4)', lineStyle: LC.LineStyle.Dotted, lineWidth: 1, axisLabelVisible: true },
    { ...oscMidline50Level },
    { price: 30, color: 'rgba(255, 255, 255, 0.4)', lineStyle: LC.LineStyle.Dotted, lineWidth: 1, axisLabelVisible: true },
    { price: 92, color: 'rgba(240, 220, 140, 0.75)', lineStyle: LC.LineStyle.Dotted, lineWidth: 1, axisLabelVisible: true },
    { price: 8, color: 'rgba(240, 220, 140, 0.75)', lineStyle: LC.LineStyle.Dotted, lineWidth: 1, axisLabelVisible: true },
  ],
  rsxLevels: [
    { price: 80, color: 'rgba(255, 190, 120, 0.75)', lineStyle: LC.LineStyle.Dotted, lineWidth: 1, axisLabelVisible: true },
    { price: 70, color: 'rgba(255, 255, 255, 0.2)', lineStyle: LC.LineStyle.Dashed, lineWidth: 1, axisLabelVisible: true },
    { ...oscMidline50Level },
    { price: 30, color: 'rgba(255, 255, 255, 0.2)', lineStyle: LC.LineStyle.Dashed, lineWidth: 1, axisLabelVisible: true },
    { price: 20, color: 'rgba(255, 190, 120, 0.75)', lineStyle: LC.LineStyle.Dotted, lineWidth: 1, axisLabelVisible: true },
  ],
  wozdux: {
    rsiPrice: { color: (typeof ChartTheme !== 'undefined') ? ChartTheme.bear : 'red', lineWidth: 2, title: 'RSI(C)', priceLineVisible: false, lastValueVisible: false },
    rsiHl2: { color: (typeof ChartTheme !== 'undefined') ? ChartTheme.short : 'purple', lineWidth: 2, title: 'RSI(HL2)', priceLineVisible: false, lastValueVisible: false },
    rsiVolFast: { color: (typeof ChartTheme !== 'undefined') ? ChartTheme.wozduhFast : 'blue', lineWidth: 2, title: 'wt11 (Blue)', priceLineVisible: false, lastValueVisible: false },
    rsiVolSlow: { color: (typeof ChartTheme !== 'undefined') ? ChartTheme.wozduhSlow : 'aqua', lineWidth: 2, title: 'wt22 (Aqua)', priceLineVisible: false, lastValueVisible: false },
  },
};

  /**
   * Single source of truth for indicator layers on Live and Backtest charts.
   * initProfessionalChart builds every series, area fill, and level line from here.
   */
  INDICATOR_CONFIG = {
  price: {
    candle: CHART_STYLES.candle,
    bar: CHART_STYLES.bar,
    line: CHART_STYLES.priceLine,
    volume: CHART_STYLES.volume,
    volumeBar: CHART_STYLES.volumeBar,
    volumeScale: { scaleMargins: { top: 0.82, bottom: 0 } },
    priceScale: { scaleMargins: { top: 0.05, bottom: 0.22 } },
  },
  rsx: {
    lines: {
      rsx_main: {
        dataKey: 'rsx',
        style: CHART_STYLES.rsx,
      },
      rsx_signal: {
        dataKey: 'rsx_signal',
        style: CHART_STYLES.rsxSignal,
      },
    },
    levels: CHART_STYLES.rsxLevels,
    areas: [
      {
        id: 'rsx_neutral_zone',
        top: 60,
        bottom: 40,
        topColor: 'rgba(225, 210, 181, 0.08)',
        bottomColor: 'rgba(225, 210, 181, 0.04)',
        lineColor: 'transparent',
        lineWidth: 0,
      },
    ],
  },
  wozduh: {
    lines: {
      wozduh_wt1: { dataKey: 'rsiVolFast', style: CHART_STYLES.wozdux.rsiVolFast },
      wozduh_wt2: { dataKey: 'rsiVolSlow', style: CHART_STYLES.wozdux.rsiVolSlow },
      rsiPrice: { dataKey: 'rsiPrice', style: CHART_STYLES.wozdux.rsiPrice },
      rsiHl2: { dataKey: 'rsiHl2', style: CHART_STYLES.wozdux.rsiHl2 },
    },
    markerSeriesKey: 'rsiVolSlow',
    levels: CHART_STYLES.wozduhLevels,
    areas: [
      {
        id: 'wozduh_ob_zone',
        top: 100,
        bottom: 70,
        topColor: 'rgba(255, 255, 0, 0.04)',
        bottomColor: 'rgba(255, 255, 0, 0.02)',
        lineColor: 'transparent',
        lineWidth: 0,
      },
      {
        id: 'wozduh_os_zone',
        top: 30,
        bottom: 0,
        topColor: 'rgba(255, 255, 0, 0.02)',
        bottomColor: 'rgba(255, 255, 0, 0.04)',
        lineColor: 'transparent',
        lineWidth: 0,
      },
    ],
  },
};

  WOZDUX_LINE_DEFS = CHART_STYLES.wozdux;
  WOZDUX_LINE_KEYS = Object.keys(WOZDUX_LINE_DEFS);

  SHARED_CROSSHAIR = {
    mode: LC.CrosshairMode.Normal,
    vertLine: { width: 1, color: (typeof ChartTheme !== 'undefined') ? ChartTheme.crosshair : '#555', style: LC.LineStyle.Dashed },
    horzLine: { width: 1, color: (typeof ChartTheme !== 'undefined') ? ChartTheme.crosshair : '#555', style: LC.LineStyle.Dashed },
  };

  return true;
}

/** PineScript default visibility (klrscena, dada, qdada, klemarsi, etc.) */
const WOZDUH_MENU_ITEMS = [
  { prefKey: 'rsiPrice', keys: ['rsiPrice'], default: true },
  { prefKey: 'rsiHl2', keys: ['rsiHl2'], default: true },
  { prefKey: 'rsiVol', keys: ['rsiVolFast', 'rsiVolSlow'], default: true },
];
const SHARED_TIME_SCALE = {
  borderColor: TV.border,
  timeVisible: true,
  secondsVisible: false,
  minBarSpacing: 0.001,
  fixLeftEdge: false,
  fixRightEdge: false,
};

const BACKTEST_HISTORY_CHUNK_LIMIT = 5000;
const LIVE_HISTORY_SCROLL_THRESHOLD = 50;
/** P1: start edge hydrate when this fraction of the visible span remains to the loaded edge. */
const HISTORY_EDGE_PREFETCH_FRAC = 0.25;
const LS_RISK_SETTINGS_KEY = 'dashboard_risk_settings';

const LIVE_CHART_SELECTORS = {
  priceWrap: 'price-wrap',
  oscWrap: 'osc-wrap',
  rsxWrap: 'rsx-wrap',
  chartContainer: 'price-chart',
  oscContainer: 'wozduh-chart',
  rsxContainer: 'rsx-chart',
};

const BACKTEST_CHART_SELECTORS = {
  priceWrap: 'bt-price-wrap',
  oscWrap: 'bt-osc-wrap',
  rsxWrap: 'bt-rsx-wrap',
  chartContainer: 'bt-price-chart',
  oscContainer: 'bt-wozduh-chart',
  rsxContainer: 'bt-rsx-chart',
};

const PANE_STACK_CONFIG = {
  live: {
    price: 'price-wrap',
    osc: 'osc-wrap',
    rsx: 'rsx-wrap',
    lsKey: LS_PANE_KEY,
    defaults: { price: 55, osc: 22, rsx: 23 },
  },
  backtest: {
    price: 'bt-price-wrap',
    osc: 'bt-osc-wrap',
    rsx: 'bt-rsx-wrap',
    lsKey: LS_PANE_KEY_BT,
    defaults: { price: 55, osc: 22, rsx: 23 },
  },
};

function defaultRsxSettings() {
  return {
    length: DEFAULT_RSX_LENGTH,
    div_lookback: DEFAULT_RSX_LOOKBACK,
    signal_length: DEFAULT_RSX_SIGNAL_LENGTH,
    source: 'hlc3',
    pivot_radius: 2,
    div_method: 'tv',
    min_price_delta_ratio: 0,
    min_osc_delta: 0,
    show_pivots: true,
  };
}

function defaultWozduhPrefs() {
  return Object.fromEntries(WOZDUH_MENU_ITEMS.map((item) => [item.prefKey, item.default]));
}

if (typeof window !== 'undefined') {
  window.CONFIG = {
    TV, SCORING_MATRIX_DEFAULTS, SCORING_MATRIX_LABELS,
    LIVE_STATE_CANDLE_LIMIT, HISTORY_CHUNK_LIMIT, LIVE_POLL_CANDLE_LIMIT,
    MAX_STORE_BARS, MAX_VISIBLE_BARS,
    STORE_BUDGET_TARGET, STORE_BUDGET_HARD_CAP,
    MAX_STORE_CAPACITY, STORE_PRUNE_CHUNK,
    LIVE_HISTORY_SCROLL_THRESHOLD, HISTORY_EDGE_PREFETCH_FRAC, BACKTEST_HISTORY_CHUNK_LIMIT,
    LIVE_CHART_SELECTORS, BACKTEST_CHART_SELECTORS, PANE_STACK_CONFIG,
    defaultRsxSettings, defaultNavigatorPaneSettings, defaultRiskSettings, defaultWozduhPrefs,
    ensureChartLibraryStyles,
  };
  window.requiresDenseTimeContinuity = requiresDenseTimeContinuity;
  window.isSparseLiveChart = isSparseLiveChart;
  window.isLiveSecondChart = isLiveSecondChart;
  window.isSparseSecondChart = isSparseSecondChart;
  window.isSecondsFamilyChart = isSecondsFamilyChart;
  window.secondsFamilyIntervalSec = secondsFamilyIntervalSec;
  window.applySecondsFamilyTfSwitchIntent = applySecondsFamilyTfSwitchIntent;
  window.isSecondsTimeframe = isSecondsTimeframe;
  window.appendLiveTickBuffer = appendLiveTickBuffer;
  window.sparseHistoryToLiveNeedsFullPaint = sparseHistoryToLiveNeedsFullPaint;
}
