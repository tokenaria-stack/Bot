/**
 * TimeCamera — ADR-021 / ADR-028 sole owner of live timeline + navigation VIEW.
 *
 * Owns: canonical visibleLogicalRange + barSpacing + rightOffset (ADR-021),
 *       shadow ViewIntent + ViewGeometry (ADR-028 D1 — compute only, no LWC).
 * Does not talk to LWC. ChartAdapter binds applyCommitted and forwards pane proposals.
 *
 * Atomic commit only — no setRange / setBarSpacing setters.
 * D1: ViewportManager remains paint authority. Shadow capture never writes camera.
 */
(function (global) {
  'use strict';

  /** ADR-029 hysteresis — tip within this many bars of `to` still counts as LIVE. */
  const SLACK = 1.5;
  const GESTURE_MUTE_MS = 100;

  /** @type {{ visibleRange: { from: number, to: number }|null, barSpacing: number|null, rightOffset: number|null }} */
  let canonical = {
    visibleRange: null,
    barSpacing: null,
    rightOffset: null,
  };

  /**
   * ADR-028 D1 shadow SSOT — not public; not applied to LWC.
   * @type {{
   *   intent: 'LIVE'|'HISTORY'|null,
   *   geometry: {
   *     centerTime: number|null,
   *     visibleBars: number|null,
   *     barSpacing: number|null,
   *     rightPadding: number|null,
   *     centerLogical: number|null,
   *   }
   * }}
   */
  let shadowView = emptyShadowView();

  /**
   * Optional tip for shadow intent (logical index of last real bar).
   * Set via noteTipLogical — does not search store. Null until noted.
   * @type {number|null}
   */
  let notedTipLogical = null;

  /**
   * DataResolve seam (ADR-028) — compositor/store implements in D2.
   * TimeCamera must never search candle arrays itself.
   * @type {null|{ nearestLogicalForTime: (centerTimeMs: number) => number|null }}
   */
  let dataResolve = null;

  let isSyncing = false;
  let cameraGesturing = false;
  let gestureTimer = null;

  /** @type {null|((state: object) => void)} */
  let applyCommitted = null;
  /** @type {null|(() => boolean)} skip propose/commit when true (e.g. live paint) */
  let shouldSkip = null;

  function emptyShadowView() {
    return {
      intent: null,
      geometry: {
        centerTime: null,
        visibleBars: null,
        barSpacing: null,
        rightPadding: null,
        centerLogical: null,
      },
    };
  }

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

  // ─── ADR-028/029 pure helpers (no LWC / store / DOM) ─────────────────────

  /**
   * @param {number} visibleTo
   * @param {number} tipLogical last real bar logical index
   * @param {number} [slack]
   * @returns {'LIVE'|'HISTORY'|null}
   */
  function classifyViewIntent(visibleTo, tipLogical, slack) {
    const to = Number(visibleTo);
    const tip = Number(tipLogical);
    const s = Number.isFinite(slack) ? slack : SLACK;
    if (!Number.isFinite(to) || !Number.isFinite(tip)) return null;
    const overhang = to - tip;
    if (overhang >= -s) return 'LIVE';
    return 'HISTORY';
  }

  /**
   * Center logical of a visible range (not left edge).
   * @param {{ from: number, to: number }} range
   * @returns {number|null}
   */
  function computeCenterLogical(range) {
    if (!isFiniteLogicalRange(range)) return null;
    return (range.from + range.to) / 2;
  }

  /**
   * Unix-ms at the bar nearest the visual center. Caller supplies times —
   * TimeCamera never owns this array search as policy; helper is pure for tests / D2.
   * @param {number[]} timesSec ascending unix seconds
   * @param {{ from: number, to: number }} range
   * @returns {number|null} unix ms
   */
  function computeCenterTimeMs(timesSec, range) {
    if (!Array.isArray(timesSec) || !timesSec.length) return null;
    const mid = computeCenterLogical(range);
    if (mid == null) return null;
    const idx = Math.max(0, Math.min(timesSec.length - 1, Math.round(mid)));
    const sec = Number(timesSec[idx]);
    if (!Number.isFinite(sec)) return null;
    return Math.floor(sec * 1000);
  }

  /**
   * Breathing room past tip (logical). Negative overhang → 0.
   * @param {number} visibleTo
   * @param {number} tipLogical
   * @returns {number|null}
   */
  function computeRightPadding(visibleTo, tipLogical) {
    const to = Number(visibleTo);
    const tip = Number(tipLogical);
    if (!Number.isFinite(to) || !Number.isFinite(tip)) return null;
    return Math.max(0, to - tip);
  }

  /**
   * ADR-029 LIVE clamp — UI padding, not market density.
   * @param {number} saved
   * @param {number} visibleBars
   * @returns {number}
   */
  function clampRightPadding(saved, visibleBars) {
    const s = Number(saved);
    const bars = Number(visibleBars);
    if (!Number.isFinite(s) || s < 0) return 0;
    const vb = Number.isFinite(bars) && bars > 0 ? bars : 150;
    const cap = Math.min(50, Math.max(5, Math.floor(vb / 4)));
    return Math.min(s, cap);
  }

  /**
   * DataResolve contract (ADR-028). D1: interface / TODO seam only.
   * @param {number} centerTimeMs
   * @returns {number|null}
   */
  function resolveNearestLogical(centerTimeMs) {
    if (!dataResolve || typeof dataResolve.nearestLogicalForTime !== 'function') {
      return null;
    }
    const t = Number(centerTimeMs);
    if (!Number.isFinite(t)) return null;
    try {
      const logical = dataResolve.nearestLogicalForTime(t);
      return Number.isFinite(logical) ? logical : null;
    } catch {
      return null;
    }
  }

  // ─── Shadow capture (D1 — after committed state only) ────────────────────

  function snapshotShadow() {
    return {
      intent: shadowView.intent,
      geometry: {
        centerTime: shadowView.geometry.centerTime,
        visibleBars: shadowView.geometry.visibleBars,
        barSpacing: shadowView.geometry.barSpacing,
        rightPadding: shadowView.geometry.rightPadding,
        centerLogical: shadowView.geometry.centerLogical,
      },
    };
  }

  /**
   * Refresh shadow from latest committed canonical. Never writes LWC.
   * @param {{ tipLogical?: number|null, timesSec?: number[]|null }} [opts]
   */
  function refreshShadowFromCanonical(opts) {
    const range = canonical.visibleRange;
    if (!isFiniteLogicalRange(range)) {
      shadowView = emptyShadowView();
      return;
    }

    const tip = opts && Object.prototype.hasOwnProperty.call(opts, 'tipLogical')
      ? opts.tipLogical
      : notedTipLogical;
    // Prefer explicit tip; else derive from committed rightOffset when coherent.
    let tipLogical = Number.isFinite(tip) ? Number(tip) : null;
    if (tipLogical == null && Number.isFinite(canonical.rightOffset)) {
      tipLogical = range.to - Number(canonical.rightOffset);
    }

    const visibleBars = range.to - range.from;
    const centerLogical = computeCenterLogical(range);
    let centerTime = null;
    const times = opts?.timesSec;
    if (Array.isArray(times) && times.length) {
      centerTime = computeCenterTimeMs(times, range);
    }

    const rightPadding = tipLogical != null
      ? computeRightPadding(range.to, tipLogical)
      : (Number.isFinite(canonical.rightOffset) ? Math.max(0, canonical.rightOffset) : null);

    const intent = tipLogical != null
      ? classifyViewIntent(range.to, tipLogical, SLACK)
      : null;

    shadowView = {
      intent,
      geometry: {
        centerTime,
        visibleBars: Number.isFinite(visibleBars) ? visibleBars : null,
        barSpacing: Number.isFinite(canonical.barSpacing) ? canonical.barSpacing : null,
        rightPadding,
        centerLogical,
      },
    };

    // Debug-only comparison — never in production paths unless explicitly enabled.
    try {
      if (typeof global !== 'undefined' && global.__TIME_CAMERA_SHADOW_DEBUG__) {
        // eslint-disable-next-line no-console
        console.debug('[TimeCamera D1 shadow]', {
          canonical: snapshot(),
          shadow: snapshotShadow(),
        });
      }
    } catch { /* */ }
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
    dataResolve = null;
    isSyncing = false;
    cameraGesturing = false;
    if (gestureTimer) {
      clearTimeout(gestureTimer);
      gestureTimer = null;
    }
  }

  /**
   * Optional tip logical for shadow intent (from series length − 1). Not a store search.
   * @param {number|null|undefined} tipLogical
   */
  function noteTipLogical(tipLogical) {
    const t = Number(tipLogical);
    notedTipLogical = Number.isFinite(t) ? t : null;
  }

  /**
   * Bind DataResolve seam (D2). D1 may leave unbound — resolveNearestLogical returns null.
   * @param {{ nearestLogicalForTime: (centerTimeMs: number) => number|null }|null} api
   */
  function bindDataResolve(api) {
    if (!api || typeof api.nearestLogicalForTime !== 'function') {
      dataResolve = null;
      return;
    }
    dataResolve = api;
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

    // D1: shadow capture only after successful committed apply (not mid-sync).
    refreshShadowFromCanonical();
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

  /** @private tests / D1 debug — shadow VIEW (not applied) */
  function _getShadowView() {
    return snapshotShadow();
  }

  /** @private tests */
  function _resetForTests() {
    unbind();
    canonical = { visibleRange: null, barSpacing: null, rightOffset: null };
    shadowView = emptyShadowView();
    notedTipLogical = null;
  }

  const TimeCamera = {
    SLACK,
    bind,
    unbind,
    commit,
    proposeFromPane,
    isSyncing: isSyncingNow,
    isGesturing,
    getCanonical,
    isFiniteLogicalRange,
    // D1 seams — not navigation public API for product callers.
    noteTipLogical,
    bindDataResolve,
    resolveNearestLogical,
    _getShadowView,
    _refreshShadowFromCanonical: refreshShadowFromCanonical,
    _resetForTests,
    // Pure helpers for unit tests / future D2 (not LWC owners).
    _helpers: {
      classifyViewIntent,
      computeCenterLogical,
      computeCenterTimeMs,
      computeRightPadding,
      clampRightPadding,
    },
  };

  global.TimeCamera = TimeCamera;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = TimeCamera;
  }
})(typeof window !== 'undefined' ? window : globalThis);
