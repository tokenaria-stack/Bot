/**
 * RenderScheduler — Core 2.3 epoch loop + priority frames (F1 price, F2 indicators).
 * Modules only markDirty; this class alone drives compositor.flush.
 */
class RenderScheduler {
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

  /** @returns {object|null} */
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
      return {
        mode: 'delta',
        tick: next.tick ?? prev.tick,
        delta: next.delta ?? prev.delta,
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
