/**
 * ChartDataStore — ms-keyed candle/osc/annotation store.
 * Annotations: Map<snappedMs, flatProps> — O(1) marker lookup on the TimeGrid.
 */
class ChartDataStore {
  constructor(context) {
    this.context = context;
    this.candles = new Map();
    this.osc = new Map();
    /** @type {Map<number, object>} snappedMs → { spikeUp, spikeDown, volCross, rsxLabel, ... } */
    this.annotations = new Map();
    this._dirtyMs = null;
    this._dirtyIsNewBar = false;
    this._dirtyAnnotations = false;
    this._sealed = false;
    this._trades = [];
    this._fingerprint = null;
  }

  static toMs(t) {
    const n = Number(t);
    if (!Number.isFinite(n) || n <= 0) return null;
    return n < 1e12 ? Math.floor(n * 1000) : Math.floor(n);
  }

  static msToChartSec(ms) {
    return Math.floor(ms / 1000);
  }

  static _snapMs(timeLike, tf) {
    const rawMs = ChartDataStore.toMs(timeLike);
    if (!rawMs) return null;
    return TimeNormalizer.snapToGrid(rawMs, tf);
  }

  clear() {
    this.candles.clear();
    this.osc.clear();
    this.annotations.clear();
    this._trades = [];
    this._fingerprint = null;
    this._resetDirtyState();
  }

  setFingerprint(symbol, interval, startSec, endSec) {
    this._fingerprint = {
      symbol: String(symbol || ''),
      interval: String(interval || ''),
      startSec: startSec != null ? Number(startSec) : null,
      endSec: endSec != null ? Number(endSec) : null,
    };
  }

  getFingerprint() {
    return this._fingerprint ? { ...this._fingerprint } : null;
  }

  _resetDirtyState() {
    this._dirtyMs = null;
    this._dirtyIsNewBar = false;
    this._dirtyAnnotations = false;
  }

  seal() { this._sealed = true; }

  unseal() {
    this._sealed = false;
    this._dirtyIsNewBar = false;
    this._pruneOldData();
  }

  _pruneOldData() {
    const maxCap = (typeof CONFIG !== 'undefined' && CONFIG.MAX_STORE_CAPACITY) || 50000;
    const chunk = (typeof CONFIG !== 'undefined' && CONFIG.STORE_PRUNE_CHUNK) || 5000;
    if (this.candles.size <= maxCap) return;
    const sortedKeys = Array.from(this.candles.keys()).sort((a, b) => a - b);
    const keysToRemove = sortedKeys.slice(0, chunk);
    for (const ms of keysToRemove) {
      this.candles.delete(ms);
      this.osc.delete(ms);
      this.annotations.delete(ms);
    }
  }

  isSealed() { return this._sealed; }

  _markDirtyMs(ms, isNewBar = false) {
    if (this._dirtyMs == null) {
      this._dirtyMs = ms;
      this._dirtyIsNewBar = isNewBar;
    } else if (this._dirtyMs === ms) {
      if (isNewBar) this._dirtyIsNewBar = true;
    } else {
      this._dirtyMs = ms;
      this._dirtyIsNewBar = isNewBar;
    }
  }

  _candleAtSnappedMs(bar, tf) {
    const ms = ChartDataStore._snapMs(bar.time, tf);
    if (!ms) return null;
    return {
      ...bar,
      timeMs: ms,
      time: ChartDataStore.msToChartSec(ms),
    };
  }

  _mergeOsc(existing, incoming) {
    return {
      ...existing,
      ...incoming,
      timeMs: existing.timeMs,
      time: existing.time,
    };
  }

  _propsFromServerAnnotation(a, ms) {
    const text = String(a?.text ?? a?.label ?? a?.Label ?? '').trim();
    const props = { timeMs: ms };
    if (text) props.rsxLabel = text.substring(0, 2).toUpperCase();
    if (a?.pane) props.pane = a.pane;
    if (a?.color) props.color = a.color;
    if (a?.position) props.position = a.position;
    if (a?.shape) props.shape = a.shape;
    return props;
  }

  _propsFromOscPoint(snapped) {
    if (!snapped) return null;
    const props = {};
    if (snapped.volumeSpikeUp) props.spikeUp = true;
    if (snapped.volumeSpikeDown) props.spikeDown = true;
    if (snapped.volCrossMarker) props.volCross = snapped.volCrossMarker;
    if (snapped.marker) {
      props.rsxLabel = String(snapped.marker).trim().substring(0, 2).toUpperCase();
    }
    return Object.keys(props).length ? props : null;
  }

  _mergeAnnotationProps(ms, incoming) {
    if (!incoming || !Object.keys(incoming).length) return this.annotations.get(ms) || null;
    const existing = this.annotations.get(ms) || { timeMs: ms };
    const merged = Object.assign({}, existing, incoming, { timeMs: ms });
    if (existing.spikeUp || incoming.spikeUp) merged.spikeUp = true;
    if (existing.spikeDown || incoming.spikeDown) merged.spikeDown = true;
    const changed = JSON.stringify(merged) !== JSON.stringify(existing);
    this.annotations.set(ms, merged);
    if (changed) this._dirtyAnnotations = true;
    return merged;
  }

  getAnnotationsMap() {
    return this.annotations;
  }

  getAnnotationAt(ms) {
    return this.annotations.get(ms) || null;
  }

  upsertAnnotationProps(ms, props, tf) {
    const snapped = TimeNormalizer.snapToGrid(ms, tf);
    if (!snapped) return null;
    return this._mergeAnnotationProps(snapped, { ...props, timeMs: snapped });
  }

  _syncOscAnnotationProps(snapped) {
    const props = this._propsFromOscPoint(snapped);
    if (!props) return;
    this._mergeAnnotationProps(snapped.timeMs, props);
  }

  _annotationsToArray() {
    const chartAnnotations = [];
    this.annotations.forEach((props, ms) => {
      const time = ChartDataStore.msToChartSec(ms);
      if (props.rsxLabel) {
        chartAnnotations.push({
          time,
          timeMs: ms,
          text: props.rsxLabel,
          pane: props.pane || 'rsx',
          color: props.color,
          position: props.position,
          shape: props.shape,
        });
      }
    });
    chartAnnotations.sort((a, b) => a.time - b.time);
    return chartAnnotations;
  }

  candlesArray() {
    return Array.from(this.candles.values()).sort((a, b) => a.timeMs - b.timeMs);
  }

  candleCount() {
    return this.candles.size;
  }

  hasBaseLayer() {
    return this.candleCount() > 0;
  }

  getCoverage() {
    return {
      startSec: this.firstCandleTimeSec(),
      endSec: this.lastCandleTimeSec(),
    };
  }

  coversRange(reqStartSec, reqEndSec) {
    if (!this.hasBaseLayer()) return false;
    const coverage = this.getCoverage();
    if (coverage.startSec == null || coverage.endSec == null) return false;
    return reqStartSec >= coverage.startSec && reqEndSec <= coverage.endSec;
  }

  setTrades(trades) {
    this._trades = trades || [];
  }

  getTrades() {
    return this._trades || [];
  }

  patchBacktestData(payload, tf) {
    let patchedOsc = 0;

    let oscillators = payload.oscillators || [];
    if (!oscillators.length && Array.isArray(payload.simData) && payload.simData.length) {
      oscillators = chartPointsToOsc(payload.simData);
    }
    if (!oscillators.length && Array.isArray(payload.chartData) && payload.chartData.length) {
      oscillators = chartPointsToOsc(payload.chartData);
    }

    oscillators.forEach((o) => {
      const norm = typeof Mappers !== 'undefined' ? Mappers.normalizeOscPoint(o) : normalizeOscPoint(o);
      if (!norm) return;
      const ms = ChartDataStore._snapMs(norm.time, tf);
      if (!ms) return;

      const snapped = { ...norm, timeMs: ms, time: ChartDataStore.msToChartSec(ms) };
      const existing = this.osc.get(ms);

      if (existing) {
        const merged = this._mergeOsc(existing, snapped);
        this.osc.set(ms, merged);
        this._syncOscAnnotationProps(merged);
      } else {
        this.osc.set(ms, snapped);
        this._syncOscAnnotationProps(snapped);
      }
      patchedOsc += 1;
    });

    if (Array.isArray(payload.annotations)) {
      this._ingestAnnotations(payload.annotations, tf);
    }

    this._resetDirtyState();
    return { patchedOsc };
  }

  sortedCandleTimesMs() {
    return Array.from(this.candles.keys()).sort((a, b) => a - b);
  }

  firstCandleTimeSec() {
    const times = this.sortedCandleTimesMs();
    return times.length ? ChartDataStore.msToChartSec(times[0]) : null;
  }

  lastCandleTimeSec() {
    const times = this.sortedCandleTimesMs();
    return times.length ? ChartDataStore.msToChartSec(times[times.length - 1]) : null;
  }

  lastCandleChartSec() {
    const times = this.sortedCandleTimesMs();
    if (!times.length) return null;
    const ms = times[times.length - 1];
    const bar = this.candles.get(ms);
    if (!bar) return null;
    return { ...bar, time: ChartDataStore.msToChartSec(ms) };
  }

  annotationCount() {
    return this.annotations.size;
  }

  _ingestAnnotations(annArray, tf, anchorMs = null) {
    if (!Array.isArray(annArray)) return;
    annArray.forEach((a) => {
      const rawMs = ChartDataStore.toMs(a.time ?? a.Time);
      if (!rawMs) return;
      const ms = TimeNormalizer.snapToGrid(rawMs, tf);
      if (!ms) return;
      if (anchorMs != null && ms >= anchorMs) return;
      const props = this._propsFromServerAnnotation(a, ms);
      this._mergeAnnotationProps(ms, props);
    });
  }

  _ingestCandle(c, tf, { allowOverwrite = true, anchorMs = null } = {}) {
    const bar = normalizeCandle(c);
    if (!bar) return;
    const snapped = this._candleAtSnappedMs(bar, tf);
    if (!snapped) return;
    const { timeMs: ms } = snapped;
    if (anchorMs != null && ms >= anchorMs) return;

    const existing = this.candles.get(ms);
    if (existing) {
      this.candles.set(ms, TimeNormalizer.mergeCandles(existing, snapped));
      return;
    }
    if (!allowOverwrite && this.candles.has(ms)) return;
    this.candles.set(ms, snapped);
  }

  _ingestOsc(o, tf, { allowOverwrite = true, anchorMs = null } = {}) {
    const norm = normalizeOscPoint(o);
    if (!norm) return;
    const ms = ChartDataStore._snapMs(norm.time, tf);
    if (!ms) return;
    if (anchorMs != null && ms >= anchorMs) return;

    const snapped = {
      ...norm,
      timeMs: ms,
      time: ChartDataStore.msToChartSec(ms),
    };

    const existing = this.osc.get(ms);
    if (existing) {
      const merged = this._mergeOsc(existing, snapped);
      this.osc.set(ms, merged);
      this._syncOscAnnotationProps(merged);
      return;
    }
    if (!allowOverwrite && this.osc.has(ms)) return;
    this.osc.set(ms, snapped);
    this._syncOscAnnotationProps(snapped);
  }

  replaceFromServer(payload, tf) {
    this.candles.clear();
    this.osc.clear();
    this.annotations.clear();

    (payload.candles || []).forEach((c) => {
      this._ingestCandle(c, tf, { allowOverwrite: true });
    });
    (payload.oscillators || []).forEach((o) => {
      this._ingestOsc(o, tf, { allowOverwrite: true });
    });
    this._ingestAnnotations(payload.annotations, tf);
    this._resetDirtyState();
  }

  prependHistory(payload, tf) {
    let added = 0;
    let rejected = 0;
    const anchorMs = this.sortedCandleTimesMs()[0] ?? null;

    (payload.candles || []).forEach((c) => {
      const bar = normalizeCandle(c);
      if (!bar) {
        rejected += 1;
        return;
      }
      const snapped = this._candleAtSnappedMs(bar, tf);
      if (!snapped) {
        rejected += 1;
        return;
      }
      const { timeMs: ms } = snapped;
      if (anchorMs != null && ms >= anchorMs) {
        rejected += 1;
        return;
      }
      if (this.candles.has(ms)) {
        this.candles.set(ms, TimeNormalizer.mergeCandles(this.candles.get(ms), snapped));
        rejected += 1;
      } else {
        this.candles.set(ms, snapped);
        added += 1;
      }
    });

    (payload.oscillators || []).forEach((o) => {
      const norm = normalizeOscPoint(o);
      if (!norm) {
        rejected += 1;
        return;
      }
      const ms = ChartDataStore._snapMs(norm.time, tf);
      if (!ms) {
        rejected += 1;
        return;
      }
      if (anchorMs != null && ms >= anchorMs) {
        rejected += 1;
        return;
      }
      const snapped = {
        ...norm,
        timeMs: ms,
        time: ChartDataStore.msToChartSec(ms),
      };
      if (this.osc.has(ms)) {
        const merged = this._mergeOsc(this.osc.get(ms), snapped);
        this.osc.set(ms, merged);
        this._syncOscAnnotationProps(merged);
        rejected += 1;
      } else {
        this.osc.set(ms, snapped);
        this._syncOscAnnotationProps(snapped);
      }
    });

    this._ingestAnnotations(payload.annotations, tf, anchorMs);
    this._resetDirtyState();

    return { added, rejected, anchorMs };
  }

  upsertCandle(c, tf) {
    const bar = normalizeCandle(c);
    if (!bar) return null;
    const snapped = this._candleAtSnappedMs(bar, tf);
    if (!snapped) return null;
    const { timeMs: ms } = snapped;
    const isNewBar = !this.candles.has(ms);
    const existing = this.candles.get(ms);
    if (existing) {
      this.candles.set(ms, TimeNormalizer.mergeCandles(existing, snapped));
    } else {
      this.candles.set(ms, snapped);
    }
    this._markDirtyMs(ms, isNewBar);
    return { ...this.candles.get(ms), time: ChartDataStore.msToChartSec(ms) };
  }

  upsertOscPoint(o, tf) {
    const norm = normalizeOscPoint(o);
    if (!norm) return null;
    const ms = ChartDataStore._snapMs(norm.time, tf);
    if (!ms) return null;
    const snapped = {
      ...norm,
      timeMs: ms,
      time: ChartDataStore.msToChartSec(ms),
    };
    const existing = this.osc.get(ms);
    if (existing) {
      const merged = this._mergeOsc(existing, snapped);
      this.osc.set(ms, merged);
      this._syncOscAnnotationProps(merged);
    } else {
      this.osc.set(ms, snapped);
      this._syncOscAnnotationProps(snapped);
    }
    this._markDirtyMs(ms, false);
    return { ...this.osc.get(ms), time: ChartDataStore.msToChartSec(ms) };
  }

  replaceOscAndAnnotations(payload, tf) {
    if (Array.isArray(payload.annotations)) {
      this.annotations.clear();
      this._ingestAnnotations(payload.annotations, tf);
    }
    (payload.oscillators || []).forEach((o) => {
      this._ingestOsc(o, tf, { allowOverwrite: true });
    });
    this._resetDirtyState();
  }

  getLatestDeltaForChart() {
    if (this._sealed) return null;
    if (this._dirtyMs == null && !this._dirtyAnnotations) return null;

    const ms = this._dirtyMs;
    const candle = ms != null ? this.candles.get(ms) : null;
    const osc = ms != null ? this.osc.get(ms) : null;
    const latestMs = ms ?? this.sortedCandleTimesMs().pop() ?? null;

    let fullAnnotations = null;
    if (this._dirtyAnnotations) {
      fullAnnotations = this._annotationsToArray();
    }

    const delta = {
      candle: candle ? { ...candle, time: ChartDataStore.msToChartSec(ms) } : null,
      osc: osc ? { ...osc, time: ChartDataStore.msToChartSec(ms) } : null,
      annotation: latestMs != null ? (this.annotations.get(latestMs) || null) : null,
      annotationMs: latestMs,
      fullAnnotations,
      isNewBar: this._dirtyIsNewBar,
    };

    this._dirtyMs = null;
    this._dirtyIsNewBar = false;
    this._dirtyAnnotations = false;
    return delta;
  }

  getSimOverlayPayload() {
    const sortedTimes = Array.from(this.osc.keys()).sort((a, b) => a - b);
    const chartOsc = sortedTimes.map((ms) => ({
      ...this.osc.get(ms),
      time: ChartDataStore.msToChartSec(ms),
    }));
    return {
      osc: chartOsc,
      annotations: this._annotationsToArray(),
    };
  }

  getForLightweightCharts() {
    const sortedCandles = Array.from(this.candles.values()).sort((a, b) => a.timeMs - b.timeMs);
    const chartCandles = sortedCandles.map((c) => ({
      ...c,
      time: ChartDataStore.msToChartSec(c.timeMs),
    }));

    const chartOsc = sortedCandles.map((c) => {
      const oscPoint = this.osc.get(c.timeMs);
      if (oscPoint) {
        return { ...oscPoint, time: ChartDataStore.msToChartSec(c.timeMs) };
      }
      return { time: ChartDataStore.msToChartSec(c.timeMs) };
    });

    return {
      candles: chartCandles,
      osc: chartOsc,
      annotations: this._annotationsToArray(),
    };
  }
}

if (typeof window !== 'undefined') {
  window.ChartDataStore = ChartDataStore;
}
