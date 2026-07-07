/**
 * Phase 28 — Store → View bridge (single paint authority for backtest charts).
 */
const ChartProjection = (() => {
  function renderBacktest(options = {}) {
    const { mode = 'full', reason = 'tab_activate', navigators = null } = options;

    if (!backtestStore.hasBaseLayer()) return;

    ChartAdapter.ensureBacktestChart();

    let anchor = null;
    if (reason === 'tab_activate' && typeof ViewportManager !== 'undefined') {
      anchor = ViewportManager.capture('backtest');
    }

    const storeData = backtestStore.getForLightweightCharts();
    const trades = typeof backtestStore.getTrades === 'function' ? backtestStore.getTrades() : [];

    if (mode === 'full') {
      ChartAdapter.applyFullData('backtest', storeData);
    } else if (mode === 'overlay') {
      ChartAdapter.applySimOverlay('backtest', { navigators: navigators || undefined });
    }

    if (navigators && mode === 'full') {
      ChartAdapter.applySimOverlay('backtest', { navigators });
    }

    ChartAdapter.applyBacktestMarkers(trades, storeData.osc);

    if (reason === 'fresh_run') {
      ChartAdapter.fitContent('backtest');
    } else if (reason === 'tab_activate' && anchor && typeof ViewportManager !== 'undefined') {
      ViewportManager.restore('backtest', anchor, backtestStore);
    }
  }

  return {
    renderBacktest,
  };
})();

if (typeof window !== 'undefined') {
  window.ChartProjection = ChartProjection;
}
