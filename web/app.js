

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
    const viewportAnchor = ViewportManager.capture('live');
    await syncLiveNavigatorSettingsToServer();
    if (ChartAdapter.chartInitialized() && liveStore.candleCount() > 0 && shouldPaintLiveChart()) {
      await refreshLiveNavigatorFromServer(viewportAnchor);
    } else {
      await loadDashboard({ viewportAnchor });
    }
    return;
  }
  buildFinalBacktestPayload();
}

async function refreshLiveNavigatorFromServer(viewportAnchor) {
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
      ViewportManager.restore('live', viewportAnchor, liveStore);
    } finally {
      endDataUpdate();
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
  return RsxController.getSettings(context);
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

  const form = BacktestController.getFormValues();
  const finalPayload = {
    symbol: overrides.symbol
      ?? form.symbol
      ?? window.currentBacktestPayload.symbol
      ?? 'BTCUSDT',
    interval: overrides.interval
      ?? form.interval
      ?? backtestTf
      ?? getActiveTfFromToolbar()
      ?? window.currentBacktestPayload.interval
      ?? '15m',
    startDate: overrides.startDate
      ?? form.start
      ?? window.currentBacktestPayload.startDate
      ?? '',
    endDate: overrides.endDate
      ?? form.end
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
let _microscopeTickMuted = false;
let liveHistoryEpoch = 0;
let pendingHistoryLoad = null;
/** History lazy-load is armed only after the user scrolls/zooms the live chart (not on TF init). */
let liveHistoryScrollArmed = false;

if (typeof window !== 'undefined') {
  window.__isSettingsUpdating = false;
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
  return BacktestController.getFormValues().interval || backtestTf;
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

function getActiveUiContext() {
  return NavigatorController.getContext();
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
  const localBeforeFetch = { ...RsxController.getSettings('live') };
  try {
    const serverSettings = await API.fetchRsxSettings();
    if (version !== rsxSettingsFetchVersion) return;
    const merged = normalizeRsxSettingsFromAPI(
      { ...serverSettings, ...localBeforeFetch },
      defaultRsxSettings(),
    );
    const applied = RsxController.setSettings('live', merged);
    RsxController.persist('live', applied);
    RsxController.applyToMenu('live', applied);
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
      }, getLiveStoreTf());
    } else if (data?.oscillators?.length) {
      liveStore.replaceOscAndAnnotations({ oscillators: data.oscillators }, getLiveStoreTf());
    }

    beginDataUpdate();
    const storeData = liveStore.getForLightweightCharts();
    ChartAdapter.applyRsxData('live', storeData.osc, storeData.annotations);
    endDataUpdate();
  } catch (err) {
    console.warn('Failed to reload RSX chart:', err);
  }
}

function appendBacktestRsxSettingsToParams(params) {
  const settings = coerceRsxSettingsForAPI(RsxController.getSettings('backtest'));
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

async function syncRsxIndicatorSettings(context = 'live') {
  const applied = RsxController.syncFromMenu(context);

  if (context === 'backtest') {
    return applied;
  }

  rsxSettingsFetchVersion += 1;
  const serverApplied = await pushRsxSettingsToServer(applied);
  if (serverApplied) {
    const liveApplied = RsxController.setSettings('live', normalizeRsxSettingsFromAPI(
      { ...serverApplied, show_pivots: applied.show_pivots },
      applied,
    ));
    RsxController.persist('live', liveApplied);
    RsxController.applyToMenu('live', liveApplied);
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

function getIntervalMs(tf) {
  return TimeNormalizer.getIntervalMs(tf);
}

function getLiveStoreTf() {
  return TimeframeController.getActiveTf();
}

function getBacktestStoreTf() {
  return BacktestController.getFormValues().interval || backtestTf || '15m';
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
    rsxLookback: RsxController.getSettings('live').div_lookback,
    limit: LIVE_STATE_CANDLE_LIMIT,
    extra,
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

function disarmLiveHistoryScroll() {
  liveHistoryScrollArmed = false;
  pendingHistoryLoad = null;
}

function updateBufferingOverlay() {
  ToolbarController.setBuffering(isOrderFlowTf() && (
    liveStore.candleCount() < 5 ||
    lastTickBufferLen < 500
  ));
}

function syncLiveRsxToolbarFromOsc(pt) {
  if (!pt) return;
  ToolbarController.updateOscHeader(pt);
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

function endDataUpdate() {
  ChartAdapter.setLiveUpdating(false);
  if (pendingHistoryLoad) {
    const job = pendingHistoryLoad;
    pendingHistoryLoad = null;
    Promise.resolve().then(() => scheduleHistoryLoad(job.range, job.options));
  }
}











































function hidePanelSettingsMenus() {
  RsxController.hideMenus();
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

  backtestTf = BacktestController.setInterval(tf) || tf;
  syncToolbarToActiveContext();
  BacktestController.applyDateRangeLimits(tf);

  const anchor = ChartAdapter.isInitialized('backtest')
    ? ViewportManager.capture('backtest')
    : null;

  resetBacktestClientCacheForTfChange();
  RsxController.syncFromMenu('backtest');

  if (window.currentBacktestPayload) {
    window.currentBacktestPayload.interval = tf;
  }
  buildFinalBacktestPayload({ interval: tf });

  try {
    BacktestController.setLoading(true);
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
    BacktestController.setLoading(false);
  }
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

function wsSubscribeTf(tf) {
  WS.subscribe(tf, resolveTf(tf) || currentTf);
}

function clearChartData() {
  _microscopeTickMuted = false;
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
  ChartAdapter.destroyLiveCharts();
  resetRuler();
  updateBufferingOverlay();
  endDataUpdate();
}



/** RSX/Wozduh warmup sentinel — 0 is never a valid live oscillator reading. */



function applyPriceBar(bar) {
  if (!bar) return;
  const last = liveStore.lastCandleChartSec();
  if (last && last.time === bar.time) {
    liveStore.upsertCandle(bar, getLiveStoreTf());
  } else if (!last || bar.time >= last.time) {
    if (last && isLiveTickGapTooLarge(last.time, bar.time)) {
      if (!_microscopeTickMuted) {
        console.log(`[Mode Switch] Time gap too large (${bar.time} vs ${last.time}). Entering historical microscope mode. Live WS ticks will be silently ignored until return to live edge.`);
        _microscopeTickMuted = true;
      }
      return;
    }
    liveStore.upsertCandle(bar, getLiveStoreTf());
  } else {
    return;
  }

  if (!shouldPaintLiveChart()) return;

  const delta = liveStore.getLatestDeltaForChart();
  if (delta && !liveStore.isSealed()) {
    ChartAdapter.applyDelta('live', delta);
  }
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
  backtestStore._ingestAnnotations(tradesToStoreAnnotations(trades), getBacktestStoreTf());
  return buildTradeMarkerPrimitiveData(trades);
}


function refreshStatsForMode(mode) {
  return BacktestController.refreshStatsForMode(mode);
}

function renderBacktestStats(result) {
  BacktestController.storeBacktestResult(result);
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
  backtestRunActive = false;
  BacktestController.setRunActive(false);
  backtestAbortController = null;
  BacktestController.setLoading(false);
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

  const anchor = options.viewportAnchor ?? ViewportManager.capture('backtest');

  const form = BacktestController.getFormValues();
  const symbol = form.symbol;
  const interval = normalizeTf(form.interval || backtestTf || getActiveTfFromToolbar() || '15m');
  BacktestController.applyDateRangeLimits(interval);
  const startDate = form.start;
  const endDate = form.end;
  const isSettingsRefresh = patchIndicatorsOnly && backtestStore.candleCount() > 0;

  if (manageLoading) {
    BacktestController.setLoading(true);
  }

  if (!isSettingsRefresh) {
    backtestRunActive = true;
    BacktestController.setRunActive(true);
  }
  backtestAbortController = new AbortController();

  try {
    let payload = buildFinalBacktestPayload({ symbol, interval, startDate, endDate });
    const settingsPayload = payload.settings;

    if (!settingsPayload?.matrix || !settingsPayload?.navigators) {
      throw new Error('Backtest payload missing matrix or navigators in settings');
    }

    if (!skipSettingsPush) {
      await pushRsxSettingsToServer(coerceRsxSettingsForAPI(RsxController.getSettings('backtest')));
    }

    let result;
    for (let attempt = 0; attempt < 2; attempt++) {
      const { ok, status, result: respResult, rawText } = await API.runBacktest(
        payload,
        backtestAbortController?.signal,
      );
      result = respResult;

      const errText = result?.error || result?.message || rawText || '';
      const notEnoughCandles = status === 400 && /not enough candles/i.test(errText);

      if (attempt === 0 && notEnoughCandles) {
        const expanded = BacktestController.expandBacktestStartDate(payload.startDate, payload.endDate, 90);
        if (expanded && expanded !== payload.startDate) {
          console.warn(`[Backtest] Auto-expanding startDate ${payload.startDate} → ${expanded} (retry after: ${errText})`);
          payload = buildFinalBacktestPayload({
            symbol: payload.symbol,
            interval: payload.interval,
            startDate: expanded,
            endDate: payload.endDate,
          });
          BacktestController.setFormValues({ start: expanded });
          continue;
        }
      }

      if (result?._parseError) {
        console.error('[FALCON NETWORK] Server returned invalid JSON. Raw response:', rawText);
        throw new Error(`Server response is not valid JSON. Check console for raw text. Status: ${status}`);
      }

      if (ok) {
        break;
      }

      const errorText = errText || `HTTP error ${status}`;
      alert(`Ошибка Бэктеста:\n${errorText}`);
      return;
    }

    ChartAdapter.applyBacktestResult(result, {
      patchIndicatorsOnly,
    }, anchor);
    renderBacktestStats(result);
    if (typeof NavigatorController !== 'undefined') {
      NavigatorController.renderChartLegends('backtest');
    }
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

function applySeriesData(options = {}) {
  const storeData = liveStore.getForLightweightCharts();
  const candles = storeData.candles;

  if (!shouldPaintLiveChart()) return;

  ChartAdapter.applyFullData('live', storeData);
  if (liveNavigatorResult) {
    ChartAdapter.setNavigatorOverlay('live', { navigators: liveNavigatorResult }, candles, {
      context: 'live',
      updateLoadedCandles: false,
    });
  }
  updateBufferingOverlay();
}

function applyLatestOscPoint(pt) {
  if (!pt) return;
  const latestCandleTime = liveStore.lastCandleTimeSec();
  if (latestCandleTime != null && pt.time !== latestCandleTime) {
    pt = { ...pt, time: latestCandleTime };
  }
  liveStore.upsertOscPoint(pt, getLiveStoreTf());
  syncLiveRsxToolbarFromOsc(pt);
  const delta = liveStore.getLatestDeltaForChart();
  if (delta && !liveStore.isSealed()) {
    ChartAdapter.applyDelta('live', delta);
  }
}

function renderState(data, options = {}) {
  syncTradingTimeframeFromState(data);
  ToolbarController.updateHeaderData(data);
  if (typeof data.tickBufferLen === 'number') {
    lastTickBufferLen = data.tickBufferLen;
  }

  const manageSeal = !options.storeReady;
  if (manageSeal) liveStore.seal();
  beginDataUpdate();
  try {
    if (!options.storeReady) {
      liveStore.replaceFromServer({
        candles: toCandles(data.candles),
        oscillators: data.oscillators || [],
        annotations: data.annotations,
      }, getLiveStoreTf());
    }
    const oscPts = data.oscillators || [];
    if (oscPts.length) syncLiveRsxToolbarFromOsc(oscPts[oscPts.length - 1]);
    const storeData = liveStore.getForLightweightCharts();
    const candles = storeData.candles;
    const masterState = data.masterState || deriveBotStatus({ ...data, trades: data.trades });
    mergeSessionTrades(data.trades, {
      masterState,
      lastCandleTime: candles.length ? candles[candles.length - 1].time : null,
    });

    const hasMore = typeof data.hasMore === 'boolean' ? data.hasMore : candles.length > 0;

    historyHasMore = hasMore;
    if (data.navigators) {
      liveNavigatorResult = data.navigators;
    }

    applySeriesData();
    ChartAdapter.renderFib(data.fibZones);
    if (options.viewportAnchor) {
      window.ViewportManager.restore('live', options.viewportAnchor, liveStore);
    } else if (options.isPreFetch) {
      const charts = [
        ChartAdapter.getChart('live', 'price'),
        ChartAdapter.getChart('live', 'osc'),
        ChartAdapter.getChart('live', 'rsx'),
      ];
      charts.forEach((chart) => {
        chart?.timeScale()?.scrollToPosition(0, false);
      });
    }
  } finally {
    if (manageSeal) liveStore.unseal();
    endDataUpdate();
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
    }, getLiveStoreTf());
    beginDataUpdate();
    try {
      applySeriesData();
    } finally {
      endDataUpdate();
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
      rsxLookback: RsxController.getSettings('live').div_lookback,
    });
    if (!shouldRunLivePoll()) return;
    if (warmingUp || !data.candles?.length) return;

    ToolbarController.updateHeaderData({ jurik: data.jurik, redLine: data.redLine, greenLine: data.greenLine });

    const candles = toCandles(data.candles);
    const latest = candles[candles.length - 1];
    if (!latest) return;

    const last = liveStore.lastCandleChartSec();
    if (last && last.time === latest.time) {
      liveStore.upsertCandle(latest, getLiveStoreTf());
    } else if (!last || latest.time > last.time) {
      if (last && isLiveTickGapTooLarge(last.time, latest.time)) {
        if (!_microscopeTickMuted) {
          console.log(`[Mode Switch] Time gap too large. Entering historical microscope mode. Live poll ticks will be silently ignored.`);
          _microscopeTickMuted = true;
        }
        return;
      }
      liveStore.upsertCandle(latest, getLiveStoreTf());
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
      liveStore.upsertOscPoint(pt, getLiveStoreTf());
      syncLiveRsxToolbarFromOsc(pt);
    }

    const delta = liveStore.getLatestDeltaForChart();
    if (delta && !liveStore.isSealed()) {
      ChartAdapter.applyDelta('live', delta);
    }
    endDataUpdate();
  } catch (err) {
    ChartAdapter.setLiveUpdating(false);
    console.error('pollLatestState:', err);
  }
}

let liveNavigatorSettingsSynced = false;

async function loadDashboard(options = {}) {
  const reqId = ++currentLiveRequestId;
  if (!options.navigatorsOnly && shouldPaintLiveChart() && !ChartAdapter.isInitialized('live')) {
    const chartsReady = ChartAdapter.initLiveCharts();
    if (!chartsReady) {
      setTimeout(() => loadDashboard(options), 500);
      return;
    }
  }
  if (!liveNavigatorSettingsSynced && shouldPaintLiveChart()) {
    liveNavigatorSettingsSynced = true;
    try {
      await syncLiveNavigatorSettingsToServer();
    } catch (err) {
      console.warn('live navigator settings sync:', err);
    }
  }

  const handleLoadError = (err) => {
    if (reqId !== currentLiveRequestId) return;
    const isTimeout = err?.name === 'TimeoutError';
    if (err?.name === 'AbortError' && !isTimeout) return;
    console.error('Dashboard load failed:', err);
    if (isOrderFlowTf(currentTf)) {
      clearChartData();
      ChartAdapter.setChartInitialized(true);
      updateBufferingOverlay();
      return;
    }
    const retryMs = isTimeout ? 1500 : 3000;
    setTimeout(() => loadDashboard(options), retryMs);
  };

  if (options.navigatorsOnly) {
    try {
      const { warmingUp, data } = await fetchLiveState({
        userTfChange: options.userTfChange === true,
        navigatorsOnly: true,
      });
      if (reqId !== currentLiveRequestId) return;
      if (warmingUp) {
        setTimeout(() => loadDashboard(options), 2000);
        return;
      }
      renderState(data, options);
    } catch (err) {
      handleLoadError(err);
    }
    return;
  }

  window.__isDashboardLoading = true;
  ToolbarController.setBuffering(true);
  try {
    const TARGET_BARS = 3000;
    const tf = getLiveStoreTf();
    const anchor = options.viewportAnchor;
    const isMicroscope = anchor && !anchor.isAtRightEdge && anchor.centerTimeMs;

    let stateData = null;
    let historyData = null;

    if (isMicroscope) {
      const intervalMs = getIntervalMs(tf);
      const halfWindowMs = (TARGET_BARS / 2) * intervalMs;
      const targetEndTimeSec = Math.floor((anchor.centerTimeMs + halfWindowMs) / 1000);

      try {
        historyData = await fetchLiveHistory(targetEndTimeSec, TARGET_BARS);
      } catch (histErr) {
        console.warn('Dashboard microscope history:', histErr);
      }
      if (reqId !== currentLiveRequestId) return;

      liveStore.seal();
      liveStore.replaceFromServer({
        candles: toCandles(historyData?.candles || []),
        oscillators: historyData?.oscillators || [],
        annotations: historyData?.annotations,
      }, tf);

      stateData = {
        candles: historyData?.candles || [],
        oscillators: historyData?.oscillators || [],
        annotations: historyData?.annotations,
        hasMore: historyData?.hasMore,
        status: 'ready',
      };
    } else {
      const { warmingUp, data } = await fetchLiveState({
        userTfChange: options.userTfChange === true,
      });
      if (reqId !== currentLiveRequestId) return;
      stateData = data;

      if (_microscopeTickMuted && !warmingUp && stateData.candles?.length) {
        console.log(`[Mode Switch] Returned to Live edge. Accepting WS ticks again.`);
        _microscopeTickMuted = false;
      }

      if (syncTradingTimeframeFromState(stateData) && !options._tfSyncRetried) {
        clearChartData();
        ChartAdapter.setChartInitialized(false);
        return loadDashboard({ ...options, _tfSyncRetried: true });
      }
      if (reqId !== currentLiveRequestId) return;
      if (warmingUp) {
        setTimeout(() => loadDashboard(options), 2000);
        return;
      }
      if (!stateData.candles || stateData.candles.length === 0) {
        if (stateData.status === 'ready') {
          if (reqId !== currentLiveRequestId) return;
          await renderState(
            { ...stateData, candles: stateData.candles || [], oscillators: stateData.oscillators || [] },
            options,
          );
          if (isOrderFlowTf(currentTf)) {
            ChartAdapter.setChartInitialized(true);
            updateBufferingOverlay();
          }
          return;
        }
        setTimeout(() => loadDashboard(options), 2000);
        return;
      }

      liveStore.seal();
      liveStore.replaceFromServer({
        candles: toCandles(stateData.candles),
        oscillators: stateData.oscillators || [],
        annotations: stateData.annotations,
      }, tf);
    }

    if (reqId !== currentLiveRequestId) return;
    await renderState(stateData, { ...options, isPreFetch: !isMicroscope, storeReady: true });
  } catch (err) {
    handleLoadError(err);
  } finally {
    liveStore.unseal();
    ToolbarController.setBuffering(false);
    window.__isDashboardLoading = false;
  }
}

async function maybeLoadBacktestHistory(range) {
  if (window.__isSettingsUpdating) return;
  if (!isBacktestTabActive()) return;
  if (!range || backtestHistoryLoading || !backtestHistoryHasMore || backtestStore.candleCount() === 0) return;
  if (range.from >= LIVE_HISTORY_SCROLL_THRESHOLD) return;

  const firstTime = backtestStore.firstCandleTimeSec();
  if (firstTime == null) return;

  const { symbol, interval } = BacktestController.getFormValues();
  const endTimeMs = firstTime < 1e12 ? firstTime * 1000 : firstTime;

  backtestHistoryLoading = true;
  let added = 0;
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

    backtestStore.seal();
    beginDataUpdate();
    try {
      ({ added } = backtestStore.prependHistory(
        chartPointsToStorePayload(data.chartData, data.annotations),
        getBacktestStoreTf(),
      ));
      backtestHistoryHasMore = data.hasMore !== false;

      if (added === 0) {
        backtestHistoryHasMore = false;
        console.warn('History pagination stalled: Zero overlap.');
        return;
      }

      const storeData = backtestStore.getForLightweightCharts();

      ChartAdapter.applyHistoryPrepend('backtest', storeData, added);
      ChartAdapter.applyWozduhVisibility('backtest');
      ChartAdapter.applyBacktestMarkers(backtestLastTrades, storeData.osc);

      backtestHistoryHasMore = data.hasMore !== false && added > 0;
    } finally {
      backtestStore.unseal();
      endDataUpdate();
    }

    if (backtestHistoryHasMore && added > 0) {
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

async function fetchLiveHistory(endTimeSec, limitOverride = null) {
  return API.fetchLiveHistory({
    tf: currentTf,
    endTimeSec,
    limit: limitOverride || HISTORY_CHUNK_LIMIT,
    rsxSettings: coerceRsxSettingsForAPI(RsxController.getSettings('live')),
  });
}

async function maybeLoadHistory(range, options = {}) {
  if (!ChartAdapter.isInitialized('live') || liveStore.isSealed() || window.__isDashboardLoading) return;
  const currentEpoch = liveHistoryEpoch;
  const force = options.force === true;
  if (isLoadingHistory || !historyHasMore) return Promise.resolve();
  if (!shouldPaintLiveChart()) return;
  if (!ChartAdapter.chartInitialized() || !liveHistoryScrollArmed) return;
  if (!range || liveStore.candleCount() === 0) return;
  if (!force && range.from >= LIVE_HISTORY_SCROLL_THRESHOLD) return;

  const firstTime = liveStore.firstCandleTimeSec();
  if (firstTime == null) return;
  const endTimeSec = Number(firstTime);
  const reqId = currentLiveRequestId;

  isLoadingHistory = true;
  let added = 0;

  try {
    const data = await fetchLiveHistory(endTimeSec);
    if (currentEpoch !== liveHistoryEpoch || !ChartAdapter.isInitialized('live')) return;
    if (reqId !== currentLiveRequestId) return;
    if (!Array.isArray(data.candles) || data.candles.length === 0) {
      historyHasMore = false;
      return;
    }

    liveStore.seal();
    beginDataUpdate();
    try {
      ({ added } = liveStore.prependHistory({
        candles: toCandles(data.candles),
        oscillators: data.oscillators || [],
        annotations: data.annotations,
      }, getLiveStoreTf()));
      historyHasMore = data.hasMore !== false;

      if (added === 0) {
        historyHasMore = false;
        console.warn('History pagination stalled: Zero overlap.');
        return;
      }

      const storeData = liveStore.getForLightweightCharts();

      ChartAdapter.applyHistoryPrepend('live', storeData, added);

      if (liveNavigatorResult) {
        ChartAdapter.setNavigatorOverlay('live', { navigators: liveNavigatorResult }, storeData.candles, {
          context: 'live',
          updateLoadedCandles: false,
        });
      }
      updateBufferingOverlay();
    } finally {
      liveStore.unseal();
      endDataUpdate();
    }

    if (!historyHasMore || currentEpoch !== liveHistoryEpoch) return;
  } catch (err) {
    console.error('lazy history:', err);
  } finally {
    isLoadingHistory = false;
  }

  if (historyHasMore && currentEpoch === liveHistoryEpoch) {
    setTimeout(() => {
      const finalRange = ChartAdapter.getVisibleLogicalRange('live');
      const threshold = (typeof CONFIG !== 'undefined' && CONFIG.LIVE_HISTORY_SCROLL_THRESHOLD) || 50;
      if (finalRange && finalRange.from < threshold) {
        scheduleHistoryLoad(finalRange);
      }
    }, 0);
  }
}

function scheduleHistoryLoad(range, options = {}) {
  if (liveStore.isSealed() || window.__isDashboardLoading) return;
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
    ToolbarController.updateHeaderData({ jurik: d.jurik, redLine: d.redLine, greenLine: d.greenLine });
    if (d.isClosed && d.volatilityRegime) {
      ToolbarController.updateHeaderData({ volatilityRegime: d.volatilityRegime });
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
  ToolbarController.setRulerActive(ruler.active);
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
  safeInit('panel settings', () => {
    initPanelSettingsOutsideClose();
    initPanelSettingsEnterNavigation();
  });
  safeInit('UI rsx', () => {
    RsxController.init();
    RsxController.onSettingsChanged(() => scheduleRsxSettingsSync('live'));
    fetchRsxIndicatorSettings();
  });
  safeInit('equity chart', initEquityChart);
  safeInit('UI backtest', () => {
    BacktestController.init();
    BacktestController.onRunRequested(() => runBacktest(true));
    BacktestController.onStopRequested(() => stopBacktest());
    BacktestController.onIntervalChange((tf) => handleBacktestIntervalChange(tf));
    backtestTf = getBacktestInterval();
  });
  safeInit('UI toolbar', () => ToolbarController.init());
  safeInit('UI layout', () => LayoutController.init());

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
          onAfterDelta: () => {
            updateBufferingOverlay();
            const osc = liveStore.getForLightweightCharts().osc;
            if (osc?.length) syncLiveRsxToolbarFromOsc(osc[osc.length - 1]);
          },
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
          WozduhController.init();
          NavigatorController.initLegends();
          ChartAdapter.initRuler();
        }
      } catch (err) {
        console.error('Failed to init charts:', err);
      }
      if (!chartsReady) {
        setTimeout(boot, 500);
        return;
      }

      safeInit('UI timeframe', () => TimeframeController.init({ useServerTf: true }));
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
