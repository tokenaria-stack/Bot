/**
 * RulerController — ADR-025 Phase 1 foundation.
 *
 * Owns: ruler lifecycle + measurement geometry state.
 * Does NOT own: DOM, LWC, rendering, formatters, statistics.
 *
 * ChartAdapter translates pointers and renders geometry via bind({ render }).
 * InteractionController only routes semantic events here.
 *
 * Future sockets (declare only — do not implement):
 *   MeasurementFormatter / PriceFormatter / TimeFormatter /
 *   StatisticsProvider / DrawingManager
 */
(function (global) {
  'use strict';

  const STATE = Object.freeze({
    IDLE: 'idle',
    ARMED: 'armed',
    DRAGGING: 'dragging',
    FINISHED: 'finished',
  });

  /** Phase 1: price pane only (overlay lives on price-wrap). */
  const PHASE1_HOST = 'price';

  /** @type {string} */
  let state = STATE.IDLE;
  /** @type {string|null} */
  let hostId = null;
  /** @type {{ time: *, price: number }|null} */
  let start = null;
  /** @type {{ time: *, price: number }|null} */
  let end = null;

  /**
   * @typedef {{ render: (geo: object|null) => void }} RulerHooks
   * @type {RulerHooks|null}
   */
  let hooks = null;

  function isActive() {
    return state !== STATE.IDLE;
  }

  function getState() {
    return state;
  }

  /**
   * Semantic geometry for ChartAdapter.renderRuler — never pixel/LWC objects.
   * @returns {{ hostId: string, startTime: *, endTime: *, startPrice: number, endPrice: number }|null}
   */
  function getGeometry() {
    if (!hostId || !start || !end) return null;
    if (state !== STATE.DRAGGING && state !== STATE.FINISHED) return null;
    return {
      hostId,
      startTime: start.time,
      endTime: end.time,
      startPrice: start.price,
      endPrice: end.price,
    };
  }

  function emitRender() {
    if (!hooks?.render) return;
    hooks.render(getGeometry());
  }

  function clearPoints() {
    hostId = null;
    start = null;
    end = null;
  }

  /**
   * @param {RulerHooks} next
   */
  function bind(next) {
    hooks = next && typeof next === 'object' ? next : null;
    emitRender();
  }

  function unbind() {
    hooks = null;
  }

  /** Toolbar: enter Armed (or disarm if already active). */
  function arm() {
    state = STATE.ARMED;
    clearPoints();
    emitRender();
    return true;
  }

  /** Exit to Idle; clear geometry. */
  function disarm() {
    state = STATE.IDLE;
    clearPoints();
    emitRender();
    return true;
  }

  function toggle() {
    if (state === STATE.IDLE) return arm();
    return disarm();
  }

  function normalizePoint(point) {
    if (!point || typeof point !== 'object') return null;
    const price = Number(point.price);
    if (!Number.isFinite(price)) return null;
    if (point.time == null) return null;
    return { time: point.time, price };
  }

  /**
   * @param {string} nextHostId
   * @param {{ time: *, price: number }} point
   * @returns {boolean} true if consumed
   */
  function onPointerDown(nextHostId, point) {
    if (state === STATE.IDLE) return false;
    const id = String(nextHostId || '').trim();
    if (id !== PHASE1_HOST) return false;
    const p = normalizePoint(point);
    if (!p) return false;

    // Finished + new down → start a fresh measure (stay armed).
    if (state === STATE.FINISHED) {
      clearPoints();
      state = STATE.ARMED;
    }
    if (state !== STATE.ARMED) return false;

    hostId = id;
    start = p;
    end = p;
    state = STATE.DRAGGING;
    emitRender();
    return true;
  }

  /**
   * @param {string} nextHostId
   * @param {{ time: *, price: number }} point
   * @returns {boolean}
   */
  function onPointerMove(nextHostId, point) {
    if (state !== STATE.DRAGGING) return false;
    if (String(nextHostId || '').trim() !== hostId) return false;
    const p = normalizePoint(point);
    if (!p) return false;
    end = p;
    emitRender();
    return true;
  }

  /**
   * @param {string} [_hostId]
   * @returns {boolean}
   */
  function onPointerUp(_hostId) {
    if (state !== STATE.DRAGGING) return false;
    state = STATE.FINISHED;
    emitRender();
    return true;
  }

  /**
   * Pane leave while dragging: cancel gesture, stay Armed (ready for next drag).
   * @returns {boolean}
   */
  function onPointerLeave() {
    if (state !== STATE.DRAGGING) return false;
    clearPoints();
    state = STATE.ARMED;
    emitRender();
    return true;
  }

  /** @private tests */
  function _resetForTests() {
    unbind();
    state = STATE.IDLE;
    clearPoints();
  }

  const RulerController = {
    STATE,
    PHASE1_HOST,
    bind,
    unbind,
    arm,
    disarm,
    toggle,
    isActive,
    getState,
    getGeometry,
    onPointerDown,
    onPointerMove,
    onPointerUp,
    onPointerLeave,
    _resetForTests,
  };

  global.RulerController = RulerController;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = RulerController;
  }
})(typeof window !== 'undefined' ? window : globalThis);
