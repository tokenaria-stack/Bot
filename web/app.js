

// ── UI controller delegates (Phase 19.5) ────────────────────────────────────
function getActiveTabId() { return TabsController.getActiveTabId(); }
function isBacktestTabActive() { return TabsController.isBacktestTabActive(); }
function isBacktestTfContext() { return TabsController.isBacktestTfContext(); }
function isLiveTabActive() { return TabsController.isLiveTabActive(); }
function getActiveStrategyContext() { return TabsController.getActiveStrategyContext(); }
function switchTab(targetId) { return TabsController.switchTab(targetId); }
function getActiveTf() { return TimeframeController.getActiveTf(); }
function getActiveTfFromToolbar() { return TimeframeController.getActiveTfFromToolbar(); }
function syncToolbarToActiveContext() { return TimeframeController.syncToolbar(); }
function switchTimeframe(tf, event) { return TimeframeController.switchTimeframe(tf, event); }
function hideWozduhSettingsMenus() { return WozduhController.hideMenus(); }
function hideRiskSettingsMenu() { return RiskController.hideMenu(); }


async function syncLiveNavigatorSettingsToServer() {
  return API.postNavigatorSettings(NavigatorController.getNavigatorPayload('live'));
}

async function triggerNavigatorAutoUpdate() {
  const context = NavigatorController.getContext();
  if (context === 'live') {
    const viewportSnapshot = ChartAdapter.captureLiveViewport();
    await syncLiveNavigatorSettingsToServer();
    if (ChartAdapter.chartInitialized() && liveStore.candleCount() > 0 && shouldPaintLiveChart()) {
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
      ChartAdapter.setNavigatorOverlay('live', { navigators: liveNavigatorResult }, liveStore.getForLightweightCharts().candles, {
        context: 'live',
        updateLoadedCandles: false,
      });
      ChartAdapter.restoreLiveViewportTwice(viewportSnapshot);
    } finally {
      endDataUpdate(0);
    }
  } catch (err) {
    if (err?.name === 'AbortError') return;
    console.error('[UI] refreshLiveNavigatorFromServer failed:', err);
  }
}

function getRiskSettingsFromUI() {
  return RiskController.getSettingsFromUI();
}

function getRSXSettingsFromUI(context = 'backtest') {
  return readRsxSettingsFromMenu(context);
}

function getWozduhSettingsFromUI(context = 'backtest') {
  return WozduhController.getSettingsFromUI(context);
}

function buildFinalBacktestPayload(overrides = {}) {
  if (!window.currentBacktestPayload) window.currentBacktestPayload = {};

  const prevSettings = (window.currentBacktestPayload.settings && typeof window.currentBacktestPayload.settings === 'object')
    ? window.currentBacktestPayload.settings
    : {};

  const matrix = StrategyController.getMatrixPayload('backtest');
  const navigators = NavigatorController.getNavigatorPayload('backtest');
  const risk = getRiskSettingsFromUI();

  const settings = {
    ...prevSettings,
    risk,
    matrix,
    navigators,
    ...StrategyController.getThresholdsPayload('backtest'),
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
    mtfOptions: NavigatorController.getMtfSyncSettings(),
  };

  window.currentBacktestPayload = finalPayload;
  currentBacktestPayload = finalPayload;
  return finalPayload;
}

function buildBacktestSettingsPayload() {
  try {
    const payload = buildFinalBacktestPayload();
    console.log('[Backtest] Settings payload from UI:', payload.settings);
    return payload.settings;
  } catch (err) {
    console.error('buildBacktestSettingsPayload failed:', err);
    const matrix = StrategyController.getMatrixPayload('backtest');
    try {
      return {
        matrix,
        navigators: NavigatorController.getNavigatorPayload('backtest'),
        risk: getRiskSettingsFromUI(),
        ...StrategyController.getThresholdsPayload('backtest'),
      };
    } catch {
      return {
        matrix,
        risk: getRiskSettingsFromUI(),
        navigators: NavigatorController.getNavigatorPayload('backtest'),
      };
    }
  }
}

function buildNavigatorBacktestPayload() {
  return buildBacktestSettingsPayload().navigators;
}

/** Lightweight Charts localization: axis labels in the user's local timezone (wire time stays UTC seconds). */



const RULER_IDLE = 0;
const RULER_MEASURING = 1;
const RULER_FIXED = 2;






let liveNavigatorResult = null;
let backtestTf = '15m';
let lastBacktestResult = null;
let statsMode = 'backtest';
let backtestAbortController = null;
let backtestRunActive = false;

let tradeMarkers = [];
let sessionTrades = [];
let spikeMarkers = [];
let lastFibZones = [];
let currentTf = '1m';
let backendTradingTimeframe = null;
let tradingTimeframeSynced = false;
let refreshTimer = null;
let historyHasMore = true;
let currentLiveRequestId = 0;
let navigatorRequestId = 0;
let isAppInitialized = false;
let livePollSuppressedByWs = false;
let lastTickBufferLen = 0;
let orderFlowPollTimer = null;
let isUpdatingData = false;
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





function canPatchBacktestIndicatorsOnly(options = {}) {
  if (options.patchIndicatorsOnly === false) return false;
  if (options.patchIndicatorsOnly === true) return true;
  return options.preserveView === true
    && backtestStore.candleCount() > 0
    && ChartAdapter.isInitialized('backtest');
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

function shouldPaintLiveChart() {
  return isLiveTabActive();
}

function getActiveChartData() {
  return ChartAdapter.getChartHandle(isBacktestTabActive() ? 'backtest' : 'live');
}

function getBacktestInterval() {
  const el = document.getElementById('bt-interval');
  return el?.value || backtestTf;
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

function applyTfToBacktestSelect(tf) {
  const el = document.getElementById('bt-interval');
  if (!el) return false;
  const hasOption = [...el.options].some((o) => o.value === tf);
  if (!hasOption) {
    const opt = document.createElement('option');
    opt.value = tf;
    opt.textContent = TF_DISPLAY[tf] || tf;
    el.appendChild(opt);
  }
  el.value = tf;
  backtestTf = tf;
  return true;
}

function rsxContextFromWrap(wrap) {
  return wrap?.id === 'bt-rsx-wrap' ? 'backtest' : 'live';
}

function getActiveUiContext() {
  return NavigatorController.getContext();
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
    ChartAdapter.applyRsxData('live', storeData.osc, storeData.annotations);
    endDataUpdate();
    ChartAdapter.forceSyncTimeScales('live');
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
  ChartAdapter.setLiveUpdating(true);
}

function endDataUpdate(delayMs = 50) {
  setTimeout(() => {
    ChartAdapter.setLiveUpdating(false);
    if (pendingHistoryLoad) {
      const job = pendingHistoryLoad;
      pendingHistoryLoad = null;
      scheduleHistoryLoad(job.range, job.options);
    }
  }, delayMs);
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

function initControls() {
  const toggles = {
    'tog-jurik': 'rsx',
    'tog-volume': 'volume',
  };

  Object.entries(toggles).forEach(([id, seriesKey]) => {
    const el = document.getElementById(id);
    if (!el) {
      console.warn(`[initControls] #${id} not found`);
      return;
    }
    const applyVisibility = () => {
      el.closest('.ind-toggle')?.classList.toggle('active', el.checked);
      ChartAdapter.setToggleSeriesVisible('live', seriesKey, el.checked);
    };
    applyVisibility();
    el.addEventListener('change', applyVisibility);
  });

  const togSpike = document.getElementById('tog-spike');
  if (togSpike) {
    togSpike.addEventListener('change', (e) => {
      e.target.closest('.ind-toggle')?.classList.toggle('active', e.target.checked);
      ChartAdapter.applyAllMarkers();
    });
  } else {
    console.warn('[initControls] #tog-spike not found');
  }

  const togFib = document.getElementById('tog-fib');
  if (togFib) {
    togFib.addEventListener('change', (e) => {
      e.target.closest('.ind-toggle')?.classList.toggle('active', e.target.checked);
      ChartAdapter.renderFib(lastFibZones);
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
        ChartAdapter.setChartType(btn.dataset.chart);
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
  NavigatorController.hideAllPopups();
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
    ChartAdapter.handleResize();
    try {
      window.dispatchEvent(new Event('resize'));
    } catch { /* noop */ }
  });
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
          const chartData = context === 'backtest' ? ChartAdapter.getChartHandle('backtest') : ChartAdapter.getChartHandle('live');
          const storeData = (chartData === ChartAdapter.getChartHandle('backtest') ? backtestStore : liveStore).getForLightweightCharts();
          const osc = storeData.osc;
          const anns = storeData.annotations;
          if (chartData?.rsxSeries && osc?.length) {
            ChartAdapter.applyRsxData(chartData === ChartAdapter.getChartHandle('backtest') ? 'backtest' : 'live', osc, anns);
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


function resetBacktestClientCacheForTfChange() {
  backtestStore.clear();
  backtestNavigatorChartLines = [];
  backtestNavigatorChartMarkers = [];
  backtestLastTrades = [];
  backtestHistoryHasMore = true;
  backtestHistoryLoading = false;

  ChartAdapter.clearSeries('backtest');
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

  const anchor = ChartAdapter.isInitialized('backtest')
    ? ChartAdapter.captureViewport('backtest')
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
  isLoadingHistory = false;
  ChartAdapter.setChartInitialized(false);
  if (!ChartAdapter.isInitialized('live')) {
    endDataUpdate();
    return;
  }
  ChartAdapter.clearSeries('live');
  ChartAdapter.clearFib();
  resetRuler();
  updateBufferingOverlay();
  endDataUpdate();
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
  ChartAdapter.applyDelta('live', delta);
}

function fmt(v) {
  return typeof v === 'number' && Number.isFinite(v) ? v.toFixed(2) : '—';
}

function fmtPrice(v) {
  return typeof v === 'number' && Number.isFinite(v)
    ? v.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })
    : '—';
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
  ChartAdapter.setTradeMarkers(tradeMarkers);
}




function deriveBotStatus(data) {
  if (data.masterState) return data.masterState;
  const trades = data.trades || sessionTrades || [];
  if (trades.length === 0) return 'IDLE';
  const sorted = [...trades].sort((a, b) => a.time - b.time);
  const last = sorted[sorted.length - 1];
  return last.kind === 'exit' ? 'IDLE' : 'IN_POSITION';
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
    ChartAdapter.setEquityData([]);
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

  if (Array.isArray(data.equityCurve)) {
    ChartAdapter.setEquityData(
      data.equityCurve.map((p) => ({ time: p.time, value: p.value })),
    );
    ChartAdapter.fitEquityContent();
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

  const anchor = options.viewportAnchor ?? ChartAdapter.captureViewport('backtest');

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

    ChartAdapter.applyBacktestResult(result, {
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
      TF_DISPLAY[state.timeframe || state.tradingTimeframe || currentTf]
        || state.timeframe || state.tradingTimeframe || currentTf,
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




function applySeriesData(options = {}) {
  const {
    isPrepend = false,
    prependPrevRange = null,
    preserveViewport = null,
  } = options;
  let prevRange = preserveViewport?.logicalRange ?? null;
  if (!prevRange && !isPrepend && shouldPaintLiveChart()) {
    prevRange = ChartAdapter.getVisibleLogicalRange('live');
  }

  const prependOldTotal = prependPrevRange?.oldTotal;
  let prependOldRange = null;
  if (isPrepend) {
    prependOldRange = ChartAdapter.getVisibleLogicalRange('live');
  }

  const storeData = liveStore.getForLightweightCharts();
  const candles = storeData.candles;

  if (!shouldPaintLiveChart()) return;

  ChartAdapter.applyFullData('live', storeData);
  ChartAdapter.setSpikeMarkers(buildSpikeMarkers(storeData.osc));
  ChartAdapter.applyAllMarkers();
  if (liveNavigatorResult) {
    ChartAdapter.setNavigatorOverlay('live', { navigators: liveNavigatorResult }, candles, {
      context: 'live',
      updateLoadedCandles: false,
    });
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
        const liveHandle = ChartAdapter.getChartHandle('live');
        liveHandle.allCharts.forEach((chart) => {
          ChartAdapter.syncVisibleLogicalRange(chart, nextRange, { animate: false });
        });
      });
    }
    return;
  }

  ChartAdapter.applyLiveViewportAfterData(candles, {
    prevRange,
    isPrepend,
  });

  if (preserveViewport?.logicalRange) {
    ChartAdapter.restoreLiveViewportTwice(preserveViewport);
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
  ChartAdapter.applyDelta('live', delta);
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
    ChartAdapter.renderFib(data.fibZones);
    if (liveStore.candleCount() > 0 && shouldPaintLiveChart()) {
      if (!options.preserveViewport?.logicalRange && !options.isPrepend) {
        ChartAdapter.syncLivePanesFromPrice();
      }
    }
  } finally {
    endDataUpdate(0);
  }

  // График полностью загружен историей, теперь WebSocket может рисовать новые тики
  ChartAdapter.setChartInitialized(true);
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
      ChartAdapter.syncLivePanesFromPrice();
    } finally {
      endDataUpdate(0);
    }
    ChartAdapter.setChartInitialized(true);
  } catch (err) {
    console.error('pollOrderFlowState:', err);
  }
}

async function pollLatestState() {
  if (!shouldPaintLiveChart()) return;
  if (!ChartAdapter.chartInitialized()) return;
  if (isOrderFlowTf()) return;
  if (ChartAdapter.isLiveUpdating()) return;
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
    ChartAdapter.applyDelta('live', delta);
    endDataUpdate();
  } catch (err) {
    ChartAdapter.setLiveUpdating(false);
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
      ChartAdapter.setChartInitialized(false);
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
          ChartAdapter.setChartInitialized(true);
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
      ChartAdapter.setChartInitialized(true);
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
  if (range.from >= LIVE_HISTORY_SCROLL_THRESHOLD) return;

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

    const prependOldRange = ChartAdapter.getVisibleLogicalRange('backtest') ?? null;
    const { added } = backtestStore.prependHistory(
      chartPointsToStorePayload(data.chartData, data.annotations),
    );
    backtestHistoryHasMore = data.hasMore !== false;

    if (added === 0) {
      backtestHistoryHasMore = false;
      console.warn('History pagination stalled: Zero overlap.');
      return;
    }

    const storeData = backtestStore.getForLightweightCharts();

    ChartAdapter.applyFullData('backtest', storeData, {
      isPrepend: true,
      addedCount: added,
      prependOldRange,
    });
    ChartAdapter.applyWozduhVisibility('backtest');
    ChartAdapter.applyBacktestMarkers(backtestLastTrades, storeData.osc);

    backtestHistoryHasMore = data.hasMore !== false && added > 0;

    if (added > 0 && backtestHistoryHasMore) {
      requestAnimationFrame(() => {
        const nextRange = ChartAdapter.getVisibleLogicalRange('backtest');
        if (nextRange && nextRange.from < LIVE_HISTORY_SCROLL_THRESHOLD) {
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
  if (isLoadingHistory || !historyHasMore) return Promise.resolve();
  if (!shouldPaintLiveChart()) return;
  if (!ChartAdapter.chartInitialized() || !liveHistoryScrollArmed) return;
  if (!range || liveStore.candleCount() === 0) return;
  if (!force && range.from >= LIVE_HISTORY_SCROLL_THRESHOLD) return;

  const firstTime = liveStore.firstCandleTimeSec();
  if (firstTime == null) return;
  const endTimeSec = Number(firstTime);
  const epoch = liveHistoryEpoch;
  const reqId = currentLiveRequestId;
  const prependOldRange = ChartAdapter.getVisibleLogicalRange('live');

  isLoadingHistory = true;

  try {
    const data = await fetchLiveHistory(endTimeSec);
    if (epoch !== liveHistoryEpoch || reqId !== currentLiveRequestId) return;
    if (!Array.isArray(data.candles) || data.candles.length === 0) {
      historyHasMore = false;
      return;
    }

    const { added } = liveStore.prependHistory({
      candles: toCandles(data.candles),
      oscillators: data.oscillators || [],
      annotations: data.annotations,
    });
    historyHasMore = data.hasMore !== false;

    if (added === 0) {
      historyHasMore = false;
      console.warn('History pagination stalled: Zero overlap.');
      return;
    }

    beginDataUpdate();
    try {
      const storeData = liveStore.getForLightweightCharts();
      ChartAdapter.applyFullData('live', storeData, {
        isPrepend: true,
        addedCount: added,
        prependOldRange,
      });
      ChartAdapter.setSpikeMarkers(buildSpikeMarkers(storeData.osc));
      ChartAdapter.applyAllMarkers();
      if (liveNavigatorResult) {
        ChartAdapter.setNavigatorOverlay('live', { navigators: liveNavigatorResult }, storeData.candles, {
          context: 'live',
          updateLoadedCandles: false,
        });
      }
      updateBufferingOverlay();
    } finally {
      endDataUpdate(0);
    }

    if (!historyHasMore || epoch !== liveHistoryEpoch) return;

    if (added > 0) {
      requestAnimationFrame(() => {
        if (epoch !== liveHistoryEpoch || !liveHistoryScrollArmed) return;
        const nextRange = ChartAdapter.getVisibleLogicalRange('live');
        if (nextRange && nextRange.from < LIVE_HISTORY_SCROLL_THRESHOLD) {
          scheduleHistoryLoad(nextRange);
        }
      });
    }
  } catch (err) {
    console.error('lazy history:', err);
  } finally {
    isLoadingHistory = false;
  }
}

function scheduleHistoryLoad(range, options = {}) {
  if (!range || !historyHasMore) return;
  if (!ChartAdapter.chartInitialized() || !liveHistoryScrollArmed) return;
  if (range.from >= LIVE_HISTORY_SCROLL_THRESHOLD) return;
  if (ChartAdapter.isLiveUpdating()) {
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
  if (!ChartAdapter.chartInitialized()) return;

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


function getRulerChartData() {
  return ruler.chartData || ChartAdapter.getChartHandle(isBacktestTabActive() ? 'backtest' : 'live');
}


function initRuler() {
  ChartAdapter.attachRuler(ChartAdapter.getChartHandle('live'));
  ChartAdapter.attachRuler(ChartAdapter.getChartHandle('backtest'));
}

function setRulerCursor(active) {
  [ChartAdapter.getChartHandle('live'), ChartAdapter.getChartHandle('backtest')].forEach((chartData) => {
    const wrap = chartData?.elements?.priceWrap;
    if (!wrap) return;
    const isActiveChart = chartData === ChartAdapter.getChartHandle(isBacktestTabActive() ? 'backtest' : 'live');
    wrap.style.cursor = active && isActiveChart ? 'crosshair' : '';
  });
}

function toggleRuler() {
  ruler.active = !ruler.active;
  ruler.chartData = ChartAdapter.getChartHandle(isBacktestTabActive() ? 'backtest' : 'live');
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

  const point = ChartAdapter.rulerPointFromParam(chartData, param);
  if (!point) return;

  if (ruler.state === RULER_IDLE) {
    ruler.p1 = point;
    ruler.p2 = point;
    ruler.state = RULER_MEASURING;
    return;
  }

  if (ruler.state === RULER_MEASURING) {
    ruler.p2 = point;
    ruler.state = RULER_FIXED;
    ChartAdapter.updateRulerOverlay(chartData);
  }
}

function onRulerMove(param, chartData) {
  if (!ruler.active || chartData !== getRulerChartData()) return;
  if (chartData === ChartAdapter.getChartHandle('backtest') ? isUpdatingData : ChartAdapter.isLiveUpdating()) return;
  if (ruler.state !== RULER_MEASURING || !param.point) return;

  const point = ChartAdapter.rulerPointFromParam(chartData, param);
  if (!point) return;

  ruler.p2 = point;
  ChartAdapter.updateRulerOverlay(chartData);
}

function countBarsBetween(t1, t2, chartData) {
  const lo = Math.min(t1, t2);
  const hi = Math.max(t1, t2);
  const candles = chartData === ChartAdapter.getChartHandle('backtest')
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


function safeInit(moduleName, fn) {
  try {
    fn();
  } catch (err) {
    console.error(`Failed to init ${moduleName}:`, err);
  }
}

function boot() {
  safeInit('UI strategy', () => StrategyController.init());
  safeInit('UI tabs', () => TabsController.init());
  safeInit('UI risk', () => RiskController.init());
  safeInit('UI navigator', () => {
    NavigatorController.init();
    NavigatorController.onSettingsChanged(() => triggerNavigatorAutoUpdate());
  });
  safeInit('equity chart', initEquityChart);
  safeInit('stats mode selector', initStatsModeSelector);
  safeInit('backtest', initBacktest);

  (async () => {
    try {
      if (typeof LightweightCharts === 'undefined' || typeof ChartAdapter === 'undefined') {
        setTimeout(boot, 500);
        return;
      }

      let chartsReady = false;
      try {
        chartsReady = ChartAdapter.initLiveCharts(LIVE_CHART_SELECTORS, {
          shouldPaint: shouldPaintLiveChart,
          onAfterDelta: updateBufferingOverlay,
          crosshairPriceSeries: () => {
            const t = ChartAdapter.getChartType();
            const h = ChartAdapter.getChartHandle('live');
            if (t === 'bars') return h.barSeries;
            if (t === 'line') return h.lineSeries;
            return h.candleSeries;
          },
          onVisibleRangeChange: (range) => {
            if (getRulerChartData() === ChartAdapter.getChartHandle('live')) {
              ChartAdapter.updateRulerOverlay(ChartAdapter.getChartHandle('live'));
            }
          },
          onScrollHistory: (range) => scheduleHistoryLoad(range),
          onLiveInit: () => {
            attachLiveHistoryScrollArm();
          },
          applyTradeMarkers: () => ChartAdapter.applyAllMarkers(),
        });
        if (chartsReady) {
          ChartAdapter.initBacktestCharts(BACKTEST_CHART_SELECTORS, {
            crosshairPriceSeries: () => ChartAdapter.getChartHandle('backtest').candleSeries,
            onVisibleRangeChange: (range) => {
              if (getRulerChartData() === ChartAdapter.getChartHandle('backtest')) {
                ChartAdapter.updateRulerOverlay(ChartAdapter.getChartHandle('backtest'));
              }
            },
            onScrollHistory: (range) => maybeLoadBacktestHistory(range),
            onBacktestInit: () => {
              ChartAdapter.attachRuler(ChartAdapter.getChartHandle('backtest'));
              NavigatorController.renderChartLegends('backtest');
            },
          });
          initRsxSettings();
          WozduhController.init();
          NavigatorController.initLegends();
          ChartAdapter.initRuler();
          initPaneResize();
          loadPaneHeights();
        }
      } catch (err) {
        console.error('Failed to init charts:', err);
      }
      if (!chartsReady) {
        setTimeout(boot, 500);
        return;
      }

      safeInit('UI timeframe', () => TimeframeController.init({ useServerTf: true }));
      safeInit('toolbar/controls', () => initControls());
      syncToolbarToActiveContext();

      if (isOrderFlowTf()) {
        ChartAdapter.applyOrderFlowTimeScale(true);
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

      requestAnimationFrame(() => ChartAdapter.handleResize());
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
