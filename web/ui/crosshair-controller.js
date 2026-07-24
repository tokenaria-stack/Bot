/**
 * CrosshairController — ADR-021 Phase 2.
 *
 * Owns: hoveredHostId + V/H visibility / time-sync policy.
 * Does NOT own: timeline, barSpacing, scale, mouse routing.
 * Never propagates foreign Y between panes.
 *
 * ChartAdapter binds apply hooks (sole LWC talker).
 */
(function (global) {
  'use strict';

  const PANE_IDS = Object.freeze(['price', 'wozduh', 'rsx']);

  /** @type {string|null} */
  let hoveredHostId = null;
  let applying = false;

  /**
   * @typedef {{
   *   applyHorzVisibility: (map: Record<string, boolean>) => void,
   *   syncPeerTime: (sourceHostId: string, time: *, param: object) => void,
   *   clearPeerCrosshairs: (sourceHostId: string|null) => void,
   *   shouldIgnore?: () => boolean,
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
    applying = false;
  }

  function getHovered() {
    return hoveredHostId;
  }

  /**
   * @param {string|null} hostId
   * @returns {boolean} true if hover changed
   */
  function setHovered(hostId) {
    const next = normalizeHostId(hostId);
    if (hoveredHostId === next) return false;
    hoveredHostId = next;
    applyHoverPolicy();
    return true;
  }

  /**
   * ChartAdapter forwards LWC subscribeCrosshairMove here.
   * @param {string} hostId
   * @param {object} param
   */
  function onCrosshairMove(hostId, param) {
    if (applying) return;
    if (hooks?.shouldIgnore && hooks.shouldIgnore()) return;

    const id = normalizeHostId(hostId);
    if (!id) return;

    // No physical point → left this pane (or camera echo).
    if (!param || !param.point) {
      if (hoveredHostId === id) {
        setHovered(null);
        hooks?.clearPeerCrosshairs?.(null);
      }
      return;
    }

    setHovered(id);

    if (param.time == null) {
      hooks?.clearPeerCrosshairs?.(id);
      return;
    }

    applying = true;
    try {
      // Peers: time sync only, each peer uses its own local Y inside ChartAdapter.
      hooks?.syncPeerTime?.(id, param.time, param);
    } finally {
      applying = false;
    }
  }

  function isApplying() {
    return applying;
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
    onCrosshairMove,
    horzVisibilityMap,
    isApplying,
    _resetForTests,
  };

  global.CrosshairController = CrosshairController;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = CrosshairController;
  }
})(typeof window !== 'undefined' ? window : globalThis);
