/**
 * HydrationOrchestrator — FSM coordinator for monolithic columnar history edge loads.
 * Wave 2: owns a single pending left-history intent (not a queue). Busy delays; never drops.
 * Microscope island: also owns a single pending right-history intent (append toward live).
 */
const HydrationState = Object.freeze({
  IDLE: 'IDLE',
  PREPENDING: 'PREPENDING',
  APPLYING: 'APPLYING',
  LIVE: 'LIVE',
});

class HydrationOrchestrator {
  constructor() {
    /** @type {'IDLE'|'PREPENDING'|'APPLYING'|'LIVE'} */
    this.state = HydrationState.IDLE;
    /** @type {object[]} */
    this.wsQueue = [];
    this.debounceTimer = null;
    this.debounceMs = 200;
    /** @type {object|null} */
    this._deps = null;
    this._inFlight = false;
    /** @type {'left'|'right'|null} */
    this._hydrateEdge = null;
    /** Last completed edge — used with isRenderBusy to ignore prune-echo opposite notes. */
    /** @type {'left'|'right'|null} */
    this._lastHydrateEdge = null;
    /**
     * Wave 2 — newest left-history need only (supersede, never accumulate).
     * @type {{ range: { from: number, to: number }, options: object }|null}
     */
    this._pendingLeftIntent = null;
    /**
     * Right-edge island fill — newest right-history need only.
     * @type {{ range: { from: number, to: number }, options: object }|null}
     */
    this._pendingRightIntent = null;
    /** LEFT store head (firstTimeSec) that returned added<=0. */
    this._zeroProgressLeftHeadSec = null;
    /** RIGHT store tip (lastTimeSec) that returned added<=0. */
    this._zeroProgressRightTipSec = null;
  }

  /**
   * @param {object} deps
   * @param {() => number} deps.getEpoch
   * @param {() => number} [deps.getReqId]
   * @param {(range: object, options?: object) => boolean} deps.shouldLoad
   * @param {() => number|null} deps.getAnchorEndTimeSec
   * @param {() => string[]} [deps.getSlotIds]
   * @param {(endTimeSec: number) => Promise<object>} deps.fetchColumnar
   * @param {(cursorSec: number) => Promise<object>} [deps.fetchRightColumnar]
   * @param {(data: object) => { added: number, viewportRange?: object|null }|null} deps.mergeIntoStore
   * @param {(range: object, options?: object) => boolean} [deps.shouldLoadRight]
   * @param {() => boolean} [deps.shouldContinueRightHistory]
   * @param {() => number|null} [deps.getRightFetchEndSec]
   * @param {(data: object) => { added: number, viewportRange?: object|null }|null} [deps.mergeAppendIntoStore]
   * @param {(intent: object) => void} deps.markDirty
   * @param {(tick: object) => void} deps.processTick
   * @param {() => boolean} [deps.isRenderBusy]
   * @param {() => boolean} [deps.isDashboardLoading]
   * @param {() => object|null} [deps.getVisibleRange]
   */
  init(deps) {
    this._deps = deps;
  }

  getState() {
    return this.state;
  }

  isBusy() {
    return this.state === HydrationState.PREPENDING
      || this.state === HydrationState.APPLYING
      || this._inFlight;
  }

  /** @returns {boolean} */
  hasPendingLeftIntent() {
    return this._pendingLeftIntent != null;
  }

  /** @returns {boolean} */
  hasPendingRightIntent() {
    return this._pendingRightIntent != null;
  }

  reset() {
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
      this.debounceTimer = null;
    }
    this.wsQueue = [];
    this._pendingLeftIntent = null;
    this._pendingRightIntent = null;
    this.state = HydrationState.IDLE;
    this._inFlight = false;
    this._hydrateEdge = null;
    this._lastHydrateEdge = null;
    this._zeroProgressLeftHeadSec = null;
    this._zeroProgressRightTipSec = null;
  }

  _currentLeftHeadSec() {
    const t = Number(this._deps?.getAnchorEndTimeSec?.());
    return Number.isFinite(t) && t > 0 ? t : null;
  }

  _currentRightTipSec() {
    const t = Number(this._deps?.getRightTipSec?.());
    return Number.isFinite(t) && t > 0 ? t : null;
  }

  /** True when LEFT fetch at the current store head already added 0 bars. */
  isLeftHeadBlocked() {
    const head = this._currentLeftHeadSec();
    return head != null
      && this._zeroProgressLeftHeadSec != null
      && head === this._zeroProgressLeftHeadSec;
  }

  /** True when RIGHT fetch at the current store tip already added 0 bars. */
  isRightTipBlocked() {
    const tip = this._currentRightTipSec();
    return tip != null
      && this._zeroProgressRightTipSec != null
      && tip === this._zeroProgressRightTipSec;
  }

  /**
   * True when an opposite-edge note would only rearm ping-pong (prune/setData echo).
   * Same-direction pending/continuation is unchanged.
   * @param {'left'|'right'} edge
   */
  _shouldIgnoreOppositeEdgeNote(edge) {
    const other = edge === 'left' ? 'right' : 'left';
    if (this._inFlight && this._hydrateEdge === other) return true;
    if (this.isBusy() && this._hydrateEdge === other) return true;
    if (this._lastHydrateEdge === other
      && typeof this._deps?.isRenderBusy === 'function'
      && this._deps.isRenderBusy()) {
      return true;
    }
    return false;
  }

  /** Drop queued ticks without replay (epoch/TF abort). */
  discardQueue() {
    this.wsQueue = [];
  }

  /**
   * Queue live WS tick while prepend/append is in flight.
   * @returns {boolean} true if queued (caller should skip immediate processing)
   */
  queueTick(tick) {
    if (this.state !== HydrationState.PREPENDING && this.state !== HydrationState.APPLYING) {
      return false;
    }
    this.wsQueue.push(tick);
    return true;
  }

  flushQueue() {
    const deps = this._deps;
    if (!deps?.processTick) {
      this.wsQueue = [];
      return;
    }
    const pending = this.wsQueue;
    this.wsQueue = [];
    for (let i = 0; i < pending.length; i++) {
      deps.processTick(pending[i]);
    }
  }

  /**
   * Wave 2 — Boot detector entry. Remembers newest need; starts when idle.
   * @param {{ from: number, to: number }} range
   * @param {object} [options]
   */
  noteLeftHistoryIntent(range, options = {}) {
    if (!this._deps || !range
      || !Number.isFinite(range.from)
      || !Number.isFinite(range.to)) {
      return;
    }
    if (this._shouldIgnoreOppositeEdgeNote('left')) return;
    if (this.isLeftHeadBlocked()) return;
    this._pendingLeftIntent = {
      range: { from: range.from, to: range.to },
      options: { ...options },
    };
    this.tryConsumePending();
  }

  /**
   * Right-edge island fill — remember newest need; starts when idle.
   * Independent of left historyHasMore / TF-switch center hydrate.
   * @param {{ from: number, to: number }} range
   * @param {object} [options]
   */
  noteRightHistoryIntent(range, options = {}) {
    if (!this._deps || !range
      || !Number.isFinite(range.from)
      || !Number.isFinite(range.to)) {
      return;
    }
    if (typeof this._deps.shouldLoadRight !== 'function'
      || typeof this._deps.getRightFetchEndSec !== 'function'
      || typeof this._deps.mergeAppendIntoStore !== 'function') {
      return;
    }
    if (this._shouldIgnoreOppositeEdgeNote('right')) return;
    if (this.isRightTipBlocked()) return;
    this._pendingRightIntent = {
      range: { from: range.from, to: range.to },
      options: { ...options },
    };
    this.tryConsumePending();
  }

  /**
   * Consume pending when Hydration (and paint/dashboard) are idle.
   * Called after prepend finishes and after compositor flush — not via polling.
   */
  tryConsumePending() {
    this._tryStartPending();
  }

  _canStartNow() {
    if (!this._deps) return false;
    if (this.isBusy() || this._inFlight) return false;
    if (typeof this._deps.isRenderBusy === 'function' && this._deps.isRenderBusy()) return false;
    if (typeof this._deps.isDashboardLoading === 'function' && this._deps.isDashboardLoading()) {
      return false;
    }
    return true;
  }

  _tryStartPending() {
    if (!this._deps) return;
    if (!this._canStartNow()) return;

    if (this._pendingLeftIntent && this._pendingRightIntent
        && typeof this._deps.pickHistoryPrefetchEdge === 'function') {
      const range = (typeof this._deps.getVisibleRange === 'function'
        ? this._deps.getVisibleRange()
        : null) || this._pendingLeftIntent.range;
      const edge = this._deps.pickHistoryPrefetchEdge(range);
      if (edge === 'right') {
        this._tryStartRightPending();
        return;
      }
      if (edge === 'left') {
        this._tryStartLeftPending();
        return;
      }
    }
    if (this._pendingLeftIntent) {
      this._tryStartLeftPending();
      return;
    }
    if (this._pendingRightIntent) {
      this._tryStartRightPending();
    }
  }

  _tryStartLeftPending() {
    if (!this._deps || !this._pendingLeftIntent) return;
    if (!this._canStartNow()) return;
    if (this.isLeftHeadBlocked()) {
      this._pendingLeftIntent = null;
      return;
    }

    const pending = this._pendingLeftIntent;
    const options = pending.options || {};
    const liveRange = (typeof this._deps.getVisibleRange === 'function'
      ? this._deps.getVisibleRange()
      : null) || pending.range;

    // Post-prepend continuation: current VIEW decides (tip return cancels).
    if (options.continuation === true) {
      if (!this._deps.shouldLoad(liveRange, options)) {
        this._pendingLeftIntent = null;
        return;
      }
      this._pendingLeftIntent = null;
      this.schedulePrepend(liveRange, options);
      return;
    }

    const liveOk = this._deps.shouldLoad(liveRange, options);
    if (liveOk) {
      this._pendingLeftIntent = null;
      this.schedulePrepend(liveRange, options);
      return;
    }

    // Preserve remaps logical from upward (≈ from + addedBars). That must not
    // cancel a remembered left-void need that was valid when noted.
    const pendingOk = this._deps.shouldLoad(pending.range, options);
    if (pendingOk && this._looksLikePreserveRemap(liveRange, pending.range)) {
      this._pendingLeftIntent = null;
      this.schedulePrepend(pending.range, options);
      return;
    }

    // Need no longer valid — explicit cancel (not busy-drop).
    this._pendingLeftIntent = null;
  }

  _tryStartRightPending() {
    if (!this._deps || !this._pendingRightIntent) return;
    if (!this._canStartNow()) return;
    if (this.isRightTipBlocked()) {
      this._pendingRightIntent = null;
      return;
    }
    if (typeof this._deps.shouldLoadRight !== 'function') {
      this._pendingRightIntent = null;
      return;
    }

    const pending = this._pendingRightIntent;
    const options = pending.options || {};
    const liveRange = (typeof this._deps.getVisibleRange === 'function'
      ? this._deps.getVisibleRange()
      : null) || pending.range;

    if (options.continuation === true) {
      if (!this._deps.shouldLoadRight(liveRange, options)) {
        this._pendingRightIntent = null;
        return;
      }
      this._pendingRightIntent = null;
      this.scheduleAppend(liveRange, options);
      return;
    }

    if (this._deps.shouldLoadRight(liveRange, options)) {
      this._pendingRightIntent = null;
      this.scheduleAppend(liveRange, options);
      return;
    }

    this._pendingRightIntent = null;
  }

  /**
   * True when live.from jumped above pending.from in the way preserve remaps indices.
   * @param {{ from: number, to: number }} liveRange
   * @param {{ from: number, to: number }} pendingRange
   */
  _looksLikePreserveRemap(liveRange, pendingRange) {
    if (!liveRange || !pendingRange) return false;
    const liveFrom = Number(liveRange.from);
    const pendingFrom = Number(pendingRange.from);
    if (!Number.isFinite(liveFrom) || !Number.isFinite(pendingFrom)) return false;
    return liveFrom > pendingFrom;
  }

  /**
   * After a successful left-history prepend: if VIEW still needs older history
   * and hasMore, re-note via Wave 2 pending (no timers / poll / second owner).
   * Continuation policy lives in Boot (`shouldContinueLeftHistory`); without it,
   * Hydration does not invent a second detector.
   */
  _noteContinuationIfNeeded() {
    const deps = this._deps;
    if (!deps) return;
    if (typeof deps.shouldContinueLeftHistory !== 'function') return;
    if (typeof deps.getHistoryHasMore === 'function' && deps.getHistoryHasMore() === false) {
      return;
    }

    const liveRange = typeof deps.getVisibleRange === 'function'
      ? deps.getVisibleRange()
      : null;
    if (!deps.shouldContinueLeftHistory(liveRange)) return;

    const noteRange = liveRange
      && Number.isFinite(liveRange.from)
      && Number.isFinite(liveRange.to)
      ? { from: liveRange.from, to: liveRange.to }
      : { from: 0, to: 50 };

    this.noteLeftHistoryIntent(noteRange, { continuation: true, force: true });
  }

  _noteRightContinuationIfNeeded() {
    const deps = this._deps;
    if (!deps) return;
    if (typeof deps.shouldContinueRightHistory !== 'function') return;

    const liveRange = typeof deps.getVisibleRange === 'function'
      ? deps.getVisibleRange()
      : null;
    if (!deps.shouldContinueRightHistory(liveRange)) return;

    const noteRange = liveRange
      && Number.isFinite(liveRange.from)
      && Number.isFinite(liveRange.to)
      ? { from: liveRange.from, to: liveRange.to }
      : { from: 0, to: 50 };

    this.noteRightHistoryIntent(noteRange, { continuation: true, force: true });
  }

  schedulePrepend(range, options = {}) {
    if (!this._deps) return;
    if (!this._canStartNow()) {
      // Supersede into single pending intent (not a queue).
      if (range && Number.isFinite(range.from) && Number.isFinite(range.to)) {
        this._pendingLeftIntent = {
          range: { from: range.from, to: range.to },
          options: { ...options },
        };
      }
      return;
    }
    if (options.force === true) {
      if (this.debounceTimer) {
        clearTimeout(this.debounceTimer);
        this.debounceTimer = null;
      }
      this.requestPrepend(range, options);
      return;
    }
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
    }
    this.debounceTimer = setTimeout(() => {
      this.debounceTimer = null;
      this.requestPrepend(range, options);
    }, this.debounceMs);
  }

  scheduleAppend(range, options = {}) {
    if (!this._deps) return;
    if (!this._canStartNow()) {
      if (range && Number.isFinite(range.from) && Number.isFinite(range.to)) {
        this._pendingRightIntent = {
          range: { from: range.from, to: range.to },
          options: { ...options },
        };
      }
      return;
    }
    if (options.force === true) {
      if (this.debounceTimer) {
        clearTimeout(this.debounceTimer);
        this.debounceTimer = null;
      }
      this.requestAppend(range, options);
      return;
    }
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
    }
    this.debounceTimer = setTimeout(() => {
      this.debounceTimer = null;
      this.requestAppend(range, options);
    }, this.debounceMs);
  }

  async requestPrepend(range, options = {}) {
    const deps = this._deps;
    if (!deps) return;
    if (this._inFlight) {
      if (this._hydrateEdge === 'right') return;
      if (range && Number.isFinite(range.from) && Number.isFinite(range.to)) {
        this._pendingLeftIntent = {
          range: { from: range.from, to: range.to },
          options: { ...options },
        };
      }
      return;
    }
    if (!this._canStartNow()) {
      if (range && Number.isFinite(range.from) && Number.isFinite(range.to)) {
        this._pendingLeftIntent = {
          range: { from: range.from, to: range.to },
          options: { ...options },
        };
      }
      return;
    }
    if (!deps.shouldLoad(range, options)) return;

    const epoch = deps.getEpoch();
    const reqId = typeof deps.getReqId === 'function' ? deps.getReqId() : null;
    const endTimeSec = deps.getAnchorEndTimeSec();
    if (!Number.isFinite(endTimeSec) || endTimeSec <= 0) return;
    if (this.isLeftHeadBlocked()) return;

    this._inFlight = true;
    this._hydrateEdge = 'left';
    this.state = HydrationState.PREPENDING;
    let completed = false;

    try {
      const data = await deps.fetchColumnar(endTimeSec);
      if (epoch !== deps.getEpoch()) return;
      if (reqId != null && typeof deps.getReqId === 'function' && reqId !== deps.getReqId()) return;

      // Wave 3: empty payload is recoverable unless server authoritatively asserts EOF.
      if (!data || !Array.isArray(data.times) || data.times.length === 0) {
        this._applyAuthoritativeEof(data);
        if (!(data && data.hasMore === false)) {
          this._zeroProgressLeftHeadSec = endTimeSec;
          this._pendingLeftIntent = null;
        }
        return;
      }

      this.state = HydrationState.APPLYING;

      if (typeof deps.setLoadingHistory === 'function') deps.setLoadingHistory(true);
      if (typeof deps.sealStore === 'function') deps.sealStore();

      try {
        const viewportRange = typeof ChartAdapter !== 'undefined'
          ? ChartAdapter.getVisibleLogicalRange('live')
          : null;

        const mergeResult = deps.mergeIntoStore(data);
        // Wave 3: zero-overlap / no-add is recoverable — never infer EOF.
        if (!mergeResult || mergeResult.added <= 0) {
          console.warn('[HydrationOrchestrator] prepend stalled: zero overlap (recoverable, not EOF)');
          this._zeroProgressLeftHeadSec = endTimeSec;
          this._pendingLeftIntent = null;
          return;
        }

        this._zeroProgressLeftHeadSec = null;

        const addedBars = Number(mergeResult.added) > 0
          ? Number(mergeResult.added)
          : (Number.isFinite(data.added) && data.added > 0 ? data.added : 0);


        if (typeof deps.markDirty === 'function') {
          const tipBefore = mergeResult.tipBefore ?? null;
          const tipAfter = mergeResult.tipAfter ?? null;
          const tipBeforeN = Number(tipBefore);
          const tipAfterN = Number(tipAfter);
          const rightBoundaryChanged = Number.isFinite(tipBeforeN)
            && Number.isFinite(tipAfterN)
            && tipBeforeN !== tipAfterN;
          const intent = {
            mode: 'prepend',
            edge: 'left',
            addedBars,
            viewportRange: mergeResult.viewportRange ?? viewportRange,
            viewportAnchor: mergeResult.viewportAnchor ?? null,
            storeBefore: mergeResult.storeBefore ?? null,
            storeAfter: mergeResult.storeAfter ?? null,
            prependedCount: mergeResult.prependedCount ?? addedBars,
            prunedRightCount: mergeResult.prunedRightCount ?? 0,
            tipBefore,
            tipAfter,
            // Camera contract fact (NOT a translation): tip moved in this LEFT merge.
            rightBoundaryChanged,
          };
          deps.markDirty(intent);
        }

        // Wave 3: historyHasMore reflects EOF only after successful merge.
        this._applyCompletionHasMore(data);
        if (typeof deps.onAfterPrepend === 'function') {
          deps.onAfterPrepend(mergeResult, addedBars);
        }
        completed = true;
      } finally {
        if (typeof deps.unsealStore === 'function') deps.unsealStore();
        if (typeof deps.setLoadingHistory === 'function') deps.setLoadingHistory(false);
      }
    } catch (err) {
      // Wave 3: errors are recoverable — must not masquerade as EOF.
      console.error('[HydrationOrchestrator] prepend failed:', err);
    } finally {
      if (completed) this._lastHydrateEdge = 'left';
      this._hydrateEdge = null;
      this._inFlight = false;
      if (completed) {
        this.state = HydrationState.LIVE;
        this.flushQueue();
        // Re-note left-history need after success so preserve-remapped `from`
        // cannot convert "still need history" into "no need".
        this._noteContinuationIfNeeded();
      } else {
        this.wsQueue = [];
        this.state = HydrationState.IDLE;
      }
      // Wave 2: leaving busy → naturally consume pending (no poll/timer retry loop).
      // Wave 3: true EOF clears pending inside _applyCompletionHasMore / _applyAuthoritativeEof.
      this.tryConsumePending();
    }
  }

  async requestAppend(range, options = {}) {
    const deps = this._deps;
    if (!deps) return;
    if (typeof deps.shouldLoadRight !== 'function'
      || typeof deps.getRightFetchEndSec !== 'function'
      || typeof deps.mergeAppendIntoStore !== 'function') {
      return;
    }
    if (this._inFlight) {
      if (this._hydrateEdge === 'left') return;
      if (range && Number.isFinite(range.from) && Number.isFinite(range.to)) {
        this._pendingRightIntent = {
          range: { from: range.from, to: range.to },
          options: { ...options },
        };
      }
      return;
    }
    if (!this._canStartNow()) {
      if (range && Number.isFinite(range.from) && Number.isFinite(range.to)) {
        this._pendingRightIntent = {
          range: { from: range.from, to: range.to },
          options: { ...options },
        };
      }
      return;
    }
    if (!deps.shouldLoadRight(range, options)) return;
    if (this.isRightTipBlocked()) return;

    const epoch = deps.getEpoch();
    const reqId = typeof deps.getReqId === 'function' ? deps.getReqId() : null;
    const endTimeSec = deps.getRightFetchEndSec();
    if (!Number.isFinite(endTimeSec) || endTimeSec <= 0) {
      this._pendingRightIntent = null;
      return;
    }
    const tipSec = this._currentRightTipSec();

    this._inFlight = true;
    this._hydrateEdge = 'right';
    this.state = HydrationState.PREPENDING;
    let completed = false;

    try {
      const fetchRight = typeof deps.fetchRightColumnar === 'function'
        ? deps.fetchRightColumnar
        : deps.fetchColumnar;
      const data = await fetchRight(endTimeSec);
      if (epoch !== deps.getEpoch()) return;
      if (reqId != null && typeof deps.getReqId === 'function' && reqId !== deps.getReqId()) return;

      if (!data || !Array.isArray(data.times) || data.times.length === 0) {
        // Empty right payload → stop right pending (no left EOF inference).
        this._pendingRightIntent = null;
        if (tipSec != null) this._zeroProgressRightTipSec = tipSec;
        return;
      }

      this.state = HydrationState.APPLYING;

      if (typeof deps.setLoadingHistory === 'function') deps.setLoadingHistory(true);
      if (typeof deps.sealStore === 'function') deps.sealStore();

      try {
        const viewportRange = typeof ChartAdapter !== 'undefined'
          ? ChartAdapter.getVisibleLogicalRange('live')
          : null;

        const mergeResult = deps.mergeAppendIntoStore(data);
        // Durable RIGHT progress = store tip advanced. `added` is insert count
        // before capacity prune and must not by itself mean success (Fix G).
        const afterTipSec = this._currentRightTipSec();
        const tipAdvanced = tipSec != null
          && afterTipSec != null
          && afterTipSec > tipSec;
        if (!mergeResult || mergeResult.added <= 0 || !tipAdvanced) {
          console.warn('[HydrationOrchestrator] append stalled: no newer bars (recoverable)');
          this._pendingRightIntent = null;
          const blockTip = afterTipSec != null ? afterTipSec : tipSec;
          if (blockTip != null) this._zeroProgressRightTipSec = blockTip;
          return;
        }

        this._zeroProgressRightTipSec = null;

        const addedBars = Number(mergeResult.added) > 0 ? Number(mergeResult.added) : 0;


        if (typeof deps.markDirty === 'function') {
          // Same paint path as prepend; edge:'right' → market-time preserve only
          // (do NOT apply +addedBars index shift — right append does not move old indices
          // unless LEFT prune, which ViewportAnchor market-time handles).
          const intent = {
            mode: 'prepend',
            edge: 'right',
            addedBars,
            viewportRange: mergeResult.viewportRange ?? viewportRange,
            viewportAnchor: mergeResult.viewportAnchor ?? null,
          };
          deps.markDirty(intent);
        }

        if (typeof deps.onAfterAppend === 'function') {
          deps.onAfterAppend(mergeResult, addedBars);
        }
        completed = true;
      } finally {
        if (typeof deps.unsealStore === 'function') deps.unsealStore();
        if (typeof deps.setLoadingHistory === 'function') deps.setLoadingHistory(false);
      }
    } catch (err) {
      console.error('[HydrationOrchestrator] append failed:', err);
    } finally {
      if (completed) this._lastHydrateEdge = 'right';
      this._hydrateEdge = null;
      this._inFlight = false;
      if (completed) {
        this.state = HydrationState.LIVE;
        this.flushQueue();
        this._noteRightContinuationIfNeeded();
      } else {
        this.wsQueue = [];
        this.state = HydrationState.IDLE;
      }
      this.tryConsumePending();
    }
  }

  /**
   * Wave 3 — EOF only from authoritative server hasMore === false.
   * Clears pending left intent (protocol complete). Does not infer from empty/overlap/error.
   * @param {object|null|undefined} data
   */
  _applyAuthoritativeEof(data) {
    if (!data || data.hasMore !== false) return;
    const deps = this._deps;
    if (deps?.setHistoryHasMore) deps.setHistoryHasMore(false);
    this._pendingLeftIntent = null;
  }

  /**
   * Wave 3 — after successful merge, publish Continue vs EOF from server hasMore.
   * @param {object} data
   */
  _applyCompletionHasMore(data) {
    const deps = this._deps;
    if (!deps?.setHistoryHasMore) return;
    const eof = data && data.hasMore === false;
    deps.setHistoryHasMore(!eof);
    if (eof) this._pendingLeftIntent = null;
  }
}

if (typeof window !== 'undefined') {
  window.HydrationOrchestrator = HydrationOrchestrator;
  window.HydrationState = HydrationState;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { HydrationOrchestrator, HydrationState };
}
