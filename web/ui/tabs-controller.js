/**
 * Main tab switching. Dashboard is Live-only after PRE-STRATEGY-CLEAN-1
 * amputated the Falcon-era Backtest/Stats tabs.
 */
const TabsController = (() => {
  function getActiveTabId() {
    return 'tab-live';
  }

  function isLiveTabActive() {
    return true;
  }

  function getActiveStrategyContext() {
    return 'live';
  }

  function applyToolbarVisibility() {
    const toolbar = document.querySelector('.toolbar');
    if (toolbar) toolbar.style.display = '';
  }

  function switchTab() {
    applyToolbarVisibility();
    if (typeof ChartAdapter !== 'undefined' && ChartAdapter.resetRuler) {
      ChartAdapter.resetRuler();
    } else if (typeof resetRuler === 'function') {
      resetRuler();
    } else if (typeof RulerController !== 'undefined' && RulerController.isActive()) {
      RulerController.disarm();
      document.getElementById('ruler-btn')?.classList.remove('active');
    }
    if (typeof TimeframeController !== 'undefined') {
      TimeframeController.syncToolbar();
    }
  }

  function init() {
    const tabs = document.querySelectorAll('.tabs-nav .tab-btn');
    tabs.forEach((btn) => {
      btn.addEventListener('click', () => switchTab());
    });
    try {
      switchTab();
    } catch (err) {
      console.error('[TabsController] switchTab failed:', err);
    }
  }

  return {
    init,
    switchTab,
    getActiveTabId,
    isLiveTabActive,
    getActiveStrategyContext,
    applyToolbarVisibility,
  };
})();

if (typeof window !== 'undefined') {
  window.TabsController = TabsController;
}
