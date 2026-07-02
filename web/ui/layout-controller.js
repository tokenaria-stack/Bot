/**
 * Phase 19.5.6 — Chart pane stack resize (live + backtest).
 */
const LayoutController = (() => {
  function validPaneFlexGrow(value, fallback) {
    const n = Number(value);
    return Number.isFinite(n) && n > 0 ? n : fallback;
  }

  function loadHeightsForStack(stackKey) {
    const cfg = PANE_STACK_CONFIG[stackKey];
    if (!cfg) return;
    try {
      const raw = localStorage.getItem(cfg.lsKey);
      const defaults = cfg.defaults;
      const h = raw ? JSON.parse(raw) : defaults;
      const price = validPaneFlexGrow(h?.price, defaults.price);
      const osc = validPaneFlexGrow(h?.osc, defaults.osc);
      const rsx = validPaneFlexGrow(h?.rsx, defaults.rsx);
      const priceEl = document.getElementById(cfg.price);
      const oscEl = document.getElementById(cfg.osc);
      const rsxEl = document.getElementById(cfg.rsx);
      if (priceEl) priceEl.style.flex = `${price} 1 0`;
      if (oscEl) oscEl.style.flex = `${osc} 1 0`;
      if (rsxEl) rsxEl.style.flex = `${rsx} 1 0`;
    } catch { /* noop */ }
  }

  function loadPaneHeights() {
    loadHeightsForStack('live');
    loadHeightsForStack('backtest');
  }

  function saveHeightsForStack(stackKey) {
    const cfg = PANE_STACK_CONFIG[stackKey];
    if (!cfg) return;
    const priceEl = document.getElementById(cfg.price);
    const oscEl = document.getElementById(cfg.osc);
    const rsxEl = document.getElementById(cfg.rsx);
    if (!priceEl || !oscEl || !rsxEl) return;
    const price = parseFloat(getComputedStyle(priceEl).flexGrow) || cfg.defaults.price;
    const osc = parseFloat(getComputedStyle(oscEl).flexGrow) || cfg.defaults.osc;
    const rsx = parseFloat(getComputedStyle(rsxEl).flexGrow) || cfg.defaults.rsx;
    localStorage.setItem(cfg.lsKey, JSON.stringify({ price, osc, rsx }));
  }

  function resizeCharts() {
    if (typeof ChartAdapter?.resizeAllCharts === 'function') {
      ChartAdapter.resizeAllCharts();
    }
  }

  function initPaneResize() {
    document.querySelectorAll('.pane-resize').forEach((handle) => {
      if (handle._paneResizeBound) return;
      handle._paneResizeBound = true;
      const stackKey = handle.dataset.paneStack || 'live';
      const cfg = PANE_STACK_CONFIG[stackKey];
      if (!cfg) return;

      handle.addEventListener('mousedown', (e) => {
        e.preventDefault();
        const kind = handle.dataset.resize;
        const priceWrap = document.getElementById(cfg.price);
        const oscWrap = document.getElementById(cfg.osc);
        const rsxWrap = document.getElementById(cfg.rsx);
        if (!priceWrap || !oscWrap || !rsxWrap) return;

        const startY = e.clientY;
        const startPrice = priceWrap.getBoundingClientRect().height;
        const startOsc = oscWrap.getBoundingClientRect().height;
        const startRsx = rsxWrap.getBoundingClientRect().height;
        handle.classList.add('dragging');

        function onMove(ev) {
          const dy = ev.clientY - startY;
          const minH = 72;
          if (kind === 'price-osc') {
            const newPrice = Math.max(minH, startPrice + dy);
            const newOsc = Math.max(minH, startOsc - dy);
            priceWrap.style.flex = `0 0 ${newPrice}px`;
            oscWrap.style.flex = `0 0 ${newOsc}px`;
          } else if (kind === 'osc-rsx') {
            const newOsc = Math.max(minH, startOsc + dy);
            const newRsx = Math.max(minH, startRsx - dy);
            oscWrap.style.flex = `0 0 ${newOsc}px`;
            rsxWrap.style.flex = `0 0 ${newRsx}px`;
          }
          resizeCharts();
        }

        function onUp() {
          handle.classList.remove('dragging');
          document.removeEventListener('mousemove', onMove);
          document.removeEventListener('mouseup', onUp);
          const price = priceWrap.getBoundingClientRect().height;
          const osc = oscWrap.getBoundingClientRect().height;
          const rsx = rsxWrap.getBoundingClientRect().height;
          priceWrap.style.flex = `${price} 1 0`;
          oscWrap.style.flex = `${osc} 1 0`;
          rsxWrap.style.flex = `${rsx} 1 0`;
          saveHeightsForStack(stackKey);
          resizeCharts();
        }

        document.addEventListener('mousemove', onMove);
        document.addEventListener('mouseup', onUp);
      });
    });
  }

  function init() {
    initPaneResize();
    loadPaneHeights();
  }

  return {
    init,
    loadPaneHeights,
  };
})();

window.LayoutController = LayoutController;
