/**
 * Phase 28 — Store → View bridge (single paint authority for backtest charts).
 */
const ChartProjection = (() => {
  let _needsInitialPaint = true;

  function trySync() {
    if (typeof backtestStore === 'undefined' || !backtestStore.hasBaseLayer()) return;

    const container = document.getElementById('backtest-chart-container');
    if (!container || container.clientWidth === 0 || container.clientHeight === 0) return;

    let intent = backtestStore.consumeViewDirty();

    if (!intent) {
      if (_needsInitialPaint) {
        intent = { mode: 'full', viewport: 'fresh' };
      } else {
        if (typeof ChartAdapter !== 'undefined') ChartAdapter.activateSurface('backtest');
        return;
      }
    }

    ChartAdapter.ensureBacktestChart();
    ChartAdapter.activateSurface('backtest');

    const storeData = backtestStore.getForLightweightCharts();

    if (intent.mode === 'full') {
      ChartAdapter.applyFullData('backtest', storeData);
    } else if (intent.mode === 'overlay') {
      ChartAdapter.applySimOverlay('backtest', intent.navigators ? { navigators: intent.navigators } : {});
    }

    ChartAdapter.applyBacktestMarkers(backtestStore.getTrades(), storeData.osc);

    if (typeof NavigatorController !== 'undefined') {
      NavigatorController.renderChartLegends('backtest');
    }

    if (intent.viewport === 'fresh' || _needsInitialPaint) {
      const chartInstance = ChartAdapter.getChart('backtest');
      if (chartInstance) {
        chartInstance.timeScale().scrollToPosition(0, false);
      }
    } else if (intent.viewport === 'restore' && typeof ViewportManager !== 'undefined') {
      const anchor = intent.anchor;
      if (anchor) ViewportManager.restore('backtest', anchor, backtestStore);
    }

    const chartHandle = ChartAdapter.getChartHandle ? ChartAdapter.getChartHandle('backtest') : null;
    if (chartHandle && chartHandle.candleSeries) {
      _needsInitialPaint = false;
    }
  }

  function bindContainerResizeObserver() {
    const containerNode = document.getElementById('backtest-chart-container');
    if (!containerNode || containerNode._chartProjectionRoBound) return;
    containerNode._chartProjectionRoBound = true;
    new ResizeObserver(() => {
      if (containerNode.clientWidth > 0 && containerNode.clientHeight > 0) {
        trySync();
      }
    }).observe(containerNode);
  }

  if (typeof document !== 'undefined') {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', bindContainerResizeObserver, { once: true });
    } else {
      bindContainerResizeObserver();
    }
  }

  return {
    trySync,
  };
})();

if (typeof window !== 'undefined') {
  window.ChartProjection = ChartProjection;
}
