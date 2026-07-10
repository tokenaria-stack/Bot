/**
 * RenderScheduler — RAF coalescer for live chart paint intents (Core 3.0 Step 2).
 */
class RenderScheduler {
  /**
   * @param {{ flush: (intent: object) => void }} compositor
   */
  constructor(compositor) {
    this._compositor = compositor;
    /** @type {object|null} */
    this._pending = null;
    this._rafId = null;
  }

  /**
   * @param {{ mode: 'full'|'prepend'|'delta', addedBars?: number, viewport?: string, viewportRange?: object|null }} intent
   */
  markDirty(intent) {
    if (!intent?.mode) return;
    this._pending = RenderScheduler._coalesce(this._pending, intent);
    if (this._rafId != null) return;
    this._rafId = requestAnimationFrame(() => {
      this._rafId = null;
      const pending = this._pending;
      this._pending = null;
      if (!pending || !this._compositor) return;
      this._compositor.flush(pending);
    });
  }

  /** @returns {object|null} */
  static _coalesce(prev, next) {
    if (!prev) return { ...next };
    if (next.mode === 'full') return { ...next };
    if (prev.mode === 'full') return { ...next };
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
