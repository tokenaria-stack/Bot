/**
 * CrosshairController — ADR-021 hover ownership + V/H policy.
 *
 * Invariant: LWC events are observational, never authoritative.
 * Browser pointer events (on PaneLayout wrappers) are authoritative for hoveredHostId.
 *
 * Owns: hoveredHostId, hover/horz visibility policy, peer time-sync requests.
 * Does NOT know: chart, series, LWC params, timeline, barSpacing.
 *
 * Public API is semantic only: setHovered / syncTime.
 */
(function (global) {
  'use strict';

  const PANE_IDS = Object.freeze(['price', 'wozduh', 'rsx']);

  /** @type {string|null} */
  let hoveredHostId = null;
  let syncingPeers = false;

  /**
   * @typedef {{
   *   applyHorzVisibility: (map: Record<string, boolean>) => void,
   *   syncPeerTime: (sourceHostId: string, time: *) => void,
   *   clearPeerCrosshairs: (sourceHostId: string|null) => void,
   *   shouldIgnoreTimeSync?: () => boolean,
   * }} CrosshairHooks
   * @type {CrosshairHooks|null}
   */
  let hooks = null;

  function normalizeHostId(hostId) {
    if (hostId == null || hostId === '') return null;
    const id = String(hostId);
    return PANE_IDS.includes(id) ? id : null;
  }

  /**
   * Pure policy: which panes show horizontal crosshair.
   * @param {string|null} hovered
   * @returns {Record<string, boolean>}
   */
  function horzVisibilityMap(hovered) {
    const h = normalizeHostId(hovered);
    const map = {};
    PANE_IDS.forEach((id) => {
      map[id] = h != null && id === h;
    });
    return map;
  }

  function applyHoverPolicy() {
    if (!hooks?.applyHorzVisibility) return;
    hooks.applyHorzVisibility(horzVisibilityMap(hoveredHostId));
  }

  /**
   * @param {CrosshairHooks} next
   */
  function bind(next) {
    hooks = next && typeof next === 'object' ? next : null;
    applyHoverPolicy();
  }

  function unbind() {
    hooks = null;
    hoveredHostId = null;
    syncingPeers = false;
  }

  function getHovered() {
    return hoveredHostId;
  }

  /**
   * ONLY authoritative path for hover ownership (DOM pointer → ChartAdapter → here).
   * @param {string|null} hostId
   * @returns {boolean} true if hover changed
   */
  function setHovered(hostId) {
    const next = normalizeHostId(hostId);
    if (hoveredHostId === next) return false;
    hoveredHostId = next;
    applyHoverPolicy();
    if (next == null) {
      hooks?.clearPeerCrosshairs?.(null);
    }
    return true;
  }

  /**
   * Time-only sync request. Never changes hoveredHostId.
   * @param {{ sourceHostId: string, time: * }} payload
   * @returns {boolean} true if peer sync was requested
   */
  function syncTime(payload) {
    if (hooks?.shouldIgnoreTimeSync && hooks.shouldIgnoreTimeSync()) return false;
    if (!payload || typeof payload !== 'object') return false;

    const sourceHostId = normalizeHostId(payload.sourceHostId);
    if (!sourceHostId) return false;
    // Only the hovered pane may drive peer time sync.
    if (hoveredHostId == null || sourceHostId !== hoveredHostId) return false;
    if (payload.time == null) {
      hooks?.clearPeerCrosshairs?.(sourceHostId);
      return false;
    }

    syncingPeers = true;
    try {
      hooks?.syncPeerTime?.(sourceHostId, payload.time);
    } finally {
      syncingPeers = false;
    }
    // Re-assert horz policy after peer setCrosshairPosition (LWC may paint H).
    applyHoverPolicy();
    return true;
  }

  function isSyncingPeers() {
    return syncingPeers;
  }

  /** @private tests */
  function _resetForTests() {
    unbind();
  }

  const CrosshairController = {
    PANE_IDS,
    bind,
    unbind,
    getHovered,
    setHovered,
    syncTime,
    horzVisibilityMap,
    isSyncingPeers,
    _resetForTests,
  };

  global.CrosshairController = CrosshairController;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = CrosshairController;
  }
})(typeof window !== 'undefined' ? window : globalThis);
