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
  // Shot 11B: single discard axis for TF / history / WS / buffer (replaces requestId + historyEpoch).
  window.projectionEpoch = window.projectionEpoch ?? 0;
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

  let backendTradingTimeframe = null;
  let liveHistoryScrollArmed = false;
  let liveNavigatorResult = null;
  let liveHydrationOrchestrator = null;
  /** Monotonic token: only the latest RSX settings sync may commit store + paint. */
  let rsxSettingsSyncSeq = 0;
  /** Serializes overlapping syncRsxIndicatorSettings calls (rapid Save). */
  let rsxSettingsSyncTail = Promise.resolve();
  /** Debounce timer for live menu auto-apply (ADR-014 / B1). */
  let rsxSettingsSyncTimer = null;
  const RSX_SETTINGS_DEBOUNCE_MS = 200;
  /** Fingerprint of last server-applied live settings (Save no-ops when equal). */
  let rsxLastAppliedFingerprint = '';
  /** @type {AbortController | null} */
  let rsxPostAbort = null;
  /** @type {AbortController | null} */
  let rsxHistoryAbort = null;

  /** Shot 11B: bump ProjectionEpoch (SSOT discard axis). */
  function bumpProjectionEpoch() {
    window.projectionEpoch = (Number(window.projectionEpoch) || 0) + 1;
    return window.projectionEpoch;
  }

  function isCurrentEpoch(epoch) {
    return epoch === window.projectionEpoch;
  }

  function rsxSettingsFingerprint(settings) {
    if (!settings || typeof settings !== 'object') return '';
    const s = typeof coerceRsxSettingsForAPI === 'function'
      ? coerceRsxSettingsForAPI(settings)
      : settings;
    return JSON.stringify({
      length: s.length,
      signal_length: s.signal_length,
      source: s.source,
      div_method: s.div_method,
      pivot_radius: s.pivot_radius,
      div_lookback: s.div_lookback,
      min_price_delta_ratio: s.min_price_delta_ratio,
      min_osc_delta: s.min_osc_delta,
    });
  }

  // ── Shot 10B: Zero-gap live tick handoff while monolith history loads ──
  const LIVE_TICK_BUFFER_MAX = 5000;
  /** @type {object[]} */
  let pendingLiveTicks = [];
	let tickBufferActive = false;
	let tickBufferTf = '';
	let tickBufferEpoch = 0;
	/** Core 4.2 tip-handoff diagnostic (temporary): history tip vs first accepted live tick. */
	let handoffDiag = null;
	/** Core 4.5 Self-Healing: last gap-triggered reload (ms); throttles reload storms. */
	let lastGapHealAt = 0;
	const GAP_HEAL_COOLDOWN_MS = 10000;
	/** Debt #69A: throttle return-to-live hydrate after history-window prune. */
	let lastReturnToLiveAt = 0;
	const RETURN_TO_LIVE_COOLDOWN_MS = 2000;

  /** ADR-018 owner — constructed after helpers exist (see initTimelineRecovery). */
  let timelineRecovery = null;

  /** Core 4.5: bind current TF bar duration to the store so appendTick can detect gaps. */
  function syncStoreTfInterval() {
    if (!liveColumnarStore?.setTfInterval) return;
    const fn = typeof getIntervalMs === 'function'
      ? getIntervalMs
      : (typeof TimeNormalizer !== 'undefined' ? TimeNormalizer.getIntervalMs : null);
    const ms = fn ? Number(fn(window.currentTf)) : 0;
    liveColumnarStore.setTfInterval(Number.isFinite(ms) && ms > 0 ? Math.floor(ms / 1000) : 0);
  }

  function beginLiveTickBuffer() {
    pendingLiveTicks = [];
    tickBufferActive = true;
    tickBufferTf = String(window.currentTf || '');
    tickBufferEpoch = window.projectionEpoch;
  }

  function abortLiveTickBuffer() {
    tickBufferActive = false;
    pendingLiveTicks = [];
    tickBufferTf = '';
    tickBufferEpoch = 0;
  }

  /**
   * While buffer is active: absorb ticks (never write store).
   * Stale TF / superseded projectionEpoch ticks are discarded (not buffered).
   * @returns {boolean} true if caller must skip immediate store write
   */
  function bufferLiveTick(tick) {
    if (!tickBufferActive || !tick) return false;
    // Case-sensitive: "1m" ≠ "1M".
    const wantTf = String(window.currentTf || tickBufferTf || '');
    const tickTf = String(tick.timeframe || backendTradingTimeframe || wantTf || '');
    if (tickBufferEpoch !== window.projectionEpoch) {
      return true; // destroy stale-epoch tick
    }
    if (tickTf && wantTf && tickTf !== wantTf) {
      return true;
    }
    pendingLiveTicks.push(tick);
    if (pendingLiveTicks.length > LIVE_TICK_BUFFER_MAX) {
      pendingLiveTicks.splice(0, pendingLiveTicks.length - LIVE_TICK_BUFFER_MAX);
    }
    return true;
  }

  function resolveLiveRsxSettings() {
    if (typeof RsxController !== 'undefined' && typeof coerceRsxSettingsForAPI === 'function') {
      return coerceRsxSettingsForAPI(RsxController.getSettings('live'));
    }
    if (typeof coerceRsxSettingsForAPI === 'function' && typeof defaultRsxSettings === 'function') {
      return coerceRsxSettingsForAPI(defaultRsxSettings());
    }
    return {
      length: 14,
      signal_length: 9,
      source: 'hlc3',
      div_method: 'tv',
      pivot_radius: 2,
      div_lookback: 90,
      min_price_delta_ratio: 0,
      min_osc_delta: 0,
    };
  }

  async function pushRsxSettingsToServer(settings) {
    const payload = typeof coerceRsxSettingsForAPI === 'function'
      ? coerceRsxSettingsForAPI(settings)
      : settings;
    if (rsxPostAbort) rsxPostAbort.abort();
    rsxPostAbort = new AbortController();
    try {
      return await API.pushRsxSettings(payload, rsxPostAbort.signal);
    } catch (err) {
      if (err?.name === 'AbortError') return null;
      throw err;
    }
  }

  async function fetchRsxIndicatorSettings() {
    try {
      const result = await API.fetchRsxSettings();
      const serverSettings = result.settings || result;
      if (typeof RsxController === 'undefined') return;
      const showPivots = RsxController.getSettings('live')?.show_pivots;
      const applied = RsxController.setSettings('live', normalizeRsxSettingsFromAPI(
        { ...serverSettings, show_pivots: showPivots },
        defaultRsxSettings(),
      ));
      // Cache only — server is authoritative (ADR-012).
      RsxController.persist('live', applied);
      RsxController.applyToMenu('live', applied);
      rsxLastAppliedFingerprint = rsxSettingsFingerprint(applied);
    } catch (err) {
      console.warn('[Renaissance] fetch RSX settings failed:', err);
    }
  }

  /**
   * Debounced auto-apply (200ms). Save / outside-click call flushRsxSettingsSync.
   */
  function scheduleRsxSettingsSync(context = 'live') {
    if (context !== 'live') return;
    if (rsxSettingsSyncTimer) clearTimeout(rsxSettingsSyncTimer);
    rsxSettingsSyncTimer = setTimeout(() => {
      rsxSettingsSyncTimer = null;
      void syncRsxIndicatorSettings('live');
    }, RSX_SETTINGS_DEBOUNCE_MS);
  }

  /**
   * Flush pending debounce immediately. Returns false when already synchronized (no POST).
   */
  async function flushRsxSettingsSync(context = 'live') {
    if (context !== 'live') return false;
    if (rsxSettingsSyncTimer) {
      clearTimeout(rsxSettingsSyncTimer);
      rsxSettingsSyncTimer = null;
    }
    if (typeof RsxController === 'undefined') return false;
    const fromMenu = RsxController.readSettingsFromMenu
      ? RsxController.readSettingsFromMenu('live')
      : RsxController.syncFromMenu('live');
    const fp = rsxSettingsFingerprint(fromMenu);
    if (fp && fp === rsxLastAppliedFingerprint) {
      return false;
    }
    await syncRsxIndicatorSettings('live');
    return true;
  }

  async function syncRsxIndicatorSettings(context = 'live') {
    if (context !== 'live') {
      return typeof RsxController !== 'undefined' ? RsxController.syncFromMenu(context) : null;
    }
    if (typeof RsxController === 'undefined') return null;

    if (rsxSettingsSyncTimer) {
      clearTimeout(rsxSettingsSyncTimer);
      rsxSettingsSyncTimer = null;
    }

    const fromMenu = RsxController.syncFromMenu('live');
    const fp = rsxSettingsFingerprint(fromMenu);
    if (fp && fp === rsxLastAppliedFingerprint) {
      return RsxController.getSettings('live');
    }

    const seq = ++rsxSettingsSyncSeq;
    rsxSettingsSyncTail = rsxSettingsSyncTail
      .catch(() => {})
      .then(async () => {
        if (seq !== rsxSettingsSyncSeq) return;
        const result = await pushRsxSettingsToServer(fromMenu);
        if (seq !== rsxSettingsSyncSeq || result == null) return;
        const serverSettings = result?.settings || result;
        const applied = RsxController.setSettings('live', normalizeRsxSettingsFromAPI(
          { ...serverSettings, show_pivots: fromMenu.show_pivots },
          fromMenu,
        ));
        RsxController.persist('live', applied);
        RsxController.applyToMenu('live', applied);
        rsxLastAppliedFingerprint = rsxSettingsFingerprint(applied);
        // Soft indicator reload only when engine actually changed (ADR-014: no camera).
        if (result.changed !== false) {
          await reloadLiveForRsxSettings(seq);
        }
      });
    await rsxSettingsSyncTail;
    return RsxController.getSettings('live');
  }

  /**
   * Opt-in ADR-015 ProjCont probe (dormant). Enable via:
   *   localStorage.setItem('DEBUG_PROJ_CONT','1')  or  ?debug_proj_cont=1
   * Permanent: TransportDiag / Self-Healing / MemoryBudget stay always-on.
   */
  function debugProjContEnabled() {
    try {
      if (typeof window !== 'undefined' && window.DEBUG_PROJ_CONT === true) return true;
      if (typeof localStorage !== 'undefined' && localStorage.getItem('DEBUG_PROJ_CONT') === '1') return true;
      if (typeof location !== 'undefined' && /(?:\?|&)debug_proj_cont=1(?:&|$)/i.test(location.search || '')) return true;
    } catch (_) { /* ignore */ }
    return false;
  }

  /** ADR-015: one-shot first WS after soft settings apply (only when DEBUG_PROJ_CONT). */
  let projContPending = null;

  function tipRSXFromPlotMap(plots) {
    if (!plots || typeof plots !== 'object') return null;
    const raw = plots.line_rsx ?? plots.jurik_rsx;
    if (Array.isArray(raw)) {
      if (!raw.length) return null;
      const v = Number(raw[raw.length - 1]);
      return Number.isFinite(v) ? v : null;
    }
    const v = Number(raw);
    return Number.isFinite(v) ? v : null;
  }

  function storeTipProbe(label) {
    if (!debugProjContEnabled()) return null;
    const store = liveColumnarStore;
    if (!store) return null;
    const snap = typeof store.snapshot === 'function' ? store.snapshot() : null;
    const plots = snap?.plots || store._plots || {};
    const timesLen = store.barCount?.() ?? store._times?.length ?? 0;
    const lastOpen = store.lastTimeSec?.() ?? null;
    const lastRSX = tipRSXFromPlotMap(plots);
    const plotsLen = Array.isArray(plots.line_rsx)
      ? plots.line_rsx.length
      : (Array.isArray(plots.jurik_rsx) ? plots.jurik_rsx.length : 0);
    const out = { label, timesLen, plotsLen, lastOpen, lastRSX };
    console.log('[ProjCont] FE store', out);
    return out;
  }

  function armProjContFirstWS(restDiag) {
    if (!debugProjContEnabled()) return;
    projContPending = {
      armedAt: Date.now(),
      rest: restDiag,
      healingAtArm: !!(timelineRecovery?.isHealing?.()),
    };
  }

  function skipProjContADR015(reason, extra) {
    if (!projContPending) return;
    const pending = projContPending;
    projContPending = null;
    if (!debugProjContEnabled()) return;
    console.log('[ProjCont] ADR015 skipped', {
      reason,
      elapsedMs: Date.now() - pending.armedAt,
      restLastOpen: pending.rest?.lastOpenSec,
      restProjectionMode: pending.rest?.projectionMode,
      ...extra,
    });
  }

  function maybeLogProjContFirstWS(tick) {
    if (!projContPending || !debugProjContEnabled()) {
      projContPending = null;
      return;
    }
    const pending = projContPending;
    const wsOpen = Number(tick?.time);
    const wsRSX = tipRSXFromPlotMap(tick?.plots)
      ?? (Number.isFinite(Number(tick?.plots?.line_rsx)) ? Number(tick.plots.line_rsx) : null);
    const rest = pending.rest || {};
    const elapsedMs = Date.now() - pending.armedAt;
    const ADR015_MAX_MS = 2000;
    const healing = !!(timelineRecovery?.isHealing?.()) || !!window.__isDashboardLoading;
    const sameOpen = Number(rest.lastOpenSec) === wsOpen;

    if (healing || pending.healingAtArm) {
      skipProjContADR015('timeline heal', { wsOpen, wsRSX });
      return;
    }
    if (!sameOpen) {
      skipProjContADR015('new bar', { wsOpen, restLastOpen: rest.lastOpenSec, wsRSX });
      return;
    }
    if (elapsedMs > ADR015_MAX_MS) {
      skipProjContADR015('elapsed', { wsOpen, wsRSX, elapsedMs, maxMs: ADR015_MAX_MS });
      return;
    }

    projContPending = null;
    const restRSX = Number(rest.lastRSX);
    const delta = (Number.isFinite(restRSX) && Number.isFinite(wsRSX)) ? (wsRSX - restRSX) : null;
    const storeAfter = storeTipProbe('after_first_ws');
    console.log('[ProjCont] first WS after soft apply', {
      restTimesLen: rest.timesLen,
      restProjectedForming: rest.projectedForming,
      restProjectionMode: rest.projectionMode,
      restLastOpen: rest.lastOpenSec,
      restLastRSX: rest.lastRSX,
      restFrameCurOpen: rest.frameCurOpenSec,
      restFrameCurRSX: rest.frameCurRSX,
      wsOpen,
      wsRSX,
      openMatch: true,
      deltaRSX: delta,
      identical: delta != null && Math.abs(delta) < 1e-9,
      storeAfter,
      elapsedMs,
    });
  }

  /**
   * Soft indicator reload (ADR-014 + ADR-015 / B2.1).
   * Atomic applyProjection(snapshot) — never plots-only updatePlots (lost forming tip).
   * Camera preserved via ViewportManager capture/restore — never viewport:fresh.
   */
  async function reloadLiveForRsxSettings(seq) {
    if (seq !== rsxSettingsSyncSeq) return;
    if (!liveColumnarStore || !ChartAdapter.isInitialized('live')) return;

    window.__isSettingsUpdating = true;
    let completed = false;
    try {
      if (window.DDRFactory && !window.DDRFactory.manifest) {
        await window.DDRFactory.fetchManifest();
      }
      if (seq !== rsxSettingsSyncSeq) return;

      const symbol = document.getElementById('symbol')?.textContent?.trim() || '';
      const endTimeSec = liveColumnarStore.lastTimeSec() ?? Math.floor(Date.now() / 1000);
      const limit = Math.max(liveColumnarStore.barCount() || 0, 3000);

      if (rsxHistoryAbort) rsxHistoryAbort.abort();
      rsxHistoryAbort = new AbortController();

      const columnar = await API.fetchColumnarHistory({
        tf: window.currentTf,
        endTimeSec,
        limit,
        slots: resolveLiveSlotIds(),
        rsxSettings: resolveLiveRsxSettings(),
        symbol,
        signal: rsxHistoryAbort.signal,
      });
      if (seq !== rsxSettingsSyncSeq) return;
      if (!columnar?.plots || typeof columnar.plots !== 'object') {
        console.warn('[Renaissance] RSX settings sync — empty plots');
        return;
      }
      if (!Array.isArray(columnar.times) || !columnar.times.length) {
        console.warn('[Renaissance] RSX settings sync — empty times (projection incomplete)');
        return;
      }

      const restDiag = columnar.projCont || {
        closedBars: null,
        projectedForming: null,
        timesLen: columnar.times.length,
        plotsLen: Array.isArray(columnar.plots?.line_rsx) ? columnar.plots.line_rsx.length : null,
        lastOpenSec: columnar.times[columnar.times.length - 1],
        lastRSX: tipRSXFromPlotMap(columnar.plots),
      };
      if (debugProjContEnabled()) {
        console.log('[ProjCont] REST history', restDiag);
      }
      const storeBefore = storeTipProbe('before_applyProjection');

      const viewportAnchor = (typeof ViewportManager !== 'undefined' && ViewportManager.capture)
        ? ViewportManager.capture('live')
        : null;

      beginDataUpdate();
      try {
        // B2.1: atomic ProjectionSnapshot (times + OHLC + plots). Server owns length.
        if (typeof liveColumnarStore.applyProjection === 'function') {
          liveColumnarStore.applyProjection(columnar);
        } else {
          liveColumnarStore.replaceMonolith(columnar);
        }
        if (!liveColumnarStore.invariantOk()) {
          console.error('[Renaissance] RSX settings sync — invariant failed', liveColumnarStore.invariantMeta());
          return;
        }
        completed = true;
        if (debugProjContEnabled()) {
          const storeAfter = storeTipProbe('after_applyProjection');
          const lostProjection = Number(restDiag.timesLen) > Number(storeAfter?.timesLen);
          const tipOpenLost = Number(restDiag.lastOpenSec) !== Number(storeAfter?.lastOpen)
            && restDiag.projectedForming === true;
          const tipRSXMatch = Number.isFinite(Number(restDiag.lastRSX))
            && Number.isFinite(Number(storeAfter?.lastRSX))
            && Math.abs(Number(restDiag.lastRSX) - Number(storeAfter.lastRSX)) < 1e-9;
          console.log('[ProjCont] soft apply verdict', {
            lostProjection,
            tipOpenLost,
            tipRSXMatch,
            restTimesLen: restDiag.timesLen,
            storeTimesLen: storeAfter?.timesLen,
            restLastOpen: restDiag.lastOpenSec,
            storeLastOpen: storeAfter?.lastOpen,
            restLastRSX: restDiag.lastRSX,
            storeLastRSX: storeAfter?.lastRSX,
            storeBefore,
          });
          armProjContFirstWS(restDiag);
        }
        // ADR-014: never viewport:fresh — restore prior camera if capture succeeded.
        liveRenderScheduler?.markDirty({
          mode: 'full',
          viewport: viewportAnchor ? 'restore' : 'preserve',
          anchor: viewportAnchor || undefined,
        });
      } finally {
        endDataUpdate();
      }
    } catch (err) {
      if (err?.name === 'AbortError') return;
      console.error('[Renaissance] syncRsxIndicatorSettings failed:', err);
    } finally {
      if (seq === rsxSettingsSyncSeq) {
        window.__isSettingsUpdating = false;
      }
      void completed;
    }
  }

  /**
   * Core 4.9: honest TF → ms parser, used only when no TimeNormalizer/global getIntervalMs
   * exists yet. Mirrors TimeNormalizer.getIntervalMs so the shim never lies to gap detection.
   */
  function parseTfIntervalMs(tf) {
    const m = /^(\d+)([a-zA-Z])$/.exec(String(tf || '').trim());
    if (!m) return 60000;
    const val = Number(m[1]);
    if (!Number.isFinite(val) || val <= 0) return 60000;
    switch (m[2]) {
      case 's': return val * 1000;
      case 'm': return val * 60000;
      case 'h': return val * 3600000;
      case 'd': return val * 86400000;
      case 'w': return val * 604800000;
      case 'M': return val * 2592000000; // 30-day month (case-sensitive: "M" ≠ "m")
      default: return 60000;
    }
  }

  function installGlobalShims() {
    const fns = {
      loadDashboard,
      reloadDashboard,
      clearChartData,
      prepareLiveTfHandoff,
      wsSubscribeTf,
      startLivePollTimer: noop,
      isOrderFlowTf: () => false,
      pollOrderFlowState: noopAsync,
      updateBufferingOverlay,
      handleBacktestIntervalChange: noop,
      getBacktestInterval: () => window.backtestTf,
      abortLiveStateFetch: noop,
      disarmLiveHistoryScroll,
      openFloatingMenu: (menu, anchor) => {
        if (window.FloatingMenu?.open) return window.FloatingMenu.open(menu, anchor);
      },
      initFloatingMenuDrag: (menu) => {
        if (window.FloatingMenu?.initDrag) return window.FloatingMenu.initDrag(menu);
      },
      buildFinalBacktestPayload: () => window.currentBacktestPayload || {},
      getActiveUiContext: () => 'live',
      shouldPaintLiveChart: () => TabsController?.isLiveTabActive?.() !== false,
      runBacktest: noopAsync,
      stopBacktest: noop,
      syncRsxIndicatorSettings,
      pushRsxSettingsToServer,
      reloadRsxChartFromServer: async () => {
        await reloadLiveForRsxSettings(rsxSettingsSyncSeq);
      },
      fetchRsxIndicatorSettings,
      scheduleRsxSettingsSync,
      flushRsxSettingsSync,
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
      getIntervalMs: typeof getIntervalMs === 'function' ? getIntervalMs : parseTfIntervalMs,
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
      onAfterFlush: () => {
        updateBufferingOverlay();
      },
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
    if (typeof ToolbarController === 'undefined') return;
    // Dashboard hydrate only — timeline heal uses #timeline-sync-badge (ADR-018).
    ToolbarController.setBuffering(!!window.__isDashboardLoading);
  }

  function enterTimelineHealing(reason) {
    if (timelineRecovery) {
      timelineRecovery.enter(reason);
      return;
    }
    // Fallback if script failed to load: buffer ticks only.
    beginLiveTickBuffer();
  }

  function onTimelineHealingFromServer() {
    enterTimelineHealing('server_timeline_healing');
  }

  function onTimelinePublishableFromServer() {
    if (timelineRecovery) {
      timelineRecovery.publishable();
      return;
    }
    if (window.__isDashboardLoading) return;
    loadDashboard();
  }

  function initTimelineRecovery() {
    if (typeof TimelineRecovery === 'undefined' || !TimelineRecovery.create) {
      console.warn('[Renaissance] TimelineRecovery module missing — heal UX degraded');
      return;
    }
    timelineRecovery = TimelineRecovery.create({
      watchdogMs: 25_000,
      onEnter() {
        if (projContPending) {
          skipProjContADR015('timeline heal', { healReason: 'enter' });
        }
        beginLiveTickBuffer();
      },
      onRecovered() {
        if (window.__isDashboardLoading) return;
        loadDashboard();
      },
    });
  }

  function wsSubscribeTf(tf) {
    if (typeof WS !== 'undefined') WS.subscribe(tf, tf);
  }

  function clearChartData(options = {}) {
    bumpProjectionEpoch();
    abortLiveTickBuffer();
    liveHydrationOrchestrator?.reset();
    disarmLiveHistoryScroll();
    // Shot 11C: keepProjection leaves LWC + store visible until Atomic Swap replaceMonolith/paint.
    if (!options.keepProjection) {
      liveColumnarStore?.clear();
      window.DDRFactory?.clear?.();
    }
    liveNavigatorResult = null;
    window.historyHasMore = true;
    window.isLoadingHistory = false;
  }

  /**
   * Soft TF handoff (Shot 11C): epoch/buffer/hydration only — no store/DDR wipe, no setData([]).
   * Old candles stay on screen under the buffering overlay until one full paint swaps the frame.
   * Caller must set window.currentTf before this so the buffer binds the new TF.
   */
  function prepareLiveTfHandoff() {
    bumpProjectionEpoch();
    syncStoreTfInterval();
    abortLiveTickBuffer();
    beginLiveTickBuffer();
    liveHydrationOrchestrator?.reset();
    disarmLiveHistoryScroll();
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
      rsx: { chart: rsx, defaultPriceScaleId: 'right' },
      wozduh: { chart: wozduh, defaultPriceScaleId: 'right' },
    }, window.DDRFactory.manifest.panes);
    if (typeof SettingsRenderer !== 'undefined') SettingsRenderer.refreshFromManifest();
    if (typeof LegendRenderer !== 'undefined') {
      LegendRenderer.mountFromManifest(window.DDRFactory.manifest);
    }
    // ADR-019: PaneLayout SSOT + Ind; LayoutController applies CSS Grid from state.
    if (typeof PaneLayout !== 'undefined') {
      if (!window.paneLayout) window.paneLayout = PaneLayout.create();
      window.paneLayout.init({ manifest: window.DDRFactory.manifest });
      window.paneLayout.mountIndMenu();
      if (typeof LayoutController !== 'undefined' && LayoutController.attach) {
        LayoutController.attach(window.paneLayout);
      }
    }
    return true;
  }

  function initHydrationOrchestrator() {
    if (typeof HydrationOrchestrator === 'undefined') return;
    liveHydrationOrchestrator = new HydrationOrchestrator();
    liveHydrationOrchestrator.init({
      getEpoch: () => window.projectionEpoch,
      getReqId: () => window.projectionEpoch,
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
          rsxSettings: resolveLiveRsxSettings(),
          symbol,
        });
      },
      mergeIntoStore: (data) => {
        const viewportRange = ChartAdapter.getVisibleLogicalRange('live');
        const cap = (typeof ViewportManager !== 'undefined' && ViewportManager.capture)
          ? ViewportManager.capture('live')
          : null;
        const focalTimeSec = (cap?.centerTimeMs != null && Number.isFinite(cap.centerTimeMs))
          ? cap.centerTimeMs / 1000
          : null;
        const { added } = liveColumnarStore.prependMonolith(data, {
          focalTimeSec,
          atLiveEdge: cap?.isAtRightEdge === true,
        });
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
    if (window.isUpdatingData || liveHydrationOrchestrator?.isBusy() || liveRenderScheduler?.isBusy()) return;
    liveHydrationOrchestrator?.schedulePrepend(range);
  }

  /** Debt #69A: if history-window tip was pruned, pinning right must hydrate from server — not scroll empty space. */
  function maybeReturnToLiveFromHistory(range) {
    if (!liveColumnarStore || liveColumnarStore.windowMode !== 'history') return;
    if (window.__isDashboardLoading || liveColumnarStore.isSealed?.()) return;
    const barCount = liveColumnarStore.barCount?.() ?? 0;
    if (!range || barCount <= 0) return;
    const slack = 2;
    const atRight = range.to >= (barCount - 1 - slack);
    if (!atRight) return;
    const now = Date.now();
    if (now - lastReturnToLiveAt < RETURN_TO_LIVE_COOLDOWN_MS) return;
    lastReturnToLiveAt = now;
    console.info('[MemoryBudget] Return to live from history window — loadDashboard()');
    loadDashboard();
  }

  /**
   * Debt #69A emergency restore: HTF server cache + FE store clear + canonical hydrate.
   * Not a memory manager — user-facing "Reload Dashboard".
   */
  async function reloadDashboard() {
    try {
      await fetch('/api/cache/clear', { method: 'POST' });
    } catch (err) {
      console.warn('[Reload Dashboard] HTF cache clear failed:', err);
    }
    liveColumnarStore?.clear?.();
    await loadDashboard();
  }

  function pushLiveTickDelta(tick, options = {}) {
    if (!liveColumnarStore || !liveRenderScheduler || liveColumnarStore.isSealed()) return false;
    // Debt #69A: history display window must not ingest live ticks (avoids gap-heal yank-to-live).
    if (liveColumnarStore.windowMode === 'history') return false;
    const appendResult = liveColumnarStore.appendTick(tick);
    if (appendResult?.gapDetected) {
      if (liveColumnarStore.windowMode === 'history') return false;
      console.warn('[Self-Healing] Time gap detected — waiting for server heal', {
        lastTime: appendResult.lastTime,
        tickTime: appendResult.tickTime,
        timeframe: tick?.timeframe || window.currentTf,
      });
      const now = Date.now();
      // Throttle: do not storm beginAwait; backend ingest gap / reconnect drives heal.
      if (!window.__isDashboardLoading && now - lastGapHealAt > GAP_HEAL_COOLDOWN_MS) {
        lastGapHealAt = now;
        enterTimelineHealing('fe_gapDetected');
      }
      return false;
    }
    if (!appendResult?.candle) return false;
    maybeLogProjContFirstWS(tick);
    if (handoffDiag?.waiting) {
      const tip = Number(handoffDiag.historyTipOpen);
      const first = Number(tick?.time);
      console.log('[TransportDiag] tip handoff', {
        historyTipOpen: tip,
        firstAcceptedTick: first,
        deltaSec: Number.isFinite(tip) && Number.isFinite(first) ? first - tip : null,
        timeframe: tick?.timeframe || window.currentTf,
      });
      handoffDiag.waiting = false;
    }
    if (options.silent) return true;
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

  /** Replay buffered ticks onto the fresh monolith (TF + projectionEpoch gated). */
  function flushLiveTickBuffer() {
    tickBufferActive = false;
    const pending = pendingLiveTicks;
    pendingLiveTicks = [];
    const wantTf = String(window.currentTf || tickBufferTf || '');
    const epoch = tickBufferEpoch;
    tickBufferTf = '';
    tickBufferEpoch = 0;
    if (!pending.length) return;
    for (let i = 0; i < pending.length; i++) {
      if (!isCurrentEpoch(epoch)) break;
      const tick = pending[i];
      const tickTf = String(tick?.timeframe || backendTradingTimeframe || wantTf || '');
      if (tickTf && wantTf && tickTf !== wantTf) continue;
      // Silent: store only — caller issues one full paint after handoff.
      pushLiveTickDelta(tick, { silent: true });
    }
  }

  function handleLiveTick(d) {
    // Core 4.2 Bouncer: case-sensitive TF assert ("1m" ≠ "1M") before buffer / queueTick / store.
    const wantTf = String(window.currentTf || '');
    const gotTf = String(d?.timeframe || '');
    if (!d || !gotTf || !wantTf || gotTf !== wantTf) return;
    const epoch = window.projectionEpoch;
    // Shot 10B: absorb ticks during monolith load (never write store until flush).
    if (bufferLiveTick(d)) return;
    if (!isCurrentEpoch(epoch)) return;
    if (liveHydrationOrchestrator?.queueTick(d)) return;
    if (!isCurrentEpoch(epoch)) return;
    if (!ChartAdapter.isInitialized('live')) return;
    if (typeof chartTime === 'function' && chartTime(d.time) == null) return;
    if (!isCurrentEpoch(epoch)) return;
    pushLiveTickDelta(d);
  }

  function initLiveWebSocket() {
    if (typeof WS === 'undefined') return;
    WS.connect({
      onTick: handleLiveTick,
      onMarker: noop,
      onOpen: () => wsSubscribeTf(window.currentTf),
      onTimelineHealing: onTimelineHealingFromServer,
      onTimelinePublishable: onTimelinePublishableFromServer,
      // Browser↔bot reconnect ≠ Binance heal. Buffer + short safety; publishable wins if both drop.
      onReconnect: () => {
        console.warn('[Self-Healing] browser WS reconnected — entering timeline recovery');
        enterTimelineHealing('browser_ws_reconnect');
      },
    });
  }

  async function loadDashboard(options = {}) {
    const viewportAnchor = options.viewportAnchor ?? null;
    const epoch = bumpProjectionEpoch();
    if (!ChartAdapter.isInitialized('live') && !ChartAdapter.initLiveCharts()) {
      setTimeout(() => loadDashboard(options), 500);
      return;
    }

    // Core 4.5: bind TF interval before any tick can reach the store (gap detection axis).
    syncStoreTfInterval();

    // Shot 10B: absorb WS ticks until monolith + replay.
    // WarmingUp retries must keep the buffer (do not drop ticks from the wait window).
    const wantTf = String(window.currentTf || '');
    if (tickBufferActive && tickBufferTf === wantTf) {
      tickBufferEpoch = window.projectionEpoch;
    } else {
      beginLiveTickBuffer();
    }
    window.__isDashboardLoading = true;
    wsSubscribeTf(window.currentTf);
    if (typeof ToolbarController !== 'undefined') ToolbarController.setBuffering(true);

    let completed = false;
    let retrying = false;
    try {
      if (window.DDRFactory && !window.DDRFactory.manifest) {
        await window.DDRFactory.fetchManifest();
      }
      if (!isCurrentEpoch(epoch)) return;

      const symbol = document.getElementById('symbol')?.textContent?.trim() || '';
      const endTimeSec = Math.floor(Date.now() / 1000);
      const limit = typeof HISTORY_CHUNK_LIMIT !== 'undefined' ? HISTORY_CHUNK_LIMIT : 3000;

      const [columnar, stateResult] = await Promise.all([
        API.fetchColumnarHistory({
          tf: window.currentTf,
          endTimeSec,
          limit,
          slots: resolveLiveSlotIds(),
          rsxSettings: resolveLiveRsxSettings(),
          symbol,
        }),
        API.fetchLiveState({ navigatorsOnly: true }),
      ]);
      if (!isCurrentEpoch(epoch)) return;

      if (stateResult?.warmingUp) {
        retrying = true;
        setTimeout(() => loadDashboard(options), 2000);
        return;
      }

      if (!columnar?.times?.length || !liveColumnarStore) {
        console.warn('[Renaissance] no columnar history — chart idle');
        return;
      }

      // Shot 11C Atomic Swap: mutate store + ensure DDR hosts offline, then ONE full paint.
      liveColumnarStore.replaceMonolith(columnar);
      const histTimes = Array.isArray(columnar.times) ? columnar.times : [];
      const historyTipOpen = histTimes.length ? Number(histTimes[histTimes.length - 1]) : null;
      handoffDiag = { historyTipOpen, waiting: true };
      console.log('[TransportDiag] history loaded', {
        historyTipOpen,
        timeframe: window.currentTf,
        bars: histTimes.length,
      });
      flushLiveTickBuffer();
      if (!liveColumnarStore.invariantOk()) {
        console.error('[Renaissance] ColumnarStore invariant failed', liveColumnarStore.invariantMeta());
        return;
      }

      window.historyHasMore = columnar.hasMore !== false;
      await mountDDRLiveCutover();
      if (!isCurrentEpoch(epoch)) return;

      completed = true;
      beginDataUpdate();
      try {
        liveRenderScheduler?.markDirty({
          mode: 'full',
          viewport: viewportAnchor ? 'restore' : 'fresh',
          anchor: viewportAnchor,
        });
      } finally {
        endDataUpdate();
      }
    } catch (err) {
      console.error('[Renaissance] loadDashboard failed:', err);
    } finally {
      liveColumnarStore?.unseal?.();
      if (!isCurrentEpoch(epoch)) {
        abortLiveTickBuffer();
        window.__isDashboardLoading = false;
        updateBufferingOverlay();
      } else if (retrying) {
        // Keep buffer + loading flag across warmingUp retry.
        window.__isDashboardLoading = true;
      } else {
        if (!completed) abortLiveTickBuffer();
        window.__isDashboardLoading = false;
        updateBufferingOverlay();
      }
    }
  }

  function safeInit(label, fn) {
    try { fn(); } catch (err) { console.error(`[Renaissance] ${label}:`, err); }
  }

  function boot() {
    installGlobalShims();
    installChartAdapterShims();
    initLiveRenderPipeline();
    initTimelineRecovery();

    safeInit('DDR factory', initDDRFactory);
    safeInit('Hydration orchestrator', initHydrationOrchestrator);
    safeInit('UI strategy', () => StrategyController.init());
    safeInit('UI risk', () => RiskController.init());
    safeInit('UI rsx', () => {
      RsxController.init();
      RsxController.onSettingsChanged(() => scheduleRsxSettingsSync('live'));
      if (window.FloatingMenu?.setBeforeOutsideClose) {
        FloatingMenu.setBeforeOutsideClose(async (menu) => {
          if (!menu?.closest?.('.rsx-wrap')) return;
          await flushRsxSettingsSync('live');
        });
      }
    });
    safeInit('UI tabs', () => TabsController.init());
    safeInit('UI timeframe', () => TimeframeController.init({ useServerTf: false }));
    safeInit('UI toolbar', () => ToolbarController.init());
    safeInit('UI scale', () => ScaleController.init());
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

      // ADR-012: engine SSOT hydrates menu before first viewport paint.
      await fetchRsxIndicatorSettings();

      attachLiveHistoryScrollArm();
      const priceChart = ChartAdapter.getChart('live', 'price');
      priceChart?.timeScale()?.subscribeVisibleLogicalRangeChange((range) => {
        scheduleHistoryLoad(range);
        maybeReturnToLiveFromHistory(range);
      });

      safeInit('UI wozduh', () => WozduhController.init());
      // Shot 10B: open WS before history fetch so ticks buffer during load (no Startup Gap).
      initLiveWebSocket();
      await loadDashboard();
      window.isAppInitialized = true;
    })().catch((err) => console.error('[Renaissance] boot async failed:', err));
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
})();
