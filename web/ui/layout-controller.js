/**
 * LayoutController — ADR-019 Phase 2–5: CSS Grid geometry driven by PaneLayout.
 * Price track is always 1fr; footers use footerHeights (px). Dynamic gutters only
 * between visible panes. Height drag + legend reorder + fullscreen from PaneLayout.
 * No setHostActive here.
 */
(function (global) {
  'use strict';

  const GUTTER_PX = 4;
  const PRICE_MIN_PX = 120;
  const TIME_SCALE_HIT_PX = 28;
  const FULLSCREEN_IGNORE_SEL = [
    'button',
    'input',
    'select',
    'textarea',
    'a',
    'label',
    '.chart-legend',
    '.pane-splitter',
    '.scale-controls',
    '.indicator-settings-menu',
    '.visibility-toggle-btn',
    '.settings-toggle-btn',
    '#orderflow-buffer',
    '#timeline-sync-badge',
    '.ruler-shade',
    '.ruler-tooltip',
    '.navigator-popup',
    '.popup-container',
  ].join(',');

  /** @type {Record<string, { stackId: string, priceId: string, hostWraps: Record<string,string>, chartHosts: Record<string,string> }>} */
  const STACKS = {
    live: {
      stackId: 'charts-stack',
      priceId: 'price-wrap',
      hostWraps: { wozduh: 'osc-wrap', rsx: 'rsx-wrap' },
      chartHosts: {
        price: 'price-chart',
        wozduh: 'wozduh-chart',
        rsx: 'rsx-chart',
      },
    },
    backtest: {
      stackId: 'bt-charts-stack',
      priceId: 'bt-price-wrap',
      hostWraps: { wozduh: 'bt-osc-wrap', rsx: 'bt-rsx-wrap' },
      chartHosts: {
        price: 'bt-price-chart',
        wozduh: 'bt-wozduh-chart',
        rsx: 'bt-rsx-chart',
      },
    },
  };

  /**
   * Pure: build grid-template-rows from PaneLayout state.
   * @param {{ visible: string[], order: string[], footerHeights: Record<string, number> }} state
   * @param {number} [gutterPx]
   * @param {number} [priceMinPx]
   * @returns {string}
   */
  function buildGridTemplateRows(state, gutterPx = GUTTER_PX, priceMinPx = PRICE_MIN_PX) {
    const rows = [`minmax(${priceMinPx}px, 1fr)`];
    if (!state || !Array.isArray(state.order)) return rows.join(' ');
    const visible = new Set(Array.isArray(state.visible) ? state.visible : []);
    const heights = state.footerHeights && typeof state.footerHeights === 'object'
      ? state.footerHeights
      : {};
    for (const hostId of state.order) {
      if (!visible.has(hostId)) continue;
      const h = Number(heights[hostId]);
      const px = Number.isFinite(h) && h > 0 ? Math.round(h) : 180;
      rows.push(`${gutterPx}px`);
      rows.push(`${px}px`);
    }
    return rows.join(' ');
  }

  /** Visible footers in order that have a known wrap in this stack. */
  function visibleHostIds(state, hostWraps) {
    const visible = new Set(Array.isArray(state.visible) ? state.visible : []);
    const out = [];
    for (const hostId of state.order || []) {
      if (!visible.has(hostId)) continue;
      if (!hostWraps[hostId]) continue;
      out.push(hostId);
    }
    return out;
  }

  function clearDynamicGutters(stack) {
    stack.querySelectorAll('.pane-splitter[data-layout-gutter="1"]').forEach((el) => el.remove());
  }

  function trackStateFor(state, hostWraps) {
    const footers = visibleHostIds(state, hostWraps);
    return {
      visible: footers.slice(),
      order: footers.slice(),
      footerHeights: state.footerHeights || {},
    };
  }

  /** Max px for one footer so price keeps PRICE_MIN_PX. */
  function maxFooterHeightFor(stackEl, state, hostId, hostWraps) {
    if (!stackEl) return 800;
    const footers = visibleHostIds(state, hostWraps);
    const heights = state.footerHeights || {};
    let others = 0;
    for (const id of footers) {
      if (id === hostId) continue;
      const h = Number(heights[id]);
      others += Number.isFinite(h) && h > 0 ? h : 180;
    }
    const gutters = footers.length * GUTTER_PX;
    const budget = stackEl.clientHeight - PRICE_MIN_PX - others - gutters;
    const minH = (typeof PaneLayout !== 'undefined' && PaneLayout.FOOTER_HEIGHT_MIN_PX) || 48;
    const maxH = (typeof PaneLayout !== 'undefined' && PaneLayout.FOOTER_HEIGHT_MAX_PX) || 800;
    return Math.max(minH, Math.min(maxH, Math.floor(budget)));
  }

  let paneLayout = null;
  let unsub = null;
  /** @type {null | { hostId: string, context: string, startY: number, startH: number, maxH: number, pointerId: number }} */
  let heightDrag = null;
  /** @type {null | { hostId: string, context: string, pointerId: number, startY: number, started: boolean, beforeHostId: string|null }} */
  let reorderDrag = null;
  let resizeRaf = 0;
  let dragApplyRaf = 0;
  let pendingDragHeight = null;

  function scheduleResize() {
    if (resizeRaf) return;
    const run = () => {
      resizeRaf = 0;
      forceChartResize('live');
      forceChartResize('backtest');
    };
    if (typeof requestAnimationFrame === 'function') {
      resizeRaf = requestAnimationFrame(run);
    } else {
      run();
    }
  }

  function applyTracksOnly(state) {
    for (const context of Object.keys(STACKS)) {
      const cfg = STACKS[context];
      const stack = typeof document !== 'undefined'
        ? document.getElementById(cfg.stackId)
        : null;
      if (!stack) continue;
      stack.style.gridTemplateRows = buildGridTemplateRows(trackStateFor(state, cfg.hostWraps));
    }
    scheduleResize();
  }

  function clearReorderHighlights(context) {
    const cfg = STACKS[context];
    if (!cfg || typeof document === 'undefined') return;
    for (const wrapId of Object.values(cfg.hostWraps)) {
      document.getElementById(wrapId)?.classList.remove('is-reorder-source', 'is-reorder-target');
    }
  }

  /** @returns {string|null} host to insert before, or null = end of visible list */
  function dropBeforeHostAtY(context, clientY, movingHostId) {
    const cfg = STACKS[context];
    if (!cfg || !paneLayout) return null;
    const state = paneLayout.getState();
    const footers = visibleHostIds(state, cfg.hostWraps).filter((id) => id !== movingHostId);
    for (const id of footers) {
      const wrap = document.getElementById(cfg.hostWraps[id]);
      if (!wrap || wrap.hidden) continue;
      const rect = wrap.getBoundingClientRect();
      if (clientY < rect.top + rect.height / 2) return id;
    }
    return null;
  }

  function isOnTimeScale(host, clientY) {
    if (!host) return false;
    const rect = host.getBoundingClientRect();
    return clientY >= rect.bottom - TIME_SCALE_HIT_PX;
  }

  function isOnPriceScale(host, context, clientX) {
    if (!host) return false;
    const wrap = host.closest?.('.chart-wrap');
    let pane = 'price';
    if (wrap?.dataset?.paneHost) {
      pane = wrap.dataset.paneHost;
    } else if (wrap?.id?.includes('price')) {
      pane = 'price';
    } else {
      const legend = wrap?.querySelector?.('.chart-legend[data-pane]');
      if (legend?.dataset?.pane) pane = legend.dataset.pane;
    }
    let chart = null;
    if (typeof ChartAdapter !== 'undefined' && ChartAdapter.getChart) {
      chart = ChartAdapter.getChart(context === 'live' ? 'live' : context, pane);
    }
    if (typeof ScaleController !== 'undefined' && ScaleController.isPointerOnPriceScale) {
      return ScaleController.isPointerOnPriceScale(host, chart, clientX);
    }
    const rect = host.getBoundingClientRect();
    return clientX >= rect.right - 56;
  }

  function paneIdFromWrap(context, wrap) {
    const cfg = STACKS[context];
    if (!cfg || !wrap) return null;
    if (wrap.id === cfg.priceId) return 'price';
    if (wrap.dataset.paneHost) return wrap.dataset.paneHost;
    const legend = wrap.querySelector('.chart-legend[data-pane]');
    const fromLegend = legend?.dataset?.pane;
    if (fromLegend && fromLegend !== 'price') return fromLegend;
    return null;
  }

  function bindFullscreenToggle(context) {
    const cfg = STACKS[context];
    if (!cfg || typeof document === 'undefined') return;
    const stack = document.getElementById(cfg.stackId);
    if (!stack || stack.dataset.fullscreenBound === '1') return;
    stack.dataset.fullscreenBound = '1';

    stack.addEventListener('dblclick', (e) => {
      if (!paneLayout || typeof paneLayout.toggleFullscreen !== 'function') return;
      if (heightDrag || reorderDrag) return;

      const wrap = e.target.closest?.('.chart-wrap');
      if (!wrap || !stack.contains(wrap)) return;
      if (e.target.closest?.(FULLSCREEN_IGNORE_SEL)) return;

      const host = wrap.querySelector('.lwc-host');
      // Empty plot chrome only: event must originate from the LWC host (canvas/plot).
      if (!host || !(e.target === host || host.contains(e.target))) return;
      if (isOnPriceScale(host, context, e.clientX)) return;
      if (isOnTimeScale(host, e.clientY)) return;

      const paneId = paneIdFromWrap(context, wrap);
      if (!paneId) return;

      e.preventDefault();
      e.stopPropagation();
      paneLayout.toggleFullscreen(paneId);
    });
  }

  function bindFooterReorder(context, hostId, legendEl) {
    if (!legendEl || legendEl.dataset.reorderBound === '1') return;
    legendEl.dataset.reorderBound = '1';
    legendEl.classList.add('chart-legend--reorderable');

    legendEl.addEventListener('pointerdown', (e) => {
      if (e.button != null && e.button !== 0) return;
      if (e.target.closest('button')) return;
      if (!paneLayout || typeof paneLayout.moveHostBefore !== 'function') return;
      if (heightDrag || reorderDrag) return;

      e.preventDefault();
      const wrap = legendEl.closest('.chart-wrap');
      reorderDrag = {
        hostId,
        context,
        pointerId: e.pointerId,
        startY: e.clientY,
        started: false,
        beforeHostId: null,
      };
      try {
        legendEl.setPointerCapture(e.pointerId);
      } catch {
        /* */
      }

      const onMove = (moveEvent) => {
        if (!reorderDrag || moveEvent.pointerId !== reorderDrag.pointerId) return;
        const dy = Math.abs(moveEvent.clientY - reorderDrag.startY);
        if (!reorderDrag.started) {
          if (dy < 5) return;
          reorderDrag.started = true;
          document.body?.classList.add('is-pane-reordering');
          wrap?.classList.add('is-reorder-source');
        }
        const before = dropBeforeHostAtY(context, moveEvent.clientY, hostId);
        if (before === reorderDrag.beforeHostId) return;
        reorderDrag.beforeHostId = before;
        clearReorderHighlights(context);
        wrap?.classList.add('is-reorder-source');
        if (before) {
          const cfg = STACKS[context];
          document.getElementById(cfg.hostWraps[before])?.classList.add('is-reorder-target');
        }
      };

      const onUp = (upEvent) => {
        if (reorderDrag && upEvent.pointerId !== reorderDrag.pointerId) return;
        legendEl.removeEventListener('pointermove', onMove);
        legendEl.removeEventListener('pointerup', onUp);
        legendEl.removeEventListener('pointercancel', onUp);
        try {
          legendEl.releasePointerCapture(upEvent.pointerId);
        } catch {
          /* */
        }
        const session = reorderDrag;
        reorderDrag = null;
        document.body?.classList.remove('is-pane-reordering');
        clearReorderHighlights(context);
        if (!session?.started) return;
        paneLayout.moveHostBefore(session.hostId, session.beforeHostId);
        // subscribe → full apply + resize
      };

      legendEl.addEventListener('pointermove', onMove);
      legendEl.addEventListener('pointerup', onUp);
      legendEl.addEventListener('pointercancel', onUp);
    });
  }

  function bindSplitterDrag(gutter, context, hostId) {
    gutter.addEventListener('pointerdown', (e) => {
      if (e.button != null && e.button !== 0) return;
      if (!paneLayout || typeof paneLayout.setFooterHeight !== 'function') return;
      if (reorderDrag) return;
      e.preventDefault();
      e.stopPropagation();

      const cfg = STACKS[context];
      const stack = document.getElementById(cfg.stackId);
      const state = paneLayout.getState();
      const startH = Number(state.footerHeights?.[hostId]);
      const fallback = (typeof PaneLayout !== 'undefined' && PaneLayout.DEFAULT_FOOTER_HEIGHT_PX) || 180;
      heightDrag = {
        hostId,
        context,
        startY: e.clientY,
        startH: Number.isFinite(startH) && startH > 0 ? startH : fallback,
        maxH: maxFooterHeightFor(stack, state, hostId, cfg.hostWraps),
        pointerId: e.pointerId,
      };
      pendingDragHeight = heightDrag.startH;
      try {
        gutter.setPointerCapture(e.pointerId);
      } catch {
        /* */
      }
      document.body?.classList.add('is-pane-resizing');

      const onMove = (moveEvent) => {
        if (!heightDrag || moveEvent.pointerId !== heightDrag.pointerId) return;
        const dy = moveEvent.clientY - heightDrag.startY;
        // Splitter sits above this footer: drag down → shorter footer; drag up → taller
        // (price 1fr absorbs the difference).
        const minH = (typeof PaneLayout !== 'undefined' && PaneLayout.FOOTER_HEIGHT_MIN_PX) || 48;
        pendingDragHeight = Math.max(minH, Math.min(heightDrag.maxH, Math.round(heightDrag.startH - dy)));
        if (dragApplyRaf) return;
        dragApplyRaf = requestAnimationFrame(() => {
          dragApplyRaf = 0;
          if (!heightDrag || pendingDragHeight == null) return;
          paneLayout.setFooterHeight(heightDrag.hostId, pendingDragHeight);
        });
      };

      const onUp = (upEvent) => {
        if (heightDrag && upEvent.pointerId !== heightDrag.pointerId) return;
        gutter.removeEventListener('pointermove', onMove);
        gutter.removeEventListener('pointerup', onUp);
        gutter.removeEventListener('pointercancel', onUp);
        try {
          if (heightDrag) gutter.releasePointerCapture(heightDrag.pointerId);
        } catch {
          /* */
        }
        if (dragApplyRaf) {
          cancelAnimationFrame(dragApplyRaf);
          dragApplyRaf = 0;
        }
        if (heightDrag && pendingDragHeight != null && paneLayout) {
          paneLayout.setFooterHeight(heightDrag.hostId, pendingDragHeight);
        }
        heightDrag = null;
        pendingDragHeight = null;
        document.body?.classList.remove('is-pane-resizing');
        // Full apply once after drag (safe to rebuild gutters).
        apply();
      };

      gutter.addEventListener('pointermove', onMove);
      gutter.addEventListener('pointerup', onUp);
      gutter.addEventListener('pointercancel', onUp);
    });
  }

  function applyStack(context, state) {
    const cfg = STACKS[context];
    if (!cfg || typeof document === 'undefined') return;
    const stack = document.getElementById(cfg.stackId);
    if (!stack) return;

    stack.classList.add('charts-stack--grid');
    stack.classList.toggle('charts-stack--fullscreen', !!state.fullscreenPaneId);
    const footers = visibleHostIds(state, cfg.hostWraps);
    stack.style.display = 'grid';
    stack.style.gridTemplateColumns = 'minmax(0, 1fr)';
    stack.style.gridTemplateRows = buildGridTemplateRows(trackStateFor(state, cfg.hostWraps));

    const priceEl = document.getElementById(cfg.priceId);
    if (priceEl) {
      priceEl.style.display = '';
      priceEl.style.gridRow = '1';
      priceEl.hidden = false;
      priceEl.classList.toggle('fullscreen-pane', state.fullscreenPaneId === 'price');
    }

    for (const [hostId, wrapId] of Object.entries(cfg.hostWraps)) {
      const wrap = document.getElementById(wrapId);
      if (!wrap) continue;
      wrap.dataset.paneHost = hostId;
      wrap.style.gridRow = '';
      wrap.style.display = 'none';
      wrap.hidden = true;
      wrap.classList.toggle('fullscreen-pane', state.fullscreenPaneId === hostId);
    }

    clearDynamicGutters(stack);

    let row = 2;
    for (const hostId of footers) {
      const wrap = document.getElementById(cfg.hostWraps[hostId]);
      if (!wrap) continue;

      const gutter = document.createElement('div');
      gutter.className = 'pane-splitter';
      gutter.dataset.layoutGutter = '1';
      gutter.dataset.boundary = `before-${hostId}`;
      gutter.dataset.hostBelow = hostId;
      gutter.setAttribute('role', 'separator');
      gutter.setAttribute('aria-orientation', 'horizontal');
      gutter.title = 'Drag to resize';
      gutter.style.gridRow = String(row++);
      wrap.parentNode.insertBefore(gutter, wrap);
      bindSplitterDrag(gutter, context, hostId);

      wrap.hidden = false;
      wrap.style.display = 'flex';
      wrap.style.gridRow = String(row++);

      const legend = wrap.querySelector('.chart-legend');
      if (legend) bindFooterReorder(context, hostId, legend);
    }

    bindFullscreenToggle(context);
  }

  function forceChartResize(context) {
    const cfg = STACKS[context];
    if (!cfg || typeof document === 'undefined') return;

    const applyHost = (pane, hostDomId) => {
      const host = document.getElementById(hostDomId);
      if (!host || host.clientWidth <= 0 || host.clientHeight <= 0) return;
      let chart = null;
      if (typeof ChartAdapter !== 'undefined' && ChartAdapter.getChart) {
        chart = ChartAdapter.getChart(context === 'live' ? 'live' : context, pane);
      }
      if (chart && typeof chart.applyOptions === 'function') {
        chart.applyOptions({ width: host.clientWidth, height: host.clientHeight });
      }
    };

    applyHost('price', cfg.chartHosts.price);
    for (const [hostId, domId] of Object.entries(cfg.chartHosts)) {
      if (hostId === 'price') continue;
      applyHost(hostId, domId);
    }
  }

  function syncLegendEyes(state) {
    if (typeof document === 'undefined') return;
    const visible = new Set(Array.isArray(state.visible) ? state.visible : []);
    document.querySelectorAll('.visibility-toggle-btn[data-target]').forEach((btn) => {
      const id = String(btn.getAttribute('data-target') || '').trim();
      if (!id) return;
      const on = visible.has(id);
      btn.classList.toggle('is-dimmed', !on);
      btn.title = on ? 'Hide pane' : 'Show pane';
    });
  }

  function syncBottomTimeAxis(state) {
    const owner = (typeof PaneLayout !== 'undefined' && PaneLayout.resolveBottomTimeAxisHostId)
      ? PaneLayout.resolveBottomTimeAxisHostId(state)
      : 'price';

    if (typeof document !== 'undefined') {
      document.querySelectorAll('.chart-wrap[data-pane-host]').forEach((wrap) => {
        const hostId = String(wrap.getAttribute('data-pane-host') || '').trim();
        if (!hostId) return;
        wrap.dataset.bottomTimeAxis = hostId === owner ? '1' : '0';
      });
    }

    // Live charts only (ChartAdapter); backtest may grow a mirror later.
    if (typeof ChartAdapter !== 'undefined' && typeof ChartAdapter.setBottomTimeAxis === 'function') {
      ChartAdapter.setBottomTimeAxis(owner);
    }
  }

  function apply() {
    if (!paneLayout || typeof paneLayout.getState !== 'function') return;
    // Mid height-drag: never rebuild gutters (would steal pointer capture).
    if (heightDrag) {
      applyTracksOnly(paneLayout.getState());
      return;
    }
    const state = paneLayout.getState();
    applyStack('live', state);
    applyStack('backtest', state);
    syncLegendEyes(state);
    syncBottomTimeAxis(state);
    document.body?.classList.toggle('is-pane-fullscreen', !!state.fullscreenPaneId);
    scheduleResize();
  }

  /**
   * Subscribe to PaneLayout and apply immediately.
   * @param {ReturnType<typeof PaneLayout.create>} layout
   */
  function attach(layout) {
    if (unsub) {
      try { unsub(); } catch { /* */ }
      unsub = null;
    }
    paneLayout = layout || null;
    if (!paneLayout || typeof paneLayout.subscribe !== 'function') return;
    unsub = paneLayout.subscribe(() => apply());
    apply();
  }

  function init() {
    if (global.paneLayout) attach(global.paneLayout);
  }

  function loadPaneHeights() {
    // Heights live in PaneLayout; kept as no-op socket for legacy callers.
  }

  const LayoutController = {
    init,
    attach,
    apply,
    loadPaneHeights,
    buildGridTemplateRows,
    maxFooterHeightFor,
    GUTTER_PX,
    PRICE_MIN_PX,
    STACKS,
  };

  global.LayoutController = LayoutController;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = LayoutController;
  }
})(typeof window !== 'undefined' ? window : globalThis);
