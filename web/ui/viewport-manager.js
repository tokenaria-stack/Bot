/**
 * ViewportManager — Unix-ms time-anchored capture/restore (Core 3.0 Multi-Chart).
 * Shot 6D: poison recovery clamps density only; preserves centerTimeMs (no right-edge force).
 * No logical-index hacks. No try/catch. Deterministic binary search on store.times.
 */
(function initViewportManager(global) {
  /** Comfortable candle width — SSOT for cold boot + poison recovery. */
  const HEALTHY_BAR_SPACING = 6;
  /** Above this → camera crushed all history into one screen (accordion). */
  const MAX_HEALTHY_VISIBLE_BARS = 400;
  /** Below this → LWC crushed barSpacing (poison). */
  const MIN_HEALTHY_BAR_SPACING = 1;
  /** Default window when recovering / right-edge restore. */
  const HEALTHY_VISIBLE_BARS = 150;

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

  /** Nearest index in ascending unix-seconds array for target unix-ms. */
  function findIndexByTimeMs(timesSec, centerTimeMs) {
    if (!timesSec.length || centerTimeMs == null || !Number.isFinite(centerTimeMs)) {
      return 0;
    }
    const targetSec = centerTimeMs / 1000;
    let lo = 0;
    let hi = timesSec.length - 1;
    while (lo < hi) {
      const mid = (lo + hi) >> 1;
      if (Number(timesSec[mid]) < targetSec) lo = mid + 1;
      else hi = mid;
    }
    if (lo > 0) {
      const prevDelta = Math.abs(Number(timesSec[lo - 1]) - targetSec);
      const currDelta = Math.abs(Number(timesSec[lo]) - targetSec);
      if (prevDelta < currDelta) return lo - 1;
    }
    return lo;
  }

  /**
   * Poison = accordion leftovers from fitContent / extreme zoom.
   * @param {{ barSpacing?: number|null, visibleBars?: number, from?: number }} state
   */
  function isPoisonCameraState(state) {
    if (!state) return true;
    if (Number.isFinite(state.from) && state.from < 0) return true;
    if (Number.isFinite(state.barSpacing) && state.barSpacing < MIN_HEALTHY_BAR_SPACING) return true;
    if (Number.isFinite(state.visibleBars) && state.visibleBars > MAX_HEALTHY_VISIBLE_BARS) return true;
    return false;
  }

  function applyBarSpacingAll(context, barSpacing) {
    if (barSpacing == null || !Number.isFinite(barSpacing)) return;
    if (typeof ChartAdapter === 'undefined') return;
    ['price', 'wozduh', 'rsx'].forEach((pane) => {
      ChartAdapter.getChart(context, pane)?.timeScale()?.applyOptions({ barSpacing });
    });
  }

  function _paneScrollToRight(context) {
    ['price', 'wozduh', 'rsx'].forEach((pane) => {
      ChartAdapter.getChart(context, pane)?.timeScale()?.scrollToPosition(0, false);
    });
  }

  const ViewportManager = {
    HEALTHY_BAR_SPACING,
    HEALTHY_VISIBLE_BARS,
    isPoisonCameraState,

    capture(context) {
      const range = typeof ChartAdapter !== 'undefined'
        ? ChartAdapter.getVisibleLogicalRange(context)
        : null;
      if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to)) return null;

      const store = storeForContext(context);
      const times = timesSecFromStore(store);
      if (!times.length) return null;

      let visibleBars = range.to - range.from;
      const centerIndex = Math.floor((range.from + range.to) / 2);
      const clampedIndex = Math.max(0, Math.min(times.length - 1, centerIndex));
      const centerSec = Number(times[clampedIndex]);
      if (!Number.isFinite(centerSec)) return null;
      const centerTimeMs = Math.floor(centerSec * 1000);

      const mainChart = typeof ChartAdapter !== 'undefined'
        ? ChartAdapter.getChart(context, 'price')
        : null;
      const timeScale = mainChart?.timeScale();
      const scrollPos = timeScale?.scrollPosition?.() ?? 0;
      let barSpacing = timeScale?.options()?.barSpacing ?? null;

      // Scalpel: trim density only — never discard centerTimeMs / force right edge.
      if (isPoisonCameraState({ barSpacing, visibleBars, from: range.from })) {
        barSpacing = HEALTHY_BAR_SPACING;
        visibleBars = Math.max(50, Math.min(visibleBars, MAX_HEALTHY_VISIBLE_BARS));
      }

      const nearRight = scrollPos >= -1;
      const nearLastIndex = clampedIndex >= times.length - 3;
      const isAtRightEdge = nearRight && nearLastIndex && range.from >= 0;

      return {
        centerTimeMs,
        visibleBars,
        isAtRightEdge,
        barSpacing: Number.isFinite(barSpacing) ? barSpacing : HEALTHY_BAR_SPACING,
      };
    },

    restore(context, anchor, store) {
      if (!anchor || anchor.centerTimeMs == null) return;
      if (typeof ChartAdapter === 'undefined') return;

      const targetStore = store || storeForContext(context);
      const times = timesSecFromStore(targetStore);
      if (!times.length) return;

      const mainChart = ChartAdapter.getChart(context, 'price');
      if (!mainChart?.timeScale) return;

      // Defense in depth: clamp density only — never discard centerTimeMs / force right edge.
      let safeAnchor = { ...anchor };
      if (isPoisonCameraState({
        barSpacing: anchor.barSpacing,
        visibleBars: anchor.visibleBars,
        from: 0,
      })) {
        safeAnchor.barSpacing = HEALTHY_BAR_SPACING;
        const currentBars = Number(anchor.visibleBars) || HEALTHY_VISIBLE_BARS;
        safeAnchor.visibleBars = Math.max(50, Math.min(currentBars, MAX_HEALTHY_VISIBLE_BARS));
      }

      const spacing = Number.isFinite(safeAnchor.barSpacing)
        ? safeAnchor.barSpacing
        : HEALTHY_BAR_SPACING;
      applyBarSpacingAll(context, spacing);

      if (safeAnchor.isAtRightEdge) {
        const n = times.length;
        const visible = Number(safeAnchor.visibleBars) > 0
          ? Number(safeAnchor.visibleBars)
          : HEALTHY_VISIBLE_BARS;
        if (typeof ChartAdapter.setVisibleLogicalRange === 'function' && n > 0) {
          const from = Math.max(0, n - visible);
          ChartAdapter.setVisibleLogicalRange(context, { from, to: n }, { animate: false });
        } else {
          _paneScrollToRight(context);
        }
        return;
      }

      const newCenterIndex = findIndexByTimeMs(times, safeAnchor.centerTimeMs);
      const half = (Number(safeAnchor.visibleBars) || HEALTHY_VISIBLE_BARS) / 2;
      const n = times.length;
      let from = newCenterIndex - half;
      let to = newCenterIndex + half;
      if (from < 0) {
        to -= from;
        from = 0;
      }
      if (to > n) {
        const over = to - n;
        from = Math.max(0, from - over);
        to = n;
      }
      if (from >= to) {
        from = Math.max(0, n - (Number(safeAnchor.visibleBars) || HEALTHY_VISIBLE_BARS));
        to = n;
      }

      if (typeof ChartAdapter.setVisibleLogicalRange === 'function') {
        ChartAdapter.setVisibleLogicalRange(context, { from, to }, { animate: false });
      } else {
        mainChart.timeScale().setVisibleLogicalRange({ from, to });
      }
    },
  };

  global.ViewportManager = ViewportManager;
})(typeof window !== 'undefined' ? window : globalThis);
