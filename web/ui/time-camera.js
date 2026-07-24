/**
 * TimeCamera — ADR-021 sole owner of the canonical live timeline.
 *
 * Owns: visibleLogicalRange, barSpacing, optional rightOffset, echo lock, gesture mute.
 * Does not talk to LWC. ChartAdapter binds applyCommitted and forwards pane proposals.
 *
 * Atomic commit only — no setRange / setBarSpacing setters.
 */
(function (global) {
  'use strict';

  /** @type {{ visibleRange: { from: number, to: number }|null, barSpacing: number|null, rightOffset: number|null }} */
  let canonical = {
    visibleRange: null,
    barSpacing: null,
    rightOffset: null,
  };

  let isSyncing = false;
  let cameraGesturing = false;
  let gestureTimer = null;

  /** @type {null|((state: object) => void)} */
  let applyCommitted = null;
  /** @type {null|(() => boolean)} skip propose/commit when true (e.g. live paint) */
  let shouldSkip = null;

  const GESTURE_MUTE_MS = 100;

  function isFiniteLogicalRange(range) {
    return !!(range
      && Number.isFinite(range.from)
      && Number.isFinite(range.to)
      && range.to > range.from);
  }

  function cloneRange(range) {
    if (!isFiniteLogicalRange(range)) return null;
    return { from: range.from, to: range.to };
  }

  function snapshot() {
    return {
      visibleRange: canonical.visibleRange
        ? { from: canonical.visibleRange.from, to: canonical.visibleRange.to }
        : null,
      barSpacing: canonical.barSpacing,
      rightOffset: canonical.rightOffset,
    };
  }

  function markGesture() {
    cameraGesturing = true;
    if (gestureTimer) clearTimeout(gestureTimer);
    gestureTimer = setTimeout(() => {
      cameraGesturing = false;
      gestureTimer = null;
    }, GESTURE_MUTE_MS);
  }

  /**
   * Bind ChartAdapter apply hook (LWC writes happen only there).
   * @param {{ applyCommitted: (state: object) => void, shouldSkip?: () => boolean }} hooks
   */
  function bind(hooks) {
    applyCommitted = typeof hooks?.applyCommitted === 'function' ? hooks.applyCommitted : null;
    shouldSkip = typeof hooks?.shouldSkip === 'function' ? hooks.shouldSkip : null;
  }

  function unbind() {
    applyCommitted = null;
    shouldSkip = null;
    isSyncing = false;
    cameraGesturing = false;
    if (gestureTimer) {
      clearTimeout(gestureTimer);
      gestureTimer = null;
    }
  }

  /**
   * Atomic commit. Null / omitted fields mean "leave canonical unchanged".
   * @param {{ visibleRange?: object|null, barSpacing?: number|null, rightOffset?: number|null, sourceHostId?: string }} patch
   * @returns {boolean} true if applied
   */
  function commit(patch) {
    if (isSyncing) return false;
    if (!applyCommitted) return false;
    if (!patch || typeof patch !== 'object') return false;

    const next = snapshot();
    let dirty = false;

    if (Object.prototype.hasOwnProperty.call(patch, 'visibleRange')) {
      const r = cloneRange(patch.visibleRange);
      if (r) {
        if (!next.visibleRange
          || next.visibleRange.from !== r.from
          || next.visibleRange.to !== r.to) {
          next.visibleRange = r;
          dirty = true;
        }
      }
    }
    if (Object.prototype.hasOwnProperty.call(patch, 'barSpacing')) {
      const s = patch.barSpacing;
      if (Number.isFinite(s) && s > 0 && next.barSpacing !== s) {
        next.barSpacing = s;
        dirty = true;
      }
    }
    if (Object.prototype.hasOwnProperty.call(patch, 'rightOffset')) {
      const o = patch.rightOffset;
      if (Number.isFinite(o) && next.rightOffset !== o) {
        next.rightOffset = o;
        dirty = true;
      }
    }

    if (!dirty) return false;

    canonical = next;
    const sourceHostId = patch.sourceHostId != null ? String(patch.sourceHostId) : 'system';
    if (sourceHostId !== 'system') markGesture();

    isSyncing = true;
    try {
      applyCommitted({
        ...snapshot(),
        sourceHostId,
      });
    } finally {
      isSyncing = false;
    }
    return true;
  }

  /**
   * Pane proposal after native LWC gesture. Ignored while syncing / skip.
   * @param {string} hostId
   * @param {{ from: number, to: number }|null} visibleRange
   * @param {number|null|undefined} barSpacing
   */
  function proposeFromPane(hostId, visibleRange, barSpacing) {
    if (isSyncing) return false;
    if (shouldSkip && shouldSkip()) return false;
    if (!isFiniteLogicalRange(visibleRange)) return false;
    const patch = {
      visibleRange,
      sourceHostId: hostId || 'unknown',
    };
    if (Number.isFinite(barSpacing) && barSpacing > 0) {
      patch.barSpacing = barSpacing;
    }
    return commit(patch);
  }

  function isSyncingNow() {
    return isSyncing;
  }

  function isGesturing() {
    return cameraGesturing;
  }

  function getCanonical() {
    return snapshot();
  }

  /** @private tests */
  function _resetForTests() {
    unbind();
    canonical = { visibleRange: null, barSpacing: null, rightOffset: null };
  }

  const TimeCamera = {
    bind,
    unbind,
    commit,
    proposeFromPane,
    isSyncing: isSyncingNow,
    isGesturing,
    getCanonical,
    isFiniteLogicalRange,
    _resetForTests,
  };

  global.TimeCamera = TimeCamera;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = TimeCamera;
  }
})(typeof window !== 'undefined' ? window : globalThis);
