/* ── TradingView palette ── */
const TV = {
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
  useRedCross: false,
  useGeometry: false,
  useGeometryBounce: false,
  useGeometryTriangle: false,
  useTrendlines: false,
  useHTFOscillators: false,
  useDivergence: false,
  useFib: false,
  useExpRegime: false,
  useJurikTrend: false,
  useWozduhSpike: false,
  useAD: false,
  useAOCross: false,
};

const SCORING_MATRIX_LABELS = [
  { key: 'useRSX', label: 'RSX L / S markers (+35/+45)' },
  { key: 'useWozduhCross', label: 'Wozduh vol cross lime/red (+35)' },
  { key: 'useRedCross', label: 'Red × Green cross (+35)' },
  { key: 'useGeometry', label: 'Geometry breakout (+30)' },
  { key: 'useGeometryBounce', label: 'Geometry bounce (+25)' },
  { key: 'useGeometryTriangle', label: 'Geometry triangle (+10)' },
  { key: 'useTrendlines', label: 'Trendlines breakout signals' },
  { key: 'useHTFOscillators', label: 'HTF RSX & Wozduh (walk-forward MTF, +20)' },
  { key: 'useDivergence', label: 'Divergence score (±)' },
  { key: 'useFib', label: 'Fib 0.618 active (+20)' },
  { key: 'useExpRegime', label: 'Regime EXPANSION (+15)' },
  { key: 'useJurikTrend', label: 'Jurik tailwind (+15/+20)' },
  { key: 'useWozduhSpike', label: 'Wozduh volume spike (+15)' },
  { key: 'useAD', label: 'AD accumulation / distribution (+20)' },
  { key: 'useAOCross', label: 'AO cross zero (+15)' },
];

let liveStrategyState = null;
let backtestStrategyState = null;
let matrixModalContext = 'live';

function defaultStrategyState() {
  return {
    matrix: { ...SCORING_MATRIX_DEFAULTS },
    thresholds: { ...DEFAULT_STRATEGY_THRESHOLDS },
  };
}

function getStrategyState(context = 'live') {
  return context === 'backtest' ? backtestStrategyState : liveStrategyState;
}

function strategyStorageKey(context) {
  return context === 'backtest' ? LS_BT_STRATEGY_KEY : LS_LIVE_STRATEGY_KEY;
}

function normalizeStrategyThresholds(raw) {
  const long = Number(raw?.long ?? raw?.longThreshold);
  const short = Number(raw?.short ?? raw?.shortThreshold);
  return {
    long: Number.isFinite(long) && long > 0 ? long : DEFAULT_STRATEGY_THRESHOLDS.long,
    short: Number.isFinite(short) && short > 0 ? short : DEFAULT_STRATEGY_THRESHOLDS.short,
  };
}

function normalizeStrategyMatrix(raw) {
  if (!raw || typeof raw !== 'object') return { ...SCORING_MATRIX_DEFAULTS };
  return { ...SCORING_MATRIX_DEFAULTS, ...raw };
}

function persistStrategyState(context) {
  const state = getStrategyState(context);
  if (!state) return;
  try {
    localStorage.setItem(strategyStorageKey(context), JSON.stringify(state));
  } catch {
    /* noop */
  }
}

function loadStrategyState(context) {
  let state = defaultStrategyState();
  try {
    const raw = localStorage.getItem(strategyStorageKey(context));
    if (raw) {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === 'object') {
        state = {
          matrix: normalizeStrategyMatrix(parsed.matrix),
          thresholds: normalizeStrategyThresholds(parsed.thresholds),
        };
      }
    } else if (context === 'live') {
      const legacyMatrix = loadLegacyScoringMatrixFromStorage();
      const legacyThresholds = loadLegacyThresholdsFromStorage();
      if (legacyMatrix) state.matrix = legacyMatrix;
      if (legacyThresholds) state.thresholds = legacyThresholds;
    } else if (context === 'backtest' && liveStrategyState) {
      state = {
        matrix: { ...liveStrategyState.matrix },
        thresholds: { ...liveStrategyState.thresholds },
      };
    }
  } catch {
    /* defaults */
  }
  if (context === 'backtest') backtestStrategyState = state;
  else liveStrategyState = state;
  persistStrategyState(context);
  return state;
}

function initStrategyState() {
  loadStrategyState('live');
  loadStrategyState('backtest');
}

function getActiveStrategyContext() {
  return isBacktestTfContext() ? 'backtest' : 'live';
}

function readThresholdsFromHeader() {
  const longEl = document.getElementById('ui-threshold-long');
  const shortEl = document.getElementById('ui-threshold-short');
  return normalizeStrategyThresholds({
    long: longEl ? Number(longEl.value) : NaN,
    short: shortEl ? Number(shortEl.value) : NaN,
  });
}

function applyThresholdsToHeader(thresholds) {
  const longEl = document.getElementById('ui-threshold-long');
  const shortEl = document.getElementById('ui-threshold-short');
  const t = normalizeStrategyThresholds(thresholds);
  if (longEl) longEl.value = String(t.long);
  if (shortEl) shortEl.value = String(t.short);
}

function saveThresholdsFromHeaderToState(context = getActiveStrategyContext()) {
  const state = getStrategyState(context);
  if (!state) return;
  state.thresholds = readThresholdsFromHeader();
  persistStrategyState(context);
}

function getThresholdsFromStrategyState(context = 'backtest') {
  const t = normalizeStrategyThresholds(getStrategyState(context)?.thresholds);
  return {
    longThreshold: t.long,
    shortThreshold: t.short,
  };
}

function getMatrixFromStrategyState(context = 'backtest') {
  return normalizeStrategyMatrix(getStrategyState(context)?.matrix);
}

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

const chartLegendState = {
  live: { price: {}, wozduh: {}, rsx: {} },
  backtest: { price: {}, wozduh: {}, rsx: {} },
};

let factsBarUserVisible = false;
let openNavigatorPopupEl = null;

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

function navigatorSettingsStorageKey(pane, context = 'backtest') {
  const prefix = context === 'live' ? LS_NAV_SETTINGS_LIVE_PREFIX : LS_NAV_SETTINGS_PREFIX;
  return `${prefix}${pane}`;
}

function loadNavigatorPaneSettings(pane, context = 'backtest') {
  try {
    const raw = localStorage.getItem(navigatorSettingsStorageKey(pane, context));
    if (raw) return { ...defaultNavigatorPaneSettings(pane), ...JSON.parse(raw) };
  } catch {
    /* defaults */
  }
  return defaultNavigatorPaneSettings(pane);
}

function saveNavigatorPaneSettings(pane, settings, context = 'backtest') {
  localStorage.setItem(navigatorSettingsStorageKey(pane, context), JSON.stringify(settings));
}

/** Navigator UI context: 'live' | 'backtest' (active dashboard tab). */
function getNavigatorContext() {
  return isBacktestTabActive() ? 'backtest' : 'live';
}

function getNavigatorChartData(context = getNavigatorContext()) {
  return context === 'backtest' ? backtestChartData : liveChartData;
}

function getNavigatorSettingsPrefix(context = getNavigatorContext()) {
  return context === 'live' ? LS_NAV_SETTINGS_LIVE_PREFIX : LS_NAV_SETTINGS_PREFIX;
}

function loadNavigatorDefaults(pane) {
  try {
    const raw = localStorage.getItem(`${LS_NAV_DEFAULTS_PREFIX}${pane}`);
    if (raw) return { ...defaultNavigatorPaneSettings(pane), ...JSON.parse(raw) };
  } catch {
    /* defaults */
  }
  return defaultNavigatorPaneSettings(pane);
}

function saveNavigatorDefaults(pane, settings) {
  localStorage.setItem(`${LS_NAV_DEFAULTS_PREFIX}${pane}`, JSON.stringify(settings));
}

function getNavigatorPluginForPane(chartData, pane) {
  if (!chartData) return null;
  if (pane === 'price') return chartData.priceNavigatorPlugin;
  if (pane === 'rsx') return chartData.rsxNavigatorPlugin;
  if (pane === 'wozduh') return chartData.wozduhNavigatorPlugin;
  return null;
}

function normalizeMtfPeriod(tf) {
  if (!tf) return '';
  const s = String(tf).trim();
  if (s === '1M') return '1M';
  return s.toLowerCase();
}

function mtfPeriodsEqual(a, b) {
  return normalizeMtfPeriod(a) === normalizeMtfPeriod(b);
}

function getMtfPeriodColor(tf) {
  const key = normalizeMtfPeriod(tf);
  return MTF_PERIOD_COLORS[key] || '#787b86';
}

/** Overlay line series that must not affect price autoscale (all MTF / helper layers). */
function createMTFOverlaySeries(chart, options = {}) {
  if (!chart?.addLineSeries) return null;
  const {
    color = 'transparent',
    lineWidth = 2,
    lineStyle = 0,
    ...rest
  } = options;
  return chart.addLineSeries({
    color,
    lineWidth,
    lineStyle,
    priceLineVisible: false,
    lastValueVisible: false,
    crosshairMarkerVisible: false,
    autoscaleInfoProvider: () => null,
    ...rest,
  });
}

function getNavigatorHostSeries(chartData, pane) {
  if (!chartData) return null;
  if (pane === 'rsx') return chartData.rsxSeries;
  if (pane === 'wozduh') return chartData.wozduhDownSeries || chartData.wozduxSeries?.rsiVolSlow;
  return chartData.candleSeries;
}

function getNavigatorPriceChart(chartData, pane) {
  if (!chartData) return null;
  if (pane === 'rsx') return chartData.rsxChart;
  if (pane === 'wozduh') return chartData.oscChart;
  return chartData.priceChart;
}

function ensureMtfNavigatorLayers(chartData) {
  if (!chartData.mtfNavigatorLayers) {
    chartData.mtfNavigatorLayers = { price: {}, rsx: {}, wozduh: {} };
  }
  return chartData.mtfNavigatorLayers;
}

function getActiveMtfPeriods(settings, chartTf) {
  const chartKey = normalizeMtfPeriod(chartTf);
  const periods = coerceNavigatorPeriods(settings);
  return periods.filter((p) => p && !mtfPeriodsEqual(p, chartKey));
}

function ensureMtfNavigatorLayer(chartData, pane, tf) {
  const key = normalizeMtfPeriod(tf);
  if (!key) return null;
  const layers = ensureMtfNavigatorLayers(chartData);
  const paneMap = layers[pane] || (layers[pane] = {});
  if (paneMap[key]) return paneMap[key];

  const priceChart = getNavigatorPriceChart(chartData, pane);
  if (!priceChart) return null;

  const anchorSeries = createMTFOverlaySeries(priceChart, {
    color: getMtfPeriodColor(key),
    lineWidth: 0,
    visible: false,
  });
  let plugin = null;
  if (anchorSeries && typeof TrendlinePrimitive !== 'undefined') {
    plugin = new TrendlinePrimitive();
    anchorSeries.attachPrimitive(plugin);
  }
  paneMap[key] = { anchorSeries, plugin, tf: key };
  return paneMap[key];
}

function getMtfNavigatorPlugin(chartData, pane, interval, chartTf) {
  const key = normalizeMtfPeriod(interval);
  const chartKey = normalizeMtfPeriod(chartTf);
  if (!key || mtfPeriodsEqual(key, chartKey)) {
    return getNavigatorPluginForPane(chartData, pane);
  }
  return ensureMtfNavigatorLayer(chartData, pane, key)?.plugin || null;
}

function syncMtfNavigatorLayerRegistry(chartData, pane, activePeriods, chartTf) {
  const layers = ensureMtfNavigatorLayers(chartData);
  const paneMap = layers[pane] || (layers[pane] = {});
  const wanted = new Set(
    (activePeriods || []).map((p) => normalizeMtfPeriod(p)).filter(Boolean),
  );
  wanted.forEach((tf) => {
    if (!mtfPeriodsEqual(tf, chartTf)) {
      ensureMtfNavigatorLayer(chartData, pane, tf);
    }
  });
  Object.keys(paneMap).forEach((key) => {
    if (wanted.has(key)) return;
    paneMap[key]?.plugin?.setData([]);
    paneMap[key]?.plugin?.setBackgroundZones([]);
  });
}

function groupNavigatorLinesByInterval(lines, chartTf) {
  const chartKey = normalizeMtfPeriod(chartTf);
  const groups = new Map();
  (lines || []).forEach((line) => {
    const key = normalizeMtfPeriod(line.interval) || chartKey;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(line);
  });
  return groups;
}

function resolveChartTfForNavigator(context) {
  if (context === 'backtest') {
    return normalizeTf(
      document.getElementById('bt-interval')?.value || backtestTf || '15m',
    );
  }
  return normalizeTf(currentTf || getActiveTfFromToolbar() || '15m');
}

function loadNavigatorPopupPos(pane) {
  try {
    const raw = localStorage.getItem(`${LS_NAV_POPUP_POS_PREFIX}${pane}`);
    if (raw) return JSON.parse(raw);
  } catch {
    /* noop */
  }
  return null;
}

function saveNavigatorPopupPos(pane, x, y) {
  localStorage.setItem(`${LS_NAV_POPUP_POS_PREFIX}${pane}`, JSON.stringify({ x, y }));
}

function navigatorSettingsToAPI(settings, source, enabled = true) {
  const s = normalizeNavigatorPaneSettings(settings);
  const resolvedSource = source || 'Price';
  return {
    enabled: !!enabled,
    source: resolvedSource,
    trendType: s.trendType || 'Wicks',
    term: s.term || 'Long',
    useLong: s.useLong !== false,
    useMedium: s.useMedium !== false,
    useShort: s.useShort !== false,
    longLen: Number(s.longLen) || 60,
    mediumLen: Number(s.mediumLen) || 30,
    shortLen: Number(s.shortLen) || 10,
    momentumEnabled: !!s.momentumEnabled,
    momentumBars: Number(s.momentumBars) || 14,
    momentumPercent: Number(s.momentumPercent) || 100,
    timeHoldEnabled: !!s.timeHoldEnabled,
    timeHoldBars: Number(s.timeHoldBars) || 2,
    barColor: !!s.barColor,
    backgroundColor: !!s.backgroundColor,
    periods: coerceNavigatorPeriods(s),
  };
}

function coerceNavigatorPeriods(settings) {
  const s = settings || {};
  if (Array.isArray(s.periods)) {
    return s.periods.filter((p) => typeof p === 'string' && p.trim() && p.trim() !== '1M').map((p) => p.trim());
  }
  if (typeof s.period === 'string' && s.period.trim()) {
    return [s.period.trim()];
  }
  return [];
}

function normalizeNavigatorPaneSettings(settings, pane = 'price') {
  const s = settings || {};
  const defaults = defaultNavigatorPaneSettings(pane);
  return {
    ...defaults,
    ...s,
    periods: coerceNavigatorPeriods(s),
    targetPrice: s.targetPrice !== false,
    targetRSX: !!s.targetRSX,
    targetWozduh: !!s.targetWozduh,
    useLong: s.useLong !== false,
    useMedium: s.useMedium !== false,
    useShort: s.useShort !== false,
    momentumEnabled: !!s.momentumEnabled,
    timeHoldEnabled: !!s.timeHoldEnabled,
    barColor: !!s.barColor,
    backgroundColor: !!s.backgroundColor,
  };
}

function navPopupField(popup, field) {
  if (!popup || !field) return null;
  return popup.querySelector(`[data-nav-field="${field}"]`);
}

function setNavPopupField(popup, field, value, type = 'value') {
  const el = navPopupField(popup, field);
  if (!el) return;
  if (type === 'checkbox') {
    el.checked = !!value;
    return;
  }
  el.value = value ?? '';
}

function getNavPopupField(popup, field, type = 'value') {
  const el = navPopupField(popup, field);
  if (!el) return null;
  if (type === 'checkbox') return el.checked;
  return el.value;
}

function getNavigatorPopup(pane) {
  return document.getElementById(`popup-${pane}`)
    || document.querySelector(`.navigator-popup[data-pane="${pane}"]`);
}

function getNavigatorPaneSettingsFromUI(pane, context = getNavigatorContext()) {
  const safePane = pane || 'price';
  try {
    const popup = getNavigatorPopup(safePane);
    if (popup && !popup.hidden) {
      return normalizeNavigatorPaneSettings(readNavigatorSettingsFromPopup(popup, safePane), safePane);
    }
    return normalizeNavigatorPaneSettings(loadNavigatorPaneSettings(safePane, context), safePane);
  } catch (err) {
    console.warn(`getNavigatorPaneSettingsFromUI(${safePane}) failed:`, err);
    return defaultNavigatorPaneSettings(safePane);
  }
}

function navigatorSourceForPane(pane) {
  return NAVIGATOR_SOURCE_MAP[pane] || 'Price';
}

function isNavigatorPaneEnabled(pane, context = getNavigatorContext()) {
  const nav = getNavigatorSettingsFromUI(pane, context);
  return nav.enabled === true;
}

function navigatorPaneEnabledFlag(pane, priceUI) {
  if (pane === 'price') return priceUI.targetPrice !== false;
  if (pane === 'rsx') return !!priceUI.targetRSX;
  if (pane === 'wozduh') return !!priceUI.targetWozduh;
  return false;
}

/** API-shaped navigator settings for backend (always returns an object, never null). */
function getNavigatorSettingsFromUI(pane, context = getNavigatorContext()) {
  const safePane = pane || 'price';
  const source = NAVIGATOR_SOURCE_MAP[safePane] || 'Price';
  try {
    const ui = getNavigatorPaneSettingsFromUI(safePane, context);
    const priceUI = safePane === 'price' ? ui : getNavigatorPaneSettingsFromUI('price', context);
    return navigatorSettingsToAPI(
      ui,
      source,
      navigatorPaneEnabledFlag(safePane, priceUI),
    );
  } catch (err) {
    console.warn(`getNavigatorSettingsFromUI(${safePane}) failed:`, err);
    return navigatorSettingsToAPI(
      defaultNavigatorPaneSettings(safePane),
      source,
      safePane === 'price',
    );
  }
}

function readMtfSyncSettings() {
  const mtfSettings = {};
  document.querySelectorAll('.mtf-sync-chk').forEach((chk) => {
    const tf = chk.dataset.tf;
    if (tf) mtfSettings[tf] = !!chk.checked;
  });
  return mtfSettings;
}

function syncMtfCheckboxesFromPeriods(periods) {
  const selected = new Set(Array.isArray(periods) ? periods : []);
  document.querySelectorAll('.mtf-sync-chk').forEach((chk) => {
    const tf = chk.dataset.tf;
    if (tf) chk.checked = selected.has(tf);
  });
}

function syncPeriodCheckboxFromMtf(tf, checked) {
  const popup = getNavigatorPopup('price');
  const container = popup?.querySelector('[data-nav-field="periods"]');
  if (!container) return;
  const periodInput = container.querySelector(`input[type="checkbox"][value="${tf}"]`);
  if (periodInput) periodInput.checked = !!checked;
}

function initMtfSyncCheckboxes() {
  initMtfSyncCheckboxGroup();
  const pricePopup = getNavigatorPopup('price');
  if (pricePopup) {
    syncMtfCheckboxesFromPeriods(readNavigatorPeriodsFromPopup(pricePopup));
  }

  document.querySelectorAll('.mtf-sync-chk').forEach((chk) => {
    if (chk.dataset.mtfSyncBound === '1') return;
    chk.dataset.mtfSyncBound = '1';
    chk.addEventListener('change', () => {
      const tf = chk.dataset.tf;
      console.log(`MTF Sync toggled for ${tf}: ${chk.checked}`);
      syncPeriodCheckboxFromMtf(tf, chk.checked);
      const context = getNavigatorContext();
      if (context === 'live') {
        if (chartInitialized && isLiveTabActive()) {
          triggerNavigatorAutoUpdate().catch((err) => {
            console.error('[UI] MTF sync live update failed:', err);
          });
        }
        return;
      }
      scheduleNavigatorAutoUpdate();
    });
  });

  const periodsContainer = pricePopup?.querySelector('[data-nav-field="periods"]');
  if (periodsContainer && periodsContainer.dataset.mtfPeriodSyncBound !== '1') {
    periodsContainer.dataset.mtfPeriodSyncBound = '1';
    periodsContainer.querySelectorAll('input[type="checkbox"]').forEach((el) => {
      el.addEventListener('change', () => {
        syncMtfCheckboxesFromPeriods(readNavigatorPeriodsFromPopup(pricePopup));
      });
    });
  }
}

function initMtfSyncCheckboxGroup() {
  const group = document.getElementById('trendlines-sync-group');
  if (!group || group.dataset.mtfBuilt === '1') return;
  group.dataset.mtfBuilt = '1';
  const body = group.querySelector('.mtf-sync-body') || group;
  const labels = MTF_SYNC_QUICK_PERIODS.map((tf) => {
    const label = tf === '1d' ? '1D' : tf.toUpperCase();
    return `<label><input type="checkbox" id="chk-sync-${tf}" class="mtf-sync-chk" data-tf="${tf}" /> Синхронизировать ${label} Тренд</label>`;
  }).join('');
  const heading = body.querySelector('h4');
  if (heading) {
    heading.insertAdjacentHTML('afterend', labels);
  } else {
    body.innerHTML = `<h4>Синхронизация ТФ (MTF)</h4>${labels}`;
  }
}

function buildNavigatorPayloadFromUI(context = getNavigatorContext()) {
  return {
    price: getNavigatorSettingsFromUI('price', context),
    rsx: getNavigatorSettingsFromUI('rsx', context),
    wozduh: getNavigatorSettingsFromUI('wozduh', context),
  };
}

async function syncLiveNavigatorSettingsToServer() {
  const navigators = buildNavigatorPayloadFromUI('live');
  const resp = await fetch('/api/settings/navigators', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ navigators }),
  });
  if (!resp.ok) {
    throw new Error(`navigator settings sync failed: ${resp.status}`);
  }
  return resp.json().catch(() => ({}));
}

let navigatorAutoUpdateTimer = null;

async function triggerNavigatorAutoUpdate() {
  const context = getNavigatorContext();
  if (context === 'live') {
    const viewportSnapshot = captureLiveViewportSnapshot();
    await syncLiveNavigatorSettingsToServer();
    if (chartInitialized && loadedCandles.length > 0 && shouldPaintLiveChart()) {
      await refreshLiveNavigatorFromServer(viewportSnapshot);
    } else {
      await loadDashboard({ preserveViewport: viewportSnapshot });
    }
    return;
  }
  await flushIndicatorSettingsAutoUpdate();
}

async function refreshLiveNavigatorFromServer(viewportSnapshot) {
  const reqId = ++navigatorRequestId;
  try {
    const { warmingUp, data } = await fetchLiveState({ navigatorsOnly: true });
    if (reqId !== navigatorRequestId) return;
    if (warmingUp) return;
    liveNavigatorResult = data.navigators || null;
    beginDataUpdate();
    try {
      applyNavigatorOverlays(
        { navigators: liveNavigatorResult },
        loadedCandles,
        liveChartData,
        { context: 'live', updateLoadedCandles: false },
      );
      restoreLiveViewportSnapshotTwice(viewportSnapshot);
    } finally {
      endDataUpdate(0);
    }
  } catch (err) {
    if (err?.name === 'AbortError') return;
    console.error('[UI] refreshLiveNavigatorFromServer failed:', err);
  }
}

function scheduleNavigatorAutoUpdate() {
  clearTimeout(navigatorAutoUpdateTimer);
  navigatorAutoUpdateTimer = setTimeout(() => {
    navigatorAutoUpdateTimer = null;
    triggerNavigatorAutoUpdate().catch((err) => {
      console.error('[UI] Navigator auto-update failed:', err);
    });
  }, 500);
}

function flushNavigatorAutoUpdate() {
  clearTimeout(navigatorAutoUpdateTimer);
  navigatorAutoUpdateTimer = null;
  return triggerNavigatorAutoUpdate();
}

function getMatrixSettingsFromUI(context = getActiveStrategyContext()) {
  return getMatrixFromStrategyState(context);
}

function getRiskSettingsFromUI() {
  try {
    return readRiskSettingsFromForm();
  } catch (err) {
    console.warn('getRiskSettingsFromUI failed:', err);
    try {
      const raw = localStorage.getItem(LS_RISK_SETTINGS_KEY);
      if (raw) return JSON.parse(raw);
    } catch {
      /* noop */
    }
    return {};
  }
}

function getThresholdsFromUI(context = getActiveStrategyContext()) {
  return getThresholdsFromStrategyState(context);
}

function getRSXSettingsFromUI(context = 'backtest') {
  return readRsxSettingsFromMenu(context);
}

function getWozduhSettingsFromUI(context = 'backtest') {
  const menu = getWozduhSettingsMenu(getOscWrap(context));
  if (menu) return readWozduhPrefsFromMenu(menu);
  return loadWozduhPrefsForContext(context) || defaultWozduhPrefs();
}

function buildFinalBacktestPayload(overrides = {}) {
  if (!window.currentBacktestPayload) window.currentBacktestPayload = {};

  const prevSettings = (window.currentBacktestPayload.settings && typeof window.currentBacktestPayload.settings === 'object')
    ? window.currentBacktestPayload.settings
    : {};

  const matrix = getMatrixFromStrategyState('backtest');
  const navigators = buildNavigatorPayloadFromUI();
  const risk = getRiskSettingsFromUI();

  const settings = {
    ...prevSettings,
    risk,
    matrix,
    navigators,
    ...getThresholdsFromStrategyState('backtest'),
    rsxSettings: getRSXSettingsFromUI('backtest'),
    wozduhSettings: getWozduhSettingsFromUI('backtest'),
  };

  const finalPayload = {
    symbol: overrides.symbol
      ?? document.getElementById('bt-symbol')?.value.trim()
      ?? window.currentBacktestPayload.symbol
      ?? 'BTCUSDT',
    interval: overrides.interval
      ?? document.getElementById('bt-interval')?.value
      ?? backtestTf
      ?? getActiveTfFromToolbar()
      ?? window.currentBacktestPayload.interval
      ?? '15m',
    startDate: overrides.startDate
      ?? document.getElementById('bt-start')?.value
      ?? window.currentBacktestPayload.startDate
      ?? '',
    endDate: overrides.endDate
      ?? document.getElementById('bt-end')?.value
      ?? window.currentBacktestPayload.endDate
      ?? '',
    settings,
    mtfOptions: readMtfSyncSettings(),
  };

  window.currentBacktestPayload = finalPayload;
  currentBacktestPayload = finalPayload;
  return finalPayload;
}

function matrixHasEntrySources(matrix) {
  if (!matrix) return false;
  return !!(matrix.useRSX || matrix.useWozduhCross || matrix.useTrendlines);
}

function readMatrixCheckbox(key) {
  const el = document.getElementById(`matrix-${key}`)
    || document.querySelector(`input[type="checkbox"][data-matrix-key="${key}"]`);
  if (!el) return null;
  return !!el.checked;
}

function injectBacktestPayloadFromPaneUI(pane) {
  const safePane = pane || 'price';
  const context = getNavigatorContext();
  const popup = getNavigatorPopup(safePane);
  if (popup) {
    const uiSettings = normalizeNavigatorPaneSettings(
      readNavigatorSettingsFromPopup(popup, safePane),
      safePane,
    );
    saveNavigatorPaneSettings(safePane, uiSettings, context);
  }

  if (context === 'live') {
    return buildNavigatorPayloadFromUI('live');
  }

  const finalPayload = buildFinalBacktestPayload();

  console.log(`[UI] Ok clicked for ${safePane}. Payload injected:`, finalPayload.settings.navigators[safePane]);
  console.log('[UI] Full navigators:', finalPayload.settings.navigators);
  console.log('[UI] Matrix:', finalPayload.settings.matrix);
  return finalPayload;
}

function initNavigatorPopupOkHandlers() {
  if (window.__navigatorOkBound) return;
  window.__navigatorOkBound = true;

  document.addEventListener('click', (event) => {
    const btn = event.target.closest('.btn-ok');
    if (!btn) return;

    const popup = btn.closest('.navigator-popup, .popup-container');
    if (!popup) return;

    const pane = popup.id?.replace('popup-', '') || popup.dataset.pane || '';
    if (!pane) return;

    event.preventDefault();
    event.stopPropagation();

    try {
      injectBacktestPayloadFromPaneUI(pane);

      renderChartLegends('backtest');
      renderChartLegends('live');

      popup.hidden = true;
      if (openNavigatorPopupEl === popup) openNavigatorPopupEl = null;

      if (getNavigatorContext() === 'backtest') {
        setBacktestLoading(true);
      }
      flushNavigatorAutoUpdate().catch((err) => {
        console.error('[UI] Navigator Ok pipeline failed:', err);
      });
    } catch (err) {
      console.error('[UI] Navigator popup Ok failed:', err);
    }
  }, true);
}

function buildBacktestSettingsPayload() {
  try {
    const payload = buildFinalBacktestPayload();
    console.log('[Backtest] Settings payload from UI:', payload.settings);
    return payload.settings;
  } catch (err) {
    console.error('buildBacktestSettingsPayload failed:', err);
    const matrix = getMatrixFromStrategyState('backtest');
    try {
      return {
        matrix,
        navigators: buildNavigatorPayloadFromUI(),
        risk: getRiskSettingsFromUI(),
        ...getThresholdsFromStrategyState('backtest'),
      };
    } catch {
      const priceUI = defaultNavigatorPaneSettings('price');
      return {
        matrix,
        risk: getRiskSettingsFromUI(),
        navigators: {
          price: navigatorSettingsToAPI(priceUI, 'Price', true),
          rsx: navigatorSettingsToAPI(defaultNavigatorPaneSettings('rsx'), 'RSX', false),
          wozduh: navigatorSettingsToAPI(defaultNavigatorPaneSettings('wozduh'), 'Wozduh', false),
        },
      };
    }
  }
}

function buildNavigatorBacktestPayload() {
  return buildBacktestSettingsPayload().navigators;
}

function getChartDataForContext(context) {
  return context === 'backtest' ? backtestChartData : liveChartData;
}

function getWrapForPane(context, pane) {
  const chartData = getChartDataForContext(context);
  if (!chartData?.elements) return null;
  if (pane === 'price') return chartData.elements.priceWrap;
  if (pane === 'wozduh') return chartData.elements.oscWrap;
  if (pane === 'rsx') return chartData.elements.rsxWrap;
  return null;
}

function ensureLegendVisibilityState(context, pane, legendId) {
  if (!chartLegendState[context][pane][legendId]) {
    chartLegendState[context][pane][legendId] = { visible: true };
  }
  return chartLegendState[context][pane][legendId];
}

function toggleLegendVisibility(context, pane, legendId) {
  const state = ensureLegendVisibilityState(context, pane, legendId);
  state.visible = !state.visible;
  const chartData = getChartDataForContext(context);
  const visible = state.visible;

  if (legendId === 'price') {
    const mode = chartType;
    const series = mode === 'bars' ? chartData.barSeries
      : mode === 'line' ? chartData.lineSeries
        : chartData.candleSeries;
    series?.applyOptions({ visible });
    chartData.candleSeries?.applyOptions({ visible: mode === 'candles' ? visible : false });
    chartData.barSeries?.applyOptions({ visible: mode === 'bars' ? visible : false });
    chartData.lineSeries?.applyOptions({ visible: mode === 'line' ? visible : false });
    chartData.volumeSeries?.applyOptions({ visible });
  } else if (legendId === 'wozduh') {
    if (visible) {
      applyWozduhVisibilityToChart(chartData, context);
    } else {
      Object.values(chartData.wozduxSeries || {}).forEach((series) => {
        series?.applyOptions({ visible: false });
      });
    }
  } else if (legendId === 'rsx') {
    chartData.rsxSeries?.applyOptions({ visible });
    chartData.rsxSignalSeries?.applyOptions({ visible });
    if (!visible) {
      chartData.rsxSeries?.applyOptions({ visible: false });
      chartData.rsxSignalSeries?.applyOptions({ visible: false });
    }
  } else if (legendId === 'trendlines') {
    const plugin = getNavigatorPluginForPane(chartData, pane);
    plugin?.setLinesVisible(visible);
    const paneSettings = loadNavigatorPaneSettings(pane, context);
    paneSettings.linesVisible = visible;
    saveNavigatorPaneSettings(pane, paneSettings, context);
  } else if (legendId === 'trades') {
    chartData.tradeMarkerPlugin?.setVisible(visible);
  }

  renderChartLegends(context);
}

function hideAllNavigatorPopups() {
  document.querySelectorAll('.navigator-popup').forEach((el) => {
    el.hidden = true;
  });
  openNavigatorPopupEl = null;
}

function readNavigatorPeriodsFromPopup(popup) {
  const container = popup?.querySelector('[data-nav-field="periods"]');
  if (!container) return [];
  return Array.from(container.querySelectorAll('input[type="checkbox"]:checked'))
    .map((el) => el.value)
    .filter(Boolean);
}

function applyNavigatorPeriodsToPopup(popup, periods) {
  const container = popup?.querySelector('[data-nav-field="periods"]');
  if (!container) return;
  const selected = new Set(Array.isArray(periods) ? periods : []);
  container.querySelectorAll('input[type="checkbox"]').forEach((el) => {
    el.checked = selected.has(el.value);
  });
  if (popup?.dataset?.pane === 'price' || popup?.id === 'popup-price') {
    syncMtfCheckboxesFromPeriods(periods);
  }
}

function buildNavigatorMTFPeriodsHTML() {
  return `
      <div class="setting-row">
        <label>Periods (MTF)</label>
        <div class="period-checkboxes" data-nav-field="periods">
          <label><input type="checkbox" value="1m" /> 1m</label>
          <label><input type="checkbox" value="3m" /> 3m</label>
          <label><input type="checkbox" value="5m" /> 5m</label>
          <label><input type="checkbox" value="15m" /> 15m</label>
          <label><input type="checkbox" value="30m" /> 30m</label>
          <label><input type="checkbox" value="1h" /> 1h</label>
          <label><input type="checkbox" value="4h" /> 4h</label>
          <label><input type="checkbox" value="1d" /> 1d</label>
          <label><input type="checkbox" value="1w" /> 1w</label>
        </div>
      </div>`;
}

function buildNavigatorPopupHTML(pane) {
  const showTargets = pane === 'price';
  const p = pane;
  return `
    <div class="navigator-popup__header">
      <span>Trendlines — ${pane.toUpperCase()}</span>
      <button type="button" class="navigator-popup__close" aria-label="Close">×</button>
    </div>
    <div class="navigator-popup__body" data-nav-pane="${p}">
      ${buildNavigatorMTFPeriodsHTML()}
      ${showTargets ? `
      <div class="setting-row">
        <label class="menu-row"><input type="checkbox" data-nav-field="targetPrice" /> Price</label>
        <label class="menu-row"><input type="checkbox" data-nav-field="targetRSX" /> RSX</label>
        <label class="menu-row"><input type="checkbox" data-nav-field="targetWozduh" /> Wozduh</label>
      </div>` : ''}
      <div class="setting-row">
        <label>Trend Type</label>
        <select data-nav-field="trendType"><option value="Wicks">Wicks</option><option value="Body">Body</option></select>
      </div>
      <div class="setting-row">
        <label>Periods</label>
        <div class="term-row"><label><input type="checkbox" data-nav-field="useLong" /> Long</label><input type="number" data-nav-field="longLen" min="3" max="500" /></div>
        <div class="term-row"><label><input type="checkbox" data-nav-field="useMedium" /> Medium</label><input type="number" data-nav-field="mediumLen" min="3" max="500" /></div>
        <div class="term-row"><label><input type="checkbox" data-nav-field="useShort" /> Short</label><input type="number" data-nav-field="shortLen" min="3" max="500" /></div>
      </div>
      <div class="setting-row">
        <label>Term</label>
        <select data-nav-field="term"><option value="Long">Long</option><option value="Medium">Medium</option><option value="Short">Short</option></select>
      </div>
      <div class="setting-row">
        <label>HH/LL</label>
        <select data-nav-field="hhll">
          <option value="None">None</option>
          <option value="Only">Only HH/LL</option>
          <option value="Previous">HH/LL &amp; previous H/L</option>
        </select>
      </div>
      <div class="setting-row">
        <label class="menu-row"><input type="checkbox" data-nav-field="momentumEnabled" /> Momentum Break</label>
        <div class="inline-pair">
          <input type="number" data-nav-field="momentumPercent" min="0" step="1" title="% of ATR" placeholder="% ATR" />
          <input type="number" data-nav-field="momentumBars" min="1" step="1" title="ATR Bars" placeholder="ATR bars" />
        </div>
      </div>
      <div class="setting-row">
        <label class="menu-row"><input type="checkbox" data-nav-field="timeHoldEnabled" /> Time Hold Filter</label>
        <input type="number" data-nav-field="timeHoldBars" min="1" step="1" title="Hold bars" />
      </div>
      <div class="setting-row">
        <label class="menu-row"><input type="checkbox" data-nav-field="backgroundColor" /> Background Color</label>
        <label class="menu-row"><input type="checkbox" data-nav-field="barColor" /> Bar Color</label>
      </div>
    </div>
    <div class="navigator-popup__footer">
      <button type="button" class="btn-reset">Reset</button>
      <button type="button" class="btn-save-default">Save as Default</button>
      <button type="button" class="btn-ok">Ok</button>
    </div>`;
}

function applyNavigatorSettingsToPopup(popup, settings) {
  if (!popup) return;
  applyNavigatorPeriodsToPopup(popup, settings.periods);
  setNavPopupField(popup, 'targetPrice', settings.targetPrice, 'checkbox');
  setNavPopupField(popup, 'targetRSX', settings.targetRSX, 'checkbox');
  setNavPopupField(popup, 'targetWozduh', settings.targetWozduh, 'checkbox');
  setNavPopupField(popup, 'trendType', settings.trendType || 'Wicks');
  setNavPopupField(popup, 'useLong', settings.useLong, 'checkbox');
  setNavPopupField(popup, 'longLen', settings.longLen ?? 60);
  setNavPopupField(popup, 'useMedium', settings.useMedium, 'checkbox');
  setNavPopupField(popup, 'mediumLen', settings.mediumLen ?? 30);
  setNavPopupField(popup, 'useShort', settings.useShort, 'checkbox');
  setNavPopupField(popup, 'shortLen', settings.shortLen ?? 10);
  setNavPopupField(popup, 'term', settings.term || 'Long');
  setNavPopupField(popup, 'hhll', settings.hhll || 'None');
  setNavPopupField(popup, 'momentumEnabled', settings.momentumEnabled, 'checkbox');
  setNavPopupField(popup, 'momentumPercent', settings.momentumPercent ?? 100);
  setNavPopupField(popup, 'momentumBars', settings.momentumBars ?? 14);
  setNavPopupField(popup, 'timeHoldEnabled', settings.timeHoldEnabled, 'checkbox');
  setNavPopupField(popup, 'timeHoldBars', settings.timeHoldBars ?? 2);
  setNavPopupField(popup, 'backgroundColor', settings.backgroundColor, 'checkbox');
  setNavPopupField(popup, 'barColor', settings.barColor, 'checkbox');
}

function readNavigatorSettingsFromPopup(popup, pane) {
  if (!popup) return defaultNavigatorPaneSettings(pane);
  const base = loadNavigatorPaneSettings(pane, getNavigatorContext());
  const chk = (field, fallback = false) => {
    const val = getNavPopupField(popup, field, 'checkbox');
    return val == null ? fallback : val;
  };
  const num = (field, fallback) => {
    const raw = getNavPopupField(popup, field);
    const n = parseInt(raw, 10);
    return Number.isFinite(n) ? n : fallback;
  };
  const flt = (field, fallback) => {
    const raw = getNavPopupField(popup, field);
    const n = parseFloat(raw);
    return Number.isFinite(n) ? n : fallback;
  };

  return {
    ...base,
    periods: readNavigatorPeriodsFromPopup(popup),
    targetPrice: chk('targetPrice', base.targetPrice !== false),
    targetRSX: chk('targetRSX', !!base.targetRSX),
    targetWozduh: chk('targetWozduh', !!base.targetWozduh),
    trendType: getNavPopupField(popup, 'trendType') || 'Wicks',
    useLong: chk('useLong', base.useLong !== false),
    longLen: num('longLen', base.longLen ?? 60),
    useMedium: chk('useMedium', base.useMedium !== false),
    mediumLen: num('mediumLen', base.mediumLen ?? 30),
    useShort: chk('useShort', base.useShort !== false),
    shortLen: num('shortLen', base.shortLen ?? 10),
    term: getNavPopupField(popup, 'term') || 'Long',
    hhll: getNavPopupField(popup, 'hhll') || 'None',
    momentumEnabled: chk('momentumEnabled', !!base.momentumEnabled),
    momentumPercent: flt('momentumPercent', base.momentumPercent ?? 100),
    momentumBars: num('momentumBars', base.momentumBars ?? 14),
    timeHoldEnabled: chk('timeHoldEnabled', !!base.timeHoldEnabled),
    timeHoldBars: num('timeHoldBars', base.timeHoldBars ?? 2),
    backgroundColor: chk('backgroundColor', !!base.backgroundColor),
    barColor: chk('barColor', !!base.barColor),
  };
}

function bindNavigatorPopupChrome(popup, pane) {
  if (!popup) return;
  popup.classList.add('navigator-popup', 'popup-container');
  if (!popup.dataset.pane) popup.dataset.pane = pane;
  if (!popup.id) popup.id = `popup-${pane}`;

  if (popup.dataset.chromeBound !== '1') {
    popup.dataset.chromeBound = '1';
    const header = popup.querySelector('.navigator-popup__header');
    if (header) initNavigatorPopupDrag(popup, header, pane);
    popup.querySelector('.navigator-popup__close')?.addEventListener('click', () => {
      popup.hidden = true;
      if (openNavigatorPopupEl === popup) openNavigatorPopupEl = null;
    });
    popup.addEventListener('mousedown', (e) => e.stopPropagation());
  }
  bindNavigatorPopupActions(popup, pane);
}

function bindNavigatorPopupActions(popup, pane) {
  if (!popup || popup.dataset.actionsBound === '1') return;
  popup.dataset.actionsBound = '1';

  // Ok is handled globally by initNavigatorPopupOkHandlers (capture phase).

  popup.querySelector('.btn-save-default')?.addEventListener('click', () => {
    const settings = readNavigatorSettingsFromPopup(popup, pane);
    saveNavigatorDefaults(pane, settings);
  });

  popup.querySelector('.btn-reset')?.addEventListener('click', () => {
    applyNavigatorSettingsToPopup(popup, loadNavigatorDefaults(pane));
  });
}

function ensureNavigatorPopup(pane) {
  let popup = document.getElementById(`popup-${pane}`)
    || document.querySelector(`.navigator-popup[data-pane="${pane}"]`);

  if (popup) {
    bindNavigatorPopupChrome(popup, pane);
    return popup;
  }

  popup = document.createElement('div');
  popup.className = 'popup-container navigator-popup';
  popup.id = `popup-${pane}`;
  popup.dataset.pane = pane;
  popup.hidden = true;
  popup.innerHTML = buildNavigatorPopupHTML(pane);
  document.body.appendChild(popup);
  bindNavigatorPopupChrome(popup, pane);

  return popup;
}

function initNavigatorPopupDrag(popup, handle, pane) {
  let dragging = false;
  let startX = 0;
  let startY = 0;
  let originX = 0;
  let originY = 0;

  handle.addEventListener('mousedown', (e) => {
    if (e.target.closest('.navigator-popup__close')) return;
    dragging = true;
    startX = e.clientX;
    startY = e.clientY;
    originX = popup.offsetLeft;
    originY = popup.offsetTop;
    e.preventDefault();
  });

  document.addEventListener('mousemove', (e) => {
    if (!dragging) return;
    const x = originX + (e.clientX - startX);
    const y = originY + (e.clientY - startY);
    popup.style.left = `${x}px`;
    popup.style.top = `${y}px`;
  });

  document.addEventListener('mouseup', () => {
    if (!dragging) return;
    dragging = false;
    saveNavigatorPopupPos(pane, popup.offsetLeft, popup.offsetTop);
  });
}

function openNavigatorPopup(pane, anchorEl) {
  const popup = ensureNavigatorPopup(pane);
  if (openNavigatorPopupEl === popup && popup && !popup.hidden) {
    hideAllNavigatorPopups();
    return;
  }

  hideAllNavigatorPopups();
  hideRsxSettingsMenus();
  hideWozduhSettingsMenus();
  hideRiskSettingsMenu();

  if (!popup.querySelector('[data-nav-field="trendType"]')) {
    popup.dataset.actionsBound = '';
    popup.dataset.chromeBound = '';
    popup.innerHTML = buildNavigatorPopupHTML(pane);
    bindNavigatorPopupChrome(popup, pane);
  }
  applyNavigatorSettingsToPopup(popup, loadNavigatorPaneSettings(pane, getNavigatorContext()));

  const saved = loadNavigatorPopupPos(pane);
  if (saved?.x != null && saved?.y != null) {
    popup.style.left = `${saved.x}px`;
    popup.style.top = `${saved.y}px`;
  } else if (anchorEl) {
    const rect = anchorEl.getBoundingClientRect();
    popup.style.left = `${rect.right + 6}px`;
    popup.style.top = `${rect.top}px`;
  } else {
    popup.style.left = '120px';
    popup.style.top = '80px';
  }

  popup.hidden = false;
  openNavigatorPopupEl = popup;
}

function renderLegendItem(context, pane, def, isChild = false) {
  const state = ensureLegendVisibilityState(context, pane, def.id);
  const hiddenClass = state.visible ? '' : ' legend-item--hidden';
  const eyeClass = state.visible ? '' : ' legend-btn--off';
  const hasGear = def.kind === 'wozduh' || def.kind === 'rsx' || def.id === 'trendlines';

  const item = document.createElement('div');
  item.className = `legend-item${isChild ? ' legend-item--child' : ''}${hiddenClass}`;
  item.dataset.legend = def.id;
  item.innerHTML = `
    <span class="legend-name">${def.label}</span>
    <button type="button" class="legend-btn legend-eye${eyeClass}" title="Toggle visibility">👁️</button>
    ${hasGear ? '<button type="button" class="legend-btn legend-gear" title="Settings">⚙️</button>' : ''}`;

  item.querySelector('.legend-eye')?.addEventListener('click', (e) => {
    e.stopPropagation();
    toggleLegendVisibility(context, pane, def.id);
  });

  item.querySelector('.legend-gear')?.addEventListener('click', (e) => {
    e.stopPropagation();
    if (def.id === 'trendlines') {
      openNavigatorPopup(pane, e.currentTarget);
      return;
    }
    const wrap = getWrapForPane(context, pane);
    if (!wrap) return;
    if (def.kind === 'wozduh') {
      hideRsxSettingsMenus();
      hideRiskSettingsMenu();
      const menu = getWozduhSettingsMenu(wrap);
      if (!menu) return;
      const willOpen = menu.hidden;
      hideWozduhSettingsMenus();
      if (willOpen) openFloatingMenu(menu, e.currentTarget);
    } else if (def.kind === 'rsx') {
      hideWozduhSettingsMenus();
      hideRiskSettingsMenu();
      const menu = getRsxSettingsMenu(wrap);
      if (!menu) return;
      const willOpen = menu.hidden;
      hideRsxSettingsMenus();
      if (willOpen) openFloatingMenu(menu, e.currentTarget);
    }
  });

  return item;
}

function renderChartLegends(context) {
  document.querySelectorAll(`.chart-legend[data-context="${context}"]`).forEach((legendEl) => {
    const pane = legendEl.dataset.pane;
    legendEl.innerHTML = '';
    const defs = CHART_LEGEND_DEFS[pane] || [];
    defs.forEach((def) => {
      legendEl.appendChild(renderLegendItem(context, pane, def, false));
    });

    // Trendlines legend + gear on live and backtest (never hide based on enabled/target flags).
    if (NAVIGATOR_PANES.includes(pane)) {
      const tlDef = { id: 'trendlines', label: 'TLine', kind: 'trendlines' };
      const tlState = loadNavigatorPaneSettings(pane, context);
      chartLegendState[context][pane].trendlines = { visible: tlState.linesVisible !== false };
      legendEl.appendChild(renderLegendItem(context, pane, tlDef, false));
    }

    if (context === 'backtest' && pane === 'price') {
      const tradeDef = { id: 'trades', label: 'Trades', kind: 'trades' };
      legendEl.appendChild(renderLegendItem(context, pane, tradeDef, true));
    }
  });
}

function initChartLegends() {
  initNavigatorPopupOkHandlers();
  ['price', 'rsx', 'wozduh'].forEach((pane) => {
    const popup = ensureNavigatorPopup(pane);
    applyNavigatorSettingsToPopup(popup, loadNavigatorPaneSettings(pane, 'backtest'));
  });
  renderChartLegends('live');
  renderChartLegends('backtest');
}

const TF_DISPLAY = {
  '1m': '1m', '2m': '2m', '3m': '3m', '5m': '5m', '15m': '15m', '30m': '30m',
  '1h': '1H', '2h': '2H', '3h': '3H', '4h': '4H',
  '1d': 'D', '1w': 'W',
  '1tick': '1 tick', '10ticks': '10 ticks', '100ticks': '100 ticks', '1000ticks': '1000 ticks',
  '1s': '1s', '5s': '5s', '10s': '10s', '15s': '15s', '30s': '30s', '45s': '45s',
};

const TF_MENU = {
  TICKS: [
    { id: '1tick', label: '1 tick' }, { id: '10ticks', label: '10 ticks' },
    { id: '100ticks', label: '100 ticks' }, { id: '1000ticks', label: '1000 ticks' },
  ],
  SECONDS: [
    { id: '1s', label: '1 second' }, { id: '5s', label: '5 seconds' },
    { id: '10s', label: '10 seconds' }, { id: '15s', label: '15 seconds' },
    { id: '30s', label: '30 seconds' }, { id: '45s', label: '45 seconds' },
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
  ],
  DAYS: [
    { id: '1d', label: '1 day' }, { id: '1w', label: '1 week' },
  ],
};

const LS_FAV_KEY = 'dashboard_tf_favorites';
const LS_TF_KEY = 'dashboard_tf_current';
const LS_PANE_KEY = 'dashboard_pane_heights';
const LS_PANE_KEY_BT = 'dashboard_pane_heights_bt';
const LS_FACTS_BAR_KEY = 'dashboard_facts_bar_visible';
const INSPECTOR_CLICK_DELAY_MS = 280;
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
const MTF_PERIOD_COLORS = {
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
};
const LIVE_STATE_CANDLE_LIMIT = 3000;
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

const RSX_DEFAULT_COLOR = '#e1d2b5';

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
    crosshairMarkerBorderColor: '#90ee90',
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
    upColor: 'rgba(8,153,129,0.55)',
    downColor: 'rgba(242,54,69,0.55)',
  },
  rsx: {
    color: RSX_DEFAULT_COLOR,
    lineWidth: 2,
    priceLineVisible: false,
    lastValueVisible: false,
  },
  rsxSignal: {
    color: '#ff9800',
    lineWidth: 1,
    lineStyle: LC.LineStyle.Dashed,
    priceLineVisible: false,
    lastValueVisible: false,
  },
  wozduhUp: {
    color: 'blue',
    lineWidth: 2,
    lineStyle: LC.LineStyle.Solid,
    title: 'wt11 (Blue)',
    priceLineVisible: false,
    lastValueVisible: false,
  },
  wozduhDown: {
    color: 'aqua',
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
    rsiPrice: { color: 'red', lineWidth: 2, title: 'RSI(C)', priceLineVisible: false, lastValueVisible: false },
    rsiHl2: { color: 'purple', lineWidth: 2, title: 'RSI(HL2)', priceLineVisible: false, lastValueVisible: false },
    rsiVolFast: { color: 'blue', lineWidth: 2, title: 'wt11 (Blue)', priceLineVisible: false, lastValueVisible: false },
    rsiVolSlow: { color: 'aqua', lineWidth: 2, title: 'wt22 (Aqua)', priceLineVisible: false, lastValueVisible: false },
    volChanMid: { color: 'orange', lineWidth: 2, title: 'Vol Chan Mid', priceLineVisible: false, lastValueVisible: false },
    volChanUp: {
      color: '#00FA9A',
      lineWidth: 1,
      lineStyle: LC.LineStyle.Dashed,
      title: 'Vol Chan Up',
      priceLineVisible: false,
      lastValueVisible: false,
    },
    volChanDn: {
      color: '#00FA9A',
      lineWidth: 1,
      lineStyle: LC.LineStyle.Dashed,
      title: 'Vol Chan Dn',
      priceLineVisible: false,
      lastValueVisible: false,
    },
    ...(window.DormantIndicators?.WOZDUX_DORMANT_LINE_STYLES || {}),
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
      volChanMid: { dataKey: 'volChanMid', style: CHART_STYLES.wozdux.volChanMid },
      volChanUp: { dataKey: 'volChanUp', style: CHART_STYLES.wozdux.volChanUp },
      volChanDn: { dataKey: 'volChanDn', style: CHART_STYLES.wozdux.volChanDn },
      ...(window.DormantIndicators?.WOZDUH_DORMANT_LINE_CONFIG || {}),
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
    vertLine: { width: 1, color: '#555', style: LC.LineStyle.Dashed },
    horzLine: { width: 1, color: '#555', style: LC.LineStyle.Dashed },
  };

  return true;
}

/** PineScript default visibility (klrscena, dada, qdada, klemarsi, etc.) */
const WOZDUH_MENU_ITEMS = [
  { prefKey: 'rsiPrice', keys: ['rsiPrice'], default: true },
  { prefKey: 'rsiHl2', keys: ['rsiHl2'], default: true },
  { prefKey: 'rsiVol', keys: ['rsiVolFast', 'rsiVolSlow'], default: true },
  { prefKey: 'volChan', keys: ['volChanMid', 'volChanUp', 'volChanDn'], default: true },
  ...((window.DormantIndicators && window.DormantIndicators.WOZDUH_DORMANT_MENU_ITEMS) || []),
];

function withSeriesDefaults(style) {
  ensureChartLibraryStyles();
  return { ...CHART_STYLES.seriesDefaults, ...style };
}

function wozduxLineSeriesOptions(def) {
  return withSeriesDefaults({
    color: def.color,
    lineWidth: def.lineWidth,
    lineStyle: def.lineStyle ?? LightweightCharts.LineStyle.Solid,
    title: '',
  });
}

const SHARED_TIME_SCALE = {
  borderColor: TV.border,
  timeVisible: true,
  secondsVisible: false,
  minBarSpacing: 0.001,
  fixLeftEdge: false,
  fixRightEdge: false,
};

function unixChartTimeToDate(time) {
  if (typeof time === 'object' && time !== null && 'year' in time) {
    return new Date(Date.UTC(time.year, time.month - 1, time.day));
  }
  return new Date(time * 1000);
}

/** Lightweight Charts localization: axis labels in the user's local timezone (wire time stays UTC seconds). */
function chartLocalizationOptions() {
  const locale = navigator.language || 'en-US';
  return {
    locale,
    timeFormatter: (time) => unixChartTimeToDate(time).toLocaleTimeString(locale, {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    }),
    dateFormatter: (time) => unixChartTimeToDate(time).toLocaleDateString(locale, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    }),
  };
}

const CHART_PRICE_SCALE_MIN_WIDTH = 70;

function sharedRightPriceScaleOptions(extra = {}) {
  return {
    borderColor: TV.border,
    autoScale: true,
    minimumWidth: CHART_PRICE_SCALE_MIN_WIDTH,
    alignLabels: true,
    borderVisible: true,
    ...extra,
  };
}

const RULER_IDLE = 0;
const RULER_MEASURING = 1;
const RULER_FIXED = 2;

function sharedChartLayout() {
  return {
    background: { color: TV.bg },
    textColor: TV.text,
    fontSize: 11,
    attributionLogo: false,
  };
}

function createPriceChartOptions(width, height) {
  return {
    autoSize: true,
    layout: sharedChartLayout(),
    localization: chartLocalizationOptions(),
    grid: {
      vertLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
      horzLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
    },
    crosshair: { ...SHARED_CROSSHAIR },
    timeScale: { ...SHARED_TIME_SCALE },
    width,
    height,
    rightPriceScale: sharedRightPriceScaleOptions({
      mode: LightweightCharts.PriceScaleMode.Logarithmic,
    }),
  };
}

function createWozduxChartOptions(width, height) {
  return {
    autoSize: true,
    layout: sharedChartLayout(),
    localization: chartLocalizationOptions(),
    grid: {
      vertLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
      horzLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
    },
    crosshair: { ...SHARED_CROSSHAIR },
    timeScale: { ...SHARED_TIME_SCALE },
    width,
    height,
    rightPriceScale: sharedRightPriceScaleOptions({
      mode: LightweightCharts.PriceScaleMode.Normal,
      scaleMargins: { top: 0.05, bottom: 0.05 },
    }),
  };
}

function createRSXChartOptions(width, height) {
  return {
    autoSize: true,
    layout: sharedChartLayout(),
    localization: chartLocalizationOptions(),
    grid: {
      vertLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
      horzLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
    },
    crosshair: { ...SHARED_CROSSHAIR },
    timeScale: { ...SHARED_TIME_SCALE },
    width,
    height,
    rightPriceScale: sharedRightPriceScaleOptions({
      mode: LightweightCharts.PriceScaleMode.Normal,
      scaleMargins: { top: 0.05, bottom: 0.05 },
    }),
  };
}

let liveChartData = {};
let backtestChartData = {};

const LOGICAL_RANGE_EPS = 0.01;
let backtestLoadedCandles = [];
let backtestNavigatorChartLines = [];
let backtestNavigatorChartMarkers = [];
let liveNavigatorResult = null;
let liveNavigatorChartLines = [];
let liveNavigatorChartMarkers = [];
let backtestTf = '15m';
let equityChart;
let equitySeries;
let lastBacktestResult = null;
let statsMode = 'backtest';
let lastTelemetrySignature = '';
let backtestAbortController = null;
let backtestRunActive = false;

let wozduxPriceLines = [];
let rsxPriceLines = [];
let tradeMarkers = [];
let sessionTrades = [];
let spikeMarkers = [];
let fibPriceLines = [];
let lastFibZones = [];
let loadedCandles = [];
let loadedOsc = [];
let liveAnnotations = [];
let currentTf = '1m';
let backendTradingTimeframe = null;
let tradingTimeframeSynced = false;
let tfFavorites = [];
let refreshTimer = null;
let historyLoading = false;
let historyHasMore = true;
let currentLiveRequestId = 0;
let navigatorRequestId = 0;
const LIVE_STATE_FETCH_TIMEOUT_MS = 10000;
let liveStateAbort = null;
let liveStateInflight = new Map();
let chartInitialized = false;
let isAppInitialized = false;
let chartType = 'candles';
let dashboardSocket = null;
let livePollSuppressedByWs = false;
let lastTickBufferLen = 0;
let orderFlowPollTimer = null;
let isUpdatingData = false;
let liveChartUpdating = false;
let isLoadingHistory = false;
let liveHistoryEpoch = 0;
let pendingHistoryLoad = null;
/** History lazy-load is armed only after the user scrolls/zooms the live chart (not on TF init). */
let liveHistoryScrollArmed = false;
/** Suppresses subscribeVisibleLogicalRangeChange history fetch during programmatic viewport updates. */
let liveHistorySuppressRangeHook = false;
const LIVE_HISTORY_SCROLL_THRESHOLD = 50;
let lastScoringData = null;
let liveScoringData = null;
let cachedSandboxMode = false;

if (typeof window !== 'undefined') {
  window.__isSettingsUpdating = false;
  window.__pendingAnchor = null;
}

function canPatchBacktestIndicatorsOnly(options = {}) {
  if (options.patchIndicatorsOnly === false) return false;
  if (options.patchIndicatorsOnly === true) return true;
  return options.preserveView === true
    && backtestLoadedCandles.length > 0
    && !!backtestChartData?.candleSeries;
}

function defaultRsxSettings() {
  return {
    length: DEFAULT_RSX_LENGTH,
    div_lookback: DEFAULT_RSX_LOOKBACK,
    signal_length: DEFAULT_RSX_SIGNAL_LENGTH,
    source: 'close',
    pivot_radius: 2,
    div_method: 'tv',
    min_price_delta_ratio: 0,
    min_osc_delta: 0,
    show_pivots: true,
  };
}

let liveRsxSettings = defaultRsxSettings();
let backtestRsxSettings = defaultRsxSettings();

let backtestAnnotations = [];
let backtestLoadedOsc = [];
let backtestHistoryHasMore = true;
let backtestHistoryLoading = false;
let backtestLastTrades = [];
let backtestChartPointsByTime = new Map();
let backtestTradeMarkerTimes = new Set();
let lockedHistoricalData = null;
let currentBacktestPayload = null;
if (typeof window !== 'undefined') {
  window.currentBacktestPayload = window.currentBacktestPayload || null;
}
const BACKTEST_HISTORY_CHUNK_LIMIT = 5000;
const LS_RISK_SETTINGS_KEY = 'dashboard_risk_settings';

const ruler = { active: false, state: RULER_IDLE, p1: null, p2: null, chartData: null };

function getActiveTabId() {
  const active = document.querySelector('.tab-content.active');
  return active?.id || 'tab-live';
}

function isBacktestTabActive() {
  return getActiveTabId() === 'tab-backtest';
}

/** Backtest workflow tabs where toolbar TF controls the backtest interval (not live). */
function isBacktestTfContext() {
  const tab = getActiveTabId();
  return tab === 'tab-backtest' || tab === 'tab-stats';
}

function isLiveTabActive() {
  return getActiveTabId() === 'tab-live';
}

function shouldPaintLiveChart() {
  return isLiveTabActive();
}

function getActiveChartData() {
  return isBacktestTabActive() ? backtestChartData : liveChartData;
}

function getBacktestInterval() {
  const el = document.getElementById('bt-interval');
  return el?.value || backtestTf;
}

function getActiveTfFromToolbar() {
  const activeBtn = document.querySelector('#tf-favorites .tf-btn.active');
  return activeBtn?.dataset?.tf || null;
}

function normalizeTf(tf) {
  return resolveTf(tf);
}

/** Client-side TF resolver (mirrors server/timeframes.go aliases). */
function resolveTf(tf) {
  const raw = String(tf || '').trim();
  if (!raw) return '';

  const lower = raw.toLowerCase();
  const alias = {
    '1min': '1m', '1minute': '1m', m: '1m',
    '1hour': '1h', h: '1h',
    d: '1d', day: '1d',
    w: '1w', week: '1w',
  };
  if (alias[lower]) return alias[lower];
  if (TF_DISPLAY[raw]) return raw;

  const all = Object.values(TF_MENU).flat();
  const hit = all.find((item) => item.id === raw || item.id.toLowerCase() === lower);
  if (hit) return hit.id;

  if (/^\d+s$/i.test(raw)) return lower;
  if (/^\d+tick(s)?$/i.test(lower)) return lower === '1tick' ? '1tick' : lower;
  if (/^\d+M$/.test(raw)) return raw;
  if (/^\d+m$/i.test(raw) && raw !== 'M') return lower;
  if (/^\d+h$/i.test(lower)) return lower;
  if (/^\d+d$/i.test(lower)) return lower;

  return raw;
}

function parseBacktestDateInput(value) {
  if (!value) return null;
  const d = new Date(`${value}T00:00:00Z`);
  return Number.isNaN(d.getTime()) ? null : d;
}

function formatBacktestDateInput(d) {
  return d.toISOString().slice(0, 10);
}

function limitBacktestDateRange(interval, startDate, endDate) {
  // Phase 5.19: no frontend clamping — server pads coarse TF history when needed.
  const end = parseBacktestDateInput(endDate) || new Date();
  return {
    startDate,
    endDate: endDate || formatBacktestDateInput(end),
    limited: false,
  };
}

function expandBacktestStartDate(startDate, endDate, days) {
  const start = parseBacktestDateInput(startDate);
  if (!start || !Number.isFinite(days) || days <= 0) return startDate;
  const expanded = new Date(start);
  expanded.setUTCDate(expanded.getUTCDate() - days);
  return formatBacktestDateInput(expanded);
}

function applyBacktestDateRangeLimits(interval) {
  const startEl = document.getElementById('bt-start');
  const endEl = document.getElementById('bt-end');
  const endDate = endEl?.value || formatBacktestDateInput(new Date());
  const startDate = startEl?.value || '';
  const clamped = limitBacktestDateRange(interval, startDate, endDate);
  if (startEl && clamped.startDate) startEl.value = clamped.startDate;
  if (endEl && clamped.endDate) endEl.value = clamped.endDate;
  return clamped;
}

function syncTradingTimeframeFromState(data) {
  const tf = resolveTf(data?.tradingTimeframe);
  if (!tf || tf === '1M') return false;
  backendTradingTimeframe = tf;
  if (tradingTimeframeSynced || isBacktestTabActive()) return false;

  tradingTimeframeSynced = true;
  if (tf === currentTf) {
    syncToolbarToActiveContext();
    return false;
  }

  currentTf = tf;
  localStorage.setItem(LS_TF_KEY, tf);
  historyHasMore = true;
  syncToolbarToActiveContext();
  wsSubscribeTf(tf);
  return true;
}

function getActiveTf() {
  if (isBacktestTfContext()) {
    return normalizeTf(getBacktestInterval() || backtestTf || getActiveTfFromToolbar());
  }
  return currentTf;
}

function applyTfToBacktestSelect(tf) {
  const el = document.getElementById('bt-interval');
  if (!el) return false;
  const hasOption = [...el.options].some((o) => o.value === tf);
  if (!hasOption) {
    const opt = document.createElement('option');
    opt.value = tf;
    opt.textContent = tfLabel(tf);
    el.appendChild(opt);
  }
  el.value = tf;
  backtestTf = tf;
  return true;
}

function defaultPriceScaleState() {
  return { logEnabled: true, autoScale: true };
}

function normalizePriceScaleView(saved) {
  if (!saved) return defaultPriceScaleState();
  if (saved.logEnabled != null || saved.autoScale != null) {
    return {
      logEnabled: saved.logEnabled !== false,
      autoScale: saved.autoScale !== false,
    };
  }
  return {
    logEnabled: saved.priceScaleMode !== 'auto',
    autoScale: saved.priceScaleMode === 'auto',
  };
}

function readPriceAutoScaleFromChart(chartData) {
  try {
    return chartData.priceChart.priceScale('right').options().autoScale !== false;
  } catch {
    return true;
  }
}

function syncPriceScaleControlsUI(chartData) {
  const controls = chartData?.scaleControlsEl;
  if (!controls) return;
  const state = chartData.priceScaleState || defaultPriceScaleState();
  const autoBtn = controls.querySelector('[data-action="auto"]');
  const logBtn = controls.querySelector('[data-action="log"]');
  if (autoBtn) autoBtn.classList.toggle('active', !!state.autoScale);
  if (logBtn) logBtn.classList.toggle('active', !!state.logEnabled);
}

function applyPriceScaleState(chartData, saved) {
  if (!chartData?.priceChart) return;
  const state = normalizePriceScaleView(saved);
  chartData.priceScaleState = state;
  chartData.priceScaleMode = state.logEnabled ? 'log' : 'normal';
  chartData.priceChart.priceScale('right').applyOptions({
    mode: state.logEnabled
      ? LightweightCharts.PriceScaleMode.Logarithmic
      : LightweightCharts.PriceScaleMode.Normal,
    autoScale: state.autoScale,
  });
  syncPriceScaleControlsUI(chartData);
}

function capturePriceScaleState(chartData) {
  const state = chartData?.priceScaleState || defaultPriceScaleState();
  return {
    logEnabled: state.logEnabled,
    autoScale: readPriceAutoScaleFromChart(chartData),
  };
}

function togglePriceLogScale(chartData) {
  if (!chartData?.priceChart) return;
  const state = { ...(chartData.priceScaleState || defaultPriceScaleState()) };
  state.logEnabled = !state.logEnabled;
  chartData.priceScaleState = state;
  chartData.priceScaleMode = state.logEnabled ? 'log' : 'normal';
  chartData.priceChart.priceScale('right').applyOptions({
    mode: state.logEnabled
      ? LightweightCharts.PriceScaleMode.Logarithmic
      : LightweightCharts.PriceScaleMode.Normal,
  });
  syncPriceScaleControlsUI(chartData);
}

function enablePriceAutoScale(chartData) {
  if (!chartData?.priceChart) return;
  const state = { ...(chartData.priceScaleState || defaultPriceScaleState()) };
  state.autoScale = true;
  chartData.priceScaleState = state;
  chartData.priceChart.priceScale('right').applyOptions({ autoScale: true });
  syncPriceScaleControlsUI(chartData);
}

function isPointerOnPriceScale(chartData, clientX) {
  const container = chartData?.elements?.chartContainer;
  if (!container || !chartData?.priceChart) return false;
  const rect = container.getBoundingClientRect();
  const scaleW = chartData.priceChart.priceScale('right').width() || CHART_PRICE_SCALE_MIN_WIDTH;
  return clientX >= rect.right - scaleW;
}

function attachPriceScaleInteractionWatch(chartData) {
  const container = chartData?.elements?.chartContainer;
  if (!container) return;
  container._priceScaleChartData = chartData;

  const syncAutoFromChart = () => {
    const active = container._priceScaleChartData;
    if (!active?.priceChart) return;
    const autoOn = readPriceAutoScaleFromChart(active);
    if (!active.priceScaleState) active.priceScaleState = defaultPriceScaleState();
    if (active.priceScaleState.autoScale !== autoOn) {
      active.priceScaleState.autoScale = autoOn;
      syncPriceScaleControlsUI(active);
    }
  };

  if (container._priceScaleWatchBound) return;
  container._priceScaleWatchBound = true;

  let draggingPriceScale = false;

  container.addEventListener('mousedown', (e) => {
    const active = container._priceScaleChartData;
    if (active && isPointerOnPriceScale(active, e.clientX)) draggingPriceScale = true;
  });
  container.addEventListener('wheel', (e) => {
    const active = container._priceScaleChartData;
    if (active && isPointerOnPriceScale(active, e.clientX)) {
      requestAnimationFrame(syncAutoFromChart);
    }
  }, { passive: true });
  container.addEventListener('dblclick', (e) => {
    const active = container._priceScaleChartData;
    if (!active || !isPointerOnPriceScale(active, e.clientX)) return;
    e.stopPropagation();
    enablePriceAutoScale(active);
  });

  const onPointerEnd = () => {
    if (!draggingPriceScale) return;
    draggingPriceScale = false;
    requestAnimationFrame(syncAutoFromChart);
  };
  container.addEventListener('mouseup', onPointerEnd);
  document.addEventListener('mouseup', onPointerEnd);
}

function initPriceScaleControls(chartData) {
  const wrap = chartData?.elements?.priceWrap;
  if (!wrap || !chartData?.priceChart) return;

  let controls = wrap.querySelector('.scale-controls');
  if (!controls) {
    controls = document.createElement('div');
    controls.className = 'scale-controls';
    controls.setAttribute('aria-label', 'Price scale controls');
    controls.innerHTML = `
      <button type="button" class="scale-btn scale-btn--auto active" data-action="auto" title="Auto scale (Y)">Auto</button>
      <button type="button" class="scale-btn scale-btn--log active" data-action="log" title="Logarithmic scale">Log</button>
    `;
    wrap.appendChild(controls);
  }
  chartData.scaleControlsEl = controls;

  if (!chartData.priceScaleState) {
    chartData.priceScaleState = defaultPriceScaleState();
  }

  if (!controls._scaleControlsBound) {
    controls._scaleControlsBound = true;
    controls.querySelector('[data-action="log"]')?.addEventListener('click', (e) => {
      e.stopPropagation();
      togglePriceLogScale(chartData);
    });
    controls.querySelector('[data-action="auto"]')?.addEventListener('click', (e) => {
      e.stopPropagation();
      enablePriceAutoScale(chartData);
    });
  }

  attachPriceScaleInteractionWatch(chartData);
  syncPriceScaleControlsUI(chartData);
}

function syncToolbarToActiveContext() {
  const activeTf = getActiveTf();
  const tfBtn = document.getElementById('tf-current-btn');
  const tfLabelEl = document.getElementById('timeframe-label');
  if (tfBtn) tfBtn.textContent = `${tfLabel(activeTf)} ▾`;
  if (tfLabelEl) tfLabelEl.textContent = tfLabel(activeTf);
  renderTfBar();
  renderTfMenu();
}

function tfLabel(id) {
  return TF_DISPLAY[id] || id;
}

function tfSortKey(id) {
  const s = id.toLowerCase();
  if (s.includes('tick')) return 100 + (parseInt(s, 10) || 1);
  if (/^\d+s$/.test(s)) return 1000 + parseInt(s, 10);
  if (/^\d+m$/.test(s)) return 2000 + parseInt(s, 10);
  if (/^\d+h$/.test(s)) return 3000 + parseInt(s, 10);
  if (s === '1d') return 4001;
  if (s === '1w') return 4002;
  return 9000;
}

function sortFavorites() {
  tfFavorites.sort((a, b) => tfSortKey(a) - tfSortKey(b));
}

function loadFavorites() {
  try {
    const raw = localStorage.getItem(LS_FAV_KEY);
    tfFavorites = raw ? JSON.parse(raw) : [...DEFAULT_FAVS];
  } catch {
    tfFavorites = [...DEFAULT_FAVS];
  }
  tfFavorites = tfFavorites.filter((id) => id !== '1M');
  sortFavorites();
}

function saveFavorites() {
  sortFavorites();
  localStorage.setItem(LS_FAV_KEY, JSON.stringify(tfFavorites));
}

function isFavorite(id) {
  return tfFavorites.includes(id);
}

function toggleFavorite(id) {
  if (isFavorite(id)) {
    tfFavorites = tfFavorites.filter((f) => f !== id);
  } else {
    tfFavorites.push(id);
  }
  saveFavorites();
  renderTfBar();
  renderTfMenu();
}

function rsxContextFromWrap(wrap) {
  return wrap?.id === 'bt-rsx-wrap' ? 'backtest' : 'live';
}

function oscContextFromWrap(wrap) {
  return wrap?.id === 'bt-osc-wrap' ? 'backtest' : 'live';
}

function getActiveUiContext() {
  return getNavigatorContext();
}

function getRsxSettingsState(context = 'live') {
  return context === 'backtest' ? backtestRsxSettings : liveRsxSettings;
}

function setRsxSettingsState(context, settings) {
  const normalized = normalizeRsxSettingsFromAPI(settings, getRsxSettingsState(context));
  if (context === 'backtest') backtestRsxSettings = normalized;
  else liveRsxSettings = normalized;
  return normalized;
}

function rsxStorageKey(context) {
  return context === 'backtest' ? LS_RSX_SETTINGS_BACKTEST_KEY : LS_RSX_SETTINGS_LIVE_KEY;
}

function persistRsxSettings(context, settings) {
  localStorage.setItem(rsxStorageKey(context), JSON.stringify(settings));
}

function loadRsxSettingsFromStorage(context) {
  try {
    const raw = localStorage.getItem(rsxStorageKey(context));
    if (raw) {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === 'object') {
        return setRsxSettingsState(context, parsed);
      }
    }
  } catch {
    /* use defaults */
  }

  if (context === 'live') {
    const migrated = defaultRsxSettings();
    try {
      const lb = parseInt(localStorage.getItem(LS_RSX_LOOKBACK_KEY), 10);
      const sl = parseInt(localStorage.getItem(LS_RSX_SIGNAL_LENGTH_KEY), 10);
      const len = parseInt(localStorage.getItem(LS_RSX_LENGTH_KEY), 10);
      if (Number.isFinite(lb)) migrated.div_lookback = clampRsxDivLookback(lb);
      if (Number.isFinite(sl)) migrated.signal_length = clampRsxSignalLength(sl);
      if (Number.isFinite(len)) migrated.length = clampRsxLength(len);
    } catch {
      /* noop */
    }
    return setRsxSettingsState('live', migrated);
  }

  return setRsxSettingsState(context, defaultRsxSettings());
}

function getRsxWrap(context = 'live') {
  return document.getElementById(context === 'backtest' ? 'bt-rsx-wrap' : 'rsx-wrap');
}

function getOscWrap(context = 'live') {
  return document.getElementById(context === 'backtest' ? 'bt-osc-wrap' : 'osc-wrap');
}

function clampRsxLength(val) {
  const n = parseInt(val, 10);
  if (!Number.isFinite(n)) return DEFAULT_RSX_LENGTH;
  return Math.min(MAX_RSX_LENGTH, Math.max(MIN_RSX_LENGTH, n));
}

function clampRsxDivLookback(val) {
  const n = parseInt(val, 10);
  if (!Number.isFinite(n)) return DEFAULT_RSX_LOOKBACK;
  return Math.min(MAX_RSX_DIV_LOOKBACK, Math.max(MIN_RSX_DIV_LOOKBACK, n));
}

function clampRsxSignalLength(val) {
  const n = parseInt(val, 10);
  if (!Number.isFinite(n)) return DEFAULT_RSX_SIGNAL_LENGTH;
  return Math.min(MAX_RSX_SIGNAL_LENGTH, Math.max(MIN_RSX_SIGNAL_LENGTH, n));
}

function getRsxSettingsMenu(wrap) {
  if (!wrap) return null;
  return wrap.querySelector('.indicator-settings-menu');
}

function rsxShowPivotsFrom(settings, fallback = true) {
  if (typeof settings?.show_pivots === 'boolean') return settings.show_pivots;
  if (typeof settings?.showPivots === 'boolean') return settings.showPivots;
  return fallback;
}

function normalizeRsxSettingsFromAPI(raw, defaults = defaultRsxSettings()) {
  if (!raw || typeof raw !== 'object') return coerceRsxSettingsForAPI(defaults, defaults);
  const num = (v, fallback) => {
    const n = Number(v);
    return Number.isFinite(n) ? n : fallback;
  };
  const divMethod = raw.div_method || raw.divMethod || defaults.div_method;
  return coerceRsxSettingsForAPI({
    length: num(raw.length ?? raw.rsxLength, defaults.length),
    div_lookback: num(raw.div_lookback ?? raw.divLookback, defaults.div_lookback),
    signal_length: num(raw.signal_length ?? raw.signalLineLength, defaults.signal_length),
    source: raw.source || raw.rsxSource || defaults.source,
    pivot_radius: num(raw.pivot_radius ?? raw.pivotRadius, defaults.pivot_radius),
    div_method: divMethod,
    min_price_delta_ratio: num(raw.min_price_delta_ratio ?? raw.minPriceDeltaRatio, defaults.min_price_delta_ratio),
    min_osc_delta: num(raw.min_osc_delta ?? raw.minOscDelta, defaults.min_osc_delta),
    show_pivots: rsxShowPivotsFrom(raw, rsxShowPivotsFrom(defaults, true)),
  }, defaults);
}

function readRsxFloatDeltaInput(el, fallback = 0) {
  if (!el) return fallback;
  const n = Number(el.value);
  return Number.isFinite(n) ? n : fallback;
}

function readRsxSettingsFromMenu(contextOrWrap, context = 'live') {
  const wrap = typeof contextOrWrap === 'string'
    ? getRsxWrap(contextOrWrap)
    : (contextOrWrap?.id === 'rsx-wrap' || contextOrWrap?.id === 'bt-rsx-wrap'
      ? contextOrWrap
      : contextOrWrap?.closest?.('.rsx-wrap') || contextOrWrap);
  const ctx = typeof contextOrWrap === 'string' ? contextOrWrap : (context || rsxContextFromWrap(wrap));
  const defaults = getRsxSettingsState(ctx);
  if (!wrap) return coerceRsxSettingsForAPI({ ...defaults }, defaults);
  const source = wrap.querySelector('.rsx-source-select')?.value || defaults.source;
  const divMethod = wrap.querySelector('.rsx-div-method-select')?.value || defaults.div_method;
  const showPivotsEl = wrap.querySelector('.rsx-show-pivots-chk');
  return coerceRsxSettingsForAPI({
    length: clampRsxLength(Number(wrap.querySelector('.rsx-length-input')?.value)),
    div_lookback: clampRsxDivLookback(Number(wrap.querySelector('.rsx-div-lookback-input')?.value)),
    signal_length: clampRsxSignalLength(Number(wrap.querySelector('.rsx-signal-length-input')?.value)),
    source: source === 'hlc3' ? 'hlc3' : 'close',
    pivot_radius: clampRsxPivotRadius(Number(wrap.querySelector('.rsx-pivot-radius-input')?.value)),
    div_method: divMethod === 'fractal' ? 'fractal' : 'tv',
    min_price_delta_ratio: readRsxFloatDeltaInput(
      wrap.querySelector('.rsx-min-price-delta-input'),
      defaults.min_price_delta_ratio ?? 0,
    ),
    min_osc_delta: readRsxFloatDeltaInput(
      wrap.querySelector('.rsx-min-osc-delta-input'),
      defaults.min_osc_delta ?? 0,
    ),
    show_pivots: showPivotsEl ? showPivotsEl.checked : rsxShowPivotsFrom(defaults, true),
  }, defaults);
}

function coerceRsxSettingsForAPI(settings, defaults = defaultRsxSettings()) {
  const pivotRadius = Number(clampRsxPivotRadius(Number(settings?.pivot_radius ?? settings?.pivotRadius)));
  const minPriceDelta = Number(settings?.min_price_delta_ratio ?? settings?.minPriceDeltaRatio ?? 0);
  const minOscDelta = Number(settings?.min_osc_delta ?? settings?.minOscDelta ?? 0);
  return {
    length: Number(clampRsxLength(Number(settings?.length))),
    div_lookback: Number(clampRsxDivLookback(Number(settings?.div_lookback))),
    signal_length: Number(clampRsxSignalLength(Number(settings?.signal_length))),
    source: settings?.source === 'hlc3' ? 'hlc3' : 'close',
    pivot_radius: pivotRadius,
    pivotRadius,
    div_method: settings?.div_method === 'fractal' ? 'fractal' : 'tv',
    min_price_delta_ratio: Number.isFinite(minPriceDelta) ? minPriceDelta : 0,
    min_osc_delta: Number.isFinite(minOscDelta) ? minOscDelta : 0,
    minPriceDeltaRatio: Number.isFinite(minPriceDelta) ? minPriceDelta : 0,
    minOscDelta: Number.isFinite(minOscDelta) ? minOscDelta : 0,
    show_pivots: rsxShowPivotsFrom(settings, rsxShowPivotsFrom(defaults, true)),
    showPivots: rsxShowPivotsFrom(settings, rsxShowPivotsFrom(defaults, true)),
  };
}

function clampRsxPivotRadius(val) {
  const n = parseInt(val, 10);
  if (!Number.isFinite(n)) return 2;
  return Math.min(10, Math.max(1, n));
}

function applyRsxSettingsToMenu(context, settings, defaults = defaultRsxSettings()) {
  const wrap = getRsxWrap(context);
  if (!wrap || !settings) return;
  const s = normalizeRsxSettingsFromAPI(settings, defaults);
  const lengthEl = wrap.querySelector('.rsx-length-input');
  const lookbackEl = wrap.querySelector('.rsx-div-lookback-input');
  const signalEl = wrap.querySelector('.rsx-signal-length-input');
  const sourceEl = wrap.querySelector('.rsx-source-select');
  const pivotEl = wrap.querySelector('.rsx-pivot-radius-input');
  const methodEl = wrap.querySelector('.rsx-div-method-select');
  const showPivotsEl = wrap.querySelector('.rsx-show-pivots-chk');
  const minPriceDeltaEl = wrap.querySelector('.rsx-min-price-delta-input');
  const minOscDeltaEl = wrap.querySelector('.rsx-min-osc-delta-input');
  if (lengthEl && Number.isFinite(s.length)) lengthEl.value = String(s.length);
  if (lookbackEl && Number.isFinite(s.div_lookback)) lookbackEl.value = String(s.div_lookback);
  if (signalEl && Number.isFinite(s.signal_length)) signalEl.value = String(s.signal_length);
  if (sourceEl && s.source) sourceEl.value = s.source;
  if (pivotEl && Number.isFinite(s.pivot_radius)) pivotEl.value = String(s.pivot_radius);
  if (methodEl && s.div_method) methodEl.value = s.div_method;
  if (minPriceDeltaEl && Number.isFinite(s.min_price_delta_ratio)) {
    minPriceDeltaEl.value = String(s.min_price_delta_ratio);
  }
  if (minOscDeltaEl && Number.isFinite(s.min_osc_delta)) {
    minOscDeltaEl.value = String(s.min_osc_delta);
  }
  if (showPivotsEl && typeof s.show_pivots === 'boolean') showPivotsEl.checked = s.show_pivots;
  syncRsxPivotRadiusFieldState(context);
}

function syncRsxPivotRadiusFieldState(contextOrWrap) {
  const wrap = typeof contextOrWrap === 'string'
    ? getRsxWrap(contextOrWrap)
    : (contextOrWrap?.id === 'rsx-wrap' || contextOrWrap?.id === 'bt-rsx-wrap'
      ? contextOrWrap
      : contextOrWrap?.closest?.('.rsx-wrap'));
  if (!wrap) return;
  const method = wrap.querySelector('.rsx-div-method-select')?.value || 'tv';
  const pivotEl = wrap.querySelector('.rsx-pivot-radius-input');
  const row = pivotEl?.closest('.setting-row');
  const isTV = method !== 'fractal';
  if (pivotEl) pivotEl.disabled = isTV;
  if (row) row.classList.toggle('setting-row--disabled', isTV);
}

function applyRsxSettingsToContextMenu(context, settings) {
  applyRsxSettingsToMenu(context, settings, getRsxSettingsState(context));
}

let rsxSettingsSyncTimer = null;
let rsxSettingsFetchVersion = 0;
let rsxChartReloadVersion = 0;

let settingsUpdateTimeout = null;
let backtestAutoUpdateInFlight = false;
let backtestIntervalChangeInFlight = false;

async function pushRsxSettingsToServer(settings) {
  const payload = coerceRsxSettingsForAPI(settings);
  try {
    const resp = await fetch('/api/settings/indicators', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!resp.ok) {
      const errText = await resp.text().catch(() => '');
      throw new Error(`RSX settings POST failed (${resp.status}): ${errText || 'no body'}`);
    }
    return coerceRsxSettingsForAPI(await resp.json());
  } catch (err) {
    console.warn('Failed to push RSX settings:', err);
    throw err;
  }
}

async function fetchRsxIndicatorSettings() {
  const version = ++rsxSettingsFetchVersion;
  const localBeforeFetch = { ...getRsxSettingsState('live') };
  try {
    const resp = await fetch('/api/settings/indicators');
    if (!resp.ok || version !== rsxSettingsFetchVersion) return;
    const serverSettings = await resp.json();
    if (version !== rsxSettingsFetchVersion) return;
    const merged = normalizeRsxSettingsFromAPI(
      { ...serverSettings, ...localBeforeFetch },
      defaultRsxSettings(),
    );
    const applied = setRsxSettingsState('live', merged);
    persistRsxSettings('live', applied);
    applyRsxSettingsToContextMenu('live', applied);
    if (JSON.stringify(coerceRsxSettingsForAPI(serverSettings)) !== JSON.stringify(coerceRsxSettingsForAPI(applied))) {
      try {
        await pushRsxSettingsToServer(applied);
      } catch (err) {
        console.warn('Failed to sync local RSX settings to server:', err);
      }
    }
  } catch (err) {
    if (version === rsxSettingsFetchVersion) {
      console.warn('Failed to load RSX indicator settings:', err);
    }
  }
}

async function reloadRsxChartFromServer() {
  if (!shouldPaintLiveChart()) return;
  const reloadVersion = ++rsxChartReloadVersion;
  try {
    const { warmingUp, data } = await fetchLiveState();
    if (reloadVersion !== rsxChartReloadVersion) return;
    if (warmingUp || !data?.oscillators?.length) return;

    if (Array.isArray(data.annotations)) {
      liveAnnotations = (Array.isArray(data.annotations) ? data.annotations : [])
        .map(normalizeAnnotation)
        .filter(Boolean);
    }

    beginDataUpdate();
    loadedOsc = alignOscillatorsToCandles(mergeOsc([], data.oscillators), loadedCandles);
    applyRsxData(loadedOsc, liveChartData, liveAnnotations);
    endDataUpdate();
    forceSyncChartTimeScales(liveChartData);
  } catch (err) {
    console.warn('Failed to reload RSX chart:', err);
  }
}

function appendBacktestRsxSettingsToParams(params) {
  const settings = coerceRsxSettingsForAPI(getRsxSettingsState('backtest'));
  params.set('rsx_length', String(settings.length));
  params.set('rsx_signal_length', String(settings.signal_length));
  params.set('rsx_source', settings.source);
  params.set('rsx_method', settings.div_method);
  params.set('rsx_pivot_radius', String(settings.pivot_radius));
  params.set('rsx_div_lookback', String(settings.div_lookback));
  params.set('min_price_delta_ratio', String(settings.min_price_delta_ratio));
  params.set('min_osc_delta', String(settings.min_osc_delta));
  return params;
}

async function syncBacktestRsxSettingsLocal() {
  const settings = readRsxSettingsFromMenu('backtest');
  const applied = setRsxSettingsState('backtest', settings);
  persistRsxSettings('backtest', applied);
  applyRsxSettingsToContextMenu('backtest', applied);
  return applied;
}

async function syncRsxIndicatorSettings(context = 'live') {
  const settings = readRsxSettingsFromMenu(context);
  const applied = setRsxSettingsState(context, settings);
  persistRsxSettings(context, applied);
  applyRsxSettingsToContextMenu(context, applied);

  if (context === 'backtest') {
    return applied;
  }

  rsxSettingsFetchVersion += 1;
  const serverApplied = await pushRsxSettingsToServer(applied);
  if (serverApplied) {
    const liveApplied = setRsxSettingsState('live', normalizeRsxSettingsFromAPI(
      { ...serverApplied, show_pivots: applied.show_pivots },
      applied,
    ));
    persistRsxSettings('live', liveApplied);
    applyRsxSettingsToContextMenu('live', liveApplied);
  }

  await reloadRsxChartFromServer();
}

function scheduleRsxSettingsSync(context = 'live') {
  if (rsxSettingsSyncTimer) clearTimeout(rsxSettingsSyncTimer);
  rsxSettingsSyncTimer = setTimeout(() => {
    rsxSettingsSyncTimer = null;
    syncRsxIndicatorSettings(context);
  }, 350);
}

function scheduleIndicatorSettingsAutoUpdate() {
  clearTimeout(settingsUpdateTimeout);
  settingsUpdateTimeout = setTimeout(() => {
    settingsUpdateTimeout = null;
    if (typeof runBacktestAutoUpdatePipeline === 'function') {
      runBacktestAutoUpdatePipeline().catch((err) => {
        console.error('[UI] Indicator settings auto-update failed:', err);
      });
    } else {
      runBacktest(false, { preserveView: true, patchIndicatorsOnly: true }).catch((err) => {
        console.error('[UI] Indicator settings auto-update failed:', err);
      });
    }
  }, 500);
}

function flushIndicatorSettingsAutoUpdate() {
  clearTimeout(settingsUpdateTimeout);
  settingsUpdateTimeout = null;
  setBacktestLoading(true);
  return runBacktestAutoUpdatePipeline();
}

function persistBacktestRsxFromMenu() {
  const settings = readRsxSettingsFromMenu('backtest');
  const applied = setRsxSettingsState('backtest', settings);
  persistRsxSettings('backtest', applied);
  return applied;
}

async function runBacktestAutoUpdatePipeline() {
  if (backtestAutoUpdateInFlight) {
    clearTimeout(settingsUpdateTimeout);
    settingsUpdateTimeout = setTimeout(() => {
      settingsUpdateTimeout = null;
      runBacktestAutoUpdatePipeline();
    }, 500);
    return;
  }
  backtestAutoUpdateInFlight = true;
  try {
    window.__isSettingsUpdating = true;
    console.log('[Pipeline] starting auto-update, navigators:', window.currentBacktestPayload?.settings?.navigators);
    await syncBacktestRsxSettingsLocal();
    await runBacktest(false, {
      manageLoading: false,
      skipSettingsPush: true,
      preserveView: true,
      patchIndicatorsOnly: true,
    });
  } catch (error) {
    console.error('Pipeline Fatal Error:', error);
    alert(`🚨 ОШИБКА В БРАУЗЕРЕ: ${error.message}\nПосмотри консоль (F12) для деталей.`);
  } finally {
    window.__isSettingsUpdating = false;
    backtestAutoUpdateInFlight = false;
    setBacktestLoading(false);
    setBacktestButtonDisabled(false);
  }
}

function initIndicatorSettingsAutoUpdate() {
  if (window.__indicatorSettingsAutoUpdateBound) return;
  window.__indicatorSettingsAutoUpdateBound = true;

  const onIndicatorSettingChange = (event) => {
    const target = event?.target;
    if (!target?.matches('input, select, textarea')) return;

    const navPopup = target.closest('.navigator-popup');
    if (navPopup) {
      const pane = navPopup.dataset.pane || navPopup.id?.replace(/^popup-/, '') || '';
      if (!pane) return;
      const context = getNavigatorContext();
      try {
        const settings = normalizeNavigatorPaneSettings(
          readNavigatorSettingsFromPopup(navPopup, pane),
          pane,
        );
        saveNavigatorPaneSettings(pane, settings, context);
        injectBacktestPayloadFromPaneUI(pane);
      } catch (err) {
        console.warn('[UI] Navigator settings persist failed:', err);
      }
      if (context === 'backtest') {
        setBacktestLoading(true);
      }
      scheduleNavigatorAutoUpdate();
      return;
    }

    if (target.closest('#bt-rsx-wrap')) {
      persistBacktestRsxFromMenu();
      setBacktestLoading(true);
      scheduleIndicatorSettingsAutoUpdate();
      return;
    }

    if (target.closest('#rsx-wrap')) {
      scheduleRsxSettingsSync('live');
      return;
    }

    if (target.matches('.wozduh-chk')) {
      const wrap = target.closest('.osc-wrap');
      if (!wrap) return;
      const context = oscContextFromWrap(wrap);
      const menu = getWozduhSettingsMenu(wrap);
      if (!menu) return;
      const prefs = readWozduhPrefsFromMenu(menu);
      saveWozduhPrefs(context, prefs);
      applyWozduhVisibilityToChart(
        context === 'backtest' ? backtestChartData : liveChartData,
        context,
      );
    }
  };

  document.addEventListener('input', onIndicatorSettingChange);
  document.addEventListener('change', onIndicatorSettingChange);
}

function initBacktestAutoUpdateDelegation() {
  initIndicatorSettingsAutoUpdate();
}

async function refreshBacktestIndicatorSeries() {
  if (!backtestChartData?.rsxSeries || backtestLoadedCandles.length === 0) {
    return false;
  }

  const firstTime = chartTime(backtestLoadedCandles[0].time);
  const lastTime = chartTime(backtestLoadedCandles[backtestLoadedCandles.length - 1].time);
  if (firstTime == null || lastTime == null) return false;

  const symbol = document.getElementById('bt-symbol')?.value.trim() || 'BTCUSDT';
  const interval = getBacktestInterval();
  const intervalMs = estimateCandleIntervalSec(backtestLoadedCandles) * 1000;
  const endTimeMs = (lastTime < 1e12 ? lastTime * 1000 : lastTime) + intervalMs * 2;
  const limit = Math.min(
    BACKTEST_HISTORY_CHUNK_LIMIT,
    backtestLoadedCandles.length + 300,
  );

  const params = new URLSearchParams({
    symbol,
    interval,
    endTime: String(endTimeMs),
    limit: String(limit),
  });
  appendBacktestRsxSettingsToParams(params);

  const resp = await fetch(`/api/history/chunk?${params.toString()}`, { cache: 'no-store' });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok || !Array.isArray(data.chartData) || data.chartData.length === 0) {
    console.warn('refreshBacktestIndicatorSeries failed:', resp.status);
    return false;
  }

  const inRange = (t) => t != null && t >= firstTime && t <= lastTime;
  const osc = chartPointsToOsc(data.chartData).filter((p) => inRange(chartTime(p.time)));
  if (osc.length === 0) return false;

  backtestLoadedOsc = mergeOsc(backtestLoadedOsc, osc);

  const mappedRSX = mapRSXData(backtestLoadedOsc);
  backtestChartData.rsxSeries.setData(
    mappedRSX.map(({ time, value, color }) => ({ time, value, color })),
  );
  if (backtestChartData.rsxSignalSeries) {
    backtestChartData.rsxSignalSeries.setData(mapRSXSignalData(backtestLoadedOsc));
  }

  applyUniversalAnnotations(
    getChartAnnotationPanes(backtestChartData),
    data.annotations,
    { rsx: new Set(mappedRSX.map((d) => d.time)) },
  );

  return true;
}

function setBacktestLoading(visible) {
  const el = document.getElementById('backtest-loading');
  if (!el) return;
  el.classList.toggle('hidden', !visible);
}

function getIntervalMs(tf) {
  const raw = String(tf || '1m').toLowerCase();
  const unit = raw.slice(-1);
  const val = parseInt(raw, 10);
  if (!Number.isFinite(val) || val <= 0) return 60000;
  switch (unit) {
    case 's': return val * 1000;
    case 'm': return val * 60000;
    case 'h': return val * 3600000;
    case 'd': return val * 86400000;
    case 'w': return val * 604800000;
    default: return 60000;
  }
}

/** Reject live ticks when viewing deep history (microscope mode). Times in Unix seconds. */
function isLiveTickGapTooLarge(lastTimeSec, newTimeSec) {
  if (lastTimeSec == null || newTimeSec == null) return false;
  if (newTimeSec <= lastTimeSec) return false;
  const maxGapMs = getIntervalMs(currentTf) * 5;
  const gapMs = (newTimeSec * 1000) - (lastTimeSec * 1000);
  return gapMs > maxGapMs;
}

function apiQueryParams(extra = {}) {
  const params = {
    tf: currentTf,
    rsxLookback: liveRsxSettings.div_lookback,
    limit: LIVE_STATE_CANDLE_LIMIT,
    ...extra,
  };

  const anchor = window.__pendingAnchor;
  if (anchor?.type === 'center' && anchor.targetTime != null && params.endTime == null) {
    const halfLimit = LIVE_STATE_CANDLE_LIMIT / 2;
    const tfMs = getIntervalMs(currentTf);
    const centerSec = Number(anchor.targetTime);
    if (Number.isFinite(centerSec) && tfMs > 0) {
      const centerMs = centerSec * 1000;

      // Смещение в будущее на половину запрашиваемых баров нового ТФ
      let targetEndTimeMs = centerMs + (halfLimit * tfMs);

      // Глубокая история: не уходим в будущее дальше текущего момента
      if (targetEndTimeMs > Date.now()) {
        targetEndTimeMs = Date.now();
      }

      // /api/state endTime is Unix seconds (server SSOT).
      params.endTime = Math.floor(targetEndTimeMs / 1000);
    }
  }

  return params;
}

function setTfDropdownOpen(open) {
  const dd = document.getElementById('tf-dropdown');
  if (!dd) return;
  dd.hidden = !open;
  dd.classList.toggle('open', open);
}

function runWithSuppressedHistoryLoad(fn) {
  liveHistorySuppressRangeHook = true;
  try {
    return fn();
  } finally {
    queueMicrotask(() => {
      requestAnimationFrame(() => {
        liveHistorySuppressRangeHook = false;
      });
    });
  }
}

function disarmLiveHistoryScroll() {
  liveHistoryScrollArmed = false;
  pendingHistoryLoad = null;
}

function attachLiveHistoryScrollArm() {
  const root = document.getElementById('live-chart-container');
  if (!root || root._historyScrollArmBound) return;
  root._historyScrollArmBound = true;
  const arm = () => {
    liveHistoryScrollArmed = true;
  };
  root.addEventListener('wheel', arm, { passive: true });
  root.addEventListener('pointerdown', arm, { passive: true });
}

function beginDataUpdate() {
  liveChartUpdating = true;
}

function endDataUpdate(delayMs = 50) {
  setTimeout(() => {
    liveChartUpdating = false;
    if (pendingHistoryLoad) {
      const job = pendingHistoryLoad;
      pendingHistoryLoad = null;
      scheduleHistoryLoad(job.range, job.options);
    }
  }, delayMs);
}

function abortLiveStateFetch() {
  if (liveStateAbort) {
    liveStateAbort.abort();
    liveStateAbort = null;
  }
  liveStateInflight.clear();
}

function liveStateRequestKey(options = {}) {
  if (options.navigatorsOnly) return `nav:${currentTf}`;
  return `tf:${currentTf}`;
}

async function fetchLiveState(options = {}) {
  const userTfChange = options.userTfChange === true;
  const key = liveStateRequestKey(options);

  if (userTfChange) {
    abortLiveStateFetch();
    liveStateAbort = new AbortController();
  } else if (liveStateInflight.has(key)) {
    return liveStateInflight.get(key);
  }

  const signal = userTfChange ? liveStateAbort.signal : options.signal;
  const promise = fetchState({ ...options, signal }).finally(() => {
    if (liveStateInflight.get(key) === promise) {
      liveStateInflight.delete(key);
    }
  });

  if (!userTfChange) {
    liveStateInflight.set(key, promise);
  }
  return promise;
}

const LIVE_CHART_SELECTORS = {
  priceWrap: 'price-wrap',
  oscWrap: 'osc-wrap',
  rsxWrap: 'rsx-wrap',
  chartContainer: 'chart-container',
  oscContainer: 'osc-container',
  rsxContainer: 'rsx-container',
};

const BACKTEST_CHART_SELECTORS = {
  priceWrap: 'bt-price-wrap',
  oscWrap: 'bt-osc-wrap',
  rsxWrap: 'bt-rsx-wrap',
  chartContainer: 'bt-chart-container',
  oscContainer: 'bt-osc-container',
  rsxContainer: 'bt-rsx-container',
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

function destroyChartInstance(chartData) {
  if (!chartData?.allCharts?.length) return;
  chartData.allCharts.forEach((chart) => {
    try {
      chart.remove();
    } catch {
      /* noop */
    }
  });
  ['oscWrap', 'rsxWrap', 'priceWrap'].forEach((key) => {
    const wrap = chartData.elements?.[key];
    if (wrap?._paneResizeObs) {
      wrap._paneResizeObs.disconnect();
      wrap._paneResizeObs = null;
    }
  });
}

function exitFullscreenPane() {
  document.querySelectorAll('.fullscreen-pane').forEach((el) => {
    el.classList.remove('fullscreen-pane');
  });
  handleResize();
}

function toggleFullscreenPane(wrapEl, chartData) {
  if (!wrapEl) return;
  const entering = !wrapEl.classList.contains('fullscreen-pane');
  exitFullscreenPane();
  if (entering) wrapEl.classList.add('fullscreen-pane');
  requestAnimationFrame(() => {
    const root = document.getElementById(chartData.containerId);
    if (root) resizeChartForContainer(chartData, root);
  });
}

function initPaneFullscreen(chartData) {
  if (!chartData?.elements) return;
  ['priceWrap', 'oscWrap', 'rsxWrap'].forEach((key) => {
    const wrap = chartData.elements[key];
    if (!wrap || wrap._fullscreenBound) return;
    wrap._fullscreenBound = true;
    wrap.addEventListener('dblclick', (e) => {
      if (e.target.closest('.indicator-settings-menu, button')) return;
      if (chartData._inspectorClickTimer) {
        clearTimeout(chartData._inspectorClickTimer);
        chartData._inspectorClickTimer = null;
      }
      toggleFullscreenPane(wrap, chartData);
    });
  });
}

function initPaneVerticalResize(chartData) {
  if (!chartData?.elements) return;
  ['priceWrap', 'oscWrap', 'rsxWrap'].forEach((key) => {
    const wrap = chartData.elements[key];
    if (!wrap || wrap._paneResizeObs) return;
    wrap._paneResizeObs = new ResizeObserver(() => {
      const root = document.getElementById(chartData.containerId);
      if (root) resizeChartForContainer(chartData, root);
    });
    wrap._paneResizeObs.observe(wrap);
  });
}

function setupChartPaneUX(chartData) {
  initPaneVerticalResize(chartData);
  initPaneFullscreen(chartData);
}

function applyIndicatorConfigStyles(chartData) {
  if (!chartData) return;
  if (!ensureChartLibraryStyles()) return;

  const priceCfg = INDICATOR_CONFIG.price;
  chartData.candleSeries?.applyOptions({
    ...priceCfg.candle,
    crosshairMarkerRadius: CHART_STYLES.seriesDefaults.crosshairMarkerRadius,
    crosshairMarkerBorderWidth: CHART_STYLES.seriesDefaults.crosshairMarkerBorderWidth,
    crosshairMarkerBorderColor: CHART_STYLES.seriesDefaults.crosshairMarkerBorderColor,
  });
  chartData.barSeries?.applyOptions({
    ...priceCfg.bar,
    crosshairMarkerRadius: CHART_STYLES.seriesDefaults.crosshairMarkerRadius,
    crosshairMarkerBorderWidth: CHART_STYLES.seriesDefaults.crosshairMarkerBorderWidth,
    crosshairMarkerBorderColor: CHART_STYLES.seriesDefaults.crosshairMarkerBorderColor,
  });
  chartData.lineSeries?.applyOptions(priceCfg.line);
  chartData.volumeSeries?.applyOptions(priceCfg.volume);

  Object.entries(INDICATOR_CONFIG.wozduh.lines).forEach(([, lineCfg]) => {
    const series = chartData.wozduxSeries?.[lineCfg.dataKey];
    if (series) series.applyOptions(wozduxLineSeriesOptions(lineCfg.style));
  });

  (INDICATOR_CONFIG.wozduh.areas || []).forEach((areaCfg) => {
    const entry = chartData.wozduhAreas?.[areaCfg.id];
    if (!entry?.series) return;
    entry.series.applyOptions({
      topColor: areaCfg.topColor,
      bottomColor: areaCfg.bottomColor,
      lineColor: areaCfg.lineColor || 'transparent',
      lineWidth: areaCfg.lineWidth ?? 0,
      priceLineVisible: false,
      lastValueVisible: false,
      crosshairMarkerVisible: false,
    });
    if (areaCfg.bottom != null && areaCfg.topLineKey == null) {
      entry.series.applyOptions({
        baseValue: { type: 'price', price: areaCfg.bottom },
      });
    }
  });

  const rsxMain = INDICATOR_CONFIG.rsx.lines.rsx_main;
  chartData.rsxSeries?.applyOptions(withSeriesDefaults(rsxMain.style));
  const rsxSignalLine = INDICATOR_CONFIG.rsx.lines.rsx_signal;
  chartData.rsxSignalSeries?.applyOptions(withSeriesDefaults(rsxSignalLine.style));

  (INDICATOR_CONFIG.rsx.areas || []).forEach((areaCfg) => {
    const entry = chartData.rsxAreas?.[areaCfg.id];
    if (!entry?.series) return;
    entry.series.applyOptions({
      topColor: areaCfg.topColor,
      bottomColor: areaCfg.bottomColor,
      lineColor: areaCfg.lineColor || 'transparent',
      lineWidth: areaCfg.lineWidth ?? 0,
      priceLineVisible: false,
      lastValueVisible: false,
      crosshairMarkerVisible: false,
    });
    if (areaCfg.bottom != null) {
      entry.series.applyOptions({
        baseValue: { type: 'price', price: areaCfg.bottom },
      });
    }
  });
}

function reinitBacktestChart() {
  destroyChartInstance(backtestChartData);
  backtestChartData = initProfessionalChart('backtest-chart-container', {
    selectors: BACKTEST_CHART_SELECTORS,
    overlayTrades: true,
    navigatorPlugin: true,
    tradeMarkers: true,
  }) || {};
  applyWozduhVisibilityToChart(backtestChartData, 'backtest');
  attachProfessionalChartSync(backtestChartData, {
    crosshairPriceSeries: () => backtestChartData.candleSeries,
    onVisibleRangeChange: (range) => {
      if (getRulerChartData() === backtestChartData) updateRulerOverlay(backtestChartData);
      maybeLoadBacktestHistory(range);
    },
  });
  attachRulerToChart(backtestChartData);
  attachBacktestScoringCrosshair(backtestChartData);
  renderChartLegends('backtest');
  return backtestChartData;
}

function normalizePriceLineLevel(level) {
  const lineColor = level.color || TV.text;
  const { labelBackgroundColor, labelTextColor, ...rest } = level;
  return {
    ...rest,
    title: level.title ?? '',
    axisLabelVisible: level.axisLabelVisible !== false,
    axisLabelColor: level.axisLabelColor ?? TV.bg,
    axisLabelTextColor: level.axisLabelTextColor ?? lineColor,
  };
}

function addLevelLines(series, levels) {
  return (levels || []).map((level) => series.createPriceLine(normalizePriceLineLevel(level)));
}

function createAreaSeries(chart, areaCfg) {
  const series = chart.addAreaSeries({
    topColor: areaCfg.topColor,
    bottomColor: areaCfg.bottomColor,
    lineColor: areaCfg.lineColor || 'transparent',
    lineWidth: areaCfg.lineWidth ?? 0,
    priceLineVisible: false,
    lastValueVisible: false,
    crosshairMarkerVisible: false,
  });
  if (areaCfg.bottom != null && areaCfg.topLineKey == null) {
    series.applyOptions({
      baseValue: { type: 'price', price: areaCfg.bottom },
    });
  }
  return series;
}

function createStaticAreaData(times, top) {
  return (times || [])
    .filter((time) => time != null)
    .map((time) => ({ time: Number(time), value: top }));
}

function buildDynamicWtBandData(osc, topKey, bottomKey) {
  const map = new Map();
  (osc || []).forEach((p) => {
    const time = chartTime(p?.time ?? p?.Time);
    const top = Number(p[topKey]);
    const bottom = Number(p[bottomKey]);
    if (time == null || isWarmupOscValue(top) || isWarmupOscValue(bottom)) return;
    map.set(Number(time), { time: Number(time), value: Math.max(top, bottom) });
  });
  return Array.from(map.values()).sort((a, b) => a.time - b.time);
}

function applyStaticAreas(chartData, paneKey, times, osc) {
  const paneCfg = INDICATOR_CONFIG[paneKey];
  const areaMap = chartData[`${paneKey}Areas`];
  if (!paneCfg?.areas || !areaMap) return;

  paneCfg.areas.forEach((areaCfg) => {
    const entry = areaMap[areaCfg.id];
    if (!entry?.series) return;

    if (areaCfg.topLineKey && areaCfg.bottomLineKey) {
      entry.series.setData(buildDynamicWtBandData(osc, areaCfg.topLineKey, areaCfg.bottomLineKey));
      return;
    }

    if (areaCfg.top != null) {
      entry.series.setData(createStaticAreaData(times, areaCfg.top));
    }
  });
}

function initProfessionalChart(containerId, options = {}) {
  if (typeof LightweightCharts === 'undefined') return null;

  const selectors = options.selectors || LIVE_CHART_SELECTORS;
  const root = document.getElementById(containerId);
  const chartContainer = document.getElementById(selectors.chartContainer);
  const oscContainer = document.getElementById(selectors.oscContainer);
  const rsxContainer = document.getElementById(selectors.rsxContainer);
  const priceWrap = document.getElementById(selectors.priceWrap);
  const oscWrap = document.getElementById(selectors.oscWrap);
  const rsxWrap = document.getElementById(selectors.rsxWrap);

  if (!root || !chartContainer || !oscContainer || !rsxContainer) return null;
  if (!ensureChartLibraryStyles()) return null;

  const CHART_MIN_PRICE_PANE_H = 400;
  const CHART_MIN_OSC_PANE_H = 150;
  const width = Math.max(chartContainer.clientWidth || 0, root.clientWidth || 0, 800);
  const priceH = Math.max(priceWrap?.clientHeight || 0, CHART_MIN_PRICE_PANE_H);
  const oscH = Math.max(oscWrap?.clientHeight || 0, CHART_MIN_OSC_PANE_H);
  const rsxH = Math.max(rsxWrap?.clientHeight || 0, CHART_MIN_OSC_PANE_H);

  const priceChart = LightweightCharts.createChart(
    chartContainer,
    createPriceChartOptions(width, priceH),
  );
  const oscChart = LightweightCharts.createChart(
    oscContainer,
    createWozduxChartOptions(oscContainer.clientWidth || width, oscH),
  );
  const rsxChart = LightweightCharts.createChart(
    rsxContainer,
    createRSXChartOptions(rsxContainer.clientWidth || width, rsxH),
  );

  const priceCfg = INDICATOR_CONFIG.price;
  const candleSeries = priceChart.addCandlestickSeries(priceCfg.candle);
  const barSeries = priceChart.addBarSeries(priceCfg.bar);
  const lineSeries = createMTFOverlaySeries(priceChart, {
    ...priceCfg.line,
    visible: priceCfg.line?.visible ?? false,
  });
  const volumeSeries = priceChart.addHistogramSeries(priceCfg.volume);
  priceChart.priceScale('volume').applyOptions(priceCfg.volumeScale);
  priceChart.priceScale('right').applyOptions({
    ...priceCfg.priceScale,
    minimumWidth: CHART_PRICE_SCALE_MIN_WIDTH,
  });
  oscChart.priceScale('right').applyOptions({ minimumWidth: CHART_PRICE_SCALE_MIN_WIDTH });
  rsxChart.priceScale('right').applyOptions({ minimumWidth: CHART_PRICE_SCALE_MIN_WIDTH });

  const wozduhAreas = {};
  INDICATOR_CONFIG.wozduh.areas.forEach((areaCfg) => {
    wozduhAreas[areaCfg.id] = { series: createAreaSeries(oscChart, areaCfg), cfg: areaCfg };
  });

  const wozduxSeries = {};
  Object.entries(INDICATOR_CONFIG.wozduh.lines).forEach(([seriesKey, lineCfg]) => {
    wozduxSeries[seriesKey] = oscChart.addLineSeries(
      wozduxLineSeriesOptions(lineCfg.style),
    );
    wozduxSeries[lineCfg.dataKey] = wozduxSeries[seriesKey];
  });

  const wozduhLevelAnchor = wozduxSeries.rsiPrice || wozduxSeries.wozduh_wt1;
  const wozduxPriceLinesLocal = addLevelLines(wozduhLevelAnchor, INDICATOR_CONFIG.wozduh.levels);

  const rsxAreas = {};
  INDICATOR_CONFIG.rsx.areas.forEach((areaCfg) => {
    rsxAreas[areaCfg.id] = { series: createAreaSeries(rsxChart, areaCfg), cfg: areaCfg };
  });

  const rsxLineCfg = INDICATOR_CONFIG.rsx.lines.rsx_main;
  const rsxSeries = rsxChart.addLineSeries(withSeriesDefaults(rsxLineCfg.style));
  const rsxSignalCfg = INDICATOR_CONFIG.rsx.lines.rsx_signal;
  const rsxSignalSeries = rsxChart.addLineSeries(withSeriesDefaults(rsxSignalCfg.style));
  const rsxPriceLinesLocal = addLevelLines(rsxSeries, INDICATOR_CONFIG.rsx.levels);

  let entryMarkerSeries = null;
  if (options.overlayTrades) {
    entryMarkerSeries = createMTFOverlaySeries(priceChart, {
      color: 'transparent',
      lineWidth: 0,
    });
  }

  let priceNavigatorPlugin = null;
  let rsxNavigatorPlugin = null;
  let wozduhNavigatorPlugin = null;
  let tradeMarkerPlugin = null;

  if (options.navigatorPlugin && typeof TrendlinePrimitive !== 'undefined') {
    priceNavigatorPlugin = new TrendlinePrimitive();
    candleSeries.attachPrimitive(priceNavigatorPlugin);

    rsxNavigatorPlugin = new TrendlinePrimitive();
    rsxSeries.attachPrimitive(rsxNavigatorPlugin);

    const wozduhSeries = wozduxSeries.rsiVolSlow || wozduxSeries.wozduh_wt2;
    if (wozduhSeries) {
      wozduhNavigatorPlugin = new TrendlinePrimitive();
      wozduhSeries.attachPrimitive(wozduhNavigatorPlugin);
    }
  }

  if (options.tradeMarkers && typeof TradeMarkerPrimitive !== 'undefined') {
    tradeMarkerPlugin = new TradeMarkerPrimitive();
    candleSeries.attachPrimitive(tradeMarkerPlugin);
  }

  const allCharts = [priceChart, oscChart, rsxChart];

  const result = {
    chart: priceChart,
    containerId,
    root,
    priceChart,
    oscChart,
    rsxChart,
    candleSeries,
    priceSeries: candleSeries,
    barSeries,
    lineSeries,
    volumeSeries,
    rsxSeries,
    rsxSignalSeries,
    wozduhUpSeries: wozduxSeries.rsiVolFast || wozduxSeries.wozduh_wt1,
    wozduhDownSeries: wozduxSeries.rsiVolSlow || wozduxSeries.wozduh_wt2,
    wozduxSeries,
    wozduhAreas,
    rsxAreas,
    wozduxPriceLines: wozduxPriceLinesLocal,
    rsxPriceLines: rsxPriceLinesLocal,
    entryMarkerSeries,
    priceNavigatorPlugin,
    rsxNavigatorPlugin,
    wozduhNavigatorPlugin,
    tradeMarkerPlugin,
    mtfNavigatorLayers: { price: {}, rsx: {}, wozduh: {} },
    allCharts,
    elements: { priceWrap, oscWrap, rsxWrap, chartContainer, oscContainer, rsxContainer },
    syncingTimeScale: false,
    priceScaleState: defaultPriceScaleState(),
    priceScaleMode: 'log',
    rulerAttached: false,
  };

  setupChartPaneUX(result);
  initPriceScaleControls(result);
  return result;
}

function logicalRangesEqual(a, b, eps = LOGICAL_RANGE_EPS) {
  if (!a || !b) return false;
  return Math.abs(a.from - b.from) < eps && Math.abs(a.to - b.to) < eps;
}

function syncVisibleLogicalRange(targetChart, range, options = {}) {
  if (!targetChart?.timeScale || !range) return;
  const timeScale = targetChart.timeScale();
  const currentRange = timeScale.getVisibleLogicalRange();
  if (logicalRangesEqual(currentRange, range)) return;
  const rangeOpts = options.animate === false ? { animate: false } : undefined;
  timeScale.setVisibleLogicalRange(range, rangeOpts);
}

function captureLiveViewportSnapshot() {
  if (!liveChartData?.priceChart?.timeScale) return null;
  try {
    const logicalRange = liveChartData.priceChart.timeScale().getVisibleLogicalRange();
    if (!logicalRange) return null;
    return { logicalRange };
  } catch {
    return null;
  }
}

function restoreLiveViewportSnapshot(snapshot, options = {}) {
  if (!snapshot?.logicalRange || !liveChartData?.allCharts?.length) return;
  const animate = options.animate === true;
  const lockAutoScale = options.lockAutoScale !== false;
  liveChartData.allCharts.forEach((chart) => {
    const apply = () => syncVisibleLogicalRange(chart, snapshot.logicalRange, { animate });
    if (lockAutoScale && chart === liveChartData.priceChart) {
      window.Viewport?.lockPriceAutoScaleDuring?.(chart, apply);
    } else {
      apply();
    }
  });
}

function restoreLiveViewportSnapshotTwice(snapshot) {
  restoreLiveViewportSnapshot(snapshot, { animate: false });
  requestAnimationFrame(() => {
    restoreLiveViewportSnapshot(snapshot, { animate: false });
    syncLiveChartPanesFromPrice();
  });
}

function syncVisibleTimeRange(targetChart, range) {
  if (!targetChart?.timeScale || !range) return;
  const timeScale = targetChart.timeScale();
  const currentRange = timeScale.getVisibleRange();
  if (logicalRangesEqual(currentRange, range)) return;
  timeScale.setVisibleRange(range);
}

function applyLiveViewportAfterData(chartData, klines, options = {}) {
  if (!chartData?.allCharts?.length || !klines.length) return;

  const {
    isPrepend = false,
    prevRange = null,
  } = options;

  const applyRange = (range, animate = false) => {
    chartData.allCharts.forEach((chart) => syncVisibleLogicalRange(chart, range, { animate }));
  };

  if (isPrepend) {
    return;
  }

  if (window.__pendingAnchor) {
    runWithSuppressedHistoryLoad(() => {
      window.Viewport?.restoreViewportToCharts(chartData, chartData.candleSeries, window.__pendingAnchor, {
        candles: klines,
        chartTime,
        animate: false,
      });
    });
    window.__pendingAnchor = null;
    return;
  }

  if (prevRange) {
    runWithSuppressedHistoryLoad(() => applyRange(prevRange, false));
  }
}

function syncChartGroupLogicalRange(charts, sourceChart, range) {
  if (!range || !charts?.length) return;
  charts.forEach((targetChart) => {
    if (targetChart !== sourceChart) {
      syncVisibleLogicalRange(targetChart, range);
    }
  });
}

function syncChartGroupTimeRange(charts, sourceChart, range) {
  if (!range || !charts?.length) return;
  charts.forEach((targetChart) => {
    if (targetChart !== sourceChart) {
      syncVisibleTimeRange(targetChart, range);
    }
  });
}

function syncPaneCrosshairs(chartGroups) {
  if (!chartGroups?.length) return;

  chartGroups.forEach((sourceData) => {
    const sourceChart = sourceData?.chart;
    if (!sourceChart || sourceChart._crosshairSyncBound) return;
    sourceChart._crosshairSyncBound = true;

    sourceChart.subscribeCrosshairMove((param) => {
      if (liveChartUpdating) return;
      if (!param.sourceEvent) return;

      const container = sourceData.container;
      if (!container?.matches) return;
      if (!container.matches(':hover') && !container.matches(':active')) return;

      chartGroups.forEach((targetData) => {
        const targetChart = targetData?.chart;
        if (!targetChart || targetChart === sourceChart) return;

        if (!param.point || param.time === undefined) {
          if (typeof targetChart.clearCrosshairPosition === 'function') {
            targetChart.clearCrosshairPosition();
          }
          return;
        }

        const series = typeof targetData.seriesGetter === 'function'
          ? targetData.seriesGetter()
          : null;
        if (!series) return;

        const price = seriesValueAtTime(series, param.time);
        if (price != null) {
          targetChart.setCrosshairPosition(price, param.time, series);
        }
      });
    });
  });
}

function attachProfessionalChartSync(chartData, hooks = {}) {
  if (!chartData?.allCharts?.length) return;

  const priceSeriesGetter = hooks.crosshairPriceSeries || (() => chartData.candleSeries);
  const wozduhSeriesGetter = () => chartData.wozduxSeries?.rsiPrice || chartData.wozduhUpSeries;
  const rsxSeriesGetter = () => chartData.rsxSeries;

  const { chartContainer, oscContainer, rsxContainer } = chartData.elements || {};

  chartData.allCharts.forEach((sourceChart) => {
    sourceChart.timeScale().subscribeVisibleLogicalRangeChange((range) => {
      if (liveChartUpdating || liveHistorySuppressRangeHook || !range) return;

      syncChartGroupLogicalRange(chartData.allCharts, sourceChart, range);

      if (typeof hooks.onVisibleRangeChange === 'function') {
        hooks.onVisibleRangeChange(range);
      }
    });
  });

  const crosshairPeers = [
    { chart: chartData.priceChart, seriesGetter: priceSeriesGetter, container: chartContainer },
    { chart: chartData.oscChart, seriesGetter: wozduhSeriesGetter, container: oscContainer },
    { chart: chartData.rsxChart, seriesGetter: rsxSeriesGetter, container: rsxContainer },
  ].filter((entry) => entry.chart && entry.container);

  syncPaneCrosshairs(crosshairPeers);
}

function resetChartCrosshair(chart) {
  if (!chart) return;
  chart.applyOptions({ crosshair: { mode: LightweightCharts.CrosshairMode.Normal } });
  if (typeof chart.crosshair === 'function') {
    chart.crosshair().applyOptions({ mode: LightweightCharts.CrosshairMode.Normal });
  }
}

function applyChartPanelResize(chart, width, height) {
  if (!chart) return;
  const opts = { width: Math.max(1, width) };
  if (height > 0) opts.height = height;
  chart.applyOptions(opts);
  resetChartCrosshair(chart);
}

function resizeChartInstance(chartData, rootEl) {
  if (!chartData?.elements || !rootEl) return;
  const rootWidth = rootEl.clientWidth;
  const { chartContainer, oscContainer, rsxContainer, priceWrap, oscWrap, rsxWrap } = chartData.elements;

  applyChartPanelResize(
    chartData.priceChart,
    chartContainer?.clientWidth || rootWidth,
    priceWrap?.clientHeight,
  );
  applyChartPanelResize(
    chartData.oscChart,
    oscContainer?.clientWidth || rootWidth,
    oscWrap?.clientHeight,
  );
  applyChartPanelResize(
    chartData.rsxChart,
    rsxContainer?.clientWidth || rootWidth,
    rsxWrap?.clientHeight,
  );
}

function resizeChartForContainer(chartData, container) {
  if (!chartData?.chart || !container) return;
  resizeChartInstance(chartData, container);
}

function handleResize() {
  const activeTab = getActiveTabId();

  if (activeTab === 'tab-live') {
    resizeChartForContainer(
      liveChartData,
      document.getElementById('live-chart-container'),
    );
  } else if (activeTab === 'tab-backtest') {
    resizeChartForContainer(
      backtestChartData,
      document.getElementById('backtest-chart-container'),
    );
  }

  if (ruler.active) {
    updateRulerOverlay(getRulerChartData());
  }
}

function fitChartInstance(chartData) {
  if (!chartData?.priceChart?.timeScale) return;
  chartData.priceChart.timeScale().fitContent();
  forceSyncChartTimeScales(chartData);
}

function fitProfessionalChart(chartData) {
  if (!chartData?.chart) return;

  const containerId = chartData.containerId;
  const container = containerId
    ? document.getElementById(containerId)
    : null;
  if (container) {
    resizeChartForContainer(chartData, container);
  }
  fitChartInstance(chartData);
}

function forceSyncChartTimeScales(chartData) {
  if (!chartData?.priceChart?.timeScale || !chartData?.allCharts?.length) return;
  if (liveChartUpdating) return;

  const applyRange = () => {
    if (liveChartUpdating) return false;
    const range = chartData.priceChart.timeScale().getVisibleLogicalRange();
    if (!range || range.from == null || range.to == null) return false;

    chartData.allCharts.forEach((chart) => {
      syncVisibleLogicalRange(chart, range);
    });
    return true;
  };

  if (!applyRange()) {
    requestAnimationFrame(() => {
      applyRange();
      requestAnimationFrame(applyRange);
    });
    return;
  }
  requestAnimationFrame(applyRange);
}

function syncTimeScaleToAll(_sourceChart, range) {
  if (!range || !liveChartData?.allCharts) return;
  if (liveChartUpdating) return;

  liveChartData.allCharts.forEach((chart) => {
    syncVisibleLogicalRange(chart, range);
  });
  requestAnimationFrame(() => forceSyncChartTimeScales(liveChartData));
}

function seriesValueAtTime(series, time) {
  const data = series.data();
  for (let i = data.length - 1; i >= 0; i--) {
    const bar = data[i];
    if (bar.time <= time) {
      return bar.value ?? bar.close ?? null;
    }
  }
  return null;
}

function activePriceSeries() {
  if (chartType === 'bars') return liveChartData.barSeries;
  if (chartType === 'line') return liveChartData.lineSeries;
  return liveChartData.candleSeries;
}

function initCharts() {
  if (typeof LightweightCharts === 'undefined') return false;
  if (liveChartData.chart) return true;

  liveChartData = initProfessionalChart('live-chart-container', {
    selectors: LIVE_CHART_SELECTORS,
    navigatorPlugin: true,
  }) || {};
  if (!liveChartData.chart) return false;

  wozduxPriceLines = liveChartData.wozduxPriceLines;
  rsxPriceLines = liveChartData.rsxPriceLines;

  attachLiveHistoryScrollArm();

  attachProfessionalChartSync(liveChartData, {
    crosshairPriceSeries: activePriceSeries,
    onVisibleRangeChange: (range) => {
      if (getRulerChartData() === liveChartData) updateRulerOverlay(liveChartData);
      scheduleHistoryLoad(range);
    },
  });

  backtestChartData = initProfessionalChart('backtest-chart-container', {
    selectors: BACKTEST_CHART_SELECTORS,
    overlayTrades: true,
    navigatorPlugin: true,
    tradeMarkers: true,
  }) || {};
  applyWozduhVisibilityToChart(backtestChartData, 'backtest');

  attachProfessionalChartSync(backtestChartData, {
    crosshairPriceSeries: () => backtestChartData.candleSeries,
    onVisibleRangeChange: (range) => {
      if (getRulerChartData() === backtestChartData) updateRulerOverlay(backtestChartData);
      maybeLoadBacktestHistory(range);
    },
  });

  if (!window.__chartResizeBound) {
    window.__chartResizeBound = true;
    window.addEventListener('resize', () => {
      handleResize();
      resizeEquityChart();
    });
  }

  initRsxSettings();
  initWozduhSettings();
  initChartLegends();
  initRuler();
  initPaneResize();
  loadPaneHeights();
  return true;
}

function resizeAllCharts() {
  handleResize();
}

function validPaneFlexGrow(value, fallback) {
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? n : fallback;
}

function loadPaneHeightsForStack(stackKey) {
  const cfg = PANE_STACK_CONFIG[stackKey];
  if (!cfg) return;
  try {
    const raw = localStorage.getItem(cfg.lsKey);
    const defaults = cfg.defaults;
    const h = raw ? JSON.parse(raw) : defaults;
    const price = validPaneFlexGrow(h?.price, defaults.price);
    const osc = validPaneFlexGrow(h?.osc, defaults.osc);
    const rsx = validPaneFlexGrow(h?.rsx, defaults.rsx);
    const priceEl = document.getElementById(cfg.price);
    const oscEl = document.getElementById(cfg.osc);
    const rsxEl = document.getElementById(cfg.rsx);
    if (priceEl) priceEl.style.flex = `${price} 1 0`;
    if (oscEl) oscEl.style.flex = `${osc} 1 0`;
    if (rsxEl) rsxEl.style.flex = `${rsx} 1 0`;
  } catch { /* noop */ }
}

function loadPaneHeights() {
  loadPaneHeightsForStack('live');
  loadPaneHeightsForStack('backtest');
}

function savePaneHeightsForStack(stackKey) {
  const cfg = PANE_STACK_CONFIG[stackKey];
  if (!cfg) return;
  const priceEl = document.getElementById(cfg.price);
  const oscEl = document.getElementById(cfg.osc);
  const rsxEl = document.getElementById(cfg.rsx);
  if (!priceEl || !oscEl || !rsxEl) return;
  const price = parseFloat(getComputedStyle(priceEl).flexGrow) || cfg.defaults.price;
  const osc = parseFloat(getComputedStyle(oscEl).flexGrow) || cfg.defaults.osc;
  const rsx = parseFloat(getComputedStyle(rsxEl).flexGrow) || cfg.defaults.rsx;
  localStorage.setItem(cfg.lsKey, JSON.stringify({ price, osc, rsx }));
}

function initPaneResize() {
  document.querySelectorAll('.pane-resize').forEach((handle) => {
    if (handle._paneResizeBound) return;
    handle._paneResizeBound = true;
    const stackKey = handle.dataset.paneStack || 'live';
    const cfg = PANE_STACK_CONFIG[stackKey];
    if (!cfg) return;

    handle.addEventListener('mousedown', (e) => {
      e.preventDefault();
      const kind = handle.dataset.resize;
      const priceWrap = document.getElementById(cfg.price);
      const oscWrap = document.getElementById(cfg.osc);
      const rsxWrap = document.getElementById(cfg.rsx);
      if (!priceWrap || !oscWrap || !rsxWrap) return;

      const startY = e.clientY;
      const startPrice = priceWrap.getBoundingClientRect().height;
      const startOsc = oscWrap.getBoundingClientRect().height;
      const startRsx = rsxWrap.getBoundingClientRect().height;
      handle.classList.add('dragging');

      function onMove(ev) {
        const dy = ev.clientY - startY;
        const minH = 72;
        if (kind === 'price-osc') {
          const newPrice = Math.max(minH, startPrice + dy);
          const newOsc = Math.max(minH, startOsc - dy);
          priceWrap.style.flex = `0 0 ${newPrice}px`;
          oscWrap.style.flex = `0 0 ${newOsc}px`;
        } else if (kind === 'osc-rsx') {
          const newOsc = Math.max(minH, startOsc + dy);
          const newRsx = Math.max(minH, startRsx - dy);
          oscWrap.style.flex = `0 0 ${newOsc}px`;
          rsxWrap.style.flex = `0 0 ${newRsx}px`;
        }
        resizeAllCharts();
      }

      function onUp() {
        handle.classList.remove('dragging');
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
        const price = priceWrap.getBoundingClientRect().height;
        const osc = oscWrap.getBoundingClientRect().height;
        const rsx = rsxWrap.getBoundingClientRect().height;
        priceWrap.style.flex = `${price} 1 0`;
        oscWrap.style.flex = `${osc} 1 0`;
        rsxWrap.style.flex = `${rsx} 1 0`;
        savePaneHeightsForStack(stackKey);
        resizeAllCharts();
      }

      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', onUp);
    });
  });
}

function initControls(options = {}) {
  const { useServerTf = false } = options;
  loadFavorites();
  if (!useServerTf) {
    currentTf = resolveTf(localStorage.getItem(LS_TF_KEY) || '1m') || '1m';
    if (currentTf === '1M') {
      currentTf = '1w';
      localStorage.setItem(LS_TF_KEY, currentTf);
    }
  }
  renderTfBar();
  renderTfMenu();
  initTfBarInteraction();

  const toggles = {
    'tog-jurik': () => [liveChartData.rsxSeries],
    'tog-volume': () => [liveChartData.volumeSeries],
  };

  Object.entries(toggles).forEach(([id, getSeriesList]) => {
    const el = document.getElementById(id);
    if (!el) {
      console.warn(`[initControls] #${id} not found`);
      return;
    }
    const applyVisibility = () => {
      el.closest('.ind-toggle')?.classList.toggle('active', el.checked);
      getSeriesList().forEach((series) => {
        if (series) series.applyOptions({ visible: el.checked });
      });
    };
    applyVisibility();
    el.addEventListener('change', applyVisibility);
  });

  const togSpike = document.getElementById('tog-spike');
  if (togSpike) {
    togSpike.addEventListener('change', (e) => {
      e.target.closest('.ind-toggle')?.classList.toggle('active', e.target.checked);
      applyAllMarkers();
    });
  } else {
    console.warn('[initControls] #tog-spike not found');
  }

  const togFib = document.getElementById('tog-fib');
  if (togFib) {
    togFib.addEventListener('change', (e) => {
      e.target.closest('.ind-toggle')?.classList.toggle('active', e.target.checked);
      renderFibZones(lastFibZones);
    });
  } else {
    console.warn('[initControls] #tog-fib not found');
  }

  const rulerBtn = document.getElementById('ruler-btn');
  if (rulerBtn) {
    rulerBtn.addEventListener('click', toggleRuler);
  } else {
    console.warn('[initControls] #ruler-btn not found');
  }
  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape') return;
    if (document.querySelector('.fullscreen-pane')) {
      exitFullscreenPane();
      return;
    }
    if (ruler.active) resetRuler();
  });

  const chartTypeGroup = document.getElementById('chart-type-group');
  if (chartTypeGroup) {
    chartTypeGroup.querySelectorAll('.seg-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        setChartType(btn.dataset.chart);
        chartTypeGroup.querySelectorAll('.seg-btn').forEach((b) => b.classList.remove('active'));
        btn.classList.add('active');
      });
    });
  } else {
    console.warn('[initControls] #chart-type-group not found');
  }

  document.getElementById('btn-clear-cache')?.addEventListener('click', async () => {
    if (!confirm('Очистить кэш базы данных и памяти на сервере?')) return;
    try {
      const resp = await fetch('/api/cache/clear', { method: 'POST' });
      if (resp.ok) {
        alert('Кэш успешно очищен!');
      } else {
        alert('Ошибка очистки: ' + await resp.text());
      }
    } catch (err) {
      console.error('Ошибка:', err);
    }
  });
}

function hidePanelSettingsMenus() {
  hideRsxSettingsMenus();
  hideWozduhSettingsMenus();
  hideRiskSettingsMenu();
  hideAllNavigatorPopups();
}

function isPanelSettingsInteractionTarget(target) {
  if (!target?.closest) return false;
  return !!(
    target.closest('.indicator-settings-menu')
    || target.closest('.risk-menu-wrap')
    || target.closest('.rsx-settings-toggle')
    || target.closest('.wozduh-settings-toggle')
    || target.closest('.legend-gear')
    || target.closest('.navigator-popup')
    || target.closest('.chart-legend')
  );
}

function getSettingsMenuFocusables(menu) {
  if (!menu) return [];
  return [...menu.querySelectorAll(
    'input:not([type="hidden"]), select, button.rsx-save-btn, button.risk-save-btn',
  )].filter((el) => !el.disabled && el.type !== 'hidden');
}

function handleSettingsMenuEnterNavigation(event) {
  const target = event.target;
  const menu = target.closest('.indicator-settings-menu');
  if (!menu || menu.hidden) return;

  if (target.matches('button.rsx-save-btn, button.risk-save-btn')) {
    event.preventDefault();
    target.click();
    return;
  }

  if (!target.matches('input, select')) return;

  event.preventDefault();
  target.blur();

  const fields = getSettingsMenuFocusables(menu);
  const idx = fields.indexOf(target);
  if (idx >= 0 && idx < fields.length - 1) {
    fields[idx + 1].focus();
    return;
  }

  const saveBtn = menu.querySelector('.rsx-save-btn, .risk-save-btn');
  if (saveBtn) {
    saveBtn.focus();
    return;
  }

  menu.hidden = true;
}

function initPanelSettingsOutsideClose() {
  if (window.__panelSettingsOutsideBound) return;
  window.__panelSettingsOutsideBound = true;

  document.addEventListener('mousedown', (event) => {
    if (window.__menuDragging) return;
    if (isPanelSettingsInteractionTarget(event.target)) return;
    hidePanelSettingsMenus();
  });
}

function initPanelSettingsEnterNavigation() {
  if (window.__panelSettingsEnterBound) return;
  window.__panelSettingsEnterBound = true;

  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Enter') return;
    if (!event.target?.closest('.indicator-settings-menu')) return;
    handleSettingsMenuEnterNavigation(event);
  });
}

function menuFloatDirection(menu) {
  return menu?.dataset?.floatDirection === 'up' ? 'up' : 'down';
}

function positionFloatingMenu(menu, anchorEl, direction = 'down') {
  if (!menu || !anchorEl) return;
  const gap = 5;
  const rect = anchorEl.getBoundingClientRect();
  const wasHidden = menu.hidden;

  menu.hidden = false;
  menu.style.visibility = 'hidden';
  menu.classList.add('indicator-settings-menu--floating');
  menu.style.position = 'fixed';
  menu.style.left = '0';
  menu.style.top = '0';
  const menuH = menu.offsetHeight || 240;
  const menuW = menu.offsetWidth || 196;
  menu.style.visibility = '';

  let top;
  if (direction === 'up') {
    top = Math.max(8, rect.top - menuH - gap);
  } else {
    top = Math.min(rect.bottom + gap, window.innerHeight - menuH - 8);
  }

  const maxLeft = window.innerWidth - menuW - 8;
  const left = Math.max(8, Math.min(rect.left, maxLeft));

  menu.style.top = `${top}px`;
  menu.style.left = `${left}px`;
  menu.style.right = 'auto';
  menu.style.bottom = 'auto';
  menu.style.zIndex = '20000';
  menu.hidden = wasHidden;
}

function openFloatingMenu(menu, anchorEl) {
  positionFloatingMenu(menu, anchorEl, menuFloatDirection(menu));
  menu.hidden = false;
}

function initFloatingMenuDrag(menu) {
  if (!menu || menu._dragBound) return;
  menu._dragBound = true;
  const handle = menu.querySelector('.indicator-settings-menu__drag-handle');
  if (!handle) return;

  let dragging = false;
  let startX = 0;
  let startY = 0;
  let startLeft = 0;
  let startTop = 0;

  const onMove = (event) => {
    if (!dragging) return;
    const dx = event.clientX - startX;
    const dy = event.clientY - startY;
    const menuW = menu.offsetWidth || 196;
    const menuH = menu.offsetHeight || 240;
    const left = Math.max(8, Math.min(startLeft + dx, window.innerWidth - menuW - 8));
    const top = Math.max(8, Math.min(startTop + dy, window.innerHeight - menuH - 8));
    menu.style.left = `${left}px`;
    menu.style.top = `${top}px`;
  };

  const stopDrag = () => {
    if (!dragging) return;
    dragging = false;
    window.__menuDragging = false;
    document.removeEventListener('mousemove', onMove);
    document.removeEventListener('mouseup', stopDrag);
  };

  handle.addEventListener('mousedown', (event) => {
    if (event.button !== 0) return;
    event.preventDefault();
    event.stopPropagation();
    const rect = menu.getBoundingClientRect();
    dragging = true;
    window.__menuDragging = true;
    startX = event.clientX;
    startY = event.clientY;
    startLeft = rect.left;
    startTop = rect.top;
    menu.style.position = 'fixed';
    menu.style.left = `${startLeft}px`;
    menu.style.top = `${startTop}px`;
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', stopDrag);
  });
}

function notifyChartsLayoutChange() {
  requestAnimationFrame(() => {
    handleResize();
    try {
      window.dispatchEvent(new Event('resize'));
    } catch { /* noop */ }
  });
}

function syncFactsBarVisibility() {
  const wrap = document.getElementById('telemetry-bar-wrap');
  const btn = document.getElementById('facts-toggle');
  const onStats = getActiveTabId() === 'tab-stats';
  if (!wrap) return;
  wrap.classList.toggle('facts-visible', factsBarUserVisible && !onStats);
  wrap.classList.toggle('is-hidden', onStats);
  if (btn) {
    btn.classList.toggle('active', factsBarUserVisible && !onStats);
    btn.disabled = onStats;
  }
  if (liveChartData?.chart || backtestChartData?.chart) {
    notifyChartsLayoutChange();
  }
}

function toggleFactsPanel() {
  factsBarUserVisible = !factsBarUserVisible;
  try {
    localStorage.setItem(LS_FACTS_BAR_KEY, factsBarUserVisible ? '1' : '0');
  } catch { /* noop */ }
  syncFactsBarVisibility();
}

function initFactsBar() {
  const wrap = document.getElementById('telemetry-bar-wrap');
  if (!wrap) {
    console.warn('[initFactsBar] #telemetry-bar-wrap not found');
    return;
  }

  try {
    factsBarUserVisible = localStorage.getItem(LS_FACTS_BAR_KEY) === '1';
  } catch {
    factsBarUserVisible = false;
  }

  syncFactsBarVisibility();

  const btn = document.getElementById('facts-toggle');
  if (btn) {
    btn.addEventListener('click', () => toggleFactsPanel());
  }
}

function hideRiskSettingsMenu() {
  const riskMenu = document.getElementById('risk-settings-menu');
  if (riskMenu) riskMenu.hidden = true;
}

function readRiskSettingsFromForm() {
  return {
    risk_per_trade: parseFloat(document.getElementById('risk-per-trade')?.value) || 1,
    max_drawdown: parseFloat(document.getElementById('risk-max-drawdown')?.value) || 5,
    leverage: parseInt(document.getElementById('risk-leverage')?.value, 10) || 10,
    stop_loss_type: document.getElementById('risk-stop-loss-type')?.value || 'fractal_atr',
    atr_multiplier: parseFloat(document.getElementById('risk-atr-multiplier')?.value) || 1.5,
  };
}

function applyRiskSettingsToForm(settings) {
  if (!settings) return;
  const map = [
    ['risk-per-trade', settings.risk_per_trade],
    ['risk-max-drawdown', settings.max_drawdown],
    ['risk-leverage', settings.leverage],
    ['risk-atr-multiplier', settings.atr_multiplier],
  ];
  map.forEach(([id, val]) => {
    const el = document.getElementById(id);
    if (el && val != null) el.value = String(val);
  });
  const typeEl = document.getElementById('risk-stop-loss-type');
  if (typeEl && settings.stop_loss_type) typeEl.value = settings.stop_loss_type;
}

async function fetchRiskSettings() {
  try {
    const resp = await fetch('/api/settings/risk');
    if (!resp.ok) return;
    const settings = await resp.json();
    applyRiskSettingsToForm(settings);
    localStorage.setItem(LS_RISK_SETTINGS_KEY, JSON.stringify(settings));
  } catch (err) {
    console.warn('Failed to load risk settings:', err);
    try {
      const raw = localStorage.getItem(LS_RISK_SETTINGS_KEY);
      if (raw) applyRiskSettingsToForm(JSON.parse(raw));
    } catch { /* noop */ }
  }
}

async function saveRiskSettings() {
  const menu = document.getElementById('risk-settings-menu');
  const active = document.activeElement;
  if (menu && active && menu.contains(active) && typeof active.blur === 'function') {
    active.blur();
  }
  const payload = readRiskSettingsFromForm();
  try {
    const resp = await fetch('/api/settings/risk', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (resp.ok) {
      const applied = await resp.json();
      applyRiskSettingsToForm(applied);
      localStorage.setItem(LS_RISK_SETTINGS_KEY, JSON.stringify(applied));
      hideRiskSettingsMenu();
    }
    if (isBacktestTabActive() && backtestLoadedCandles.length > 0) {
      await runBacktest(false, { switchTab: false, preserveView: true, patchIndicatorsOnly: true });
    }
  } catch (err) {
    console.warn('Failed to save risk settings:', err);
  }
}

function initRiskSettings() {
  const btn = document.getElementById('btn-risk-menu');
  const menu = document.getElementById('risk-settings-menu');
  if (!btn || !menu) return;

  fetchRiskSettings();

  btn.addEventListener('click', (e) => {
    e.stopPropagation();
    hideRsxSettingsMenus();
    hideWozduhSettingsMenus();
    const willOpen = menu.hidden;
    if (willOpen) openFloatingMenu(menu, btn);
    else menu.hidden = true;
  });

  menu.addEventListener('mousedown', (e) => e.stopPropagation());
  menu.addEventListener('click', (e) => e.stopPropagation());
  document.getElementById('risk-save-btn')?.addEventListener('click', () => { saveRiskSettings(); });
}

async function saveRsxSettingsFromMenu(menu, context) {
  if (!menu) return;
  const active = document.activeElement;
  if (active && menu.contains(active) && typeof active.blur === 'function') {
    active.blur();
  }
  try {
    if (context === 'backtest') {
      persistBacktestRsxFromMenu();
      await flushIndicatorSettingsAutoUpdate();
    } else {
      await syncRsxIndicatorSettings('live');
    }
  } finally {
    menu.hidden = true;
  }
}

function getWozduhSettingsMenu(wrap) {
  return wrap?.querySelector('.wozduh-settings-menu');
}

function defaultWozduhPrefs() {
  return Object.fromEntries(WOZDUH_MENU_ITEMS.map((item) => [item.prefKey, item.default]));
}

function wozduhStorageKey(context) {
  return context === 'backtest' ? WOZDUH_PREFS_BACKTEST_KEY : WOZDUH_PREFS_LIVE_KEY;
}

function applyWozduhPrefsToMenu(menu, prefs) {
  if (!menu) return;
  WOZDUH_MENU_ITEMS.forEach((item) => {
    const el = menu.querySelector(`.wozduh-chk[data-pref-key="${item.prefKey}"]`);
    if (!el) return;
    el.checked = typeof prefs[item.prefKey] === 'boolean' ? prefs[item.prefKey] : item.default;
  });
}

function readWozduhPrefsFromMenu(menu) {
  const prefs = {};
  if (!menu) return prefs;
  menu.querySelectorAll('.wozduh-chk').forEach((el) => {
    const key = el.dataset.prefKey;
    if (key) prefs[key] = el.checked;
  });
  return prefs;
}

function saveWozduhPrefs(context, prefs) {
  localStorage.setItem(wozduhStorageKey(context), JSON.stringify(prefs));
}

function loadWozduhPrefsForContext(context) {
  try {
    const raw = localStorage.getItem(wozduhStorageKey(context));
    if (raw) {
      const prefs = JSON.parse(raw);
      if (prefs && typeof prefs === 'object') return prefs;
    }
    if (context === 'live') {
      const legacy = localStorage.getItem(WOZDUH_PREFS_KEY);
      if (legacy) {
        const prefs = JSON.parse(legacy);
        if (prefs && typeof prefs === 'object') return prefs;
      }
    }
  } catch {
    /* use defaults */
  }
  return null;
}

function applyWozduhVisibilityToChart(chartData, context = 'live') {
  if (!chartData?.wozduxSeries) return;
  const menu = getWozduhSettingsMenu(getOscWrap(context));
  WOZDUH_MENU_ITEMS.forEach((item) => {
    const el = menu?.querySelector(`.wozduh-chk[data-pref-key="${item.prefKey}"]`);
    const visible = el ? el.checked : item.default;
    item.keys.forEach((key) => {
      if (chartData.wozduxSeries[key]) {
        chartData.wozduxSeries[key].applyOptions({ visible });
      }
    });
  });
}

function hideWozduhSettingsMenus() {
  document.querySelectorAll('.osc-wrap .wozduh-settings-menu').forEach((menu) => {
    menu.hidden = true;
  });
}

function initWozduhSettings() {
  document.querySelectorAll('.osc-wrap').forEach((wrap) => {
    const context = oscContextFromWrap(wrap);
    const menu = getWozduhSettingsMenu(wrap);
    if (!menu) return;
    const prefs = loadWozduhPrefsForContext(context) || defaultWozduhPrefs();
    applyWozduhPrefsToMenu(menu, prefs);
    saveWozduhPrefs(context, prefs);
    applyWozduhVisibilityToChart(context === 'backtest' ? backtestChartData : liveChartData, context);
  });

  document.querySelectorAll('.osc-wrap').forEach((wrap) => {
    const context = oscContextFromWrap(wrap);
    const toggle = wrap.querySelector('.wozduh-settings-toggle');
    const menu = getWozduhSettingsMenu(wrap);
    if (!menu) return;

    toggle?.addEventListener('click', (e) => {
      e.stopPropagation();
      hideRsxSettingsMenus();
      hideRiskSettingsMenu();
      const willOpen = menu.hidden;
      hideWozduhSettingsMenus();
      if (willOpen) openFloatingMenu(menu, toggle);
      else menu.hidden = true;
    });

    menu.addEventListener('mousedown', (e) => e.stopPropagation());
    menu.addEventListener('click', (e) => e.stopPropagation());
  });
}

function hideRsxSettingsMenus() {
  document.querySelectorAll('.rsx-wrap .indicator-settings-menu').forEach((menu) => {
    menu.hidden = true;
  });
}

function initRsxSettings() {
  loadRsxSettingsFromStorage('live');
  loadRsxSettingsFromStorage('backtest');
  applyRsxSettingsToContextMenu('live', liveRsxSettings);
  applyRsxSettingsToContextMenu('backtest', backtestRsxSettings);
  fetchRsxIndicatorSettings();

  const rsxFieldSelector = '.rsx-length-input, .rsx-div-lookback-input, .rsx-signal-length-input, .rsx-source-select, .rsx-pivot-radius-input, .rsx-div-method-select, .rsx-min-price-delta-input, .rsx-min-osc-delta-input, .rsx-show-pivots-chk';
  document.querySelectorAll('.rsx-wrap').forEach((wrap) => {
    const toggle = wrap.querySelector('.rsx-settings-toggle');
    const menu = getRsxSettingsMenu(wrap);
    if (!menu) return;

    const context = rsxContextFromWrap(wrap);
    initFloatingMenuDrag(menu);
    syncRsxPivotRadiusFieldState(context);

    toggle?.addEventListener('click', (e) => {
      e.stopPropagation();
      hideWozduhSettingsMenus();
      hideRiskSettingsMenu();
      const willOpen = menu.hidden;
      hideRsxSettingsMenus();
      if (willOpen) openFloatingMenu(menu, toggle);
      else menu.hidden = true;
    });

    menu.querySelectorAll(rsxFieldSelector).forEach((el) => {
      if (el.classList.contains('rsx-div-method-select')) {
        el.addEventListener('change', () => syncRsxPivotRadiusFieldState(context));
      }
      if (el.classList.contains('rsx-show-pivots-chk')) {
        el.addEventListener('change', () => {
          const settings = readRsxSettingsFromMenu(context);
          const applied = setRsxSettingsState(context, settings);
          persistRsxSettings(context, applied);
          const chartData = context === 'backtest' ? backtestChartData : liveChartData;
          const osc = context === 'backtest' ? backtestLoadedOsc : loadedOsc;
          const anns = context === 'backtest' ? backtestAnnotations : liveAnnotations;
          if (chartData?.rsxSeries && osc?.length) {
            applyRsxData(osc, chartData, anns);
          }
        });
        return;
      }
      if (context !== 'backtest') {
        el.addEventListener('input', () => scheduleRsxSettingsSync(context));
        el.addEventListener('change', () => scheduleRsxSettingsSync(context));
      }
    });

    menu.addEventListener('mousedown', (e) => e.stopPropagation());
    menu.addEventListener('click', (e) => e.stopPropagation());

    menu.querySelector('.rsx-save-btn')?.addEventListener('click', async () => {
      await saveRsxSettingsFromMenu(menu, context);
    });
  });

  initBacktestAutoUpdateDelegation();
  initPanelSettingsOutsideClose();
  initPanelSettingsEnterNavigation();
}

function setChartType(type) {
  chartType = type;
  liveChartData.candleSeries.applyOptions({ visible: type === 'candles' });
  liveChartData.barSeries.applyOptions({ visible: type === 'bars' });
  liveChartData.lineSeries.applyOptions({ visible: type === 'line' });
}

function applyCustomTf() {
  const input = document.getElementById('tf-custom-input');
  const val = input.value.trim();
  if (!val) return;
  input.value = '';
  switchTimeframe(val);
  setTfDropdownOpen(false);
}

function initTfBarInteraction() {
  if (window.__tfBarInteractionBound) return;

  const tfBar = document.getElementById('tf-bar');
  const tfDropdown = document.getElementById('tf-dropdown');
  const tfCurrentBtn = document.getElementById('tf-current-btn');
  if (!tfBar || !tfDropdown || !tfCurrentBtn) {
    console.warn('[initTfBarInteraction] TF bar elements missing (#tf-bar, #tf-dropdown, or #tf-current-btn)');
    return;
  }

  window.__tfBarInteractionBound = true;

  const handleTfBarClick = (e) => {
    console.log('TF Menu Clicked', e.target);

    if (e.target.closest('#tf-current-btn')) {
      e.preventDefault();
      e.stopPropagation();
      setTfDropdownOpen(tfDropdown.hidden);
      return;
    }

    const favBtn = e.target.closest('.tf-btn');
    if (favBtn?.dataset?.tf) {
      e.preventDefault();
      e.stopPropagation();
      switchTimeframe(favBtn.dataset.tf, e);
      return;
    }

    const menuBtn = e.target.closest('.tf-menu-name');
    if (menuBtn?.dataset?.tf) {
      e.preventDefault();
      e.stopPropagation();
      switchTimeframe(menuBtn.dataset.tf, e);
      setTfDropdownOpen(false);
      return;
    }

    const starBtn = e.target.closest('.tf-star');
    if (starBtn?.dataset?.tf) {
      e.preventDefault();
      e.stopPropagation();
      toggleFavorite(starBtn.dataset.tf);
    }
  };

  tfBar.addEventListener('click', handleTfBarClick, true);

  document.getElementById('tf-custom-add')?.addEventListener('click', (e) => {
    console.log('TF Menu Clicked', e.target);
    applyCustomTf();
  });

  document.getElementById('tf-custom-input')?.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') applyCustomTf();
  });

  document.addEventListener('click', (e) => {
    if (e.target.closest('#tf-bar')) return;
    setTfDropdownOpen(false);
  });
}

function renderTfBar() {
  const favEl = document.getElementById('tf-favorites');
  if (!favEl) {
    console.warn('[renderTfBar] #tf-favorites not found');
    return;
  }
  const activeTf = getActiveTf();
  favEl.innerHTML = '';
  tfFavorites.forEach((id) => {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'tf-btn' + (id === activeTf ? ' active' : '');
    btn.textContent = tfLabel(id);
    btn.dataset.tf = id;
    favEl.appendChild(btn);
  });
  const tfBtn = document.getElementById('tf-current-btn');
  const tfLabelEl = document.getElementById('timeframe-label');
  if (tfBtn) tfBtn.textContent = `${tfLabel(activeTf)} ▾`;
  if (tfLabelEl) tfLabelEl.textContent = tfLabel(activeTf);
}

function renderTfMenu() {
  const body = document.getElementById('tf-menu-body');
  if (!body) return;
  const activeTf = getActiveTf();
  body.innerHTML = '';

  Object.entries(TF_MENU).forEach(([group, items]) => {
    const label = document.createElement('div');
    label.className = 'tf-group-label';
    label.textContent = group;
    body.appendChild(label);

    items.forEach((item) => {
      const row = document.createElement('div');
      row.className = 'tf-menu-item' + (item.id === activeTf ? ' selected' : '');

      const name = document.createElement('button');
      name.type = 'button';
      name.className = 'tf-menu-name';
      name.textContent = item.label;
      name.dataset.tf = item.id;

      const star = document.createElement('button');
      star.type = 'button';
      star.className = 'tf-star' + (isFavorite(item.id) ? ' fav' : '');
      star.textContent = '★';
      star.title = 'Add to favorites';
      star.dataset.tf = item.id;

      row.appendChild(name);
      row.appendChild(star);
      body.appendChild(row);
    });
  });
}

function switchTimeframe(tf, event) {
  if (event) {
    event.preventDefault();
    event.stopPropagation();
  }
  const nextTf = resolveTf(tf);
  if (!nextTf) return;

  if (isBacktestTfContext()) {
    switchBacktestTimeframe(nextTf, event);
    return;
  }

  switchLiveTimeframe(nextTf);
}

function switchBacktestTimeframe(tf, event) {
  if (event) {
    event.preventDefault();
    event.stopPropagation();
  }
  handleBacktestIntervalChange(tf);
}

function cancelBacktestAutoUpdatePipeline() {
  if (settingsUpdateTimeout) {
    clearTimeout(settingsUpdateTimeout);
    settingsUpdateTimeout = null;
  }
  if (navigatorAutoUpdateTimer) {
    clearTimeout(navigatorAutoUpdateTimer);
    navigatorAutoUpdateTimer = null;
  }
  window.__isSettingsUpdating = false;
}

function resetBacktestClientCacheForTfChange() {
  backtestLoadedCandles = [];
  backtestLoadedOsc = [];
  backtestChartPointsByTime = new Map();
  backtestTradeMarkerTimes = new Set();
  lockedHistoricalData = null;
  backtestNavigatorChartLines = [];
  backtestNavigatorChartMarkers = [];
  backtestLastTrades = [];
  backtestHistoryHasMore = true;
  backtestHistoryLoading = false;

  clearNavigatorOverlays(backtestChartData);

  if (backtestChartData?.candleSeries) {
    backtestChartData.candleSeries.setData([]);
    backtestChartData.barSeries?.setData([]);
    backtestChartData.lineSeries?.setData([]);
    backtestChartData.candleSeries.setMarkers([]);
  }
  if (backtestChartData?.rsxSeries) {
    backtestChartData.rsxSeries.setData([]);
    backtestChartData.rsxSeries.setMarkers([]);
    backtestChartData.rsxSignalSeries?.setData([]);
  }
  if (backtestChartData?.tradeMarkerPlugin) {
    backtestChartData.tradeMarkerPlugin.setData([]);
  }
  if (backtestChartData?.wozduxSeries) {
    Object.values(backtestChartData.wozduxSeries).forEach((series) => series.setData([]));
  }
  if (backtestChartData?.volumeSeries) {
    backtestChartData.volumeSeries.setData([]);
  }
  if (backtestChartData?.entryMarkerSeries) {
    backtestChartData.entryMarkerSeries.setData([]);
  }
}

async function handleBacktestIntervalChange(newTf) {
  const tf = normalizeTf(newTf || getBacktestInterval());
  if (!tf) return;

  if (tf === normalizeTf(backtestTf) && tf === normalizeTf(getBacktestInterval()) && backtestLoadedCandles.length > 0) {
    return;
  }

  if (backtestIntervalChangeInFlight) return;

  cancelBacktestAutoUpdatePipeline();
  backtestIntervalChangeInFlight = true;

  applyTfToBacktestSelect(tf);
  syncToolbarToActiveContext();
  applyBacktestDateRangeLimits(tf);

  const anchor = backtestChartData?.chart && backtestChartData?.candleSeries
    ? window.Viewport?.captureViewport(backtestChartData.chart, backtestChartData.candleSeries) ?? null
    : null;

  resetBacktestClientCacheForTfChange();
  persistBacktestRsxFromMenu();

  if (window.currentBacktestPayload) {
    window.currentBacktestPayload.interval = tf;
  }
  buildFinalBacktestPayload({ interval: tf });

  try {
    setBacktestLoading(true);
    await runBacktest(false, {
      manageLoading: false,
      skipSettingsPush: false,
      switchTab: false,
      patchIndicatorsOnly: false,
      viewportAnchor: anchor,
    });
  } catch (err) {
    console.error('Backtest TF change failed:', err);
    alert(`🚨 Ошибка при смене ТФ: ${err.message}`);
  } finally {
    backtestIntervalChangeInFlight = false;
    setBacktestLoading(false);
    setBacktestButtonDisabled(false);
  }
}

function initBacktestIntervalHandler() {
  const el = document.getElementById('bt-interval');
  if (!el || el.dataset.autoHandlerBound === '1') return;
  el.dataset.autoHandlerBound = '1';
  el.addEventListener('change', () => {
    handleBacktestIntervalChange(el.value);
  });
}

function switchLiveTimeframe(tf) {
  const resolved = resolveTf(tf);
  if (!resolved) return;
  if (resolved === '1M') return;

  const changed = resolved !== currentTf;

  if (changed) {
    window.__pendingAnchor = null;
    abortLiveStateFetch();
    ++currentLiveRequestId;
    const chart = liveChartData?.chart;
    const series = liveChartData?.candleSeries;
    if (chart && series) {
      window.__pendingAnchor = window.Viewport?.captureViewport(chart, series) ?? null;
    }
  }

  currentTf = resolved;
  localStorage.setItem(LS_TF_KEY, resolved);
  historyHasMore = true;
  disarmLiveHistoryScroll();

  syncToolbarToActiveContext();

  if (changed) {
    clearChartData();
    chartInitialized = false;
  }

  loadDashboard({ userTfChange: changed });

  if (refreshTimer) clearInterval(refreshTimer);
  if (orderFlowPollTimer) clearInterval(orderFlowPollTimer);
  wsSubscribeTf(resolved);
  if (!isDashboardWsOpen()) {
    startLivePollTimer();
  }
  if (isOrderFlowTf(resolved)) {
    orderFlowPollTimer = setInterval(pollOrderFlowState, 500);
    applyOrderFlowTimeScale(true);
  } else {
    applyOrderFlowTimeScale(false);
  }
  updateBufferingOverlay();
}

function applyOrderFlowTimeScale(enabled) {
  if (!liveChartData.priceChart) return;
  liveChartData.priceChart.timeScale().applyOptions({
    secondsVisible: enabled,
    timeVisible: true,
  });
}

function isDashboardWsOpen() {
  return dashboardSocket?.readyState === WebSocket.OPEN;
}

function shouldRunLivePoll() {
  return !livePollSuppressedByWs && !isDashboardWsOpen();
}

function startLivePollTimer() {
  if (!shouldRunLivePoll()) return;
  if (refreshTimer) clearInterval(refreshTimer);
  if (isOrderFlowTf(currentTf)) return;
  refreshTimer = setInterval(pollLatestState, pollIntervalForTf());
}

function stopLivePollTimer() {
  if (!refreshTimer) return;
  clearInterval(refreshTimer);
  refreshTimer = null;
}

function suppressLivePollForWs() {
  livePollSuppressedByWs = true;
  stopLivePollTimer();
}

function resumeLivePollAfterWs() {
  livePollSuppressedByWs = false;
  startLivePollTimer();
}

function pollIntervalForTf(tf) {
  const id = String(tf || currentTf || '1m').toLowerCase();
  if (isOrderFlowTf(id)) return 2000;
  const unit = id.slice(-1);
  const val = parseInt(id, 10);
  if (unit === 'm') {
    if (val === 1) return 3000;
    if (val === 3) return 5000;
    return 10000;
  }
  if (unit === 'h') return 15000;
  return 30000;
}

function isOrderFlowTf(tf) {
  const id = (tf || currentTf).toLowerCase();
  if (id.includes('tick')) return true;
  return /^\d+s$/.test(id);
}

function updateBufferingOverlay() {
  const el = document.getElementById('orderflow-buffer');
  if (!el) return;
  const buffering = isOrderFlowTf() && (
    loadedCandles.length < 5 ||
    lastTickBufferLen < 500
  );
  el.style.display = buffering ? 'flex' : 'none';
}

function wsSubscribeTf(tf) {
  const send = () => {
    if (dashboardSocket && dashboardSocket.readyState === WebSocket.OPEN) {
      dashboardSocket.send(JSON.stringify({ type: 'subscribe', tf: resolveTf(tf) || currentTf }));
    }
  };
  send();
  if (dashboardSocket && dashboardSocket.readyState === WebSocket.CONNECTING) {
    dashboardSocket.addEventListener('open', send, { once: true });
  }
}

function clearChartData() {
  liveHistoryEpoch += 1;
  disarmLiveHistoryScroll();
  beginDataUpdate();
  loadedCandles = [];
  loadedOsc = [];
  liveNavigatorResult = null;
  liveNavigatorChartLines = [];
  liveNavigatorChartMarkers = [];
  sessionTrades = [];
  tradeMarkers = [];
  spikeMarkers = [];
  historyLoading = false;
  isLoadingHistory = false;
  chartInitialized = false;
  if (!liveChartData?.candleSeries) {
    endDataUpdate();
    return;
  }
  liveChartData.candleSeries.setData([]);
  liveChartData.barSeries.setData([]);
  liveChartData.lineSeries.setData([]);
  liveChartData.volumeSeries.setData([]);
  liveChartData.rsxSeries.setData([]);
  liveChartData.rsxSeries.setMarkers([]);
  liveChartData.rsxSignalSeries?.setData([]);
  WOZDUX_LINE_KEYS.forEach((key) => {
    if (liveChartData.wozduxSeries[key]) liveChartData.wozduxSeries[key].setData([]);
  });
  if (liveChartData.wozduxSeries.rsiVolSlow) liveChartData.wozduxSeries.rsiVolSlow.setMarkers([]);
  liveChartData.candleSeries.setMarkers([]);
  clearNavigatorOverlays(liveChartData);
  clearFibLines();
  resetRuler();
  updateBufferingOverlay();
  endDataUpdate();
}

function clearFibLines() {
  fibPriceLines.forEach((pl) => {
    try { liveChartData.candleSeries.removePriceLine(pl); } catch (_) { /* noop */ }
  });
  fibPriceLines = [];
}

function chartTime(raw) {
  const t = Number(raw);
  if (!Number.isFinite(t)) return null;
  // Navigator trendlines / fib zones may arrive in ms; candles & oscillators use seconds (API SSOT).
  return t >= 1e12 ? Math.floor(t / 1000) : Math.floor(t);
}

/** RSX/Wozduh warmup sentinel — 0 is never a valid live oscillator reading. */
function isWarmupOscValue(value) {
  return value == null || !Number.isFinite(value) || value === 0 || value === 50;
}

function isValidOHLC(open, high, low, close) {
  return (
    Number.isFinite(open) && Number.isFinite(high) &&
    Number.isFinite(low) && Number.isFinite(close) &&
    open > 0 && high > 0 && low > 0 && close > 0
  );
}

function apiFetchUrl(path, params = {}) {
  const qs = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value != null && value !== '') qs.set(key, String(value));
  });
  qs.set('_t', String(Date.now()));
  return `${path}?${qs.toString()}`;
}

function normalizeCandle(c) {
  if (!c) return null;
  const time = chartTime(c?.time ?? c?.Time);
  const open = Number(c.open);
  const high = Number(c.high);
  const low = Number(c.low);
  const close = Number(c.close);
  if (time == null) return null;
  if (!isValidOHLC(open, high, low, close)) return null;
  return {
    time,
    open,
    high,
    low,
    close,
    volume: Number.isFinite(Number(c.volume)) ? Number(c.volume) : 0,
  };
}

function normalizeAnnotation(a) {
  if (!a) return null;
  const time = chartTime(a.time ?? a.Time);
  if (time == null) return null;
  return {
    ...a,
    time,
    text: a.text ?? a.label ?? a.Label ?? '',
  };
}

function applyPriceBar(bar) {
  if (!bar) return;
  const last = loadedCandles[loadedCandles.length - 1];
  if (last && last.time === bar.time) {
    // Same bar: full replace (never merge OHLC fields from partial ticks).
    loadedCandles[loadedCandles.length - 1] = { ...bar };
  } else if (!last || bar.time >= last.time) {
    if (last && isLiveTickGapTooLarge(last.time, bar.time)) {
      console.warn(`Time gap too large (${bar.time} vs ${last.time}). Ignoring live tick in history mode.`);
      return;
    }
    loadedCandles.push(bar);
  } else {
    return;
  }

  if (!shouldPaintLiveChart()) return;

  if (loadedCandles.length <= 1) {
    beginDataUpdate();
    setAllPriceData(loadedCandles);
    liveChartData.volumeSeries.setData(toVolumeBars(loadedCandles));
    updateVolumeLabel(loadedCandles);
    endDataUpdate();
  } else {
    updateAllPriceSeries(bar);
  }
  updateBufferingOverlay();
}

function fmt(v) {
  return typeof v === 'number' && Number.isFinite(v) ? v.toFixed(2) : '—';
}

function fmtPrice(v) {
  return typeof v === 'number' && Number.isFinite(v)
    ? v.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })
    : '—';
}

function toCandles(raw) {
  return (raw || [])
    .map(normalizeCandle)
    .filter(Boolean);
}

function toLineClose(candles) {
  return candles.map((c) => ({ time: Number(c.time), value: Number(c.close) }));
}

function dedupeCandles(candles) {
  const map = new Map();
  (candles || []).forEach((c) => {
    const bar = normalizeCandle(c);
    if (!bar) return;
    map.set(bar.time, bar);
  });
  const data = Array.from(map.values());
  data.sort((a, b) => a.time - b.time);
  return data;
}

/** One oscillator row per candle time (same length as candles; gaps use time-only placeholders). */
function alignOscillatorsToCandles(osc, candles) {
  const bars = dedupeCandles(candles);
  if (!bars.length) return [];
  const byTime = new Map();
  (osc || []).forEach((p) => {
    const norm = normalizeOscPoint(p);
    if (norm) byTime.set(norm.time, norm);
  });
  return bars.map((c) => {
    const hit = byTime.get(c.time);
    if (hit) return hit;
    return { time: c.time };
  });
}

function stripOlderThanBars(bars, anchorTime) {
  const anchor = Number(chartTime(anchorTime));
  if (!Number.isFinite(anchor)) {
    return (bars || []).map(normalizeCandle).filter(Boolean);
  }
  return (bars || [])
    .map((item) => normalizeCandle(item))
    .filter((bar) => bar && bar.time < anchor);
}

/** Prepend history with zero overlap: incoming bars must be strictly older than existing[0]. */
function prependCandlesZeroOverlap(existing, incoming) {
  const existingBars = dedupeCandles(existing);
  if (!existingBars.length) return dedupeCandles(incoming);
  const anchor = existingBars[0].time;
  const clean = stripOlderThanBars(incoming, anchor);
  return dedupeCandles([...clean, ...existingBars]);
}

function stripOlderThanOscPoints(osc, anchorTime) {
  const anchor = Number(chartTime(anchorTime));
  if (!Number.isFinite(anchor)) return osc || [];
  return (osc || [])
    .map((p) => normalizeOscPoint(p))
    .filter((p) => p && p.time < anchor);
}

function prependOscZeroOverlap(existing, incoming, anchorTime) {
  const clean = stripOlderThanOscPoints(incoming, anchorTime);
  return mergeOsc(clean, existing);
}

function prependAnnotationsZeroOverlap(incoming, existing, anchorTime) {
  if (!existing) existing = [];
  if (!incoming) incoming = [];

  const normOld = existing.map(normalizeAnnotation).filter(Boolean);
  const normNew = incoming.map(normalizeAnnotation).filter(Boolean);

  const anchor = chartTime(anchorTime);
  const filteredNew = Number.isFinite(anchor)
    ? normNew.filter((a) => a.time < anchor)
    : normNew;

  return [...filteredNew, ...normOld].sort((a, b) => a.time - b.time);
}

function mergeCandles(existing, incoming) {
  const map = new Map();
  (existing || []).forEach((c) => {
    const bar = normalizeCandle(c);
    if (!bar) return;
    map.set(bar.time, bar);
  });
  (incoming || []).forEach((c) => {
    const bar = normalizeCandle(c);
    if (!bar) return;
    map.set(bar.time, bar);
  });
  const data = Array.from(map.values());
  data.sort((a, b) => a.time - b.time);
  return data;
}

function toLine(raw, key) {
  return (raw || []).map((p) => {
    const time = chartTime(p?.time);
    if (time == null) return { time: 0 };
    const value = Number(p[key]);
    if (isWarmupOscValue(value)) return { time: Number(time) };
    return { time: Number(time), value };
  });
}

function normalizeOscPoint(p) {
  const time = chartTime(p?.time ?? p?.Time);
  if (time == null) return null;
  return {
    time: Number(time),
    jurik: p.jurik,
    rsx: p.rsx,
    rsx_signal: p.rsx_signal ?? p.rsxSignal,
    rsiPrice: p.rsiPrice,
    emaRsi: p.emaRsi,
    rsiRsi: p.rsiRsi,
    rsiHl2: p.rsiHl2,
    rsiVolFast: p.rsiVolFast,
    rsiVolSlow: p.rsiVolSlow,
    macdRsi: p.macdRsi,
    rsiAd: p.rsiAd,
    rsiHl2Vol: p.rsiHl2Vol,
    volChanMid: p.volChanMid,
    volChanUp: p.volChanUp,
    volChanDn: p.volChanDn,
    priceChanMid: p.priceChanMid,
    priceChanUp: p.priceChanUp,
    priceChanDn: p.priceChanDn,
    volCrossMarker: p.volCrossMarker,
    color: p.color,
    marker: p.marker,
    volumeSpikeUp: p.volumeSpikeUp,
    volumeSpikeDown: p.volumeSpikeDown,
  };
}

function chartPointsToOsc(points) {
  return (points || [])
    .map((p) => normalizeOscPoint({
      time: chartTime(p.time ?? p.Time),
      jurik: p.jurik,
      rsx: p.rsx,
      rsx_signal: p.rsx_signal ?? p.rsxSignal,
      rsiPrice: p.rsiPrice,
      emaRsi: p.emaRsi,
      rsiRsi: p.rsiRsi,
      rsiHl2: p.rsiHl2,
      rsiVolFast: p.rsiVolFast ?? p.wozduh_up,
      rsiVolSlow: p.rsiVolSlow ?? p.wozduh_down,
      macdRsi: p.macdRsi,
      rsiAd: p.rsiAd,
      rsiHl2Vol: p.rsiHl2Vol,
      volChanMid: p.volChanMid,
      volChanUp: p.volChanUp,
      volChanDn: p.volChanDn,
      priceChanMid: p.priceChanMid,
      priceChanUp: p.priceChanUp,
      priceChanDn: p.priceChanDn,
      volCrossMarker: p.volCrossMarker,
      color: p.color,
      marker: p.marker,
      volumeSpikeUp: p.volumeSpikeUp,
      volumeSpikeDown: p.volumeSpikeDown,
    }))
    .filter(Boolean);
}

function chartPointsToCandles(points) {
  return dedupeCandles(
    (points || []).map((p) => ({
      time: chartTime(p.time ?? p.Time),
      open: p.open,
      high: p.high,
      low: p.low,
      close: p.close,
      volume: p.volume,
    })),
  );
}

function applyPriceToChart(chartData, candles, options = {}) {
  if (!chartData?.candleSeries) return;
  const sorted = dedupeCandles(candles);
  const lineData = toLineClose(sorted);
  lineData.sort((a, b) => a.time - b.time);
  chartData.candleSeries.setData(sorted);
  if (chartData.barSeries) chartData.barSeries.setData(sorted);
  if (chartData.lineSeries) chartData.lineSeries.setData(lineData);
  if (options.includeVolume !== false && chartData.volumeSeries) {
    chartData.volumeSeries.setData(toVolumeBars(sorted));
  }
}

function applyOscillatorToChart(chartData, osc, annotations = null) {
  applyWozduxData(osc, chartData);
  applyRsxData(osc, chartData, annotations);
  const times = (osc || []).map((p) => chartTime(p?.time)).filter((t) => t != null);
  applyStaticAreas(chartData, 'rsx', times, osc);
  applyStaticAreas(chartData, 'wozduh', times, osc);
}

function applyCandleMarkers(chartData, markers) {
  if (!chartData?.candleSeries) return;
  chartData.candleSeries.setMarkers(
    (markers || []).slice().sort((a, b) => a.time - b.time),
  );
}

function wozduxMarkersFromOsc(osc) {
  const markers = [];
  (osc || []).forEach((d) => {
    if (!d.volCrossMarker) return;
    const time = chartTime(d.time);
    if (time == null) return;
    markers.push({
      time: Number(time),
      position: 'inBar',
      color: d.volCrossMarker,
      shape: 'circle',
      size: 1,
    });
  });
  return markers.sort((a, b) => a.time - b.time);
}

function applyWozduxData(osc, chartData = liveChartData) {
  WOZDUX_LINE_KEYS.forEach((key) => {
    const series = chartData.wozduxSeries?.[key];
    if (series) series.setData(toLine(osc, key));
  });
  applyWozduxMarkers(osc, chartData);
}

function applyWozduxMarkers(osc, chartData = liveChartData) {
  const markerKey = INDICATOR_CONFIG.wozduh.markerSeriesKey;
  const markerSeries = chartData.wozduxSeries?.[markerKey];
  if (markerSeries) {
    markerSeries.setMarkers(wozduxMarkersFromOsc(osc));
  }
}

function updateWozduxPoint(pt, chartData = liveChartData) {
  if (!pt) return;
  const time = chartTime(pt?.time ?? pt?.Time);
  if (time == null) return;
  WOZDUX_LINE_KEYS.forEach((key) => {
    const value = Number(pt[key]);
    if (!Number.isFinite(value) || !chartData.wozduxSeries?.[key]) return;
    chartData.wozduxSeries[key].update({ time, value });
  });
  if (pt.volCrossMarker) {
    applyWozduxMarkers(loadedOsc, chartData);
  }
}

function fmtVolume(v) {
  if (!Number.isFinite(v) || v <= 0) return '—';
  if (v >= 1e9) return `${(v / 1e9).toFixed(2)} B`;
  if (v >= 1e6) return `${(v / 1e6).toFixed(2)} M`;
  if (v >= 1e3) return `${(v / 1e3).toFixed(2)} K`;
  return v.toFixed(2);
}

function toVolumeBars(candles) {
  return (candles || []).map((c) => ({
    time: c.time,
    value: c.volume || 0,
    color: c.close >= c.open ? CHART_STYLES.volumeBar.upColor : CHART_STYLES.volumeBar.downColor,
  }));
}

function updateVolumeLabel(candles) {
  const el = document.getElementById('volume-val');
  if (!el || !candles?.length) return;
  const last = candles[candles.length - 1];
  el.textContent = fmtVolume(last.volume);
  el.style.color = last.close >= last.open ? TV.green : TV.red;
}

function mapRSXSignalData(osc) {
  return (osc || []).map((d) => {
    const time = chartTime(d?.time);
    if (time == null) return { time: 0 };
    const raw = d.rsx_signal ?? d.rsxSignal;
    const value = parseFloat(raw);
    if (isWarmupOscValue(value)) return { time: Number(time) };
    return { time: Number(time), value };
  });
}

function mapRSXData(osc) {
  return (osc || []).map((d) => {
    const time = chartTime(d?.time);
    if (time == null) return { time: 0 };
    const value = parseFloat(d.rsx);
    if (isWarmupOscValue(value)) {
      return { time: Number(time) };
    }
    return {
      time: Number(time),
      value,
      color: d.color || RSX_DEFAULT_COLOR,
      marker: d.marker || '',
    };
  });
}

function rsxMarkerStyle(marker) {
  const m = String(marker || '').toUpperCase();
  if (m === 'S' || m === 'SS') {
    return { position: 'aboveBar', color: '#b71c1c', shape: 'circle', size: 1 };
  }
  if (m === 'L' || m === 'LL') {
    return { position: 'belowBar', color: '#004d40', shape: 'circle', size: 1 };
  }
  if (m === 'P') {
    return { position: 'belowBar', color: '#1565c0', shape: 'circle', size: 1 };
  }
  return { position: 'belowBar', color: '#2962ff', shape: 'circle', size: 1 };
}

function getChartAnnotationPanes(chartData) {
  const wozduhMarkerKey = INDICATOR_CONFIG.wozduh.markerSeriesKey;
  const panes = {
    price: chartData?.candleSeries || null,
    rsx: chartData?.rsxSeries || null,
    wozduh: chartData?.wozduxSeries?.[wozduhMarkerKey] || null,
  };
  return Object.fromEntries(
    Object.entries(panes).map(([pane, series]) => [pane.toLowerCase(), series]),
  );
}

function normalizeAnnotationPane(pane) {
  const key = String(pane || 'rsx').trim().toLowerCase();
  if (key === 'price' || key === 'wozduh') return key;
  return 'rsx';
}

function annotationToNativeMarker(ann) {
  const rawTime = ann?.time ?? ann?.Time;
  const time = chartTime(rawTime);
  if (time == null || !Number.isFinite(time)) return null;
  const label = String(ann?.label ?? ann?.Label ?? ann?.text ?? '').toUpperCase();
  const style = rsxMarkerStyle(label);
  const useRsxStyle = ['S', 'SS', 'L', 'LL', 'P'].includes(label);
  return {
    time: Number(time),
    position: useRsxStyle ? style.position : (ann?.position || 'belowBar'),
    color: useRsxStyle ? style.color : (ann?.color || '#26a69a'),
    shape: useRsxStyle ? style.shape : (ann?.shape || 'circle'),
    size: useRsxStyle ? (style.size ?? 1) : undefined,
    text: label,
    _rawTime: rawTime,
  };
}

function rsxSettingsContextForChart(chartData) {
  return chartData === backtestChartData ? 'backtest' : 'live';
}

function applyUniversalAnnotations(chartPanes, annotations, seriesTimesByPane = {}, options = {}) {
  if (!chartPanes) return;
  const showPivots = options.showPivots !== false;
  const grouped = {};
  (annotations || []).forEach((ann) => {
    const label = String(ann?.label ?? ann?.Label ?? ann?.text ?? '').toUpperCase();
    if (!showPivots && label === 'P') return;
    const pane = normalizeAnnotationPane(ann?.pane);
    const series = chartPanes[pane];
    if (!series) {
      console.warn('[Annotations] unknown pane', ann?.pane, '→', pane);
      return;
    }
    const native = annotationToNativeMarker(ann);
    if (!native) {
      console.warn('[Annotations] invalid time', pane, ann?.time ?? ann?.Time);
      return;
    }
    const { _rawTime, ...marker } = native;
    if (!grouped[pane]) grouped[pane] = [];
    grouped[pane].push(marker);
  });
  Object.entries(chartPanes).forEach(([pane, series]) => {
    if (!series) return;
    const formattedMarkers = (grouped[pane] || []).slice().sort((a, b) => a.time - b.time);
    console.log('Applying markers to', pane, formattedMarkers);
    series.setMarkers(formattedMarkers);
  });
}

function mergeAnnotations(existing, incoming) {
  const map = new Map();
  const keyOf = (ann) => {
    const pane = normalizeAnnotationPane(ann?.pane);
    const time = chartTime(ann?.time ?? ann?.Time);
    return `${pane}:${time}`;
  };
  const normalizeAnn = (ann) => {
    const time = chartTime(ann?.time ?? ann?.Time);
    if (time == null) return null;
    return {
      ...ann,
      pane: normalizeAnnotationPane(ann?.pane),
      time,
    };
  };
  (existing || []).forEach((ann) => {
    const norm = normalizeAnn(ann);
    if (norm) map.set(keyOf(ann), norm);
  });
  (incoming || []).forEach((ann) => {
    const norm = normalizeAnn(ann);
    if (norm) map.set(keyOf(ann), norm);
  });
  return [...map.values()].sort((a, b) => a.time - b.time);
}

function applyRsxData(osc, chartData = liveChartData, annotations = null) {
  if (!chartData?.rsxSeries) return;

  const mappedRSX = mapRSXData(osc);
  const lineData = mappedRSX.map(({ time, value, color }) => {
    if (isWarmupOscValue(value)) return { time };
    return { time, value, color };
  });
  console.log('[X-Ray MAPPED] First mapped point:', lineData[0]);
  console.log('[X-Ray MAPPED] Last mapped point:', lineData[lineData.length - 1]);
  chartData.rsxSeries.setData(lineData);
  if (chartData.rsxSignalSeries) {
    chartData.rsxSignalSeries.setData(mapRSXSignalData(osc));
  }

  const seriesTimesByPane = {
    rsx: new Set(lineData.map((d) => d.time)),
  };
  const resolved = annotations
    ?? (chartData === liveChartData ? liveAnnotations : backtestAnnotations);
  const showPivots = rsxShowPivotsFrom(getRsxSettingsState(rsxSettingsContextForChart(chartData)), true);
  applyUniversalAnnotations(getChartAnnotationPanes(chartData), resolved, seriesTimesByPane, { showPivots });

  if (chartData === liveChartData) {
    const last = mappedRSX.length ? mappedRSX[mappedRSX.length - 1] : null;
    const rsxEl = document.getElementById('rsx-val');
    if (rsxEl && last) {
      rsxEl.textContent = Number.isFinite(last.value) ? last.value.toFixed(1) : '—';
      rsxEl.style.color = last.color || RSX_DEFAULT_COLOR;
    }
  }
}

function updateRsxPoint(time, value, color, marker, rsxSignal, chartData = liveChartData) {
  if (time == null || !Number.isFinite(value)) return;
  const t = Number(time);
  const ptColor = color || RSX_DEFAULT_COLOR;
  chartData.rsxSeries.update({ time: t, value, color: ptColor });

  const signalVal = parseFloat(rsxSignal);
  if (chartData.rsxSignalSeries && Number.isFinite(signalVal)) {
    chartData.rsxSignalSeries.update({ time: t, value: signalVal });
  }

  if (marker || (chartData === liveChartData && liveAnnotations.length > 0)) {
    const mapped = mapRSXData(chartData === liveChartData ? loadedOsc : backtestLoadedOsc);
    const showPivots = rsxShowPivotsFrom(getRsxSettingsState(rsxSettingsContextForChart(chartData)), true);
    applyUniversalAnnotations(
      getChartAnnotationPanes(chartData),
      chartData === liveChartData ? liveAnnotations : backtestAnnotations,
      { rsx: new Set(mapped.map((d) => d.time)) },
      { showPivots },
    );
  }

  if (chartData === liveChartData) {
    const rsxEl = document.getElementById('rsx-val');
    if (rsxEl) {
      rsxEl.textContent = value.toFixed(1);
      rsxEl.style.color = ptColor;
    }
  }
}

function mergeOsc(existing, incoming) {
  const map = new Map();
  [...incoming, ...existing].forEach((p) => {
    const norm = normalizeOscPoint(p);
    if (!norm) return;
    map.set(norm.time, norm);
  });
  return Array.from(map.values()).sort((a, b) => a.time - b.time);
}

function tradeSide(trade) {
  return String(trade?.side || trade?.action || '').toUpperCase();
}

function tradeToMarker(trade) {
  const time = chartTime(trade.time);
  if (time == null) return null;
  const side = tradeSide(trade);
  const kind = String(trade.kind || 'entry').toLowerCase();

  if (side === 'CLOSE_LONG') {
    return {
      time,
      position: 'aboveBar',
      color: '#f23645',
      shape: 'circle',
      text: 'EXIT',
    };
  }
  if (side === 'CLOSE_SHORT') {
    return {
      time,
      position: 'belowBar',
      color: '#089981',
      shape: 'circle',
      text: 'EXIT',
    };
  }

  const isBuy = side === 'BUY';
  return {
    time,
    position: isBuy ? 'belowBar' : 'aboveBar',
    color: isBuy ? '#089981' : '#f23645',
    shape: isBuy ? 'arrowUp' : 'arrowDown',
    size: 2,
    text: kind === 'exit' ? 'EXIT' : (isBuy ? 'BUY' : 'SELL'),
  };
}

function mergeSessionTrades(trades, context = {}) {
  if (!Array.isArray(trades)) return;
  const masterState = context.masterState;
  const lastCandleTime = context.lastCandleTime;
  trades.forEach((trade) => {
    const time = chartTime(trade?.time ?? trade?.Time);
    const side = tradeSide(trade);
    const price = parseFloat(trade.price);
    if (time == null || !side || !Number.isFinite(price)) return;
    const kind = String(trade.kind || 'entry').toLowerCase();
    if (
      kind === 'entry'
      && masterState
      && masterState !== 'IN_POSITION'
      && lastCandleTime != null
      && time === lastCandleTime
    ) {
      return;
    }
    const key = `${time}:${side}:${price}:${kind}`;
    if (sessionTrades.some((t) => `${chartTime(t.time)}:${tradeSide(t)}:${t.price}:${String(t.kind || 'entry').toLowerCase()}` === key)) return;
    sessionTrades.push({ time, side, price, kind });
  });
}

function applyTradeMarkers() {
  tradeMarkers = sessionTrades
    .map(tradeToMarker)
    .filter(Boolean)
    .sort((a, b) => a.time - b.time);
  applyAllMarkers();
}

function buildSpikeMarkers(osc) {
  const markers = [];
  (osc || []).forEach((p) => {
    const time = chartTime(p?.time ?? p?.Time);
    if (time == null) return;
    if (p.volumeSpikeUp) markers.push({ time, position: 'belowBar', color: TV.green, shape: 'circle', text: '▲' });
    if (p.volumeSpikeDown) markers.push({ time, position: 'aboveBar', color: TV.red, shape: 'circle', text: '▼' });
  });
  return markers.sort((a, b) => a.time - b.time);
}

function applyAllMarkers() {
  const showSpike = document.getElementById('tog-spike').checked;
  const combined = [...tradeMarkers];
  if (showSpike) combined.push(...spikeMarkers);
  combined.sort((a, b) => a.time - b.time);
  liveChartData.candleSeries.setMarkers(combined);
}

function renderFibZones(zones) {
  lastFibZones = zones || [];
  clearFibLines();
  if (!document.getElementById('tog-fib').checked) return;
  lastFibZones.forEach((z) => {
    if (!z.isActive || !Number.isFinite(z.price)) return;
    const color = z.ratio === 0.618 ? '#e3b341' : 'rgba(120,123,134,0.7)';
    fibPriceLines.push(liveChartData.candleSeries.createPriceLine(normalizePriceLineLevel({
      price: z.price,
      color,
      lineWidth: z.ratio === 0.618 ? 2 : 1,
      lineStyle: LightweightCharts.LineStyle.Dashed,
      axisLabelVisible: true,
      title: `${(z.ratio * 100).toFixed(1)}%`,
    })));
  });
}

function deriveBotStatus(data) {
  if (data.masterState) return data.masterState;
  const trades = data.trades || sessionTrades || [];
  if (trades.length === 0) return 'IDLE';
  const sorted = [...trades].sort((a, b) => a.time - b.time);
  const last = sorted[sorted.length - 1];
  return last.kind === 'exit' ? 'IDLE' : 'IN_POSITION';
}

function getScoreThreshold(side) {
  const context = getActiveStrategyContext();
  const thresholds = normalizeStrategyThresholds(getStrategyState(context)?.thresholds);
  const val = side === 'long' ? thresholds.long : thresholds.short;
  return Number.isFinite(val) && val > 0 ? val : DEFAULT_STRATEGY_THRESHOLDS.long;
}

function resetScoringTelemetryPlaceholder() {
  lastTelemetrySignature = '';
  const line = document.getElementById('telemetry-compact-line');
  if (line) line.innerHTML = '<span class="telemetry-idle">—</span>';
}

function buildTelemetrySignature(data) {
  if (!data) return '';
  const factorKeys = data.factors && typeof data.factors === 'object'
    ? Object.keys(data.factors).sort().join(',')
    : '';
  return [
    data.longScore,
    data.shortScore,
    data.rawAction,
    data.finalAction,
    data.isVetoed,
    data.vetoReason,
    factorKeys,
  ].join('|');
}

function topTelemetryFactors(factors, limit = 4) {
  if (!factors || typeof factors !== 'object') return [];
  return Object.entries(factors)
    .map(([key, factor]) => ({
      key,
      name: factor?.name || key,
      score: Number(factor?.score) || 0,
      direction: (factor?.direction || 'WAIT').toUpperCase(),
    }))
    .sort((a, b) => Math.abs(b.score) - Math.abs(a.score))
    .slice(0, limit);
}

function buildCompactTelemetryHTML(data, options = {}) {
  if (!data) {
    return '<span class="telemetry-idle">—</span>';
  }

  const long = Number(data.longScore) || 0;
  const short = Number(data.shortScore) || 0;
  const raw = (data.rawAction || 'WAIT').toUpperCase();
  const final = (data.finalAction || 'WAIT').toUpperCase();
  const action = data.isVetoed ? `${raw}→${final}` : final;
  const actionClass = data.isVetoed ? 'telemetry-vetoed' : (
    final === 'BUY' ? 'telemetry-buy' : final === 'SELL' ? 'telemetry-sell' : 'telemetry-wait'
  );

  const factorChips = topTelemetryFactors(data.factors).map((f) => {
    const pts = f.score > 0 ? `+${f.score}` : f.score < 0 ? String(f.score) : '·';
    return `<span class="telemetry-chip" title="${f.name}">${f.name} ${pts}</span>`;
  }).join('');

  const veto = data.isVetoed
    ? `<span class="telemetry-veto">⛔ ${data.vetoReason || 'Veto'}</span>`
    : '';

  if (!factorChips && action === 'WAIT' && long === 0 && short === 0) {
    return options.locked
      ? '<span class="telemetry-lock-badge">🔍 History</span><span class="telemetry-sep">·</span><span class="telemetry-idle">No signal data</span>'
      : '<span class="telemetry-idle">WAIT — no signal</span>';
  }

  const lockPrefix = options.locked
    ? `<span class="telemetry-lock-badge">🔍 History${data.inspectLabel ? ` · ${data.inspectLabel}` : ''}</span><span class="telemetry-sep">·</span>`
    : '';

  return `
    ${lockPrefix}
    <span class="telemetry-score">L <strong>${long}</strong></span>
    <span class="telemetry-sep">·</span>
    <span class="telemetry-score">S <strong>${short}</strong></span>
    <span class="telemetry-sep">·</span>
    <span class="telemetry-action ${actionClass}">${action}</span>
    ${factorChips ? `<span class="telemetry-sep">·</span><span class="telemetry-factors">${factorChips}</span>` : ''}
    ${veto}
  `;
}

function renderCompactTelemetry(data, options = {}) {
  if (lockedHistoricalData && !options.force) return;

  const line = document.getElementById('telemetry-compact-line');
  const card = document.getElementById('scoring-telemetry-card');
  if (!line) return;

  const signature = buildTelemetrySignature(data);
  if (signature === lastTelemetrySignature) return;
  lastTelemetrySignature = signature;

  card?.classList.remove('telemetry-locked');
  line.innerHTML = buildCompactTelemetryHTML(data);
}

function chartPointToScoringData(point) {
  if (!point) return null;
  return {
    longScore: point.longScore,
    shortScore: point.shortScore,
    rawAction: point.rawAction,
    finalAction: point.finalAction,
    isVetoed: point.isVetoed,
    vetoReason: point.vetoReason,
    factors: point.factors,
    inspectTime: chartTime(point.time),
    inspectLabel: null,
  };
}

function isInspectableChartPoint(point) {
  if (!point) return false;
  const hasFactors = point.factors && typeof point.factors === 'object' && Object.keys(point.factors).length > 0;
  const hasScores = (Number(point.longScore) || 0) > 0 || (Number(point.shortScore) || 0) > 0;
  const raw = (point.rawAction || 'WAIT').toUpperCase();
  const final = (point.finalAction || 'WAIT').toUpperCase();
  const hasAction = raw !== 'WAIT' || final !== 'WAIT';
  return hasFactors || hasScores || hasAction || !!point.isVetoed;
}

function rebuildBacktestTradeMarkerTimes(trades) {
  backtestTradeMarkerTimes = new Set();
  (trades || []).forEach((trade) => {
    const entryT = chartTime(trade.entryTime || trade.EntryTime);
    const exitT = chartTime(trade.time || trade.Time);
    if (entryT != null) backtestTradeMarkerTimes.add(entryT);
    if (exitT != null) backtestTradeMarkerTimes.add(exitT);
  });
}

function findBacktestTradeAtTime(t) {
  return (backtestLastTrades || []).find((trade) => {
    const entryT = chartTime(trade.entryTime || trade.EntryTime);
    const exitT = chartTime(trade.time || trade.Time);
    return t === entryT || t === exitT;
  }) || null;
}

function formatTradeInspectLabel(trade, t) {
  const entryT = chartTime(trade.entryTime || trade.EntryTime);
  const side = String(trade.side || trade.Side || '—').toUpperCase();
  if (t === entryT) return `Entry ${side}`;
  return `Exit ${side}`;
}

function resolveInspectablePointAtTime(t) {
  if (t == null) return null;

  const point = backtestChartPointsByTime.get(t);
  const onTradeMarker = backtestTradeMarkerTimes.has(t);

  if (isInspectableChartPoint(point)) {
    return { point, tradeHint: onTradeMarker };
  }
  if (onTradeMarker && point) {
    return { point, tradeHint: true };
  }
  return null;
}

function renderLockedTelemetry() {
  const line = document.getElementById('telemetry-compact-line');
  const card = document.getElementById('scoring-telemetry-card');
  if (!line || !lockedHistoricalData) return;

  lastTelemetrySignature = `locked|${lockedHistoricalData.inspectTime}|${buildTelemetrySignature(lockedHistoricalData)}`;
  card?.classList.add('telemetry-locked');
  line.innerHTML = buildCompactTelemetryHTML(lockedHistoricalData, { locked: true });
}

function getDefaultBacktestTelemetryData() {
  const chartData = lastBacktestResult?.chartData;
  if (!Array.isArray(chartData) || chartData.length === 0) return null;
  return chartPointToScoringData(chartData[chartData.length - 1]);
}

function refreshTelemetryAfterUnlock() {
  if (isBacktestTabActive()) {
    const endData = getDefaultBacktestTelemetryData();
    if (endData) {
      updateScoringUI(endData, { source: 'backtest', force: true });
      return;
    }
  }
  if (liveScoringData) {
    updateScoringUI(liveScoringData, { force: true });
    return;
  }
  resetScoringTelemetryPlaceholder();
}

function clearHistoricalInspection(refresh = true) {
  if (!lockedHistoricalData) return;
  lockedHistoricalData = null;
  document.getElementById('scoring-telemetry-card')?.classList.remove('telemetry-locked');
  lastTelemetrySignature = '';
  if (refresh) refreshTelemetryAfterUnlock();
}

function lockHistoricalInspection(data) {
  if (!data) return;
  lockedHistoricalData = data;
  renderLockedTelemetry();
}

function handleBacktestInspectorClick(param) {
  if (!isBacktestTabActive() || isUpdatingData) return;
  if (ruler.active) return;

  if (param.time == null) {
    clearHistoricalInspection();
    return;
  }

  const t = chartTime(param.time);
  const resolved = resolveInspectablePointAtTime(t);
  if (!resolved) {
    clearHistoricalInspection();
    return;
  }

  if (lockedHistoricalData?.inspectTime === t) {
    clearHistoricalInspection();
    return;
  }

  const data = chartPointToScoringData(resolved.point);
  if (!data) {
    clearHistoricalInspection();
    return;
  }

  if (resolved.tradeHint) {
    const trade = findBacktestTradeAtTime(t);
    if (trade) data.inspectLabel = formatTradeInspectLabel(trade, t);
  }

  lockHistoricalInspection(data);
}

function ensureCrosshairScoringHint(chartData) {
  const wrap = chartData?.elements?.priceWrap;
  if (!wrap || chartData.crosshairScoringHint) return;

  const hint = document.createElement('div');
  hint.className = 'crosshair-scoring-hint';
  hint.hidden = true;
  wrap.appendChild(hint);
  chartData.crosshairScoringHint = hint;
}

function updateCrosshairScoringHint(chartData, param, point) {
  ensureCrosshairScoringHint(chartData);
  const hint = chartData?.crosshairScoringHint;
  if (!hint) return;

  if (!point || !param?.point || param.time == null) {
    hint.hidden = true;
    return;
  }

  const long = Number(point.longScore) || 0;
  const short = Number(point.shortScore) || 0;
  const final = (point.finalAction || 'WAIT').toUpperCase();
  const veto = point.isVetoed ? ` · ⛔ ${point.vetoReason || 'Veto'}` : '';
  hint.textContent = `L ${long} · S ${short} · ${final}${veto}`;
  hint.hidden = false;

  const wrap = chartData.elements?.priceWrap;
  if (!wrap) return;
  const x = Math.min(Math.max(8, param.point.x + 12), wrap.clientWidth - 180);
  const y = Math.max(8, param.point.y - 28);
  hint.style.left = `${x}px`;
  hint.style.top = `${y}px`;
}

function attachBacktestScoringCrosshair(chartData) {
  if (!chartData?.priceChart || chartData._scoringCrosshairBound) return;
  chartData._scoringCrosshairBound = true;
  ensureCrosshairScoringHint(chartData);

  chartData.priceChart.subscribeCrosshairMove((param) => {
    if (!isBacktestTabActive() || isUpdatingData) return;

    if (param.time == null) {
      updateCrosshairScoringHint(chartData, param, null);
      return;
    }

    const t = chartTime(param.time);
    if (t == null) {
      updateCrosshairScoringHint(chartData, param, null);
      return;
    }
    const point = backtestChartPointsByTime.get(t);
    // updateCrosshairScoringHint(chartData, param, point);
  });

  chartData.priceChart.subscribeClick((param) => {
    if (chartData !== backtestChartData) return;
    if (chartData._inspectorClickTimer) {
      clearTimeout(chartData._inspectorClickTimer);
    }
    chartData._inspectorClickTimer = setTimeout(() => {
      chartData._inspectorClickTimer = null;
      handleBacktestInspectorClick(param);
    }, INSPECTOR_CLICK_DELAY_MS);
  });
}

function updateScoringUI(data, options = {}) {
  if (!data || options.source === 'crosshair') return;
  if (lockedHistoricalData && !options.force) return;

  lastScoringData = data;
  if (options.source !== 'backtest') {
    liveScoringData = data;
  }
  // renderCompactTelemetry(data, options);
  const long = Number(data.longScore) || 0;
  const short = Number(data.shortScore) || 0;
  const longThreshold = getScoreThreshold('long');
  const shortThreshold = getScoreThreshold('short');

  const longEl = document.getElementById('hdr-score-long');
  const shortEl = document.getElementById('hdr-score-short');
  if (longEl) {
    const nextLong = String(long);
    if (longEl.textContent !== nextLong) longEl.textContent = nextLong;
    const longColor = long >= longThreshold ? '#00e676' : long > 50 ? TV.green : TV.text;
    if (longEl.style.color !== longColor) longEl.style.color = longColor;
  }
  if (shortEl) {
    const nextShort = String(short);
    if (shortEl.textContent !== nextShort) shortEl.textContent = nextShort;
    const shortColor = short >= shortThreshold ? '#ff1744' : short > 50 ? TV.red : TV.text;
    if (shortEl.style.color !== shortColor) shortEl.style.color = shortColor;
  }

  const barLong = document.getElementById('bar-long');
  const barShort = document.getElementById('bar-short');
  const longScale = Math.min(1, long / longThreshold);
  const shortScale = Math.min(1, short / shortThreshold);
  const longTransform = `scaleX(${longScale})`;
  const shortTransform = `scaleX(${shortScale})`;
  if (barLong && barLong.style.transform !== longTransform) {
    barLong.style.transform = longTransform;
  }
  if (barShort && barShort.style.transform !== shortTransform) {
    barShort.style.transform = shortTransform;
  }

  const botEl = document.getElementById('bot-status');
  if (botEl) {
    const status = deriveBotStatus(data);
    if (botEl.textContent !== status) botEl.textContent = status;
    const statusClass = `status-badge ${status === 'IN_POSITION' ? 'in-position' : 'idle'}`;
    if (botEl.className !== statusClass) botEl.className = statusClass;
  }
}

function switchTab(targetId) {
  const prevTab = document.querySelector('.tab-content.active')?.id || 'tab-live';
  saveThresholdsFromHeaderToState(getActiveStrategyContext());
  if (prevTab !== targetId) {
    clearHistoricalInspection(false);
  }
  const tabs = document.querySelectorAll('.tabs-nav .tab-btn');
  const panels = document.querySelectorAll('.tab-content');
  tabs.forEach((b) => b.classList.toggle('active', b.dataset.tab === targetId));
  panels.forEach((panel) => {
    const isActive = panel.id === targetId;
    panel.classList.toggle('active', isActive);
    panel.style.display = isActive ? '' : 'none';
  });

  const backtestControls = document.getElementById('backtest-controls');
  if (backtestControls) {
    backtestControls.classList.toggle('visible', targetId === 'tab-backtest');
  }

  const telemetryWrap = document.getElementById('telemetry-bar-wrap');
  if (telemetryWrap) {
    syncFactsBarVisibility();
  }

  if (targetId === 'tab-live' && prevTab !== 'tab-live') {
    if (liveScoringData) {
      updateScoringUI(liveScoringData);
    } else {
      resetScoringTelemetryPlaceholder();
    }
  }

  if (ruler.active) {
    ruler.active = false;
    document.getElementById('ruler-btn')?.classList.remove('active');
    setRulerCursor(false);
  }
  resetRuler();
  syncToolbarToActiveContext();
  applyWozduhVisibilityToChart(getActiveChartData(), getActiveUiContext());

  const nextStrategyContext = (targetId === 'tab-backtest' || targetId === 'tab-stats') ? 'backtest' : 'live';
  applyThresholdsToHeader(getStrategyState(nextStrategyContext).thresholds);
  if (lastScoringData && (targetId === 'tab-live' || targetId === 'tab-backtest')) {
    updateScoringUI(lastScoringData, {
      source: targetId === 'tab-backtest' ? 'backtest' : undefined,
      force: targetId === 'tab-backtest',
    });
  }

  if (targetId === 'tab-live') {
    pushRsxSettingsToServer(coerceRsxSettingsForAPI(liveRsxSettings))
      .then(() => {
        if (chartInitialized && isLiveTabActive()) {
          reloadRsxChartFromServer();
        }
      })
      .catch((err) => console.warn('Failed to restore live RSX settings:', err));
  }

  requestAnimationFrame(() => {
    handleResize();

    if (targetId === 'tab-live' && liveChartData.chart) {
      wsSubscribeTf(currentTf);
      if (!chartInitialized) {
        loadDashboard();
      } else {
        beginDataUpdate();
        try {
          applySeriesData();
          syncLiveChartPanesFromPrice();
        } finally {
          endDataUpdate(0);
        }
        if (shouldRunLivePoll()) {
          pollLatestState();
        }
      }
    } else if (targetId === 'tab-backtest' && backtestChartData.chart) {
      forceSyncChartTimeScales(backtestChartData);
      if (!lockedHistoricalData) {
        const endData = getDefaultBacktestTelemetryData();
        if (endData) updateScoringUI(endData, { source: 'backtest', force: true });
      }
    } else if (targetId === 'tab-stats') {
      resizeEquityChart();
      equityChart?.timeScale().fitContent();
      refreshStatsForMode(statsMode);
    }
  });
}

function initTabs() {
  const tabs = document.querySelectorAll('.tabs-nav .tab-btn');
  if (!tabs.length) {
    console.warn('[initTabs] No tab buttons found (.tabs-nav .tab-btn)');
    return;
  }
  tabs.forEach((btn) => {
    if (!btn.dataset.tab) {
      console.warn('[initTabs] Tab button missing data-tab attribute', btn);
      return;
    }
    btn.addEventListener('click', () => switchTab(btn.dataset.tab));
  });
  try {
    switchTab('tab-live');
  } catch (err) {
    console.error('[initTabs] switchTab(tab-live) failed:', err);
  }
}

function loadLegacyThresholdsFromStorage() {
  try {
    const raw = localStorage.getItem(THRESHOLDS_KEY);
    if (!raw) return null;
    const saved = JSON.parse(raw);
    return normalizeStrategyThresholds(saved);
  } catch {
    return null;
  }
}

function loadThresholdsFromStorage() {
  applyThresholdsToHeader(liveStrategyState?.thresholds || DEFAULT_STRATEGY_THRESHOLDS);
}

async function postThresholdsToServer(thresholds) {
  const t = normalizeStrategyThresholds(thresholds);
  try {
    await fetch('/api/settings/thresholds', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ long: t.long, short: t.short }),
    });
  } catch (err) {
    console.warn('Failed to sync thresholds:', err);
  }
}

async function syncThresholds() {
  const context = getActiveStrategyContext();
  const thresholds = readThresholdsFromHeader();
  const state = getStrategyState(context);
  if (!state) return;
  state.thresholds = thresholds;
  persistStrategyState(context);

  if (context === 'live') {
    await postThresholdsToServer(thresholds);
  }

  if (lastScoringData) updateScoringUI(lastScoringData);
}

function initThresholdInputs() {
  loadThresholdsFromStorage();
  if (isLiveTabActive()) {
    postThresholdsToServer(liveStrategyState.thresholds);
  }

  ['ui-threshold-long', 'ui-threshold-short'].forEach((id) => {
    const el = document.getElementById(id);
    if (!el) return;
    el.addEventListener('change', () => syncThresholds());
  });
}

function collectMatrixFromUI() {
  const matrix = { ...SCORING_MATRIX_DEFAULTS };
  SCORING_MATRIX_LABELS.forEach(({ key }) => {
    const checked = readMatrixCheckbox(key);
    if (checked !== null) {
      matrix[key] = checked;
    }
  });
  return matrix;
}

function applyMatrixToUI(matrix) {
  const merged = { ...SCORING_MATRIX_DEFAULTS, ...matrix };
  SCORING_MATRIX_LABELS.forEach(({ key }) => {
    const el = document.getElementById(`matrix-${key}`)
      || document.querySelector(`input[type="checkbox"][data-matrix-key="${key}"]`);
    if (el) el.checked = !!merged[key];
  });
}

function loadLegacyScoringMatrixFromStorage() {
  try {
    const raw = localStorage.getItem(SCORING_MATRIX_KEY);
    if (!raw) return null;
    return normalizeStrategyMatrix(JSON.parse(raw));
  } catch {
    return null;
  }
}

function loadScoringMatrixFromStorage() {
  return getMatrixFromStrategyState('live');
}

async function postMatrixToServer(matrix) {
  const payload = normalizeStrategyMatrix(matrix);
  try {
    await fetch('/api/settings/matrix', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
  } catch (err) {
    console.warn('Failed to sync scoring matrix:', err);
  }
}

async function saveMatrixForContext(context = matrixModalContext) {
  const matrix = collectMatrixFromUI();
  const state = getStrategyState(context);
  if (!state) return;
  state.matrix = normalizeStrategyMatrix(matrix);
  persistStrategyState(context);

  if (context === 'live') {
    await postMatrixToServer(state.matrix);
  } else if (backtestLoadedCandles.length > 0) {
    setBacktestLoading(true);
    scheduleIndicatorSettingsAutoUpdate();
  }
}

function openMatrixModal() {
  matrixModalContext = getActiveStrategyContext();
  applyMatrixToUI(getStrategyState(matrixModalContext).matrix);
  const title = document.getElementById('matrix-modal-title');
  if (title) {
    title.textContent = matrixModalContext === 'backtest'
      ? 'Signal Matrix (Backtest)'
      : 'Signal Matrix (Live)';
  }
  const modal = document.getElementById('matrix-modal');
  if (!modal) return;
  modal.classList.add('open');
  modal.setAttribute('aria-hidden', 'false');
}

function closeMatrixModal() {
  const modal = document.getElementById('matrix-modal');
  if (!modal) return;
  modal.classList.remove('open');
  modal.setAttribute('aria-hidden', 'true');
}

function initScoringMatrix() {
  const body = document.getElementById('matrix-modal-body');
  if (!body) return;

  if (!body.querySelector('[data-matrix-key]')) {
    body.innerHTML = SCORING_MATRIX_LABELS.map(({ key, label }) => `
      <label class="matrix-toggle">
        <input type="checkbox" id="matrix-${key}" data-matrix-key="${key}" checked />
        ${label}
      </label>
    `).join('');
  }

  applyMatrixToUI(liveStrategyState?.matrix || SCORING_MATRIX_DEFAULTS);

  document.getElementById('matrix-open-btn')?.addEventListener('click', openMatrixModal);
  document.getElementById('matrix-modal-close')?.addEventListener('click', closeMatrixModal);
  document.getElementById('matrix-modal-backdrop')?.addEventListener('click', closeMatrixModal);
  document.getElementById('matrix-save-btn')?.addEventListener('click', async () => {
    try {
      await saveMatrixForContext(matrixModalContext);
      closeMatrixModal();
    } catch (err) {
      console.warn('Failed to save scoring matrix:', err);
    }
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') closeMatrixModal();
  });
}

function estimateCandleIntervalSec(candles) {
  if (!candles || candles.length < 2) return 60;
  const t0 = chartTime(candles[0].time);
  const t1 = chartTime(candles[1].time);
  if (t0 == null || t1 == null) return 60;
  return Math.max(1, t1 - t0);
}

function buildBacktestTradeMarkers(trades) {
  return buildTradeMarkerPrimitiveData(trades);
}

function buildTradeMarkerPrimitiveData(trades) {
  const markers = [];
  (trades || []).forEach((trade) => {
    const entryT = chartTime(trade.entryTime || trade.EntryTime);
    const exitT = chartTime(trade.time || trade.Time);
    const entryP = parseFloat(trade.entryPrice ?? trade.EntryPrice);
    const exitP = parseFloat(trade.exitPrice ?? trade.ExitPrice);
    const side = String(trade.side || trade.Side || '').toUpperCase();
    const isLong = side === 'LONG' || side === 'BUY';

    if (entryT && Number.isFinite(entryP)) {
      markers.push({ time: entryT, price: entryP, kind: isLong ? 'long' : 'short' });
    }
    if (exitT && Number.isFinite(exitP)) {
      markers.push({ time: exitT, price: exitP, kind: 'exit' });
    }
  });
  return markers.sort((a, b) => a.time - b.time);
}

function filterNavigatorLinesByTerm(lines, pane) {
  const settings = getNavigatorPaneSettingsFromUI(pane);
  const allowed = [];
  if (settings.useLong !== false) allowed.push('solid');
  if (settings.useMedium !== false) allowed.push('dashed');
  if (settings.useShort !== false) allowed.push('dotted');
  if (allowed.length === 0) return lines || [];
  return (lines || []).filter((line) => {
    const style = String(line.style || 'solid').toLowerCase();
    return allowed.includes(style);
  });
}

function filterNavigatorMarkersByHHLL(markers, pane) {
  const mode = loadNavigatorPaneSettings(pane, getNavigatorContext()).hhll || 'None';
  if (mode === 'None') return [];
  if (mode === 'Only') {
    return (markers || []).filter((m) => {
      const type = String(m.type || m.text || '').trim();
      return type === 'HH' || type === 'LL';
    });
  }
  return markers || [];
}

function mapNavigatorBackgroundZones(zones) {
  return (zones || []).map((zone) => {
    const time1 = chartTime(zone.startTime);
    const time2 = chartTime(zone.endTime);
    if (time1 == null || time2 == null) return null;
    return {
      startTime: Math.floor(time1),
      endTime: Math.floor(time2),
      time1: Math.floor(time1),
      time2: Math.floor(time2),
      color: zone.color,
    };
  }).filter(Boolean);
}

function navigatorBarColorMap(barColors, candles) {
  const colorByTime = new Map();
  if (!barColors) return colorByTime;

  if (Array.isArray(barColors)) {
    barColors.forEach((entry) => {
      if (!entry?.color) return;
      const t = chartTime(entry.time)
        ?? navigatorBarIndexToTime(entry.index, candles);
      if (t != null) colorByTime.set(t, entry.color);
    });
    return colorByTime;
  }

  if (typeof barColors === 'object') {
    Object.entries(barColors).forEach(([rawTime, color]) => {
      const t = chartTime(rawTime);
      if (t != null && color) colorByTime.set(t, color);
    });
  }
  return colorByTime;
}

function applyNavigatorBarColors(chartData, barColors, candles, enabled) {
  const colorByTime = navigatorBarColorMap(barColors, candles);
  if (!enabled || !chartData?.candleSeries || !candles?.length || colorByTime.size === 0) {
    return candles;
  }

  const colored = candles.map((candle) => {
    const color = colorByTime.get(candle.time);
    if (!color) return { ...candle };
    return {
      ...candle,
      color,
      borderColor: color,
      wickColor: color,
    };
  });

  const applyColoredData = () => {
    chartData.candleSeries.setData(colored);
    if (chartData.barSeries) chartData.barSeries.setData(colored);
  };
  if (chartData?.priceChart) {
    window.Viewport?.lockPriceAutoScaleDuring?.(chartData.priceChart, applyColoredData);
  } else {
    applyColoredData();
  }

  return colored;
}

function navigatorBarIndexToTime(index, candles) {
  const idx = Number(index);
  if (!Number.isInteger(idx) || idx < 0 || !candles || idx >= candles.length) return null;
  return chartTime(candles[idx].time);
}

function mapNavigatorLinesForChart(lines, candles) {
  const minTime = candles?.length ? chartTime(candles[0].time) : null;
  return (lines || []).map((line) => {
    let time1 = chartTime(line.time1);
    let time2 = chartTime(line.time2);
    if (time1 == null) time1 = navigatorBarIndexToTime(line.x1, candles);
    if (time2 == null) time2 = navigatorBarIndexToTime(line.x2, candles);
    if (time1 == null || time2 == null) return null;
    if (minTime != null) {
      if (time2 < minTime) return null;
      if (time1 < minTime) {
        const y1 = Number(line.y1);
        const y2 = Number(line.y2);
        if (Number.isFinite(y1) && Number.isFinite(y2) && time2 !== time1) {
          line = {
            ...line,
            y1: y1 + (y2 - y1) * (minTime - time1) / (time2 - time1),
          };
        }
        time1 = minTime;
      }
    }
    return {
      time1: Math.floor(time1),
      time2: Math.floor(time2),
      x1: Math.floor(time1),
      y1: line.y1,
      x2: Math.floor(time2),
      y2: line.y2,
      barX1: line.x1,
      barX2: line.x2,
      slope: Number.isFinite(line.slope) ? line.slope : barSlopeFromLine(line),
      isActive: line.isActive === true,
      color: line.color,
      style: line.style,
    };
  }).filter(Boolean);
}

function barSlopeFromLine(line) {
  const y1 = Number(line.y1);
  const y2 = Number(line.y2);
  const x1 = Number(line.x1);
  const x2 = Number(line.x2);
  if (!Number.isFinite(y1) || !Number.isFinite(y2) || !Number.isFinite(x1) || !Number.isFinite(x2)) {
    return 0;
  }
  const dx = x2 - x1;
  if (dx === 0) return 0;
  return (y2 - y1) / dx;
}

function buildNavigatorMarkers(navigatorData, candles, pane = 'price') {
  const filtered = filterNavigatorMarkersByHHLL(navigatorData?.markers, pane);
  const markers = [];
  filtered.forEach((m) => {
    const time = chartTime(m.time) ?? navigatorBarIndexToTime(m.index, candles);
    if (time == null) return;
    const type = String(m.type || m.text || '').trim();
    if (type === 'HH') {
      markers.push({
        time,
        position: 'aboveBar',
        shape: 'circle',
        text: 'HH',
        color: m.color || '#089981',
      });
    } else if (type === 'LL') {
      markers.push({
        time,
        position: 'belowBar',
        shape: 'circle',
        text: 'LL',
        color: m.color || '#f23645',
      });
    } else if (type === 'WickBreak') {
      markers.push({
        time,
        position: 'inBar',
        shape: 'circle',
        color: m.color || '#ff5d00',
        size: 1,
      });
    }
  });
  return markers.sort((a, b) => a.time - b.time);
}

function navigatorHasBarColors(barColors) {
  if (Array.isArray(barColors)) return barColors.length > 0;
  if (barColors && typeof barColors === 'object') return Object.keys(barColors).length > 0;
  return false;
}

function navigatorResultHasContent(dto) {
  if (!dto) return false;
  return (dto.lines?.length > 0)
    || (dto.markers?.length > 0)
    || navigatorHasBarColors(dto.barColors)
    || (dto.backgroundZones?.length > 0);
}

function resolveNavigatorResults(result) {
  if (!result) return {};

  if (result.navigators && typeof result.navigators === 'object') {
    const keys = Object.keys(result.navigators);
    if (keys.length > 0) {
      return result.navigators;
    }
  }

  const legacy = result.navigatorData || result.navigatorPrice;
  if (navigatorResultHasContent(legacy)) {
    return { price: legacy };
  }

  return {};
}

function reapplyCachedNavigatorPlugins(chartData = backtestChartData) {
  if (!chartData || !backtestLoadedCandles.length) return;

  const pricePlugin = chartData.priceNavigatorPlugin;
  if (pricePlugin && backtestNavigatorChartLines?.length) {
    pricePlugin.setData(backtestNavigatorChartLines);
    pricePlugin.setLinesVisible(true);
  }

  const navMarkers = backtestNavigatorChartMarkers || [];
  if (navMarkers.length) {
    applyBacktestCandleMarkers(chartData, backtestLastTrades, backtestLoadedOsc, navMarkers);
  }
}

function clearNavigatorOverlays(chartData = backtestChartData) {
  ['price', 'rsx', 'wozduh'].forEach((pane) => {
    syncMtfNavigatorLayerRegistry(chartData, pane, [], '');
    const plugin = getNavigatorPluginForPane(chartData, pane);
    plugin?.setData([]);
    plugin?.setBackgroundZones([]);
  });
  backtestNavigatorChartLines = [];
  backtestNavigatorChartMarkers = [];
}

function applyNavigatorPaneOverlay(pane, navigatorData, candles, chartData, options = {}) {
  if (!chartData) return [];

  const context = options.context || (chartData === liveChartData ? 'live' : 'backtest');
  const chartTf = resolveChartTfForNavigator(context);
  const paneEnabled = isNavigatorPaneEnabled(pane, context);
  const settings = getNavigatorPaneSettingsFromUI(pane, context);
  const hasLines = navigatorData?.lines?.length > 0;
  const hasZones = navigatorData?.backgroundZones?.length > 0;
  const activeMtfPeriods = getActiveMtfPeriods(settings, chartTf);
  const chartKey = normalizeMtfPeriod(chartTf);

  const clearAllLayers = () => {
    syncMtfNavigatorLayerRegistry(chartData, pane, [], chartTf);
    const chartPlugin = getNavigatorPluginForPane(chartData, pane);
    chartPlugin?.setData([]);
    chartPlugin?.setBackgroundZones([]);
    chartPlugin?.setLinesVisible(false);
    chartPlugin?.setBackgroundVisible(false);
  };

  if (!paneEnabled || (!hasLines && !hasZones)) {
    clearAllLayers();
    return [];
  }

  const grouped = groupNavigatorLinesByInterval(navigatorData.lines || [], chartTf);
  syncMtfNavigatorLayerRegistry(chartData, pane, activeMtfPeriods, chartTf);

  const zones = settings.backgroundColor
    ? mapNavigatorBackgroundZones(navigatorData.backgroundZones)
    : [];

  const pushLinesToPlugin = (targetPlugin, rawLines, isChartTf) => {
    if (!targetPlugin) return [];
    const mapped = filterNavigatorLinesByTerm(
      mapNavigatorLinesForChart(rawLines, candles),
      pane,
    );
    const applyPluginData = () => {
      targetPlugin.setData(mapped);
      if (isChartTf) {
        targetPlugin.setBackgroundZones(zones);
        targetPlugin.setBackgroundVisible(
          settings.backgroundVisible !== false && settings.backgroundColor,
        );
      } else {
        targetPlugin.setBackgroundZones([]);
        targetPlugin.setBackgroundVisible(false);
      }
      targetPlugin.setLinesVisible(settings.linesVisible !== false);
    };
    const hostChart = getNavigatorPriceChart(chartData, pane);
    if (pane === 'price' && hostChart) {
      window.Viewport?.lockPriceAutoScaleDuring?.(hostChart, applyPluginData);
    } else {
      applyPluginData();
    }
    return mapped;
  };

  let allMappedLines = [];
  allMappedLines = allMappedLines.concat(
    pushLinesToPlugin(
      getMtfNavigatorPlugin(chartData, pane, chartKey, chartTf),
      grouped.get(chartKey) || [],
      true,
    ),
  );

  activeMtfPeriods.forEach((tf) => {
    const key = normalizeMtfPeriod(tf);
    allMappedLines = allMappedLines.concat(
      pushLinesToPlugin(
        getMtfNavigatorPlugin(chartData, pane, key, chartTf),
        grouped.get(key) || [],
        false,
      ),
    );
  });

  if (pane === 'price' && allMappedLines.length) {
    console.log('Lines received:', allMappedLines.length);
    console.log('First line sample:', allMappedLines[0]);
  }

  if (pane === 'price' && settings.barColor && navigatorHasBarColors(navigatorData.barColors)) {
    const colored = applyNavigatorBarColors(
      chartData,
      navigatorData.barColors,
      candles,
      true,
    );
    if (colored && options.updateLoadedCandles !== false) {
      backtestLoadedCandles = colored;
    }
  }

  return buildNavigatorMarkers(navigatorData, candles, pane);
}

function applyNavigatorOverlays(result, candles, chartData = backtestChartData, options = {}) {
  const context = options.context || (chartData === liveChartData ? 'live' : 'backtest');
  const navigators = resolveNavigatorResults(result);
  const allNavMarkers = [];

  NAVIGATOR_PANES.forEach((pane) => {
    const navData = navigators[pane] || null;
    const markers = applyNavigatorPaneOverlay(pane, navData, candles, chartData, { ...options, context });
    if (pane === 'price') {
      const lines = (isNavigatorPaneEnabled('price', context) && navData?.lines?.length)
        ? filterNavigatorLinesByTerm(mapNavigatorLinesForChart(navData.lines, candles), pane)
        : [];
      if (context === 'live') {
        liveNavigatorChartLines = lines;
        liveNavigatorChartMarkers = markers;
      } else if (isNavigatorPaneEnabled('price', context) && navData?.lines?.length) {
        backtestNavigatorChartLines = lines;
        backtestNavigatorChartMarkers = markers;
      } else {
        backtestNavigatorChartLines = [];
        backtestNavigatorChartMarkers = [];
      }
    }
    allNavMarkers.push(...markers);
  });

  return allNavMarkers;
}

function applyNavigatorPriceOverlay(navigatorPrice, candles) {
  applyNavigatorOverlays({ navigatorData: navigatorPrice }, candles, backtestChartData);
}

function applyBacktestCandleMarkers(chartData, trades, osc, navigatorMarkers) {
  const spikeMarkers = buildSpikeMarkers(osc);
  const navMarkers = navigatorMarkers ?? backtestNavigatorChartMarkers;
  applyCandleMarkers(chartData, [...spikeMarkers, ...navMarkers]);
}

function applyBacktestTradeMarkerPrimitive(chartData, trades) {
  if (!chartData?.tradeMarkerPlugin) return;
  chartData.tradeMarkerPlugin.setData(buildTradeMarkerPrimitiveData(trades));
}

function buildBacktestEntryOverlay(trades) {
  const data = [];
  const markers = [];
  (trades || []).forEach((trade) => {
    const entryT = chartTime(trade.entryTime || trade.EntryTime);
    const entryP = parseFloat(trade.entryPrice ?? trade.EntryPrice);
    const side = String(trade.side || trade.Side || '').toUpperCase();
    const isLong = side === 'LONG' || side === 'BUY';
    if (!entryT || !Number.isFinite(entryP)) return;
    data.push({ time: entryT, value: entryP });
    markers.push({
      time: entryT,
      position: 'inBar',
      shape: 'circle',
      color: isLong ? '#9C27B0' : '#E91E63',
      size: 1,
    });
  });
  return {
    data: data.sort((a, b) => a.time - b.time),
    markers: markers.sort((a, b) => a.time - b.time),
  };
}

function applyBacktestEntryMarkers(chartData, trades) {
  applyBacktestTradeMarkerPrimitive(chartData, trades);
  if (!chartData?.entryMarkerSeries) return;
  chartData.entryMarkerSeries.setData([]);
  chartData.entryMarkerSeries.setMarkers([]);
}

function captureBacktestChartView() {
  if (!backtestChartData?.chart || !backtestChartData?.candleSeries) return null;
  const anchor = window.Viewport?.captureViewport(backtestChartData.chart, backtestChartData.candleSeries);
  return {
    anchor,
    priceScaleState: capturePriceScaleState(backtestChartData),
  };
}

function restoreBacktestChartView(view, options = {}) {
  if (window.__isSettingsUpdating) return;
  if (!backtestChartData?.allCharts?.length) return;

  applyPriceScaleState(backtestChartData, view?.priceScaleState ?? view);

  const anchor = view?.anchor ?? null;
  if (!anchor && !backtestLoadedCandles.length) return;

  const apply = () => {
    window.Viewport?.restoreViewportToCharts(
      backtestChartData,
      backtestChartData.candleSeries,
      anchor,
      { candles: backtestLoadedCandles, chartTime, animate: false },
    );
  };

  if (options.immediate) {
    apply();
    return;
  }

  requestAnimationFrame(apply);
  setTimeout(apply, 10);
  setTimeout(apply, 50);
}

function scheduleRestoreBacktestChartView(view, options = {}) {
  if (window.__isSettingsUpdating) return;
  if (!view?.anchor && !view?.timeRange) return;
  if (view.anchor) {
    restoreBacktestChartView(view, options);
    return;
  }
  if (view.timeRange?.from != null && view.timeRange?.to != null) {
    const range = { from: view.timeRange.from, to: view.timeRange.to };
    const applyRange = () => {
      backtestChartData.allCharts.forEach((chart) => syncVisibleTimeRange(chart, range));
    };
    requestAnimationFrame(applyRange);
    setTimeout(applyRange, 10);
  }
}

function applyBacktestIndicatorPatch(result) {
  if (!result || !Array.isArray(result.chartData) || result.chartData.length === 0) return;
  if (!backtestChartData?.candleSeries || backtestLoadedCandles.length === 0) return;

  console.log('[Backtest patch] raw navigatorData:', result.navigatorData, 'navigators:', result.navigators);

  const trades = result.trades || [];
  backtestLastTrades = trades;
  indexBacktestChartPoints(result.chartData);

  const osc = chartPointsToOsc(result.chartData);
  backtestLoadedOsc = alignOscillatorsToCandles(
    mergeOsc(backtestLoadedOsc, osc),
    backtestLoadedCandles,
  );

  const priceChart = backtestChartData.priceChart;
  const prevRange = priceChart?.timeScale()?.getVisibleLogicalRange();

  isUpdatingData = true;
  try {
    applyOscillatorToChart(backtestChartData, backtestLoadedOsc, result.annotations);
    applyWozduhVisibilityToChart(backtestChartData, 'backtest');

    applyNavigatorOverlays(result, backtestLoadedCandles, backtestChartData, {
      preserveView: true,
      updateLoadedCandles: true,
    });
    applyBacktestCandleMarkers(backtestChartData, trades, backtestLoadedOsc, backtestNavigatorChartMarkers);
    applyBacktestEntryMarkers(backtestChartData, trades);
  } finally {
    setTimeout(() => { isUpdatingData = false; }, 0);
  }

  if (prevRange && backtestChartData.allCharts?.length) {
    backtestChartData.allCharts.forEach((chart) => syncVisibleLogicalRange(chart, prevRange));
  }

  renderChartLegends('backtest');
}

function applyBacktestResultToChart(result, options = {}, viewportAnchor = null) {
  if (!result || !Array.isArray(result.chartData) || result.chartData.length === 0) return;

  if (Array.isArray(result.annotations)) {
    backtestAnnotations = result.annotations;
  }

  const anchor = viewportAnchor ?? options.viewportAnchor ?? null;

  const patchIndicatorsOnly = options.patchIndicatorsOnly === true
    || (options.patchIndicatorsOnly !== false && canPatchBacktestIndicatorsOnly(options));

  const container = document.getElementById('backtest-chart-container');
  if (container) container.classList.add('visible');

  if (!backtestChartData?.candleSeries) {
    reinitBacktestChart();
  }
  if (!backtestChartData.candleSeries) return;

  if (patchIndicatorsOnly) {
    applyBacktestIndicatorPatch(result);
    applyIndicatorConfigStyles(backtestChartData);
    return;
  }

  const candles = chartPointsToCandles(result.chartData);
  const osc = alignOscillatorsToCandles(chartPointsToOsc(result.chartData), candles);
  backtestLoadedCandles = candles;
  backtestLoadedOsc = osc;
  backtestLastTrades = result.trades || [];
  indexBacktestChartPoints(result.chartData);
  backtestHistoryHasMore = true;
  backtestHistoryLoading = false;

  isUpdatingData = true;
  try {
    applyPriceToChart(backtestChartData, candles);
    applyOscillatorToChart(backtestChartData, osc, result.annotations);
    applyWozduhVisibilityToChart(backtestChartData, 'backtest');

    applyNavigatorOverlays(result, candles, backtestChartData, {
      updateLoadedCandles: true,
    });
    applyBacktestCandleMarkers(backtestChartData, result.trades, osc, backtestNavigatorChartMarkers);
    applyBacktestEntryMarkers(backtestChartData, result.trades);
    renderChartLegends('backtest');

    applyIndicatorConfigStyles(backtestChartData);

    window.Viewport?.restoreViewportToCharts(
      backtestChartData,
      backtestChartData.candleSeries,
      anchor,
      { candles, chartTime, animate: false },
    );
  } finally {
    setTimeout(() => { isUpdatingData = false; }, 0);
  }
}

function renderBacktestChart(result) {
  applyBacktestResultToChart(result, {});
}


function initEquityChart() {
  const container = document.getElementById('equity-chart');
  if (!container || typeof LightweightCharts === 'undefined') return;

  equityChart = LightweightCharts.createChart(container, {
    layout: { ...sharedChartLayout(), background: { type: 'solid', color: TV.bg }, textColor: '#d1d4dc' },
    grid: {
      vertLines: { color: TV.grid },
      horzLines: { color: TV.grid },
    },
    crosshair: { mode: LightweightCharts.CrosshairMode.Normal },
    timeScale: {
      borderColor: TV.border,
      timeVisible: true,
      secondsVisible: false,
    },
    width: container.clientWidth,
    height: 300,
    rightPriceScale: {
      borderColor: TV.border,
      autoScale: true,
    },
  });

  equitySeries = equityChart.addLineSeries({
    color: TV.green,
    lineWidth: 2,
    priceLineVisible: false,
    lastValueVisible: true,
  });
}

function resizeEquityChart() {
  const container = document.getElementById('equity-chart');
  if (!equityChart || !container) return;
  equityChart.applyOptions({ width: container.clientWidth, height: 300 });
}

function formatStatNum(value, digits = 2) {
  const n = Number(value);
  if (!Number.isFinite(n)) return '0.00';
  return n.toFixed(digits);
}

function setStatValue(id, value, digits = 2, colorize = false) {
  const el = document.getElementById(id);
  if (!el) return;
  el.textContent = formatStatNum(value, digits);
  if (!colorize) return;
  el.classList.remove('positive', 'negative');
  const n = Number(value);
  if (n > 0) el.classList.add('positive');
  else if (n < 0) el.classList.add('negative');
}

function formatStatTime(sec) {
  const n = Number(sec);
  if (!Number.isFinite(n) || n <= 0) return '—';
  return new Date(n * 1000).toLocaleString();
}

function normalizeTradeRow(t) {
  const pnl = Number(t.pnl ?? 0);
  return {
    entryTime: t.entryTime ?? 0,
    exitTime: t.exitTime ?? t.time ?? 0,
    side: t.side || '—',
    entryPrice: t.entryPrice,
    exitPrice: t.exitPrice,
    fee: t.fee,
    slippagePct: t.slippagePct,
    pnl,
    exitReason: t.exitReason || t.reason || '—',
    duration: t.duration,
  };
}

function renderStatsDashboard(data, mode) {
  if (!data) {
    setStatValue('stat-total-trades', 0, 0, false);
    setStatValue('stat-win-rate', 0, 2, false);
    setStatValue('stat-net-profit', 0, 2, true);
    setStatValue('stat-profit-factor', 0, 2, false);
    setStatValue('stat-max-drawdown', 0, 2, false);
    setStatValue('stat-recovery-factor', 0, 2, false);
    if (equitySeries) equitySeries.setData([]);
    const tbody = document.getElementById('trade-history-body');
    if (tbody) {
      const hint = mode === 'backtest'
        ? 'Run a backtest to populate statistics'
        : 'No trades yet';
      tbody.innerHTML = `<tr><td colspan="8" class="stats-empty">${hint}</td></tr>`;
    }
    return;
  }

  setStatValue('stat-total-trades', data.totalTrades, 0, false);
  setStatValue('stat-win-rate', data.winRate, 2, false);
  setStatValue('stat-net-profit', data.netProfit, 2, true);
  setStatValue('stat-profit-factor', data.profitFactor, 2, false);
  setStatValue('stat-max-drawdown', data.maxDrawdown, 2, false);
  setStatValue('stat-recovery-factor', data.recoveryFactor, 2, false);

  if (equitySeries && Array.isArray(data.equityCurve)) {
    equitySeries.setData(
      data.equityCurve.map((p) => ({ time: p.time, value: p.value })),
    );
    equityChart?.timeScale().fitContent();
  }

  const tbody = document.getElementById('trade-history-body');
  if (!tbody) return;

  const trades = (data.trades || []).map(normalizeTradeRow);
  if (trades.length === 0) {
    const hint = mode === 'backtest'
      ? 'Run a backtest to populate statistics'
      : 'No trades yet';
    tbody.innerHTML = `<tr><td colspan="8" class="stats-empty">${hint}</td></tr>`;
    return;
  }

  tbody.innerHTML = trades.map((t) => {
    const pnlClass = t.pnl >= 0 ? 'pnl-pos' : 'pnl-neg';
    const pnlSign = t.pnl >= 0 ? '+' : '';
    const feeParts = [];
    if (Number.isFinite(t.fee) && t.fee > 0) feeParts.push(`$${formatStatNum(t.fee, 2)}`);
    if (Number.isFinite(t.slippagePct) && t.slippagePct > 0) {
      feeParts.push(`${formatStatNum(t.slippagePct, 3)}% slip`);
    }
    const feeCell = feeParts.length ? feeParts.join(' / ') : '—';
    return `<tr>
      <td>${formatStatTime(t.entryTime)}</td>
      <td>${formatStatTime(t.exitTime)}</td>
      <td>${t.side}</td>
      <td>${formatStatNum(t.entryPrice, 2)}</td>
      <td>${formatStatNum(t.exitPrice, 2)}</td>
      <td>${feeCell}</td>
      <td class="${pnlClass}">${pnlSign}${formatStatNum(t.pnl, 2)}%</td>
      <td>${t.exitReason}</td>
    </tr>`;
  }).join('');
}

async function refreshStatsForMode(mode) {
  statsMode = mode || 'backtest';
  document.querySelectorAll('.stats-mode-selector .mode-btn').forEach((btn) => {
    btn.classList.toggle('active', btn.dataset.mode === statsMode);
  });

  if (statsMode === 'backtest') {
    renderStatsDashboard(lastBacktestResult, statsMode);
    return;
  }

  try {
    const resp = await fetch(`/api/stats?mode=${encodeURIComponent(statsMode)}`);
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const payload = await resp.json();
    renderStatsDashboard(payload, statsMode);
  } catch (err) {
    console.warn('Stats fetch failed:', err);
    renderStatsDashboard({
      totalTrades: 0,
      winRate: 0,
      netProfit: 0,
      maxDrawdown: 0,
      profitFactor: 0,
      recoveryFactor: 0,
      trades: [],
      equityCurve: [],
    }, statsMode);
  }
}

function initStatsModeSelector() {
  const root = document.querySelector('.stats-mode-selector');
  if (!root) return;
  root.querySelectorAll('.mode-btn').forEach((btn) => {
    btn.addEventListener('click', () => {
      const mode = btn.dataset.mode;
      if (mode) refreshStatsForMode(mode);
    });
  });
}

function renderBacktestStats(result) {
  lastBacktestResult = result;
  backtestAnnotations = Array.isArray(result.annotations) ? result.annotations : [];
  if (statsMode === 'backtest') {
    renderStatsDashboard(result, 'backtest');
  }
}

function appendBacktestLog(message) {
  const el = document.getElementById('backtest-logs');
  if (!el) return;
  el.textContent += `${message}\n`;
  el.scrollTop = el.scrollHeight;
}

function setDefaultBacktestDates() {
  const end = new Date();
  const start = new Date(end);
  start.setDate(start.getDate() - 30);
  const fmt = (d) => d.toISOString().slice(0, 10);
  const startEl = document.getElementById('bt-start');
  const endEl = document.getElementById('bt-end');
  if (startEl && !startEl.value) startEl.value = fmt(start);
  if (endEl && !endEl.value) endEl.value = fmt(end);
}

function indexBacktestChartPoints(points) {
  backtestChartPointsByTime = new Map();
  if (!Array.isArray(points)) {
    rebuildBacktestTradeMarkerTimes(backtestLastTrades);
    return;
  }
  points.forEach((p) => {
    const t = chartTime(p?.time ?? p?.Time);
    if (t != null) backtestChartPointsByTime.set(t, p);
  });
  rebuildBacktestTradeMarkerTimes(backtestLastTrades);
}

function setBacktestRunState(active) {
  backtestRunActive = active;
  const runBtn = document.getElementById('btn-run-backtest');
  const stopBtn = document.getElementById('btn-stop-backtest');
  if (runBtn) {
    runBtn.disabled = active;
    runBtn.textContent = active ? 'Running…' : 'Run';
  }
  if (stopBtn) stopBtn.disabled = !active;
}

async function stopBacktest() {
  if (!backtestRunActive) return;
  appendBacktestLog('[Backtest] Stop requested…');
  try {
    await fetch('/api/backtest/stop', { method: 'POST' });
  } catch (err) {
    console.warn('Backtest stop request failed:', err);
  }
  backtestAbortController?.abort();
}

function resetBacktestRunUi() {
  setBacktestRunState(false);
  backtestAbortController = null;
  setBacktestLoading(false);
}

async function runBacktest(autoSwitchTabOrOptions = true, options = {}) {
  let autoSwitchTab = true;
  if (typeof autoSwitchTabOrOptions === 'object' && autoSwitchTabOrOptions !== null) {
    options = autoSwitchTabOrOptions;
    autoSwitchTab = options.switchTab !== false;
  } else {
    autoSwitchTab = autoSwitchTabOrOptions !== false;
  }

  const manageLoading = options.manageLoading !== false;
  const skipSettingsPush = options.skipSettingsPush === true;
  const shouldSwitchTab = options.switchTab !== undefined ? options.switchTab : autoSwitchTab;
  const forceFullReload = options.patchIndicatorsOnly === false;
  const patchIndicatorsOnly = !forceFullReload && (
    options.patchIndicatorsOnly === true
    || (canPatchBacktestIndicatorsOnly(options) && backtestLoadedCandles.length > 0)
  );

  const anchor = options.viewportAnchor
    ?? (backtestChartData?.chart && backtestChartData?.candleSeries
      ? window.Viewport?.captureViewport(backtestChartData.chart, backtestChartData.candleSeries)
      : null);

  const symbol = document.getElementById('bt-symbol')?.value.trim() || 'BTCUSDT';
  const interval = normalizeTf(
    document.getElementById('bt-interval')?.value || backtestTf || getActiveTfFromToolbar() || '15m',
  );
  applyBacktestDateRangeLimits(interval);
  const startDate = document.getElementById('bt-start')?.value || '';
  const endDate = document.getElementById('bt-end')?.value || '';
  const isSettingsRefresh = patchIndicatorsOnly && backtestLoadedCandles.length > 0;

  if (manageLoading) {
    setBacktestLoading(true);
  }

  const logs = document.getElementById('backtest-logs');
  if (!isSettingsRefresh && logs) logs.textContent = '';

  if (!isSettingsRefresh) {
    appendBacktestLog(`[Backtest] Symbol: ${symbol} | TF: ${interval}`);
    appendBacktestLog(`[Backtest] Range: ${startDate || '?'} → ${endDate || '?'}`);
    appendBacktestLog('[Backtest] Downloading historical data...');
  } else {
    appendBacktestLog('[Backtest] Recalculating with updated settings...');
  }

  if (!isSettingsRefresh) setBacktestRunState(true);
  backtestAbortController = new AbortController();

  const slowMsgTimer = isSettingsRefresh
    ? null
    : setTimeout(() => {
        appendBacktestLog('Downloading data... This may take time for large ranges');
      }, 5000);

  try {
    let payload = buildFinalBacktestPayload({ symbol, interval, startDate, endDate });
    const settingsPayload = payload.settings;

    console.log('[FALCON SEND] Final payload built:', payload);
    console.log('[FALCON SEND] Matrix keys:', Object.keys(settingsPayload.matrix || {}));
    console.log('[FALCON SEND] Navigator panes:', Object.keys(settingsPayload.navigators || {}));

    if (!settingsPayload?.matrix || !settingsPayload?.navigators) {
      throw new Error('Backtest payload missing matrix or navigators in settings');
    }

    if (!skipSettingsPush) {
      await pushRsxSettingsToServer(coerceRsxSettingsForAPI(backtestRsxSettings));
    }

    let result;
    let rawText = '';
    for (let attempt = 0; attempt < 2; attempt++) {
      const resp = await fetch('/api/backtest/run', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
        signal: backtestAbortController?.signal,
      });

      rawText = await resp.text();
      try {
        result = rawText ? JSON.parse(rawText) : {};
      } catch (parseErr) {
        console.error('[FALCON NETWORK] Server returned invalid JSON. Raw response:', rawText);
        throw new Error(`Server response is not valid JSON. Check console for raw text. Status: ${resp.status}`);
      }

      if (resp.ok) {
        break;
      }

      const errText = result.error || result.message || rawText || '';
      const notEnoughCandles = resp.status === 400 && /not enough candles/i.test(errText);
      if (attempt === 0 && notEnoughCandles) {
        const expanded = expandBacktestStartDate(payload.startDate, payload.endDate, 90);
        if (expanded && expanded !== payload.startDate) {
          console.warn(`[Backtest] Auto-expanding startDate ${payload.startDate} → ${expanded} (retry after: ${errText})`);
          appendBacktestLog(`[Backtest] Extending range: start → ${expanded}`);
          payload = buildFinalBacktestPayload({
            symbol: payload.symbol,
            interval: payload.interval,
            startDate: expanded,
            endDate: payload.endDate,
          });
          const startEl = document.getElementById('bt-start');
          if (startEl) startEl.value = expanded;
          continue;
        }
      }

      const errorText = errText || `HTTP error ${resp.status}`;
      alert(`Ошибка Бэктеста:\n${errorText}`);
      appendBacktestLog(`[Backtest] Error: ${errorText}`);
      return;
    }

    const navResults = resolveNavigatorResults(result);
    const priceLineCount = navResults.price?.lines?.length
      ?? result.navigatorData?.price?.lines?.length
      ?? result.navigatorData?.lines?.length
      ?? result.navigatorPrice?.lines?.length
      ?? 0;
    console.log('Lines received:', priceLineCount);

    appendBacktestLog(result.cancelled
      ? `[Backtest] Stopped early. Trades: ${result.totalTrades} | Win Rate: ${result.winRate}%`
      : '[Backtest] Simulation complete.');
    if (!result.cancelled) {
      appendBacktestLog(`[Backtest] Trades: ${result.totalTrades} | Win Rate: ${result.winRate}%`);
    }
    if (Array.isArray(result.chartData)) {
      appendBacktestLog(`[Backtest] Chart points: ${result.chartData.length}`);
    }

    applyBacktestResultToChart(result, {
      patchIndicatorsOnly,
    }, anchor);
    renderBacktestStats(result);
    clearHistoricalInspection(false);
    if (!shouldSwitchTab) {
      const endData = getDefaultBacktestTelemetryData();
      if (endData) updateScoringUI(endData, { source: 'backtest', force: true });
    }
    if (shouldSwitchTab) {
      switchTab('tab-stats');
    }
  } catch (err) {
    if (err?.name === 'AbortError') {
      appendBacktestLog('[Backtest] Client request aborted.');
      return;
    }
    const msg = err?.message || String(err);
    appendBacktestLog(`[Backtest] Error: ${msg}`);
    console.error('Backtest failed:', err);
    alert(`Ошибка Бэктеста:\n${msg}`);
  } finally {
    if (slowMsgTimer) clearTimeout(slowMsgTimer);
    resetBacktestRunUi();
  }
}

function initConsoleDrawer() {
  const drawer = document.getElementById('console-drawer');
  const toggle = document.getElementById('console-toggle');
  const closeBtn = document.getElementById('console-close');
  if (!drawer) return;

  const open = () => {
    drawer.hidden = false;
    toggle?.classList.add('active');
  };
  const close = () => {
    drawer.hidden = true;
    toggle?.classList.remove('active');
  };

  toggle?.addEventListener('click', () => {
    if (drawer.hidden) open();
    else close();
  });
  closeBtn?.addEventListener('click', close);
}

function shiftBacktestDate(inputId, deltaMonths) {
  const el = document.getElementById(inputId);
  if (!el) return;
  const seed = el.value || new Date().toISOString().slice(0, 10);
  const d = new Date(`${seed}T12:00:00Z`);
  d.setUTCMonth(d.getUTCMonth() + deltaMonths);
  el.value = d.toISOString().slice(0, 10);
}

function initBacktestDateNav() {
  document.querySelectorAll('[data-date-nav]').forEach((btn) => {
    btn.addEventListener('click', (e) => {
      e.preventDefault();
      const id = btn.dataset.dateNav;
      const dir = Number(btn.dataset.dir) || 0;
      if (id && dir) shiftBacktestDate(id, dir);
    });
  });
}

function initBacktest() {
  setDefaultBacktestDates();
  backtestTf = getBacktestInterval();
  initBacktestDateNav();
  initBacktestIntervalHandler();
  document.getElementById('btn-run-backtest')?.addEventListener('click', () => runBacktest(true));
  document.getElementById('btn-stop-backtest')?.addEventListener('click', () => stopBacktest());
  initConsoleDrawer();
}

function setTextIfChanged(el, next) {
  if (el && el.textContent !== next) el.textContent = next;
}

function updateHeader(state) {
  if (!state) return;

  if (typeof state.sandboxMode !== 'undefined') {
    cachedSandboxMode = !!state.sandboxMode;
  }
  const isSandbox = typeof state.sandboxMode !== 'undefined'
    ? !!state.sandboxMode
    : cachedSandboxMode;

  if (state.symbol) {
    setTextIfChanged(document.getElementById('symbol'), state.symbol || 'BTCUSDT');
  }
  if (state.timeframe || state.tradingTimeframe) {
    setTextIfChanged(
      document.getElementById('timeframe-label'),
      tfLabel(state.timeframe || state.tradingTimeframe || currentTf),
    );
  }

  if ('volatilityRegime' in state) {
    const regime = state.volatilityRegime || '';
    const regimeEl = document.getElementById('regime');
    if (regimeEl) {
      setTextIfChanged(regimeEl, regime || '—');
      const regimeClass = regime ? `regime meta-val ${regime}` : 'regime meta-val';
      if (regimeEl.className !== regimeClass) regimeEl.className = regimeClass;
    }
  }

  if (state.jurik != null) {
    setTextIfChanged(document.getElementById('jurik-val'), fmt(state.jurik));
  }
  if (state.redLine != null) {
    setTextIfChanged(document.getElementById('red-val'), fmt(state.redLine));
  }
  if (state.greenLine != null) {
    setTextIfChanged(document.getElementById('green-val'), fmt(state.greenLine));
  }

  const sandboxEl = document.getElementById('sandbox-badge');
  if (sandboxEl) {
    if (sandboxEl.classList.contains('active') !== isSandbox) {
      sandboxEl.classList.toggle('active', isSandbox);
    }
  }
}

async function fetchPollState() {
  if (!shouldRunLivePoll()) {
    return { warmingUp: false, data: {} };
  }
  const resp = await fetch(apiFetchUrl('/api/state', {
    tf: currentTf,
    limit: LIVE_POLL_CANDLE_LIMIT,
    poll: '1',
    rsxLookback: liveRsxSettings.div_lookback,
  }), { cache: 'no-store' });
  const data = await resp.json().catch(() => ({}));
  if (resp.status === 400 && data.status === 'unavailable') {
    throw new Error(`Timeframe ${data.timeframe} not available`);
  }
  if (resp.status === 503 || data.status === 'warming_up') {
    return { warmingUp: true, data };
  }
  if (!resp.ok) throw new Error(`API error: ${resp.status}`);
  return { warmingUp: false, data };
}

async function fetchState(options = {}) {
  const params = apiQueryParams(options.params || {});
  if (options.navigatorsOnly) {
    params.navigators = '1';
    params.limit = 0;
  }

  const timeoutMs = options.timeoutMs ?? LIVE_STATE_FETCH_TIMEOUT_MS;
  const timeoutController = new AbortController();
  const timeoutId = setTimeout(() => {
    timeoutController.abort(new DOMException('fetchState timeout', 'TimeoutError'));
  }, timeoutMs);

  let signal = options.signal;
  if (signal) {
    const combined = new AbortController();
    const abortCombined = () => combined.abort();
    signal.addEventListener('abort', abortCombined, { once: true });
    timeoutController.signal.addEventListener('abort', abortCombined, { once: true });
    signal = combined.signal;
  } else {
    signal = timeoutController.signal;
  }

  try {
    const resp = await fetch(apiFetchUrl('/api/state', params), {
      cache: 'no-store',
      signal,
    });
    const data = await resp.json().catch(() => ({}));
    if (resp.status === 400 && data.status === 'unavailable') {
      throw new Error(`Timeframe ${data.timeframe} not available`);
    }
    if (resp.status === 503 || data.status === 'warming_up') {
      return { warmingUp: true, data };
    }
    if (!resp.ok) throw new Error(`API error: ${resp.status}`);
    return { warmingUp: false, data };
  } finally {
    clearTimeout(timeoutId);
  }
}

function setAllPriceData(candles) {
  const sorted = dedupeCandles(candles);
  const lineData = toLineClose(sorted);
  lineData.sort((a, b) => a.time - b.time);
  liveChartData.candleSeries.setData(sorted);
  liveChartData.barSeries.setData(sorted);
  liveChartData.lineSeries.setData(lineData);
}

function updateAllPriceSeries(bar) {
  const normalized = normalizeCandle(bar);
  if (!normalized) return;
  liveChartData.candleSeries.update(normalized);
  liveChartData.barSeries.update(normalized);
  liveChartData.lineSeries.update({ time: normalized.time, value: normalized.close });
  if (normalized.volume != null) {
    liveChartData.volumeSeries.update({
      time: normalized.time,
      value: normalized.volume,
      color: normalized.close >= normalized.open ? CHART_STYLES.volumeBar.upColor : CHART_STYLES.volumeBar.downColor,
    });
    updateVolumeLabel(loadedCandles.length ? loadedCandles : [normalized]);
  }
  if (tradeMarkers.length > 0) {
    applyAllMarkers();
  }
}

function syncLiveChartPanesFromPrice() {
  if (!liveChartData?.allCharts?.length || !liveChartData?.priceChart) return;
  const mainRange = liveChartData.priceChart.timeScale().getVisibleLogicalRange();
  if (mainRange) {
    syncChartGroupLogicalRange(liveChartData.allCharts, liveChartData.priceChart, mainRange);
  }
}

function applySeriesData(options = {}) {
  const {
    isPrepend = false,
    prependPrevRange = null,
    preserveViewport = null,
  } = options;
  let prevRange = preserveViewport?.logicalRange ?? null;
  if (!prevRange && !isPrepend && shouldPaintLiveChart() && liveChartData?.priceChart) {
    prevRange = liveChartData.priceChart.timeScale().getVisibleLogicalRange();
  }

  const prependOldTotal = prependPrevRange?.oldTotal;
  let prependOldRange = null;
  if (isPrepend && liveChartData?.priceChart?.timeScale) {
    prependOldRange = liveChartData.priceChart.timeScale().getVisibleLogicalRange();
  }

  const candles = dedupeCandles(loadedCandles);
  loadedCandles = candles;

  if (!shouldPaintLiveChart()) return;

  applyPriceToChart(liveChartData, candles);
  updateVolumeLabel(candles);
  loadedOsc = alignOscillatorsToCandles(mergeOsc([], loadedOsc), candles);
  applyOscillatorToChart(liveChartData, loadedOsc, liveAnnotations);
  applyWozduhVisibilityToChart(liveChartData, 'live');
  spikeMarkers = buildSpikeMarkers(loadedOsc);
  applyTradeMarkers();
  if (liveNavigatorResult) {
    applyNavigatorOverlays(
      { navigators: liveNavigatorResult },
      loadedCandles,
      liveChartData,
      { context: 'live', updateLoadedCandles: false },
    );
  }
  updateBufferingOverlay();

  if (isPrepend && prependOldRange != null && prependOldTotal != null) {
    const shiftCount = candles.length - prependOldTotal;
    if (shiftCount > 0) {
      const nextRange = {
        from: prependOldRange.from + shiftCount,
        to: prependOldRange.to + shiftCount,
      };
      runWithSuppressedHistoryLoad(() => {
        liveChartData.allCharts.forEach((chart) => {
          syncVisibleLogicalRange(chart, nextRange, { animate: false });
        });
      });
    }
    return;
  }

  applyLiveViewportAfterData(liveChartData, candles, {
    prevRange,
    isPrepend,
  });

  if (preserveViewport?.logicalRange) {
    restoreLiveViewportSnapshotTwice(preserveViewport);
  }
}

function applyLatestOscPoint(pt) {
  if (!pt) return;
  if (loadedCandles && loadedCandles.length > 0) {
    const latestCandleTime = loadedCandles[loadedCandles.length - 1].time;
    if (pt.time !== latestCandleTime) {
      pt.time = latestCandleTime;
    }
  }
  const time = chartTime(pt?.time ?? pt?.Time);
  if (time == null) return;

  loadedOsc = mergeOsc(loadedOsc, [pt]);
  updateWozduxPoint(loadedOsc[loadedOsc.length - 1]);

  const rsxVal = parseFloat(pt.rsx ?? pt.jurik);
  if (Number.isFinite(rsxVal)) {
    updateRsxPoint(time, rsxVal, pt.color, pt.marker, pt.rsx_signal ?? pt.rsxSignal);
  }

  setTextIfChanged(document.getElementById('jurik-val'), fmt(rsxVal));
  setTextIfChanged(document.getElementById('red-val'), fmt(pt.redLine));
  setTextIfChanged(document.getElementById('green-val'), fmt(pt.greenLine));
}

function renderState(data, options = {}) {
  console.log('[X-Ray RAW] First raw osc point:', data?.oscillators?.[0]);
  console.log('[X-Ray RAW] Last raw osc point:', data?.oscillators?.[data?.oscillators?.length - 1]);

  syncTradingTimeframeFromState(data);
  updateHeader(data);
  // updateScoringUI(data);
  if (typeof data.tickBufferLen === 'number') {
    lastTickBufferLen = data.tickBufferLen;
  }

  const candles = dedupeCandles(toCandles(data.candles));
  const masterState = data.masterState || deriveBotStatus({ ...data, trades: data.trades });
  mergeSessionTrades(data.trades, {
    masterState,
    lastCandleTime: candles.length ? candles[candles.length - 1].time : null,
  });

  const hasMore = typeof data.hasMore === 'boolean' ? data.hasMore : candles.length > 0;

  if (candles.length > 0 && candles.length < 20 && hasMore) {
    // Бэкенд отдал слишком мало свечей (вероятно, только live-буфер), потому что в фоне
    // прямо сейчас идет скачивание истории (gap fill). Одна свеча растянется на весь экран.
    setTimeout(() => loadDashboard(), 500);
    updateBufferingOverlay();
    return;
  }

  loadedCandles = candles;
  loadedOsc = alignOscillatorsToCandles(mergeOsc([], data.oscillators || []), loadedCandles);
  const incomingAnnotations = (Array.isArray(data.annotations) ? data.annotations : [])
    .map(normalizeAnnotation)
    .filter(Boolean);

  if (!liveAnnotations) liveAnnotations = [];
  const annMap = new Map();
  liveAnnotations.map(normalizeAnnotation).filter(Boolean).forEach((a) => {
    annMap.set(`${a.time}-${a.text}`, a);
  });
  incomingAnnotations.forEach((a) => annMap.set(`${a.time}-${a.text}`, a));
  liveAnnotations = Array.from(annMap.values()).sort((a, b) => a.time - b.time);
  historyHasMore = hasMore;
  if (data.navigators) {
    liveNavigatorResult = data.navigators;
  }

  beginDataUpdate();
  try {
    applySeriesData({
      preserveViewport: options.preserveViewport ?? null,
      isPrepend: options.isPrepend === true,
      prependPrevRange: options.prependPrevRange ?? null,
    });
    renderFibZones(data.fibZones);
    if (loadedCandles.length > 0 && shouldPaintLiveChart()) {
      if (!options.preserveViewport?.logicalRange && !options.isPrepend) {
        syncLiveChartPanesFromPrice();
      }
    }
  } finally {
    endDataUpdate(0);
  }

  // График полностью загружен историей, теперь WebSocket может рисовать новые тики
  chartInitialized = true;
  updateBufferingOverlay();
}

async function pollOrderFlowState() {
  if (!shouldPaintLiveChart()) return;
  if (!isOrderFlowTf()) return;
  try {
    const { warmingUp, data } = await fetchLiveState();
    if (warmingUp) return;
    if (typeof data.tickBufferLen === 'number') {
      lastTickBufferLen = data.tickBufferLen;
    }
    const candles = dedupeCandles(toCandles(data.candles));
    if (candles.length === 0) {
      updateBufferingOverlay();
      return;
    }
    loadedCandles = candles;
    loadedOsc = mergeOsc([], data.oscillators || []);
    beginDataUpdate();
    try {
      applySeriesData();
      syncLiveChartPanesFromPrice();
    } finally {
      endDataUpdate(0);
    }
    chartInitialized = true;
  } catch (err) {
    console.error('pollOrderFlowState:', err);
  }
}

async function pollLatestState() {
  if (!shouldPaintLiveChart()) return;
  if (!chartInitialized) return;
  if (isOrderFlowTf()) return;
  if (liveChartUpdating) return;
  if (!shouldRunLivePoll()) return;
  try {
    const { warmingUp, data } = await fetchPollState();
    if (!shouldRunLivePoll()) return;
    if (warmingUp || !data.candles?.length) return;

    updateHeader({ jurik: data.jurik, redLine: data.redLine, greenLine: data.greenLine });

    const candles = dedupeCandles(toCandles(data.candles));
    const latest = candles[candles.length - 1];
    if (!latest) return;

    const last = loadedCandles[loadedCandles.length - 1];
    if (last && last.time === latest.time) {
      loadedCandles[loadedCandles.length - 1] = latest;
    } else if (!last || latest.time > last.time) {
      if (last && isLiveTickGapTooLarge(last.time, latest.time)) {
        console.warn(`Time gap too large (${latest.time} vs ${last.time}). Ignoring live poll tick in history mode.`);
        return;
      }
      loadedCandles.push(latest);
    } else {
      return;
    }

    beginDataUpdate();
    updateAllPriceSeries(latest);

    const osc = data.oscillators || [];
    if (osc.length > 0) {
      const latestOsc = osc[osc.length - 1];
      applyLatestOscPoint(latestOsc);
      loadedOsc = mergeOsc(loadedOsc, [latestOsc]);
    }
    endDataUpdate();
  } catch (err) {
    liveChartUpdating = false;
    console.error('pollLatestState:', err);
  }
}

let liveNavigatorSettingsSynced = false;

async function loadDashboard(options = {}) {
  const reqId = ++currentLiveRequestId;
  if (!liveNavigatorSettingsSynced && shouldPaintLiveChart()) {
    liveNavigatorSettingsSynced = true;
    try {
      await syncLiveNavigatorSettingsToServer();
    } catch (err) {
      console.warn('live navigator settings sync:', err);
    }
  }
  try {
    const { warmingUp, data } = await fetchLiveState({
      userTfChange: options.userTfChange === true,
      navigatorsOnly: options.navigatorsOnly,
    });
    if (reqId !== currentLiveRequestId) return;
    if (syncTradingTimeframeFromState(data) && !options._tfSyncRetried) {
      clearChartData();
      chartInitialized = false;
      return loadDashboard({ ...options, _tfSyncRetried: true });
    }
    if (reqId !== currentLiveRequestId) return;
    if (warmingUp) {
      setTimeout(() => loadDashboard(options), 2000);
      return;
    }
    if (!data.candles || data.candles.length === 0) {
      if (data.status === 'ready') {
        renderState({ ...data, candles: data.candles || [], oscillators: data.oscillators || [] }, options);
        if (isOrderFlowTf(currentTf)) {
          chartInitialized = true;
          updateBufferingOverlay();
        }
        return;
      }
      setTimeout(() => loadDashboard(options), 2000);
      return;
    }
    renderState(data, options);
  } catch (err) {
    if (reqId !== currentLiveRequestId) return;
    const isTimeout = err?.name === 'TimeoutError';
    if (err?.name === 'AbortError' && !isTimeout) {
      return;
    }
    console.error('loadDashboard:', err);
    if (isOrderFlowTf(currentTf)) {
      clearChartData();
      chartInitialized = true;
      updateBufferingOverlay();
      return;
    }
    const retryMs = isTimeout ? 1500 : 3000;
    setTimeout(() => loadDashboard(options), retryMs);
  }
}

async function maybeLoadBacktestHistory(range) {
  if (window.__isSettingsUpdating) return;
  if (!isBacktestTabActive()) return;
  if (!range || backtestHistoryLoading || !backtestHistoryHasMore || backtestLoadedCandles.length === 0) return;
  if (range.from >= 50) return;

  const firstTime = chartTime(backtestLoadedCandles[0].time);
  if (firstTime == null) return;

  const symbol = document.getElementById('bt-symbol')?.value.trim() || 'BTCUSDT';
  const interval = document.getElementById('bt-interval')?.value || '15m';
  const endTimeMs = firstTime < 1e12 ? firstTime * 1000 : firstTime;

  backtestHistoryLoading = true;
  try {
    const params = new URLSearchParams({
      symbol,
      interval,
      endTime: String(endTimeMs),
      limit: String(BACKTEST_HISTORY_CHUNK_LIMIT),
    });
    appendBacktestRsxSettingsToParams(params);
    const resp = await fetch(`/api/history/chunk?${params.toString()}`, { cache: 'no-store' });
    const data = await resp.json().catch(() => ({}));
    if (!resp.ok || !Array.isArray(data.chartData) || data.chartData.length === 0) {
      backtestHistoryHasMore = false;
      return;
    }

    const prependOldRange = backtestChartData.priceChart?.timeScale()?.getVisibleLogicalRange() ?? null;
    const newCandles = chartPointsToCandles(data.chartData);
    const newOsc = chartPointsToOsc(data.chartData);
    const oldLen = backtestLoadedCandles.length;
    const anchorTime = backtestLoadedCandles[0].time;

    const cleanCandles = stripOlderThanBars(newCandles, anchorTime);
    if (cleanCandles.length === 0) {
      backtestHistoryHasMore = data.hasMore !== false;
      return;
    }

    backtestLoadedCandles = prependCandlesZeroOverlap(backtestLoadedCandles, newCandles);
    backtestLoadedOsc = alignOscillatorsToCandles(
      prependOscZeroOverlap(backtestLoadedOsc, newOsc, anchorTime),
      backtestLoadedCandles,
    );
    if (Array.isArray(data.annotations)) {
      backtestAnnotations = prependAnnotationsZeroOverlap(
        data.annotations,
        backtestAnnotations,
        anchorTime,
      );
    }
    const added = backtestLoadedCandles.length - oldLen;

    applyPriceToChart(backtestChartData, backtestLoadedCandles);
    applyOscillatorToChart(backtestChartData, backtestLoadedOsc, backtestAnnotations);
    applyWozduhVisibilityToChart(backtestChartData, 'backtest');
    applyBacktestCandleMarkers(backtestChartData, backtestLastTrades, backtestLoadedOsc);
    applyBacktestEntryMarkers(backtestChartData, backtestLastTrades);

    if (prependOldRange != null && added > 0) {
      const nextRange = {
        from: prependOldRange.from + added,
        to: prependOldRange.to + added,
      };
      backtestChartData.allCharts.forEach((chart) => {
        syncVisibleLogicalRange(chart, nextRange, { animate: false });
      });
    }

    backtestHistoryHasMore = data.hasMore !== false && cleanCandles.length > 0;

    if (added > 0 && backtestHistoryHasMore) {
      requestAnimationFrame(() => {
        const nextRange = backtestChartData.priceChart?.timeScale()?.getVisibleLogicalRange();
        if (nextRange && nextRange.from < 50) {
          maybeLoadBacktestHistory(nextRange);
        }
      });
    }
  } catch (err) {
    console.error('backtest lazy history:', err);
  } finally {
    backtestHistoryLoading = false;
  }
}

async function fetchLiveHistory(endTimeSec) {
  const settings = coerceRsxSettingsForAPI(getRsxSettingsState('live'));
  const params = new URLSearchParams({
    tf: currentTf,
    endTime: String(endTimeSec),
    limit: String(LIVE_STATE_CANDLE_LIMIT),
    rsx_length: String(settings.length),
    rsx_signal_length: String(settings.signal_length),
    rsx_source: settings.source,
    rsx_method: settings.div_method,
    rsx_pivot_radius: String(settings.pivot_radius),
    rsx_div_lookback: String(settings.div_lookback),
    min_price_delta_ratio: String(settings.min_price_delta_ratio),
    min_osc_delta: String(settings.min_osc_delta),
  });
  const resp = await fetch(`/api/history?${params.toString()}`, { cache: 'no-store' });
  const data = await resp.json().catch(() => ({}));
  if (!resp.ok) throw new Error(`history API: ${resp.status}`);
  return data;
}

async function maybeLoadHistory(range, options = {}) {
  const force = options.force === true;
  if (isLoadingHistory || historyLoading || !historyHasMore) return Promise.resolve();
  if (!shouldPaintLiveChart()) return;
  if (!chartInitialized || !liveHistoryScrollArmed) return;
  if (!range || loadedCandles.length === 0) return;
  if (!force && range.from >= LIVE_HISTORY_SCROLL_THRESHOLD) return;

  const firstTime = loadedCandles[0].time;
  if (firstTime == null) return;
  const endTimeSec = Number(firstTime);
  const epoch = liveHistoryEpoch;
  const reqId = currentLiveRequestId;
  const oldLen = loadedCandles.length;
  const anchorTime = loadedCandles[0].time;

  isLoadingHistory = true;
  historyLoading = true;

  try {
    const data = await fetchLiveHistory(endTimeSec);
    if (epoch !== liveHistoryEpoch || reqId !== currentLiveRequestId) return;
    const newCandles = dedupeCandles(toCandles(data.candles));
    const newOsc = (data.oscillators || [])
      .map((p) => normalizeOscPoint(p))
      .filter(Boolean);
    if (newCandles.length === 0) {
      historyHasMore = false;
      return;
    }

    const cleanCandles = stripOlderThanBars(newCandles, anchorTime);
    if (cleanCandles.length === 0) {
      historyHasMore = data.hasMore !== false;
      return;
    }

    loadedCandles = prependCandlesZeroOverlap(loadedCandles, newCandles);
    loadedOsc = alignOscillatorsToCandles(
      prependOscZeroOverlap(loadedOsc, newOsc, anchorTime),
      loadedCandles,
    );
    if (Array.isArray(data.annotations)) {
      liveAnnotations = prependAnnotationsZeroOverlap(data.annotations, liveAnnotations, anchorTime);
    }
    historyHasMore = data.hasMore !== false;

    beginDataUpdate();
    try {
      applySeriesData({
        isPrepend: true,
        prependPrevRange: { oldTotal: oldLen },
      });
    } finally {
      endDataUpdate(0);
    }
    updateBufferingOverlay();
    if (!historyHasMore || epoch !== liveHistoryEpoch) return;

    const added = loadedCandles.length - oldLen;
    if (added > 0) {
      requestAnimationFrame(() => {
        if (epoch !== liveHistoryEpoch || !liveHistoryScrollArmed) return;
        const nextRange = liveChartData.priceChart?.timeScale().getVisibleLogicalRange();
        if (nextRange && nextRange.from < LIVE_HISTORY_SCROLL_THRESHOLD) {
          scheduleHistoryLoad(nextRange);
        }
      });
    }
  } catch (err) {
    console.error('lazy history:', err);
  } finally {
    isLoadingHistory = false;
    historyLoading = false;
  }
}

function scheduleHistoryLoad(range, options = {}) {
  if (!range || !historyHasMore) return;
  if (!chartInitialized || !liveHistoryScrollArmed) return;
  if (range.from >= LIVE_HISTORY_SCROLL_THRESHOLD) return;
  if (liveChartUpdating) {
    pendingHistoryLoad = { range, options };
    return;
  }
  maybeLoadHistory(range, options);
}

function isLiveTf() {
  if (isOrderFlowTf()) return true;
  const scoringTf = backendTradingTimeframe || currentTf;
  return scoringTf.toLowerCase() === currentTf.toLowerCase();
}

function handleWSMessage(event) {
  let msg;
  try { msg = JSON.parse(event.data); } catch { return; }
  if (msg.type !== 'tick' || !msg.data) return;

  const tickTf = (msg.data.timeframe || backendTradingTimeframe || currentTf || '1m').toLowerCase();
  if (tickTf !== currentTf.toLowerCase()) return;

  const d = msg.data;
  const time = chartTime(d.time);
  if (time == null) return;

  if (isLiveTf()) {
    updateHeader({ jurik: d.jurik, redLine: d.redLine, greenLine: d.greenLine });
    if (d.isClosed) {
      if (d.volatilityRegime) {
        updateHeader({ volatilityRegime: d.volatilityRegime });
      }
      // updateScoringUI(d);
    }
  }

  if (!shouldPaintLiveChart()) return;

  // Игнорируем любые тики по WS, пока HTTP-запрос /api/state не принесет исторический фундамент.
  // Бэкенд уже включает самые свежие тики в HTTP-ответ, поэтому мы ничего не теряем.
  if (!chartInitialized) return;

  const bar = normalizeCandle({
    time, open: d.open, high: d.high, low: d.low, close: d.close, volume: d.volume,
  });
  if (!bar) return;

  applyPriceBar(bar);
}

function handleWSMarker(event) {
  if (!shouldPaintLiveChart()) return;
  let msg;
  try { msg = JSON.parse(event.data); } catch { return; }
  if (msg.type !== 'marker' || !msg.data) return;
  const d = msg.data;
  mergeSessionTrades([{
    time: chartTime(d.time),
    side: d.side || d.action,
    price: d.price,
    kind: d.kind || 'entry',
  }]);
  applyTradeMarkers();
}

function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  if (dashboardSocket) {
    dashboardSocket.onopen = null;
    dashboardSocket.onclose = null;
    dashboardSocket.onerror = null;
    dashboardSocket.onmessage = null;
    if (dashboardSocket.readyState === WebSocket.OPEN || dashboardSocket.readyState === WebSocket.CONNECTING) {
      dashboardSocket.close();
    }
  }
  dashboardSocket = new WebSocket(`${proto}://${location.host}/ws`);
  dashboardSocket.onopen = () => {
    wsSubscribeTf(currentTf);
    suppressLivePollForWs();
  };
  dashboardSocket.onmessage = (e) => { handleWSMessage(e); handleWSMarker(e); };
  dashboardSocket.onerror = () => {
    resumeLivePollAfterWs();
  };
  dashboardSocket.onclose = () => {
    resumeLivePollAfterWs();
    setTimeout(connectWS, 3000);
  };
}

function ensureRulerElements(chartData) {
  const wrap = chartData?.elements?.priceWrap;
  if (!wrap) return;

  if (!chartData.rulerShade) {
    chartData.rulerShade = wrap.querySelector('.ruler-shade');
    if (!chartData.rulerShade) {
      chartData.rulerShade = document.createElement('div');
      chartData.rulerShade.className = 'ruler-shade';
      wrap.appendChild(chartData.rulerShade);
    }
  }
  if (!chartData.rulerTooltip) {
    chartData.rulerTooltip = wrap.querySelector('.ruler-tooltip');
    if (!chartData.rulerTooltip) {
      chartData.rulerTooltip = document.createElement('div');
      chartData.rulerTooltip.className = 'ruler-tooltip';
      wrap.appendChild(chartData.rulerTooltip);
    }
  }
}

function rulerPriceSeries(chartData) {
  if (chartData === liveChartData) return activePriceSeries();
  return chartData?.candleSeries;
}

function getRulerChartData() {
  return ruler.chartData || getActiveChartData();
}

function attachRulerToChart(chartData) {
  if (!chartData?.priceChart || chartData.rulerAttached) return;
  ensureRulerElements(chartData);

  const clickHandler = (param) => onRulerClick(param, chartData);
  const moveHandler = (param) => onRulerMove(param, chartData);
  chartData.priceChart.subscribeClick(clickHandler);
  chartData.priceChart.subscribeCrosshairMove(moveHandler);
  chartData._rulerClickHandler = clickHandler;
  chartData._rulerMoveHandler = moveHandler;
  chartData.rulerAttached = true;
}

function initRuler() {
  attachRulerToChart(liveChartData);
  attachRulerToChart(backtestChartData);
  attachBacktestScoringCrosshair(backtestChartData);
}

function setRulerCursor(active) {
  [liveChartData, backtestChartData].forEach((chartData) => {
    const wrap = chartData?.elements?.priceWrap;
    if (!wrap) return;
    const isActiveChart = chartData === getActiveChartData();
    wrap.style.cursor = active && isActiveChart ? 'crosshair' : '';
  });
}

function toggleRuler() {
  ruler.active = !ruler.active;
  ruler.chartData = getActiveChartData();
  document.getElementById('ruler-btn')?.classList.toggle('active', ruler.active);
  setRulerCursor(ruler.active);
  if (!ruler.active) resetRuler();
}

function resetRuler() {
  ruler.state = RULER_IDLE;
  ruler.p1 = null;
  ruler.p2 = null;
  const chartData = getRulerChartData();
  if (chartData?.rulerShade) chartData.rulerShade.style.display = 'none';
  if (chartData?.rulerTooltip) chartData.rulerTooltip.style.display = 'none';
}

function onRulerClick(param, chartData) {
  if (!ruler.active || chartData !== getRulerChartData() || !param.point) return;

  if (ruler.state === RULER_FIXED) {
    resetRuler();
    return;
  }

  const series = rulerPriceSeries(chartData);
  const price = series?.coordinateToPrice(param.point.y);
  const time = chartData.priceChart.timeScale().coordinateToTime(param.point.x);
  if (price == null || time == null) return;

  if (ruler.state === RULER_IDLE) {
    ruler.p1 = { time, price };
    ruler.p2 = { time, price };
    ruler.state = RULER_MEASURING;
    return;
  }

  if (ruler.state === RULER_MEASURING) {
    ruler.p2 = { time, price };
    ruler.state = RULER_FIXED;
    updateRulerOverlay(chartData);
  }
}

function onRulerMove(param, chartData) {
  if (!ruler.active || chartData !== getRulerChartData()) return;
  if (chartData === backtestChartData ? isUpdatingData : liveChartUpdating) return;
  if (ruler.state !== RULER_MEASURING || !param.point) return;

  const series = rulerPriceSeries(chartData);
  const price = series?.coordinateToPrice(param.point.y);
  const time = chartData.priceChart.timeScale().coordinateToTime(param.point.x);
  if (price == null || time == null) return;

  ruler.p2 = { time, price };
  updateRulerOverlay(chartData);
}

function countBarsBetween(t1, t2, chartData) {
  const lo = Math.min(t1, t2);
  const hi = Math.max(t1, t2);
  const candles = chartData === backtestChartData ? backtestLoadedCandles : loadedCandles;
  return candles.filter((c) => c.time >= lo && c.time <= hi).length;
}

function formatDuration(barCount, tf) {
  const interval = tf || getActiveTf();
  if (interval.endsWith('s') || interval.includes('tick')) {
    return `${barCount} bars`;
  }
  if (barCount < 60) return `${barCount} bars, ${barCount}m`;
  const h = Math.floor(barCount / 60);
  const m = barCount % 60;
  return `${barCount} bars, ${h}h ${m}m`;
}

function updateRulerOverlay(chartData) {
  chartData = chartData || getRulerChartData();
  ensureRulerElements(chartData);

  const shade = chartData.rulerShade;
  const tooltip = chartData.rulerTooltip;
  const wrap = chartData.elements?.priceWrap;
  const series = rulerPriceSeries(chartData);

  if (!shade || !tooltip || !wrap || !series) return;

  if (ruler.state === RULER_IDLE || !ruler.p1 || !ruler.p2) {
    shade.style.display = 'none';
    tooltip.style.display = 'none';
    return;
  }

  const x1 = chartData.priceChart.timeScale().timeToCoordinate(ruler.p1.time);
  const x2 = chartData.priceChart.timeScale().timeToCoordinate(ruler.p2.time);
  const y1 = series.priceToCoordinate(ruler.p1.price);
  const y2 = series.priceToCoordinate(ruler.p2.price);

  if (x1 == null || x2 == null || y1 == null || y2 == null) {
    shade.style.display = 'none';
    tooltip.style.display = 'none';
    return;
  }

  const left = Math.min(x1, x2);
  const top = Math.min(y1, y2);
  const width = Math.abs(x2 - x1);
  const height = Math.abs(y2 - y1);

  shade.style.display = 'block';
  shade.style.left = `${left}px`;
  shade.style.top = `${top}px`;
  shade.style.width = `${Math.max(width, 2)}px`;
  shade.style.height = `${Math.max(height, 2)}px`;

  const delta = ruler.p2.price - ruler.p1.price;
  const pct = ruler.p1.price !== 0 ? (delta / ruler.p1.price) * 100 : 0;
  const bars = countBarsBetween(ruler.p1.time, ruler.p2.time, chartData);
  const sign = delta >= 0 ? '+' : '';

  tooltip.style.display = 'block';
  tooltip.innerHTML = `
    <div class="pct">${sign}${fmtPrice(delta)} (${sign}${pct.toFixed(2)}%)</div>
    <div>${formatDuration(bars)}</div>
  `;

  const tipLeft = left + width / 2 - 60;
  tooltip.style.left = `${Math.max(4, Math.min(tipLeft, wrap.clientWidth - 140))}px`;
  tooltip.style.top = `${Math.max(4, top - 52)}px`;
}

function safeInit(moduleName, fn) {
  try {
    fn();
  } catch (err) {
    console.error(`Failed to init ${moduleName}:`, err);
  }
}

function boot() {
  safeInit('strategy state', initStrategyState);
  safeInit('threshold inputs', initThresholdInputs);
  safeInit('tabs', initTabs);
  safeInit('scoring matrix', initScoringMatrix);
  safeInit('risk settings', initRiskSettings);
  safeInit('equity chart', initEquityChart);
  safeInit('stats mode selector', initStatsModeSelector);
  safeInit('backtest', initBacktest);
  safeInit('navigator popup handlers', initNavigatorPopupOkHandlers);
  safeInit('MTF sync checkboxes', initMtfSyncCheckboxes);
  safeInit('facts bar', initFactsBar);

  (async () => {
    try {
      if (typeof LightweightCharts === 'undefined') {
        setTimeout(boot, 500);
        return;
      }

      let chartsReady = false;
      try {
        chartsReady = initCharts();
      } catch (err) {
        console.error('Failed to init charts:', err);
      }
      if (!chartsReady) {
        setTimeout(boot, 500);
        return;
      }

      safeInit('toolbar/controls', () => initControls({ useServerTf: true }));
      syncToolbarToActiveContext();

      if (isOrderFlowTf()) {
        applyOrderFlowTimeScale(true);
      }

      liveNavigatorSettingsSynced = true;
      try {
        await syncLiveNavigatorSettingsToServer();
      } catch (err) {
        console.warn('live navigator settings sync:', err);
      }

      await loadDashboard();
      connectWS();
      if (isOrderFlowTf()) {
        orderFlowPollTimer = setInterval(pollOrderFlowState, 500);
      }

      requestAnimationFrame(() => handleResize());
      isAppInitialized = true;
    } catch (err) {
      console.error('boot failed:', err);
      setTimeout(boot, 3000);
    }
  })();
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', boot);
} else {
  boot();
}
