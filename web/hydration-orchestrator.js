/**
 * HydrationOrchestrator — FSM coordinator for monolithic columnar history prepend.
 * Wave 2: owns a single pending left-history intent (not a queue). Busy delays; never drops.
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
    /**
     * Wave 2 — newest left-history need only (supersede, never accumulate).
     * @type {{ range: { from: number, to: number }, options: object }|null}
     */
    this._pendingLeftIntent = null;
  }

  /**
   * @param {object} deps
   * @param {() => number} deps.getEpoch
   * @param {() => number} [deps.getReqId]
   * @param {(range: object, options?: object) => boolean} deps.shouldLoad
   * @param {() => number|null} deps.getAnchorEndTimeSec
   * @param {() => string[]} [deps.getSlotIds]
   * @param {(endTimeSec: number) => Promise<object>} deps.fetchColumnar
   * @param {(data: object) => { added: number, viewportRange?: object|null }|null} deps.mergeIntoStore
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

  reset() {
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
      this.debounceTimer = null;
    }
    this.wsQueue = [];
    this._pendingLeftIntent = null;
    this.state = HydrationState.IDLE;
    this._inFlight = false;
  }

  /** Drop queued ticks without replay (epoch/TF abort). */
  discardQueue() {
    this.wsQueue = [];
  }

  /**
   * Queue live WS tick while prepend is in flight.
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
    this._pendingLeftIntent = {
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
    if (!this._deps || !this._pendingLeftIntent) return;
    if (!this._canStartNow()) return;

    const liveRange = (typeof this._deps.getVisibleRange === 'function'
      ? this._deps.getVisibleRange()
      : null) || this._pendingLeftIntent.range;
    const options = this._pendingLeftIntent.options || {};

    if (!this._deps.shouldLoad(liveRange, options)) {
      // Need no longer valid — explicit cancel (not busy-drop).
      this._pendingLeftIntent = null;
      return;
    }

    this._pendingLeftIntent = null;
    this.schedulePrepend(liveRange, options);
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

  async requestPrepend(range, options = {}) {
    const deps = this._deps;
    if (!deps) return;
    if (this._inFlight) {
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

    this._inFlight = true;
    this.state = HydrationState.PREPENDING;
    let completed = false;

    try {
      const data = await deps.fetchColumnar(endTimeSec);
      if (epoch !== deps.getEpoch()) return;
      if (reqId != null && typeof deps.getReqId === 'function' && reqId !== deps.getReqId()) return;

      // Wave 3: empty payload is recoverable unless server authoritatively asserts EOF.
      if (!data || !Array.isArray(data.times) || data.times.length === 0) {
        this._applyAuthoritativeEof(data);
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
          return;
        }

        const addedBars = Number(mergeResult.added) > 0
          ? Number(mergeResult.added)
          : (Number.isFinite(data.added) && data.added > 0 ? data.added : 0);

        if (typeof deps.markDirty === 'function') {
          deps.markDirty({
            mode: 'prepend',
            addedBars,
            viewportRange: mergeResult.viewportRange ?? viewportRange,
          });
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
      this._inFlight = false;
      if (completed) {
        this.state = HydrationState.LIVE;
        this.flushQueue();
      } else {
        this.wsQueue = [];
        this.state = HydrationState.IDLE;
      }
      // Wave 2: leaving busy → naturally consume pending (no poll/timer retry loop).
      // Wave 3: true EOF clears pending inside _applyCompletionHasMore / _applyAuthoritativeEof.
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
