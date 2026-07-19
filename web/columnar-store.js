/**
 * ColumnarStore — SSOT for live columnar history (Core 3.0).
 * Mirrors server wire JSON; annotations indexed by snapped ms for O(1) marker patches.
 *
 * Debt #69A: bounded display cache (not a historical DB). Server owns durable history.
 */
class ColumnarStore {
  static get BUDGET_TARGET() {
    return (typeof STORE_BUDGET_TARGET !== 'undefined' && Number.isFinite(STORE_BUDGET_TARGET))
      ? STORE_BUDGET_TARGET
      : 12000;
  }

  static get BUDGET_HARD_CAP() {
    return (typeof STORE_BUDGET_HARD_CAP !== 'undefined' && Number.isFinite(STORE_BUDGET_HARD_CAP))
      ? STORE_BUDGET_HARD_CAP
      : 16000;
  }

  static get PRUNE_FROM_OLDEST() { return 'oldest'; }
  static get PRUNE_FROM_NEWEST() { return 'newest'; }

  constructor() {
    this._times = [];
    this._candles = { open: [], high: [], low: [], close: [], volume: [] };
    this._plots = {};
    /** @type {object[]} wire-format annotations for full paint */
    this._annotations = [];
    /** @type {Map<number, object>} snappedMs → { spikeUp, spikeDown, volCross, rsxLabel, ... } */
    this._annotationMap = new Map();
    this._meta = { hasMore: false, tf: '', warmupDropped: 0, added: 0 };
    this._sealed = false;
    /** @type {number} TF bar duration (sec); 0 = gap detection disabled. */
    this._intervalSec = 0;
    /**
     * Display-window mode (Debt #69A).
     * 'live' — store tip tracks the market; WS may appendTick.
     * 'history' — user exploring past (e.g. after FROM_NEWEST prune); WS must not feed store.
     * @type {'live'|'history'}
     */
    this.windowMode = 'live';
  }

  /** Core 4.5: bind current TF interval so appendTick can detect chronology gaps. */
  setTfInterval(seconds) {
    const s = Number(seconds);
    this._intervalSec = Number.isFinite(s) && s > 0 ? s : 0;
  }

  /** Same sec/ms normalization as appendTick (chartTime), safe under Node tests. */
  static _normTimeSec(raw) {
    if (typeof chartTime === 'function') return chartTime(raw);
    const t = Number(raw);
    if (!Number.isFinite(t)) return null;
    return t >= 1e12 ? Math.floor(t / 1000) : Math.floor(t);
  }

  static _toMs(timeLike) {
    const n = Number(timeLike);
    if (!Number.isFinite(n) || n <= 0) return null;
    return n < 1e12 ? Math.floor(n * 1000) : Math.floor(n);
  }

  static _msToSec(ms) {
    return Math.floor(ms / 1000);
  }

  static plotAbsent() {
    return typeof DDRFactory !== 'undefined' ? DDRFactory.HISTORY_ABSENT : Number.NaN;
  }

  clear() {
    this._times = [];
    this._candles = { open: [], high: [], low: [], close: [], volume: [] };
    this._plots = {};
    this._annotations = [];
    this._annotationMap.clear();
    this._meta = { hasMore: false, tf: '', warmupDropped: 0, added: 0 };
    this._sealed = false;
    this.windowMode = 'live';
  }

  seal() {
    this._sealed = true;
  }

  unseal() {
    this._sealed = false;
  }

  isSealed() {
    return this._sealed;
  }

  replaceMonolith(columnarJson) {
    const data = columnarJson && typeof columnarJson === 'object' ? columnarJson : {};
    // Core 4.5: normalize sec/ms exactly like appendTick — history and ticks share one time axis.
    // map (not filter): keeps index alignment with candle columns; invariantOk stays honest.
    const times = Array.isArray(data.times)
      ? data.times.map((t) => ColumnarStore._normTimeSec(t))
      : [];
    const src = data.candles && typeof data.candles === 'object' ? data.candles : {};
    this._times = times;
    this._candles = {
      open: Array.isArray(src.open) ? src.open.slice() : [],
      high: Array.isArray(src.high) ? src.high.slice() : [],
      low: Array.isArray(src.low) ? src.low.slice() : [],
      close: Array.isArray(src.close) ? src.close.slice() : [],
      volume: Array.isArray(src.volume) ? src.volume.slice() : [],
    };
    this._plots = {};
    const plots = data.plots && typeof data.plots === 'object' ? data.plots : {};
    for (const [id, col] of Object.entries(plots)) {
      this._plots[id] = Array.isArray(col) ? col.slice() : [];
    }
    this._annotations = Array.isArray(data.annotations) ? data.annotations.slice() : [];
    this._rebuildAnnotationMapFromArray(this._annotations);
    this._meta = {
      hasMore: data.hasMore === true,
      tf: data.timeframe || '',
      warmupDropped: Number(data.warmupDropped) || 0,
      added: Number(data.added) || times.length,
    };
    this.windowMode = 'live';
    this._enforceBudget(ColumnarStore.PRUNE_FROM_OLDEST);
  }

  /**
   * Soft update: replace/add plot columns only. Never mutates _times or _candles.
   * Arrays are padded/truncated to current barCount for invariant safety.
   * @param {Record<string, number[]>} newPlots
   */
  updatePlots(newPlots) {
    if (!newPlots || typeof newPlots !== 'object') return;
    const n = this._times.length;
    const absent = ColumnarStore.plotAbsent();
    for (const [id, col] of Object.entries(newPlots)) {
      if (!Array.isArray(col)) continue;
      const next = col.slice(0, n);
      while (next.length < n) next.push(absent);
      this._plots[id] = next;
    }
  }

  mergeAnnotations(annotations) {
    if (!Array.isArray(annotations)) return;
    this._annotations = annotations.slice();
    this._rebuildAnnotationMapFromArray(this._annotations);
  }

  _rebuildAnnotationMapFromArray(annotations) {
    this._annotationMap.clear();
    for (const ann of annotations || []) {
      const ms = ColumnarStore._toMs(ann?.time ?? ann?.Time);
      if (!ms) continue;
      const props = ColumnarStore._propsFromWireAnnotation(ann, ms);
      if (props) this._annotationMap.set(ms, props);
    }
  }

  static _propsFromWireAnnotation(ann, ms) {
    const text = String(ann?.text ?? ann?.label ?? ann?.Label ?? '').trim();
    const label = text.substring(0, 2).toUpperCase();
    // Phase F: do not store purged RSX trading labels.
    if (['S', 'SS', 'L', 'LL', 'P'].includes(label)) {
      return null;
    }
    const props = { timeMs: ms };
    if (text) props.rsxLabel = label;
    if (ann?.pane) props.pane = ann.pane;
    if (ann?.color) props.color = ann.color;
    if (ann?.position) props.position = ann.position;
    if (ann?.shape) props.shape = ann.shape;
    if (ann?.spikeUp) props.spikeUp = true;
    if (ann?.spikeDown) props.spikeDown = true;
    if (ann?.volCross) props.volCross = ann.volCross;
    return props;
  }

  static _propsFromTick(tick) {
    if (!tick || typeof tick !== 'object') return null;
    const props = {};
    if (tick.volumeSpikeUp || tick.VolumeSpikeUp) props.spikeUp = true;
    if (tick.volumeSpikeDown || tick.VolumeSpikeDown) props.spikeDown = true;
    const volCross = tick.volCrossMarker ?? tick.VolCrossMarker;
    if (volCross) props.volCross = volCross;
    // Phase F: tick.marker L/LL/S/SS no longer published to the chart store.
    return Object.keys(props).length ? props : null;
  }

  _mergeAnnotationProps(ms, incoming) {
    if (!incoming || !Object.keys(incoming).length) {
      return this._annotationMap.get(ms) || null;
    }
    const existing = this._annotationMap.get(ms) || { timeMs: ms };
    const merged = { ...existing, ...incoming, timeMs: ms };
    if (existing.spikeUp || incoming.spikeUp) merged.spikeUp = true;
    if (existing.spikeDown || incoming.spikeDown) merged.spikeDown = true;
    this._annotationMap.set(ms, merged);
    return merged;
  }

  _ingestTickMarkers(tick, timeSec) {
    const props = ColumnarStore._propsFromTick(tick);
    if (!props) return null;
    const ms = ColumnarStore._toMs(timeSec);
    if (!ms) return null;
    return this._mergeAnnotationProps(ms, props);
  }

  getAnnotationsMap() {
    return this._annotationMap;
  }

  /** Viewport / navigator compatibility shim (replaces ChartDataStore for live). */
  getForLightweightCharts() {
    const columnar = { times: this._times, candles: this._candles };
    const candles = typeof columnarToCandles === 'function'
      ? columnarToCandles(columnar)
      : [];
    return {
      candles,
      osc: [],
      annotations: this._annotations.slice(),
      annotationMap: this._annotationMap,
    };
  }

  getSimOverlayPayload() {
    return this.getForLightweightCharts();
  }

  barCount() {
    return this._times.length;
  }

  candleCount() {
    return this.barCount();
  }

  firstCandleTimeSec() {
    return this.firstTimeSec();
  }

  lastCandleTimeSec() {
    return this.lastTimeSec();
  }

  lastCandleChartSec() {
    const t = this.lastTimeSec();
    return t == null ? null : { time: t };
  }

  candlesArray() {
    return this._times.map((t) => {
      const sec = Number(t);
      return { timeMs: sec * 1000, time: sec };
    });
  }

  firstTimeSec() {
    return this._times.length > 0 ? Number(this._times[0]) : null;
  }

  lastTimeSec() {
    return this._times.length > 0 ? Number(this._times[this._times.length - 1]) : null;
  }

  /** Computed window bounds (Debt #69A) — no duplicated mutable start/end fields. */
  windowStartSec() {
    return this.firstTimeSec();
  }

  windowEndSec() {
    return this.lastTimeSec();
  }

  isLiveWindow() {
    return this.windowMode !== 'history';
  }

  /**
   * Atomic bar-window prune (Debt #69A). Same [start, end) for times, candles.*, every plot, annotations.
   * @param {number} keepCount
   * @param {'oldest'|'newest'} direction
   */
  _pruneToCount(keepCount, direction) {
    const n = this._times.length;
    const keep = Math.max(0, Math.floor(Number(keepCount)) || 0);
    if (n <= keep) return;
    const drop = n - keep;
    let start;
    let end;
    if (direction === ColumnarStore.PRUNE_FROM_NEWEST) {
      start = 0;
      end = keep;
    } else {
      start = drop;
      end = n;
    }

    this._times = this._times.slice(start, end);
    this._candles = {
      open: this._candles.open.slice(start, end),
      high: this._candles.high.slice(start, end),
      low: this._candles.low.slice(start, end),
      close: this._candles.close.slice(start, end),
      volume: this._candles.volume.slice(start, end),
    };
    const nextPlots = {};
    for (const [id, col] of Object.entries(this._plots)) {
      nextPlots[id] = Array.isArray(col) ? col.slice(start, end) : [];
    }
    this._plots = nextPlots;

    if (this._times.length === 0) {
      this._annotations = [];
      this._annotationMap.clear();
    } else {
      const t0 = Number(this._times[0]);
      const t1 = Number(this._times[this._times.length - 1]);
      this._annotations = this._annotations.filter((ann) => {
        const t = Number(ann?.time ?? ann?.Time);
        return Number.isFinite(t) && t >= t0 && t <= t1;
      });
      this._rebuildAnnotationMapFromArray(this._annotations);
    }

    if (direction === ColumnarStore.PRUNE_FROM_NEWEST) {
      this.windowMode = 'history';
    }
    this._meta = { ...this._meta, added: this._times.length };
  }

  /**
   * @param {'oldest'|'newest'} direction
   */
  _enforceBudget(direction) {
    if (this._times.length <= ColumnarStore.BUDGET_HARD_CAP) return;
    this._pruneToCount(ColumnarStore.BUDGET_TARGET, direction);
  }

  /**
   * Debt #69C: pick prune side farthest from the user's focal time.
   * Drop OLDEST when focal is nearer the right (live) edge; drop NEWEST when nearer the left.
   *
   * @param {number|null|undefined} windowStartSec
   * @param {number|null|undefined} windowEndSec
   * @param {number|null|undefined} focalTimeSec
   * @param {{ atLiveEdge?: boolean, defaultDirection?: 'oldest'|'newest' }} [opts]
   * @returns {'oldest'|'newest'}
   */
  static pruneDirectionFromFocal(windowStartSec, windowEndSec, focalTimeSec, opts = {}) {
    if (opts.atLiveEdge === true) {
      return ColumnarStore.PRUNE_FROM_OLDEST;
    }
    const fallback = opts.defaultDirection === ColumnarStore.PRUNE_FROM_OLDEST
      ? ColumnarStore.PRUNE_FROM_OLDEST
      : ColumnarStore.PRUNE_FROM_NEWEST;
    const start = Number(windowStartSec);
    const end = Number(windowEndSec);
    const focal = Number(focalTimeSec);
    if (![start, end, focal].every(Number.isFinite) || end <= start) {
      return fallback;
    }
    // Clamp focal into window for distance math (off-window scroll still chooses nearest edge).
    const f = Math.min(end, Math.max(start, focal));
    const distLeft = f - start;
    const distRight = end - f;
    // Farthest side from focal gets dropped.
    return distLeft <= distRight
      ? ColumnarStore.PRUNE_FROM_NEWEST
      : ColumnarStore.PRUNE_FROM_OLDEST;
  }

  /**
   * Resolve budget prune direction for the current window + optional viewport focal.
   * @param {{ focalTimeSec?: number|null, atLiveEdge?: boolean, defaultDirection?: 'oldest'|'newest', pruneDirection?: 'oldest'|'newest' }} [opts]
   */
  resolveBudgetPruneDirection(opts = {}) {
    if (opts.pruneDirection === ColumnarStore.PRUNE_FROM_OLDEST
      || opts.pruneDirection === ColumnarStore.PRUNE_FROM_NEWEST) {
      return opts.pruneDirection;
    }
    return ColumnarStore.pruneDirectionFromFocal(
      this.windowStartSec(),
      this.windowEndSec(),
      opts.focalTimeSec,
      {
        atLiveEdge: opts.atLiveEdge === true,
        defaultDirection: opts.defaultDirection,
      },
    );
  }

  /**
   * Apply live WS tick to tail bar (update) or append new bar.
   * @returns {{ candle: object, isNewBar: boolean, barCount: number, tick: object, delta: object }|null}
   */
  appendTick(tick) {
    if (this._sealed || !tick || typeof tick !== 'object') return null;
    const time = typeof chartTime === 'function' ? chartTime(tick.time) : null;
    if (time == null) return null;

    const open = Number(tick.open);
    const high = Number(tick.high);
    const low = Number(tick.low);
    const close = Number(tick.close);
    const volume = Number(tick.volume);
    if (![open, high, low, close].every(Number.isFinite)) return null;

    const n = this._times.length;
    const lastTime = n > 0 ? this._times[n - 1] : null;
    if (lastTime != null && time < lastTime) return null;

    const isNewBar = lastTime == null || time > lastTime;

    // Core 4.5 Self-Healing: a forward jump beyond 1.5 intervals is a chronology gap.
    // The store never glues over holes — it reports the gap; composition root decides how to heal.
    if (isNewBar && this._intervalSec > 0 && lastTime != null
      && (time - lastTime) > this._intervalSec * 1.5) {
      return { gapDetected: true, lastTime, tickTime: time };
    }

    const plots = tick.plots && typeof tick.plots === 'object' ? tick.plots : null;
    const absent = ColumnarStore.plotAbsent();

    if (!isNewBar && lastTime === time) {
      const i = n - 1;
      this._candles.open[i] = open;
      this._candles.high[i] = high;
      this._candles.low[i] = low;
      this._candles.close[i] = close;
      if (Number.isFinite(volume)) this._candles.volume[i] = volume;
      if (plots) {
        for (const [id, raw] of Object.entries(plots)) {
          if (!this._plots[id]) this._plots[id] = new Array(n).fill(absent);
          this._plots[id][i] = Number(raw);
        }
      }
    } else {
      this._times.push(time);
      this._candles.open.push(open);
      this._candles.high.push(high);
      this._candles.low.push(low);
      this._candles.close.push(close);
      this._candles.volume.push(Number.isFinite(volume) ? volume : 0);
      const newN = this._times.length;
      const plotIds = new Set(Object.keys(this._plots));
      if (plots) Object.keys(plots).forEach((id) => plotIds.add(id));
      for (const id of plotIds) {
        const col = this._plots[id] || [];
        while (col.length < newN - 1) col.push(absent);
        const raw = plots?.[id];
        col.push(plots && raw !== undefined ? Number(raw) : absent);
        this._plots[id] = col;
      }
    }

    const candle = {
      time,
      open,
      high,
      low,
      close,
      volume: Number.isFinite(volume) ? volume : this._candles.volume[this._candles.volume.length - 1],
    };

    const mergedAnn = this._ingestTickMarkers(tick, time);
    const ms = ColumnarStore._toMs(time);

    const delta = {
      candle,
      isNewBar,
      barCount: this._times.length,
    };
    if (mergedAnn && ms != null) {
      delta.annotationMs = ms;
      delta.annotation = mergedAnn;
    }

    if (isNewBar) {
      this._enforceBudget(ColumnarStore.PRUNE_FROM_OLDEST);
      delta.barCount = this._times.length;
    }

    return { candle, isNewBar, barCount: this._times.length, tick, delta };
  }

  prependMonolith(columnarJson, options = {}) {
    const data = columnarJson && typeof columnarJson === 'object' ? columnarJson : {};
    const incomingTimes = Array.isArray(data.times) ? data.times : [];
    if (incomingTimes.length === 0) return { added: 0 };

    const anchorTime = this._times.length > 0 ? this._times[0] : null;
    const existing = new Set(this._times);
    const src = data.candles && typeof data.candles === 'object' ? data.candles : {};
    const incomingPlots = data.plots && typeof data.plots === 'object' ? data.plots : {};

    const indices = [];
    for (let i = 0; i < incomingTimes.length; i++) {
      const t = incomingTimes[i];
      if (anchorTime != null && t >= anchorTime) continue;
      if (existing.has(t)) continue;
      indices.push(i);
    }

    if (indices.length === 0) return { added: 0 };

    let useIndices = indices;
    if (Number.isFinite(data.added) && data.added > 0 && data.added < indices.length) {
      useIndices = indices.slice(indices.length - data.added);
    }
    const added = useIndices.length;

    const prependTimes = new Array(added);
    const prepended = {
      open: new Array(added),
      high: new Array(added),
      low: new Array(added),
      close: new Array(added),
      volume: new Array(added),
    };
    for (let j = 0; j < added; j++) {
      const i = useIndices[j];
      prependTimes[j] = incomingTimes[i];
      prepended.open[j] = src.open?.[i];
      prepended.high[j] = src.high?.[i];
      prepended.low[j] = src.low?.[i];
      prepended.close[j] = src.close?.[i];
      prepended.volume[j] = src.volume?.[i];
    }

    const plotIds = new Set([...Object.keys(this._plots), ...Object.keys(incomingPlots)]);
    const mergedPlots = {};
    for (const id of plotIds) {
      const cur = this._plots[id] || [];
      const inc = incomingPlots[id] || [];
      const head = new Array(added);
      for (let j = 0; j < added; j++) {
        head[j] = inc[useIndices[j]];
      }
      mergedPlots[id] = head.concat(cur);
    }

    const prependAnnTimes = new Set(prependTimes);
    const keptAnn = this._annotations.filter((ann) => {
      const t = Number(ann?.time ?? ann?.Time);
      return Number.isFinite(t) && !prependAnnTimes.has(t);
    });
    const incomingAnns = Array.isArray(data.annotations) ? data.annotations : [];
    const newAnns = incomingAnns.filter((ann) => {
      const t = Number(ann?.time ?? ann?.Time);
      return Number.isFinite(t) && prependAnnTimes.has(t);
    });

    this._times = prependTimes.concat(this._times);
    this._candles = {
      open: prepended.open.concat(this._candles.open),
      high: prepended.high.concat(this._candles.high),
      low: prepended.low.concat(this._candles.low),
      close: prepended.close.concat(this._candles.close),
      volume: prepended.volume.concat(this._candles.volume),
    };
    this._plots = mergedPlots;
    this._annotations = newAnns.concat(keptAnn);
    this._rebuildAnnotationMapFromArray(this._annotations);
    this._meta = {
      ...this._meta,
      hasMore: data.hasMore === true,
      added: this._times.length,
    };

    // Debt #69C: direction from viewport focal (default NEWEST = safe for left-scroll history).
    const direction = this.resolveBudgetPruneDirection({
      focalTimeSec: options.focalTimeSec,
      atLiveEdge: options.atLiveEdge === true,
      pruneDirection: options.pruneDirection,
      defaultDirection: ColumnarStore.PRUNE_FROM_NEWEST,
    });
    this._enforceBudget(direction);

    return { added, pruneDirection: direction, windowMode: this.windowMode };
  }

  invariantOk() {
    const n = this._times.length;
    if (n === 0) return false;
    const c = this._candles;
    if (c.open.length !== n || c.high.length !== n || c.low.length !== n
      || c.close.length !== n || c.volume.length !== n) {
      return false;
    }
    for (const col of Object.values(this._plots)) {
      if (!Array.isArray(col) || col.length !== n) return false;
    }
    return true;
  }

  invariantMeta() {
    const n = this._times.length;
    const plotLens = {};
    for (const [id, col] of Object.entries(this._plots)) {
      plotLens[id] = Array.isArray(col) ? col.length : -1;
    }
    return {
      times: n,
      candles: {
        open: this._candles.open.length,
        high: this._candles.high.length,
        low: this._candles.low.length,
        close: this._candles.close.length,
        volume: this._candles.volume.length,
      },
      plots: plotLens,
    };
  }

  snapshot() {
    const plots = {};
    for (const [id, col] of Object.entries(this._plots)) {
      plots[id] = col.slice();
    }
    return Object.freeze({
      times: this._times.slice(),
      candles: {
        open: this._candles.open.slice(),
        high: this._candles.high.slice(),
        low: this._candles.low.slice(),
        close: this._candles.close.slice(),
        volume: this._candles.volume.slice(),
      },
      plots,
      annotations: this._annotations.slice(),
      meta: { ...this._meta },
    });
  }
}

if (typeof window !== 'undefined') {
  window.ColumnarStore = ColumnarStore;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { ColumnarStore };
}
