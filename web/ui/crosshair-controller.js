/**
 * CrosshairController — ADR-021 hover ownership + V/H policy (+ ADR-026 empty-space sync).
 *
 * Invariant: LWC events are observational, never authoritative.
 * Browser pointer events (on PaneLayout wrappers) are authoritative for hoveredHostId.
 *
 * Owns: hoveredHostId, hover/horz visibility policy, peer sync requests.
 * Does NOT know: chart, series, LWC params, timeline, barSpacing, pixels, Y.
 *
 * Public API is semantic only: setHovered / syncPosition.
 * Sync payload: { sourceHostId, logical, time? } — logical primary; time optional.
 * Never fabricates timestamps (ADR-026).
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
   *   syncPeerCrosshair: (sourceHostId: string, pos: { logical: number, time?: *|null }) => void,
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
   * Peer sync request. Never changes hoveredHostId. Never invents time.
   * Clears peers only when logical is missing (leave / invalid) — not when time is null.
   * @param {{ sourceHostId: string, logical: number, time?: *|null }} payload
   * @returns {boolean} true if peer sync was requested
   */
  function syncPosition(payload) {
    if (hooks?.shouldIgnoreTimeSync && hooks.shouldIgnoreTimeSync()) return false;
    if (!payload || typeof payload !== 'object') return false;

    const sourceHostId = normalizeHostId(payload.sourceHostId);
    if (!sourceHostId) return false;
    // Only the hovered pane may drive peer sync.
    if (hoveredHostId == null || sourceHostId !== hoveredHostId) return false;

    const logical = Number(payload.logical);
    if (!Number.isFinite(logical)) {
      hooks?.clearPeerCrosshairs?.(sourceHostId);
      return false;
    }

    const time = Object.prototype.hasOwnProperty.call(payload, 'time')
      ? (payload.time == null ? null : payload.time)
      : null;

    syncingPeers = true;
    try {
      hooks?.syncPeerCrosshair?.(sourceHostId, { logical, time });
    } finally {
      syncingPeers = false;
    }
    // Re-assert horz policy after peer setCrosshairPosition (LWC may paint H).
    applyHoverPolicy();
    return true;
  }

  /** @deprecated ADR-026 — use syncPosition({ logical, time? }) */
  function syncTime(payload) {
    if (!payload || typeof payload !== 'object') return false;
    // Legacy callers that only passed time cannot sync empty space; require logical.
    if (!Object.prototype.hasOwnProperty.call(payload, 'logical')) {
      return false;
    }
    return syncPosition(payload);
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
    syncPosition,
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
