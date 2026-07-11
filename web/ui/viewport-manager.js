/**
 * ViewportManager — Unix-ms time-anchored capture/restore (Core 3.0 Multi-Chart).
 * No logical-index hacks. No try/catch. Deterministic binary search on store.times.
 */
(function initViewportManager(global) {
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

  const ViewportManager = {
    capture(context) {
      const range = typeof ChartAdapter !== 'undefined'
        ? ChartAdapter.getVisibleLogicalRange(context)
        : null;
      if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to)) return null;

      const store = storeForContext(context);
      const times = timesSecFromStore(store);
      if (!times.length) return null;

      const centerIndex = Math.floor((range.from + range.to) / 2);
      const clampedIndex = Math.max(0, Math.min(times.length - 1, centerIndex));
      const centerSec = Number(times[clampedIndex]);
      if (!Number.isFinite(centerSec)) return null;

      const mainChart = typeof ChartAdapter !== 'undefined'
        ? ChartAdapter.getChart(context, 'price')
        : null;
      const timeScale = mainChart?.timeScale();
      const scrollPos = timeScale?.scrollPosition?.() ?? 0;
      const isAtRightEdge = scrollPos >= -1;
      const barSpacing = timeScale?.options()?.barSpacing ?? null;

      return {
        centerTimeMs: Math.floor(centerSec * 1000),
        visibleBars: range.to - range.from,
        isAtRightEdge,
        barSpacing,
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

      if (anchor.barSpacing != null && Number.isFinite(anchor.barSpacing)) {
        mainChart.timeScale().applyOptions({ barSpacing: anchor.barSpacing });
      }

      if (anchor.isAtRightEdge) {
        _paneScrollToRight(context);
        return;
      }

      const newCenterIndex = findIndexByTimeMs(times, anchor.centerTimeMs);
      const half = (Number(anchor.visibleBars) || 80) / 2;
      const range = {
        from: newCenterIndex - half,
        to: newCenterIndex + half,
      };

      if (typeof ChartAdapter.setVisibleLogicalRange === 'function') {
        ChartAdapter.setVisibleLogicalRange(context, range, { animate: false });
      } else {
        ChartAdapter.syncVisibleLogicalRange(mainChart, range, { animate: false });
      }
    },
  };

  function _paneScrollToRight(context) {
    ['price', 'wozduh', 'rsx'].forEach((pane) => {
      ChartAdapter.getChart(context, pane)?.timeScale()?.scrollToPosition(0, false);
    });
  }

  global.ViewportManager = ViewportManager;
})(typeof window !== 'undefined' ? window : globalThis);
