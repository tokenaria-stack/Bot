

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





const chartLegendState = {
  live: { price: {}, wozduh: {}, rsx: {} },
  backtest: { price: {}, wozduh: {}, rsx: {} },
};

let openNavigatorPopupEl = null;


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
  return API.postNavigatorSettings(navigators);
}

let navigatorAutoUpdateTimer = null;

async function triggerNavigatorAutoUpdate() {
  const context = getNavigatorContext();
  if (context === 'live') {
    const viewportSnapshot = captureLiveViewportSnapshot();
    await syncLiveNavigatorSettingsToServer();
    if (chartInitialized && liveStore.candleCount() > 0 && shouldPaintLiveChart()) {
      await refreshLiveNavigatorFromServer(viewportSnapshot);
    } else {
      await loadDashboard({ preserveViewport: viewportSnapshot });
    }
    return;
  }
  buildFinalBacktestPayload();
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
        liveStore.getForLightweightCharts().candles,
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

      if (getNavigatorContext() === 'live') {
        flushNavigatorAutoUpdate().catch((err) => {
          console.error('[UI] Navigator Ok pipeline failed:', err);
        });
      }
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
let backtestAbortController = null;
let backtestRunActive = false;

let wozduxPriceLines = [];
let rsxPriceLines = [];
let tradeMarkers = [];
let sessionTrades = [];
let spikeMarkers = [];
let fibPriceLines = [];
let lastFibZones = [];
let currentTf = '1m';
let backendTradingTimeframe = null;
let tradingTimeframeSynced = false;
let tfFavorites = [];
let refreshTimer = null;
let historyLoading = false;
let historyHasMore = true;
let currentLiveRequestId = 0;
let navigatorRequestId = 0;
let chartInitialized = false;
let isAppInitialized = false;
let chartType = 'candles';
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
let cachedSandboxMode = false;

if (typeof window !== 'undefined') {
  window.__isSettingsUpdating = false;
  window.__pendingAnchor = null;
}

const liveStore = new ChartDataStore('live');
const backtestStore = new ChartDataStore('backtest');

function getDataStoreForChart(chartData) {
  return chartData === backtestChartData ? backtestStore : liveStore;
}

function getStoreDataForChart(chartData) {
  return getDataStoreForChart(chartData).getForLightweightCharts();
}


function applyOscPointDelta(oscPt, chartData = liveChartData) {
  if (!oscPt || !chartData) return;
  const time = chartTime(oscPt.time);
  if (time == null) return;

  WOZDUX_LINE_KEYS.forEach((key) => {
    const value = Number(oscPt[key]);
    if (!Number.isFinite(value) || !chartData.wozduxSeries?.[key]) return;
    chartData.wozduxSeries[key].update({ time, value });
  });
  if (oscPt.volCrossMarker) {
    applyWozduxMarkers(getStoreDataForChart(chartData).osc, chartData);
  }

  const rsxVal = parseFloat(oscPt.rsx ?? oscPt.jurik);
  if (Number.isFinite(rsxVal) && chartData.rsxSeries) {
    const ptColor = oscPt.color || RSX_DEFAULT_COLOR;
    chartData.rsxSeries.update({ time, value: rsxVal, color: ptColor });
    const signalVal = parseFloat(oscPt.rsx_signal ?? oscPt.rsxSignal);
    if (chartData.rsxSignalSeries && Number.isFinite(signalVal)) {
      chartData.rsxSignalSeries.update({ time, value: signalVal });
    }
    if (chartData === liveChartData) {
      const rsxEl = document.getElementById('rsx-val');
      if (rsxEl) {
        rsxEl.textContent = rsxVal.toFixed(1);
        rsxEl.style.color = ptColor;
      }
    }
  }
}

function applyLiveChartDelta(delta) {
  if (!delta || !shouldPaintLiveChart()) return;

  if (delta.candle) {
    if (liveStore.candleCount() <= 1) {
      const candles = liveStore.getForLightweightCharts().candles;
      setAllPriceData(candles);
      liveChartData.volumeSeries.setData(toVolumeBars(candles));
      updateVolumeLabel(candles);
    } else {
      updateAllPriceSeries(delta.candle);
    }
  }

  if (delta.osc) {
    applyOscPointDelta(delta.osc, liveChartData);
  }

  if (delta.fullAnnotations) {
    const showPivots = rsxShowPivotsFrom(getRsxSettingsState('live'), true);
    const rsxTimes = delta.osc?.time != null
      ? new Set([delta.osc.time])
      : new Set();
    applyUniversalAnnotations(
      getChartAnnotationPanes(liveChartData),
      delta.fullAnnotations,
      { rsx: rsxTimes },
      { showPivots },
    );
  }

  updateBufferingOverlay();
}

function canPatchBacktestIndicatorsOnly(options = {}) {
  if (options.patchIndicatorsOnly === false) return false;
  if (options.patchIndicatorsOnly === true) return true;
  return options.preserveView === true
    && backtestStore.candleCount() > 0
    && !!backtestChartData?.candleSeries;
}


let liveRsxSettings = defaultRsxSettings();
let backtestRsxSettings = defaultRsxSettings();

let backtestHistoryHasMore = true;
let backtestHistoryLoading = false;
let backtestLastTrades = [];
let currentBacktestPayload = null;
if (typeof window !== 'undefined') {
  window.currentBacktestPayload = window.currentBacktestPayload || null;
}

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


/** Client-side TF resolver (mirrors server/timeframes.go aliases). */

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




function getRsxSettingsMenu(wrap) {
  if (!wrap) return null;
  return wrap.querySelector('.indicator-settings-menu');
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

let backtestIntervalChangeInFlight = false;

async function pushRsxSettingsToServer(settings) {
  const payload = coerceRsxSettingsForAPI(settings);
  try {
    const applied = await API.pushRsxSettings(payload);
    return coerceRsxSettingsForAPI(applied);
  } catch (err) {
    console.warn('Failed to push RSX settings:', err);
    throw err;
  }
}

async function fetchRsxIndicatorSettings() {
  const version = ++rsxSettingsFetchVersion;
  const localBeforeFetch = { ...getRsxSettingsState('live') };
  try {
    const serverSettings = await API.fetchRsxSettings();
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
      liveStore.replaceOscAndAnnotations({
        oscillators: data.oscillators,
        annotations: data.annotations,
      });
    } else if (data?.oscillators?.length) {
      liveStore.replaceOscAndAnnotations({ oscillators: data.oscillators });
    }

    beginDataUpdate();
    const storeData = liveStore.getForLightweightCharts();
    applyRsxData(storeData.osc, liveChartData, storeData.annotations);
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

function persistBacktestRsxFromMenu() {
  const settings = readRsxSettingsFromMenu('backtest');
  const applied = setRsxSettingsState('backtest', settings);
  persistRsxSettings('backtest', applied);
  return applied;
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

function buildLiveStateQueryParams(extra = {}) {
  return API.apiQueryParams({
    tf: currentTf,
    rsxLookback: liveRsxSettings.div_lookback,
    limit: LIVE_STATE_CANDLE_LIMIT,
    extra,
    pendingAnchor: window.__pendingAnchor,
    intervalMs: getIntervalMs(currentTf),
  });
}

function abortLiveStateFetch() {
  API.abortLiveStateFetch();
}

async function fetchLiveState(options = {}) {
  return API.fetchLiveState({
    tf: currentTf,
    userTfChange: options.userTfChange === true,
    navigatorsOnly: options.navigatorsOnly === true,
    signal: options.signal,
    params: buildLiveStateQueryParams(options.params || {}),
    timeoutMs: options.timeoutMs,
  });
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
    const settings = await API.fetchRiskSettings();
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
    const applied = await API.postRiskSettings(payload);
    applyRiskSettingsToForm(applied);
    localStorage.setItem(LS_RISK_SETTINGS_KEY, JSON.stringify(applied));
    hideRiskSettingsMenu();
    if (isBacktestTabActive() && backtestStore.candleCount() > 0) {
      buildFinalBacktestPayload();
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

    menu.querySelectorAll('.wozduh-chk').forEach((el) => {
      el.addEventListener('change', () => {
        const prefs = readWozduhPrefsFromMenu(menu);
        saveWozduhPrefs(context, prefs);
        applyWozduhVisibilityToChart(
          context === 'backtest' ? backtestChartData : liveChartData,
          context,
        );
      });
    });
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
          const storeData = getStoreDataForChart(chartData);
          const osc = storeData.osc;
          const anns = storeData.annotations;
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

function resetBacktestClientCacheForTfChange() {
  backtestStore.clear();
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

  if (tf === normalizeTf(backtestTf) && tf === normalizeTf(getBacktestInterval()) && backtestStore.candleCount() > 0) {
    return;
  }

  if (backtestIntervalChangeInFlight) return;

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
  if (!WS.isOpen()) {
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
  return WS.isOpen();
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
    liveStore.candleCount() < 5 ||
    lastTickBufferLen < 500
  );
  el.style.display = buffering ? 'flex' : 'none';
}

function wsSubscribeTf(tf) {
  WS.subscribe(tf, resolveTf(tf) || currentTf);
}

function clearChartData() {
  liveHistoryEpoch += 1;
  disarmLiveHistoryScroll();
  beginDataUpdate();
  liveStore.clear();
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


/** RSX/Wozduh warmup sentinel — 0 is never a valid live oscillator reading. */



function applyPriceBar(bar) {
  if (!bar) return;
  const last = liveStore.lastCandleChartSec();
  if (last && last.time === bar.time) {
    liveStore.upsertCandle(bar);
  } else if (!last || bar.time >= last.time) {
    if (last && isLiveTickGapTooLarge(last.time, bar.time)) {
      console.warn(`Time gap too large (${bar.time} vs ${last.time}). Ignoring live tick in history mode.`);
      return;
    }
    liveStore.upsertCandle(bar);
  } else {
    return;
  }

  if (!shouldPaintLiveChart()) return;

  const delta = liveStore.getLatestDeltaForChart();
  applyLiveChartDelta(delta);
}

function fmt(v) {
  return typeof v === 'number' && Number.isFinite(v) ? v.toFixed(2) : '—';
}

function fmtPrice(v) {
  return typeof v === 'number' && Number.isFinite(v)
    ? v.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })
    : '—';
}







function applyPriceToChart(chartData, candles, options = {}) {
  if (!chartData?.candleSeries) return;
  const sorted = [...(candles || [])].sort((a, b) => a.time - b.time);
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
    applyWozduxMarkers(getStoreDataForChart(chartData).osc, chartData);
  }
}

function fmtVolume(v) {
  if (!Number.isFinite(v) || v <= 0) return '—';
  if (v >= 1e9) return `${(v / 1e9).toFixed(2)} B`;
  if (v >= 1e6) return `${(v / 1e6).toFixed(2)} M`;
  if (v >= 1e3) return `${(v / 1e3).toFixed(2)} K`;
  return v.toFixed(2);
}


function updateVolumeLabel(candles) {
  const el = document.getElementById('volume-val');
  if (!el || !candles?.length) return;
  const last = candles[candles.length - 1];
  el.textContent = fmtVolume(last.volume);
  el.style.color = last.close >= last.open ? TV.green : TV.red;
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
  chartData.rsxSeries.setData(lineData);
  if (chartData.rsxSignalSeries) {
    chartData.rsxSignalSeries.setData(mapRSXSignalData(osc));
  }

  const seriesTimesByPane = {
    rsx: new Set(lineData.map((d) => d.time)),
  };
  const resolved = annotations ?? getStoreDataForChart(chartData).annotations;
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

  if (marker || getDataStoreForChart(chartData).annotationCount() > 0) {
    const storeData = getStoreDataForChart(chartData);
    const mapped = mapRSXData(storeData.osc);
    const showPivots = rsxShowPivotsFrom(getRsxSettingsState(rsxSettingsContextForChart(chartData)), true);
    applyUniversalAnnotations(
      getChartAnnotationPanes(chartData),
      storeData.annotations,
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

function switchTab(targetId) {
  saveThresholdsFromHeaderToState(getActiveStrategyContext());
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
    await API.postThresholds({ long: t.long, short: t.short });
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
    await API.postMatrix(payload);
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

function tradesToStoreAnnotations(trades) {
  const anns = [];
  (trades || []).forEach((trade) => {
    const side = String(trade.side || trade.Side || '').toUpperCase();
    const isLong = side === 'LONG' || side === 'BUY';
    const entryT = trade.entryTime || trade.EntryTime;
    const exitT = trade.time || trade.Time;
    if (entryT) {
      anns.push({
        time: entryT,
        text: isLong ? 'L' : 'S',
        pane: 'rsx',
        position: isLong ? 'belowBar' : 'aboveBar',
      });
    }
    if (exitT) {
      anns.push({ time: exitT, text: 'X', pane: 'rsx', position: 'inBar' });
    }
  });
  return anns;
}

function buildBacktestTradeMarkers(trades) {
  backtestStore._ingestAnnotations(tradesToStoreAnnotations(trades));
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
  if (!chartData || !backtestStore.candleCount()) return;

  const pricePlugin = chartData.priceNavigatorPlugin;
  if (pricePlugin && backtestNavigatorChartLines?.length) {
    pricePlugin.setData(backtestNavigatorChartLines);
    pricePlugin.setLinesVisible(true);
  }

  const navMarkers = backtestNavigatorChartMarkers || [];
  if (navMarkers.length) {
    const storeOsc = backtestStore.getForLightweightCharts().osc;
    applyBacktestCandleMarkers(chartData, backtestLastTrades, storeOsc, navMarkers);
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
    if (colored && options.updateLoadedCandles !== false && context === 'backtest') {
      colored.forEach((c) => backtestStore.upsertCandle(c));
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

function applyBacktestIndicatorPatch(result) {
  if (!result || !Array.isArray(result.chartData) || result.chartData.length === 0) return;
  if (!backtestChartData?.candleSeries || backtestStore.candleCount() === 0) return;

  const trades = result.trades || [];
  backtestLastTrades = trades;

  const patchPayload = { oscillators: chartPointsToOsc(result.chartData) };
  if (Array.isArray(result.annotations)) {
    patchPayload.annotations = result.annotations;
  }
  backtestStore.replaceOscAndAnnotations(patchPayload);
  buildBacktestTradeMarkers(trades);

  const storeData = backtestStore.getForLightweightCharts();
  const priceChart = backtestChartData.priceChart;
  const prevRange = priceChart?.timeScale()?.getVisibleLogicalRange();

  isUpdatingData = true;
  try {
    applyOscillatorToChart(backtestChartData, storeData.osc, storeData.annotations);
    applyWozduhVisibilityToChart(backtestChartData, 'backtest');

    applyNavigatorOverlays(result, storeData.candles, backtestChartData, {
      preserveView: true,
      updateLoadedCandles: true,
    });
    applyBacktestCandleMarkers(backtestChartData, trades, storeData.osc, backtestNavigatorChartMarkers);
    applyBacktestTradeMarkerPrimitive(backtestChartData, trades);
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

  backtestStore.replaceFromServer(chartPointsToStorePayload(result.chartData, result.annotations));
  backtestLastTrades = result.trades || [];
  backtestHistoryHasMore = true;
  backtestHistoryLoading = false;
  buildBacktestTradeMarkers(result.trades);

  const storeData = backtestStore.getForLightweightCharts();

  isUpdatingData = true;
  try {
    applyPriceToChart(backtestChartData, storeData.candles);
    applyOscillatorToChart(backtestChartData, storeData.osc, storeData.annotations);
    applyWozduhVisibilityToChart(backtestChartData, 'backtest');

    applyNavigatorOverlays(result, storeData.candles, backtestChartData, {
      updateLoadedCandles: true,
    });
    applyBacktestCandleMarkers(backtestChartData, result.trades, storeData.osc, backtestNavigatorChartMarkers);
    applyBacktestTradeMarkerPrimitive(backtestChartData, result.trades);
    renderChartLegends('backtest');

    applyIndicatorConfigStyles(backtestChartData);

    window.Viewport?.restoreViewportToCharts(
      backtestChartData,
      backtestChartData.candleSeries,
      anchor,
      { candles: storeData.candles, chartTime, animate: false },
    );
  } finally {
    setTimeout(() => { isUpdatingData = false; }, 0);
  }
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
    const payload = await API.fetchStats(statsMode);
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
  if (statsMode === 'backtest') {
    renderStatsDashboard(result, 'backtest');
  }
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
  try {
    await API.stopBacktest();
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
    || (canPatchBacktestIndicatorsOnly(options) && backtestStore.candleCount() > 0)
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
  const isSettingsRefresh = patchIndicatorsOnly && backtestStore.candleCount() > 0;

  if (manageLoading) {
    setBacktestLoading(true);
  }

  if (!isSettingsRefresh) setBacktestRunState(true);
  backtestAbortController = new AbortController();

  try {
    let payload = buildFinalBacktestPayload({ symbol, interval, startDate, endDate });
    const settingsPayload = payload.settings;

    if (!settingsPayload?.matrix || !settingsPayload?.navigators) {
      throw new Error('Backtest payload missing matrix or navigators in settings');
    }

    if (!skipSettingsPush) {
      await pushRsxSettingsToServer(coerceRsxSettingsForAPI(backtestRsxSettings));
    }

    let result;
    for (let attempt = 0; attempt < 2; attempt++) {
      const { ok, status, result: respResult, rawText } = await API.runBacktest(
        payload,
        backtestAbortController?.signal,
      );
      result = respResult;

      if (result?._parseError) {
        console.error('[FALCON NETWORK] Server returned invalid JSON. Raw response:', rawText);
        throw new Error(`Server response is not valid JSON. Check console for raw text. Status: ${status}`);
      }

      if (ok) {
        break;
      }

      const errText = result.error || result.message || rawText || '';
      const notEnoughCandles = status === 400 && /not enough candles/i.test(errText);
      if (attempt === 0 && notEnoughCandles) {
        const expanded = expandBacktestStartDate(payload.startDate, payload.endDate, 90);
        if (expanded && expanded !== payload.startDate) {
          console.warn(`[Backtest] Auto-expanding startDate ${payload.startDate} → ${expanded} (retry after: ${errText})`);
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

      const errorText = errText || `HTTP error ${status}`;
      alert(`Ошибка Бэктеста:\n${errorText}`);
      return;
    }

    applyBacktestResultToChart(result, {
      patchIndicatorsOnly,
    }, anchor);
    renderBacktestStats(result);
    if (shouldSwitchTab) {
      switchTab('tab-stats');
    }
  } catch (err) {
    if (err?.name === 'AbortError') {
      return;
    }
    const msg = err?.message || String(err);
    console.error('Backtest failed:', err);
    alert(`Ошибка Бэктеста:\n${msg}`);
  } finally {
    resetBacktestRunUi();
  }
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

function setAllPriceData(candles) {
  const sorted = [...(candles || [])].sort((a, b) => a.time - b.time);
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
    updateVolumeLabel(liveStore.candleCount() ? liveStore.getForLightweightCharts().candles : [normalized]);
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

  const storeData = liveStore.getForLightweightCharts();
  const candles = storeData.candles;

  if (!shouldPaintLiveChart()) return;

  applyPriceToChart(liveChartData, candles);
  updateVolumeLabel(candles);
  applyOscillatorToChart(liveChartData, storeData.osc, storeData.annotations);
  applyWozduhVisibilityToChart(liveChartData, 'live');
  spikeMarkers = buildSpikeMarkers(storeData.osc);
  applyTradeMarkers();
  if (liveNavigatorResult) {
    applyNavigatorOverlays(
      { navigators: liveNavigatorResult },
      candles,
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
  const latestCandleTime = liveStore.lastCandleTimeSec();
  if (latestCandleTime != null && pt.time !== latestCandleTime) {
    pt = { ...pt, time: latestCandleTime };
  }
  liveStore.upsertOscPoint(pt);
  const delta = liveStore.getLatestDeltaForChart();
  applyLiveChartDelta(delta);
}

function renderState(data, options = {}) {
  syncTradingTimeframeFromState(data);
  updateHeader(data);
  if (typeof data.tickBufferLen === 'number') {
    lastTickBufferLen = data.tickBufferLen;
  }

  liveStore.replaceFromServer({
    candles: toCandles(data.candles),
    oscillators: data.oscillators || [],
    annotations: data.annotations,
  });
  const storeData = liveStore.getForLightweightCharts();
  const candles = storeData.candles;
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
    if (liveStore.candleCount() > 0 && shouldPaintLiveChart()) {
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
    const candles = toCandles(data.candles);
    if (candles.length === 0) {
      updateBufferingOverlay();
      return;
    }
    liveStore.replaceFromServer({
      candles,
      oscillators: data.oscillators || [],
      annotations: data.annotations,
    });
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
    const { warmingUp, data } = await API.fetchPollState({
      tf: currentTf,
      limit: LIVE_POLL_CANDLE_LIMIT,
      rsxLookback: liveRsxSettings.div_lookback,
    });
    if (!shouldRunLivePoll()) return;
    if (warmingUp || !data.candles?.length) return;

    updateHeader({ jurik: data.jurik, redLine: data.redLine, greenLine: data.greenLine });

    const candles = toCandles(data.candles);
    const latest = candles[candles.length - 1];
    if (!latest) return;

    const last = liveStore.lastCandleChartSec();
    if (last && last.time === latest.time) {
      liveStore.upsertCandle(latest);
    } else if (!last || latest.time > last.time) {
      if (last && isLiveTickGapTooLarge(last.time, latest.time)) {
        console.warn(`Time gap too large (${latest.time} vs ${last.time}). Ignoring live poll tick in history mode.`);
        return;
      }
      liveStore.upsertCandle(latest);
    } else {
      return;
    }

    beginDataUpdate();
    const osc = data.oscillators || [];
    if (osc.length > 0) {
      const latestOsc = osc[osc.length - 1];
      const latestCandleTime = liveStore.lastCandleTimeSec();
      let pt = latestOsc;
      if (latestCandleTime != null && pt.time !== latestCandleTime) {
        pt = { ...pt, time: latestCandleTime };
      }
      liveStore.upsertOscPoint(pt);
      const rsxVal = parseFloat(pt.rsx ?? pt.jurik);
      setTextIfChanged(document.getElementById('jurik-val'), fmt(rsxVal));
      setTextIfChanged(document.getElementById('red-val'), fmt(pt.redLine));
      setTextIfChanged(document.getElementById('green-val'), fmt(pt.greenLine));
    }

    const delta = liveStore.getLatestDeltaForChart();
    applyLiveChartDelta(delta);
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
  if (!range || backtestHistoryLoading || !backtestHistoryHasMore || backtestStore.candleCount() === 0) return;
  if (range.from >= 50) return;

  const firstTime = backtestStore.firstCandleTimeSec();
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
    const { ok, data } = await API.fetchBacktestHistoryChunk(params);
    if (!ok || !Array.isArray(data.chartData) || data.chartData.length === 0) {
      backtestHistoryHasMore = false;
      return;
    }

    const prependOldRange = backtestChartData.priceChart?.timeScale()?.getVisibleLogicalRange() ?? null;
    const oldLen = backtestStore.candleCount();
    backtestStore.prependHistory(chartPointsToStorePayload(data.chartData, data.annotations));
    const added = backtestStore.candleCount() - oldLen;
    if (added === 0) {
      backtestHistoryHasMore = data.hasMore !== false;
      return;
    }

    const storeData = backtestStore.getForLightweightCharts();

    applyPriceToChart(backtestChartData, storeData.candles);
    applyOscillatorToChart(backtestChartData, storeData.osc, storeData.annotations);
    applyWozduhVisibilityToChart(backtestChartData, 'backtest');
    applyBacktestCandleMarkers(backtestChartData, backtestLastTrades, storeData.osc);
    applyBacktestTradeMarkerPrimitive(backtestChartData, backtestLastTrades);

    if (prependOldRange != null && added > 0) {
      const nextRange = {
        from: prependOldRange.from + added,
        to: prependOldRange.to + added,
      };
      backtestChartData.allCharts.forEach((chart) => {
        syncVisibleLogicalRange(chart, nextRange, { animate: false });
      });
    }

    backtestHistoryHasMore = data.hasMore !== false && added > 0;

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
  return API.fetchLiveHistory({
    tf: currentTf,
    endTimeSec,
    limit: LIVE_STATE_CANDLE_LIMIT,
    rsxSettings: coerceRsxSettingsForAPI(getRsxSettingsState('live')),
  });
}

async function maybeLoadHistory(range, options = {}) {
  const force = options.force === true;
  if (isLoadingHistory || historyLoading || !historyHasMore) return Promise.resolve();
  if (!shouldPaintLiveChart()) return;
  if (!chartInitialized || !liveHistoryScrollArmed) return;
  if (!range || liveStore.candleCount() === 0) return;
  if (!force && range.from >= LIVE_HISTORY_SCROLL_THRESHOLD) return;

  const firstTime = liveStore.firstCandleTimeSec();
  if (firstTime == null) return;
  const endTimeSec = Number(firstTime);
  const epoch = liveHistoryEpoch;
  const reqId = currentLiveRequestId;
  const oldLen = liveStore.candleCount();

  isLoadingHistory = true;
  historyLoading = true;

  try {
    const data = await fetchLiveHistory(endTimeSec);
    if (epoch !== liveHistoryEpoch || reqId !== currentLiveRequestId) return;
    if (!Array.isArray(data.candles) || data.candles.length === 0) {
      historyHasMore = false;
      return;
    }

    liveStore.prependHistory({
      candles: toCandles(data.candles),
      oscillators: data.oscillators || [],
      annotations: data.annotations,
    });
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

    const added = liveStore.candleCount() - oldLen;
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

function handleLiveTick(d) {
  const tickTf = (d.timeframe || backendTradingTimeframe || currentTf || '1m').toLowerCase();
  if (tickTf !== currentTf.toLowerCase()) return;

  const time = chartTime(d.time);
  if (time == null) return;

  if (isLiveTf()) {
    updateHeader({ jurik: d.jurik, redLine: d.redLine, greenLine: d.greenLine });
    if (d.isClosed && d.volatilityRegime) {
      updateHeader({ volatilityRegime: d.volatilityRegime });
    }
  }

  if (!shouldPaintLiveChart()) return;
  if (!chartInitialized) return;

  const bar = normalizeCandle({
    time, open: d.open, high: d.high, low: d.low, close: d.close, volume: d.volume,
  });
  if (!bar) return;

  applyPriceBar(bar);
}

function handleLiveMarker(d) {
  if (!shouldPaintLiveChart()) return;
  mergeSessionTrades([{
    time: chartTime(d.time),
    side: d.side || d.action,
    price: d.price,
    kind: d.kind || 'entry',
  }]);
  applyTradeMarkers();
}

function initLiveWebSocket() {
  WS.connect({
    onTick: handleLiveTick,
    onMarker: handleLiveMarker,
    onOpen: () => {
      wsSubscribeTf(currentTf);
      suppressLivePollForWs();
    },
    onError: () => {
      resumeLivePollAfterWs();
    },
    onClose: () => {
      resumeLivePollAfterWs();
    },
    onReconnect: () => {
      wsSubscribeTf(currentTf);
    },
  });
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
  const candles = chartData === backtestChartData
    ? backtestStore.getForLightweightCharts().candles
    : liveStore.getForLightweightCharts().candles;
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
      initLiveWebSocket();
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
