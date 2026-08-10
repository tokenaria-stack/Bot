/**
 * TimeCamera — ADR-021 / ADR-028 / ADR-029 sole owner of live timeline + navigation VIEW.
 *
 * Owns: ViewIntent + ViewGeometry (SSOT); canonical range/spacing/offset as CameraCommit outputs.
 * Does not talk to LWC. ChartAdapter binds applyCommitted.
 * Does not search market bars — DataResolve (bound by compositor) owns time→logical.
 *
 * Atomic commit only. D2: navigation policy lives here; ViewportManager is capture/translate only.
 */
(function (global) {
  'use strict';

  const SLACK = 1.5;
  const GESTURE_MUTE_MS = 100;
  const HEALTHY_BAR_SPACING = 6;
  const HEALTHY_VISIBLE_BARS = 150;
  const MAX_HEALTHY_VISIBLE_BARS = 400;
  const MIN_HEALTHY_BAR_SPACING = 1;

  /** @type {{ visibleRange: { from: number, to: number }|null, barSpacing: number|null, rightOffset: number|null }} */
  let canonical = {
    visibleRange: null,
    barSpacing: null,
    rightOffset: null,
  };

  let shadowView = emptyShadowView();
  let notedTipLogical = null;
  let notedTimesSec = null;
  /** @type {null|{ nearestLogicalForTime: (centerTimeMs: number) => number|null }} */
  let dataResolve = null;

  let isSyncing = false;
  let cameraGesturing = false;
  let gestureTimer = null;
  /** @type {null|((state: object) => void)} */
  let applyCommitted = null;
  /** @type {null|(() => boolean)} */
  let shouldSkip = null;

  /**
   * Preserve transaction: system-owned VIEW after prepend remapping.
   * Stale LWC/pane echoes must not overwrite until released.
   * User gesture (wheel/pointer → releasePreserveTransaction, or non-system commit)
   * takes ownership immediately.
   * @type {number|null}
   */
  let openPreserveEpoch = null;
  let preserveEpochSeq = 0;

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

  function classifyViewIntent(visibleTo, tipLogical, slack) {
    const to = Number(visibleTo);
    const tip = Number(tipLogical);
    const s = Number.isFinite(slack) ? slack : SLACK;
    if (!Number.isFinite(to) || !Number.isFinite(tip)) return null;
    return (to - tip) >= -s ? 'LIVE' : 'HISTORY';
  }

  function computeCenterLogical(range) {
    if (!isFiniteLogicalRange(range)) return null;
    return (range.from + range.to) / 2;
  }

  function computeCenterTimeMs(timesSec, range) {
    if (!Array.isArray(timesSec) || !timesSec.length) return null;
    const mid = computeCenterLogical(range);
    if (mid == null) return null;
    const idx = Math.max(0, Math.min(timesSec.length - 1, Math.round(mid)));
    const sec = Number(timesSec[idx]);
    if (!Number.isFinite(sec)) return null;
    return Math.floor(sec * 1000);
  }

  function computeRightPadding(visibleTo, tipLogical) {
    const to = Number(visibleTo);
    const tip = Number(tipLogical);
    if (!Number.isFinite(to) || !Number.isFinite(tip)) return null;
    return Math.max(0, to - tip);
  }

  function clampRightPadding(saved, visibleBars) {
    const s = Number(saved);
    const bars = Number(visibleBars);
    if (!Number.isFinite(s) || s < 0) return 0;
    const vb = Number.isFinite(bars) && bars > 0 ? bars : HEALTHY_VISIBLE_BARS;
    const cap = Math.min(50, Math.max(5, Math.floor(vb / 4)));
    return Math.min(s, cap);
  }

  function sanitizeVisibleBars(bars) {
    let b = Number(bars);
    if (!Number.isFinite(b) || b <= 0) b = HEALTHY_VISIBLE_BARS;
    if (b > MAX_HEALTHY_VISIBLE_BARS) b = MAX_HEALTHY_VISIBLE_BARS;
    if (b < 50) b = 50;
    return b;
  }

  function sanitizeBarSpacing(spacing) {
    const s = Number(spacing);
    if (!Number.isFinite(s) || s < MIN_HEALTHY_BAR_SPACING) return HEALTHY_BAR_SPACING;
    return s;
  }

  function resolveNearestLogical(centerTimeMs) {
    if (!dataResolve || typeof dataResolve.nearestLogicalForTime !== 'function') return null;
    const t = Number(centerTimeMs);
    if (!Number.isFinite(t)) return null;
    try {
      const logical = dataResolve.nearestLogicalForTime(t);
      return Number.isFinite(logical) ? logical : null;
    } catch {
      return null;
    }
  }

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

  function refreshShadowFromCanonical(opts) {
    const range = canonical.visibleRange;
    if (!isFiniteLogicalRange(range)) {
      shadowView = emptyShadowView();
      return;
    }
    const tipLogical = opts && Object.prototype.hasOwnProperty.call(opts, 'tipLogical')
      ? (Number.isFinite(opts.tipLogical) ? Number(opts.tipLogical) : null)
      : notedTipLogical;
    const times = opts && Object.prototype.hasOwnProperty.call(opts, 'timesSec')
      ? opts.timesSec
      : notedTimesSec;
    const visibleBars = range.to - range.from;
    const centerLogical = computeCenterLogical(range);
    let centerTime = null;
    if (Array.isArray(times) && times.length) {
      centerTime = computeCenterTimeMs(times, range);
    }
    const rightPadding = tipLogical != null
      ? computeRightPadding(range.to, tipLogical)
      : null;
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
    try {
      if (typeof global !== 'undefined' && global.__TIME_CAMERA_SHADOW_DEBUG__) {
        // eslint-disable-next-line no-console
        console.debug('[TimeCamera shadow]', { canonical: snapshot(), shadow: snapshotShadow() });
      }
    } catch { /* */ }
  }

  function observeCommittedWorld(world) {
    if (!world || typeof world !== 'object') return;
    const tip = Number(world.tipLogical);
    notedTipLogical = Number.isFinite(tip) ? tip : null;
    if (Array.isArray(world.timesSec) && world.timesSec.length) {
      notedTimesSec = world.timesSec.slice();
    } else {
      notedTimesSec = null;
    }
    refreshShadowFromCanonical({
      tipLogical: notedTipLogical,
      timesSec: notedTimesSec,
    });
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
    releasePreserveTransaction();
    cameraGesturing = true;
    if (gestureTimer) clearTimeout(gestureTimer);
    gestureTimer = setTimeout(() => {
      cameraGesturing = false;
      gestureTimer = null;
    }, GESTURE_MUTE_MS);
  }

  /** Open a system preserve txn; returns epoch token. */
  function beginPreserveTransaction() {
    preserveEpochSeq += 1;
    openPreserveEpoch = preserveEpochSeq;
    return openPreserveEpoch;
  }

  /** End system preserve ownership (echo consumed or user took control). */
  function releasePreserveTransaction() {
    openPreserveEpoch = null;
  }

  function hasOpenPreserveTransaction() {
    return openPreserveEpoch != null;
  }

  function bind(hooks) {
    applyCommitted = typeof hooks?.applyCommitted === 'function' ? hooks.applyCommitted : null;
    shouldSkip = typeof hooks?.shouldSkip === 'function' ? hooks.shouldSkip : null;
  }

  function unbind() {
    applyCommitted = null;
    shouldSkip = null;
    dataResolve = null;
    notedTipLogical = null;
    notedTimesSec = null;
    isSyncing = false;
    cameraGesturing = false;
    openPreserveEpoch = null;
    if (gestureTimer) {
      clearTimeout(gestureTimer);
      gestureTimer = null;
    }
  }

  function noteTipLogical(tipLogical) {
    const t = Number(tipLogical);
    notedTipLogical = Number.isFinite(t) ? t : null;
  }

  function bindDataResolve(api) {
    if (!api || typeof api.nearestLogicalForTime !== 'function') {
      dataResolve = null;
      return;
    }
    dataResolve = api;
  }

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

    refreshShadowFromCanonical({
      tipLogical: notedTipLogical,
      timesSec: notedTimesSec,
    });
    return true;
  }

  function proposeFromPane(hostId, visibleRange, barSpacing) {
    if (isSyncing) return false;
    if (shouldSkip && shouldSkip()) return false;
    // Stale echo after system preserve: consume txn, do not overwrite reconstructed VIEW.
    // Real user gestures call releasePreserveTransaction() first (wheel/pointer).
    if (openPreserveEpoch != null) {
      releasePreserveTransaction();
      return false;
    }
    if (!isFiniteLogicalRange(visibleRange)) return false;
    const patch = {
      visibleRange,
      sourceHostId: hostId || 'unknown',
    };
    if (Number.isFinite(barSpacing) && barSpacing > 0) {
      patch.barSpacing = barSpacing;
    }
    if (notedTipLogical != null && Number.isFinite(visibleRange.to)) {
      const pad = computeRightPadding(visibleRange.to, notedTipLogical);
      if (pad != null) patch.rightOffset = pad;
    }
    return commit(patch);
  }

  /**
   * Preserve VIEW after left-side data growth (prepend).
   * Store time is identity; logicalOffset preserves intentional empty space (incl. from < 0).
   * Never FreshLive on failure — returns false so Loading cannot smuggle navigation.
   * @param {{
   *   anchorTimeMs?: number,
   *   leftTimeMs?: number,
   *   logicalOffset?: number,
   *   visibleBars?: number,
   *   tipLogical?: number|null,
   *   timesSec?: number[],
   *   barSpacing?: number|null,
   * }} opts
   */
  function proposePreserveViewport(opts) {
    if (!opts || typeof opts !== 'object') return false;
    if (Array.isArray(opts.timesSec) && opts.timesSec.length) {
      notedTimesSec = opts.timesSec.slice();
    }
    const tipIn = Number(opts.tipLogical);
    if (Number.isFinite(tipIn) && tipIn >= 0) {
      notedTipLogical = tipIn;
    } else if (notedTimesSec && notedTimesSec.length) {
      notedTipLogical = notedTimesSec.length - 1;
    }

    const anchorMs = Number(
      Object.prototype.hasOwnProperty.call(opts, 'anchorTimeMs')
        ? opts.anchorTimeMs
        : opts.leftTimeMs,
    );
    if (!Number.isFinite(anchorMs)) return false;

    const anchorIndex = resolveNearestLogical(anchorMs);
    if (anchorIndex == null) return false;

    let offset = Number(opts.logicalOffset);
    if (!Number.isFinite(offset)) offset = 0;

    // Preserve path: keep the pre-prepend viewport width (incl. wide HISTORY zoom).
    // Do NOT clamp to MAX_HEALTHY_VISIBLE_BARS — that truncates `to = from + bars` and
    // LWC then re-expands the range, moving the left market-time edge (P0 FAIL).
    let bars = Number(opts.visibleBars);
    if (!Number.isFinite(bars) || bars <= 0) bars = HEALTHY_VISIBLE_BARS;

    const from = anchorIndex + offset;
    const to = from + bars;
    if (!(to > from) || !Number.isFinite(from) || !Number.isFinite(to)) return false;

    const tip = notedTipLogical;
    const patch = {
      visibleRange: { from, to },
      sourceHostId: 'system',
    };
    if (Object.prototype.hasOwnProperty.call(opts, 'barSpacing') && Number.isFinite(opts.barSpacing)) {
      patch.barSpacing = sanitizeBarSpacing(opts.barSpacing);
    }
    if (Number.isFinite(tip)) {
      patch.rightOffset = Math.max(0, to - tip);
    }

    beginPreserveTransaction();
    const ok = commit(patch);
    if (!ok) releasePreserveTransaction();
    return ok;
  }

  /**
   * ADR-029 Fresh LIVE — tip + healthy zoom + zero breathing room (or cold spacing-only).
   * @param {{ tipLogical?: number|null }} [opts]
   */
  function proposeFreshLive(opts) {
    const tip = Number(opts?.tipLogical);
    if (!Number.isFinite(tip) || tip < 0) {
      return commit({
        barSpacing: HEALTHY_BAR_SPACING,
        rightOffset: 0,
        sourceHostId: 'system',
      });
    }
    const bars = HEALTHY_VISIBLE_BARS;
    const to = tip;
    const from = Math.max(0, to - bars);
    return commit({
      visibleRange: { from, to },
      barSpacing: HEALTHY_BAR_SPACING,
      rightOffset: 0,
      sourceHostId: 'system',
    });
  }

  /**
   * ADR-029 propose after series data is painted + tip/times observed.
   * @param {{
   *   tipLogical: number,
   *   timesSec?: number[],
   *   seed?: object|null,
   *   mode?: 'fresh'|'switch'|'preserve',
   * }} opts
   */
  function proposeAfterData(opts) {
    if (!opts || typeof opts !== 'object') return false;
    const mode = opts.mode || 'switch';
    if (mode === 'preserve') return false;

    const tip = Number(opts.tipLogical);
    if (Array.isArray(opts.timesSec) && opts.timesSec.length) {
      notedTimesSec = opts.timesSec.slice();
    }
    if (Number.isFinite(tip)) notedTipLogical = tip;

    if (mode === 'fresh' || !opts.seed) {
      return proposeFreshLive({ tipLogical: Number.isFinite(tip) ? tip : null });
    }

    const seed = opts.seed;
    let intent = seed.intent === 'LIVE' || seed.intent === 'HISTORY' ? seed.intent : null;
    if (seed._liveEdge === true) intent = 'LIVE';
    if (seed._liveEdge === false) intent = 'HISTORY';
    if (!intent) intent = 'LIVE';

    if (intent === 'LIVE') {
      if (!Number.isFinite(tip)) return proposeFreshLive({});
      const bars = sanitizeVisibleBars(seed.visibleBars);
      const spacing = sanitizeBarSpacing(seed.barSpacing);
      let pad = Number(seed.rightPadding);
      if (!Number.isFinite(pad)) pad = Number(seed.rightOffset);
      if (!Number.isFinite(pad)) pad = 0;
      pad = clampRightPadding(pad, bars);
      const to = tip + pad;
      const from = Math.max(0, to - bars);
      return commit({
        visibleRange: { from, to },
        barSpacing: spacing,
        rightOffset: pad,
        sourceHostId: 'system',
      });
    }

    // HISTORY — center via DataResolve; never jump to tip/live.
    if (!Number.isFinite(tip)) return false;
    const centerTime = Number(seed.centerTime);
    const centerLogical = resolveNearestLogical(centerTime);
    if (centerLogical == null) {
      return false;
    }
    const bars = sanitizeVisibleBars(seed.visibleBars);
    const spacing = sanitizeBarSpacing(seed.barSpacing);
    const half = bars / 2;
    let from = centerLogical - half;
    let to = centerLogical + half;
    if (from < 0) {
      to -= from;
      from = 0;
    }
    if (to <= from) {
      // Degenerate clamp — keep center; never snap tip (HISTORY must not jump live).
      from = Math.max(0, centerLogical - half);
      to = from + bars;
    }
    const rightPadding = Math.max(0, to - tip);
    return commit({
      visibleRange: { from, to },
      barSpacing: spacing,
      rightOffset: rightPadding,
      sourceHostId: 'system',
    });
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

  function _getShadowView() {
    return snapshotShadow();
  }

  function _resetForTests() {
    unbind();
    canonical = { visibleRange: null, barSpacing: null, rightOffset: null };
    shadowView = emptyShadowView();
    notedTipLogical = null;
    notedTimesSec = null;
    openPreserveEpoch = null;
    preserveEpochSeq = 0;
  }

  const TimeCamera = {
    SLACK,
    HEALTHY_BAR_SPACING,
    HEALTHY_VISIBLE_BARS,
    bind,
    unbind,
    commit,
    proposeFromPane,
    proposeFreshLive,
    proposePreserveViewport,
    proposeAfterData,
    beginPreserveTransaction,
    releasePreserveTransaction,
    hasOpenPreserveTransaction,
    isSyncing: isSyncingNow,
    isGesturing,
    getCanonical,
    isFiniteLogicalRange,
    observeCommittedWorld,
    noteTipLogical,
    bindDataResolve,
    resolveNearestLogical,
    _getShadowView,
    _refreshShadowFromCanonical: refreshShadowFromCanonical,
    _resetForTests,
    _helpers: {
      classifyViewIntent,
      computeCenterLogical,
      computeCenterTimeMs,
      computeRightPadding,
      clampRightPadding,
      sanitizeVisibleBars,
      sanitizeBarSpacing,
    },
  };

  global.TimeCamera = TimeCamera;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = TimeCamera;
  }
})(typeof window !== 'undefined' ? window : globalThis);
