/**
 * RulerController — ADR-025 TradingView-style measure (geometry + FSM only).
 *
 * Anchors: { logical, price, time? } — never x/y pixels.
 * Two-click: armed → placing (A) → finished (B). pointerUp does NOT finish.
 * Third click while finished: one-shot exit — clear → idle (disarm + UI notify).
 * Cancel (Esc/right-click) → armed (tool stays on).
 *
 * ChartAdapter projects logical→x / price→y on every render.
 * RulerMetrics owns numbers; this module never formats tooltip HTML.
 */
(function (global) {
  'use strict';

  const STATE = Object.freeze({
    IDLE: 'idle',
    ARMED: 'armed',
    PLACING: 'placing',
    FINISHED: 'finished',
  });

  /** Phase 1: price pane only (overlay on price-wrap). */
  const PHASE1_HOST = 'price';

  /** @type {string} */
  let state = STATE.IDLE;
  /** @type {string|null} */
  let hostId = null;
  /** @type {{ logical: number, price: number, time?: *|null }|null} */
  let anchorA = null;
  /** @type {{ logical: number, price: number, time?: *|null }|null} */
  let anchorB = null;

  /**
   * @typedef {{
   *   render: (geo: object|null) => void,
   *   onActiveChange?: (active: boolean) => void,
   * }} RulerHooks
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
   * @param {object|null|undefined} point
   * @returns {{ logical: number, price: number, time: *|null }|null}
   */
  function normalizeAnchor(point) {
    if (!point || typeof point !== 'object') return null;
    const logical = Number(point.logical);
    const price = Number(point.price);
    if (!Number.isFinite(logical) || !Number.isFinite(price)) return null;
    const time = Object.prototype.hasOwnProperty.call(point, 'time')
      ? (point.time == null ? null : point.time)
      : null;
    return { logical, price, time };
  }

  /**
   * Semantic geometry — ChartAdapter projects to pixels.
   * @returns {{
   *   hostId: string,
   *   anchorA: object,
   *   anchorB: object,
   *   preview: boolean,
   * }|null}
   */
  function getGeometry() {
    if (!hostId || !anchorA || !anchorB) return null;
    if (state !== STATE.PLACING && state !== STATE.FINISHED) return null;
    return {
      hostId,
      anchorA: { ...anchorA },
      anchorB: { ...anchorB },
      preview: state === STATE.PLACING,
    };
  }

  function emitRender() {
    if (!hooks?.render) return;
    hooks.render(getGeometry());
  }

  function emitActiveChange(active) {
    if (typeof hooks?.onActiveChange === 'function') {
      hooks.onActiveChange(!!active);
    }
  }

  function clearAnchors() {
    hostId = null;
    anchorA = null;
    anchorB = null;
  }

  function bind(next) {
    hooks = next && typeof next === 'object' ? next : null;
    emitRender();
  }

  function unbind() {
    hooks = null;
  }

  function arm() {
    state = STATE.ARMED;
    clearAnchors();
    emitRender();
    emitActiveChange(true);
    return true;
  }

  function disarm() {
    state = STATE.IDLE;
    clearAnchors();
    emitRender();
    emitActiveChange(false);
    return true;
  }

  function toggle() {
    if (state === STATE.IDLE) return arm();
    return disarm();
  }

  /**
   * Cancel mid-measure (Esc / right-click). Stay armed.
   * @returns {boolean}
   */
  function cancel() {
    if (state === STATE.IDLE) return false;
    if (state === STATE.ARMED) {
      clearAnchors();
      emitRender();
      return true;
    }
    clearAnchors();
    state = STATE.ARMED;
    emitRender();
    return true;
  }

  /**
   * @param {string} nextHostId
   * @param {{ logical: number, price: number, time?: *|null }} point
   * @returns {boolean}
   */
  function onPointerDown(nextHostId, point) {
    if (state === STATE.IDLE) return false;
    const id = String(nextHostId || '').trim();
    if (id !== PHASE1_HOST) return false;
    const p = normalizeAnchor(point);
    if (!p) return false;

    if (state === STATE.FINISHED) {
      // One-shot: clear + full exit (idle). Toolbar notified via onActiveChange.
      disarm();
      return true;
    }

    if (state === STATE.ARMED) {
      hostId = id;
      anchorA = p;
      anchorB = p;
      state = STATE.PLACING;
      emitRender();
      return true;
    }

    if (state === STATE.PLACING) {
      if (id !== hostId) return false;
      anchorB = p;
      state = STATE.FINISHED;
      emitRender();
      return true;
    }

    return false;
  }

  /**
   * Preview while placing — does not finish.
   * @param {string} nextHostId
   * @param {{ logical: number, price: number, time?: *|null }} point
   * @returns {boolean}
   */
  function onPointerMove(nextHostId, point) {
    if (state !== STATE.PLACING) return false;
    if (String(nextHostId || '').trim() !== hostId) return false;
    const p = normalizeAnchor(point);
    if (!p) return false;
    anchorB = p;
    emitRender();
    return true;
  }

  /** pointerUp must NOT finish (TV two-click). */
  function onPointerUp(_hostId) {
    return false;
  }

  /** @deprecated use cancel(); kept for IC onPointerCancel */
  function onPointerLeave() {
    return cancel();
  }

  function _resetForTests() {
    unbind();
    state = STATE.IDLE;
    clearAnchors();
  }

  const RulerController = {
    STATE,
    PHASE1_HOST,
    bind,
    unbind,
    arm,
    disarm,
    toggle,
    cancel,
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
