/**
 * InteractionController — ADR-021 P3 / ADR-024.
 * Routes user interaction only. No LWC, no camera math, no hover policy.
 *
 * ChartAdapter adapts LWC → semantic events.
 * This module forwards to TimeCamera / CrosshairController.
 * ScaleController Y-hit stays with LayoutController until a second consumer needs it here.
 */
(function (global) {
  'use strict';

  /**
   * Authoritative hover enter (DOM pointer → ChartAdapter → here).
   * @param {string} hostId
   * @returns {boolean}
   */
  function onPointerEnter(hostId) {
    if (typeof CrosshairController === 'undefined') return false;
    return !!CrosshairController.setHovered(hostId);
  }

  /**
   * Authoritative hover leave (ChartAdapter already resolved leave-stack).
   * @returns {boolean}
   */
  function onPointerLeave() {
    if (typeof CrosshairController === 'undefined') return false;
    return !!CrosshairController.setHovered(null);
  }

  /**
   * Pane visible-logical-range proposal after native LWC time gesture.
   * @param {string} hostId
   * @param {{ from: number, to: number }|null} range
   * @param {number|null|undefined} [barSpacing]
   * @returns {boolean}
   */
  function onRangeChanged(hostId, range, barSpacing) {
    if (typeof TimeCamera === 'undefined') return false;
    return !!TimeCamera.proposeFromPane(hostId, range, barSpacing);
  }

  /**
   * Observational crosshair time from the hovered pane (LWC filters stay in ChartAdapter).
   * @param {string} hostId
   * @param {*} time business time or null (clear peers)
   * @returns {boolean}
   */
  function onCrosshairMove(hostId, time) {
    if (typeof CrosshairController === 'undefined') return false;
    return !!CrosshairController.syncTime({ sourceHostId: hostId, time });
  }

  /** Socket for future attach/detach of transient listeners — currently stateless. */
  function dispose() {
    /* no retained subscriptions */
  }

  /** @private tests */
  function _resetForTests() {
    dispose();
  }

  const InteractionController = {
    onPointerEnter,
    onPointerLeave,
    onRangeChanged,
    onCrosshairMove,
    dispose,
    _resetForTests,
  };

  global.InteractionController = InteractionController;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = InteractionController;
  }
})(typeof window !== 'undefined' ? window : globalThis);
