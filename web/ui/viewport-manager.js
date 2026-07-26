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
    if (Number.isFinite(state.visibleBars) && state.visibleBars > MAX_HEALTHY_VISIBLE_BARS) return true;
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
      visibleBars = Math.max(50, Math.min(visibleBars, MAX_HEALTHY_VISIBLE_BARS));
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
   * @param {object|null} captured
   */
  function cameraIntentForTfSwitch(captured) {
    if (!captured || captured.centerTimeMs == null) return null;
    return {
      centerTimeMs: captured.centerTimeMs,
      visibleBars: captured.visibleBars || HEALTHY_VISIBLE_BARS,
      rightOffset: Number.isFinite(captured.rightOffset) ? captured.rightOffset : 0,
      rightPadding: Number.isFinite(captured.rightPadding)
        ? captured.rightPadding
        : (Number.isFinite(captured.rightOffset) ? captured.rightOffset : 0),
      barSpacing: captured.barSpacing,
      isAtRightEdge: captured.isAtRightEdge === true || captured.intent === 'LIVE',
      intent: captured.intent === 'HISTORY' ? 'HISTORY' : 'LIVE',
    };
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
    hostHasLayout,
    whenHostHasLayout,
    capture,
    restore,
  };

  global.ViewportManager = ViewportManager;
})(typeof window !== 'undefined' ? window : globalThis);
