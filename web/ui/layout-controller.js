/**
 * LayoutController — ADR-019 Phase 2–3: CSS Grid geometry driven by PaneLayout.
 * Price track is always 1fr; footers use footerHeights (px). Dynamic gutters only
 * between visible panes. Phase 3: splitter drag mutates footerHeights only.
 * No reorder / fullscreen / setHostActive here.
 */
(function (global) {
  'use strict';

  const GUTTER_PX = 4;
  const PRICE_MIN_PX = 120;

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
  let drag = null;
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

  function bindSplitterDrag(gutter, context, hostId) {
    gutter.addEventListener('pointerdown', (e) => {
      if (e.button != null && e.button !== 0) return;
      if (!paneLayout || typeof paneLayout.setFooterHeight !== 'function') return;
      e.preventDefault();
      e.stopPropagation();

      const cfg = STACKS[context];
      const stack = document.getElementById(cfg.stackId);
      const state = paneLayout.getState();
      const startH = Number(state.footerHeights?.[hostId]);
      const fallback = (typeof PaneLayout !== 'undefined' && PaneLayout.DEFAULT_FOOTER_HEIGHT_PX) || 180;
      drag = {
        hostId,
        context,
        startY: e.clientY,
        startH: Number.isFinite(startH) && startH > 0 ? startH : fallback,
        maxH: maxFooterHeightFor(stack, state, hostId, cfg.hostWraps),
        pointerId: e.pointerId,
      };
      pendingDragHeight = drag.startH;
      try {
        gutter.setPointerCapture(e.pointerId);
      } catch {
        /* */
      }
      document.body?.classList.add('is-pane-resizing');

      const onMove = (moveEvent) => {
        if (!drag || moveEvent.pointerId !== drag.pointerId) return;
        const dy = moveEvent.clientY - drag.startY;
        // Splitter sits above this footer: drag down → taller footer; price (1fr) shrinks.
        const minH = (typeof PaneLayout !== 'undefined' && PaneLayout.FOOTER_HEIGHT_MIN_PX) || 48;
        pendingDragHeight = Math.max(minH, Math.min(drag.maxH, Math.round(drag.startH + dy)));
        if (dragApplyRaf) return;
        dragApplyRaf = requestAnimationFrame(() => {
          dragApplyRaf = 0;
          if (!drag || pendingDragHeight == null) return;
          paneLayout.setFooterHeight(drag.hostId, pendingDragHeight);
        });
      };

      const onUp = (upEvent) => {
        if (drag && upEvent.pointerId !== drag.pointerId) return;
        gutter.removeEventListener('pointermove', onMove);
        gutter.removeEventListener('pointerup', onUp);
        gutter.removeEventListener('pointercancel', onUp);
        try {
          if (drag) gutter.releasePointerCapture(drag.pointerId);
        } catch {
          /* */
        }
        if (dragApplyRaf) {
          cancelAnimationFrame(dragApplyRaf);
          dragApplyRaf = 0;
        }
        if (drag && pendingDragHeight != null && paneLayout) {
          paneLayout.setFooterHeight(drag.hostId, pendingDragHeight);
        }
        drag = null;
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
    const footers = visibleHostIds(state, cfg.hostWraps);
    stack.style.display = 'grid';
    stack.style.gridTemplateColumns = 'minmax(0, 1fr)';
    stack.style.gridTemplateRows = buildGridTemplateRows(trackStateFor(state, cfg.hostWraps));

    const priceEl = document.getElementById(cfg.priceId);
    if (priceEl) {
      priceEl.style.display = '';
      priceEl.style.gridRow = '1';
      priceEl.hidden = false;
    }

    for (const [hostId, wrapId] of Object.entries(cfg.hostWraps)) {
      const wrap = document.getElementById(wrapId);
      if (!wrap) continue;
      wrap.style.gridRow = '';
      wrap.style.display = 'none';
      wrap.hidden = true;
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
    }
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

  function apply() {
    if (!paneLayout || typeof paneLayout.getState !== 'function') return;
    // Mid-drag: never rebuild gutters (would steal pointer capture).
    if (drag) {
      applyTracksOnly(paneLayout.getState());
      return;
    }
    const state = paneLayout.getState();
    applyStack('live', state);
    applyStack('backtest', state);
    syncLegendEyes(state);
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
