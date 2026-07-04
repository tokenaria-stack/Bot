/**
 * ViewportManager — time-anchored viewport capture/restore (no logical-index shift hacks).
 */
(function initViewportManager(global) {
  function storeForContext(context) {
    return context === 'backtest' ? backtestStore : liveStore;
  }

  function candlesArray(store) {
    if (typeof store.candlesArray === 'function') {
      return store.candlesArray();
    }
    return [];
  }

  function findIndexByTimeMs(candles, centerTimeMs) {
    if (!candles.length || centerTimeMs == null) return 0;

    let lo = 0;
    let hi = candles.length - 1;
    while (lo < hi) {
      const mid = Math.floor((lo + hi) / 2);
      if (candles[mid].timeMs < centerTimeMs) lo = mid + 1;
      else hi = mid;
    }

    if (lo > 0) {
      const prevDelta = Math.abs(candles[lo - 1].timeMs - centerTimeMs);
      const currDelta = Math.abs(candles[lo].timeMs - centerTimeMs);
      if (prevDelta < currDelta) return lo - 1;
    }
    return lo;
  }

  function chartHandle(context) {
    return typeof ChartAdapter !== 'undefined' ? ChartAdapter.getChartHandle(context) : null;
  }

  const ViewportManager = {
    capture(context) {
      const range = typeof ChartAdapter !== 'undefined'
        ? ChartAdapter.getVisibleLogicalRange(context)
        : null;
      if (!range) return null;

      const store = storeForContext(context);
      const candles = candlesArray(store);
      if (!candles.length) return null;

      const centerIndex = Math.floor((range.from + range.to) / 2);
      const clampedIndex = Math.max(0, Math.min(candles.length - 1, centerIndex));
      const centerTimeMs = candles[clampedIndex]?.timeMs;
      if (centerTimeMs == null) return null;

      const mainChart = typeof ChartAdapter !== 'undefined'
        ? ChartAdapter.getChart(context, 'price')
        : null;
      const timeScale = mainChart?.timeScale();
      const scrollPos = timeScale?.scrollPosition?.() ?? 0;
      const isAtRightEdge = scrollPos >= -1;
      const barSpacing = timeScale?.options()?.barSpacing ?? null;

      return {
        centerTimeMs,
        visibleBars: range.to - range.from,
        isAtRightEdge,
        barSpacing,
      };
    },

    restore(context, anchor, store) {
      if (!anchor) return;

      const chartData = chartHandle(context);
      if (!chartData?.chart?.timeScale) return;

      const candles = candlesArray(store);
      if (!candles.length) return;

      if (anchor.barSpacing != null && Number.isFinite(anchor.barSpacing)) {
        chartData.chart.timeScale().applyOptions({ barSpacing: anchor.barSpacing });
      }

      if (anchor.isAtRightEdge) {
        chartData.chart.timeScale().scrollToPosition(0, false);
        return;
      }

      const newCenterIndex = findIndexByTimeMs(candles, anchor.centerTimeMs);
      const half = anchor.visibleBars / 2;
      const range = {
        from: newCenterIndex - half,
        to: newCenterIndex + half,
      };

      ChartAdapter.syncVisibleLogicalRange(chartData.chart, range, { animate: false });
    },
  };

  global.ViewportManager = ViewportManager;
})(typeof window !== 'undefined' ? window : globalThis);
