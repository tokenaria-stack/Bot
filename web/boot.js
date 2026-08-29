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
  window.historyHasNewer = window.historyHasNewer ?? true;
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
	/** MICRO-2C: last sparse TimeCamera intent (LIVE|HISTORY); dense path leaves it null. */
	let sparseViewIntent = null;
	let lastGapHealAt = 0;
	const GAP_HEAL_COOLDOWN_MS = 10000;
	/** Sparse-child source watermark (1s OpenTime sec). 0 = next right page is startTime only. */
	let sparseParentResumeAfterSec = 0;

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
    if (typeof liveColumnarStore.setDenseContinuity === 'function') {
      const dense = typeof requiresDenseTimeContinuity === 'function'
        ? requiresDenseTimeContinuity(window.currentTf)
        : true;
      liveColumnarStore.setDenseContinuity(dense);
    }
  }

  /**
   * Track A / WS-01…WS-03: capture committed VIEW time bounds before a preserve-paired mutation.
   * @returns {{ viewFromSec: number, viewToSec: number }|null}
   */
  function captureStoreViewTimes() {
    if (!liveColumnarStore || typeof ColumnarStore === 'undefined'
      || typeof ColumnarStore.logicalRangeToViewTimes !== 'function') {
      return null;
    }
    if (typeof ChartAdapter === 'undefined' || typeof ChartAdapter.getVisibleLogicalRange !== 'function') {
      return null;
    }
    return ColumnarStore.logicalRangeToViewTimes(
      liveColumnarStore.timesSec?.(),
      ChartAdapter.getVisibleLogicalRange('live'),
    );
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
    if (typeof appendLiveTickBuffer === 'function') {
      appendLiveTickBuffer(pendingLiveTicks, tick, LIVE_TICK_BUFFER_MAX, wantTf);
    } else {
      pendingLiveTicks.push(tick);
      if (pendingLiveTicks.length > LIVE_TICK_BUFFER_MAX) {
        pendingLiveTicks.splice(0, pendingLiveTicks.length - LIVE_TICK_BUFFER_MAX);
      }
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
      // Preserve-paired (ADR-014 restore): budget must not amputate VIEW.
      const viewTimes = captureStoreViewTimes();

      beginDataUpdate();
      try {
        // B2.1: atomic ProjectionSnapshot (times + OHLC + plots). Server owns length.
        if (typeof liveColumnarStore.applyProjection === 'function') {
          liveColumnarStore.applyProjection(columnar, {
            viewFromSec: viewTimes?.viewFromSec,
            viewToSec: viewTimes?.viewToSec,
          });
        } else {
          liveColumnarStore.replaceMonolith(columnar, {
            viewFromSec: viewTimes?.viewFromSec,
            viewToSec: viewTimes?.viewToSec,
          });
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
      returnToLive,
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
      toggleRuler: () => (typeof ChartAdapter !== 'undefined' && ChartAdapter.toggleRuler
        ? ChartAdapter.toggleRuler()
        : undefined),
      resetRuler: () => (typeof ChartAdapter !== 'undefined' && ChartAdapter.resetRuler
        ? ChartAdapter.resetRuler()
        : undefined),
      setRulerCursor: (active) => {
        if (typeof ChartAdapter !== 'undefined' && ChartAdapter.setRulerCursor) {
          ChartAdapter.setRulerCursor(active);
        }
      },
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
      syncVisibleLogicalRange: noop,
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
        // Service existing HUMAN pending only. Paint must not invent another page.
        queueMicrotask(() => {
          liveHydrationOrchestrator?.tryConsumePending?.();
        });
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
    // Wave 2: full-hydrate busy ended — pending left-history may proceed.
    liveHydrationOrchestrator?.tryConsumePending?.();
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
    if (typeof isSparseLiveChart === 'function' && isSparseLiveChart(window.currentTf)) {
      return;
    }
    if (timelineRecovery) {
      timelineRecovery.enter(reason);
      return;
    }
    // Fallback if script failed to load: buffer ticks only.
    beginLiveTickBuffer();
  }

  function onTimelineHealingFromServer() {
    if (typeof isSparseLiveChart === 'function' && isSparseLiveChart(window.currentTf)) {
      return;
    }
    enterTimelineHealing('server_timeline_healing');
  }

  function onTimelinePublishableFromServer() {
    if (typeof isSparseLiveChart === 'function' && isSparseLiveChart(window.currentTf)) {
      // Clear leftover dense HEALING if the user switched TF; never hydrate 1s from Master.
      timelineRecovery?.publishable?.();
      return;
    }
    if (timelineRecovery) {
      timelineRecovery.publishable();
      return;
    }
    if (window.__isDashboardLoading) return;
    loadDashboard();
  }

  function captureReconnectViewportAnchor() {
    if (typeof ViewportManager === 'undefined' || typeof ViewportManager.capture !== 'function') {
      return null;
    }
    const captured = ViewportManager.capture('live');
    if (!captured) return null;
    if (typeof ViewportManager.cameraIntentForTfSwitch === 'function') {
      return ViewportManager.cameraIntentForTfSwitch(captured);
    }
    return captured;
  }

  function onBrowserReconnect() {
    if (typeof isSparseLiveChart === 'function' && isSparseLiveChart(window.currentTf)) {
      const viewportAnchor = captureReconnectViewportAnchor();
      if (!tickBufferActive) beginLiveTickBuffer();
      loadDashboard({ viewportAnchor, quiet: true });
      return;
    }
    console.warn('[Self-Healing] browser WS reconnected — entering timeline recovery');
    enterTimelineHealing('browser_ws_reconnect');
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
        if (typeof isSparseLiveChart === 'function' && isSparseLiveChart(window.currentTf)) {
          return;
        }
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
    window.historyHasNewer = true;
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
    sparseViewIntent = null;
    liveHydrationOrchestrator?.reset();
    disarmLiveHistoryScroll();
    liveNavigatorResult = null;
    window.historyHasMore = true;
    window.historyHasNewer = true;
    window.isLoadingHistory = false;
    sparseParentResumeAfterSec = 0;
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

  function liveTfIntervalSec() {
    const intervalFn = typeof getIntervalMs === 'function'
      ? getIntervalMs
      : (typeof TimeNormalizer !== 'undefined' ? TimeNormalizer.getIntervalMs : parseTfIntervalMs);
    const intervalMs = Number(intervalFn(window.currentTf));
    return Number.isFinite(intervalMs) && intervalMs > 0
      ? Math.floor(intervalMs / 1000)
      : 60;
  }

  function isSecondsHistoryNavChart(tf) {
    const id = tf || window.currentTf;
    return (typeof isLiveSecondChart === 'function' && isLiveSecondChart(id))
      || (typeof isSparseSecondChart === 'function' && isSparseSecondChart(id));
  }

  /** Child-tip CloseTime in unix seconds. 1s: same as OpenTime. Do not reuse child OpenTime as right cursor. */
  function sparseChildRightCursorSec(openSec) {
    const open = Number(openSec);
    if (!Number.isFinite(open) || open <= 0) return null;
    if (typeof isSparseSecondChart === 'function' && isSparseSecondChart(window.currentTf)) {
      const iv = liveTfIntervalSec();
      return open + iv - 1;
    }
    return open;
  }

  /** Store tip is behind wall-clock — Microscope island can extend toward live. */
  function canExtendHistoryRight() {
    const last = liveColumnarStore?.lastTimeSec?.();
    if (!Number.isFinite(last) || last <= 0) return false;
    if (isSecondsHistoryNavChart(window.currentTf)) {
      return window.historyHasNewer !== false;
    }
    if (typeof requiresDenseTimeContinuity === 'function'
      && !requiresDenseTimeContinuity(window.currentTf)) {
      return false;
    }
    const nowSec = Math.floor(Date.now() / 1000);
    const iv = liveTfIntervalSec();
    return last + iv < nowSec;
  }

  /** Visible range within zoom-aware prefetch runway of the newest loaded bar. */
  function isApproachingLoadedRightEdge(range) {
    if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to)) return false;
    const n = liveColumnarStore?.barCount?.() ?? 0;
    if (n <= 0) return false;
    const tip = n - 1;
    const hardMin = typeof LIVE_HISTORY_SCROLL_THRESHOLD !== 'undefined'
      ? LIVE_HISTORY_SCROLL_THRESHOLD
      : 50;
    const frac = typeof HISTORY_EDGE_PREFETCH_FRAC !== 'undefined'
      ? HISTORY_EDGE_PREFETCH_FRAC
      : 0.25;
    if (typeof ViewportManager !== 'undefined'
      && typeof ViewportManager.isWithinRightEdgePrefetch === 'function') {
      return ViewportManager.isWithinRightEdgePrefetch(range, tip, { hardMin, frac });
    }
    return Number(range.to) >= tip - hardMin;
  }

  /** Visible range within zoom-aware prefetch runway of the loaded head (left). */
  function isApproachingLoadedLeftEdge(range) {
    if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to)) return false;
    const hardMin = typeof LIVE_HISTORY_SCROLL_THRESHOLD !== 'undefined'
      ? LIVE_HISTORY_SCROLL_THRESHOLD
      : 50;
    const frac = typeof HISTORY_EDGE_PREFETCH_FRAC !== 'undefined'
      ? HISTORY_EDGE_PREFETCH_FRAC
      : 0.25;
    if (typeof ViewportManager !== 'undefined'
      && typeof ViewportManager.isWithinLeftEdgePrefetch === 'function') {
      return ViewportManager.isWithinLeftEdgePrefetch(range, { hardMin, frac });
    }
    return Number(range.from) < hardMin;
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
      shouldLoad: (range, options = {}) => {
        // Need validity only (not busy). Busy → pending inside Hydration (Wave 2).
        if (!ChartAdapter.isInitialized('live')) return false;
        if (!window.historyHasMore) return false;
        if (!range || (liveColumnarStore?.barCount?.() ?? 0) === 0) return false;
        if (liveHydrationOrchestrator?.isLeftHeadBlocked?.()) return false;
        // Post-prepend continuation: same LEFT-edge runway as initial prefetch.
        // Tip-outside-VIEW (HISTORY) is not sufficient — user may be in the middle.
        if (options.continuation === true) {
          return isApproachingLoadedLeftEdge(range);
        }
        if (!liveHistoryScrollArmed && options.cause !== 'userNav') return false;
        // Edge validity always — force only skips debounce in schedule*, not this gate.
        if (!isApproachingLoadedLeftEdge(range)) return false;
        return true;
      },
      shouldContinueLeftHistory: (range) => {
        if (isSecondsHistoryNavChart(window.currentTf) && !liveHistoryScrollArmed) return false;
        if (!window.historyHasMore) return false;
        if (liveHydrationOrchestrator?.isLeftHeadBlocked?.()) return false;
        return isApproachingLoadedLeftEdge(range);
      },
      getAnchorEndTimeSec: () => liveColumnarStore?.firstTimeSec?.() ?? null,
      getRightTipSec: () => liveColumnarStore?.lastTimeSec?.() ?? null,
      getSlotIds: () => resolveLiveSlotIds(),
      isRenderBusy: () => !!(liveRenderScheduler?.isBusy?.() || window.isUpdatingData),
      isDashboardLoading: () => !!window.__isDashboardLoading,
      getVisibleRange: () => {
        if (typeof TimeCamera !== 'undefined'
          && typeof TimeCamera.getCanonicalVisibleRange === 'function') {
          return TimeCamera.getCanonicalVisibleRange();
        }
        return null;
      },
      pickHistoryPrefetchEdge,
      consumeViewportHistoryAuth: () => {
        liveHistoryScrollArmed = false;
      },
      shouldLoadRight: (range, options = {}) => {
        if (!ChartAdapter.isInitialized('live')) return false;
        if (!range || (liveColumnarStore?.barCount?.() ?? 0) === 0) return false;
        if (!canExtendHistoryRight()) return false;
        if (liveHydrationOrchestrator?.isRightTipBlocked?.()) return false;
        if (options.sourceContinue === true) return true;
        if (options.continuation === true) {
          return isApproachingLoadedRightEdge(range);
        }
        if (!liveHistoryScrollArmed && options.cause !== 'userNav') return false;
        return isApproachingLoadedRightEdge(range);
      },
      shouldContinueRightHistory: (range) => {
        if (isSecondsHistoryNavChart(window.currentTf) && !liveHistoryScrollArmed) return false;
        if (!canExtendHistoryRight()) return false;
        if (liveHydrationOrchestrator?.isRightTipBlocked?.()) return false;
        if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to)) {
          return true;
        }
        return isApproachingLoadedRightEdge(range);
      },
      getRightFetchEndSec: () => {
        const last = liveColumnarStore?.lastTimeSec?.();
        if (!Number.isFinite(last) || last <= 0) return null;
        if (isSecondsHistoryNavChart(window.currentTf)) {
          return sparseChildRightCursorSec(last);
        }
        if (typeof ViewportManager === 'undefined'
          || typeof ViewportManager.resolveRightHistoryFetchEndSec !== 'function') {
          return null;
        }
        const limit = typeof HISTORY_CHUNK_LIMIT !== 'undefined' ? HISTORY_CHUNK_LIMIT : 3000;
        return ViewportManager.resolveRightHistoryFetchEndSec({
          lastTimeSec: last,
          nowSec: Math.floor(Date.now() / 1000),
          limit,
          intervalSec: liveTfIntervalSec(),
        });
      },
      fetchRightColumnar: (cursorSec) => {
        const symbol = document.getElementById('symbol')?.textContent?.trim() || '';
        const limit = typeof HISTORY_CHUNK_LIMIT !== 'undefined' ? HISTORY_CHUNK_LIMIT : 3000;
        const base = {
          tf: window.currentTf,
          limit,
          slots: resolveLiveSlotIds(),
          rsxSettings: resolveLiveRsxSettings(),
          symbol,
        };
        const req = isSecondsHistoryNavChart(window.currentTf)
          ? API.fetchColumnarHistory({
            ...base,
            startTimeSec: cursorSec,
            ...(typeof isSparseSecondChart === 'function' && isSparseSecondChart(window.currentTf)
              ? { includeForming: false }
              : {}),
            ...(typeof isSparseSecondChart === 'function'
              && isSparseSecondChart(window.currentTf)
              && sparseParentResumeAfterSec > 0
              ? { parentResumeAfterSec: sparseParentResumeAfterSec }
              : {}),
          })
          : API.fetchColumnarHistory({
            ...base,
            endTimeSec: cursorSec,
            ...(typeof isSparseSecondChart === 'function' && isSparseSecondChart(window.currentTf)
              ? { includeForming: false }
              : {}),
          });
        return req;
      },
      fetchColumnar: (endTimeSec) => {
        const symbol = document.getElementById('symbol')?.textContent?.trim() || '';
        return API.fetchColumnarHistory({
          tf: window.currentTf,
          endTimeSec,
          limit: typeof HISTORY_CHUNK_LIMIT !== 'undefined' ? HISTORY_CHUNK_LIMIT : 3000,
          slots: resolveLiveSlotIds(),
          rsxSettings: resolveLiveRsxSettings(),
          symbol,
          ...(typeof isSparseSecondChart === 'function' && isSparseSecondChart(window.currentTf)
            ? { includeForming: false }
            : {}),
        });
      },
      mergeIntoStore: (data) => {
        const rawRange = ChartAdapter.getVisibleLogicalRange('live');
        // Clone — LWC may mutate its live range object during setData.
        const viewportRange = rawRange && Number.isFinite(rawRange.from) && Number.isFinite(rawRange.to)
          ? { from: rawRange.from, to: rawRange.to }
          : null;
        const cap = (typeof ViewportManager !== 'undefined' && ViewportManager.capture)
          ? ViewportManager.capture('live')
          : null;
        const focalTimeSec = (cap?.centerTimeMs != null && Number.isFinite(cap.centerTimeMs))
          ? cap.centerTimeMs / 1000
          : null;
        const viewTimes = captureStoreViewTimes();
        // Store-time ViewportAnchor BEFORE mutation (identity for preserve after +N).
        const preSnap = typeof liveColumnarStore.snapshot === 'function'
          ? liveColumnarStore.snapshot()
          : null;
        const viewportAnchor = (typeof ChartCompositor !== 'undefined'
          && ChartCompositor.captureViewportAnchor)
          ? ChartCompositor.captureViewportAnchor(preSnap?.times, viewportRange)
          : null;
        const storeBefore = liveColumnarStore?.barCount?.() ?? 0;
        const headBefore = typeof liveColumnarStore?.firstTimeSec === 'function'
          ? liveColumnarStore.firstTimeSec()
          : null;
        const tipBefore = typeof liveColumnarStore?.lastTimeSec === 'function'
          ? liveColumnarStore.lastTimeSec()
          : null;
        const merge = liveColumnarStore.prependMonolith(data, {
          focalTimeSec,
          atLiveEdge: cap?.isAtRightEdge === true,
          viewFromSec: viewTimes?.viewFromSec,
          viewToSec: viewTimes?.viewToSec,
        });
        const added = Number(merge?.added) || 0;
        if (added <= 0) {
          return null;
        }
        const storeAfter = liveColumnarStore?.barCount?.() ?? 0;
        const headAfter = merge.headAfter ?? (typeof liveColumnarStore?.firstTimeSec === 'function'
          ? liveColumnarStore.firstTimeSec()
          : null);
        const tipAfter = merge.tipAfter ?? (typeof liveColumnarStore?.lastTimeSec === 'function'
          ? liveColumnarStore.lastTimeSec()
          : null);
        // Detached island: prune dropped the FE tip. Do not glue live now onto it.
        if (isSecondsHistoryNavChart(window.currentTf)
            && Number.isFinite(tipBefore) && Number.isFinite(tipAfter)
            && tipAfter < tipBefore) {
          window.historyHasNewer = true;
        }
        return {
          added,
          prependedCount: merge.prependedCount ?? added,
          prunedRightCount: merge.prunedRightCount ?? 0,
          prunedLeftCount: merge.prunedLeftCount ?? 0,
          headBefore: merge.headBefore ?? headBefore,
          headAfter,
          tipBefore,
          tipAfter,
          viewportRange,
          viewportAnchor,
          storeBefore,
          storeAfter,
        };
      },
      mergeAppendIntoStore: (data) => {
        const rawRange = ChartAdapter.getVisibleLogicalRange('live');
        const viewportRange = rawRange && Number.isFinite(rawRange.from) && Number.isFinite(rawRange.to)
          ? { from: rawRange.from, to: rawRange.to }
          : null;
        const cap = (typeof ViewportManager !== 'undefined' && ViewportManager.capture)
          ? ViewportManager.capture('live')
          : null;
        const focalTimeSec = (cap?.centerTimeMs != null && Number.isFinite(cap.centerTimeMs))
          ? cap.centerTimeMs / 1000
          : null;
        const viewTimes = captureStoreViewTimes();
        const preSnap = typeof liveColumnarStore.snapshot === 'function'
          ? liveColumnarStore.snapshot()
          : null;
        const viewportAnchor = (typeof ChartCompositor !== 'undefined'
          && ChartCompositor.captureViewportAnchor)
          ? ChartCompositor.captureViewportAnchor(preSnap?.times, viewportRange)
          : null;
        const tipBefore = typeof liveColumnarStore?.lastTimeSec === 'function'
          ? liveColumnarStore.lastTimeSec()
          : null;
        const merge = liveColumnarStore.appendMonolith(data, {
          focalTimeSec,
          atLiveEdge: false,
          viewFromSec: viewTimes?.viewFromSec,
          viewToSec: viewTimes?.viewToSec,
        });
        const added = Number(merge?.added) || 0;
        if (added <= 0) return null;
        if (isSecondsHistoryNavChart(window.currentTf)
            && data && typeof data.hasNewer === 'boolean') {
          window.historyHasNewer = data.hasNewer === true;
        }
        sparseParentResumeAfterSec = 0;
        return {
          added,
          viewportRange,
          viewportAnchor,
          tipBefore: merge.tipBefore ?? tipBefore,
          tipAfter: merge.tipAfter ?? (typeof liveColumnarStore?.lastTimeSec === 'function'
            ? liveColumnarStore.lastTimeSec()
            : null),
          headBefore: merge.headBefore,
          headAfter: merge.headAfter,
        };
      },
      rightEmptyClearsDetached: () => isSecondsHistoryNavChart(window.currentTf),
      onRightSourceTail: () => {
        window.historyHasNewer = false;
        sparseParentResumeAfterSec = 0;
      },
      onSparseRightNoProgress: (data) => {
        if (typeof isLiveSecondChart === 'function' && isLiveSecondChart(window.currentTf)) {
          if (data && data.hasNewer === true) {
            console.warn('[HydrationOrchestrator] 1s right page added==0 but hasNewer=true (source/merge inconsistency)');
            return 'stop';
          }
          return 'stall';
        }
        if (typeof isSparseSecondChart !== 'function' || !isSparseSecondChart(window.currentTf)) {
          return 'stall';
        }
        const wm = Number(data?.parentResumeAfterSec);
        const prev = Number(sparseParentResumeAfterSec) || 0;
        if (data && data.hasNewer === false) {
          sparseParentResumeAfterSec = 0;
          return 'eof';
        }
        if (Number.isFinite(wm) && wm > prev) {
          sparseParentResumeAfterSec = wm;
          return 'continue';
        }
        console.warn('[SECONDS-HISTORY] parent resume watermark stuck', {
          parentResumeAfterSec: wm,
          previous: prev,
          hasNewer: data?.hasNewer,
        });
        return 'stop';
      },
      logRightAppendDiag: (data, meta) => {
        if (typeof isSparseSecondChart !== 'function' || !isSparseSecondChart(window.currentTf)) {
          return;
        }
        const folded = Array.isArray(data?.times) ? data.times.length : 0;
        console.log('[SECONDS-HISTORY] right append', {
          parentResumeAfterSec: data?.parentResumeAfterSec ?? 0,
          folded,
          added: meta?.added ?? 0,
          tipBefore: meta?.tipBefore ?? null,
          tipAfter: meta?.tipAfter ?? null,
          hasNewer: data?.hasNewer,
        });
      },
      markDirty: (intent) => liveRenderScheduler?.markDirty(intent),
      processTick: (tick) => pushLiveTickDelta(tick),
    });
  }

  function attachLiveHistoryScrollArm() {
    const root = document.getElementById('live-chart-container');
    if (!root || root._historyScrollArmBound) return;
    root._historyScrollArmBound = true;
    const arm = (ev) => {
      // Hover must not authorize pages. Drag (primary button) refreshes arm
      // after consumeViewportHistoryAuth so in-flight travel can coalesce.
      if (ev && ev.type === 'pointermove' && !(ev.buttons & 1)) return;
      liveHistoryScrollArmed = true;
      // User gesture takes camera ownership immediately (ends system preserve txn).
      if (typeof TimeCamera !== 'undefined' && TimeCamera.releasePreserveTransaction) {
        TimeCamera.releasePreserveTransaction();
      }
    };
    root.addEventListener('wheel', arm, { passive: true });
    root.addEventListener('pointerdown', arm, { passive: true });
    root.addEventListener('pointermove', arm, { passive: true });
  }

  function resolveCanonicalPrefetchView() {
    if (typeof TimeCamera === 'undefined') return null;
    if (typeof TimeCamera.getCanonicalVisibleRange === 'function') {
      return TimeCamera.getCanonicalVisibleRange();
    }
    const c = typeof TimeCamera.getCanonical === 'function' ? TimeCamera.getCanonical() : null;
    const r = c && c.visibleRange;
    if (r && Number.isFinite(r.from) && Number.isFinite(r.to) && r.to > r.from) {
      return { from: r.from, to: r.to };
    }
    return null;
  }

  /** Bars remaining from the visible range to the island head / tip (logical indices). */
  function remainingIslandRunway(range) {
    const n = liveColumnarStore?.barCount?.() ?? 0;
    const tip = n > 0 ? n - 1 : 0;
    const left = Math.max(0, Number(range.from));
    const right = Math.max(0, tip - Number(range.to));
    return { left, right };
  }

  /**
   * One prefetch edge per range event. Both eligible → smaller remaining runway.
   * No pan-direction FSM.
   */
  function pickHistoryPrefetchEdge(range) {
    if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to)) return null;
    const leftOk = !!window.historyHasMore && isApproachingLoadedLeftEdge(range);
    const rightOk = canExtendHistoryRight() && isApproachingLoadedRightEdge(range);
    if (leftOk && !rightOk) return 'left';
    if (rightOk && !leftOk) return 'right';
    if (!leftOk && !rightOk) return null;
    const { left, right } = remainingIslandRunway(range);
    if (right < left) return 'right';
    return 'left';
  }

  function scheduleHistoryLoad(_rawLwcRange) {
    // Wave 2: Boot detects only — never retries, never owns pending.
    // Busy must not drop: HydrationOrchestrator remembers newest left/right intent.
    // Trigger = LWC range event; decision VIEW = TimeCamera canonical (clamped).
    // P1: zoom-aware prefetch runway + force skips debounce reset-while-scrolling
    // (does not remove debounceMs; in-flight still coalesces to one fetch).
    if (!liveHistoryScrollArmed) return;
    if ((liveColumnarStore?.barCount?.() ?? 0) === 0) return;
    if (!ChartAdapter.isInitialized('live')) return;

    const range = resolveCanonicalPrefetchView();
    if (!range) return;

    const edge = pickHistoryPrefetchEdge(range);
    if (edge === 'left') {
      liveHydrationOrchestrator?.noteLeftHistoryIntent?.(range, { force: true, cause: 'userNav' });
    } else if (edge === 'right') {
      liveHydrationOrchestrator?.noteRightHistoryIntent?.(range, { force: true, cause: 'userNav' });
    }
  }

  /**
   * Wave 1 (ADR-028): windowMode is a Data fact only — Boot must not decide VIEW.
   * Former path: windowMode=history + right-edge → loadDashboard() → FreshLive (E2-01).
   * Tip rehydration without Boot→FreshLive → later wave.
   * Right-edge island fill is HydrationOrchestrator.append (not FreshLive).
   */
  function maybeReturnToLiveFromHistory(_range) {
    /* no-op — Data never changes VIEW */
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

  /**
   * Seconds WS ingest veto. historyHasNewer is paging/source only for 5s–45s.
   * 1s keeps the detached-island hasNewer veto. TimeCamera is paint-only.
   * @param {string} tf
   * @param {string} windowMode
   * @param {boolean} historyHasNewer
   */
  function shouldVetoSecondsLiveIngest(tf, windowMode, historyHasNewer) {
    if (typeof isSparseSecondChart === 'function' && isSparseSecondChart(tf)) {
      return windowMode === 'history';
    }
    if (typeof isLiveSecondChart === 'function' && isLiveSecondChart(tf)
        && historyHasNewer === true) {
      return true;
    }
    return false;
  }

  function pushLiveTickDelta(tick, options = {}) {
    if (!liveColumnarStore || !liveRenderScheduler || liveColumnarStore.isSealed()) return false;
    const tickTf = tick?.timeframe || window.currentTf;
    // Dense HISTORY island must not ingest live ticks (Debt #69A gap-heal yank).
    // Sparse 1s: MICRO-2C still ingests; paint is gated by TimeCamera VIEW.
    // Blocking ingest here deadlocks 1s: quiet seconds never promote windowMode.
    // 5s–45s: windowMode is island identity (HISTORY microscope rejects "now").
    if (liveColumnarStore.windowMode === 'history') {
      if (typeof isSparseSecondChart === 'function' && isSparseSecondChart(tickTf)) {
        return false;
      }
      const sparseTick = typeof isSparseLiveChart === 'function'
        && isSparseLiveChart(tickTf);
      if (!sparseTick) return false;
    }
    if (shouldVetoSecondsLiveIngest(tickTf, liveColumnarStore.windowMode, window.historyHasNewer === true)) {
      return false;
    }
    // Preserve-paired: capture VIEW before append so budget cannot drop visible oldest bars.
    const viewTimes = captureStoreViewTimes();
    const appendResult = liveColumnarStore.appendTick(tick, {
      viewFromSec: viewTimes?.viewFromSec,
      viewToSec: viewTimes?.viewToSec,
    });
    if (appendResult?.gapDetected) {
      const tf = tick?.timeframe || window.currentTf;
      if (typeof requiresDenseTimeContinuity === 'function' && !requiresDenseTimeContinuity(tf)) {
        return false;
      }
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
    // Render gate only (Fix F / MICRO-2C): store already ingested. VIEW ≠ windowMode.
    const viewIntent = liveCameraViewIntent();
    const sparse = typeof isSparseLiveChart === 'function'
      && isSparseLiveChart(tick?.timeframe || window.currentTf);
    if (maybeSparseHistoryToLivePaint(viewIntent, sparse)) {
      return true;
    }
    if (!shouldMarkDirtyLiveDelta(viewIntent, appendResult.isNewBar, sparse)) {
      return true;
    }
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

  /**
   * TimeCamera shadow VIEW intent (LIVE | HISTORY). Null = unknown → fail open.
   * Does not read windowMode.
   */
  function liveCameraViewIntent() {
    if (typeof TimeCamera === 'undefined' || typeof TimeCamera._getShadowView !== 'function') {
      return null;
    }
    try {
      const intent = TimeCamera._getShadowView()?.intent;
      if (intent === 'LIVE' || intent === 'HISTORY') return intent;
      return null;
    } catch {
      return null;
    }
  }

  /**
   * Delta *paint* policy after appendTick. Ingest is never gated here.
   * Native/derived: HISTORY same-bar skip; new bars still delta (Fix F).
   * Sparse: HISTORY skips all deltas (MICRO-2C). Unknown intent fails open.
   * @param {'LIVE'|'HISTORY'|null|undefined} viewIntent
   * @param {boolean} isNewBar
   * @param {boolean} [sparse]
   */
  function shouldMarkDirtyLiveDelta(viewIntent, isNewBar, sparse) {
    if (sparse === true && viewIntent === 'HISTORY') return false;
    return !(viewIntent === 'HISTORY' && isNewBar !== true);
  }

  /**
   * Sparse HISTORY→LIVE: one full setData from store, then deltas resume.
   * Dense TFs never take this path. Does not move VIEW (restore/preserve only).
   */
  function maybeSparseHistoryToLivePaint(viewIntent, sparse) {
    if (sparse !== true) {
      sparseViewIntent = null;
      return false;
    }
    if (viewIntent !== 'LIVE' && viewIntent !== 'HISTORY') {
      return false;
    }
    const prev = sparseViewIntent;
    sparseViewIntent = viewIntent;
    const needsFull = typeof sparseHistoryToLiveNeedsFullPaint === 'function'
      ? sparseHistoryToLiveNeedsFullPaint(prev, viewIntent)
      : (prev === 'HISTORY' && viewIntent === 'LIVE');
    if (!needsFull || !liveRenderScheduler) return false;
    const anchor = captureReconnectViewportAnchor();
    liveRenderScheduler.markDirty({
      mode: 'full',
      viewport: anchor ? 'restore' : 'preserve',
      anchor: anchor || undefined,
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
      // Browser↔bot reconnect ≠ Binance heal. Sparse: quiet Shot 10B. Dense: TimelineRecovery.
      onReconnect: onBrowserReconnect,
    });
  }

  /**
   * Sparse 1s: HISTORY acquisition is allowed only for explicit navigation
   * (focus well behind now). A lagged live camera that classified HISTORY
   * must not pick a historical endTime / windowMode=history island.
   * Scroll-left prepend is a different path (HydrationOrchestrator).
   */
  function sparseExplicitHistoryHydrate(anchor, nowSec) {
    if (!anchor || anchor.intent !== 'HISTORY') return false;
    const centerMs = Number(anchor.centerTimeMs);
    if (!Number.isFinite(centerMs) || centerMs <= 0) return false;
    let centerSec = Math.floor(centerMs / 1000);
    try {
      const CDS = typeof ChartDataStore !== 'undefined' ? ChartDataStore : globalThis.ChartDataStore;
      if (CDS && typeof CDS.msToChartSec === 'function') {
        const sec = Number(CDS.msToChartSec(centerMs));
        if (Number.isFinite(sec) && sec > 0) centerSec = sec;
      }
    } catch {
      /* keep ms/1000 fallback */
    }
    if (!Number.isFinite(centerSec) || centerSec <= 0) return false;
    const now = Math.floor(Number(nowSec));
    if (!Number.isFinite(now) || now <= 0) return false;
    const visible = Math.floor(Number(anchor.visibleBars));
    const viewSpanSec = Math.max(60, Number.isFinite(visible) && visible > 0 ? visible : 300);
    return (now - centerSec) > viewSpanSec;
  }

  /**
   * Explicit Return-to-Live: latest 1s tail replace + full paint + VIEW LIVE.
   * Does not stitch intermediate chunks. Does not move camera on hasNewer=false paging.
   */
  function returnToLive() {
    let viewportAnchor = null;
    if (typeof ViewportManager !== 'undefined' && typeof ViewportManager.capture === 'function') {
      const cap = ViewportManager.capture('live');
      if (cap) {
        viewportAnchor = {
          intent: 'LIVE',
          isAtRightEdge: true,
          visibleBars: cap.visibleBars,
          barSpacing: cap.barSpacing,
          rightPadding: cap.rightPadding != null ? cap.rightPadding : cap.rightOffset,
        };
      }
    }
    return loadDashboard({ viewportAnchor, userReturnToLive: true });
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
    if (!options.quiet && typeof ToolbarController !== 'undefined') {
      ToolbarController.setBuffering(true);
    }

    let completed = false;
    let retrying = false;
    try {
      if (window.DDRFactory && !window.DDRFactory.manifest) {
        await window.DDRFactory.fetchManifest();
      }
      if (!isCurrentEpoch(epoch)) return;

      const symbol = document.getElementById('symbol')?.textContent?.trim() || '';
      const chunkLimit = typeof HISTORY_CHUNK_LIMIT !== 'undefined' ? HISTORY_CHUNK_LIMIT : 3000;
      let limit = chunkLimit;
      // TF-2A / HIST-VIEW-1: user TF switch covers preserved VIEW. Scroll chunks stay chunkLimit.
      if (options.userTfChange === true
        && viewportAnchor
        && (viewportAnchor.intent === 'LIVE' || viewportAnchor.intent === 'HISTORY')
        && typeof ViewportManager !== 'undefined'
        && typeof ViewportManager.resolveLiveTfSwitchFetchLimit === 'function') {
        limit = ViewportManager.resolveLiveTfSwitchFetchLimit(viewportAnchor.visibleBars);
      }
      const nowSec = Math.floor(Date.now() / 1000);
      const sparseTf = typeof isSparseLiveChart === 'function'
        && isSparseLiveChart(window.currentTf);
      const returnToLiveJump = options.userReturnToLive === true;
      const sparseHistory = !returnToLiveJump
        && sparseTf
        && sparseExplicitHistoryHydrate(viewportAnchor, nowSec);
      // Dense: HISTORY TF switch hydrates around captured centerTime.
      // Sparse live-entry: latest tail (stale HISTORY classification is not an endTime).
      // Sparse explicit history: same centered BeforeEnd window as native.
      // Explicit Return-to-Live: latest tail, never a historical endTime.
      let endTimeSec = nowSec;
      const historyIsland = !returnToLiveJump
        && viewportAnchor?.intent === 'HISTORY'
        && Number.isFinite(Number(viewportAnchor.centerTimeMs))
        && (!sparseTf || sparseHistory);
      if (!returnToLiveJump
        && viewportAnchor?.intent === 'HISTORY'
        && Number.isFinite(Number(viewportAnchor.centerTimeMs))
        && (!sparseTf || sparseHistory)
        && typeof ViewportManager !== 'undefined'
        && typeof ViewportManager.resolveHistoryTfFetchEndSec === 'function') {
        const intervalFn = typeof getIntervalMs === 'function'
          ? getIntervalMs
          : (typeof TimeNormalizer !== 'undefined' ? TimeNormalizer.getIntervalMs : parseTfIntervalMs);
        const intervalMs = Number(intervalFn(window.currentTf));
        const intervalSec = Number.isFinite(intervalMs) && intervalMs > 0
          ? Math.floor(intervalMs / 1000)
          : 60;
        endTimeSec = ViewportManager.resolveHistoryTfFetchEndSec({
          intent: 'HISTORY',
          centerTimeMs: viewportAnchor.centerTimeMs,
          nowSec,
          limit,
          intervalSec,
        });
      }

      const [columnar, stateResult] = await Promise.all([
        API.fetchColumnarHistory({
          tf: window.currentTf,
          endTimeSec,
          limit,
          slots: resolveLiveSlotIds(),
          rsxSettings: resolveLiveRsxSettings(),
          symbol,
          ...(typeof isSparseSecondChart === 'function' && isSparseSecondChart(window.currentTf)
            ? { includeForming: !historyIsland }
            : {}),
        }),
        API.fetchLiveState({ navigatorsOnly: true }),
      ]);
      if (!isCurrentEpoch(epoch)) return;

      if (stateResult?.warmingUp) {
        retrying = true;
        setTimeout(() => loadDashboard(options), 2000);
        return;
      }

      if (columnar?.noData === true || columnar?.status === 'no_data') {
        console.warn('[Renaissance] history NO_DATA — keep previous island');
        return;
      }
      if (!columnar?.times?.length || !liveColumnarStore) {
        console.warn('[Renaissance] no columnar history — chart idle');
        return;
      }

      // Shot 11C Atomic Swap: mutate store + ensure DDR hosts offline, then ONE full paint.
      // Commit-paired (FreshLive / TF hydrate): intentional world replace — no Mutation Set.
      // P0: HISTORY TF island must be windowMode=history so live ticks do not gap-heal
      // (Debt #69A). Tip-behind-live is intentional Microscope, not a transport failure.
      // windowMode = which data window was fetched. TimeCamera intent = paint.
      // Dense HISTORY island: windowMode=history (Debt #69A). Sparse explicit
      // history: same label, but live ticks still ingest (MICRO-2C).
      liveColumnarStore.replaceMonolith(columnar, {
        commitPaired: true,
        windowMode: historyIsland ? 'history' : 'live',
      });
      const histTimes = Array.isArray(columnar.times) ? columnar.times : [];
      const historyTipOpen = histTimes.length ? Number(histTimes[histTimes.length - 1]) : null;
      handoffDiag = { historyTipOpen, waiting: true };
      console.log('[TransportDiag] history loaded', {
        historyTipOpen,
        timeframe: window.currentTf,
        bars: histTimes.length,
        windowMode: liveColumnarStore.windowMode,
      });
      window.historyHasMore = columnar.hasMore !== false;
      if (isSecondsHistoryNavChart(window.currentTf)) {
        window.historyHasNewer = columnar.hasNewer === true;
      }
      sparseParentResumeAfterSec = 0;
      flushLiveTickBuffer();
      if (!liveColumnarStore.invariantOk()) {
        console.error('[Renaissance] ColumnarStore invariant failed', liveColumnarStore.invariantMeta());
        return;
      }
      await mountDDRLiveCutover();
      if (!isCurrentEpoch(epoch)) return;

      completed = true;
      beginDataUpdate();
      try {
        const restoreLive = returnToLiveJump || (viewportAnchor && viewportAnchor.intent === 'LIVE');
        liveRenderScheduler?.markDirty({
          mode: 'full',
          viewport: restoreLive ? 'restore' : (viewportAnchor ? 'restore' : 'fresh'),
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
        liveHydrationOrchestrator?.tryConsumePending?.();
      } else if (retrying) {
        // Keep buffer + loading flag across warmingUp retry.
        window.__isDashboardLoading = true;
      } else {
        if (!completed) abortLiveTickBuffer();
        window.__isDashboardLoading = false;
        updateBufferingOverlay();
        liveHydrationOrchestrator?.tryConsumePending?.();
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
        // Mid-paint LWC echoes are not user navigation. Paint must not re-arm
        // viewport history; onAfterFlush only consumes existing human pending.
        if (typeof ChartAdapter !== 'undefined' && ChartAdapter.isLiveUpdating()) {
          return;
        }
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
