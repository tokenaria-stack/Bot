/**
 * Phase 19.5 — Main tab switching (Live / Stats / Backtest).
 * DOM class toggles only; chart orchestration delegates to app.js globals at runtime.
 */
const TabsController = (() => {
  function getActiveTabId() {
    const active = document.querySelector('.tab-content.active');
    return active?.id || 'tab-live';
  }

  function isBacktestTabActive() {
    return getActiveTabId() === 'tab-backtest';
  }

  function isBacktestTfContext() {
    const tab = getActiveTabId();
    return tab === 'tab-backtest' || tab === 'tab-stats';
  }

  function isLiveTabActive() {
    return getActiveTabId() === 'tab-live';
  }

  function getActiveStrategyContext() {
    return isBacktestTfContext() ? 'backtest' : 'live';
  }

  function applyToolbarVisibility(targetId) {
    const toolbar = document.querySelector('.toolbar');
    if (toolbar) {
      toolbar.style.display = targetId === 'tab-stats' ? 'none' : '';
    }
    const backtestControls = document.getElementById('backtest-controls');
    if (backtestControls) {
      backtestControls.classList.toggle('visible', targetId === 'tab-backtest');
    }
  }

  function switchTab(targetId) {
    if (typeof StrategyController !== 'undefined') {
      StrategyController.saveThresholdsFromHeaderToState(getActiveStrategyContext());
    }

    const tabs = document.querySelectorAll('.tabs-nav .tab-btn');
    const panels = document.querySelectorAll('.tab-content');
    tabs.forEach((b) => b.classList.toggle('active', b.dataset.tab === targetId));
    panels.forEach((panel) => {
      const isActive = panel.id === targetId;
      panel.classList.toggle('active', isActive);
      panel.style.display = isActive ? '' : 'none';
    });

    applyToolbarVisibility(targetId);

    if (typeof ruler !== 'undefined' && ruler.active) {
      ruler.active = false;
      document.getElementById('ruler-btn')?.classList.remove('active');
      if (typeof setRulerCursor === 'function') setRulerCursor(false);
    }
    if (typeof resetRuler === 'function') resetRuler();

    if (typeof TimeframeController !== 'undefined') {
      TimeframeController.syncToolbar();
    }
    if (typeof ChartAdapter !== 'undefined') {
      ChartAdapter.applyWozduhVisibility(getActiveUiContext());
    }

    const nextStrategyContext = (targetId === 'tab-backtest' || targetId === 'tab-stats') ? 'backtest' : 'live';
    if (typeof StrategyController !== 'undefined') {
      StrategyController.applyThresholdsToHeader(
        StrategyController.getStrategyState(nextStrategyContext).thresholds,
      );
    }

    if (targetId === 'tab-live' && typeof pushRsxSettingsToServer === 'function') {
      pushRsxSettingsToServer(coerceRsxSettingsForAPI(liveRsxSettings))
        .then(() => {
          if (ChartAdapter.chartInitialized() && isLiveTabActive()) {
            reloadRsxChartFromServer();
          }
        })
        .catch((err) => console.warn('Failed to restore live RSX settings:', err));
    }

    requestAnimationFrame(() => {
      ChartAdapter?.handleResize?.();

      if (targetId === 'tab-live' && ChartAdapter?.getChartHandle('live')?.chart) {
        if (typeof wsSubscribeTf === 'function') wsSubscribeTf(currentTf);
        if (!ChartAdapter.chartInitialized()) {
          loadDashboard();
        } else {
          beginDataUpdate();
          try {
            applySeriesData();
            ChartAdapter.syncLivePanesFromPrice();
          } finally {
            endDataUpdate(0);
          }
          if (typeof shouldRunLivePoll === 'function' && shouldRunLivePoll()) {
            pollLatestState();
          }
        }
      } else if (targetId === 'tab-backtest' && ChartAdapter?.getChartHandle('backtest')?.chart) {
        ChartAdapter.forceSyncTimeScales('backtest');
      } else if (targetId === 'tab-stats') {
        ChartAdapter?.resizeEquity?.();
        ChartAdapter?.fitEquityContent?.();
        if (typeof refreshStatsForMode === 'function') refreshStatsForMode(statsMode);
      }
    });
  }

  function init() {
    const tabs = document.querySelectorAll('.tabs-nav .tab-btn');
    if (!tabs.length) {
      console.warn('[TabsController] No tab buttons found (.tabs-nav .tab-btn)');
      return;
    }
    tabs.forEach((btn) => {
      if (!btn.dataset.tab) {
        console.warn('[TabsController] Tab button missing data-tab attribute', btn);
        return;
      }
      btn.addEventListener('click', () => switchTab(btn.dataset.tab));
    });
    try {
      switchTab('tab-live');
    } catch (err) {
      console.error('[TabsController] switchTab(tab-live) failed:', err);
    }
  }

  return {
    init,
    switchTab,
    getActiveTabId,
    isBacktestTabActive,
    isBacktestTfContext,
    isLiveTabActive,
    getActiveStrategyContext,
    applyToolbarVisibility,
  };
})();

if (typeof window !== 'undefined') {
  window.TabsController = TabsController;
}
