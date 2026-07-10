/**
 * HydrationOrchestrator — FSM coordinator for monolithic columnar history prepend.
 * Commands only: fetch → store/DDR merge → ChartAdapter atomic apply.
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
  }

  /**
   * @param {object} deps
   * @param {() => number} deps.getEpoch
   * @param {() => boolean} deps.shouldLoad
   * @param {() => number|null} deps.getAnchorEndTimeSec
   * @param {() => string[]} deps.getSlotIds
   * @param {(endTimeSec: number) => Promise<object>} deps.fetchColumnar
   * @param {(data: object) => { added: number, viewportRange?: object|null }} deps.mergeIntoStore
   * @param {(intent: object) => void} deps.markDirty
   * @param {(tick: object) => void} deps.processTick
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

  reset() {
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
      this.debounceTimer = null;
    }
    this.wsQueue = [];
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

  schedulePrepend(range, options = {}) {
    if (!this._deps) return;
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
    if (!deps || this._inFlight) return;
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
      if (!data || !Array.isArray(data.times) || data.times.length === 0) {
        if (deps.setHistoryHasMore) deps.setHistoryHasMore(false);
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
        if (!mergeResult || mergeResult.added <= 0) {
          if (deps.setHistoryHasMore) deps.setHistoryHasMore(false);
          console.warn('[HydrationOrchestrator] prepend stalled: zero overlap');
          return;
        }

        const addedBars = Number.isFinite(data.added) && data.added > 0
          ? data.added
          : mergeResult.added;

        if (typeof deps.markDirty === 'function') {
          deps.markDirty({
            mode: 'prepend',
            addedBars,
            viewportRange: mergeResult.viewportRange ?? viewportRange,
          });
        }

        if (deps.setHistoryHasMore) {
          deps.setHistoryHasMore(data.hasMore !== false);
        }
        if (typeof deps.onAfterPrepend === 'function') {
          deps.onAfterPrepend(mergeResult, addedBars);
        }
        completed = true;

        if (deps.getHistoryHasMore?.() !== false && epoch === deps.getEpoch()) {
          const threshold = (typeof CONFIG !== 'undefined' && CONFIG.LIVE_HISTORY_SCROLL_THRESHOLD) || 50;
          const finalRange = typeof ChartAdapter !== 'undefined'
            ? ChartAdapter.getVisibleLogicalRange('live')
            : null;
          if (finalRange && finalRange.from < threshold) {
            this.schedulePrepend(finalRange);
          }
        }
      } finally {
        if (typeof deps.unsealStore === 'function') deps.unsealStore();
        if (typeof deps.setLoadingHistory === 'function') deps.setLoadingHistory(false);
      }
    } catch (err) {
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
    }
  }
}

if (typeof window !== 'undefined') {
  window.HydrationOrchestrator = HydrationOrchestrator;
  window.HydrationState = HydrationState;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { HydrationOrchestrator, HydrationState };
}
