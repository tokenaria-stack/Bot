/**
 * Phase 28 — Store → View bridge (single paint authority for backtest charts).
 */
const ChartProjection = (() => {
  function renderBacktest(options = {}) {
    const { mode = 'full', reason = 'tab_activate', navigators = null } = options;

    if (!backtestStore.hasBaseLayer()) return;

    // 1. СНАЧАЛА СОЗДАЕМ ГРАФИК (чтобы привязать его к DOM и получить .root)
    ChartAdapter.ensureBacktestChart();

    // 2. ПОТОМ ПРОВЕРЯЕМ ЕГО ГЕОМЕТРИЮ (Вентиль)
    if (typeof ChartAdapter !== 'undefined' && typeof ChartAdapter.activateSurface === 'function') {
      const isReady = ChartAdapter.activateSurface('backtest');
      if (!isReady) {
        console.warn('[Projection] Surface not ready (0x0). Render deferred until tab activation.');
        return;
      }
    }

    let anchor = null;
    if (reason === 'tab_activate' && typeof ViewportManager !== 'undefined') {
      anchor = ViewportManager.capture('backtest');
    }

    const storeData = backtestStore.getForLightweightCharts();
    const trades = typeof backtestStore.getTrades === 'function' ? backtestStore.getTrades() : [];
    const chartHandle = typeof ChartAdapter.getChartHandle === 'function' ? ChartAdapter.getChartHandle('backtest') : null;
    // #region agent log
    fetch('http://127.0.0.1:7650/ingest/e96d7e9c-02c2-4eef-b8f6-4424f0be67d3',{method:'POST',headers:{'Content-Type':'application/json','X-Debug-Session-Id':'39f875'},body:JSON.stringify({sessionId:'39f875',runId:'bt-black-screen-diagnosis',hypothesisId:'H5',location:'web/ui/chart-projection.js:renderBacktest',message:'Projection render payload',data:{mode,reason,candles:storeData?.candles?.length||0,osc:storeData?.osc?.length||0,annotations:storeData?.annotations?.length||0,trades:trades?.length||0,chartReady:!!chartHandle?.chart,rootW:chartHandle?.root?.clientWidth??null,rootH:chartHandle?.root?.clientHeight??null},timestamp:Date.now()})}).catch(()=>{});
    // #endregion

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
