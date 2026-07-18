

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
    if (ChartAdapter.chartInitialized() && liveColumnarStore?.candleCount() > 0 && shouldPaintLiveChart()) {
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
      ChartAdapter.setNavigatorOverlay('live', { navigators: liveNavigatorResult }, liveColumnarStore.getForLightweightCharts().candles, {
        context: 'live',
        updateLoadedCandles: false,
      });
      ViewportManager.restore('live', viewportAnchor, liveColumnarStore);
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

  const navigators = NavigatorController.getNavigatorPayload('backtest');

  const settings = {
    ...prevSettings,
    navigators,
    rsxSettings: getRSXSettingsFromUI('backtest'),
    wozduhSettings: getWozduhSettingsFromUI('backtest'),
  };
  delete settings.matrix;
  delete settings.risk;
  delete settings.longThreshold;
  delete settings.shortThreshold;

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
    try {
      return {
        navigators: NavigatorController.getNavigatorPayload('backtest'),
      };
    } catch {
      return {
        navigators: {},
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
/** History lazy-load is armed only after the user scrolls/zooms the live chart (not on TF init). */
let liveHistoryScrollArmed = false;
/** @type {HydrationOrchestrator|null} */
let liveHydrationOrchestrator = null;

if (typeof window !== 'undefined') {
  window.__isSettingsUpdating = false;
}

const liveColumnarStore = typeof ColumnarStore !== 'undefined' ? new ColumnarStore() : null;
if (typeof window !== 'undefined' && liveColumnarStore) {
  window.liveColumnarStore = liveColumnarStore;
}

const liveChartCompositor = (typeof ChartCompositor !== 'undefined' && liveColumnarStore)
  ? new ChartCompositor({
    store: liveColumnarStore,
    shouldPaint: () => shouldPaintLiveChart(),
    getNavigatorResult: () => liveNavigatorResult,
    onAfterFlush: (intent) => {
      if (intent?.mode === 'full') {
        ChartAdapter.setChartInitialized(true);
      }
      updateBufferingOverlay();
    },
  })
  : null;

const liveRenderScheduler = (typeof RenderScheduler !== 'undefined' && liveChartCompositor)
  ? new RenderScheduler(liveChartCompositor)
  : null;





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
  if (!shouldPaintLiveChart() || !liveColumnarStore) return;
  const reloadVersion = ++rsxChartReloadVersion;
  try {
    const tf = getLiveStoreTf();
    const symbol = document.getElementById('symbol')?.textContent?.trim() || '';
    const endTimeSec = liveColumnarStore.lastTimeSec() ?? Math.floor(Date.now() / 1000);
    const limit = Math.max(liveColumnarStore.barCount(), 3000);
    const columnar = await API.fetchColumnarHistory({
      tf,
      endTimeSec,
      limit,
      slots: resolveLiveSlotIds(),
      rsxSettings: coerceRsxSettingsForAPI(RsxController.getSettings('live')),
      symbol,
    });
    if (reloadVersion !== rsxChartReloadVersion) return;
    if (!columnar?.plots || typeof columnar.plots !== 'object') return;

    beginDataUpdate();
    try {
      liveColumnarStore.updatePlots(columnar.plots);
      if (Array.isArray(columnar.annotations) && columnar.annotations.length) {
        liveColumnarStore.mergeAnnotations(columnar.annotations);
      }
      if (liveRenderScheduler && liveColumnarStore.invariantOk()) {
        liveRenderScheduler.markDirty({ mode: 'indicators' });
      }
    } finally {
      endDataUpdate();
    }
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
}

function updateBufferingOverlay() {
  ToolbarController.setBuffering(isOrderFlowTf() && (
    (liveColumnarStore?.candleCount() || 0) < 5 ||
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
  liveHydrationOrchestrator?.reset();
  disarmLiveHistoryScroll();
  beginDataUpdate();
  liveColumnarStore?.clear();
  if (window.DDRFactory) window.DDRFactory.clear();
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
  const T = (typeof ChartTheme !== 'undefined') ? ChartTheme : null;

  if (side === 'CLOSE_LONG') {
    return {
      time,
      position: 'aboveBar',
      color: T?.bear ?? '#f23645',
      shape: 'circle',
      text: 'EXIT',
    };
  }
  if (side === 'CLOSE_SHORT') {
    return {
      time,
      position: 'belowBar',
      color: T?.bull ?? '#089981',
      shape: 'circle',
      text: 'EXIT',
    };
  }

  const isBuy = side === 'BUY';
  return {
    time,
    position: isBuy ? 'belowBar' : 'aboveBar',
    color: isBuy ? (T?.buy ?? '#089981') : (T?.sell ?? '#f23645'),
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
  ChartAdapter.refreshPriceMarkers();
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
  if (typeof autoSwitchTabOrOptions === 'object' && autoSwitchTabOrOptions !== null) {
    options = autoSwitchTabOrOptions;
  }

  const form = typeof BacktestController !== 'undefined' ? BacktestController.getFormValues() : {};
  const symbol = form.symbol || 'BTCUSDT';
  const interval = form.interval || backtestTf || getActiveTfFromToolbar() || '15m';
  const tf = normalizeTf(interval);

  const startStr = form.start || form.startDate;
  const endStr = form.end || form.endDate;

  let reqStartSec = 0;
  let reqEndSec = Math.floor(Date.now() / 1000);
  if (startStr) {
    reqStartSec = Math.floor(new Date(`${startStr}T00:00:00Z`).getTime() / 1000);
  }
  if (endStr) {
    reqEndSec = Math.floor(
      (new Date(`${endStr}T00:00:00Z`).getTime() + 24 * 60 * 60 * 1000 - 1) / 1000,
    );
  }

  const fp = backtestStore.getFingerprint();
  const needsBaseReload = !fp
    || fp.symbol !== symbol
    || fp.interval !== tf
    || (fp.startSec != null && reqStartSec < fp.startSec)
    || (fp.endSec != null && reqEndSec > fp.endSec);

  // #region agent log
  fetch('http://127.0.0.1:7650/ingest/e96d7e9c-02c2-4eef-b8f6-4424f0be67d3',{method:'POST',headers:{'Content-Type':'application/json','X-Debug-Session-Id':'39f875'},body:JSON.stringify({sessionId:'39f875',runId:'bt-black-screen-diagnosis',hypothesisId:'H1',location:'web/app.js:runBacktest:beforePipeline',message:'runBacktest input/fingerprint snapshot',data:{symbol,interval,tf,reqStartSec,reqEndSec,needsBaseReload,hasBaseLayer:backtestStore.hasBaseLayer(),fp},timestamp:Date.now()})}).catch(()=>{});
  // #endregion

  BacktestController.applyDateRangeLimits(interval);
  const startDate = form.start;
  const endDate = form.end;

  const manageLoading = options.manageLoading !== false;
  const skipSettingsPush = options.skipSettingsPush === true;

  backtestRunActive = true;
  BacktestController.setRunActive(true);
  backtestAbortController = new AbortController();

  try {
    const payload = buildFinalBacktestPayload({ symbol, interval, startDate, endDate });
    const isExplicitRefresh = options.navigatorRefresh === true;
    const isUiDirty = typeof NavigatorController !== 'undefined' && typeof NavigatorController.consumeDirtyState === 'function'
      ? NavigatorController.consumeDirtyState('backtest')
      : false;
    const isNavRefresh = isExplicitRefresh || isUiDirty;

    if (typeof BacktestPipeline === 'undefined') {
      throw new Error('BacktestPipeline is not loaded');
    }

    await BacktestPipeline.run({
      payload,
      tf,
      needsBaseReload,
      isNavRefresh,
      signal: backtestAbortController?.signal,
      skipSettingsPush,
      manageLoading,
    });
  } catch (err) {
    if (err?.name === 'AbortError') {
      return;
    }
  } finally {
    resetBacktestRunUi();
    if (typeof ChartProjection !== 'undefined') {
      ChartProjection.trySync();
    }
  }
}

function applySeriesData() {
  if (!shouldPaintLiveChart()) return;
  if (
    liveRenderScheduler
    && liveColumnarStore?.barCount() > 0
    && liveColumnarStore.invariantOk()
  ) {
    liveRenderScheduler.markDirty({ mode: 'full', viewport: 'fresh' });
  }
  updateBufferingOverlay();
}

function liveCandlesFromColumnar() {
  if (!liveColumnarStore?.barCount()) return [];
  const snap = liveColumnarStore.snapshot();
  return columnarToCandles({ times: snap.times, candles: snap.candles });
}

function commitLiveHeaderState(data, options = {}) {
  syncTradingTimeframeFromState(data);
  ToolbarController.updateHeaderData(data);
  if (typeof data.tickBufferLen === 'number') {
    lastTickBufferLen = data.tickBufferLen;
  }

  const candles = liveCandlesFromColumnar();

  const masterState = data.masterState || deriveBotStatus({ ...data, trades: data.trades });
  mergeSessionTrades(data.trades, {
    masterState,
    lastCandleTime: candles.length ? candles[candles.length - 1].time : null,
  });

  historyHasMore = typeof data.hasMore === 'boolean'
    ? data.hasMore
    : (liveColumnarStore?.barCount() > 0);
  if (data.navigators) {
    liveNavigatorResult = data.navigators;
  }

  ChartAdapter.renderFib(data.fibZones);

  if (options.viewportAnchor && typeof ViewportManager !== 'undefined' && liveColumnarStore?.barCount()) {
    ViewportManager.restore('live', options.viewportAnchor, liveColumnarStore);
  }
}

function rowsPayloadToColumnar(candles, annotations = []) {
  const times = [];
  const open = [];
  const high = [];
  const low = [];
  const close = [];
  const volume = [];
  for (const bar of candles || []) {
    const normalized = typeof normalizeCandle === 'function' ? normalizeCandle(bar) : bar;
    if (!normalized?.time) continue;
    times.push(normalized.time);
    open.push(normalized.open);
    high.push(normalized.high);
    low.push(normalized.low);
    close.push(normalized.close);
    volume.push(normalized.volume ?? 0);
  }
  return {
    times,
    candles: { open, high, low, close, volume },
    plots: {},
    annotations: annotations || [],
    added: times.length,
  };
}

function pushLiveTickDelta(tick) {
  if (!liveColumnarStore || !liveRenderScheduler) return false;
  if (liveColumnarStore.isSealed()) return false;

  const appendResult = liveColumnarStore.appendTick(tick);
  if (!appendResult?.candle) return false;

  const delta = appendResult.delta ?? {
    candle: appendResult.candle,
    isNewBar: appendResult.isNewBar,
    barCount: appendResult.barCount,
  };
  if (!delta?.candle) return false;

  liveRenderScheduler.markDirty({
    mode: 'delta',
    tick,
    delta,
  });
  return true;
}

function applyLatestOscPoint(pt) {
  if (!pt) return;
  const latestCandleTime = liveColumnarStore?.lastTimeSec();
  if (latestCandleTime != null && pt.time !== latestCandleTime) {
    pt = { ...pt, time: latestCandleTime };
  }
  syncLiveRsxToolbarFromOsc(pt);
}

function renderState(data, options = {}) {
  const compositorReady = liveRenderScheduler
    && liveColumnarStore?.barCount() > 0
    && liveColumnarStore.invariantOk()
    && options.forceLegacyPaint !== true;

  commitLiveHeaderState(data, options);

  if (compositorReady && !options.skipPaint) {
    beginDataUpdate();
    try {
      liveRenderScheduler.markDirty({
        mode: 'full',
        viewport: options.viewportAnchor ? 'restore' : (options.isPreFetch ? 'fresh' : undefined),
        anchor: options.viewportAnchor || null,
      });
    } finally {
      endDataUpdate();
    }
  }

  ChartAdapter.setChartInitialized(compositorReady || options.forceLegacyPaint !== true);
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
    if (liveColumnarStore && liveRenderScheduler) {
      liveColumnarStore.replaceMonolith(rowsPayloadToColumnar(candles, data.annotations));
      commitLiveHeaderState({ ...data, hasMore: data.hasMore });
      liveRenderScheduler.markDirty({ mode: 'full', viewport: 'fresh' });
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

    ToolbarController.updateHeaderData({ jurik: data.jurik, plots: data.plots });

    const candles = toCandles(data.candles);
    const latest = candles[candles.length - 1];
    if (!latest) return;

    const last = liveColumnarStore?.lastTimeSec();
    if (last && latest.time === last) {
      pushLiveTickDelta({ ...latest, time: latest.time });
    } else if (!last || latest.time > last) {
      if (last && isLiveTickGapTooLarge(last, latest.time)) {
        if (!_microscopeTickMuted) {
          console.log('[Mode Switch] Time gap too large. Entering historical microscope mode. Live poll ticks will be silently ignored.');
          _microscopeTickMuted = true;
        }
        return;
      }
      pushLiveTickDelta({ ...latest, time: latest.time });
    } else {
      return;
    }

    const osc = data.oscillators || [];
    if (osc.length > 0) {
      applyLatestOscPoint(osc[osc.length - 1]);
    }
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
    let usedCompositorPaint = false;

    const symbol = document.getElementById('symbol')?.textContent?.trim() || '';
    const rsxSettings = typeof coerceRsxSettingsForAPI === 'function' && typeof RsxController !== 'undefined'
      ? coerceRsxSettingsForAPI(RsxController.getSettings('live'))
      : undefined;

    if (isMicroscope) {
      if (window.DDRFactory && !window.DDRFactory.manifest) {
        try {
          await window.DDRFactory.fetchManifest();
        } catch (err) {
          console.warn('[Boot] manifest fetch failed (microscope):', err);
        }
      }
      if (reqId !== currentLiveRequestId) return;

      const intervalMs = getIntervalMs(tf);
      const halfWindowMs = (TARGET_BARS / 2) * intervalMs;
      const targetEndTimeSec = Math.floor((anchor.centerTimeMs + halfWindowMs) / 1000);

      try {
        historyData = await API.fetchColumnarHistory({
          tf,
          endTimeSec: targetEndTimeSec,
          limit: TARGET_BARS,
          slots: resolveLiveSlotIds(),
          rsxSettings,
          symbol,
        });
      } catch (histErr) {
        console.warn('Dashboard microscope history:', histErr);
      }
      if (reqId !== currentLiveRequestId) return;

      if (!liveColumnarStore) {
        console.error('[Boot] ColumnarStore unavailable — blocking microscope paint');
        return;
      }

      liveColumnarStore.seal();
      liveColumnarStore.replaceMonolith(historyData || {});

      stateData = {
        candles: historyData?.times?.length ? columnarToCandles(historyData) : [],
        oscillators: [],
        annotations: historyData?.annotations || [],
        hasMore: historyData?.hasMore,
        status: 'ready',
      };
      usedCompositorPaint = liveColumnarStore.invariantOk();
    } else {
      if (window.DDRFactory && !window.DDRFactory.manifest) {
        try {
          await window.DDRFactory.fetchManifest();
        } catch (err) {
          console.warn('[Boot] manifest fetch failed:', err);
        }
      }
      if (reqId !== currentLiveRequestId) return;

      const slots = resolveLiveSlotIds();
      const endTimeSec = Math.floor(Date.now() / 1000);

      const [columnar, headerResult] = await Promise.all([
        API.fetchColumnarHistory({
          tf,
          endTimeSec,
          limit: TARGET_BARS,
          slots,
          rsxSettings,
          symbol,
        }),
        fetchLiveState({
          navigatorsOnly: true,
          userTfChange: options.userTfChange === true,
        }),
      ]);
      if (reqId !== currentLiveRequestId) return;

      const { warmingUp, data: headerState } = headerResult;
      stateData = headerState || {};

      if (_microscopeTickMuted && !warmingUp && columnar?.times?.length) {
        console.log('[Mode Switch] Returned to Live edge. Accepting WS ticks again.');
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

      if (!columnar?.times?.length) {
        if (stateData.status === 'ready') {
          await renderState(
            { ...stateData, candles: [], oscillators: [] },
            { ...options, forceLegacyPaint: true },
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

      if (!liveColumnarStore) {
        console.error('[Boot] ColumnarStore unavailable — blocking chart paint');
        return;
      }

      liveColumnarStore.replaceMonolith(columnar);
      if (!liveColumnarStore.invariantOk()) {
        console.error('[Boot] ColumnarStore invariant failed — blocking chart paint', liveColumnarStore.invariantMeta());
        return;
      }


      if (window.DDRFactory) {
        try {
          await mountDDRLiveCutover();
        } catch (err) {
          console.warn('[Boot] DDR mount failed:', err);
        }
      }

      stateData = {
        ...stateData,
        candles: columnarToCandles(columnar),
        oscillators: [],
        annotations: columnar.annotations || [],
        hasMore: columnar.hasMore,
        status: columnar.status || 'ready',
      };
      usedCompositorPaint = true;
    }

    if (reqId !== currentLiveRequestId) return;
    if (usedCompositorPaint && liveRenderScheduler) {
      beginDataUpdate();
      try {
        commitLiveHeaderState(stateData, {
          ...options,
          viewportAnchor: options.viewportAnchor,
        });
        liveRenderScheduler.markDirty({
          mode: 'full',
          viewport: options.viewportAnchor ? 'restore' : 'fresh',
          anchor: options.viewportAnchor || null,
        });
      } finally {
        endDataUpdate();
      }
    } else {
      await renderState(stateData, { ...options, isPreFetch: !isMicroscope, storeReady: true });
    }
  } catch (err) {
    handleLoadError(err);
  } finally {
    liveColumnarStore?.unseal();
    ToolbarController.setBuffering(false);
    window.__isDashboardLoading = false;
  }
}

async function loadBacktestHistoryShell(options = {}) {
  if (typeof BacktestPipeline === 'undefined') {
    console.warn('[Backtest] BacktestPipeline is not loaded');
    return;
  }
  return BacktestPipeline.loadShell(options);
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
      const prependResult = backtestStore.prependHistory(
        chartPointsToStorePayload(data.chartData, data.annotations),
        getBacktestStoreTf(),
      );
      ({ added } = prependResult);
      backtestHistoryHasMore = data.hasMore !== false;

      if (added === 0) {
        // #region agent log
        fetch('http://127.0.0.1:7650/ingest/e96d7e9c-02c2-4eef-b8f6-4424f0be67d3',{method:'POST',headers:{'Content-Type':'application/json','X-Debug-Session-Id':'39f875'},body:JSON.stringify({sessionId:'39f875',runId:'bt-black-screen-diagnosis',hypothesisId:'H4',location:'web/app.js:maybeLoadBacktestHistory:zeroOverlap',message:'Backtest history prepend added zero',data:{symbol,interval,range,firstTime,dataLen:Array.isArray(data.chartData)?data.chartData.length:0,dataFirstTime:data.chartData?.[0]?.time??data.chartData?.[0]?.Time??null,dataLastTime:data.chartData?.[data.chartData.length-1]?.time??data.chartData?.[data.chartData.length-1]?.Time??null,prependResult,storeFirst:backtestStore.firstCandleTimeSec(),storeLast:backtestStore.lastCandleTimeSec()},timestamp:Date.now()})}).catch(()=>{});
        // #endregion
        backtestHistoryHasMore = false;
        console.warn('History pagination stalled: Zero overlap.');
        return;
      }

      const storeData = backtestStore.getForLightweightCharts();

      ChartAdapter.applyHistoryPrepend('backtest', storeData, added);
      ChartAdapter.applyWozduhVisibility('backtest');
      ChartAdapter.applyBacktestMarkers(backtestStore.getTrades(), storeData.osc);

      backtestHistoryHasMore = data.hasMore !== false && added > 0;

      const fp = backtestStore.getFingerprint();
      if (fp) {
        const newStart = backtestStore.firstCandleTimeSec();
        if (newStart != null) {
          backtestStore.setFingerprint(fp.symbol, fp.interval, newStart, fp.endSec);
        }
      }
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
    slots: resolveLiveSlotIds(),
    rsxSettings: coerceRsxSettingsForAPI(RsxController.getSettings('live')),
  });
}

function collectManifestScalarSlotIds(manifest) {
  if (!manifest?.panes || typeof manifest.panes !== 'object') return [];
  const ids = [];
  for (const components of Object.values(manifest.panes)) {
    if (!Array.isArray(components)) continue;
    for (const component of components) {
      const kind = String(component?.kind || 'line').toLowerCase();
      if (kind === 'marker' || component?.dataMode === 'annotations') continue;
      if (component?.id) ids.push(component.id);
    }
  }
  return ids;
}

function resolveLiveSlotIds() {
  const seriesSlots = window.DDRFactory ? [...window.DDRFactory.seriesMap.keys()] : [];
  if (seriesSlots.length > 0) return seriesSlots;
  return collectManifestScalarSlotIds(window.DDRFactory?.manifest);
}

function initHydrationOrchestrator() {
  if (typeof HydrationOrchestrator === 'undefined') return;
  liveHydrationOrchestrator = new HydrationOrchestrator();
  liveHydrationOrchestrator.init({
    getEpoch: () => liveHistoryEpoch,
    getReqId: () => currentLiveRequestId,
    getHistoryHasMore: () => historyHasMore,
    setHistoryHasMore: (v) => { historyHasMore = v; },
    setLoadingHistory: (v) => { isLoadingHistory = v; },
    sealStore: () => { liveColumnarStore?.seal(); },
    unsealStore: () => { liveColumnarStore?.unseal(); },
    shouldLoad: (range, options) => {
      if (!ChartAdapter.isInitialized('live') || window.__isDashboardLoading) return false;
      // Microscope mute guards WS ticks only — never block historical REST prepend (#5).
      const force = options.force === true;
      if (isLoadingHistory || liveHydrationOrchestrator?.isBusy() || !historyHasMore) return false;
      if (!shouldPaintLiveChart()) return false;
      if (!ChartAdapter.chartInitialized() || !liveHistoryScrollArmed) return false;
      const barCount = liveColumnarStore?.barCount?.() ?? 0;
      if (!range || barCount === 0) return false;
      if (!force && range.from >= LIVE_HISTORY_SCROLL_THRESHOLD) return false;
      return true;
    },
    getAnchorEndTimeSec: () => {
      const firstTime = liveColumnarStore?.firstTimeSec?.() ?? null;
      return firstTime == null ? null : Number(firstTime);
    },
    getSlotIds: () => resolveLiveSlotIds(),
    fetchColumnar: async (endTimeSec) => {
      const symbol = document.getElementById('symbol')?.textContent?.trim() || '';
      return API.fetchColumnarHistory({
        tf: currentTf,
        endTimeSec,
        limit: HISTORY_CHUNK_LIMIT,
        slots: resolveLiveSlotIds(),
        rsxSettings: coerceRsxSettingsForAPI(RsxController.getSettings('live')),
        symbol,
      });
    },
    mergeIntoStore: (data) => {
      if (!liveColumnarStore) return null;
      const viewportRange = ChartAdapter.getVisibleLogicalRange('live');
      const { added } = liveColumnarStore.prependMonolith(data);
      if (added <= 0) return null;
      return { added, viewportRange };
    },
    markDirty: (intent) => {
      if (liveRenderScheduler) {
        liveRenderScheduler.markDirty(intent);
      }
    },
    processTick: (tick) => _processLiveTickCore(tick),
  });
}

async function maybeLoadHistory(range, options = {}) {
  if (!liveHydrationOrchestrator) return;
  if (options.force === true) {
    await liveHydrationOrchestrator.requestPrepend(range, options);
    return;
  }
  liveHydrationOrchestrator.schedulePrepend(range, options);
}

function scheduleHistoryLoad(range, options = {}) {
  if (liveColumnarStore?.isSealed() || window.__isDashboardLoading) return;
  if (!range || !historyHasMore) return;
  if (!ChartAdapter.chartInitialized() || !liveHistoryScrollArmed) return;
  if (range.from >= LIVE_HISTORY_SCROLL_THRESHOLD) return;
  if (ChartAdapter.isLiveUpdating() || liveHydrationOrchestrator?.isBusy()) return;
  if (liveHydrationOrchestrator) {
    liveHydrationOrchestrator.schedulePrepend(range, options);
  }
}

function isLiveTf() {
  if (isOrderFlowTf()) return true;
  const scoringTf = backendTradingTimeframe || currentTf;
  return scoringTf.toLowerCase() === currentTf.toLowerCase();
}

function _processLiveTickCore(d) {
  const tickTf = (d.timeframe || backendTradingTimeframe || currentTf || '1m').toLowerCase();
  if (tickTf !== currentTf.toLowerCase()) return;

  if (isLiveTf()) {
    ToolbarController.updateHeaderData({ jurik: d.jurik, plots: d.plots });
    if (d.isClosed && d.volatilityRegime) {
      ToolbarController.updateHeaderData({ volatilityRegime: d.volatilityRegime });
    }
  }

  if (!shouldPaintLiveChart()) return;
  if (!ChartAdapter.chartInitialized()) return;

  const time = chartTime(d.time);
  if (time == null) return;

  const lastTime = liveColumnarStore?.lastTimeSec();
  if (lastTime != null && time < lastTime) return;

  if (lastTime != null && time > lastTime && isLiveTickGapTooLarge(lastTime, time)) {
    if (!_microscopeTickMuted) {
      console.log(`[Mode Switch] Time gap too large (${time} vs ${lastTime}). Entering historical microscope mode. Live WS ticks will be silently ignored until return to live edge.`);
      _microscopeTickMuted = true;
    }
    return;
  }

  pushLiveTickDelta(d);
}

function handleLiveTick(d) {
  if (liveHydrationOrchestrator?.queueTick(d)) return;
  _processLiveTickCore(d);
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
    : (liveColumnarStore?.getForLightweightCharts().candles || []);
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

function buildLiveDDRChartRegistry() {
  const price = ChartAdapter.getChart('live', 'price');
  const rsx = ChartAdapter.getChart('live', 'rsx');
  const wozduh = ChartAdapter.getChart('live', 'wozduh');
  if (!price || !rsx || !wozduh) return null;
  return {
    pane_osc: { chart: rsx, defaultPriceScaleId: 'right' },
    pane_score: { chart: wozduh, defaultPriceScaleId: 'right' },
  };
}

async function mountDDRLiveCutover() {
  if (!window.DDRFactory?.manifest || !ChartAdapter.isInitialized('live')) {
    return false;
  }
  const chartRegistry = buildLiveDDRChartRegistry();
  if (!chartRegistry) return false;

  window.DDRFactory.buildPanes(chartRegistry, window.DDRFactory.manifest.panes);
  window.DDRFactory.applyHydratedData();

  if (typeof SettingsRenderer !== 'undefined') {
    SettingsRenderer.refreshFromManifest();
  }

  ChartAdapter.hideLegacyOscillatorSeries('live');
  ChartAdapter.enableDDROscCutover();
  return true;
}

function initDDRFactory() {
  if (typeof DDRFactory === 'undefined') return;
  window.DDRFactory = new DDRFactory({
    normalizeTime: (raw) => {
      if (typeof chartTime === 'function') return chartTime(raw);
      return DDRFactory.defaultNormalizeTime(raw);
    },
  });
  window.DDRFactory.fetchManifest().catch((err) => {
    console.warn('[DDRFactory] manifest fetch failed:', err);
  });
}

function boot() {
  safeInit('DDR factory', initDDRFactory);
  safeInit('Hydration orchestrator', initHydrationOrchestrator);
  safeInit('UI strategy', () => StrategyController.init());
  safeInit('UI risk', () => RiskController.init());
  safeInit('UI rsx', () => {
    RsxController.init();
    RsxController.onSettingsChanged(() => scheduleRsxSettingsSync('live'));
    fetchRsxIndicatorSettings();
  });
  safeInit('UI tabs', () => TabsController.init());
  safeInit('UI timeframe', () => TimeframeController.init({ useServerTf: true }));
  safeInit('UI toolbar', () => ToolbarController.init());
  safeInit('UI layout', () => LayoutController.init());
  syncToolbarToActiveContext();
  safeInit('UI navigator', () => {
    NavigatorController.init();
    NavigatorController.onSettingsChanged(() => triggerNavigatorAutoUpdate());
  });
  safeInit('panel settings', () => {
    initPanelSettingsOutsideClose();
    initPanelSettingsEnterNavigation();
  });
  safeInit('equity chart', initEquityChart);
  safeInit('UI backtest', () => {
    BacktestController.init();
    BacktestController.onRunRequested(() => runBacktest(true));
    BacktestController.onStopRequested(() => stopBacktest());
    BacktestController.onIntervalChange((tf) => handleBacktestIntervalChange(tf));
    backtestTf = getBacktestInterval();
  });

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
        });
        if (chartsReady) {
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
