/**
 * ColumnarStore — SSOT for live columnar history (Core 3.0).
 * Mirrors server wire JSON; annotations indexed by snapped ms for O(1) marker patches.
 *
 * Debt #69A: bounded display cache (not a historical DB). Server owns durable history.
 * Track A / Working Set: retention may never invalidate the committed VIEW (WS-01…WS-03).
 * Track B Step 1: Mutation Set — same-op growth prune must not discard just-grown bars.
 * Track B Step 2: Retained Neighborhood — Lifetime fact across exploration growth (not Capacity).
 * Track B Step 3: Lazy Contract — exploration events ≠ contraction; RN lifetime only under
 *   pressure (existing budget trigger) or explicit world replacement.
 */
class ColumnarStore {
  static get BUDGET_TARGET() {
    if (typeof MAX_STORE_BARS !== 'undefined' && Number.isFinite(MAX_STORE_BARS) && MAX_STORE_BARS > 0) {
      return MAX_STORE_BARS;
    }
    return (typeof STORE_BUDGET_TARGET !== 'undefined' && Number.isFinite(STORE_BUDGET_TARGET))
      ? STORE_BUDGET_TARGET
      : 9000;
  }

  static get BUDGET_HARD_CAP() {
    // Single hard working-set (= TARGET). RN must not veto this cap.
    return ColumnarStore.BUDGET_TARGET;
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
    /** Dense TFs report chronology gaps; sparse micro (seconds/ticks) never does. */
    this._denseContinuity = true;
    /**
     * Display-window mode (Debt #69A).
     * 'live' — store tip tracks the market; WS may appendTick.
     * 'history' — user exploring past (e.g. after FROM_NEWEST prune); WS must not feed store.
     * @type {'live'|'history'}
     */
    this.windowMode = 'live';
    /**
     * Retained Neighborhood (Track B Step 2–3) — logical time interval [from, to] sec.
     * Eagerly expanded by absorbing Mutation Sets from exploration growth.
     * Track B Step 3 (Lazy Contract): exploration events must never contract RN.
     * Cleared only by explicit world replacement (commit-paired / clear).
     * Eviction under pressure is Capacity-owned (existing budget trigger may drop outside RN).
     * @type {number|null}
     */
    this._rnFromSec = null;
    /** @type {number|null} */
    this._rnToSec = null;
  }

  /** Core 4.5: bind current TF interval so appendTick can detect chronology gaps. */
  setTfInterval(seconds) {
    const s = Number(seconds);
    this._intervalSec = Number.isFinite(s) && s > 0 ? s : 0;
  }

  /** Core 4.5 / MICRO-1.1: native+derived dense; seconds/ticks sparse. */
  setDenseContinuity(dense) {
    this._denseContinuity = dense !== false;
  }

  /** Columnar times[] are Unix seconds (ChartTimeSec). No 1e12 unit guess. */
  static _normTimeSec(raw) {
    if (typeof chartTime === 'function') return chartTime(raw);
    const t = Number(raw);
    if (!Number.isFinite(t)) return null;
    return Math.floor(t);
  }

  static _toMs(timeLike) {
    const n = Number(timeLike);
    if (!Number.isFinite(n) || n <= 0) return null;
    const CDS = typeof ChartDataStore !== 'undefined' ? ChartDataStore : globalThis.ChartDataStore;
    return CDS.secToMs(n);
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
    this._clearRetainedNeighborhood();
  }

  /**
   * Track B Step 2: logical Retained Neighborhood bounds (sec), or null if unset.
   * @returns {{ fromSec: number, toSec: number }|null}
   */
  retainedNeighborhoodBounds() {
    const a = this._rnFromSec;
    const b = this._rnToSec;
    if (a == null || b == null) return null;
    if (![a, b].every(Number.isFinite) || b < a) return null;
    return { fromSec: a, toSec: b };
  }

  _clearRetainedNeighborhood() {
    this._rnFromSec = null;
    this._rnToSec = null;
  }

  /**
   * Absorb a Mutation Set time span into the Retained Neighborhood (eager expand only).
   * Track B Step 3: never shrinks RN — exploration must not contract the neighborhood.
   * @param {number} fromSec
   * @param {number} toSec
   */
  _absorbIntoRetainedNeighborhood(fromSec, toSec) {
    const a = Number(fromSec);
    const b = Number(toSec);
    if (![a, b].every(Number.isFinite) || b < a) return;
    if (this._rnFromSec == null || this._rnToSec == null) {
      this._rnFromSec = a;
      this._rnToSec = b;
      return;
    }
    // Expand-only (Lazy Contract): never raise fromSec or lower toSec.
    this._rnFromSec = Math.min(this._rnFromSec, a);
    this._rnToSec = Math.max(this._rnToSec, b);
  }

  /**
   * Snapshot bars currently inside the Retained Neighborhood (for soft projection merge).
   * @returns {{
   *   times: number[],
   *   candles: { open: number[], high: number[], low: number[], close: number[], volume: number[] },
   *   plots: Record<string, number[]>,
   *   annotations: object[],
   * }|null}
   */
  _snapshotRetainedNeighborhoodBars() {
    const bounds = this.retainedNeighborhoodBounds();
    if (!bounds || this._times.length === 0) return null;
    const times = [];
    const open = [];
    const high = [];
    const low = [];
    const close = [];
    const volume = [];
    const plotIds = Object.keys(this._plots);
    const plots = {};
    for (const id of plotIds) plots[id] = [];
    for (let i = 0; i < this._times.length; i++) {
      const t = Number(this._times[i]);
      if (!Number.isFinite(t) || t < bounds.fromSec || t > bounds.toSec) continue;
      times.push(t);
      open.push(this._candles.open[i]);
      high.push(this._candles.high[i]);
      low.push(this._candles.low[i]);
      close.push(this._candles.close[i]);
      volume.push(this._candles.volume[i]);
      for (const id of plotIds) {
        const col = this._plots[id];
        plots[id].push(Array.isArray(col) ? col[i] : ColumnarStore.plotAbsent());
      }
    }
    if (times.length === 0) return null;
    const annotations = this._annotations.filter((ann) => {
      const t = Number(ann?.time ?? ann?.Time);
      return Number.isFinite(t) && t >= bounds.fromSec && t <= bounds.toSec;
    });
    return {
      times,
      candles: { open, high, low, close, volume },
      plots,
      annotations,
    };
  }

  /**
   * Track B Step 3: re-insert RN bars missing after a soft projection replace.
   * Projection merge must not amputate the Retained Neighborhood.
   * @param {ReturnType<ColumnarStore['_snapshotRetainedNeighborhoodBars']>} snap
   */
  _restoreMissingRetainedBars(snap) {
    if (!snap || !Array.isArray(snap.times) || snap.times.length === 0) return;
    const have = new Set(this._times.map((t) => Number(t)));
    const missingIdx = [];
    for (let j = 0; j < snap.times.length; j++) {
      const t = Number(snap.times[j]);
      if (Number.isFinite(t) && !have.has(t)) missingIdx.push(j);
    }
    if (missingIdx.length === 0) return;

    const absent = ColumnarStore.plotAbsent();
    const entries = [];
    for (let i = 0; i < this._times.length; i++) {
      const plotRow = {};
      for (const id of Object.keys(this._plots)) {
        plotRow[id] = this._plots[id][i];
      }
      entries.push({
        time: Number(this._times[i]),
        open: this._candles.open[i],
        high: this._candles.high[i],
        low: this._candles.low[i],
        close: this._candles.close[i],
        volume: this._candles.volume[i],
        plots: plotRow,
      });
    }
    const snapPlotIds = Object.keys(snap.plots || {});
    for (const j of missingIdx) {
      const plotRow = {};
      for (const id of new Set([...Object.keys(this._plots), ...snapPlotIds])) {
        const col = snap.plots?.[id];
        plotRow[id] = Array.isArray(col) ? col[j] : absent;
      }
      entries.push({
        time: Number(snap.times[j]),
        open: snap.candles.open[j],
        high: snap.candles.high[j],
        low: snap.candles.low[j],
        close: snap.candles.close[j],
        volume: snap.candles.volume[j],
        plots: plotRow,
      });
    }
    entries.sort((a, b) => a.time - b.time);

    const allPlotIds = new Set([...Object.keys(this._plots), ...snapPlotIds]);
    this._times = entries.map((e) => e.time);
    this._candles = {
      open: entries.map((e) => e.open),
      high: entries.map((e) => e.high),
      low: entries.map((e) => e.low),
      close: entries.map((e) => e.close),
      volume: entries.map((e) => e.volume),
    };
    const nextPlots = {};
    for (const id of allPlotIds) {
      nextPlots[id] = entries.map((e) => (e.plots[id] !== undefined ? e.plots[id] : absent));
    }
    this._plots = nextPlots;

    const t0 = this._times[0];
    const t1 = this._times[this._times.length - 1];
    const keepAnn = this._annotations.filter((ann) => {
      const t = Number(ann?.time ?? ann?.Time);
      return Number.isFinite(t) && t >= t0 && t <= t1;
    });
    const haveAnn = new Set(keepAnn.map((a) => Number(a?.time ?? a?.Time)));
    for (const ann of snap.annotations || []) {
      const t = Number(ann?.time ?? ann?.Time);
      if (Number.isFinite(t) && t >= t0 && t <= t1 && !haveAnn.has(t)) {
        keepAnn.push(ann);
        haveAnn.add(t);
      }
    }
    this._annotations = keepAnn;
    this._rebuildAnnotationMapFromArray(this._annotations);
    this._meta = { ...this._meta, added: this._times.length };
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

  /**
   * ADR-015 / B2.1: apply a server ProjectionSnapshot atomically.
   * Accepts the columnar history response as-is (times + OHLC + plots + annotations).
   * Never truncates to prior store length. Never fabricates candles.
   * Server owns projection length; FE only applies.
   *
   * Budget: preserve-paired callers must pass VIEW bounds (WS-01…WS-03).
   * Commit-paired callers (TF / FreshLive / loadDashboard) pass commitPaired: true
   * (no Mutation Set — intentional world replace).
   * Track B Step 1: non-commit growth establishes Mutation Set for same-operation prune.
   * Track B Step 2: absorb Mutation into Retained Neighborhood; commit-paired resets RN.
   * Track B Step 3: soft/preserve merge must not contract RN (restore missing RN bars).
   * @param {object} snapshot
   * @param {{
   *   viewFromSec?: number|null,
   *   viewToSec?: number|null,
   *   commitPaired?: boolean,
   * }} [options]
   */
  applyProjection(snapshot, options = {}) {
    const data = snapshot && typeof snapshot === 'object' ? snapshot : {};
    const times = Array.isArray(data.times)
      ? data.times.map((t) => ColumnarStore._normTimeSec(t))
      : [];
    const src = data.candles && typeof data.candles === 'object' ? data.candles : {};
    const commitPaired = options.commitPaired === true;
    // Lazy Contract: soft projection merge must not amputate RN — capture before replace.
    const rnSnap = commitPaired ? null : this._snapshotRetainedNeighborhoodBars();
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
      if (!Array.isArray(col)) continue;
      // Full column as projected — never slice to a previous store length (ADR-015).
      this._plots[id] = col.slice();
    }
    this._annotations = Array.isArray(data.annotations) ? data.annotations.slice() : [];
    this._rebuildAnnotationMapFromArray(this._annotations);
    const proj = data.projCont && typeof data.projCont === 'object' ? data.projCont : null;
    this._meta = {
      hasMore: data.hasMore === true,
      tf: data.timeframe || '',
      warmupDropped: Number(data.warmupDropped) || 0,
      added: Number(data.added) || times.length,
      projectedForming: proj ? proj.projectedForming === true : undefined,
      generation: data.generation != null ? Number(data.generation) : undefined,
    };
    // Debt #69A: Microscope / HISTORY island must not stay 'live' or WS gap-heal fires.
    // Default remains 'live' (FreshLive / tip hydrate). Callers pass windowMode:'history'
    // for intentional behind-live islands (TF-switch center hydrate).
    if (options.windowMode === 'history' || options.windowMode === 'live') {
      this.windowMode = options.windowMode;
    } else {
      this.windowMode = 'live';
    }
    const budgetOpts = {
      viewFromSec: options.viewFromSec,
      viewToSec: options.viewToSec,
    };
    // Explicit world replacement resets Retained Neighborhood (CameraCommit / commit-paired).
    // S6: accept the full monolith — do NOT TARGET/HARD_CAP-truncate on commit-paired accept
    // (P-01 / Lifetime&Capacity Rules 1, 2, 8). Preserve-paired paths still enforce budget below.
    if (commitPaired) {
      this._clearRetainedNeighborhood();
      return;
    }
    // Exploration merge: restore any RN bars the projection omitted, then absorb Mutation.
    this._restoreMissingRetainedBars(rnSnap);
    if (times.length > 0) {
      const mutFrom = Number(times[0]);
      const mutTo = Number(times[times.length - 1]);
      budgetOpts.mutationFromSec = mutFrom;
      budgetOpts.mutationToSec = mutTo;
      this._absorbIntoRetainedNeighborhood(mutFrom, mutTo);
    }
    // Existing HARD_CAP check = pressure trigger (Capacity-owned); must not clear RN.
    this._enforceBudget(ColumnarStore.PRUNE_FROM_OLDEST, budgetOpts);
  }

  /**
   * Full history hydrate — same atomic accept as applyProjection (legacy name).
   * Commit-paired world replace (FreshLive / TF / loadDashboard): pass commitPaired: true.
   * S6: commit-paired accepts the full monolith (no TARGET amputate). Soft/preserve omit the flag.
   */
  replaceMonolith(columnarJson, options = {}) {
    this.applyProjection(columnarJson, options);
  }

  /**
   * Soft update: replace/add plot columns only. Never mutates _times or _candles.
   * Arrays are padded/truncated to current barCount for invariant safety.
   * Do NOT use for ADR-010 projected forming tips — use applyProjection (B2.1).
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

  /** Read-only bar open times (sec). Caller must not mutate. */
  timesSec() {
    return this._times;
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
   * Map a visible logical range to store time bounds (capture before merge/prune).
   * @param {number[]} timesSec
   * @param {{ from: number, to: number }|null|undefined} range
   * @returns {{ viewFromSec: number, viewToSec: number }|null}
   */
  static logicalRangeToViewTimes(timesSec, range) {
    if (!Array.isArray(timesSec) || !timesSec.length || !range) return null;
    if (!Number.isFinite(range.from) || !Number.isFinite(range.to) || !(range.to > range.from)) {
      return null;
    }
    const n = timesSec.length;
    let i0 = Math.floor(range.from);
    let i1 = Math.floor(range.to);
    if (i1 < i0) i1 = i0;
    i0 = Math.max(0, Math.min(n - 1, i0));
    i1 = Math.max(0, Math.min(n - 1, i1));
    const viewFromSec = Number(timesSec[i0]);
    const viewToSec = Number(timesSec[i1]);
    if (![viewFromSec, viewToSec].every(Number.isFinite)) return null;
    return { viewFromSec, viewToSec };
  }

  /**
   * Inclusive VIEW time → half-open index range [start, end) in current _times.
   * @returns {{ start: number, end: number }|null}
   */
  _viewIndexRange(viewFromSec, viewToSec) {
    const n = this._times.length;
    if (n === 0) return null;
    const a = Number(viewFromSec);
    const b = Number(viewToSec);
    if (![a, b].every(Number.isFinite) || b < a) return null;
    let start = 0;
    while (start < n && Number(this._times[start]) < a) start += 1;
    let end = n;
    while (end > start && Number(this._times[end - 1]) > b) end -= 1;
    if (start >= end) return null;
    return { start, end };
  }

  /**
   * Atomic slice [start, end) for times, candles.*, plots, annotations.
   * @param {number} start
   * @param {number} end exclusive
   * @param {{ droppedNewest?: boolean }} [meta]
   */
  _applySlice(start, end, meta = {}) {
    const n = this._times.length;
    const s = Math.max(0, Math.min(n, Math.floor(Number(start)) || 0));
    const e = Math.max(s, Math.min(n, Math.floor(Number(end)) || 0));
    if (s === 0 && e === n) return;

    this._times = this._times.slice(s, e);
    this._candles = {
      open: this._candles.open.slice(s, e),
      high: this._candles.high.slice(s, e),
      low: this._candles.low.slice(s, e),
      close: this._candles.close.slice(s, e),
      volume: this._candles.volume.slice(s, e),
    };
    const nextPlots = {};
    for (const [id, col] of Object.entries(this._plots)) {
      nextPlots[id] = Array.isArray(col) ? col.slice(s, e) : [];
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

    if (meta.droppedNewest === true) {
      this.windowMode = 'history';
    }
    this._meta = { ...this._meta, added: this._times.length };
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
    this._applySlice(start, end, {
      droppedNewest: direction === ColumnarStore.PRUNE_FROM_NEWEST,
    });
  }

  /**
   * Track A + Track B: prune only outside Working Set, Mutation Set, and Retained Neighborhood.
   * Never remove VIEW (WS). Never remove same-op Mutation (Step 1). Never remove RN (Step 2).
   *
   * P-02: keep set MUST be a contiguous slice. Scattering drops between Mutation and VIEW
   * produces Frankenstein islands (price cliffs / empty time gaps) on the chart.
   *
   * @param {number} targetCount
   * @param {'oldest'|'newest'} direction
   * @param {{ start: number, end: number }|null|undefined} view
   * @param {{ start: number, end: number }|null|undefined} mutation
   * @param {{ start: number, end: number }|null|undefined} neighborhood
   */
  _pruneOutsideProtected(targetCount, direction, view, mutation, neighborhood) {
    const n = this._times.length;
    if (n === 0) return;

    const viewLen = view ? Math.max(0, view.end - view.start) : 0;
    const keepGoal = Math.max(Math.floor(Number(targetCount)) || 0, viewLen);
    if (n <= keepGoal) return;

    let protLo = n;
    let protHi = 0;
    const markSpan = (range) => {
      if (!range) return;
      const a = Math.max(0, Math.min(n, range.start));
      const b = Math.max(a, Math.min(n, range.end));
      if (a >= b) return;
      if (a < protLo) protLo = a;
      if (b > protHi) protHi = b;
    };
    markSpan(view);
    markSpan(mutation);
    markSpan(neighborhood);

    let start;
    let end;
    if (protLo >= protHi) {
      if (direction === ColumnarStore.PRUNE_FROM_NEWEST) {
        start = 0;
        end = keepGoal;
      } else {
        start = Math.max(0, n - keepGoal);
        end = n;
      }
    } else if (protLo === 0 && protHi === n) {
      // Entire series sits in the protected union (VIEW + Mutation + bridge).
      const hard = ColumnarStore.BUDGET_HARD_CAP;
      if (Number.isFinite(hard) && hard > keepGoal && n <= hard) {
        return;
      }
      // Opposite-end VIEW + tip Mutation: contiguous shrink must kill one of them
      // or punch a hole. Prefer soft overage (P-02 + WS-02) over Frankenstein.
      if (view && mutation
        && view.start < mutation.start
        && mutation.end >= n
        && (view.end - view.start) + (mutation.end - mutation.start) < n) {
        return;
      }
      const forceKeep = Math.min(
        n,
        Math.max(keepGoal, Number.isFinite(hard) && hard > 0 ? hard : keepGoal),
      );
      if (direction === ColumnarStore.PRUNE_FROM_OLDEST
        && mutation && mutation.end >= n && mutation.start >= 0
        && !(view && view.start === 0)) {
        start = Math.max(0, n - forceKeep);
        end = n;
      } else if (view && view.end > view.start && (view.end - view.start) <= forceKeep) {
        end = Math.min(n, view.end);
        start = end - forceKeep;
        if (start < 0) {
          start = 0;
          end = forceKeep;
        }
      } else if (direction === ColumnarStore.PRUNE_FROM_NEWEST) {
        start = 0;
        end = forceKeep;
      } else {
        start = Math.max(0, n - forceKeep);
        end = n;
      }
    } else {
      // Contiguous cover of the protected union (includes any bridge between
      // Mutation and VIEW — that bridge must NOT be punched out).
      let lo = protLo;
      let hi = protHi;
      let span = hi - lo;
      if (span < keepGoal) {
        let need = keepGoal - span;
        if (direction === ColumnarStore.PRUNE_FROM_NEWEST) {
          const leftTake = Math.min(lo, need);
          lo -= leftTake;
          need -= leftTake;
          hi = Math.min(n, hi + need);
        } else {
          const rightTake = Math.min(n - hi, need);
          hi += rightTake;
          need -= rightTake;
          lo = Math.max(0, lo - need);
        }
        start = lo;
        end = hi;
      } else if (span > keepGoal) {
        // Cannot keep full Mutation∪VIEW∪bridge under budget without a hole.
        // P-02 forbids holes → contiguous window of keepGoal, anchored on VIEW (WS-02).
        if (view && view.end > view.start && (view.end - view.start) <= keepGoal) {
          end = Math.min(n, view.end);
          start = end - keepGoal;
          if (start < 0) {
            start = 0;
            end = keepGoal;
          }
          if (direction === ColumnarStore.PRUNE_FROM_NEWEST && start > 0 && mutation
            && view) {
            const m0 = Math.max(0, mutation.start);
            if (m0 < start) {
              const shift = Math.min(start - m0, n - end);
              const nextStart = start - shift;
              const nextEnd = end - shift;
              // Only absorb Mutation if VIEW remains fully inside the window.
              if (nextStart <= view.start && nextEnd >= view.end) {
                start = nextStart;
                end = nextEnd;
              }
            }
          }
        } else if (direction === ColumnarStore.PRUNE_FROM_NEWEST) {
          start = lo;
          end = lo + keepGoal;
        } else {
          start = hi - keepGoal;
          end = hi;
        }
      } else {
        start = lo;
        end = hi;
      }
    }

    start = Math.max(0, Math.min(n, start));
    end = Math.max(start, Math.min(n, end));
    if (end - start >= n) return;

    const oldLast = Number(this._times[n - 1]);
    this._applySlice(start, end, {
      droppedNewest: Number.isFinite(oldLast)
        && end > start
        && Number(this._times[end - 1]) < oldLast,
    });
  }

  /**
   * Keep listed indices (ascending). Contiguous → slice; else gather.
   * @param {number[]} keepIdx
   * @param {{ droppedNewest?: boolean }} [meta]
   */
  _gatherIndices(keepIdx, meta = {}) {
    const n = this._times.length;
    if (!Array.isArray(keepIdx) || keepIdx.length === 0) {
      this._times = [];
      this._candles = { open: [], high: [], low: [], close: [], volume: [] };
      this._plots = {};
      this._annotations = [];
      this._annotationMap.clear();
      this._meta = { ...this._meta, added: 0 };
      this._clearRetainedNeighborhood();
      return;
    }
    if (keepIdx.length === n) return;

    const start = keepIdx[0];
    const endEx = keepIdx[keepIdx.length - 1] + 1;
    const contiguous = keepIdx.length === endEx - start
      && keepIdx.every((v, j) => v === start + j);
    if (contiguous) {
      this._applySlice(start, endEx, meta);
      return;
    }

    const pick = (arr) => keepIdx.map((i) => arr[i]);
    this._times = pick(this._times);
    this._candles = {
      open: pick(this._candles.open),
      high: pick(this._candles.high),
      low: pick(this._candles.low),
      close: pick(this._candles.close),
      volume: pick(this._candles.volume),
    };
    const nextPlots = {};
    for (const [id, col] of Object.entries(this._plots)) {
      nextPlots[id] = Array.isArray(col) ? pick(col) : [];
    }
    this._plots = nextPlots;

    const t0 = Number(this._times[0]);
    const t1 = Number(this._times[this._times.length - 1]);
    this._annotations = this._annotations.filter((ann) => {
      const t = Number(ann?.time ?? ann?.Time);
      return Number.isFinite(t) && t >= t0 && t <= t1;
    });
    this._rebuildAnnotationMapFromArray(this._annotations);
    if (meta.droppedNewest === true) {
      this.windowMode = 'history';
    }
    this._meta = { ...this._meta, added: this._times.length };
  }

  /**
   * Pressure prune when over MAX_STORE / HARD_CAP.
   * Moving-window policy: VIEW (+ same-op Mutation) is sacred; RN must not veto the
   * hard working-set cap (otherwise island scroll grows past the cap indefinitely).
   * @param {'oldest'|'newest'} direction
   * @param {{
   *   viewFromSec?: number|null,
   *   viewToSec?: number|null,
   *   mutationFromSec?: number|null,
   *   mutationToSec?: number|null,
   * }} [opts]
   */
  _enforceBudget(direction, opts = {}) {
    if (this._times.length <= ColumnarStore.BUDGET_HARD_CAP) return;
    const view = this._viewIndexRange(opts.viewFromSec, opts.viewToSec);
    const mutation = this._viewIndexRange(opts.mutationFromSec, opts.mutationToSec);
    if (!view && !mutation) {
      this._pruneToCount(ColumnarStore.BUDGET_TARGET, direction);
      this._clampRetainedNeighborhoodToSeries();
      return;
    }
    // Hard cap: do not pass RN as protected — viewport remains sacred via `view`.
    this._pruneOutsideProtected(
      ColumnarStore.BUDGET_TARGET,
      direction,
      view,
      mutation,
      null,
    );
    this._clampRetainedNeighborhoodToSeries();
  }

  /** After capacity prune, keep RN inside the remaining series (or clear). */
  _clampRetainedNeighborhoodToSeries() {
    if (this._times.length === 0) {
      this._clearRetainedNeighborhood();
      return;
    }
    if (this._rnFromSec == null || this._rnToSec == null) return;
    const t0 = Number(this._times[0]);
    const t1 = Number(this._times[this._times.length - 1]);
    if (![t0, t1].every(Number.isFinite)) {
      this._clearRetainedNeighborhood();
      return;
    }
    this._rnFromSec = Math.max(this._rnFromSec, t0);
    this._rnToSec = Math.min(this._rnToSec, t1);
    if (!(this._rnFromSec <= this._rnToSec)) {
      this._clearRetainedNeighborhood();
    }
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
   * Preserve-paired: pass VIEW bounds so new-bar budget prune cannot invalidate VIEW.
   * @param {object} tick
   * @param {{ viewFromSec?: number|null, viewToSec?: number|null }} [options]
   * @returns {{ candle: object, isNewBar: boolean, barCount: number, tick: object, delta: object }|null}
   */
  appendTick(tick, options = {}) {
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

    // Dense TFs: a forward jump beyond 1.5 intervals is a chronology gap (server heal).
    // Sparse micro TFs (seconds/ticks): missing buckets are legal; never report a gap.
    if (this._denseContinuity && isNewBar && this._intervalSec > 0 && lastTime != null
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
      // Track B Step 1–3: Mutation Set + absorb into RN (exploration expand); pressure may drop outside RN.
      this._absorbIntoRetainedNeighborhood(time, time);
      this._enforceBudget(ColumnarStore.PRUNE_FROM_OLDEST, {
        viewFromSec: options.viewFromSec,
        viewToSec: options.viewToSec,
        mutationFromSec: time,
        mutationToSec: time,
      });
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

    // Moving-window: LEFT growth discards RIGHT (newest). Explicit override allowed.
    const direction = (options.pruneDirection === ColumnarStore.PRUNE_FROM_OLDEST
      || options.pruneDirection === ColumnarStore.PRUNE_FROM_NEWEST)
      ? options.pruneDirection
      : ColumnarStore.PRUNE_FROM_NEWEST;
    const mutationFromSec = Number(prependTimes[0]);
    const mutationToSec = Number(prependTimes[added - 1]);
    const lengthAfterPrepend = this._times.length;
    this._absorbIntoRetainedNeighborhood(mutationFromSec, mutationToSec);
    this._enforceBudget(direction, {
      viewFromSec: options.viewFromSec,
      viewToSec: options.viewToSec,
      mutationFromSec,
      mutationToSec,
    });
    const lengthAfter = this._times.length;
    const prunedTotal = Math.max(0, lengthAfterPrepend - lengthAfter);
    // LEFT growth → opposite-side (RIGHT) prune under the moving-window policy.
    const prunedRightCount = direction === ColumnarStore.PRUNE_FROM_NEWEST ? prunedTotal : 0;
    const prunedLeftCount = direction === ColumnarStore.PRUNE_FROM_OLDEST ? prunedTotal : 0;

    return {
      added,
      prependedCount: added,
      prunedLeftCount,
      prunedRightCount,
      pruneDirection: direction,
      windowMode: this.windowMode,
    };
  }

  /**
   * Append newer monolith bars after the current tip (right-edge history island fill).
   * Symmetric to prependMonolith: only times strictly after lastTimeSec are accepted.
   * Moving-window: RIGHT growth discards LEFT (oldest) under MAX_STORE.
   * @param {object} columnarJson
   * @param {{
   *   focalTimeSec?: number|null,
   *   atLiveEdge?: boolean,
   *   pruneDirection?: string,
   *   viewFromSec?: number|null,
   *   viewToSec?: number|null,
   * }} [options]
   * @returns {{ added: number, pruneDirection?: string, windowMode?: string }}
   */
  appendMonolith(columnarJson, options = {}) {
    const data = columnarJson && typeof columnarJson === 'object' ? columnarJson : {};
    const incomingTimes = Array.isArray(data.times) ? data.times : [];
    if (incomingTimes.length === 0) return { added: 0 };

    const tipTime = this._times.length > 0 ? Number(this._times[this._times.length - 1]) : null;
    const existing = new Set(this._times);
    const src = data.candles && typeof data.candles === 'object' ? data.candles : {};
    const incomingPlots = data.plots && typeof data.plots === 'object' ? data.plots : {};

    const indices = [];
    for (let i = 0; i < incomingTimes.length; i++) {
      const t = Number(incomingTimes[i]);
      if (!Number.isFinite(t)) continue;
      if (tipTime != null && t <= tipTime) continue;
      if (existing.has(t) || existing.has(incomingTimes[i])) continue;
      indices.push(i);
    }

    if (indices.length === 0) return { added: 0 };

    const added = indices.length;
    const appendTimes = new Array(added);
    const appended = {
      open: new Array(added),
      high: new Array(added),
      low: new Array(added),
      close: new Array(added),
      volume: new Array(added),
    };
    for (let j = 0; j < added; j++) {
      const i = indices[j];
      appendTimes[j] = incomingTimes[i];
      appended.open[j] = src.open?.[i];
      appended.high[j] = src.high?.[i];
      appended.low[j] = src.low?.[i];
      appended.close[j] = src.close?.[i];
      appended.volume[j] = src.volume?.[i];
    }

    const plotIds = new Set([...Object.keys(this._plots), ...Object.keys(incomingPlots)]);
    const mergedPlots = {};
    for (const id of plotIds) {
      const cur = this._plots[id] || [];
      const inc = incomingPlots[id] || [];
      const tail = new Array(added);
      for (let j = 0; j < added; j++) {
        tail[j] = inc[indices[j]];
      }
      mergedPlots[id] = cur.concat(tail);
    }

    const appendAnnTimes = new Set(appendTimes);
    const keptAnn = this._annotations.filter((ann) => {
      const t = Number(ann?.time ?? ann?.Time);
      return Number.isFinite(t) && !appendAnnTimes.has(t);
    });
    const incomingAnns = Array.isArray(data.annotations) ? data.annotations : [];
    const newAnns = incomingAnns.filter((ann) => {
      const t = Number(ann?.time ?? ann?.Time);
      return Number.isFinite(t) && appendAnnTimes.has(t);
    });

    this._times = this._times.concat(appendTimes);
    this._candles = {
      open: this._candles.open.concat(appended.open),
      high: this._candles.high.concat(appended.high),
      low: this._candles.low.concat(appended.low),
      close: this._candles.close.concat(appended.close),
      volume: this._candles.volume.concat(appended.volume),
    };
    this._plots = mergedPlots;
    this._annotations = keptAnn.concat(newAnns);
    this._rebuildAnnotationMapFromArray(this._annotations);
    this._meta = {
      ...this._meta,
      added: this._times.length,
    };

    // Moving-window: RIGHT growth discards LEFT (oldest). Explicit override allowed.
    const direction = (options.pruneDirection === ColumnarStore.PRUNE_FROM_OLDEST
      || options.pruneDirection === ColumnarStore.PRUNE_FROM_NEWEST)
      ? options.pruneDirection
      : ColumnarStore.PRUNE_FROM_OLDEST;
    const mutationFromSec = Number(appendTimes[0]);
    const mutationToSec = Number(appendTimes[added - 1]);
    const lengthAfterAppend = this._times.length;
    this._absorbIntoRetainedNeighborhood(mutationFromSec, mutationToSec);
    this._enforceBudget(direction, {
      viewFromSec: options.viewFromSec,
      viewToSec: options.viewToSec,
      mutationFromSec,
      mutationToSec,
    });

    // Right-edge fill reached wall-clock tip → resume live WS ingest (Debt #69A).
    this._maybePromoteLiveWindow();

    const lengthAfter = this._times.length;
    const prunedTotal = Math.max(0, lengthAfterAppend - lengthAfter);
    const prunedLeftCount = direction === ColumnarStore.PRUNE_FROM_OLDEST ? prunedTotal : 0;
    const prunedRightCount = direction === ColumnarStore.PRUNE_FROM_NEWEST ? prunedTotal : 0;

    return {
      added,
      appendedCount: added,
      prunedLeftCount,
      prunedRightCount,
      pruneDirection: direction,
      windowMode: this.windowMode,
    };
  }

  /**
   * When store tip is within ~2 bars of wall clock, HISTORY island rejoins LIVE.
   * Does not move the camera — only restores WS eligibility.
   */
  _maybePromoteLiveWindow() {
    if (this.windowMode !== 'history') return;
    const tip = this.lastTimeSec();
    if (!Number.isFinite(tip) || tip <= 0) return;
    const iv = this._intervalSec > 0 ? this._intervalSec : 60;
    const nowSec = Math.floor(Date.now() / 1000);
    if (tip + 2 * iv >= nowSec) {
      this.windowMode = 'live';
    }
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
  if (typeof globalThis.ChartDataStore === 'undefined') {
    globalThis.ChartDataStore = require('./store.js').ChartDataStore;
  }
  module.exports = { ColumnarStore };
}
