/**
 * PaneLayout — ADR-019 FE SSOT for footer pane membership / order / heights / fullscreen.
 * Phase 1: state + persist ∩ manifest + Ind menu.
 * Phase 2–5: LayoutController reads this state for CSS Grid, heights, reorder, fullscreen.
 *
 * Price is always present and is not a HostID. footerHeights are pixels only (no price key).
 */
(function (global) {
  'use strict';

  const VERSION = 1;
  const LS_KEY = 'dashboard_pane_layout';
  const DEFAULT_FOOTER_HEIGHT_PX = 180;
  const FOOTER_HEIGHT_MIN_PX = 48;
  const FOOTER_HEIGHT_MAX_PX = 800;

  function parseRenderOpts(raw) {
    if (!raw) return {};
    if (typeof raw === 'object') return raw;
    try {
      const parsed = JSON.parse(raw);
      return parsed && typeof parsed === 'object' ? parsed : {};
    } catch {
      return {};
    }
  }

  /** Short ids (RSX, ATR) → UPPER; longer hostIds → Title case. No hardcoded indicator names. */
  function defaultTitleForHost(hostId) {
    const id = String(hostId || '').trim();
    if (!id) return '';
    if (id.length <= 4) return id.toUpperCase();
    return id.charAt(0).toUpperCase() + id.slice(1);
  }

  /**
   * Unique footer HostIDs from UIManifest (price excluded — never a HostID).
   * @returns {{ hostId: string, title: string }[]}
   */
  function collectCatalog(manifest) {
    const panes = manifest?.panes;
    if (!panes || typeof panes !== 'object') return [];
    const seen = new Map();
    for (const comps of Object.values(panes)) {
      if (!Array.isArray(comps)) continue;
      for (const c of comps) {
        if (!c) continue;
        const hostId = String(c.hostId || c.hostID || '').trim();
        if (!hostId || hostId === 'price') continue;
        if (seen.has(hostId)) continue;
        const opts = parseRenderOpts(c.renderOptions ?? c.RenderOpts);
        const paneTitle = opts.paneTitle != null ? String(opts.paneTitle).trim() : '';
        seen.set(hostId, {
          hostId,
          title: paneTitle || defaultTitleForHost(hostId),
        });
      }
    }
    return Array.from(seen.values());
  }

  function clampHeightPx(n, fallback) {
    const v = Number(n);
    if (!Number.isFinite(v) || v <= 0) {
      const f = Number(fallback);
      const base = Number.isFinite(f) && f > 0 ? f : DEFAULT_FOOTER_HEIGHT_PX;
      return Math.max(FOOTER_HEIGHT_MIN_PX, Math.min(FOOTER_HEIGHT_MAX_PX, Math.round(base)));
    }
    return Math.max(FOOTER_HEIGHT_MIN_PX, Math.min(FOOTER_HEIGHT_MAX_PX, Math.round(v)));
  }

  function defaultState(catalog, defaultFooterHeightPx) {
    const height = clampHeightPx(defaultFooterHeightPx, DEFAULT_FOOTER_HEIGHT_PX);
    const order = catalog.map((c) => c.hostId);
    const footerHeights = {};
    for (const id of order) footerHeights[id] = height;
    return {
      version: VERSION,
      visible: order.slice(),
      order,
      footerHeights,
      fullscreenPaneId: null,
    };
  }

  function allowedSet(catalog) {
    return new Set(catalog.map((c) => c.hostId));
  }

  function isValidFullscreen(id, allowed) {
    if (id == null || id === '') return true;
    if (id === 'price') return true;
    return allowed.has(id);
  }

  /**
   * Intersect saved layout with current catalog. Version mismatch → defaults.
   * Hidden hosts keep footerHeights; unknown hosts dropped; new hosts appended.
   */
  function restoreFromSaved(saved, catalog, defaultFooterHeightPx = DEFAULT_FOOTER_HEIGHT_PX) {
    const defaults = defaultState(catalog, defaultFooterHeightPx);
    if (!catalog.length) {
      return {
        version: VERSION,
        visible: [],
        order: [],
        footerHeights: {},
        fullscreenPaneId: null,
      };
    }
    if (!saved || typeof saved !== 'object' || saved.version !== VERSION) {
      return defaults;
    }

    const allowed = allowedSet(catalog);
    const heightFallback = clampHeightPx(defaultFooterHeightPx, DEFAULT_FOOTER_HEIGHT_PX);
    const savedHeights = saved.footerHeights && typeof saved.footerHeights === 'object'
      ? saved.footerHeights
      : {};

    const order = [];
    const seen = new Set();
    if (Array.isArray(saved.order)) {
      for (const raw of saved.order) {
        const id = String(raw || '').trim();
        if (!id || !allowed.has(id) || seen.has(id)) continue;
        order.push(id);
        seen.add(id);
      }
    }
    for (const c of catalog) {
      if (seen.has(c.hostId)) continue;
      order.push(c.hostId);
      seen.add(c.hostId);
    }

    const visible = [];
    const visSeen = new Set();
    if (Array.isArray(saved.visible)) {
      for (const raw of saved.visible) {
        const id = String(raw || '').trim();
        if (!id || !allowed.has(id) || visSeen.has(id)) continue;
        visible.push(id);
        visSeen.add(id);
      }
    } else {
      // Missing visible → all on (same as defaults).
      for (const id of order) visible.push(id);
    }

    const footerHeights = {};
    for (const id of order) {
      footerHeights[id] = clampHeightPx(savedHeights[id], heightFallback);
    }

    let fullscreenPaneId = saved.fullscreenPaneId != null
      ? String(saved.fullscreenPaneId)
      : null;
    if (fullscreenPaneId === '') fullscreenPaneId = null;
    if (!isValidFullscreen(fullscreenPaneId, allowed)) fullscreenPaneId = null;

    return {
      version: VERSION,
      visible,
      order,
      footerHeights,
      fullscreenPaneId,
    };
  }

  function cloneState(state) {
    return {
      version: state.version,
      visible: state.visible.slice(),
      order: state.order.slice(),
      footerHeights: { ...state.footerHeights },
      fullscreenPaneId: state.fullscreenPaneId,
    };
  }

  function memoryStorage() {
    const map = new Map();
    return {
      getItem(key) {
        return map.has(key) ? map.get(key) : null;
      },
      setItem(key, value) {
        map.set(key, String(value));
      },
    };
  }

  function resolveStorage(explicit) {
    if (explicit) return explicit;
    try {
      if (typeof global.localStorage !== 'undefined' && global.localStorage) {
        return global.localStorage;
      }
    } catch {
      /* private mode / node */
    }
    return memoryStorage();
  }

  /**
   * @param {object} [options]
   * @param {Storage|{getItem:Function,setItem:Function}} [options.storage]
   * @param {string} [options.lsKey]
   * @param {number} [options.defaultFooterHeightPx]
   */
  function create(options = {}) {
    const lsKey = options.lsKey || LS_KEY;
    const defaultFooterHeightPx = clampHeightPx(
      options.defaultFooterHeightPx,
      DEFAULT_FOOTER_HEIGHT_PX,
    );
    const storage = resolveStorage(options.storage);

    /** @type {{ hostId: string, title: string }[]} */
    let catalog = [];
    let state = defaultState([], defaultFooterHeightPx);
    const listeners = new Set();
    let indMenuBound = false;
    let unsubIndMenu = null;

    function emit() {
      const snap = cloneState(state);
      for (const fn of listeners) {
        try {
          fn(snap);
        } catch (err) {
          try {
            console.error('[PaneLayout] subscriber failed:', err);
          } catch {
            /* */
          }
        }
      }
    }

    function persist() {
      try {
        storage.setItem(lsKey, JSON.stringify(cloneState(state)));
      } catch (err) {
        try {
          console.warn('[PaneLayout] persist failed:', err);
        } catch {
          /* */
        }
      }
    }

    function loadSaved() {
      try {
        const raw = storage.getItem(lsKey);
        if (!raw) return null;
        const parsed = JSON.parse(raw);
        return parsed && typeof parsed === 'object' ? parsed : null;
      } catch {
        return null;
      }
    }

    function commit(next, { silent } = {}) {
      state = next;
      persist();
      if (!silent) emit();
    }

    /**
     * @param {{ manifest: object }} args
     */
    function init({ manifest } = {}) {
      catalog = collectCatalog(manifest);
      state = restoreFromSaved(loadSaved(), catalog, defaultFooterHeightPx);
      persist();
      emit();
      return getState();
    }

    function getCatalog() {
      return catalog.map((c) => ({ ...c }));
    }

    function getState() {
      return cloneState(state);
    }

    function isVisible(hostId) {
      const id = String(hostId || '').trim();
      return state.visible.includes(id);
    }

    function setVisible(hostId, on) {
      const id = String(hostId || '').trim();
      if (!id || !state.order.includes(id)) return false;
      const want = !!on;
      const currently = state.visible.includes(id);
      if (want === currently) return false;

      const visible = want
        ? state.visible.concat(id)
        : state.visible.filter((h) => h !== id);

      let fullscreenPaneId = state.fullscreenPaneId;
      if (!want && fullscreenPaneId === id) fullscreenPaneId = null;

      commit({
        ...cloneState(state),
        visible,
        fullscreenPaneId,
      });
      return true;
    }

    function toggle(hostId) {
      return setVisible(hostId, !isVisible(hostId));
    }

    /**
     * Set one footer height in pixels. Price is never stored (always 1fr).
     * @returns {boolean} true if state changed
     */
    function setFooterHeight(hostId, px) {
      const id = String(hostId || '').trim();
      if (!id || !state.order.includes(id)) return false;
      const prev = state.footerHeights[id];
      const next = clampHeightPx(px, prev != null ? prev : defaultFooterHeightPx);
      if (prev === next) return false;
      commit({
        ...cloneState(state),
        footerHeights: { ...state.footerHeights, [id]: next },
      });
      return true;
    }

    /**
     * Replace footer order. Must be a permutation of known hosts (extras dropped,
     * missing appended). Does not touch visible / heights / fullscreenPaneId.
     * @param {string[]} nextOrder
     * @returns {boolean}
     */
    function setOrder(nextOrder) {
      if (!Array.isArray(nextOrder)) return false;
      const allowed = new Set(state.order);
      if (!allowed.size) return false;
      const cleaned = [];
      const seen = new Set();
      for (const raw of nextOrder) {
        const id = String(raw || '').trim();
        if (!id || !allowed.has(id) || seen.has(id)) continue;
        cleaned.push(id);
        seen.add(id);
      }
      for (const id of state.order) {
        if (!seen.has(id)) cleaned.push(id);
      }
      if (cleaned.length !== state.order.length) return false;
      let same = true;
      for (let i = 0; i < cleaned.length; i++) {
        if (cleaned[i] !== state.order[i]) {
          same = false;
          break;
        }
      }
      if (same) return false;
      commit({
        ...cloneState(state),
        order: cleaned,
      });
      return true;
    }

    /**
     * Move host among visible footers; hidden hosts keep relative slots.
     * @param {string} hostId
     * @param {string|null} beforeHostId insert before this visible host; null = end of visible
     * @returns {boolean}
     */
    function moveHostBefore(hostId, beforeHostId) {
      const id = String(hostId || '').trim();
      if (!id || !state.order.includes(id)) return false;
      if (!state.visible.includes(id)) return false;

      const vis = state.order.filter((h) => state.visible.includes(h));
      const from = vis.indexOf(id);
      if (from < 0) return false;
      vis.splice(from, 1);

      let to = vis.length;
      if (beforeHostId != null && beforeHostId !== '') {
        const before = String(beforeHostId).trim();
        if (before === id) return false;
        const bi = vis.indexOf(before);
        to = bi >= 0 ? bi : vis.length;
      }
      vis.splice(to, 0, id);

      let vi = 0;
      const next = state.order.map((h) => (
        state.visible.includes(h) ? vis[vi++] : h
      ));
      return setOrder(next);
    }

    function setFullscreen(hostIdOrNull) {
      const allowed = allowedSet(catalog);
      let next = hostIdOrNull == null || hostIdOrNull === ''
        ? null
        : String(hostIdOrNull);
      if (!isValidFullscreen(next, allowed)) return false;
      if (state.fullscreenPaneId === next) return false;
      commit({
        ...cloneState(state),
        fullscreenPaneId: next,
      });
      return true;
    }

    function getFullscreen() {
      return state.fullscreenPaneId;
    }

    /** Toggle fullscreen for a pane id (`price` or footer HostID). */
    function toggleFullscreen(hostId) {
      const id = hostId == null || hostId === '' ? null : String(hostId);
      if (id == null) return setFullscreen(null);
      if (state.fullscreenPaneId === id) return setFullscreen(null);
      return setFullscreen(id);
    }

    function subscribe(listener) {
      if (typeof listener !== 'function') return () => {};
      listeners.add(listener);
      return () => listeners.delete(listener);
    }

    function syncIndCheckboxes(menuEl) {
      if (!menuEl) return;
      menuEl.querySelectorAll('input[data-pane-host]').forEach((input) => {
        const id = input.getAttribute('data-pane-host');
        input.checked = isVisible(id);
      });
    }

    /**
     * Fill #ind-menu (or given el) with checkboxes from catalog. Idempotent rebuild.
     * @param {{ button?: HTMLElement|null, menu?: HTMLElement|null }} [els]
     */
    function mountIndMenu(els = {}) {
      if (typeof document === 'undefined') return;

      const button = els.button
        || document.getElementById('btn-ind-menu');
      const menu = els.menu
        || document.getElementById('ind-menu');
      if (!menu) return;

      menu.replaceChildren();
      if (!catalog.length) {
        const empty = document.createElement('div');
        empty.className = 'setting-row ind-menu-empty';
        empty.textContent = 'No indicators';
        menu.appendChild(empty);
      } else {
        for (const entry of catalog) {
          const row = document.createElement('div');
          row.className = 'setting-row';
          const label = document.createElement('label');
          label.className = 'ind-menu-label';
          const input = document.createElement('input');
          input.type = 'checkbox';
          input.setAttribute('data-pane-host', entry.hostId);
          input.checked = isVisible(entry.hostId);
          input.addEventListener('change', () => {
            setVisible(entry.hostId, input.checked);
          });
          label.appendChild(input);
          label.appendChild(document.createTextNode(` ${entry.title}`));
          row.appendChild(label);
          menu.appendChild(row);
        }
      }

      if (button && !indMenuBound) {
        indMenuBound = true;
        button.addEventListener('click', (ev) => {
          ev.preventDefault();
          ev.stopPropagation();
          if (typeof FloatingMenu !== 'undefined' && FloatingMenu.toggle) {
            FloatingMenu.toggle(menu, button);
          } else {
            menu.hidden = !menu.hidden;
          }
        });
      }

      if (typeof FloatingMenu !== 'undefined' && FloatingMenu.initDrag) {
        menu._dragBound = false;
        FloatingMenu.initDrag(menu);
      }

      if (unsubIndMenu) unsubIndMenu();
      unsubIndMenu = subscribe(() => syncIndCheckboxes(menu));
    }

    return {
      init,
      catalog: getCatalog,
      isVisible,
      setVisible,
      toggle,
      setFooterHeight,
      setOrder,
      moveHostBefore,
      subscribe,
      setFullscreen,
      toggleFullscreen,
      getFullscreen,
      getState,
      mountIndMenu,
      /** @private tests */
      _loadSaved: loadSaved,
    };
  }

  const PaneLayout = {
    create,
    collectCatalog,
    restoreFromSaved,
    defaultState,
    clampHeightPx,
    VERSION,
    LS_KEY,
    DEFAULT_FOOTER_HEIGHT_PX,
    FOOTER_HEIGHT_MIN_PX,
    FOOTER_HEIGHT_MAX_PX,
  };

  global.PaneLayout = PaneLayout;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = PaneLayout;
  }
})(typeof window !== 'undefined' ? window : globalThis);
