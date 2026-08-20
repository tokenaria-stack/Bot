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
  /** Max zoom-out (logical bars). Gesture + LIVE TF restore — one wall (`MAX_VISIBLE_BARS`). */
  const MAX_VISIBLE_LOGICAL_BARS = (typeof MAX_VISIBLE_BARS !== 'undefined'
    && Number.isFinite(MAX_VISIBLE_BARS) && MAX_VISIBLE_BARS > 0)
    ? MAX_VISIBLE_BARS
    : 5000;

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
    if (dataResolve && typeof dataResolve.nearestLogicalForTime === 'function') {
      const t = Number(centerTimeMs);
      if (!Number.isFinite(t)) return null;
      try {
        const logical = dataResolve.nearestLogicalForTime(t);
        if (Number.isFinite(logical)) return logical;
      } catch { /* fall through */ }
    }
    // Fallback: binary search on noted series (Mode B must not silently abort).
    // Empty noted series stays null (findIndexByTimeMs would return 0).
    if (!notedTimesSec || !notedTimesSec.length) return null;
    const t = Number(centerTimeMs);
    if (!Number.isFinite(t)) return null;
    const CDS = typeof ChartDataStore !== 'undefined' ? ChartDataStore : global.ChartDataStore;
    const targetSec = CDS.msToChartSec(t);
    let lo = 0;
    let hi = notedTimesSec.length - 1;
    while (lo < hi) {
      const mid = (lo + hi) >> 1;
      if (Number(notedTimesSec[mid]) < targetSec) lo = mid + 1;
      else hi = mid;
    }
    if (lo > 0) {
      const prevDelta = Math.abs(Number(notedTimesSec[lo - 1]) - targetSec);
      const currDelta = Math.abs(Number(notedTimesSec[lo]) - targetSec);
      if (prevDelta < currDelta) return lo - 1;
    }
    return lo;
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

  function commit(patch, options = {}) {
    if (isSyncing) return false;
    if (!applyCommitted) return false;
    if (!patch || typeof patch !== 'object') return false;

    const force = options.force === true;
    const next = snapshot();
    let dirty = false;

    if (Object.prototype.hasOwnProperty.call(patch, 'visibleRange')) {
      const r = cloneRange(patch.visibleRange);
      if (r) {
        if (force
          || !next.visibleRange
          || next.visibleRange.from !== r.from
          || next.visibleRange.to !== r.to) {
          next.visibleRange = r;
          dirty = true;
        }
      }
    }
    if (Object.prototype.hasOwnProperty.call(patch, 'barSpacing')) {
      const s = patch.barSpacing;
      if (Number.isFinite(s) && s > 0 && (force || next.barSpacing !== s)) {
        next.barSpacing = s;
        dirty = true;
      }
    }
    if (Object.prototype.hasOwnProperty.call(patch, 'rightOffset')) {
      const o = patch.rightOffset;
      if (Number.isFinite(o) && (force || next.rightOffset !== o)) {
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
        rangeOnly: patch.rangeOnly === true,
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
    const clamped = clampVisibleLogicalWidth(visibleRange, MAX_VISIBLE_LOGICAL_BARS);
    const patch = {
      visibleRange: clamped,
      sourceHostId: hostId || 'unknown',
    };
    if (Number.isFinite(barSpacing) && barSpacing > 0) {
      patch.barSpacing = barSpacing;
    }
    if (notedTipLogical != null && Number.isFinite(clamped.to)) {
      const pad = computeRightPadding(clamped.to, notedTipLogical);
      if (pad != null) patch.rightOffset = pad;
    }
    return commit(patch);
  }

  /**
   * Cap zoom-out width. Keep the right edge of the range (trading-chart convention).
   * @param {{ from: number, to: number }} range
   * @param {number} maxBars
   */
  function clampVisibleLogicalWidth(range, maxBars) {
    const from = Number(range.from);
    const to = Number(range.to);
    const max = Number(maxBars);
    if (![from, to].every(Number.isFinite) || !(to > from)) return range;
    if (!Number.isFinite(max) || max <= 0) return range;
    const width = to - from;
    if (width <= max) return range;
    return { from: to - max, to };
  }

  /**
   * Preserve VIEW after history prepend/append paint (post-setData).
   * Sacred: market-time edges captured before mutation. Resolve against the NEW
   * series and force-apply to LWC. Never derive shift from newLength - oldLength.
   *
   * @param {{
   *   anchorTimeMs?: number,
   *   leftTimeMs?: number,
   *   rightTimeMs?: number,
   *   logicalOffset?: number,
   *   rightLogicalOffset?: number,
   *   visibleBars?: number,
   *   tipLogical?: number|null,
   *   timesSec?: number[],
   *   barSpacing?: number|null,
   *   force?: boolean,
   * }} opts
   */
  function barTimeToMs(sec) {
    const t = Number(sec);
    if (!Number.isFinite(t)) return null;
    const CDS = typeof ChartDataStore !== 'undefined' ? ChartDataStore : global.ChartDataStore;
    return CDS.secToMs(t);
  }

  /**
   * Mode B policy only — resolve pre-merge market times → final logical range.
   * Does NOT write LWC. Compositor applies via forceVisibleLogicalRange (sync).
   *
   * B1: both edges survive → exact [from, to]
   * B2: left survives, right evicted → left-anchored width (NOT tip snap)
   * B3: left gone → tip-anchored width fallback
   *
   * @returns {{ from: number, to: number, caseId: string, targetFromMs: number, targetToMs: number }|null}
   */
  function resolveMarketTimePreserve(opts) {
    if (!opts || typeof opts !== 'object') return null;
    if (Array.isArray(opts.timesSec) && opts.timesSec.length) {
      notedTimesSec = opts.timesSec.slice();
    }
    if (!notedTimesSec || !notedTimesSec.length) return null;

    const tipIn = Number(opts.tipLogical);
    if (Number.isFinite(tipIn) && tipIn >= 0) {
      notedTipLogical = tipIn;
    } else {
      notedTipLogical = notedTimesSec.length - 1;
    }

    const leftMs = Number(
      Object.prototype.hasOwnProperty.call(opts, 'leftTimeMs')
        ? opts.leftTimeMs
        : opts.anchorTimeMs,
    );
    const rightMs = Number(opts.rightTimeMs);
    if (!Number.isFinite(leftMs) || !Number.isFinite(rightMs) || !(rightMs > leftMs)) {
      return null;
    }

    const dataFirstMs = barTimeToMs(notedTimesSec[0]);
    const dataLastMs = barTimeToMs(notedTimesSec[notedTimesSec.length - 1]);
    if (dataFirstMs == null || dataLastMs == null || !(dataLastMs >= dataFirstMs)) {
      return null;
    }

    const widthMs = rightMs - leftMs;
    const leftOk = leftMs >= dataFirstMs && leftMs <= dataLastMs;
    const rightOk = rightMs >= dataFirstMs && rightMs <= dataLastMs;

    let targetFromMs = leftMs;
    let targetToMs = rightMs;
    let caseId = 'B1';
    if (leftOk && rightOk) {
      caseId = 'B1';
    } else if (leftOk && !rightOk) {
      caseId = 'B2';
      targetFromMs = leftMs;
      targetToMs = Math.min(leftMs + widthMs, dataLastMs);
    } else {
      caseId = 'B3';
      targetToMs = dataLastMs;
      targetFromMs = Math.max(dataFirstMs, dataLastMs - widthMs);
    }
    if (!(targetToMs > targetFromMs)) return null;

    const fromIdx = resolveNearestLogical(targetFromMs);
    const toIdx = resolveNearestLogical(targetToMs);
    if (fromIdx == null || toIdx == null) return null;

    let leftOff = Number(opts.logicalOffset);
    if (!Number.isFinite(leftOff)) leftOff = 0;
    let rightOff = Number(opts.rightLogicalOffset);
    if (!Number.isFinite(rightOff)) rightOff = 0;

    let from;
    let to;
    if (caseId === 'B1') {
      from = fromIdx + leftOff;
      to = toIdx + rightOff;
    } else if (caseId === 'B2') {
      from = fromIdx + leftOff;
      to = toIdx;
    } else {
      from = fromIdx;
      to = toIdx;
    }

    if (!(to > from) || !Number.isFinite(from) || !Number.isFinite(to)) return null;
    return { from, to, caseId, targetFromMs, targetToMs };
  }

  /**
   * Mode B — resolve + commit canonical (tests / non-LWC). Live paint must apply
   * via ChartAdapter.forceVisibleLogicalRange after resolve — commit-only is not
   * authoritative during compositor liveUpdating (Mode A already learned this).
   */
  function proposeMarketTimePreserve(opts) {
    const resolved = resolveMarketTimePreserve(opts);
    if (!resolved) return false;

    const tip = notedTipLogical;
    const patch = {
      visibleRange: { from: resolved.from, to: resolved.to },
      sourceHostId: 'system',
      rangeOnly: true,
    };
    if (Number.isFinite(tip)) {
      patch.rightOffset = Math.max(0, resolved.to - tip);
    }

    beginPreserveTransaction();
    const commitOpts = { force: opts?.force !== false };
    const ok = commit(patch, commitOpts);
    if (!ok) releasePreserveTransaction();
    try {
      if (typeof global !== 'undefined' && global.__TIME_CAMERA_MARKET_PRESERVE_DEBUG__) {
        // eslint-disable-next-line no-console
        console.debug('[TimeCamera] proposeMarketTimePreserve', resolved);
      }
    } catch { /* */ }
    return ok;
  }

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

    const from = anchorIndex + offset;

    let to;
    const rightMs = Number(opts.rightTimeMs);
    if (Number.isFinite(rightMs)) {
      const rightIndex = resolveNearestLogical(rightMs);
      if (rightIndex == null) return false;
      let rightOffset = Number(opts.rightLogicalOffset);
      if (!Number.isFinite(rightOffset)) rightOffset = 0;
      to = rightIndex + rightOffset;
    } else {
      // Preserve path: keep the pre-prepend viewport width (incl. wide HISTORY zoom).
      let bars = Number(opts.visibleBars);
      if (!Number.isFinite(bars) || bars <= 0) bars = HEALTHY_VISIBLE_BARS;
      to = from + bars;
    }

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
    const ok = commit(patch, { force: opts.force === true });
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
      const bars = Number(seed.visibleBars);
      if (!Number.isFinite(bars) || bars <= 0) {
        return proposeFreshLive({ tipLogical: tip });
      }
      let spacing = Number(seed.barSpacing);
      if (!Number.isFinite(spacing) || spacing < MIN_HEALTHY_BAR_SPACING) {
        spacing = HEALTHY_BAR_SPACING;
      }
      let pad = Number(seed.rightPadding);
      if (!Number.isFinite(pad)) pad = Number(seed.rightOffset);
      if (!Number.isFinite(pad) || pad < 0) pad = 0;
      const to = tip + pad;
      const from = to - bars;
      const visibleRange = clampVisibleLogicalWidth({ from, to }, MAX_VISIBLE_LOGICAL_BARS);
      return commit({
        visibleRange,
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

  /** Read-only clamped VIEW for prefetch. Null until a finite CameraCommit exists. */
  function getCanonicalVisibleRange() {
    return cloneRange(canonical.visibleRange);
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
    MAX_HEALTHY_VISIBLE_BARS,
    MAX_VISIBLE_LOGICAL_BARS,
    bind,
    unbind,
    commit,
    proposeFromPane,
    proposeFreshLive,
    proposePreserveViewport,
    resolveMarketTimePreserve,
    proposeMarketTimePreserve,
    proposeAfterData,
    beginPreserveTransaction,
    releasePreserveTransaction,
    hasOpenPreserveTransaction,
    isSyncing: isSyncingNow,
    isGesturing,
    getCanonical,
    getCanonicalVisibleRange,
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
      barTimeToMs,
    },
  };

  global.TimeCamera = TimeCamera;
  if (typeof module !== 'undefined' && module.exports) {
    if (typeof global.ChartDataStore === 'undefined') {
      global.ChartDataStore = require('../store.js').ChartDataStore;
    }
    module.exports = TimeCamera;
  }
})(typeof window !== 'undefined' ? window : globalThis);
