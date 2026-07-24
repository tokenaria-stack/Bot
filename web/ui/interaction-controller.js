/**
 * InteractionController — ADR-021 P3 / ADR-024 (+ ADR-025 ruler routes).
 * Routes user interaction only. No LWC, no camera math, no hover policy, no ruler math.
 *
 * Invariant: accepts only semantic events. DOM/LWC/browser translation is
 * ChartAdapter's exclusive job — never onCrosshairMove(hostId, lwcParam).
 */
(function (global) {
  'use strict';

  function onPointerEnter(hostId) {
    if (typeof CrosshairController === 'undefined') return false;
    return !!CrosshairController.setHovered(hostId);
  }

  function onPointerLeave() {
    if (typeof CrosshairController === 'undefined') return false;
    return !!CrosshairController.setHovered(null);
  }

  function onRangeChanged(hostId, range, barSpacing) {
    if (typeof TimeCamera === 'undefined') return false;
    return !!TimeCamera.proposeFromPane(hostId, range, barSpacing);
  }

  function onCrosshairMove(hostId, time) {
    if (typeof CrosshairController === 'undefined') return false;
    return !!CrosshairController.syncTime({ sourceHostId: hostId, time });
  }

  /**
   * Semantic pointer down. Point: { logical, price, time? } — never MouseEvent / x,y.
   * @param {string} hostId
   * @param {{ logical: number, price: number, time?: *|null }} logicalPoint
   */
  function onPointerDown(hostId, logicalPoint) {
    if (typeof RulerController === 'undefined') return false;
    return !!RulerController.onPointerDown(hostId, logicalPoint);
  }

  function onPointerMove(hostId, logicalPoint) {
    if (typeof RulerController === 'undefined') return false;
    return !!RulerController.onPointerMove(hostId, logicalPoint);
  }

  /** pointerUp does not finish the ruler (two-click). Kept as route socket. */
  function onPointerUp(hostId) {
    if (typeof RulerController === 'undefined') return false;
    return !!RulerController.onPointerUp(hostId);
  }

  function onPointerCancel(_hostId) {
    if (typeof RulerController === 'undefined') return false;
    return !!RulerController.cancel();
  }

  /** Esc / right-click cancel — semantic only. */
  function onCancel() {
    if (typeof RulerController === 'undefined') return false;
    return !!RulerController.cancel();
  }

  function dispose() {
    /* no retained subscriptions */
  }

  function _resetForTests() {
    dispose();
  }

  const InteractionController = {
    onPointerEnter,
    onPointerLeave,
    onPointerDown,
    onPointerMove,
    onPointerUp,
    onPointerCancel,
    onCancel,
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
