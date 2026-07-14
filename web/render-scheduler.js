/**
 * RenderScheduler — Core 2.3 epoch loop + priority frames (F1 price, F2 indicators).
 * Shot 11E: NewBar is a boundary event — never coalesced away (delta chain).
 * Modules only markDirty; this class alone drives compositor.flush.
 */
class RenderScheduler {
  /** Soft cap: extreme bursts keep last N boundary+tip slots (still contiguous). */
  static get DELTA_CHAIN_CAP() {
    return 64;
  }

  /**
   * @param {{ flush: (intent: object) => void }} compositor
   */
  constructor(compositor) {
    this._compositor = compositor;
    /** @type {object|null} */
    this._pending = null;
    this._busy = false;
    this._epoch = 0;
  }

  isBusy() {
    return this._busy;
  }

  /**
   * @param {{ mode: 'full'|'prepend'|'delta'|'indicators', phase?: string, addedBars?: number, viewport?: string, viewportRange?: object|null, anchor?: object, tick?: object, delta?: object }} intent
   */
  markDirty(intent) {
    if (!intent?.mode) return;
    this._pending = RenderScheduler._coalesce(this._pending, intent);
    if (!this._busy) this._runLoop();
  }

  _runLoop() {
    if (!this._pending) {
      this._busy = false;
      return;
    }
    if (!this._compositor) {
      this._pending = null;
      this._busy = false;
      return;
    }

    this._busy = true;
    this._epoch += 1;
    const intent = this._pending;
    this._pending = null;

    const mode = intent.mode;
    if (mode === 'full' || mode === 'prepend') {
      requestAnimationFrame(() => {
        this._compositor.flush({ ...intent, phase: 'F1' });
        requestAnimationFrame(() => {
          this._compositor.flush({ ...intent, phase: 'F2' });
          this._busy = false;
          this._runLoop();
        });
      });
      return;
    }

    requestAnimationFrame(() => {
      this._compositor.flush(intent);
      this._busy = false;
      this._runLoop();
    });
  }

  /** Normalize a delta intent into parallel deltas[] / ticks[] chains. */
  static _asDeltaChain(intent) {
    if (!intent || intent.mode !== 'delta') {
      return { deltas: [], ticks: [] };
    }
    if (Array.isArray(intent.deltas) && intent.deltas.length) {
      const ticks = Array.isArray(intent.ticks) ? intent.ticks : [];
      return {
        deltas: intent.deltas.slice(),
        ticks: ticks.length === intent.deltas.length
          ? ticks.slice()
          : intent.deltas.map((_, i) => ticks[i] ?? (i === intent.deltas.length - 1 ? intent.tick : null)),
      };
    }
    if (intent.delta?.candle) {
      return { deltas: [intent.delta], ticks: [intent.tick ?? null] };
    }
    return { deltas: [], ticks: [] };
  }

  static _trimDeltaChain(deltas, ticks) {
    const cap = RenderScheduler.DELTA_CHAIN_CAP;
    if (deltas.length <= cap) return { deltas, ticks };
    return {
      deltas: deltas.slice(deltas.length - cap),
      ticks: ticks.slice(ticks.length - cap),
    };
  }

  /**
   * Shot 11E: coalesce same-bar tips; append on isNewBar (never overwrite a boundary).
   * @returns {object|null}
   */
  static _coalesce(prev, next) {
    if (!prev) return { ...next };
    if (next.mode === 'full') {
      return { ...next, anchor: next.anchor ?? prev.anchor ?? null };
    }
    if (prev.mode === 'full') {
      return { ...prev, anchor: prev.anchor ?? next.anchor ?? null };
    }
    if (next.mode === 'indicators' && prev.mode === 'indicators') return { mode: 'indicators' };
    if (prev.mode === 'indicators' && next.mode === 'delta') return { ...next };
    if (prev.mode === 'delta' && next.mode === 'indicators') return { mode: 'indicators' };
    if (prev.mode === 'delta' && next.mode === 'delta') {
      const { deltas, ticks } = RenderScheduler._asDeltaChain(prev);
      const nextDelta = next.delta;
      const nextTick = next.tick ?? null;
      if (!nextDelta?.candle) {
        return prev;
      }

      const nextIsNewBar = nextDelta.isNewBar === true;
      if (nextIsNewBar || deltas.length === 0) {
        // Boundary (or empty chain): append — closing tip of prior bar stays in chain.
        deltas.push(nextDelta);
        ticks.push(nextTick);
      } else {
        // Same forming bar: rewrite tip only.
        deltas[deltas.length - 1] = nextDelta;
        ticks[ticks.length - 1] = nextTick;
      }

      const trimmed = RenderScheduler._trimDeltaChain(deltas, ticks);
      const tip = trimmed.deltas[trimmed.deltas.length - 1];
      const tipTick = trimmed.ticks[trimmed.ticks.length - 1];
      return {
        mode: 'delta',
        deltas: trimmed.deltas,
        ticks: trimmed.ticks,
        delta: tip,
        tick: tipTick,
      };
    }
    if (prev.mode === 'prepend' && next.mode === 'prepend') {
      return {
        mode: 'prepend',
        addedBars: (Number(prev.addedBars) || 0) + (Number(next.addedBars) || 0),
        viewportRange: next.viewportRange ?? prev.viewportRange ?? null,
        anchor: next.anchor ?? prev.anchor ?? null,
      };
    }
    return { ...next };
  }
}

if (typeof window !== 'undefined') {
  window.RenderScheduler = RenderScheduler;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { RenderScheduler };
}
