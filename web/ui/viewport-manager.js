/**
 * ViewportManager — ADR-028 D2 capture / translate helper only.
 *
 * Allowed: capture geometry seed for TF handoff, host layout helpers, backtest legacy restore shim.
 * Forbidden: live navigation policy, direct LWC camera writes (applyOptions/scroll/setVisible).
 *
 * Live navigation → TimeCamera.proposeAfterData / proposeFreshLive → CameraCommit.
 */
(function initViewportManager(global) {
  const HEALTHY_BAR_SPACING = 6;
  const MAX_HEALTHY_VISIBLE_BARS = 400;
  const MIN_HEALTHY_BAR_SPACING = 1;
  const HEALTHY_VISIBLE_BARS = 150;
  /** Wide zoom is legitimate up to MAX_VISIBLE_BARS — not "poison". */
  const MAX_CAPTURE_VISIBLE_BARS = (typeof MAX_VISIBLE_BARS !== 'undefined'
    && Number.isFinite(MAX_VISIBLE_BARS) && MAX_VISIBLE_BARS > 0)
    ? MAX_VISIBLE_BARS
    : 5000;

  function priceHostId(context) {
    return context === 'backtest' ? 'bt-price-chart' : 'price-chart';
  }

  function hostHasLayout(context) {
    const el = typeof document !== 'undefined'
      ? document.getElementById(priceHostId(context))
      : null;
    return !!(el && el.clientWidth > 0 && el.clientHeight > 0);
  }

  function storeForContext(context) {
    if (context === 'backtest') {
      return typeof backtestStore !== 'undefined' ? backtestStore : null;
    }
    return global.liveColumnarStore || null;
  }

  function timesSecFromStore(store) {
    if (!store) return [];
    if (typeof store.snapshot === 'function') {
      const snap = store.snapshot();
      return Array.isArray(snap?.times) ? snap.times : [];
    }
    if (typeof store.candlesArray === 'function') {
      return store.candlesArray().map((c) => Number(c.time));
    }
    return [];
  }

  function isPoisonCameraState(state) {
    if (!state) return true;
    if (Number.isFinite(state.from) && state.from < 0) return true;
    if (Number.isFinite(state.barSpacing) && state.barSpacing < MIN_HEALTHY_BAR_SPACING) return true;
    if (Number.isFinite(state.visibleBars) && state.visibleBars > MAX_CAPTURE_VISIBLE_BARS) return true;
    return false;
  }

  /**
   * Capture navigation seed (translate LWC+store → semantic fields).
   * Uses TimeCamera.classifyViewIntent for LIVE/HISTORY — not density branches.
   */
  function capture(context) {
    const range = typeof ChartAdapter !== 'undefined'
      ? ChartAdapter.getVisibleLogicalRange(context)
      : null;
    if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to)) return null;

    const store = storeForContext(context);
    const times = timesSecFromStore(store);
    if (!times.length) return null;

    let visibleBars = range.to - range.from;
    const tipLogical = times.length - 1;
    const centerIndex = Math.floor((range.from + range.to) / 2);
    const clampedIndex = Math.max(0, Math.min(tipLogical, centerIndex));
    const centerSec = Number(times[clampedIndex]);
    if (!Number.isFinite(centerSec)) return null;
    const centerTimeMs = Math.floor(centerSec * 1000);

    const mainChart = typeof ChartAdapter !== 'undefined'
      ? ChartAdapter.getChart(context, 'price')
      : null;
    let barSpacing = mainChart?.timeScale()?.options()?.barSpacing ?? null;

    if (isPoisonCameraState({ barSpacing, visibleBars, from: range.from })) {
      barSpacing = HEALTHY_BAR_SPACING;
      visibleBars = Math.max(50, Math.min(visibleBars, MAX_CAPTURE_VISIBLE_BARS));
    }

    const classify = (typeof TimeCamera !== 'undefined' && TimeCamera._helpers?.classifyViewIntent)
      ? TimeCamera._helpers.classifyViewIntent
      : null;
    const slack = (typeof TimeCamera !== 'undefined' && Number.isFinite(TimeCamera.SLACK))
      ? TimeCamera.SLACK
      : 1.5;
    const intent = classify
      ? classify(range.to, tipLogical, slack)
      : (range.to >= tipLogical - slack ? 'LIVE' : 'HISTORY');
    const rightPadding = Math.max(0, range.to - tipLogical);

    return {
      centerTimeMs,
      visibleBars,
      tipVisible: range.to >= tipLogical,
      isAtRightEdge: intent === 'LIVE',
      intent,
      rightOffset: rightPadding,
      rightPadding,
      barSpacing: Number.isFinite(barSpacing) ? barSpacing : HEALTHY_BAR_SPACING,
    };
  }

  /**
   * Build TF handoff seed from capture. No TF-density branching (ADR-028).
   * HISTORY → HISTORY: preserve centerTime only; use healthy default zoom for the NEW TF
   * (do not carry the old visibleBars / old time-span into the denser TF).
   * @param {object|null} captured
   */
  function cameraIntentForTfSwitch(captured) {
    if (!captured || captured.centerTimeMs == null) return null;
    const isHistory = captured.intent === 'HISTORY';
    const healthyBars = (typeof TimeCamera !== 'undefined'
      && Number.isFinite(TimeCamera.HEALTHY_VISIBLE_BARS))
      ? TimeCamera.HEALTHY_VISIBLE_BARS
      : HEALTHY_VISIBLE_BARS;
    return {
      centerTimeMs: captured.centerTimeMs,
      // Microscope contract: center is sacred; new TF gets a normal viewport size.
      visibleBars: isHistory ? healthyBars : (captured.visibleBars || healthyBars),
      rightOffset: Number.isFinite(captured.rightOffset) ? captured.rightOffset : 0,
      rightPadding: Number.isFinite(captured.rightPadding)
        ? captured.rightPadding
        : (Number.isFinite(captured.rightOffset) ? captured.rightOffset : 0),
      barSpacing: isHistory
        ? ((typeof TimeCamera !== 'undefined' && Number.isFinite(TimeCamera.HEALTHY_BAR_SPACING))
          ? TimeCamera.HEALTHY_BAR_SPACING
          : HEALTHY_BAR_SPACING)
        : captured.barSpacing,
      isAtRightEdge: captured.isAtRightEdge === true || captured.intent === 'LIVE',
      intent: isHistory ? 'HISTORY' : 'LIVE',
    };
  }

  /**
   * HISTORY TF hydrate: API endTime so a normal chunk is centered on the captured
   * market-time focus (not the live tip). LIVE / missing center → nowSec.
   * @param {{
   *   intent?: string|null,
   *   centerTimeMs?: number|null,
   *   nowSec: number,
   *   limit: number,
   *   intervalSec: number,
   * }} opts
   * @returns {number} unix seconds endTimeSec for BeforeEnd-style history fetch
   */
  function resolveHistoryTfFetchEndSec(opts) {
    const nowSec = Number(opts?.nowSec);
    const fallback = Number.isFinite(nowSec) && nowSec > 0
      ? Math.floor(nowSec)
      : Math.floor(Date.now() / 1000);
    if (!opts || opts.intent !== 'HISTORY') return fallback;
    const centerMs = Number(opts.centerTimeMs);
    if (!Number.isFinite(centerMs) || centerMs <= 0) return fallback;
    const CDS = typeof ChartDataStore !== 'undefined' ? ChartDataStore : global.ChartDataStore;
    const centerSec = CDS.msToChartSec(centerMs);
    const bars = Number(opts.limit);
    const limit = Number.isFinite(bars) && bars > 0 ? Math.floor(bars) : 3000;
    const iv = Number(opts.intervalSec);
    const intervalSec = Number.isFinite(iv) && iv > 0 ? Math.floor(iv) : 60;
    const end = centerSec + Math.floor(limit / 2) * intervalSec;
    return Math.min(end, fallback);
  }

  /**
   * Right-edge island fill: BeforeEnd window ending ~one normal chunk after store tip.
   * Independent of TF-switch centerTime. Returns null when tip is already at/after now.
   * @param {{
   *   lastTimeSec: number,
   *   nowSec: number,
   *   limit: number,
   *   intervalSec: number,
   * }} opts
   * @returns {number|null}
   */
  function resolveRightHistoryFetchEndSec(opts) {
    const nowSec = Number(opts?.nowSec);
    const fallback = Number.isFinite(nowSec) && nowSec > 0
      ? Math.floor(nowSec)
      : Math.floor(Date.now() / 1000);
    const last = Number(opts?.lastTimeSec);
    if (!Number.isFinite(last) || last <= 0) return null;
    if (last >= fallback) return null;
    const bars = Number(opts?.limit);
    const limit = Number.isFinite(bars) && bars > 0 ? Math.floor(bars) : 3000;
    const iv = Number(opts?.intervalSec);
    const intervalSec = Number.isFinite(iv) && iv > 0 ? Math.floor(iv) : 60;
    return Math.min(last + limit * intervalSec, fallback);
  }

  /**
   * P1 edge prefetch: bars of runway from viewport edge → loaded data edge.
   * Zoom-aware: max(hardMin, ceil(visibleBars * frac)). Not "20% of store length".
   * @param {number} visibleBars
   * @param {number} [hardMin]
   * @param {number} [frac]
   * @returns {number}
   */
  function historyEdgePrefetchBars(visibleBars, hardMin, frac) {
    const hard = Number(hardMin);
    const minBars = Number.isFinite(hard) && hard > 0 ? Math.floor(hard) : 50;
    const vb = Number(visibleBars);
    const f = Number(frac);
    if (!Number.isFinite(vb) || vb <= 0 || !Number.isFinite(f) || f <= 0) {
      return minBars;
    }
    return Math.max(minBars, Math.ceil(vb * f));
  }

  /**
   * True when VIEW is within prefetch runway of the loaded tip (right edge).
   * @param {{ from: number, to: number }} range
   * @param {number} tipLogical
   * @param {{ hardMin?: number, frac?: number }} [opts]
   */
  function isWithinRightEdgePrefetch(range, tipLogical, opts = {}) {
    if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to)) return false;
    const tip = Number(tipLogical);
    if (!Number.isFinite(tip)) return false;
    const visible = Number(range.to) - Number(range.from);
    const runway = historyEdgePrefetchBars(visible, opts.hardMin, opts.frac);
    return Number(range.to) >= tip - runway;
  }

  /**
   * True when VIEW is within prefetch runway of the loaded head (left void).
   * @param {{ from: number, to: number }} range
   * @param {{ hardMin?: number, frac?: number }} [opts]
   */
  function isWithinLeftEdgePrefetch(range, opts = {}) {
    if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to)) return false;
    const visible = Number(range.to) - Number(range.from);
    const runway = historyEdgePrefetchBars(visible, opts.hardMin, opts.frac);
    return Number(range.from) < runway;
  }

  /**
   * Debt #80 — run fn once host has layout. Does not own navigation.
   * @param {string} context
   * @param {() => void} fn
   */
  function whenHostHasLayout(context, fn) {
    if (typeof fn !== 'function') return;
    if (typeof document === 'undefined') return;
    const host = document.getElementById(priceHostId(context));
    if (!host) return;

    const run = () => {
      if (!hostHasLayout(context)) return false;
      if (host._vmDeferredNavRo) {
        host._vmDeferredNavRo.disconnect();
        host._vmDeferredNavRo = null;
      }
      try { fn(); } catch { /* */ }
      return true;
    };

    if (run()) return;

    if (typeof ResizeObserver !== 'undefined') {
      if (host._vmDeferredNavRo) return;
      const ro = new ResizeObserver(() => { run(); });
      host._vmDeferredNavRo = ro;
      ro.observe(host);
      return;
    }
    if (typeof requestAnimationFrame === 'function') {
      requestAnimationFrame(() => {
        requestAnimationFrame(() => { run(); });
      });
    }
  }

  /**
   * @deprecated live navigation — use TimeCamera.proposeAfterData.
   * Backtest-only temporary shim (ChartProjection).
   */
  function restore(context, anchor, store) {
    if (context === 'live') {
      // Live path must not restore here — compositor owns TimeCamera propose.
      return;
    }
    // Backtest compatibility shim (not live D2 surface).
    if (!anchor || anchor.centerTimeMs == null) return;
    if (typeof ChartAdapter === 'undefined') return;
    const targetStore = store || storeForContext(context);
    const times = timesSecFromStore(targetStore);
    if (!times.length) return;
    const tip = times.length - 1;
    const seed = {
      intent: anchor.isAtRightEdge ? 'LIVE' : 'HISTORY',
      _liveEdge: !!anchor.isAtRightEdge,
      centerTime: anchor.centerTimeMs,
      visibleBars: anchor.visibleBars,
      barSpacing: anchor.barSpacing,
      rightPadding: anchor.rightOffset,
    };
    if (typeof TimeCamera !== 'undefined' && TimeCamera.bindDataResolve) {
      TimeCamera.bindDataResolve({
        nearestLogicalForTime: (ms) => {
          if (typeof ChartCompositor !== 'undefined' && ChartCompositor.findIndexByTimeMs) {
            return ChartCompositor.findIndexByTimeMs(times, ms);
          }
          return null;
        },
      });
    }
    if (typeof TimeCamera !== 'undefined' && TimeCamera.proposeAfterData) {
      TimeCamera.observeCommittedWorld?.({ tipLogical: tip, timesSec: times });
      TimeCamera.proposeAfterData({
        tipLogical: tip,
        timesSec: times,
        seed,
        mode: 'switch',
      });
    }
  }

  const ViewportManager = {
    HEALTHY_BAR_SPACING,
    HEALTHY_VISIBLE_BARS,
    isPoisonCameraState,
    cameraIntentForTfSwitch,
    resolveHistoryTfFetchEndSec,
    resolveRightHistoryFetchEndSec,
    historyEdgePrefetchBars,
    isWithinRightEdgePrefetch,
    isWithinLeftEdgePrefetch,
    hostHasLayout,
    whenHostHasLayout,
    capture,
    restore,
  };

  global.ViewportManager = ViewportManager;
  if (typeof module !== 'undefined' && module.exports) {
    if (typeof global.ChartDataStore === 'undefined') {
      global.ChartDataStore = require('../store.js').ChartDataStore;
    }
    module.exports = ViewportManager;
  }
})(typeof window !== 'undefined' ? window : globalThis);
