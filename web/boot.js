/**
 * boot.js — Project Renaissance composition root (live Core 3.0).
 * Shims keep legacy UI controllers from ReferenceError; paint via Compositor + DDR.
 */
(function () {
  'use strict';

  const noop = () => {};
  const noopAsync = async () => {};

  // ── Global state (legacy UI contract) — must live on window for UI controllers ──
  window.currentTf = window.currentTf || '15m';
  window.backtestTf = window.backtestTf || '15m';
  window.currentLiveRequestId = window.currentLiveRequestId ?? 0;
  window.navigatorRequestId = window.navigatorRequestId ?? 0;
  window.historyHasMore = window.historyHasMore ?? true;
  window.isLoadingHistory = window.isLoadingHistory ?? false;
  window.backtestHistoryLoading = window.backtestHistoryLoading ?? false;
  window.backtestHistoryHasMore = window.backtestHistoryHasMore ?? true;
  window.isAppInitialized = window.isAppInitialized ?? false;
  window.backtestRunActive = window.backtestRunActive ?? false;
  window.tradeMarkers = window.tradeMarkers ?? [];
  window.sessionTrades = window.sessionTrades ?? [];
  window.spikeMarkers = window.spikeMarkers ?? [];
  window.refreshTimer = window.refreshTimer ?? null;
  window.orderFlowPollTimer = window.orderFlowPollTimer ?? null;
  window.isUpdatingData = false;
  window.__isDashboardLoading = false;
  window.__isSettingsUpdating = false;
  window.lastFibZones = window.lastFibZones ?? [];
  window.currentBacktestPayload = window.currentBacktestPayload || null;

  const DEFAULT_RSX_SETTINGS = {
    length: 14,
    signal_length: 9,
    source: 'close',
    div_method: 'standard',
    pivot_radius: 5,
    div_lookback: 60,
    min_price_delta_ratio: 0.1,
    min_osc_delta: 0.1,
  };

  let backendTradingTimeframe = null;
  let liveHistoryEpoch = 0;
  let liveHistoryScrollArmed = false;
  let liveNavigatorResult = null;
  let liveHydrationOrchestrator = null;

  function installGlobalShims() {
    const fns = {
      loadDashboard,
      clearChartData,
      wsSubscribeTf,
      startLivePollTimer: noop,
      isOrderFlowTf: () => false,
      pollOrderFlowState: noopAsync,
      updateBufferingOverlay,
      handleBacktestIntervalChange: noop,
      getBacktestInterval: () => window.backtestTf,
      abortLiveStateFetch: noop,
      disarmLiveHistoryScroll,
      openFloatingMenu: noop,
      initFloatingMenuDrag: noop,
      buildFinalBacktestPayload: () => window.currentBacktestPayload || {},
      getActiveUiContext: () => 'live',
      shouldPaintLiveChart: () => TabsController?.isLiveTabActive?.() !== false,
      runBacktest: noopAsync,
      stopBacktest: noop,
      syncRsxIndicatorSettings: noopAsync,
      pushRsxSettingsToServer: noopAsync,
      reloadRsxChartFromServer: noopAsync,
      fetchRsxIndicatorSettings: noopAsync,
      scheduleRsxSettingsSync: noop,
      triggerNavigatorAutoUpdate: noop,
      refreshStatsForMode: noop,
      initPanelSettingsOutsideClose: noop,
      initPanelSettingsEnterNavigation: noop,
      initEquityChart: noop,
      toggleRuler: noop,
      resetRuler: noop,
      setRulerCursor: noop,
      getRulerChartData: () => null,
      shouldRunLivePoll: () => false,
      pollLatestState: noopAsync,
      applySeriesData: noop,
      beginDataUpdate,
      endDataUpdate,
      getIntervalMs: typeof getIntervalMs === 'function' ? getIntervalMs : () => 60000,
      isLiveTf: () => true,
      getLiveStoreTf: () => window.currentTf,
    };
    Object.entries(fns).forEach(([name, fn]) => { window[name] = fn; });
  }

  function installChartAdapterShims() {
    if (typeof ChartAdapter === 'undefined') return;
    Object.assign(ChartAdapter, {
      chartInitialized: () => ChartAdapter.isInitialized('live'),
      setChartInitialized: noop,
      isLiveUpdating: () => window.isUpdatingData,
      applyWozduhVisibility: noop,
      applyOrderFlowTimeScale: noop,
      getChartHandle: (ctx) => ({
        chart: ChartAdapter.getChart(ctx, 'price'),
        charts: {
          price: ChartAdapter.getChart(ctx, 'price'),
          wozduh: ChartAdapter.getChart(ctx, 'wozduh'),
          rsx: ChartAdapter.getChart(ctx, 'rsx'),
        },
        candleSeries: null,
      }),
      setToggleSeriesVisible: noop,
      applyAllMarkers: noop,
      setChartType: noop,
      renderFib: noop,
      setEquityData: noop,
      fitEquityContent: noop,
      resizeEquity: noop,
      setLegendVisibility: noop,
      getChartType: () => 'candles',
      applyRsxData: noop,
      applyLiveAnnotationLayer: noop,
      setNavigatorOverlay: noop,
      hideLegacyOscillatorSeries: noop,
      enableDDROscCutover: noop,
      destroyLiveCharts: noop,
      syncVisibleLogicalRange: (chart, range) => chart?.timeScale()?.setVisibleLogicalRange(range),
      ensureBacktestChart: () => false,
      activateSurface: () => false,
      applySimOverlay: noop,
      applyBacktestMarkers: noop,
      initRuler: noop,
      attachRuler: noop,
      updateRulerOverlay: noop,
    });
  }

  const liveColumnarStore = typeof ColumnarStore !== 'undefined' ? new ColumnarStore() : null;
  /** @type {RenderScheduler|null} */
  let liveRenderScheduler = null;

  if (liveColumnarStore) window.liveColumnarStore = liveColumnarStore;

  function initLiveRenderPipeline() {
    if (!liveColumnarStore || typeof ChartCompositor === 'undefined' || typeof RenderScheduler === 'undefined') return;
    const compositor = new ChartCompositor({
      store: liveColumnarStore,
      shouldPaint: () => (typeof window.shouldPaintLiveChart === 'function' ? window.shouldPaintLiveChart() : true),
      getNavigatorResult: () => liveNavigatorResult,
      onAfterFlush: () => updateBufferingOverlay(),
    });
    liveRenderScheduler = new RenderScheduler(compositor);
  }

  function beginDataUpdate() {
    window.isUpdatingData = true;
    ChartAdapter?.setLiveUpdating?.(true);
  }

  function endDataUpdate() {
    window.isUpdatingData = false;
    ChartAdapter?.setLiveUpdating?.(false);
  }

  function disarmLiveHistoryScroll() {
    liveHistoryScrollArmed = false;
  }

  function updateBufferingOverlay() {
    if (typeof ToolbarController !== 'undefined') {
      ToolbarController.setBuffering(false);
    }
  }

  function wsSubscribeTf(tf) {
    if (typeof WS !== 'undefined') WS.subscribe(tf, tf);
  }

  function clearChartData() {
    liveHistoryEpoch += 1;
    liveHydrationOrchestrator?.reset();
    disarmLiveHistoryScroll();
    liveColumnarStore?.clear();
    window.DDRFactory?.clear?.();
    liveNavigatorResult = null;
    window.historyHasMore = true;
    window.isLoadingHistory = false;
  }

  function collectManifestScalarSlotIds(manifest) {
    if (!manifest?.panes) return [];
    const ids = [];
    for (const components of Object.values(manifest.panes)) {
      if (!Array.isArray(components)) continue;
      for (const c of components) {
        if (String(c?.kind || 'line').toLowerCase() === 'marker') continue;
        if (c?.dataMode === 'annotations') continue;
        if (c?.id) ids.push(c.id);
      }
    }
    return ids;
  }

  function resolveLiveSlotIds() {
    const fromMap = window.DDRFactory ? [...window.DDRFactory.seriesMap.keys()] : [];
    if (fromMap.length) return fromMap;
    return collectManifestScalarSlotIds(window.DDRFactory?.manifest);
  }

  function initDDRFactory() {
    if (typeof DDRFactory === 'undefined') return;
    window.DDRFactory = new DDRFactory({
      normalizeTime: (raw) => (typeof chartTime === 'function' ? chartTime(raw) : DDRFactory.defaultNormalizeTime(raw)),
    });
    window.DDRFactory.fetchManifest().catch((err) => {
      console.warn('[Renaissance] manifest fetch failed:', err);
    });
  }

  async function mountDDRLiveCutover() {
    if (!window.DDRFactory?.manifest || !ChartAdapter.isInitialized('live')) return false;
    const rsx = ChartAdapter.getChart('live', 'rsx');
    const wozduh = ChartAdapter.getChart('live', 'wozduh');
    if (!rsx || !wozduh) return false;
    window.DDRFactory.buildPanes({
      pane_osc: { chart: rsx, defaultPriceScaleId: 'right' },
      pane_score: { chart: wozduh, defaultPriceScaleId: 'right' },
    }, window.DDRFactory.manifest.panes);
    if (typeof SettingsRenderer !== 'undefined') SettingsRenderer.refreshFromManifest();
    return true;
  }

  function initHydrationOrchestrator() {
    if (typeof HydrationOrchestrator === 'undefined') return;
    liveHydrationOrchestrator = new HydrationOrchestrator();
    liveHydrationOrchestrator.init({
      getEpoch: () => liveHistoryEpoch,
      getReqId: () => window.currentLiveRequestId,
      getHistoryHasMore: () => window.historyHasMore,
      setHistoryHasMore: (v) => { window.historyHasMore = v; },
      setLoadingHistory: (v) => { window.isLoadingHistory = v; },
      sealStore: () => liveColumnarStore?.seal(),
      unsealStore: () => liveColumnarStore?.unseal(),
      shouldLoad: (range, options) => {
        if (!ChartAdapter.isInitialized('live') || window.__isDashboardLoading) return false;
        if (window.isLoadingHistory || liveHydrationOrchestrator?.isBusy() || !window.historyHasMore) return false;
        if (!liveHistoryScrollArmed) return false;
        if (!range || (liveColumnarStore?.barCount?.() ?? 0) === 0) return false;
        if (options.force !== true && range.from >= (typeof LIVE_HISTORY_SCROLL_THRESHOLD !== 'undefined' ? LIVE_HISTORY_SCROLL_THRESHOLD : 50)) return false;
        return true;
      },
      getAnchorEndTimeSec: () => liveColumnarStore?.firstTimeSec?.() ?? null,
      getSlotIds: () => resolveLiveSlotIds(),
      fetchColumnar: (endTimeSec) => {
        const symbol = document.getElementById('symbol')?.textContent?.trim() || '';
        return API.fetchColumnarHistory({
          tf: window.currentTf,
          endTimeSec,
          limit: typeof HISTORY_CHUNK_LIMIT !== 'undefined' ? HISTORY_CHUNK_LIMIT : 3000,
          slots: resolveLiveSlotIds(),
          rsxSettings: DEFAULT_RSX_SETTINGS,
          symbol,
        });
      },
      mergeIntoStore: (data) => {
        const viewportRange = ChartAdapter.getVisibleLogicalRange('live');
        const { added } = liveColumnarStore.prependMonolith(data);
        if (added <= 0) return null;
        return { added, viewportRange };
      },
      markDirty: (intent) => liveRenderScheduler?.markDirty(intent),
      processTick: (tick) => pushLiveTickDelta(tick),
    });
  }

  function attachLiveHistoryScrollArm() {
    const root = document.getElementById('live-chart-container');
    if (!root || root._historyScrollArmBound) return;
    root._historyScrollArmBound = true;
    const arm = () => { liveHistoryScrollArmed = true; };
    root.addEventListener('wheel', arm, { passive: true });
    root.addEventListener('pointerdown', arm, { passive: true });
  }

  function scheduleHistoryLoad(range) {
    if (liveColumnarStore?.isSealed?.() || window.__isDashboardLoading) return;
    if (!range || !window.historyHasMore) return;
    if (!liveHistoryScrollArmed) return;
    if (range.from >= (typeof LIVE_HISTORY_SCROLL_THRESHOLD !== 'undefined' ? LIVE_HISTORY_SCROLL_THRESHOLD : 50)) return;
    if (window.isUpdatingData || liveHydrationOrchestrator?.isBusy()) return;
    liveHydrationOrchestrator?.schedulePrepend(range);
  }

  function pushLiveTickDelta(tick) {
    if (!liveColumnarStore || !liveRenderScheduler || liveColumnarStore.isSealed()) return false;
    const appendResult = liveColumnarStore.appendTick(tick);
    if (!appendResult?.candle) return false;
    liveRenderScheduler.markDirty({
      mode: 'delta',
      tick,
      delta: appendResult.delta ?? {
        candle: appendResult.candle,
        isNewBar: appendResult.isNewBar,
        barCount: appendResult.barCount,
      },
    });
    return true;
  }

  function handleLiveTick(d) {
    if (liveHydrationOrchestrator?.queueTick(d)) return;
    const tickTf = (d.timeframe || backendTradingTimeframe || window.currentTf || '15m').toLowerCase();
    if (tickTf !== window.currentTf.toLowerCase()) return;
    if (!ChartAdapter.isInitialized('live')) return;
    if (typeof chartTime === 'function' && chartTime(d.time) == null) return;
    pushLiveTickDelta(d);
  }

  function initLiveWebSocket() {
    if (typeof WS === 'undefined') return;
    WS.connect({
      onTick: handleLiveTick,
      onMarker: noop,
      onOpen: () => wsSubscribeTf(window.currentTf),
    });
  }

  async function loadDashboard() {
    const reqId = ++window.currentLiveRequestId;
    if (!ChartAdapter.isInitialized('live') && !ChartAdapter.initLiveCharts()) {
      setTimeout(loadDashboard, 500);
      return;
    }

    window.__isDashboardLoading = true;
    if (typeof ToolbarController !== 'undefined') ToolbarController.setBuffering(true);
    try {
      if (window.DDRFactory && !window.DDRFactory.manifest) {
        await window.DDRFactory.fetchManifest();
      }
      if (reqId !== window.currentLiveRequestId) return;

      const symbol = document.getElementById('symbol')?.textContent?.trim() || '';
      const endTimeSec = Math.floor(Date.now() / 1000);
      const limit = typeof HISTORY_CHUNK_LIMIT !== 'undefined' ? HISTORY_CHUNK_LIMIT : 3000;

      const [columnar, stateResult] = await Promise.all([
        API.fetchColumnarHistory({
          tf: window.currentTf,
          endTimeSec,
          limit,
          slots: resolveLiveSlotIds(),
          rsxSettings: DEFAULT_RSX_SETTINGS,
          symbol,
        }),
        API.fetchLiveState({ navigatorsOnly: true }),
      ]);
      if (reqId !== window.currentLiveRequestId) return;

      if (stateResult?.warmingUp) {
        setTimeout(loadDashboard, 2000);
        return;
      }

      if (!columnar?.times?.length || !liveColumnarStore) {
        console.warn('[Renaissance] no columnar history — chart idle');
        return;
      }

      liveColumnarStore.replaceMonolith(columnar);
      if (!liveColumnarStore.invariantOk()) {
        console.error('[Renaissance] ColumnarStore invariant failed', liveColumnarStore.invariantMeta());
        return;
      }

      window.historyHasMore = columnar.hasMore !== false;
      await mountDDRLiveCutover();

      beginDataUpdate();
      try {
        liveRenderScheduler?.markDirty({ mode: 'full', viewport: 'fresh' });
      } finally {
        endDataUpdate();
      }
    } catch (err) {
      console.error('[Renaissance] loadDashboard failed:', err);
    } finally {
      liveColumnarStore?.unseal?.();
      window.__isDashboardLoading = false;
      updateBufferingOverlay();
    }
  }

  function safeInit(label, fn) {
    try { fn(); } catch (err) { console.error(`[Renaissance] ${label}:`, err); }
  }

  function boot() {
    installGlobalShims();
    installChartAdapterShims();
    initLiveRenderPipeline();

    safeInit('DDR factory', initDDRFactory);
    safeInit('Hydration orchestrator', initHydrationOrchestrator);
    safeInit('UI strategy', () => StrategyController.init());
    safeInit('UI risk', () => RiskController.init());
    safeInit('UI rsx', () => RsxController.init());
    safeInit('UI tabs', () => TabsController.init());
    safeInit('UI timeframe', () => TimeframeController.init({ useServerTf: false }));
    safeInit('UI toolbar', () => ToolbarController.init());
    safeInit('UI layout', () => LayoutController.init());
    safeInit('UI navigator', () => NavigatorController.init());
    safeInit('UI backtest', () => BacktestController.init());

    (async () => {
      if (typeof LightweightCharts === 'undefined' || typeof ChartAdapter === 'undefined') {
        setTimeout(boot, 500);
        return;
      }
      if (!ChartAdapter.initLiveCharts()) {
        setTimeout(boot, 500);
        return;
      }

      attachLiveHistoryScrollArm();
      const priceChart = ChartAdapter.getChart('live', 'price');
      priceChart?.timeScale()?.subscribeVisibleLogicalRangeChange((range) => {
        scheduleHistoryLoad(range);
      });

      safeInit('UI wozduh', () => WozduhController.init());
      await loadDashboard();
      initLiveWebSocket();
      window.isAppInitialized = true;
    })().catch((err) => console.error('[Renaissance] boot async failed:', err));
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
})();
